package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/resources"
	"go-zero/internal/satellite"
	"go-zero/simpleui"
)

func RunSatelliteMap(path string) {
	simpleui.SetMode(1440, 840, simpleui.Fit)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.25)
	simpleui.SetTitle("IC-SDR · Satellite tracking")
	simpleui.SetMinimumSize(960, 560)
	v := &satelliteMap{path: path, enabled: map[int]bool{}, expanded: map[string]bool{"Space stations": true, "Amateur radio": true, "Weather": true}, selected: 25544}
	v.search = simpleui.NewTextField("satelliteMapSearch", 28, 105, 296, 34, "SEARCH NAME, NORAD OR GROUP", 12)
	v.search.SetMaxLength(64)
	v.search.OnChange(func(value string) { v.query = strings.ToLower(strings.TrimSpace(value)); v.scroll = 0 })
	simpleui.Add(v.search)
	simpleui.Run(v.draw)
}

type satelliteMap struct {
	path        string
	snapshot    satellite.Snapshot
	next        time.Time
	enabled     map[int]bool
	expanded    map[string]bool
	selected    int
	scroll      int
	initialized bool
	mapTexture  rl.Texture2D
	search      *simpleui.TextField
	query       string
}

func (v *satelliteMap) read() {
	if time.Now().Before(v.next) {
		return
	}
	v.next = time.Now().Add(500 * time.Millisecond)
	data, err := os.ReadFile(v.path)
	if err != nil {
		return
	}
	var snap satellite.Snapshot
	if json.Unmarshal(data, &snap) != nil {
		return
	}
	v.snapshot = snap
	if validTheme(snap.Theme) {
		colors = paletteForTheme(snap.Theme)
		simpleui.SetTheme(simpleUITheme(colors, snap.Theme == themeLight))
	}
	if !v.initialized {
		for _, s := range snap.Satellites {
			v.enabled[s.NORAD] = s.NORAD == 25544 || s.NORAD == 43700
		}
		v.selected = snap.SelectedNORAD
		v.initialized = true
	}
}
func worldProject(lat, lon float64, b rl.Rectangle) rl.Vector2 {
	return rl.Vector2{X: b.X + float32((lon+180)/360)*b.Width, Y: b.Y + float32((90-lat)/180)*b.Height}
}

func worldUnproject(p rl.Vector2, b rl.Rectangle) (lat, lon float64) {
	if b.Width <= 0 || b.Height <= 0 {
		return 0, 0
	}
	lon = (float64(p.X-b.X)/float64(b.Width))*360 - 180
	lat = 90 - (float64(p.Y-b.Y)/float64(b.Height))*180
	lat = math.Min(90, math.Max(-90, lat))
	lon = math.Min(180, math.Max(-180, lon))
	return lat, lon
}

func drawDisclosureIcon(center rl.Vector2, open bool, color rl.Color) {
	if open {
		left := rl.Vector2{X: center.X - 6, Y: center.Y - 4}
		right := rl.Vector2{X: center.X + 6, Y: center.Y - 4}
		bottom := rl.Vector2{X: center.X, Y: center.Y + 5}
		rl.DrawTriangleLines(left, right, bottom, color)
		return
	}
	top := rl.Vector2{X: center.X - 4, Y: center.Y - 6}
	bottom := rl.Vector2{X: center.X - 4, Y: center.Y + 6}
	right := rl.Vector2{X: center.X + 5, Y: center.Y}
	rl.DrawTriangleLines(top, right, bottom, color)
}

func drawMapToggle(center rl.Vector2, enabled bool) {
	bounds := rl.Rectangle{X: center.X - 6, Y: center.Y - 6, Width: 12, Height: 12}
	rl.DrawRectangleLinesEx(bounds, 1.5, colors.cyan)
	if enabled {
		rl.DrawLineEx(rl.Vector2{X: center.X - 4, Y: center.Y}, rl.Vector2{X: center.X - 1, Y: center.Y + 4}, 2, colors.cyan)
		rl.DrawLineEx(rl.Vector2{X: center.X - 1, Y: center.Y + 4}, rl.Vector2{X: center.X + 5, Y: center.Y - 4}, 2, colors.cyan)
	}
}

