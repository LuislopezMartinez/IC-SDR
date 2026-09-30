package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"go-zero/internal/i18n"
	"go-zero/internal/resources"
	"go-zero/internal/voacap"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func RunVOACAPMap(path string) {
	simpleui.SetMode(1440, 840, simpleui.Fit)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.5)
	simpleui.SetTitle("PROPAGACIÓN DX · VOACAP")
	simpleui.SetMinimumSize(1000, 600)
	viewer := newVOACAPMap(path)
	simpleui.Run(viewer.draw)
}

type voacapMapPreferences struct {
	LayerAlpha float32              `json:"layerAlpha"`
	RouteSet   bool                 `json:"routeSet"`
	TX         voacap.RouteLocation `json:"tx"`
	RX         voacap.RouteLocation `json:"rx"`
}

type routeCalculationResult struct {
	id         uint64
	prediction voacap.RoutePrediction
}

type voacapMap struct {
	path          string
	snapshot      voacap.Snapshot
	nextRead      time.Time
	lat           float64
	lon           float64
	span          float64
	dragging      bool
	last          rl.Vector2
	viewSet       bool
	manager       *simpleui.Manager
	alpha         *simpleui.Slider
	prefs         voacapMapPreferences
	prefsPath     string
	dragPin       int
	route         voacap.RoutePrediction
	routeAt       time.Time
	routeID       uint64
	routeBusy     bool
	routeCancel   context.CancelFunc
	routeResults  chan routeCalculationResult
	routeSettings string
}

func newVOACAPMap(path string) *voacapMap {
	viewer := &voacapMap{
		path:         path,
		manager:      simpleui.NewManager(),
		prefs:        voacapMapPreferences{LayerAlpha: 65},
		prefsPath:    resources.WritablePath("config", "voacap-map.json"),
		routeResults: make(chan routeCalculationResult, 1),
	}
	if data, err := os.ReadFile(viewer.prefsPath); err == nil {
		_ = json.Unmarshal(data, &viewer.prefs)
	}
	viewer.prefs.LayerAlpha = min(max(viewer.prefs.LayerAlpha, float32(0)), float32(100))
	viewer.alpha = simpleui.NewSlider("voacapMapAlpha", 1062, 792, 330, 20, 0, 100, viewer.prefs.LayerAlpha)
	viewer.alpha.OnChange(func(value float32) { viewer.prefs.LayerAlpha = value })
	viewer.alpha.OnRelease(func(float32) { viewer.savePreferences() })
	viewer.manager.Add(viewer.alpha)
	return viewer
}

func (viewer *voacapMap) savePreferences() {
	data, err := json.MarshalIndent(viewer.prefs, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(viewer.prefsPath), 0o755)
	_ = os.WriteFile(viewer.prefsPath, append(data, '\n'), 0o644)
}

func (viewer *voacapMap) read() {
	if time.Now().Before(viewer.nextRead) {
		return
	}
	viewer.nextRead = time.Now().Add(300 * time.Millisecond)
	data, err := os.ReadFile(viewer.path)
	if err != nil {
		return
	}
	var snapshot voacap.Snapshot
	if json.Unmarshal(data, &snapshot) != nil {
		return
	}
	viewer.snapshot = snapshot
	if !viewer.viewSet {
		viewer.lat, viewer.lon, viewer.span = 0, snapshot.Settings.Longitude, 360
		viewer.viewSet = true
		if !viewer.prefs.RouteSet {
			viewer.prefs.TX = voacap.RouteLocation{Latitude: snapshot.Settings.Latitude, Longitude: snapshot.Settings.Longitude}
			viewer.prefs.RX = voacap.RouteLocation{Latitude: 40.7128, Longitude: -74.0060}
			viewer.prefs.RouteSet = true
			viewer.savePreferences()
		}
	}
	routeSettings := viewer.routeSettingsKey()
	if viewer.dragPin == 0 && routeSettings != viewer.routeSettings {
		viewer.routeSettings = routeSettings
		viewer.scheduleRoute()
	}
	if validTheme(snapshot.Theme) {
		colors = paletteForTheme(snapshot.Theme)
		simpleui.SetTheme(simpleUITheme(colors, snapshot.Theme == themeLight))
	}
}

