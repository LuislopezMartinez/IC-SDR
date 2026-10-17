package screens

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go-zero/internal/dxcc"
	"go-zero/internal/ft8"
	"go-zero/internal/localtime"
	"go-zero/internal/resources"
	"go-zero/simpleui"

	"github.com/biter777/countries"
	rl "github.com/gen2brain/raylib-go/raylib"
)

//go:embed assets/flags/atlas.png
var ft8FlagAtlasPNG []byte

//go:embed assets/flags/dxcc-flags.json
var ft8DXCCFlagsJSON []byte

var (
	ft8MapModal     = rl.Rectangle{X: 7, Y: 5, Width: 1586, Height: 890}
	ft8MapBounds    = rl.Rectangle{X: 32, Y: 142, Width: 1080, Height: 718}
	ft8MapMax       = rl.Rectangle{X: 1466, Y: 22, Width: 48, Height: 48}
	ft8MapClose     = rl.Rectangle{X: 1524, Y: 22, Width: 48, Height: 48}
	ft8MapBack      = rl.Rectangle{X: 1390, Y: 586, Width: 164, Height: 26}
	ft8RegionClear  = rl.Rectangle{X: 1448, Y: 299, Width: 108, Height: 28}
	ft8DXCCOnce     sync.Once
	ft8DXCCData     *dxcc.Database
	ft8FlagOnce     sync.Once
	ft8FlagAtlas    rl.Texture2D
	ft8FlagIndex    map[string]int
	ft8DXCCFlagOnce sync.Once
	ft8DXCCFlags    map[string]string
)

type ft8MapPoint struct {
	decode      ft8.Decode
	call, grid  string
	lat, lon    float64
	hasPosition bool
}

type ft8MapStation struct {
	call, grid   string
	lat, lon     float64
	hasPosition  bool
	latest       ft8.Decode
	messageCount int
	selected     bool
}

type ft8ActivityEvent struct {
	decode                ft8.Decode
	from, to, kind, phase string
}

type ft8QSOStats struct {
	total, active, completed int
}

type ft8QSOConversation struct {
	first, last  time.Time
	callA, callB string
	phase        string
	messages     int
}

const (
	ft8QSOReply       = "reply"
	ft8QSOReport      = "report"
	ft8QSORogerReport = "roger_report"
	ft8QSOConfirmed   = "confirmed"
	ft8QSOComplete    = "complete"
)

func (p *FT8Panel) openMap(index int) {
	if index < 0 || index >= len(p.status.Decodes) {
		return
	}
	p.openExternalViewer(index)
}

func (p *FT8Panel) refreshOpenMap() {
	selectedKey := ""
	if p.mapSelected >= 0 && p.mapSelected < len(p.mapDecodes) {
		selectedKey = ft8DecodeKey(p.mapDecodes[p.mapSelected])
	}
	p.mapDecodes = append(p.mapDecodes[:0], p.status.Decodes...)
	p.mapSelected = -1
	for index, decode := range p.mapDecodes {
		if ft8DecodeKey(decode) == selectedKey {
			p.mapSelected = index
			break
		}
	}
}

func ft8DecodeKey(decode ft8.Decode) string {
	return fmt.Sprintf("%d|%s|%.1f", decode.Slot.Unix(), decode.Message, decode.Frequency)
}

func (p *FT8Panel) focusMapSelection() {
	if p.mapSelected < 0 || p.mapSelected >= len(p.mapDecodes) {
		p.mapLat, p.mapLon, p.mapSpan = 15, 0, 300
		return
	}
	// A global view guarantees that every station remains visible. Selection
	// is expressed through marker emphasis rather than hiding distant context.
	p.mapLat, p.mapLon, p.mapSpan = 10, 0, 360
}

func (p *FT8Panel) UpdateOverlay(input simpleui.Input) bool {
	if !p.mapOpen {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		p.mapOpen = false
		return true
	}
	if !p.viewerMode && input.Pressed && input.Over(ft8MapClose) {
		p.mapOpen = false
		if p.viewerMode {
			simpleui.RequestClose()
		}
		return true
	}
	if !p.viewerMode && input.Pressed && input.Over(ft8MapMax) {
		if rl.IsWindowMaximized() {
			rl.RestoreWindow()
		} else {
			rl.MaximizeWindow()
		}
		return true
	}
	if input.Pressed && input.Over(ft8MapBack) {
		p.mapSelected = -1
		return true
	}
	regionBounds := rl.Rectangle{X: 1140, Y: 304, Width: 416, Height: 268}
	if input.Over(regionBounds) && input.Wheel != 0 {
		regionCount := len(p.ft8RegionActivity(p.ft8MapStations()))
		p.mapRegionOffset = min(max(0, p.mapRegionOffset-int(input.Wheel)*2), max(0, regionCount-8))
		return true
	}
	if input.Pressed && p.mapRegionFilter != "" && input.Over(ft8RegionClear) {
		p.mapRegionFilter = ""
		p.mapSelected = -1
		return true
	}
	if input.Pressed && input.Pointer.X >= 1140 && input.Pointer.X <= 1556 && input.Pointer.Y >= 359 && input.Pointer.Y < 559 {
		regions := p.stableFT8RegionActivity(p.ft8MapStations(), time.Now())
		row := int((input.Pointer.Y - 359) / 25)
		index := p.mapRegionOffset + row
		if row >= 0 && row < 8 && index >= 0 && index < len(regions) {
			if p.mapRegionFilter == regions[index].name {
				p.mapRegionFilter = ""
			} else {
				p.mapRegionFilter = regions[index].name
			}
			p.mapSelected = -1
			return true
		}
	}
	if input.Pressed && input.Pointer.Y >= 91 && input.Pointer.Y <= 125 {
		switch {
		case input.Pointer.X >= 32 && input.Pointer.X <= 112:
			p.mapWindow = 5 * time.Minute
		case input.Pointer.X >= 120 && input.Pointer.X <= 205:
			p.mapWindow = 15 * time.Minute
		case input.Pointer.X >= 213 && input.Pointer.X <= 288:
			p.mapWindow = time.Hour
		case input.Pointer.X >= 296 && input.Pointer.X <= 390:
			p.mapWindow = 0
		case input.Pointer.X >= 438 && input.Pointer.X <= 520:
			p.mapFilter = "all"
		case input.Pointer.X >= 528 && input.Pointer.X <= 598:
			p.mapFilter = "cq"
		case input.Pointer.X >= 606 && input.Pointer.X <= 710:
			p.mapFilter = "calls"
		case input.Pointer.X >= 718 && input.Pointer.X <= 916:
			p.mapOnlyPosition = !p.mapOnlyPosition
		}
		p.mapNextRegionRank = time.Time{}
		return true
	}
	inside := input.Over(ft8MapBounds)
	if inside && input.Wheel != 0 {
		p.mapLat, p.mapLon, p.mapSpan = zoomMap(p.mapLat, p.mapLon, p.mapSpan, input.Pointer, ft8MapBounds, input.Wheel)
	}
	if inside && input.Pressed {
		p.mapDragging = true
		p.mapDragLast = input.Pointer
	}
	if p.mapDragging && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		delta := rl.Vector2Subtract(input.Pointer, p.mapDragLast)
		p.mapLat, p.mapLon = panMap(p.mapLat, p.mapLon, p.mapSpan, delta, ft8MapBounds)
		p.mapLat, p.mapLon, p.mapSpan = clampMapView(p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds)
		p.mapDragLast = input.Pointer
	}
	if p.mapDragging && input.Released {
		p.mapDragging = false
		p.selectFT8MapMarker(input.Pointer)
	}
	return true
}

func (p *FT8Panel) selectFT8MapMarker(pointer rl.Vector2) {
	best, bestDistance := -1, float32(18)
	for _, station := range p.visibleFT8MapStations() {
		if !station.hasPosition {
			continue
		}
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, station.lat, station.lon, ft8MapBounds)
		distance := rl.Vector2Distance(pointer, position)
		if distance < bestDistance {
			for index, decode := range p.mapDecodes {
				if ft8MessageCall(decode.Message) == station.call {
					best, bestDistance = index, distance
					break
				}
			}
		}
	}
	if best >= 0 {
		p.mapSelected = best
	}
}