func drawVisibilityIndicator(center rl.Vector2, visible bool) {
	color := colors.muted
	if visible {
		color = colors.green
		rl.DrawCircleV(center, 3, color)
	}
	rl.DrawCircleLines(int32(center.X), int32(center.Y), 6, color)
}
func (v *satelliteMap) draw() {
	v.read()
	rl.ClearBackground(colors.background)
	left := rl.Rectangle{X: 16, Y: 68, Width: 320, Height: 748}
	mapBox := rl.Rectangle{X: 352, Y: 68, Width: 1072, Height: 556}
	details := rl.Rectangle{X: 352, Y: 642, Width: 1072, Height: 174}
	drawPanel(left.X, left.Y, left.Width, left.Height)
	drawPanel(details.X, details.Y, details.Width, details.Height)
	v.drawHeader()
	v.drawList(left)
	v.drawMap(mapBox)
	v.drawDetails(details)
}
func (v *satelliteMap) drawHeader() {
	simpleui.DrawTextStyled("SATELLITE TRACKING", 20, 19, 23, simpleui.FontSemiBold, colors.cyan)
	visible := 0
	for _, s := range v.snapshot.Satellites {
		if s.Visible {
			visible++
		}
	}
	simpleui.DrawText(fmt.Sprintf("%d objects · %d visible from %s · click empty map to set your location · %s", len(v.snapshot.Satellites), visible, v.snapshot.Station.Name, v.snapshot.Source), 380, 27, 13, colors.muted)
}
func (v *satelliteMap) grouped() ([]string, map[string][]satellite.State) {
	m := map[string][]satellite.State{}
	for _, s := range v.snapshot.Satellites {
		if v.query != "" {
			haystack := strings.ToLower(fmt.Sprintf("%s %s %d", s.Name, s.Group, s.NORAD))
			if !strings.Contains(haystack, v.query) {
				continue
			}
		}
		m[s.Group] = append(m[s.Group], s)
	}
	order := []string{"Space stations", "Amateur radio", "CubeSats", "Weather", "GPS", "Galileo", "GLONASS", "BeiDou", "Iridium NEXT", "Orbcomm", "Starlink"}
	return order, m
}
func (v *satelliteMap) drawList(b rl.Rectangle) {
	simpleui.DrawTextStyled("CATALOG", b.X+12, b.Y+10, 15, simpleui.FontSemiBold, colors.orange)
	order, groups := v.grouped()
	mouse := simpleui.MousePosition()
	contentBounds := rl.Rectangle{X: b.X + 1, Y: b.Y + 78, Width: b.Width - 2, Height: b.Height - 79}
	if rl.CheckCollisionPointRec(mouse, contentBounds) {
		v.scroll -= int(rl.GetMouseWheelMove())
		v.scroll = max(0, v.scroll)
	}
	rl.BeginScissorMode(int32(contentBounds.X), int32(contentBounds.Y), int32(contentBounds.Width), int32(contentBounds.Height))
	listTop := b.Y + 82
	scrollOffset := float32(v.scroll * 24)
	y := listTop - scrollOffset
	for _, g := range order {
		list := groups[g]
		if len(list) == 0 {
			continue
		}
		row := rl.Rectangle{X: b.X + 4, Y: y, Width: b.Width - 8, Height: 28}
		if row.Y+row.Height > contentBounds.Y && row.Y < contentBounds.Y+contentBounds.Height {
			if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mouse, contentBounds) && rl.CheckCollisionPointRec(mouse, row) {
				v.expanded[g] = !v.expanded[g]
			}
			drawDisclosureIcon(rl.Vector2{X: row.X + 10, Y: row.Y + 14}, v.expanded[g] || v.query != "", colors.text)
			simpleui.DrawTextStyled(fmt.Sprintf("%s  (%d)", g, len(list)), row.X+24, row.Y+4, 14, simpleui.FontSemiBold, colors.text)
		}
		y += 30
		if !v.expanded[g] && v.query == "" {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		for _, s := range list {
			r := rl.Rectangle{X: b.X + 28, Y: y, Width: b.Width - 36, Height: 27}
			if r.Y+r.Height > contentBounds.Y && r.Y < contentBounds.Y+contentBounds.Height {
				if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mouse, contentBounds) && rl.CheckCollisionPointRec(mouse, r) {
					if mouse.X < r.X+42 {
						v.enabled[s.NORAD] = !v.enabled[s.NORAD]
					} else {
						v.selected = s.NORAD
						v.enabled[s.NORAD] = true
						_ = os.WriteFile(v.path+".select", []byte(strconv.Itoa(s.NORAD)), 0644)
					}
				}
				drawMapToggle(rl.Vector2{X: r.X + 7, Y: r.Y + 13}, v.enabled[s.NORAD])
				drawVisibilityIndicator(rl.Vector2{X: r.X + 28, Y: r.Y + 13}, s.Visible)
				name := s.Name
				if len([]rune(name)) > 25 {
					name = string([]rune(name)[:22]) + "..."
				}
				c := colors.text
				if s.NORAD == v.selected {
					c = colors.orange
				}
				simpleui.DrawText(name, r.X+43, r.Y+5, 12, c)
			}
			y += 28
		}
	}
	rl.EndScissorMode()
	// Restore the visual offset before measuring the content. Measuring `y`
	// directly made the reported height shrink while scrolling, so the clamp
	// stopped before the final satellites and any following groups.
	contentHeight := y + scrollOffset - listTop
	viewportHeight := contentBounds.Y + contentBounds.Height - listTop
	v.scroll = min(v.scroll, catalogMaxScroll(contentHeight, viewportHeight))
}