func (viewer *voacapMap) draw() {
	viewer.read()
	viewer.updateRoute()
	rl.ClearBackground(colors.background)
	title := "PROPAGACIÓN DX · VOACAP"
	titleX := float32(20)
	simpleui.DrawTextStyled(title, titleX, 16, 24, simpleui.FontSemiBold, colors.cyan)
	titleWidth := simpleui.MeasureTextStyled(title, 24, simpleui.FontSemiBold).X
	separatorX := titleX + titleWidth + 18
	rl.DrawLineEx(rl.Vector2{X: separatorX, Y: 17}, rl.Vector2{X: separatorX, Y: 48}, 1, colors.border)
	simpleui.DrawText("MAPA DE COBERTURA HF · FIABILIDAD ESTIMADA", separatorX+18, 24, 13, colors.muted)
	mapBox := rl.Rectangle{X: 18, Y: 66, Width: 1000, Height: 752}
	panel := rl.Rectangle{X: 1034, Y: 66, Width: 388, Height: 752}
	drawPanel(panel.X, panel.Y, panel.Width, panel.Height)
	pointer := simpleui.MousePosition()
	wheel := rl.GetMouseWheelMove()
	viewer.manager.Update(simpleui.Input{Pointer: pointer, PointerInCanvas: true, Pressed: rl.IsMouseButtonPressed(rl.MouseButtonLeft), Down: rl.IsMouseButtonDown(rl.MouseButtonLeft), Released: rl.IsMouseButtonReleased(rl.MouseButtonLeft), Wheel: wheel})
	viewer.input(mapBox, wheel)
	viewer.drawWorld(mapBox)
	viewer.drawRoutePanel(panel)
	viewer.manager.Draw()
}

func (viewer *voacapMap) input(bounds rl.Rectangle, wheel float32) {
	mouse := simpleui.MousePosition()
	overMap := rl.CheckCollisionPointRec(mouse, bounds)
	if overMap && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		tx := projectMercator(viewer.lat, viewer.lon, viewer.span, viewer.prefs.TX.Latitude, viewer.prefs.TX.Longitude, bounds)
		rx := projectMercator(viewer.lat, viewer.lon, viewer.span, viewer.prefs.RX.Latitude, viewer.prefs.RX.Longitude, bounds)
		if rl.Vector2Distance(mouse, tx) <= 20 {
			viewer.dragPin = 1
		} else if rl.Vector2Distance(mouse, rx) <= 20 {
			viewer.dragPin = 2
		} else {
			viewer.dragging = true
		}
		viewer.last = mouse
	}
	if viewer.dragPin != 0 && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		lat, lon := unprojectMercator(viewer.lat, viewer.lon, viewer.span, mouse, bounds)
		location := voacap.RouteLocation{Latitude: min(max(lat, -85.0), 85.0), Longitude: wrapLongitude(lon)}
		if viewer.dragPin == 1 {
			viewer.prefs.TX = location
		} else {
			viewer.prefs.RX = location
		}
	}
	if viewer.dragging && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		delta := rl.Vector2Subtract(mouse, viewer.last)
		viewer.lat, viewer.lon = panMap(viewer.lat, viewer.lon, viewer.span, delta, bounds)
		viewer.last = mouse
	}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		if viewer.dragPin != 0 {
			viewer.dragPin = 0
			viewer.savePreferences()
			viewer.routeSettings = viewer.routeSettingsKey()
			viewer.scheduleRoute()
		}
		viewer.dragging = false
	}
	if overMap && wheel != 0 {
		viewer.lat, viewer.lon, viewer.span = zoomMap(viewer.lat, viewer.lon, viewer.span, mouse, bounds, wheel)
	}
	viewer.lat, viewer.lon, viewer.span = clampMapView(viewer.lat, viewer.lon, viewer.span, bounds)
}