func (p *FT8Panel) DrawOverlay() {
	if !p.mapOpen {
		return
	}
	modal := ft8MapModal
	if p.viewerMode {
		modal = rl.Rectangle{Width: designWidth, Height: designHeight}
		rl.DrawRectangleRec(modal, colors.panel)
	} else {
		rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{R: 0, G: 0, B: 0, A: 218})
		rl.DrawRectangleRounded(modal, .022, 10, colors.panel)
		rl.DrawRectangleRoundedLinesEx(modal, .022, 10, 2, colors.cyan)
	}
	simpleui.DrawTextStyled("ACTIVIDAD FT8 GLOBAL", 32, 25, 23, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("Actividad recibida en esta estación · posición estimada mediante locator Maidenhead", 32, 57, uiCaptionFontSize, colors.muted)
	clockText, clockColor := ft8ClockPresentation(p.status)
	simpleui.DrawTextStyled("●  EN VIVO", 1175, 38, uiCaptionFontSize, simpleui.FontSemiBold, colors.green)
	simpleui.DrawText(clockText, 1270, 38, 10, clockColor)
	if !p.viewerMode {
		drawFT8WindowButton(ft8MapMax, "maximize", rl.IsWindowMaximized())
		drawFT8WindowButton(ft8MapClose, "close", false)
	}
	p.drawFT8MapFilters()

	drawGeoMap(p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds)
	drawMapGrid(p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds)
	rl.DrawRectangleLinesEx(ft8MapBounds, 1, colors.border)
	p.drawFT8MapStations()
	p.drawFT8Dashboard()
}

func drawFT8WindowButton(bounds rl.Rectangle, kind string, restored bool) {
	background := colors.panelAlt
	rl.DrawRectangleRounded(bounds, .09, 6, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .09, 6, 2, colors.border)
	line := simpleui.EnsureTextContrast(colors.text, background)
	if kind == "close" {
		rl.DrawLineEx(rl.Vector2{X: bounds.X + 16, Y: bounds.Y + 16}, rl.Vector2{X: bounds.X + 32, Y: bounds.Y + 32}, 2, line)
		rl.DrawLineEx(rl.Vector2{X: bounds.X + 32, Y: bounds.Y + 16}, rl.Vector2{X: bounds.X + 16, Y: bounds.Y + 32}, 2, line)
		return
	}
	if restored {
		rl.DrawRectangleLinesEx(rl.Rectangle{X: bounds.X + 19, Y: bounds.Y + 15, Width: 15, Height: 14}, 2, line)
		rl.DrawRectangleLinesEx(rl.Rectangle{X: bounds.X + 14, Y: bounds.Y + 20, Width: 15, Height: 14}, 2, line)
	} else {
		rl.DrawRectangleLinesEx(rl.Rectangle{X: bounds.X + 15, Y: bounds.Y + 15, Width: 18, Height: 18}, 2, line)
	}
}

func (p *FT8Panel) drawFT8MapFilters() {
	items := []struct {
		bounds rl.Rectangle
		label  string
		active bool
	}{
		{rl.Rectangle{X: 32, Y: 91, Width: 80, Height: 34}, "5 MIN", p.mapWindow == 5*time.Minute},
		{rl.Rectangle{X: 120, Y: 91, Width: 85, Height: 34}, "15 MIN", p.mapWindow == 15*time.Minute},
		{rl.Rectangle{X: 213, Y: 91, Width: 75, Height: 34}, "1 H", p.mapWindow == time.Hour},
		{rl.Rectangle{X: 296, Y: 91, Width: 94, Height: 34}, "SESIÓN", p.mapWindow == 0},
	}
	for _, item := range items {
		drawModalAction(item.bounds, item.label, map[bool]rl.Color{true: colors.cyan, false: colors.border}[item.active], item.active)
	}
	filters := []struct {
		bounds rl.Rectangle
		label  string
		active bool
	}{
		{rl.Rectangle{X: 438, Y: 91, Width: 82, Height: 34}, "TODOS", p.mapFilter == "all"},
		{rl.Rectangle{X: 528, Y: 91, Width: 70, Height: 34}, "CQ", p.mapFilter == "cq"},
		{rl.Rectangle{X: 606, Y: 91, Width: 104, Height: 34}, "LLAMADAS", p.mapFilter == "calls"},
		{rl.Rectangle{X: 718, Y: 91, Width: 198, Height: 34}, "SOLO CON POSICIÓN", p.mapOnlyPosition},
	}
	for _, item := range filters {
		drawModalAction(item.bounds, item.label, map[bool]rl.Color{true: colors.cyan, false: colors.border}[item.active], item.active)
	}
}

func (p *FT8Panel) drawFT8MapStations() {
	stations := p.visibleFT8MapStations()
	p.drawFT8LiveEvents()
	selected := ft8MapStation{}
	for _, station := range stations {
		if station.selected {
			selected = station
			break
		}
	}
	homeLat, homeLon := p.ft8HomePosition()
	if selected.hasPosition {
		drawFT8Route(homeLat, homeLon, selected.lat, selected.lon, p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds, rl.Color{R: colors.cyan.R, G: colors.cyan.G, B: colors.cyan.B, A: 210})
	}

	// Draw context first with subdued markers, then the selected station on top.
	for _, station := range stations {
		if station.selected {
			continue
		}
		if !station.hasPosition {
			continue
		}
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, station.lat, station.lon, ft8MapBounds)
		if !rl.CheckCollisionPointRec(position, ft8MapBounds) {
			continue
		}
		drawFT8StationMarker(position, ft8MarkerColor(station.latest), false)
	}
	home := projectMercator(p.mapLat, p.mapLon, p.mapSpan, homeLat, homeLon, ft8MapBounds)
	if rl.CheckCollisionPointRec(home, ft8MapBounds) {
		rl.DrawCircleV(home, 7, colors.cyan)
		rl.DrawCircleLines(int32(home.X), int32(home.Y), 11, colors.text)
		rl.DrawCircleLines(int32(home.X), int32(home.Y), 15, colors.cyan)
	}
	if selected.hasPosition {
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, selected.lat, selected.lon, ft8MapBounds)
		if rl.CheckCollisionPointRec(position, ft8MapBounds) {
			color := ft8MarkerColor(selected.latest)
			drawFT8StationMarker(position, color, true)
			drawMapCallout(selected.call+" · "+selected.grid, position.X+17, position.Y-8)
		}
	}
}

func ft8MarkerColor(decode ft8.Decode) rl.Color {
	event, ok := classifyFT8Activity(decode)
	if ok {
		if event.kind == "cq" {
			return colors.orange
		}
		if event.kind == "complete" {
			return colors.green
		}
	}
	return colors.cyan
}

func drawFT8StationMarker(position rl.Vector2, color rl.Color, selected bool) {
	halo := color
	halo.A = 48
	radius := float32(10)
	if selected {
		radius = 17
	}
	rl.DrawCircleV(position, radius, halo)
	rl.DrawCircleV(position, 7, colors.panel)
	rl.DrawCircleV(position, 5, color)
	rl.DrawCircleLines(int32(position.X), int32(position.Y), 8, colors.text)
	if selected {
		rl.DrawCircleLines(int32(position.X), int32(position.Y), 14, color)
		rl.DrawCircleLines(int32(position.X), int32(position.Y), 19, colors.text)
	}
}

func (p *FT8Panel) drawFT8LiveEvents() {
	locations := ft8StationLocations(p.mapDecodes)
	decodes := p.filterFT8DecodesByRegion(p.mapDecodes)
	now := time.Now()
	drawn := 0
	for _, decode := range decodes {
		event, ok := classifyFT8Activity(decode)
		if !ok || decode.ReceivedAt.IsZero() {
			continue
		}
		age := now.Sub(decode.ReceivedAt)
		if age < 0 || age > 5*time.Second {
			continue
		}
		progress := float32(age) / float32(5*time.Second)
		alpha := uint8(220 * (1 - progress))
		from, fromOK := locations[event.from]
		if event.kind == "cq" && fromOK {
			position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, from.lat, from.lon, ft8MapBounds)
			if rl.CheckCollisionPointRec(position, ft8MapBounds) {
				radius := float32(10) + progress*32
				rl.DrawCircleLines(int32(position.X), int32(position.Y), radius, rl.Color{R: colors.orange.R, G: colors.orange.G, B: colors.orange.B, A: alpha})
				rl.DrawCircleV(position, 5, colors.orange)
			}
			drawn++
			continue
		}
		to, toOK := locations[event.to]
		if !fromOK || !toOK {
			continue
		}
		color := ft8EventColor(event.kind)
		color.A = alpha
		drawFT8Route(from.lat, from.lon, to.lat, to.lon, p.mapLat, p.mapLon, p.mapSpan, ft8MapBounds, color)
		pulse := ft8RoutePosition(from.lat, from.lon, to.lat, to.lon, float64(math.Mod(float64(progress)*1.7, 1)))
		position := projectMercator(p.mapLat, p.mapLon, p.mapSpan, pulse.lat, pulse.lon, ft8MapBounds)
		if rl.CheckCollisionPointRec(position, ft8MapBounds) {
			rl.DrawCircleV(position, 7, color)
			rl.DrawCircleLines(int32(position.X), int32(position.Y), 12, rl.Color{R: color.R, G: color.G, B: color.B, A: alpha / 2})
		}
		drawn++
		if drawn >= 12 {
			break
		}
	}
}

