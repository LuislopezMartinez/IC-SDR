package screens

import (
	"go-zero/internal/i18n"

	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	simpleui.SetTitle(i18n.Source("text.89a7a72d31ea"))
	simpleui.SetMinimumSize(960, 560)
	v := &satelliteMap{path: path, enabled: map[int]bool{}, expanded: map[string]bool{i18n.Source("text.12ed7b219e4d"): true, "Radioaficionados": true, i18n.Source("text.a5e950a77b49"): true}, selected: 25544}
	v.search = simpleui.NewTextField("satelliteMapSearch", 28, 105, 296, 34, i18n.Source("text.577eace9bdbc"), 12)
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
	p := projectMercator(0, 0, 360, lat, lon, b)
	// Preserve both edges of the world when drawing parallels and trajectories.
	p.X = b.X + b.Width/2 + float32(lon/360*math.Exp2(mapZoom(360, b.Width))*256)
	return p
}
func worldUnproject(p rl.Vector2, b rl.Rectangle) (lat, lon float64) {
	if b.Width <= 0 || b.Height <= 0 {
		return 0, 0
	}
	return unprojectMercator(0, 0, 360, p, b)
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
	simpleui.DrawTextStyled(i18n.Source("text.a7278267cc2b"), 20, 19, 23, simpleui.FontSemiBold, colors.cyan)
	visible := 0
	for _, s := range v.snapshot.Satellites {
		if s.Visible {
			visible++
		}
	}
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.05cc284fd358"), len(v.snapshot.Satellites), visible, v.snapshot.Station.Name, v.snapshot.Source), 380, 27, 13, colors.muted)
}
func (v *satelliteMap) grouped() ([]string, map[string][]satellite.State) {
	m := map[string][]satellite.State{}
	for _, s := range v.snapshot.Satellites {
		if v.query != "" {
			haystack := strings.ToLower(fmt.Sprintf(i18n.Source("text.d5789ceb5d45"), s.Name, s.Group, s.NORAD))
			if !strings.Contains(haystack, v.query) {
				continue
			}
		}
		m[s.Group] = append(m[s.Group], s)
	}
	order := []string{i18n.Source("text.12ed7b219e4d"), "Radioaficionados", "CubeSats", i18n.Source("text.a5e950a77b49"), i18n.Source("text.176c7866b945"), "Galileo", i18n.Source("text.af67a0dd11d7"), "BeiDou", i18n.Source("text.71cbe8f23926"), "Orbcomm", "Starlink"}
	return order, m
}
func (v *satelliteMap) drawList(b rl.Rectangle) {
	simpleui.DrawTextStyled(i18n.Source("text.a9f55fbb5a9f"), b.X+12, b.Y+10, 15, simpleui.FontSemiBold, colors.orange)
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
	drawGeoMap(0, 0, 360, b)
	rl.BeginScissorMode(int32(b.X), int32(b.Y), int32(b.Width), int32(b.Height))
	defer rl.EndScissorMode()
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
		rl.DrawCircleV(p, 8, mapCalloutEdge)
		rl.DrawCircleV(p, 6, c)
		drawMapCallout(sondeClip(s.Name, 18), p.X+9, p.Y-7)
		if s.NORAD == v.selected {
			selectedState, selectedPoint = s, p
		}
	}
	station := worldProject(v.snapshot.Station.Latitude, v.snapshot.Station.Longitude, b)
	rl.DrawCircleV(station, 5, colors.green)
	drawMapCallout(v.snapshot.Station.Name, station.X+8, station.Y-7)
	if selectedState != nil {
		v.drawPassCard(b, selectedPoint, selectedState.NextPass)
	}
	simpleui.DrawText(i18n.Source("text.6ec2a8e7cdac"), b.X+8, b.Y+b.Height-44, 9, colors.text)
	rl.DrawRectangleLinesEx(b, 2, colors.border)
	legendBar := rl.Rectangle{X: b.X + 1, Y: b.Y + b.Height - 28, Width: b.Width - 2, Height: 27}
	legendBackground := colors.panel
	legendBackground.A = 232
	rl.DrawRectangleRec(legendBar, legendBackground)
	rl.DrawLine(int32(legendBar.X), int32(legendBar.Y), int32(legendBar.X+legendBar.Width), int32(legendBar.Y), colors.border)
	legendY := legendBar.Y + 7
	drawMapToggle(rl.Vector2{X: b.X + 7, Y: legendY + 5}, true)
	simpleui.DrawText("mostrar/ocultar", b.X+19, legendY, 11, colors.muted)
	drawVisibilityIndicator(rl.Vector2{X: b.X + 151, Y: legendY + 5}, true)
	simpleui.DrawText(i18n.Source("text.3923c5ec5c41"), b.X+163, legendY, 11, colors.muted)
	drawVisibilityIndicator(rl.Vector2{X: b.X + 355, Y: legendY + 5}, false)
	simpleui.DrawText(i18n.Source("text.1a620700ec39"), b.X+367, legendY, 11, colors.muted)
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
	for _, s := range v.snapshot.Satellites {
		if !v.enabled[s.NORAD] {
			continue
		}
		p := worldProject(s.Latitude, s.Longitude, b)
		if math.Hypot(float64(mouse.X-p.X), float64(mouse.Y-p.Y)) < 14 {
			return
		}
		if s.NORAD == v.selected && rl.CheckCollisionPointRec(mouse, satellitePassCardBounds(b, p)) {
			return
		}
	}
	lat, lon := worldUnproject(mouse, b)
	station := v.snapshot.Station
	if station.Name == "" || (station.Name == "Madrid" && math.Abs(station.Latitude-40.4168) < .02 && math.Abs(station.Longitude+3.7038) < .02) {
		station.Name = i18n.Source("text.647a2fb71d48")
	}
	station.Latitude, station.Longitude = lat, lon
	data, err := json.MarshalIndent(station, "", "  ")
	if err != nil {
		return
	}
	path := resources.WritablePath("config", "satellite-station.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return
	}
	v.snapshot.Station = station
}