func (viewer *voacapMap) routeSettingsKey() string {
	settings := viewer.snapshot.Settings
	return fmt.Sprintf("%.4f/%.4f/%.4f/%.4f/%s/%d/%s/%d", viewer.prefs.TX.Latitude, viewer.prefs.TX.Longitude, viewer.prefs.RX.Latitude, viewer.prefs.RX.Longitude, settings.Mode, settings.PowerW, settings.Noise, settings.SSN)
}

func (viewer *voacapMap) drawWorld(bounds rl.Rectangle) {
	drawGeoMap(viewer.lat, viewer.lon, viewer.span, bounds)
	rl.BeginScissorMode(int32(bounds.X), int32(bounds.Y), int32(bounds.Width), int32(bounds.Height))
	viewer.drawCoverage(bounds)
	viewer.drawRoute(bounds)
	viewer.drawRoutePin(bounds, viewer.prefs.TX, "TX", colors.cyan)
	viewer.drawRoutePin(bounds, viewer.prefs.RX, "RX", rl.Color{R: 235, G: 70, B: 210, A: 255})
	if viewer.dragPin == 0 {
		viewer.drawCursorReliability(bounds)
	}
	rl.EndScissorMode()
	drawMapGrid(viewer.lat, viewer.lon, viewer.span, bounds)
	rl.DrawRectangleLinesEx(bounds, 2, colors.border)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("ZOOM %.1fx · ARRASTRA TX/RX PARA RECALCULAR · RUEDA PARA ZOOM"), 360/viewer.span), bounds.X+12, bounds.Y+12, 10, colors.text)
	if len(viewer.snapshot.Prediction.Cells) > 0 {
		return
	}
	warning := rl.Rectangle{X: bounds.X + 190, Y: bounds.Y + bounds.Height/2 - 38, Width: 710, Height: 76}
	background := colors.panel
	background.A = 238
	rl.DrawRectangleRounded(warning, .12, 7, background)
	rl.DrawRectangleRoundedLinesEx(warning, .12, 7, 1.5, colors.orange)
	drawCentered(viewer.snapshot.Engine, rl.Rectangle{X: warning.X, Y: warning.Y + 7, Width: warning.Width, Height: 26}, 17, colors.orange)
	drawCentered("El motor local está preparando la capa de cobertura.", rl.Rectangle{X: warning.X, Y: warning.Y + 39, Width: warning.Width, Height: 22}, 12, colors.muted)
}

func (viewer *voacapMap) drawRoute(bounds rl.Rectangle) {
	points := greatCirclePoints(viewer.prefs.TX, viewer.prefs.RX, 72)
	var previous rl.Vector2
	for index, location := range points {
		current := projectMercator(viewer.lat, viewer.lon, viewer.span, location.Latitude, location.Longitude, bounds)
		if index > 0 && float32(math.Abs(float64(current.X-previous.X))) < bounds.Width*.55 {
			shadow := rl.Color{R: 4, G: 12, B: 20, A: 210}
			rl.DrawLineEx(previous, current, 5, shadow)
			rl.DrawLineEx(previous, current, 2.2, colors.cyan)
		}
		previous = current
	}
}

func (viewer *voacapMap) drawRoutePin(bounds rl.Rectangle, location voacap.RouteLocation, label string, color rl.Color) {
	point := projectMercator(viewer.lat, viewer.lon, viewer.span, location.Latitude, location.Longitude, bounds)
	rl.DrawTriangle(rl.Vector2{X: point.X, Y: point.Y + 17}, rl.Vector2{X: point.X - 8, Y: point.Y + 3}, rl.Vector2{X: point.X + 8, Y: point.Y + 3}, color)
	rl.DrawCircleV(point, 11, mapCalloutEdge)
	rl.DrawCircleV(point, 8, color)
	rl.DrawCircleV(point, 3, colors.panel)
	drawMapCallout(label, point.X+13, point.Y-10)
}