func catalogMaxScroll(contentHeight, viewportHeight float32) int {
	if contentHeight <= viewportHeight {
		return 0
	}
	return int(math.Ceil(float64((contentHeight - viewportHeight) / 24)))
}
func (v *satelliteMap) drawMap(b rl.Rectangle) {
	v.handleMapClick(b)
	if v.mapTexture.ID == 0 {
		img := rl.LoadImageFromMemory(".png", aisWorldPNG, int32(len(aisWorldPNG)))
		if img != nil && img.Data != nil {
			v.mapTexture = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
			rl.SetTextureFilter(v.mapTexture, rl.FilterBilinear)
		}
	}
	if v.mapTexture.ID != 0 {
		tex := v.mapTexture
		rl.DrawTexturePro(tex, rl.Rectangle{Width: float32(tex.Width), Height: float32(tex.Height)}, b, rl.Vector2{}, 0, rl.White)
	} else {
		rl.DrawRectangleRec(b, colors.panelAlt)
	}
	for lon := -180; lon <= 180; lon += 30 {
		a := worldProject(-90, float64(lon), b)
		z := worldProject(90, float64(lon), b)
		rl.DrawLineEx(a, z, 1, withAlpha(colors.grid, 70))
	}
	for lat := -60; lat <= 60; lat += 30 {
		a := worldProject(float64(lat), -180, b)
		z := worldProject(float64(lat), 180, b)
		rl.DrawLineEx(a, z, 1, withAlpha(colors.grid, 70))
	}
	var selectedState *satellite.State
	var selectedPoint rl.Vector2
	for i := range v.snapshot.Satellites {
		s := &v.snapshot.Satellites[i]
		if !v.enabled[s.NORAD] {
			continue
		}
		c := colors.cyan
		if !s.Visible {
			c = colors.muted
		}
		if s.NORAD == v.selected {
			c = colors.orange
			for i := 1; i < len(s.Trajectory); i++ {
				a, z := s.Trajectory[i-1], s.Trajectory[i]
				if math.Abs(a.Longitude-z.Longitude) < 180 {
					rl.DrawLineEx(worldProject(a.Latitude, a.Longitude, b), worldProject(z.Latitude, z.Longitude, b), 2, withAlpha(c, 180))
				}
			}
		}
		p := worldProject(s.Latitude, s.Longitude, b)
		rl.DrawCircleV(p, 6, c)
		simpleui.DrawText(sondeClip(s.Name, 18), p.X+9, p.Y-7, 11, c)
		if s.NORAD == v.selected {
			selectedState, selectedPoint = s, p
		}
	}
	station := worldProject(v.snapshot.Station.Latitude, v.snapshot.Station.Longitude, b)
	rl.DrawCircleV(station, 5, colors.green)
	simpleui.DrawText(v.snapshot.Station.Name, station.X+8, station.Y-7, 11, colors.text)
	if selectedState != nil {
		v.drawPassCard(b, selectedPoint, selectedState.NextPass)
	}
	rl.DrawRectangleLinesEx(b, 2, colors.border)
	legendBar := rl.Rectangle{X: b.X + 1, Y: b.Y + b.Height - 28, Width: b.Width - 2, Height: 27}
	legendBackground := colors.panel
	legendBackground.A = 232
	rl.DrawRectangleRec(legendBar, legendBackground)
	rl.DrawLine(int32(legendBar.X), int32(legendBar.Y), int32(legendBar.X+legendBar.Width), int32(legendBar.Y), colors.border)
	legendY := legendBar.Y + 7
	drawMapToggle(rl.Vector2{X: b.X + 7, Y: legendY + 5}, true)
	simpleui.DrawText("show/hide", b.X+19, legendY, 11, colors.muted)
	drawVisibilityIndicator(rl.Vector2{X: b.X + 151, Y: legendY + 5}, true)
	simpleui.DrawText("visible from the station", b.X+163, legendY, 11, colors.muted)
	drawVisibilityIndicator(rl.Vector2{X: b.X + 355, Y: legendY + 5}, false)
	simpleui.DrawText("below the horizon", b.X+367, legendY, 11, colors.muted)
}