type ft8Location struct{ lat, lon float64 }

func ft8StationLocations(decodes []ft8.Decode) map[string]ft8Location {
	result := make(map[string]ft8Location)
	for index := len(decodes) - 1; index >= 0; index-- {
		call, grid := ft8MessageStation(decodes[index].Message)
		if lat, lon, ok := maidenheadCenter(grid); ok && call != "" {
			result[call] = ft8Location{lat: lat, lon: lon}
		}
	}
	return result
}

func ft8RoutePosition(lat1, lon1, lat2, lon2, progress float64) ft8Location {
	return ft8Location{lat: lat1 + (lat2-lat1)*progress, lon: wrapLongitude(lon1 + wrapLongitude(lon2-lon1)*progress)}
}

func ft8EventColor(kind string) rl.Color {
	switch kind {
	case ft8QSOReport, ft8QSORogerReport:
		return colors.orange
	case ft8QSOConfirmed, ft8QSOComplete:
		return colors.green
	default:
		return colors.cyan
	}
}

func classifyFT8Activity(decode ft8.Decode) (ft8ActivityEvent, bool) {
	fields := strings.Fields(strings.ToUpper(strings.TrimSpace(decode.Message)))
	if len(fields) < 2 {
		return ft8ActivityEvent{}, false
	}
	if fields[0] == "CQ" {
		from := ft8MessageCall(decode.Message)
		return ft8ActivityEvent{decode: decode, from: from, kind: "cq"}, from != ""
	}
	from, to := strings.Trim(fields[1], "<>"), strings.Trim(fields[0], "<>")
	if !ft8PlausibleCall(from) || !ft8PlausibleCall(to) {
		return ft8ActivityEvent{}, false
	}
	phase := ft8QSOReply
	for _, token := range fields[2:] {
		token = strings.TrimSpace(strings.ToUpper(token))
		switch {
		case token == "RR73" || token == "73":
			phase = ft8QSOComplete
		case token == "RRR" && ft8QSOPhaseRank(phase) < ft8QSOPhaseRank(ft8QSOConfirmed):
			phase = ft8QSOConfirmed
		case (strings.HasPrefix(token, "R+") || strings.HasPrefix(token, "R-")) && ft8QSOPhaseRank(phase) < ft8QSOPhaseRank(ft8QSORogerReport):
			phase = ft8QSORogerReport
		case (strings.HasPrefix(token, "+") || strings.HasPrefix(token, "-")) && ft8QSOPhaseRank(phase) < ft8QSOPhaseRank(ft8QSOReport):
			phase = ft8QSOReport
		}
	}
	return ft8ActivityEvent{decode: decode, from: from, to: to, kind: phase, phase: phase}, true
}

func ft8QSOPhaseRank(phase string) int {
	switch phase {
	case ft8QSOReport:
		return 2
	case ft8QSORogerReport:
		return 3
	case ft8QSOConfirmed:
		return 4
	case ft8QSOComplete:
		return 5
	default:
		return 1
	}
}

type ft8RegionActivity struct {
	name               string
	stations, messages int
	bestSNR            int
	maxDistance        float64
}

func (p *FT8Panel) activityDecodes() []ft8.Decode {
	cutoff := time.Time{}
	if p.mapWindow > 0 {
		cutoff = time.Now().UTC().Add(-p.mapWindow)
	}
	result := make([]ft8.Decode, 0, len(p.mapDecodes))
	for _, decode := range p.mapDecodes {
		if cutoff.IsZero() || !decode.Slot.Before(cutoff) {
			upper := strings.ToUpper(strings.TrimSpace(decode.Message))
			if p.mapFilter == "cq" && !strings.HasPrefix(upper, "CQ ") {
				continue
			}
			if p.mapFilter == "calls" && strings.HasPrefix(upper, "CQ ") {
				continue
			}
			result = append(result, decode)
		}
	}
	return result
}