func greatCirclePoints(from, to voacap.RouteLocation, segments int) []voacap.RouteLocation {
	lat1, lon1 := from.Latitude*math.Pi/180, from.Longitude*math.Pi/180
	lat2, lon2 := to.Latitude*math.Pi/180, to.Longitude*math.Pi/180
	a := []float64{math.Cos(lat1) * math.Cos(lon1), math.Cos(lat1) * math.Sin(lon1), math.Sin(lat1)}
	b := []float64{math.Cos(lat2) * math.Cos(lon2), math.Cos(lat2) * math.Sin(lon2), math.Sin(lat2)}
	omega := math.Acos(min(max(a[0]*b[0]+a[1]*b[1]+a[2]*b[2], -1.0), 1.0))
	result := make([]voacap.RouteLocation, 0, segments+1)
	for index := 0; index <= segments; index++ {
		t := float64(index) / float64(segments)
		if omega < 1e-8 {
			result = append(result, from)
			continue
		}
		sa, sb := math.Sin((1-t)*omega)/math.Sin(omega), math.Sin(t*omega)/math.Sin(omega)
		x, y, z := sa*a[0]+sb*b[0], sa*a[1]+sb*b[1], sa*a[2]+sb*b[2]
		result = append(result, voacap.RouteLocation{Latitude: math.Atan2(z, math.Hypot(x, y)) * 180 / math.Pi, Longitude: math.Atan2(y, x) * 180 / math.Pi})
	}
	return result
}

func (viewer *voacapMap) scheduleRoute() {
	viewer.routeID++
	if viewer.routeCancel != nil {
		viewer.routeCancel()
		viewer.routeCancel = nil
	}
	viewer.routeAt = time.Now().Add(350 * time.Millisecond)
	viewer.route = voacap.RoutePrediction{}
}

func (viewer *voacapMap) updateRoute() {
	select {
	case result := <-viewer.routeResults:
		viewer.routeBusy = false
		viewer.routeCancel = nil
		if result.id == viewer.routeID {
			viewer.route = result.prediction
		}
	default:
	}
	if viewer.viewSet && !viewer.routeBusy && !viewer.routeAt.IsZero() && time.Now().After(viewer.routeAt) {
		viewer.routeAt = time.Time{}
		viewer.routeBusy = true
		id, settings, tx, rx := viewer.routeID, viewer.snapshot.Settings, viewer.prefs.TX, viewer.prefs.RX
		ctx, cancel := context.WithCancel(context.Background())
		viewer.routeCancel = cancel
		go func() {
			prediction := voacap.CalculateRoute(ctx, resources.Path("tools", "voacap", "runtime"), settings, tx, rx)
			viewer.routeResults <- routeCalculationResult{id: id, prediction: prediction}
		}()
	}
}