func passCardRect(mapBounds rl.Rectangle, target rl.Vector2) rl.Rectangle {
	const width, height = float32(244), float32(132)
	usableBottom := mapBounds.Y + mapBounds.Height - 36
	x, y := target.X+18, target.Y+18
	if x+width > mapBounds.X+mapBounds.Width-8 {
		x = target.X - width - 18
	}
	if y+height > usableBottom {
		y = target.Y - height - 18
	}
	x = min(max(x, mapBounds.X+8), mapBounds.X+mapBounds.Width-width-8)
	y = min(max(y, mapBounds.Y+8), usableBottom-height)
	return rl.Rectangle{X: x, Y: y, Width: width, Height: height}
}

func (v *satelliteMap) handleMapClick(b rl.Rectangle) {
	if !rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		return
	}
	mouse := simpleui.MousePosition()
	legend := rl.Rectangle{X: b.X + 1, Y: b.Y + b.Height - 28, Width: b.Width - 2, Height: 27}
	if !rl.CheckCollisionPointRec(mouse, b) || rl.CheckCollisionPointRec(mouse, legend) {
		return
	}
	bestNORAD := 0
	bestDist := 14.0
	var selectedPoint rl.Vector2
	haveSelected := false
	for i := range v.snapshot.Satellites {
		s := &v.snapshot.Satellites[i]
		if !v.enabled[s.NORAD] {
			continue
		}
		p := worldProject(s.Latitude, s.Longitude, b)
		if s.NORAD == v.selected {
			selectedPoint, haveSelected = p, true
		}
		d := math.Hypot(float64(mouse.X-p.X), float64(mouse.Y-p.Y))
		if d < bestDist {
			bestDist = d
			bestNORAD = s.NORAD
		}
	}
	if bestNORAD != 0 {
		v.selected = bestNORAD
		v.enabled[bestNORAD] = true
		_ = os.WriteFile(v.path+".select", []byte(strconv.Itoa(bestNORAD)), 0644)
		return
	}
	if haveSelected && rl.CheckCollisionPointRec(mouse, passCardRect(b, selectedPoint)) {
		return
	}
	lat, lon := worldUnproject(mouse, b)
	station := v.snapshot.Station
	if strings.TrimSpace(station.Name) == "" || satellite.IsFactoryDefault(station) {
		station.Name = "Home"
	}
	station.Latitude, station.Longitude = lat, lon
	if err := satellite.SaveStationFile(resources.WritablePath("config", "satellite-station.json"), station); err != nil {
		return
	}
	v.snapshot.Station = station
}