func (p *FT8Panel) drawFT8Dashboard() {
	decodes := p.activityDecodes()
	allStations := p.ft8MapStations()
	stations := p.filterFT8StationsByRegion(allStations)
	visibleDecodes := p.filterFT8DecodesByRegion(decodes)
	positioned, maxDistance := 0, 0.0
	homeLat, homeLon := p.ft8HomePosition()
	for _, station := range stations {
		if station.hasPosition {
			positioned++
			maxDistance = math.Max(maxDistance, ft8DistanceKM(homeLat, homeLon, station.lat, station.lon))
		}
	}
	regions := p.stableFT8RegionActivity(allStations, time.Now())
	p.mapRegionOffset = min(p.mapRegionOffset, max(0, len(regions)-8))
	qso := calculateFT8QSOStats(visibleDecodes, time.Now())

	sidebar := rl.Rectangle{X: 1128, Y: 142, Width: 440, Height: 718}
	rl.DrawRectangleRounded(sidebar, .018, 8, colors.background)
	rl.DrawRectangleRoundedLinesEx(sidebar, .018, 8, 1, colors.border)
	metrics := []struct {
		label, value string
		color        rl.Color
	}{
		{"ESTACIONES", fmt.Sprintf("%d", len(stations)), colors.green},
		{"MENSAJES", fmt.Sprintf("%d", len(visibleDecodes)), colors.cyan},
		{"QSO ACTIVOS", fmt.Sprintf("%d / %d", qso.active, qso.total), colors.orange},
		{"DX MÁXIMO", fmt.Sprintf("%.0f km", maxDistance), colors.blue},
	}
	for index, metric := range metrics {
		x := float32(1140 + (index%2)*208)
		y := float32(154 + (index/2)*70)
		box := rl.Rectangle{X: x, Y: y, Width: 196, Height: 60}
		rl.DrawRectangleRounded(box, .06, 6, mixColor(colors.panelAlt, metric.color, .07))
		rl.DrawRectangleRoundedLinesEx(box, .06, 6, 1, mixColor(colors.border, metric.color, .45))
		simpleui.DrawText(metric.label, x+12, y+9, 10, colors.muted)
		simpleui.DrawTextStyled(metric.value, x+12, y+27, 20, simpleui.FontSemiBold, metric.color)
	}
	heading := "PAÍSES Y REGIONES"
	if p.mapRegionFilter != "" {
		heading = "REGIÓN: " + strings.ToUpper(clipFT8(p.mapRegionFilter, 18))
		drawModalAction(ft8RegionClear, "VER TODAS", colors.cyan, false)
	} else {
		simpleui.DrawText(fmt.Sprintf("%d regiones · %d QSO completados", len(regions), qso.completed), 1343, 308, 9, colors.muted)
	}
	simpleui.DrawTextStyled(heading, 1140, 304, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("REGIÓN", 1140, 335, 10, colors.muted)
	drawFT8RightAligned("EST.", 1374, 335, 10, simpleui.FontRegular, colors.muted)
	drawFT8RightAligned("MSG", 1420, 335, 10, simpleui.FontRegular, colors.muted)
	drawFT8RightAligned("SNR", 1470, 335, 10, simpleui.FontRegular, colors.muted)
	drawFT8RightAligned("DIST.", 1544, 335, 10, simpleui.FontRegular, colors.muted)
	rl.DrawLine(1140, 354, 1556, 354, colors.border)
	for row, region := range regions[p.mapRegionOffset:] {
		if row >= 8 {
			break
		}
		y := float32(362 + row*25)
		selectedRegion := region.name == p.mapRegionFilter
		if selectedRegion {
			rl.DrawRectangle(1140, int32(y-3), 416, 24, mixColor(colors.panelAlt, colors.cyan, .18))
			rl.DrawRectangle(1140, int32(y-3), 3, 24, colors.cyan)
		} else if row%2 == 0 {
			rl.DrawRectangle(1140, int32(y-3), 416, 24, mixColor(colors.panelAlt, colors.blue, .04))
		}
		drawFT8RegionFlag(region.name, 1144, y+2)
		simpleui.DrawText(clipFT8(region.name, 16), 1167, y+3, 10, colors.text)
		drawFT8RightAligned(fmt.Sprintf("%d", region.stations), 1374, y+3, 10, simpleui.FontMono, colors.text)
		drawFT8RightAligned(fmt.Sprintf("%d", region.messages), 1420, y+3, 10, simpleui.FontMono, colors.text)
		drawFT8RightAligned(fmt.Sprintf("%+d", region.bestSNR), 1470, y+3, 10, simpleui.FontMono, ft8SNRColor(region.bestSNR))
		drawFT8RightAligned(fmt.Sprintf("%.0f km", region.maxDistance), 1544, y+3, 10, simpleui.FontMono, colors.muted)
	}
	if len(regions) > 8 {
		track := rl.Rectangle{X: 1552, Y: 360, Width: 4, Height: 198}
		rl.DrawRectangleRounded(track, 1, 4, colors.panelAlt)
		thumb := track
		thumb.Height = max(float32(24), track.Height*8/float32(len(regions)))
		thumb.Y += (track.Height - thumb.Height) * float32(p.mapRegionOffset) / float32(len(regions)-8)
		rl.DrawRectangleRounded(thumb, 1, 4, colors.cyan)
	}
	if len(regions) == 0 {
		simpleui.DrawText("Esperando estaciones identificables…", 1140, 370, uiBodyFontSize, colors.muted)
	}
	p.drawFT8SelectedCard(positioned)
	p.drawFT8Timeline(visibleDecodes)
}

func drawFT8RightAligned(text string, right, y float32, size int32, font simpleui.FontStyle, color rl.Color) {
	width := simpleui.MeasureTextStyled(text, size, font).X
	simpleui.DrawTextStyled(text, right-width, y, size, font, color)
}

func calculateFT8QSOStats(decodes []ft8.Decode, now time.Time) ft8QSOStats {
	conversations := reconstructFT8QSOs(decodes)
	result := ft8QSOStats{total: len(conversations)}
	for _, item := range conversations {
		if ft8QSOPhaseRank(item.phase) >= ft8QSOPhaseRank(ft8QSOConfirmed) {
			result.completed++
		}
		if ft8QSOPhaseRank(item.phase) < ft8QSOPhaseRank(ft8QSOConfirmed) && now.Sub(item.last) <= 2*time.Minute {
			result.active++
		}
	}
	return result
}

func reconstructFT8QSOs(decodes []ft8.Decode) map[string]ft8QSOConversation {
	ordered := append([]ft8.Decode(nil), decodes...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ft8DecodeTime(ordered[i]).Before(ft8DecodeTime(ordered[j]))
	})
	conversations := make(map[string]ft8QSOConversation)
	for _, decode := range ordered {
		event, ok := classifyFT8Activity(decode)
		if !ok || event.to == "" {
			continue
		}
		a, b := event.from, event.to
		if a > b {
			a, b = b, a
		}
		key := a + "|" + b
		item := conversations[key]
		when := ft8DecodeTime(decode)
		if item.messages == 0 {
			item.first, item.callA, item.callB = when, a, b
		}
		if when.After(item.last) {
			item.last = when
		}
		item.messages++
		if ft8QSOPhaseRank(event.phase) > ft8QSOPhaseRank(item.phase) {
			item.phase = event.phase
		}
		conversations[key] = item
	}
	return conversations
}

func ft8DecodeTime(decode ft8.Decode) time.Time {
	if !decode.ReceivedAt.IsZero() {
		return decode.ReceivedAt
	}
	return decode.Slot
}

func (p *FT8Panel) drawFT8SelectedCard(positioned int) {
	station := p.selectedFT8MapStation()
	panel := rl.Rectangle{X: 1140, Y: 578, Width: 416, Height: 130}
	rl.DrawRectangleRounded(panel, .035, 7, mixColor(colors.panelAlt, colors.cyan, .035))
	rl.DrawRectangleRoundedLinesEx(panel, .035, 7, 1, colors.border)
	cardTitle := "ESTACIÓN SELECCIONADA"
	if station.call == "" && p.mapRegionFilter != "" {
		cardTitle = "RESUMEN DE LA REGIÓN"
	}
	simpleui.DrawTextStyled(cardTitle, 1152, 589, uiCaptionFontSize, simpleui.FontSemiBold, colors.cyan)
	if station.call == "" {
		if p.mapRegionFilter != "" {
			stations := p.visibleFT8MapStations()
			messages, bestSNR := 0, -99
			for _, item := range stations {
				messages += item.messageCount
				bestSNR = max(bestSNR, item.latest.SNR)
			}
			drawFT8RegionFlag(p.mapRegionFilter, 1152, 619)
			simpleui.DrawTextStyled(clipFT8(p.mapRegionFilter, 27), 1177, 615, 18, simpleui.FontSemiBold, colors.cyan)
			simpleui.DrawText(fmt.Sprintf("%d estaciones · %d mensajes · mejor SNR %+d dB", len(stations), messages, bestSNR), 1152, 646, uiBodyFontSize, colors.text)
			simpleui.DrawText("Haz clic en un marcador para ver la estación.", 1152, 674, uiCaptionFontSize, colors.muted)
			return
		}
		simpleui.DrawTextStyled("NINGUNA", 1152, 614, uiSectionFontSize, simpleui.FontSemiBold, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d estaciones con posición", positioned), 1152, 641, uiCaptionFontSize, colors.muted)
		simpleui.DrawText("Haz clic en un marcador para ver sus datos.", 1152, 666, uiCaptionFontSize, colors.muted)
		return
	}
	backColor := colors.cyan
	rl.DrawLineEx(rl.Vector2{X: 1396, Y: 597}, rl.Vector2{X: 1401, Y: 592}, 1.5, backColor)
	rl.DrawLineEx(rl.Vector2{X: 1396, Y: 597}, rl.Vector2{X: 1401, Y: 602}, 1.5, backColor)
	simpleui.DrawText("VOLVER AL RANKING", 1407, 591, 10, backColor)
	region := ft8CallRegion(station.call)
	drawFT8RegionFlag(region, 1152, 619)
	simpleui.DrawTextStyled(region, 1177, 619, 11, simpleui.FontSemiBold, colors.text)
	callX := float32(1177) + simpleui.MeasureTextStyled(region, 11, simpleui.FontSemiBold).X + 12
	simpleui.DrawTextStyled(station.call+"  ·  "+station.gridOrDash(), callX, 615, 18, simpleui.FontSemiBold, colors.cyan)
	location := "Sin posición disponible"
	if station.hasPosition {
		homeLat, homeLon := p.ft8HomePosition()
		location = fmt.Sprintf("%.0f km  ·  %.0f°  ·  SNR %+d dB", ft8DistanceKM(homeLat, homeLon, station.lat, station.lon), ft8Bearing(homeLat, homeLon, station.lat, station.lon), station.latest.SNR)
	}
	simpleui.DrawText(location, 1152, 641, uiBodyFontSize, colors.text)
	localClock := "HORA LOCAL  NO DISPONIBLE"
	if station.hasPosition {
		if label, ok := localtime.Label(time.Now().UTC(), station.lat, station.lon); ok {
			localClock = "HORA LOCAL  " + label
		}
	}
	simpleui.DrawText(fmt.Sprintf("%d mensajes · %s", station.messageCount, station.latest.Slot.UTC().Format("15:04:05 UTC")), 1152, 665, 10, colors.muted)
	clockWidth := simpleui.MeasureTextStyled(localClock, 10, simpleui.FontRegular).X
	simpleui.DrawTextStyled(localClock, 1544-clockWidth, 665, 10, simpleui.FontRegular, colors.orange)
	messageX := float32(1152)
	if qso, ok := latestFT8QSOForCall(p.activityDecodes(), station.call); ok {
		label, color := ft8QSOPhasePresentation(qso.phase)
		label += "  ·  "
		simpleui.DrawTextStyled(label, messageX, 685, uiCaptionFontSize, simpleui.FontSemiBold, color)
		messageX += simpleui.MeasureTextStyled(label, uiCaptionFontSize, simpleui.FontSemiBold).X
	}
	simpleui.DrawText(clipFT8(station.latest.Message, 34), messageX, 685, uiBodyFontSize, colors.text)
}

func latestFT8QSOForCall(decodes []ft8.Decode, call string) (ft8QSOConversation, bool) {
	call = strings.ToUpper(strings.TrimSpace(call))
	var latest ft8QSOConversation
	found := false
	for _, qso := range reconstructFT8QSOs(decodes) {
		if qso.callA != call && qso.callB != call {
			continue
		}
		if !found || qso.last.After(latest.last) {
			latest, found = qso, true
		}
	}
	return latest, found
}

func ft8QSOPhasePresentation(phase string) (string, rl.Color) {
	switch phase {
	case ft8QSOReport:
		return "QSO · REPORTE", colors.orange
	case ft8QSORogerReport:
		return "QSO · REPORTE R", colors.orange
	case ft8QSOConfirmed:
		return "QSO · CONFIRMADO", colors.green
	case ft8QSOComplete:
		return "QSO · CERRADO", colors.green
	default:
		return "QSO · RESPUESTA", colors.cyan
	}
}

func (p *FT8Panel) drawFT8Timeline(decodes []ft8.Decode) {
	bounds := rl.Rectangle{X: 1140, Y: 718, Width: 416, Height: 130}
	rl.DrawRectangleRounded(bounds, .018, 7, colors.background)
	rl.DrawRectangleRoundedLinesEx(bounds, .018, 7, 1, colors.border)
	simpleui.DrawTextStyled("ÚLTIMOS 60 MIN", 1152, 730, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("Actividad por minuto", 1430, 733, 9, colors.muted)
	counts := make([]int, 60)
	now := time.Now().UTC().Truncate(time.Minute)
	maximum := 1
	for _, decode := range decodes {
		age := int(now.Sub(decode.Slot.UTC().Truncate(time.Minute)) / time.Minute)
		if age >= 0 && age < 60 {
			counts[59-age]++
			maximum = max(maximum, counts[59-age])
		}
	}
	chart := rl.Rectangle{X: 1152, Y: 765, Width: 392, Height: 58}
	rl.DrawLine(int32(chart.X), int32(chart.Y+chart.Height), int32(chart.X+chart.Width), int32(chart.Y+chart.Height), colors.border)
	barWidth := chart.Width / 60
	for index, count := range counts {
		height := chart.Height * float32(count) / float32(maximum)
		color := colors.cyan
		if index == 59 {
			color = colors.green
		}
		rl.DrawRectangle(int32(chart.X+float32(index)*barWidth+1), int32(chart.Y+chart.Height-height), int32(max(2, barWidth-2)), int32(height), color)
	}
	simpleui.DrawText("−60 min", 1152, 828, 9, colors.muted)
	simpleui.DrawText("AHORA", 1508, 828, 9, colors.green)
}

func (p *FT8Panel) ft8RegionActivity(stations []ft8MapStation) []ft8RegionActivity {
	byRegion := make(map[string]*ft8RegionActivity)
	homeLat, homeLon := p.ft8HomePosition()
	for _, station := range stations {
		region := ft8CallRegion(station.call)
		item := byRegion[region]
		if item == nil {
			item = &ft8RegionActivity{name: region, bestSNR: -99}
			byRegion[region] = item
		}
		item.stations++
		item.messages += station.messageCount
		item.bestSNR = max(item.bestSNR, station.latest.SNR)
		if station.hasPosition {
			item.maxDistance = math.Max(item.maxDistance, ft8DistanceKM(homeLat, homeLon, station.lat, station.lon))
		}
	}
	result := make([]ft8RegionActivity, 0, len(byRegion))
	for _, item := range byRegion {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].messages != result[j].messages {
			return result[i].messages > result[j].messages
		}
		return result[i].name < result[j].name
	})
	return result
}

