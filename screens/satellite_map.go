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
	simpleui.DrawText(fmt.Sprintf("%d objects · %d visible from %s · %s", len(v.snapshot.Satellites), visible, v.snapshot.Station.Name, v.snapshot.Source), 380, 27, 13, colors.muted)
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
	y := b.Y + 82 - float32(v.scroll*24)
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
	content := int((y - (b.Y + 82)) / 24)
	maxRows := int((b.Height - 85) / 24)
	v.scroll = min(v.scroll, max(0, content-maxRows))
}
func (v *satelliteMap) drawMap(b rl.Rectangle) {
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
	for _, s := range v.snapshot.Satellites {
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
	}
	station := worldProject(v.snapshot.Station.Latitude, v.snapshot.Station.Longitude, b)
	rl.DrawCircleV(station, 5, colors.green)
	simpleui.DrawText(v.snapshot.Station.Name, station.X+8, station.Y-7, 11, colors.text)
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
	simpleui.DrawText("visible from the station", b.X+163, legendY, 11, colors.muted)
	drawVisibilityIndicator(rl.Vector2{X: b.X + 355, Y: legendY + 5}, false)
	simpleui.DrawText("below the horizon", b.X+367, legendY, 11, colors.muted)
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
	eye := "BAJO EL HORIZONTE"
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
	simpleui.DrawText(fmt.Sprintf("Distancia   %.0f km", s.RangeKM), x, b.Y+124, 13, colors.text)

	x = b.X + 808
	simpleui.DrawText("RADIO / TELEMETRY", x, b.Y+45, 11, colors.muted)
	simpleui.DrawText(sondeClip(s.Signal, 28), x, b.Y+72, 13, colors.text)
	simpleui.DrawText("Modo          "+s.Mode, x, b.Y+98, 13, colors.text)
	frequency := "Downlink    --"
	if s.DownlinkHz > 0 {
		frequency = fmt.Sprintf("Downlink    %.6f MHz", float64(s.DownlinkHz)/1e6)
	}
	simpleui.DrawText(frequency, x, b.Y+124, 13, colors.text)
}