func (v *satelliteMap) drawPassCard(mapBounds rl.Rectangle, target rl.Vector2, pass satellite.PassPrediction) {
	card := passCardRect(mapBounds, target)

	anchor := rl.Vector2{X: min(max(target.X, card.X), card.X+card.Width), Y: min(max(target.Y, card.Y), card.Y+card.Height)}
	rl.DrawLineEx(target, anchor, 2, colors.orange)
	background := colors.panel
	background.A = 245
	rl.DrawRectangleRounded(card, .06, 8, background)
	rl.DrawRectangleRoundedLinesEx(card, .06, 8, 2, colors.orange)

	simpleui.DrawTextStyled(T("NEXT PASS"), card.X+13, card.Y+10, 13, simpleui.FontSemiBold, colors.orange)
	if pass.Continuous {
		status := T("CONTINUOUSLY VISIBLE")
		if !pass.InProgress {
			status = T("BELOW THE HORIZON")
		}
		simpleui.DrawTextStyled(status, card.X+13, card.Y+43, 14, simpleui.FontSemiBold, colors.text)
		simpleui.DrawText(T("No AOS/LOS in the next 48 h"), card.X+13, card.Y+72, 11, colors.muted)
		return
	}
	if !pass.Found {
		simpleui.DrawTextStyled(T("NO PASSES IN 48 H"), card.X+13, card.Y+43, 14, simpleui.FontSemiBold, colors.text)
		simpleui.DrawText(T("Based on current orbital elements"), card.X+13, card.Y+72, 11, colors.muted)
		return
	}

	now := v.snapshot.Updated.Local()
	tca := pass.TCA.Local()
	simpleui.DrawTextStyled(passDayLabel(now, tca)+" · "+tca.Format("15:04")+" "+localZoneLabel(tca), card.X+13, card.Y+32, 14, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText(passCountdown(v.snapshot.Updated, pass.TCA, pass.InProgress), card.X+13, card.Y+55, 11, colors.text)
	rl.DrawLine(int32(card.X+12), int32(card.Y+75), int32(card.X+card.Width-12), int32(card.Y+75), withAlpha(colors.border, 180))
	simpleui.DrawText(fmt.Sprintf("%s   %.0f km", T("MIN DISTANCE"), pass.MinRangeKM), card.X+13, card.Y+82, 11, colors.text)
	simpleui.DrawText(fmt.Sprintf("%s     %.1f°", T("MAX ELEVATION"), pass.MaxElevation), card.X+13, card.Y+101, 11, colors.text)
	simpleui.DrawText(fmt.Sprintf("AOS %s  ·  LOS %s", pass.AOS.Local().Format("15:04"), pass.LOS.Local().Format("15:04")), card.X+13, card.Y+119, 10, colors.muted)
}

func passDayLabel(now, event time.Time) string {
	y1, m1, d1 := now.Date()
	y2, m2, d2 := event.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return T("TODAY")
	}
	tomorrow := now.AddDate(0, 0, 1)
	y3, m3, d3 := tomorrow.Date()
	if y2 == y3 && m2 == m3 && d2 == d3 {
		return T("TOMORROW")
	}
	return event.Format("02/01")
}