func (p *FT8Panel) stableFT8RegionActivity(stations []ft8MapStation, now time.Time) []ft8RegionActivity {
	regions := p.ft8RegionActivity(stations)
	if len(p.mapRegionOrder) == 0 || !now.Before(p.mapNextRegionRank) {
		p.mapRegionOrder = p.mapRegionOrder[:0]
		for _, region := range regions {
			p.mapRegionOrder = append(p.mapRegionOrder, region.name)
		}
		// Runtime emits the messages in one slot individually. Holding the row
		// order briefly prevents the ranking from jumping for every decode.
		p.mapNextRegionRank = now.Add(3 * time.Second)
		return regions
	}
	order := make(map[string]int, len(p.mapRegionOrder))
	for index, name := range p.mapRegionOrder {
		order[name] = index
	}
	sort.SliceStable(regions, func(i, j int) bool {
		left, leftKnown := order[regions[i].name]
		right, rightKnown := order[regions[j].name]
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown {
			return left < right
		}
		return regions[i].name < regions[j].name
	})
	return regions
}

func drawFT8RegionFlag(name string, x, y float32) {
	b := rl.Rectangle{X: x, Y: y, Width: 17, Height: 12}
	if code := ft8RegionCountryCode(name); code != countries.Unknown {
		loadFT8FlagAtlas()
		if index, ok := ft8FlagIndex[code.Alpha2()]; ok && ft8FlagAtlas.ID != 0 {
			source := rl.Rectangle{X: float32(index%16) * 24, Y: float32(index/16) * 18, Width: 24, Height: 18}
			rl.DrawTexturePro(ft8FlagAtlas, source, b, rl.Vector2{}, 0, rl.White)
			rl.DrawRectangleLinesEx(b, 1, withAlpha(colors.text, 90))
			return
		}
	}
	white := rl.Color{R: 245, G: 248, B: 250, A: 255}
	red := rl.Color{R: 215, G: 40, B: 52, A: 255}
	blue := rl.Color{R: 30, G: 77, B: 150, A: 255}
	yellow := rl.Color{R: 247, G: 196, B: 35, A: 255}
	green := rl.Color{R: 28, G: 150, B: 78, A: 255}
	stripeH := func(a, middle, c rl.Color) {
		rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y, Width: 17, Height: 4}, a)
		rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y + 4, Width: 17, Height: 4}, middle)
		rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y + 8, Width: 17, Height: 4}, c)
	}
	stripeV := func(a, middle, c rl.Color) {
		rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y, Width: 6, Height: 12}, a)
		rl.DrawRectangleRec(rl.Rectangle{X: x + 6, Y: y, Width: 5, Height: 12}, middle)
		rl.DrawRectangleRec(rl.Rectangle{X: x + 11, Y: y, Width: 6, Height: 12}, c)
	}
	switch name {
	case "España", "Spain", "Balearic Islands", "Canary Islands", "Ceuta & Melilla":
		stripeH(red, yellow, red)
	case "Alemania", "Fed. Rep. of Germany":
		stripeH(rl.Black, red, yellow)
	case "Francia", "France", "Corsica":
		stripeV(blue, white, red)
	case "Italia", "Italy", "Sardinia":
		stripeV(green, white, red)
	case "Países Bajos", "Netherlands":
		stripeH(red, white, blue)
	case "Bélgica", "Belgium":
		stripeV(rl.Black, yellow, red)
	case "Austria":
		stripeH(red, white, red)
	case "Polonia", "Poland":
		stripeH(white, white, red)
	case "Finlandia", "Finland":
		rl.DrawRectangleRec(b, white)
		rl.DrawRectangle(int32(x+5), int32(y), 3, 12, blue)
		rl.DrawRectangle(int32(x), int32(y+5), 17, 3, blue)
	case "Suecia", "Sweden":
		rl.DrawRectangleRec(b, blue)
		rl.DrawRectangle(int32(x+5), int32(y), 3, 12, yellow)
		rl.DrawRectangle(int32(x), int32(y+5), 17, 3, yellow)
	case "Suiza", "Switzerland":
		rl.DrawRectangleRec(b, red)
		rl.DrawRectangle(int32(x+7), int32(y+2), 3, 8, white)
		rl.DrawRectangle(int32(x+4), int32(y+5), 9, 3, white)
	case "Japón", "Japan":
		rl.DrawRectangleRec(b, white)
		rl.DrawCircle(int32(x+8), int32(y+6), 3, red)
	case "Portugal", "Azores", "Madeira Islands":
		stripeV(green, green, red)
	case "Canadá", "Canada":
		stripeV(red, white, red)
	case "Estados Unidos", "United States":
		for row := 0; row < 6; row++ {
			color := white
			if row%2 == 0 {
				color = red
			}
			rl.DrawRectangle(int32(x), int32(y)+int32(row*2), 17, 2, color)
		}
		rl.DrawRectangle(int32(x), int32(y), 7, 7, blue)
	case "Reino Unido", "England", "Scotland", "Wales", "Northern Ireland", "Isle of Man":
		rl.DrawRectangleRec(b, blue)
		rl.DrawRectangle(int32(x+7), int32(y), 3, 12, white)
		rl.DrawRectangle(int32(x), int32(y+4), 17, 4, white)
		rl.DrawRectangle(int32(x+8), int32(y), 1, 12, red)
		rl.DrawRectangle(int32(x), int32(y+5), 17, 2, red)
	case "Australia", "Nueva Zelanda", "New Zealand":
		rl.DrawRectangleRec(b, blue)
		rl.DrawCircle(int32(x+12), int32(y+4), 2, white)
	case "Brasil", "Brazil":
		rl.DrawRectangleRec(b, green)
		rl.DrawTriangle(rl.Vector2{X: x + 8.5, Y: y + 2}, rl.Vector2{X: x + 14, Y: y + 6}, rl.Vector2{X: x + 8.5, Y: y + 10}, yellow)
		rl.DrawTriangle(rl.Vector2{X: x + 8.5, Y: y + 2}, rl.Vector2{X: x + 3, Y: y + 6}, rl.Vector2{X: x + 8.5, Y: y + 10}, yellow)
		rl.DrawCircle(int32(x+8), int32(y+6), 2, blue)
	case "Argentina":
		stripeH(rl.SkyBlue, white, rl.SkyBlue)
		rl.DrawCircle(int32(x+8), int32(y+6), 1, yellow)
	case "Chile":
		stripeH(white, white, red)
		rl.DrawRectangle(int32(x), int32(y), 6, 6, blue)
	case "Uruguay":
		stripeH(white, rl.SkyBlue, white)
		rl.DrawCircle(int32(x+3), int32(y+3), 1, yellow)
	case "México", "Mexico":
		stripeV(green, white, red)
	case "Irlanda", "Ireland":
		stripeV(green, white, rl.Orange)
	case "Rumanía", "Romania":
		stripeV(blue, yellow, red)
	case "Ucrania", "Ukraine":
		rl.DrawRectangle(int32(x), int32(y), 17, 6, blue)
		rl.DrawRectangle(int32(x), int32(y+6), 17, 6, yellow)
	case "China":
		rl.DrawRectangleRec(b, red)
		rl.DrawCircle(int32(x+4), int32(y+4), 2, yellow)
	default:
		rl.DrawRectangleRec(b, colors.panelAlt)
		rl.DrawCircleLines(int32(x+8), int32(y+6), 5, colors.muted)
		rl.DrawLine(int32(x+3), int32(y+6), int32(x+13), int32(y+6), colors.muted)
	}
	rl.DrawRectangleLinesEx(b, 1, withAlpha(colors.text, 90))
}