func (viewer *voacapMap) drawCursorReliability(bounds rl.Rectangle) {
	mouse := simpleui.MousePosition()
	if !rl.CheckCollisionPointRec(mouse, bounds) || len(viewer.snapshot.Prediction.Cells) == 0 {
		return
	}
	lat, lon := unprojectMercator(viewer.lat, viewer.lon, viewer.span, mouse, bounds)
	reliability, ok := viewer.reliabilityAt(lat, lon)
	if !ok {
		return
	}

	assessment := voacap.AssessLiveConditions(lat, lon, time.Now().UTC(), viewer.snapshot.Solar)
	label := fmt.Sprintf("VOACAP %.0f%%", reliability)
	detail := i18n.Source("CONFIANZA") + " " + i18n.Source(assessment.Confidence)
	reason := i18n.Source(assessment.Reason)
	box := rl.Rectangle{X: mouse.X + 16, Y: mouse.Y - 86, Width: 220, Height: 78}
	if box.X+box.Width > bounds.X+bounds.Width-5 {
		box.X = mouse.X - box.Width - 16
	}
	if box.Y < bounds.Y+5 {
		box.Y = mouse.Y + 16
	}
	if box.Y+box.Height > bounds.Y+bounds.Height-5 {
		box.Y = bounds.Y + bounds.Height - box.Height - 5
	}
	fill := colors.panel
	fill.A = 238
	rl.DrawRectangleRounded(box, .22, 5, fill)
	rl.DrawRectangleRoundedLinesEx(box, .18, 5, 1.5, confidenceColor(assessment.Confidence))
	drawCentered(label, rl.Rectangle{X: box.X, Y: box.Y + 5, Width: box.Width, Height: 24}, 15, colors.text)
	drawCentered(detail, rl.Rectangle{X: box.X, Y: box.Y + 31, Width: box.Width, Height: 20}, 11, confidenceColor(assessment.Confidence))
	drawCentered(reason, rl.Rectangle{X: box.X, Y: box.Y + 53, Width: box.Width, Height: 17}, 9, colors.muted)
}

func confidenceColor(confidence string) rl.Color {
	if confidence == "BAJA" {
		return colors.red
	}
	if confidence == "MEDIA" {
		return colors.orange
	}
	return colors.green
}

func (viewer *voacapMap) reliabilityAt(lat, lon float64) (float64, bool) {
	type nearbyCell struct {
		distance float64
		value    float64
	}
	nearest := make([]nearbyCell, 0, 4)
	for _, cell := range viewer.snapshot.Prediction.Cells {
		value, active := viewer.cellReliability(cell)
		if !active {
			continue
		}
		dLon := math.Abs(cell.Longitude - lon)
		dLon = math.Min(dLon, 360-dLon) * math.Cos(lat*math.Pi/180)
		dLat := cell.Latitude - lat
		distance := dLat*dLat + dLon*dLon
		if distance < 1e-9 {
			return value, true
		}
		candidate := nearbyCell{distance: distance, value: value}
		index := 0
		for index < len(nearest) && nearest[index].distance < distance {
			index++
		}
		nearest = append(nearest, nearbyCell{})
		copy(nearest[index+1:], nearest[index:])
		nearest[index] = candidate
		if len(nearest) > 4 {
			nearest = nearest[:4]
		}
	}
	if len(nearest) == 0 {
		return 0, false
	}
	weighted, totalWeight := 0.0, 0.0
	for _, cell := range nearest {
		weight := 1 / cell.distance
		weighted += cell.value * weight
		totalWeight += weight
	}
	return weighted / totalWeight, true
}

func (viewer *voacapMap) cellReliability(cell voacap.Cell) (float64, bool) {
	reliability, active := 0.0, false
	for band, value := range cell.Reliability {
		if viewer.snapshot.Settings.Bands[band] {
			active = true
			if value > reliability {
				reliability = value
			}
		}
	}
	return reliability, active
}

func (viewer *voacapMap) drawCoverage(bounds rl.Rectangle) {
	if viewer.prefs.LayerAlpha <= 0 {
		return
	}
	for _, cell := range viewer.snapshot.Prediction.Cells {
		reliability, active := viewer.cellReliability(cell)
		if !active {
			continue
		}
		if reliability < 20 {
			continue
		}
		center := projectMercator(viewer.lat, viewer.lon, viewer.span, cell.Latitude, cell.Longitude, bounds)
		top := projectMercator(viewer.lat, viewer.lon, viewer.span, cell.Latitude+4.5, cell.Longitude, bounds)
		bottom := projectMercator(viewer.lat, viewer.lon, viewer.span, cell.Latitude-4.5, cell.Longitude, bounds)
		cellWidth := float32(18 / viewer.span * float64(bounds.Width))
		cellHeight := max(float32(3), bottom.Y-top.Y)
		color := reliabilityColor(reliability)
		strength := float32((reliability-15)/85) * viewer.prefs.LayerAlpha / 100
		color.A = uint8(min(max(strength*185, float32(0)), float32(185)))
		outer := color
		outer.A = 0
		rl.DrawCircleGradient(center, max(cellWidth, cellHeight)*.72, color, outer)
	}
}