func satellitePassCardBounds(b rl.Rectangle, target rl.Vector2) rl.Rectangle {
	const width, height = float32(244), float32(132)
	usableBottom := b.Y + b.Height - 36
	x, y := target.X+18, target.Y+18
	if x+width > b.X+b.Width-8 {
		x = target.X - width - 18
	}
	if y+height > usableBottom {
		y = target.Y - height - 18
	}
	x = min(max(x, b.X+8), b.X+b.Width-width-8)
	y = min(max(y, b.Y+8), usableBottom-height)
	return rl.Rectangle{X: x, Y: y, Width: width, Height: height}
}

func (v *satelliteMap) drawPassCard(mapBounds rl.Rectangle, target rl.Vector2, pass satellite.PassPrediction) {
	card := satellitePassCardBounds(mapBounds, target)

	anchor := rl.Vector2{X: min(max(target.X, card.X), card.X+card.Width), Y: min(max(target.Y, card.Y), card.Y+card.Height)}
	rl.DrawLineEx(target, anchor, 2, colors.orange)
	background := colors.panel
	background.A = 245
	rl.DrawRectangleRounded(card, .06, 8, background)
	rl.DrawRectangleRoundedLinesEx(card, .06, 8, 2, colors.orange)

	simpleui.DrawTextStyled(i18n.Source("text.0db7a75b4034"), card.X+13, card.Y+10, 13, simpleui.FontSemiBold, colors.orange)
	if pass.Continuous {
		status := i18n.Source("text.45c87b23947c")
		if !pass.InProgress {
			status = i18n.Source("text.f87a02debadd")
		}
		simpleui.DrawTextStyled(status, card.X+13, card.Y+43, 14, simpleui.FontSemiBold, colors.text)
		simpleui.DrawText(i18n.Source("text.be000ed0287e"), card.X+13, card.Y+72, 11, colors.muted)
		return
	}
	if !pass.Found {
		simpleui.DrawTextStyled(i18n.Source("text.ba7b723e1b81"), card.X+13, card.Y+43, 14, simpleui.FontSemiBold, colors.text)
		simpleui.DrawText(i18n.Source("text.9c0cca241d13"), card.X+13, card.Y+72, 11, colors.muted)
		return
	}

	now := v.snapshot.Updated.Local()
	tca := pass.TCA.Local()
	simpleui.DrawTextStyled(passDayLabel(now, tca)+" · "+tca.Format("15:04")+" "+localZoneLabel(tca), card.X+13, card.Y+32, 14, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText(passCountdown(v.snapshot.Updated, pass.TCA, pass.InProgress), card.X+13, card.Y+55, 11, colors.text)
	rl.DrawLine(int32(card.X+12), int32(card.Y+75), int32(card.X+card.Width-12), int32(card.Y+75), withAlpha(colors.border, 180))
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.d2df23d2db9c"), pass.MinRangeKM), card.X+13, card.Y+82, 11, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.aa7199f6c9c6"), pass.MaxElevation), card.X+13, card.Y+101, 11, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.5c7dfb58ff04"), pass.AOS.Local().Format("15:04"), pass.LOS.Local().Format("15:04")), card.X+13, card.Y+119, 10, colors.muted)
}