func localZoneLabel(at time.Time) string {
	zone, _ := at.Zone()
	if zone == "" {
		return "LOCAL"
	}
	return zone
}

func passCountdown(now, event time.Time, inProgress bool) string {
	if inProgress {
		return T("PASS IN PROGRESS")
	}
	minutes := int(math.Round(event.Sub(now).Minutes()))
	if minutes <= 0 {
		return T("NOW")
	}
	if minutes < 60 {
		return fmt.Sprintf(T("IN %d min"), minutes)
	}
	return fmt.Sprintf(T("IN %d h %02d min"), minutes/60, minutes%60)
}
func (v *satelliteMap) drawDetails(b rl.Rectangle) {
	simpleui.DrawTextStyled("SELECTED SATELLITE", b.X+14, b.Y+10, 14, simpleui.FontSemiBold, colors.orange)
	var s *satellite.State
	for i := range v.snapshot.Satellites {
		if v.snapshot.Satellites[i].NORAD == v.selected {
			s = &v.snapshot.Satellites[i]
			break
		}
	}
	if s == nil {
		simpleui.DrawText("Click a satellite on the map or in the catalog", b.X+14, b.Y+48, 13, colors.muted)
		return
	}
	eye := "BELOW THE HORIZON"
	c := colors.muted
	if s.Visible {
		eye = "VISIBLE FROM " + v.snapshot.Station.Name
		c = colors.green
	}
	separator := withAlpha(colors.border, 180)
	for _, x := range []float32{b.X + 286, b.X + 548, b.X + 790} {
		rl.DrawLine(int32(x), int32(b.Y+38), int32(x), int32(b.Y+b.Height-12), separator)
	}

	simpleui.DrawTextStyled(sondeClip(s.Name, 27), b.X+14, b.Y+45, 17, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText(fmt.Sprintf("NORAD %d · %s", s.NORAD, s.Group), b.X+14, b.Y+72, 12, colors.muted)
	simpleui.DrawText(eye, b.X+14, b.Y+105, 12, c)

	x := b.X + 304
	simpleui.DrawText("ORBITAL POSITION", x, b.Y+45, 11, colors.muted)
	simpleui.DrawText(fmt.Sprintf("Latitude      %.4f°", s.Latitude), x, b.Y+72, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("Longitude     %.4f°", s.Longitude), x, b.Y+98, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("Altitude      %.0f km", s.AltitudeKM), x, b.Y+124, 13, colors.text)

	x = b.X + 566
	simpleui.DrawText("FROM "+v.snapshot.Station.Name, x, b.Y+45, 11, colors.muted)
	simpleui.DrawText(fmt.Sprintf("Azimuth       %.1f°", s.Azimuth), x, b.Y+72, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("Elevation    %.1f°", s.Elevation), x, b.Y+98, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("Distance     %.0f km", s.RangeKM), x, b.Y+124, 13, colors.text)

	x = b.X + 808
	simpleui.DrawText("RADIO / TELEMETRY", x, b.Y+45, 11, colors.muted)
	simpleui.DrawText(sondeClip(s.Signal, 28), x, b.Y+72, 13, colors.text)
	simpleui.DrawText("Mode          "+s.Mode, x, b.Y+98, 13, colors.text)
	frequency := "Downlink    --"
	if s.DownlinkHz > 0 {
		frequency = fmt.Sprintf("Downlink    %.6f MHz", float64(s.DownlinkHz)/1e6)
	}
	simpleui.DrawText(frequency, x, b.Y+124, 13, colors.text)
}