func reliabilityColor(value float64) rl.Color {
	if value >= 80 {
		return rl.Color{R: 238, G: 91, B: 38, A: 255}
	}
	if value >= 60 {
		return rl.Color{R: 247, G: 196, B: 42, A: 255}
	}
	if value >= 40 {
		return rl.Color{R: 79, G: 207, B: 118, A: 255}
	}
	return rl.Color{R: 18, G: 170, B: 205, A: 255}
}

func (viewer *voacapMap) drawRoutePanel(bounds rl.Rectangle) {
	x := bounds.X + 14
	cardWidth := bounds.Width - 28
	drawPanel(x, bounds.Y+10, cardWidth, 142)
	simpleui.DrawTextStyled("RUTA TX → RX", x+12, bounds.Y+20, 16, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled("TX", x+12, bounds.Y+52, 12, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(fmt.Sprintf("%.4f°, %.4f°", viewer.prefs.TX.Latitude, viewer.prefs.TX.Longitude), x+48, bounds.Y+52, 11, simpleui.FontMono, colors.text)
	rxColor := rl.Color{R: 235, G: 70, B: 210, A: 255}
	simpleui.DrawTextStyled("RX", x+12, bounds.Y+79, 12, simpleui.FontSemiBold, rxColor)
	simpleui.DrawTextStyled(fmt.Sprintf("%.4f°, %.4f°", viewer.prefs.RX.Latitude, viewer.prefs.RX.Longitude), x+48, bounds.Y+79, 11, simpleui.FontMono, colors.text)
	distance, azimuth := viewer.route.DistanceKM, viewer.route.Azimuth
	if distance == 0 {
		distance, azimuth = routeDisplayGeometry(viewer.prefs.TX, viewer.prefs.RX)
	}
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("DISTANCIA  %.0f km"), distance), x+12, bounds.Y+112, 11, simpleui.FontSemiBold, colors.text)
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("AZIMUT  %.0f°"), azimuth), x+210, bounds.Y+112, 11, simpleui.FontSemiBold, colors.text)
	band := viewer.selectedBand()
	simpleui.DrawText(fmt.Sprintf("%s · %.3f MHz · %d W · %s", band, routeBandFrequency(band), viewer.snapshot.Settings.PowerW, viewer.snapshot.Settings.Mode), x+12, bounds.Y+137, 10, colors.muted)

	simpleui.DrawTextStyled("PROPAGACIÓN · PROBABILIDAD DE QSO", x+8, bounds.Y+169, 13, simpleui.FontSemiBold, colors.cyan)
	viewer.drawRouteWheel(rl.Vector2{X: bounds.X + bounds.Width/2, Y: bounds.Y + 390}, 164)
	viewer.drawRouteLegend(rl.Rectangle{X: x + 6, Y: bounds.Y + 579, Width: cardWidth - 12, Height: 18})
	viewer.drawRouteSummary(rl.Rectangle{X: x, Y: bounds.Y + 625, Width: cardWidth, Height: 82}, band)
	simpleui.DrawTextStyled(fmt.Sprintf("INTENSIDAD DE CAPA   %.0f%%", viewer.prefs.LayerAlpha), x+12, bounds.Y+714, 11, simpleui.FontSemiBold, colors.cyan)
}

func (viewer *voacapMap) selectedBand() string {
	for _, band := range voacap.AmateurBands {
		if viewer.snapshot.Settings.Bands[band] {
			return band
		}
	}
	return "20 m"
}