func passDayLabel(now, event time.Time) string {
	y1, m1, d1 := now.Date()
	y2, m2, d2 := event.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return i18n.Source("text.98282ed20269")
	}
	tomorrow := now.AddDate(0, 0, 1)
	y3, m3, d3 := tomorrow.Date()
	if y2 == y3 && m2 == m3 && d2 == d3 {
		return i18n.Source("text.7355e5d68b41")
	}
	return event.Format("02/01")
}

func localZoneLabel(at time.Time) string {
	zone, _ := at.Zone()
	if zone == "" {
		return i18n.Source("text.646c19373ac9")
	}
	return zone
}

func passCountdown(now, event time.Time, inProgress bool) string {
	if inProgress {
		return i18n.Source("text.90eacd81e049")
	}
	minutes := int(math.Round(event.Sub(now).Minutes()))
	if minutes <= 0 {
		return i18n.Source("text.e2a1179886b9")
	}
	if minutes < 60 {
		return fmt.Sprintf(i18n.Source("text.d1a7c522ad62"), minutes)
	}
	return fmt.Sprintf(i18n.Source("text.ab7f32869147"), minutes/60, minutes%60)
}
func (v *satelliteMap) drawDetails(b rl.Rectangle) {
	simpleui.DrawTextStyled(i18n.Source("text.14075bbe5cce"), b.X+14, b.Y+10, 14, simpleui.FontSemiBold, colors.orange)
	var s *satellite.State
	for i := range v.snapshot.Satellites {
		if v.snapshot.Satellites[i].NORAD == v.selected {
			s = &v.snapshot.Satellites[i]
			break
		}
	}
	if s == nil {
		simpleui.DrawText(i18n.Source("text.148f81691b98"), b.X+14, b.Y+48, 13, colors.muted)
		return
	}
	eye := i18n.Source("text.f87a02debadd")
	c := colors.muted
	if s.Visible {
		eye = i18n.Source("text.f250cb959ccb") + v.snapshot.Station.Name
		c = colors.green
	}
	separator := withAlpha(colors.border, 180)
	for _, x := range []float32{b.X + 286, b.X + 548, b.X + 790} {
		rl.DrawLine(int32(x), int32(b.Y+38), int32(x), int32(b.Y+b.Height-12), separator)
	}

	simpleui.DrawTextStyled(sondeClip(s.Name, 27), b.X+14, b.Y+45, 17, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.99d936b58fec"), s.NORAD, s.Group), b.X+14, b.Y+72, 12, colors.muted)
	simpleui.DrawText(eye, b.X+14, b.Y+105, 12, c)

	x := b.X + 304
	simpleui.DrawText(i18n.Source("text.6d1552b81deb"), x, b.Y+45, 11, colors.muted)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.be1b505449fa"), s.Latitude), x, b.Y+72, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.4dca705a264b"), s.Longitude), x, b.Y+98, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.6ee17b4c5e43"), s.AltitudeKM), x, b.Y+124, 13, colors.text)

	x = b.X + 566
	simpleui.DrawText(i18n.Source("text.bb5cc8177156")+v.snapshot.Station.Name, x, b.Y+45, 11, colors.muted)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.a9c797db1add"), s.Azimuth), x, b.Y+72, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.5740705078a0"), s.Elevation), x, b.Y+98, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.64a2c9c90766"), s.RangeKM), x, b.Y+124, 13, colors.text)

	x = b.X + 808
	simpleui.DrawText(i18n.Source("text.bef3d7100e84"), x, b.Y+45, 11, colors.muted)
	simpleui.DrawText(sondeClip(s.Signal, 28), x, b.Y+72, 13, colors.text)
	simpleui.DrawText(i18n.Source("text.9f8d8cfecbfe")+s.Mode, x, b.Y+98, 13, colors.text)
	frequency := i18n.Source("text.6fd9091947db")
	if s.DownlinkHz > 0 {
		frequency = fmt.Sprintf(i18n.Source("text.9b058d6bf831"), float64(s.DownlinkHz)/1e6)
	}
	simpleui.DrawText(frequency, x, b.Y+124, 13, colors.text)
}