func loadFT8FlagAtlas() {
	ft8FlagOnce.Do(func() {
		codes := countries.All()
		sort.Slice(codes, func(i, j int) bool { return codes[i].Alpha2() < codes[j].Alpha2() })
		ft8FlagIndex = make(map[string]int, len(codes))
		for index, code := range codes {
			ft8FlagIndex[code.Alpha2()] = index
		}
		image := rl.LoadImageFromMemory(".png", ft8FlagAtlasPNG, int32(len(ft8FlagAtlasPNG)))
		if image == nil {
			return
		}
		ft8FlagAtlas = rl.LoadTextureFromImage(image)
		rl.UnloadImage(image)
		rl.SetTextureFilter(ft8FlagAtlas, rl.FilterPoint)
	})
}

func ft8RegionCountryCode(name string) countries.CountryCode {
	ft8DXCCFlagOnce.Do(func() {
		ft8DXCCFlags = make(map[string]string)
		_ = json.Unmarshal(ft8DXCCFlagsJSON, &ft8DXCCFlags)
	})
	if code := ft8DXCCFlags[name]; code != "" {
		if country := countries.ByName(code); country != countries.Unknown {
			return country
		}
	}
	aliases := map[string]string{
		"España": "Spain", "Canary Islands": "Spain", "Balearic Islands": "Spain", "Ceuta & Melilla": "Spain",
		"Alemania": "Germany", "Fed. Rep. of Germany": "Germany",
		"Francia": "France", "Corsica": "France",
		"Italia": "Italy", "Sardinia": "Italy",
		"Países Bajos": "Netherlands", "Bélgica": "Belgium", "Polonia": "Poland",
		"Suecia": "Sweden", "Finlandia": "Finland", "Suiza": "Switzerland",
		"Japón": "Japan", "Canadá": "Canada", "Estados Unidos": "United States",
		"Reino Unido": "United Kingdom", "England": "United Kingdom", "Scotland": "United Kingdom",
		"Wales": "United Kingdom", "Northern Ireland": "United Kingdom", "Isle of Man": "Isle of Man",
		"Nueva Zelanda": "New Zealand", "Brasil": "Brazil", "México": "Mexico",
		"Irlanda": "Ireland", "Rumanía": "Romania", "Ucrania": "Ukraine",
		"Sudáfrica": "South Africa", "Rusia europea": "Russia", "Rusia asiática": "Russia",
		"Azores": "Portugal", "Madeira Islands": "Portugal",
		"Agalega & St. Brandon": "Mauritius", "Bouvet": "Bouvet Island", "Peter 1 Island": "Norway",
		"Dem. Rep. of the Congo": "CD", "Trinidad & Tobago": "Trinidad and Tobago",
		"San Felix & San Ambrosio": "Chile", "Juan Fernandez Islands": "Chile",
		"St. Barthelemy": "Saint Barthelemy", "Reunion Island": "Reunion", "St. Martin": "Saint Martin",
		"Juan de Nova, Europa": "France", "Galapagos Islands": "Ecuador",
		"San Andres & Providencia": "Colombia", "Republic of Korea": "South Korea",
		"African Italy": "Italy", "Sicily": "Italy", "Minami Torishima": "Japan", "Ogasawara": "Japan",
		"Bear Island": "Norway", "Saba & St. Eustatius": "Bonaire, Sint Eustatius and Saba",
		"St. Peter & St. Paul": "Brazil", "Trindade & Martim Vaz": "Brazil",
		"Asiatic Turkey": "Turkey", "European Turkey": "Turkey", "Heard Island": "Heard Island and McDonald Islands",
		"Revillagigedo": "Mexico",
	}
	if alias, ok := aliases[name]; ok {
		name = alias
	}
	return countries.ByName(name)
}

func ft8CallRegion(call string) string {
	ft8DXCCOnce.Do(func() {
		ft8DXCCData, _ = dxcc.Load(resources.Path("tools", "ft8", "cty.dat"))
		if ft8DXCCData == nil {
			ft8DXCCData, _ = dxcc.LoadBundled()
		}
	})
	if entity, ok := ft8DXCCData.Lookup(call); ok {
		return ft8EntityDisplayName(entity.Name)
	}
	return ft8FallbackCallRegion(call)
}

func ft8EntityDisplayName(name string) string {
	translations := map[string]string{
		"Spain": "España", "Fed. Rep. of Germany": "Alemania", "France": "Francia",
		"Italy": "Italia", "Netherlands": "Países Bajos", "Belgium": "Bélgica",
		"Poland": "Polonia", "Sweden": "Suecia", "Finland": "Finlandia",
		"Switzerland": "Suiza", "Japan": "Japón", "Canada": "Canadá",
		"United States": "Estados Unidos", "England": "Reino Unido",
		"New Zealand": "Nueva Zelanda",
		"Brazil":      "Brasil", "Mexico": "México", "Ireland": "Irlanda",
		"Romania": "Rumanía", "Ukraine": "Ucrania", "South Africa": "Sudáfrica",
		"European Russia": "Rusia europea", "Asiatic Russia": "Rusia asiática",
	}
	if translated, ok := translations[name]; ok {
		return translated
	}
	return name
}