func (viewer *voacapMap) drawRouteWheel(center rl.Vector2, radius float32) {
	inner, ringWidth := float32(44), (radius-44)/float32(len(voacap.AmateurBands))
	for bandIndex, band := range voacap.AmateurBands {
		values := viewer.route.Reliability[band]
		for hour := 0; hour < 24; hour++ {
			value := 0.0
			if len(values) == 24 {
				value = values[hour]
			}
			color := routeWheelColor(value)
			start := float32(hour*15 - 90)
			rl.DrawRing(center, inner+float32(bandIndex)*ringWidth, inner+float32(bandIndex+1)*ringWidth-.7, start+.4, start+14.6, 4, color)
		}
	}
	for hour := 0; hour < 24; hour++ {
		angle := (float64(hour*15-90) + 7.5) * math.Pi / 180
		labelRadius := float64(radius + 13)
		label := fmt.Sprintf("%d", hour)
		size := simpleui.MeasureTextStyled(label, 9, simpleui.FontSemiBold)
		simpleui.DrawTextStyled(label, center.X+float32(math.Cos(angle)*labelRadius)-size.X/2, center.Y+float32(math.Sin(angle)*labelRadius)-size.Y/2, 9, simpleui.FontSemiBold, colors.text)
	}
	for index, band := range voacap.AmateurBands {
		r := inner + (float32(index)+.5)*ringWidth
		labelSize := simpleui.MeasureTextStyled(band, 8, simpleui.FontSemiBold)
		plate := rl.Rectangle{X: center.X - labelSize.X/2 - 4, Y: center.Y - r - labelSize.Y/2, Width: labelSize.X + 8, Height: labelSize.Y}
		plateColor := colors.panel
		plateColor.A = 225
		rl.DrawRectangleRounded(plate, .25, 4, plateColor)
		simpleui.DrawTextStyled(band, center.X-labelSize.X/2, plate.Y, 8, simpleui.FontSemiBold, colors.text)
	}
	now := time.Now().UTC().Hour()
	current := 0.0
	if values := viewer.route.Reliability[viewer.selectedBand()]; len(values) == 24 {
		current = values[now]
	}
	rl.DrawCircleV(center, inner-1, colors.panel)
	drawCentered("AHORA", rl.Rectangle{X: center.X - inner, Y: center.Y - 28, Width: inner * 2, Height: 14}, 8, colors.muted)
	drawCentered(fmt.Sprintf("%02d UTC", now), rl.Rectangle{X: center.X - inner, Y: center.Y - 11, Width: inner * 2, Height: 21}, 13, colors.text)
	drawCentered(viewer.selectedBand(), rl.Rectangle{X: center.X - inner, Y: center.Y + 11, Width: inner * 2, Height: 16}, 9, colors.muted)
	drawCentered(fmt.Sprintf("%.0f%%", current), rl.Rectangle{X: center.X - inner, Y: center.Y + 27, Width: inner * 2, Height: 18}, 13, reliabilityColor(current))
	if viewer.routeBusy || len(viewer.route.Reliability) == 0 {
		fill := colors.panel
		fill.A = 225
		rl.DrawCircleV(center, radius+2, fill)
		status := "CALCULANDO RUTA…"
		if viewer.route.Error != "" {
			status = viewer.route.Error
		}
		drawCentered(i18n.Source(status), rl.Rectangle{X: center.X - radius, Y: center.Y - 12, Width: radius * 2, Height: 24}, 11, colors.orange)
	}
}

func routeWheelColor(value float64) rl.Color {
	if value < 1 {
		return rl.Color{R: 31, G: 42, B: 54, A: 255}
	}
	return reliabilityColor(value)
}

func (viewer *voacapMap) drawRouteLegend(bounds rl.Rectangle) {
	steps := 80
	for index := 0; index < steps; index++ {
		value := float64(index) / float64(steps-1) * 100
		x := bounds.X + bounds.Width*float32(index)/float32(steps)
		rl.DrawRectangle(int32(x), int32(bounds.Y), int32(bounds.Width/float32(steps)+2), int32(bounds.Height), reliabilityColor(value))
	}
	for index := 0; index <= 5; index++ {
		label := fmt.Sprintf("%d%%", index*20)
		labelWidth := simpleui.MeasureText(label, 8).X
		x := bounds.X + bounds.Width*float32(index)/5 - labelWidth/2
		simpleui.DrawText(label, x, bounds.Y+24, 8, colors.muted)
	}
}

func (viewer *voacapMap) drawRouteSummary(bounds rl.Rectangle, band string) {
	drawPanel(bounds.X, bounds.Y, bounds.Width, bounds.Height)
	bestHour, bestValue := 0, 0.0
	values := viewer.route.Reliability[band]
	for hour, value := range values {
		if value > bestValue {
			bestHour, bestValue = hour, value
		}
	}
	endHour := (bestHour + 3) % 24
	muf := 0.0
	if len(viewer.route.MUF) == 24 {
		muf = viewer.route.MUF[time.Now().UTC().Hour()]
	}
	assessment := viewer.assessRouteConditions()
	x := bounds.X + 12
	simpleui.DrawTextStyled("MEJOR VENTANA", x, bounds.Y+8, 9, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(fmt.Sprintf("%02d:00–%02d:00 UTC", bestHour, endHour), x, bounds.Y+28, 13, simpleui.FontSemiBold, colors.text)
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("MÁX %.0f%%"), bestValue), x, bounds.Y+57, 9, simpleui.FontSemiBold, reliabilityColor(bestValue))
	simpleui.DrawTextStyled("MUF", bounds.X+210, bounds.Y+8, 8, simpleui.FontSemiBold, colors.muted)
	simpleui.DrawTextStyled(fmt.Sprintf("%.1f MHz", muf), bounds.X+210, bounds.Y+27, 10, simpleui.FontSemiBold, colors.text)
	simpleui.DrawTextStyled("CONFIANZA", bounds.X+210, bounds.Y+49, 8, simpleui.FontSemiBold, colors.muted)
	simpleui.DrawTextStyled(assessment.Confidence, bounds.X+285, bounds.Y+49, 9, simpleui.FontSemiBold, confidenceColor(assessment.Confidence))
	simpleui.DrawText(fmt.Sprintf("R%d · S%d · G%d", viewer.snapshot.Solar.RScale, viewer.snapshot.Solar.SScale, viewer.snapshot.Solar.GScale), bounds.X+210, bounds.Y+67, 8, colors.muted)
}

func (viewer *voacapMap) assessRouteConditions() voacap.LiveAssessment {
	worst := voacap.LiveAssessment{Confidence: "ALTA", Reason: "CONDICIONES NORMALES"}
	severity := map[string]int{"ALTA": 0, "MEDIA": 1, "BAJA": 2}
	for _, point := range greatCirclePoints(viewer.prefs.TX, viewer.prefs.RX, 24) {
		assessment := voacap.AssessLiveConditions(point.Latitude, point.Longitude, time.Now().UTC(), viewer.snapshot.Solar)
		if severity[assessment.Confidence] > severity[worst.Confidence] {
			worst = assessment
		}
	}
	return worst
}

func routeBandFrequency(band string) float64 {
	frequencies := map[string]float64{"80 m": 3.6, "60 m": 5.357, "40 m": 7.1, "30 m": 10.12, "20 m": 14.1, "17 m": 18.1, "15 m": 21.1, "12 m": 24.93, "10 m": 28.4}
	return frequencies[band]
}

func routeDisplayGeometry(tx, rx voacap.RouteLocation) (float64, float64) {
	lat1, lat2 := tx.Latitude*math.Pi/180, rx.Latitude*math.Pi/180
	dLon := (rx.Longitude - tx.Longitude) * math.Pi / 180
	a := math.Sin((lat2-lat1)/2)*math.Sin((lat2-lat1)/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	distance := 6371.0088 * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	return distance, math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}