// ft8FallbackCallRegion keeps useful identification when an installation has
// lost or damaged its local CTY database.
func ft8FallbackCallRegion(call string) string {
	call = strings.ToUpper(strings.TrimSpace(call))
	prefixes := []struct{ prefix, name string }{
		{"EA", "España"}, {"DL", "Alemania"}, {"G", "Reino Unido"}, {"M", "Reino Unido"},
		{"F", "Francia"}, {"I", "Italia"}, {"IK", "Italia"}, {"IZ", "Italia"}, {"CT", "Portugal"},
		{"PA", "Países Bajos"}, {"ON", "Bélgica"}, {"OE", "Austria"}, {"HB", "Suiza"},
		{"SP", "Polonia"}, {"SM", "Suecia"}, {"OH", "Finlandia"}, {"JA", "Japón"},
		{"VE", "Canadá"}, {"VK", "Australia"}, {"ZL", "Nueva Zelanda"},
	}
	for _, item := range prefixes {
		if strings.HasPrefix(call, item.prefix) {
			return item.name
		}
	}
	if len(call) > 0 && strings.Contains("KNAW", call[:1]) {
		return "Estados Unidos"
	}
	return "Otras regiones"
}

func (p *FT8Panel) drawFT8MapDetail() {
	panel := rl.Rectangle{X: 1120, Y: 126, Width: 390, Height: 492}
	rl.DrawRectangleRounded(panel, .025, 8, mixColor(colors.panelAlt, colors.blue, .05))
	rl.DrawRectangleRoundedLinesEx(panel, .025, 8, 1, colors.border)
	simpleui.DrawTextStyled("ESTACIÓN SELECCIONADA", 1143, 146, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	if p.mapSelected < 0 || p.mapSelected >= len(p.mapDecodes) {
		simpleui.DrawText("Sin estación seleccionada", 1143, 190, uiBodyFontSize, colors.muted)
		return
	}
	decode := p.mapDecodes[p.mapSelected]
	station := p.selectedFT8MapStation()
	call := station.call
	if call == "" {
		call = "ESTACIÓN SIN IDENTIFICAR"
	}
	simpleui.DrawTextStyled(call, 1143, 184, 28, simpleui.FontSemiBold, colors.text)
	location := "Sin locator en el mensaje"
	if station.hasPosition {
		location = fmt.Sprintf("%s  ·  %.3f°, %.3f°", station.grid, station.lat, station.lon)
	}
	simpleui.DrawText(location, 1143, 221, uiBodyFontSize, map[bool]rl.Color{true: colors.cyan, false: colors.orange}[station.hasPosition])
	rl.DrawLine(1143, 251, 1488, 251, colors.border)

	detailRows := []struct{ label, value string }{
		{"UTC", decode.Slot.UTC().Format("15:04:05")},
		{"SNR", fmt.Sprintf("%+d dB", decode.SNR)},
		{"DESFASE", fmt.Sprintf("%+.2f s", decode.DT)},
		{"AUDIO", fmt.Sprintf("%.0f Hz", decode.Frequency)},
	}
	if station.hasPosition {
		homeLat, homeLon := p.ft8HomePosition()
		detailRows = append(detailRows,
			struct{ label, value string }{"DISTANCIA", fmt.Sprintf("%.0f km", ft8DistanceKM(homeLat, homeLon, station.lat, station.lon))},
			struct{ label, value string }{"RUMBO", fmt.Sprintf("%.0f°", ft8Bearing(homeLat, homeLon, station.lat, station.lon))},
		)
	}
	for index, row := range detailRows {
		y := float32(275 + index*45)
		simpleui.DrawText(row.label, 1143, y, uiCaptionFontSize, colors.muted)
		width := simpleui.MeasureTextStyled(row.value, uiBodyFontSize, simpleui.FontMono).X
		simpleui.DrawTextStyled(row.value, 1488-width, y-2, uiBodyFontSize, simpleui.FontMono, colors.text)
	}
	y := float32(275 + len(detailRows)*45 + 8)
	simpleui.DrawText("MENSAJE RECIBIDO", 1143, y, uiCaptionFontSize, colors.muted)
	drawWrapped(decode.Message, 1143, y+25, 335, uiBodyFontSize, colors.cyan)
	if !station.hasPosition {
		simpleui.DrawTextStyled("POSICIÓN NO DISPONIBLE", 1143, 568, uiCaptionFontSize, simpleui.FontSemiBold, colors.orange)
	}
	simpleui.DrawText("Arrastra para mover · rueda para zoom · ESC para cerrar", 1143, 598, 10, colors.muted)
}

func (p *FT8Panel) ft8MapStations() []ft8MapStation {
	decodes := p.activityDecodes()
	type location struct {
		grid     string
		lat, lon float64
	}
	locations := make(map[string]location)
	counts := make(map[string]int)
	// Oldest to newest builds a session memory; a later valid locator refreshes
	// the position used by reports, RRR, RR73 and 73 messages.
	for index := len(decodes) - 1; index >= 0; index-- {
		decode := decodes[index]
		call := ft8MessageCall(decode.Message)
		if call == "" {
			continue
		}
		counts[call]++
		_, grid := ft8MessageStation(decode.Message)
		if lat, lon, ok := maidenheadCenter(grid); ok {
			locations[call] = location{grid: grid, lat: lat, lon: lon}
		}
	}
	selectedCall := ""
	if p.mapSelected >= 0 && p.mapSelected < len(p.mapDecodes) {
		selectedCall = ft8MessageCall(p.mapDecodes[p.mapSelected].Message)
	}
	seen := make(map[string]bool)
	stations := make([]ft8MapStation, 0, len(locations))
	for _, decode := range decodes {
		call := ft8MessageCall(decode.Message)
		if call == "" || seen[call] {
			continue
		}
		seen[call] = true
		station := ft8MapStation{call: call, latest: decode, messageCount: counts[call], selected: call == selectedCall}
		if value, exists := locations[call]; exists {
			station.grid, station.lat, station.lon, station.hasPosition = value.grid, value.lat, value.lon, true
		}
		stations = append(stations, station)
	}
	for index := range stations {
		if stations[index].selected && index > 0 {
			selected := stations[index]
			copy(stations[1:index+1], stations[0:index])
			stations[0] = selected
			break
		}
	}
	if p.mapOnlyPosition {
		positioned := stations[:0]
		for _, station := range stations {
			if station.hasPosition {
				positioned = append(positioned, station)
			}
		}
		stations = positioned
	}
	return stations
}

func (p *FT8Panel) visibleFT8MapStations() []ft8MapStation {
	return p.filterFT8StationsByRegion(p.ft8MapStations())
}

func (p *FT8Panel) filterFT8StationsByRegion(stations []ft8MapStation) []ft8MapStation {
	if p.mapRegionFilter == "" {
		return stations
	}
	filtered := make([]ft8MapStation, 0, len(stations))
	for _, station := range stations {
		if ft8CallRegion(station.call) == p.mapRegionFilter {
			filtered = append(filtered, station)
		}
	}
	return filtered
}

func (p *FT8Panel) filterFT8DecodesByRegion(decodes []ft8.Decode) []ft8.Decode {
	if p.mapRegionFilter == "" {
		return decodes
	}
	filtered := make([]ft8.Decode, 0, len(decodes))
	for _, decode := range decodes {
		matched := false
		if event, ok := classifyFT8Activity(decode); ok {
			matched = (event.from != "" && ft8CallRegion(event.from) == p.mapRegionFilter) ||
				(event.to != "" && ft8CallRegion(event.to) == p.mapRegionFilter)
		} else if call := ft8MessageCall(decode.Message); call != "" {
			matched = ft8CallRegion(call) == p.mapRegionFilter
		}
		if matched {
			filtered = append(filtered, decode)
		}
	}
	return filtered
}

func (p *FT8Panel) selectedFT8MapStation() ft8MapStation {
	for _, station := range p.ft8MapStations() {
		if station.selected {
			return station
		}
	}
	return ft8MapStation{}
}

func (p *FT8Panel) drawFT8MapTable() {
	bounds := rl.Rectangle{X: 90, Y: 636, Width: 1420, Height: 180}
	rl.DrawRectangleRounded(bounds, .018, 7, colors.background)
	rl.DrawRectangleRoundedLinesEx(bounds, .018, 7, 1, colors.border)
	stations := p.ft8MapStations()
	positioned := 0
	for _, station := range stations {
		if station.hasPosition {
			positioned++
		}
	}
	simpleui.DrawTextStyled("ESTACIONES DE LA SESIÓN", 108, 649, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%d estaciones · %d con posición", len(stations), positioned), 345, 651, uiCaptionFontSize, colors.muted)
	drawModalAction(rl.Rectangle{X: 1130, Y: 645, Width: 100, Height: 30}, "TODAS", colors.cyan, true)
	drawModalAction(rl.Rectangle{X: 1240, Y: 645, Width: 90, Height: 30}, "CQ", colors.border, false)
	drawModalAction(rl.Rectangle{X: 1340, Y: 645, Width: 150, Height: 30}, "CONTACTOS", colors.border, false)

	rl.DrawRectangle(91, 681, 1418, 27, colors.panelAlt)
	for _, header := range []struct {
		x    float32
		text string
	}{{108, "UTC"}, {220, "INDICATIVO"}, {390, "LOCATOR"}, {520, "SNR"}, {620, "DISTANCIA"}, {780, "MENSAJE MÁS RECIENTE"}, {1390, "RX"}} {
		simpleui.DrawText(header.text, header.x, 689, 10, colors.cyan)
	}
	homeLat, homeLon := p.ft8HomePosition()
	row := 0
	for _, station := range stations {
		if row >= 4 {
			break
		}
		y := float32(709 + row*26)
		if station.selected {
			selection, _ := tableSelectionColors()
			rl.DrawRectangle(91, int32(y), 1418, 25, selection)
		} else if row%2 == 0 {
			rl.DrawRectangle(91, int32(y), 1418, 25, mixColor(colors.panelAlt, colors.blue, .05))
		}
		distance := "—"
		if station.hasPosition {
			distance = fmt.Sprintf("%.0f km", ft8DistanceKM(homeLat, homeLon, station.lat, station.lon))
		}
		simpleui.DrawTextStyled(station.latest.Slot.UTC().Format("15:04:05"), 108, y+6, 11, simpleui.FontMono, colors.muted)
		simpleui.DrawTextStyled(station.call, 220, y+6, 11, simpleui.FontSemiBold, map[bool]rl.Color{true: colors.text, false: colors.muted}[station.selected])
		simpleui.DrawText(station.gridOrDash(), 390, y+6, 11, colors.text)
		simpleui.DrawTextStyled(fmt.Sprintf("%+d", station.latest.SNR), 520, y+6, 11, simpleui.FontMono, ft8SNRColor(station.latest.SNR))
		simpleui.DrawText(distance, 620, y+6, 11, colors.text)
		simpleui.DrawText(clipFT8(station.latest.Message, 62), 780, y+6, 11, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d", station.messageCount), 1400, y+6, 11, colors.muted)
		row++
	}
}

func (station ft8MapStation) gridOrDash() string {
	if station.grid == "" {
		return "—"
	}
	return station.grid
}

func (p *FT8Panel) ft8HomePosition() (float64, float64) {
	if p.mapHomeSet {
		return p.mapHomeLat, p.mapHomeLon
	}
	if p.screen != nil && p.screen.voacapPanel != nil {
		settings := p.screen.voacapPanel.settings
		if settings.Latitude >= -90 && settings.Latitude <= 90 && settings.Longitude >= -180 && settings.Longitude <= 180 {
			return settings.Latitude, settings.Longitude
		}
	}
	return 40.4168, -3.7038
}

func ft8DecodeMapPoint(decode ft8.Decode) ft8MapPoint {
	call, grid := ft8MessageStation(decode.Message)
	lat, lon, ok := maidenheadCenter(grid)
	return ft8MapPoint{decode: decode, call: call, grid: grid, lat: lat, lon: lon, hasPosition: ok}
}

func ft8MessageStation(message string) (call, grid string) {
	fields := strings.Fields(strings.ToUpper(message))
	for index := len(fields) - 1; index >= 0; index-- {
		if _, _, ok := ft8LocatorToken(fields[index]); ok {
			grid = fields[index]
			if index > 0 {
				call = strings.Trim(fields[index-1], "<>")
			}
			return call, grid
		}
	}
	return "", ""
}

func ft8MessageCall(message string) string {
	if call, _ := ft8MessageStation(message); call != "" {
		return call
	}
	fields := strings.Fields(strings.ToUpper(message))
	if len(fields) == 0 {
		return ""
	}
	if fields[0] == "CQ" {
		for index := len(fields) - 1; index >= 1; index-- {
			if ft8PlausibleCall(fields[index]) {
				return strings.Trim(fields[index], "<>")
			}
		}
		return ""
	}
	// In a directed FT8 message the second callsign is the transmitting
	// station; the first is the addressee.
	if len(fields) >= 2 && ft8PlausibleCall(fields[1]) {
		return strings.Trim(fields[1], "<>")
	}
	if ft8PlausibleCall(fields[0]) {
		return strings.Trim(fields[0], "<>")
	}
	return ""
}

func ft8PlausibleCall(value string) bool {
	value = strings.Trim(strings.ToUpper(value), "<>")
	if len(value) < 3 || len(value) > 13 {
		return false
	}
	hasLetter, hasDigit := false, false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '/':
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}

func ft8LocatorToken(token string) (lat, lon float64, ok bool) {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "RR73":
		return 0, 0, false
	}
	return maidenheadCenter(token)
}

func maidenheadCenter(locator string) (lat, lon float64, ok bool) {
	locator = strings.ToUpper(strings.TrimSpace(locator))
	if len(locator) != 4 && len(locator) != 6 && len(locator) != 8 {
		return 0, 0, false
	}
	for _, r := range locator {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return 0, 0, false
		}
	}
	a, b := int(locator[0]-'A'), int(locator[1]-'A')
	c, d := int(locator[2]-'0'), int(locator[3]-'0')
	if a < 0 || a >= 18 || b < 0 || b >= 18 || c < 0 || c > 9 || d < 0 || d > 9 {
		return 0, 0, false
	}
	lon, lat = -180+float64(a)*20+float64(c)*2, -90+float64(b)*10+float64(d)
	lonSize, latSize := 2.0, 1.0
	if len(locator) >= 6 {
		e, f := int(locator[4]-'A'), int(locator[5]-'A')
		if e < 0 || e >= 24 || f < 0 || f >= 24 {
			return 0, 0, false
		}
		lonSize, latSize = 2.0/24, 1.0/24
		lon += float64(e) * lonSize
		lat += float64(f) * latSize
	}
	if len(locator) == 8 {
		g, h := int(locator[6]-'0'), int(locator[7]-'0')
		if g < 0 || g > 9 || h < 0 || h > 9 {
			return 0, 0, false
		}
		lonSize, latSize = lonSize/10, latSize/10
		lon += float64(g) * lonSize
		lat += float64(h) * latSize
	}
	return lat + latSize/2, lon + lonSize/2, true
}

func ft8SNRColor(snr int) rl.Color {
	if snr >= -10 {
		return colors.green
	}
	if snr >= -18 {
		return colors.cyan
	}
	return colors.orange
}

func ft8DistanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earth = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := (lat2-lat1)*math.Pi/180, (lon2-lon1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return earth * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func ft8Bearing(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dl := (lon2 - lon1) * math.Pi / 180
	y := math.Sin(dl) * math.Cos(p2)
	x := math.Cos(p1)*math.Sin(p2) - math.Sin(p1)*math.Cos(p2)*math.Cos(dl)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

func drawFT8Route(lat1, lon1, lat2, lon2, centerLat, centerLon, span float64, bounds rl.Rectangle, color rl.Color) {
	previous := projectMercator(centerLat, centerLon, span, lat1, lon1, bounds)
	dLon := wrapLongitude(lon2 - lon1)
	for step := 1; step <= 48; step++ {
		t := float64(step) / 48
		lat := lat1 + (lat2-lat1)*t
		lon := wrapLongitude(lon1 + dLon*t)
		current := projectMercator(centerLat, centerLon, span, lat, lon, bounds)
		if rl.CheckCollisionPointRec(previous, bounds) || rl.CheckCollisionPointRec(current, bounds) {
			rl.DrawLineEx(previous, current, 2, color)
		}
		previous = current
	}
}
