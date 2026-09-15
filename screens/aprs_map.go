package screens

import (
	"go-zero/internal/i18n"

	"encoding/json"
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/aprs"
	"go-zero/simpleui"
	"math"
	"os"
	"strings"
	"time"
)

type aprsMap struct {
	detailScroll     float32
	detailSource     string
	path, selected   string
	stations         []aprs.MapStation
	next             time.Time
	lat, lon, span   float64
	fitted, dragging bool
	origin, last     rl.Vector2
}

func RunAPRSMap(path string) {
	simpleui.SetMode(1360, 800, simpleui.Fit)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.25)
	simpleui.SetTitle(i18n.Source("text.1baf62c8473b"))
	simpleui.SetMinimumSize(900, 540)
	v := &aprsMap{path: path, lat: 40.2, lon: -3.7, span: 14}
	center := simpleui.NewButton("aprsMapCenter", 1030, 18, 170, 42, i18n.Source("text.174e7eeca8a8"), 12)
	center.OnClick(v.fit)
	world := simpleui.NewButton("aprsMapWorld", 1210, 18, 120, 42, i18n.Source("text.4b20060b2d1c"), 12)
	world.OnClick(func() { v.lat, v.lon, v.span = 15, 0, 260 })
	simpleui.Add(center)
	simpleui.Add(world)
	simpleui.Run(v.draw)
}
func (v *aprsMap) read() {
	if time.Now().Before(v.next) {
		return
	}
	v.next = time.Now().Add(250 * time.Millisecond)
	data, err := os.ReadFile(v.path)
	if err != nil {
		return
	}
	var stations []aprs.MapStation
	if json.Unmarshal(data, &stations) != nil {
		return
	}
	v.stations = nil
	for _, s := range stations {
		if s.HasPosition && aprs.ValidMapPosition(s.Packet) {
			v.stations = append(v.stations, s)
		}
	}
	if !v.fitted && len(v.stations) > 0 {
		v.fit()
		v.fitted = true
	}
}
func (v *aprsMap) fit() {
	if len(v.stations) == 0 {
		return
	}
	loLat, hiLat, loLon, hiLon := 90., -90., 180., -180.
	for _, s := range v.stations {
		loLat = math.Min(loLat, s.Latitude)
		hiLat = math.Max(hiLat, s.Latitude)
		loLon = math.Min(loLon, s.Longitude)
		hiLon = math.Max(hiLon, s.Longitude)
	}
	v.lat, v.lon = (loLat+hiLat)/2, (loLon+hiLon)/2
	v.span = math.Max(.35, math.Max((hiLon-loLon)*1.7, (hiLat-loLat)*2.6))
}
func (v *aprsMap) input(b rl.Rectangle) {
	m := simpleui.MousePosition()
	inside := rl.CheckCollisionPointRec(m, b)
	if w := rl.GetMouseWheelMove(); inside && w != 0 {
		v.lat, v.lon, v.span = zoomMap(v.lat, v.lon, v.span, m, b, w)
	}
	if inside && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		v.dragging = true
		v.origin = m
		v.last = m
	}
	if v.dragging && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		v.lat, v.lon = panMap(v.lat, v.lon, v.span, rl.Vector2Subtract(m, v.last), b)
		v.last = m
	}
	if v.dragging && rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		if rl.Vector2Distance(v.origin, m) < 10 {
			best := float32(33)
			for _, s := range v.stations {
				p := projectMercator(v.lat, v.lon, v.span, s.Latitude, s.Longitude, b)
				if d := rl.Vector2Distance(m, p); d < best {
					best = d
					v.selected = s.Source
				}
			}
		}
		v.dragging = false
	}
	v.lat, v.lon, v.span = clampMapView(v.lat, v.lon, v.span, b)
}
func (v *aprsMap) draw() {
	v.read()
	rl.ClearBackground(colors.background)
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	v.input(b)
	drawGeoMap(v.lat, v.lon, v.span, b)
	rl.DrawRectangleLinesEx(b, 2, colors.border)
	rl.BeginScissorMode(20, 78, 970, 690)
	labels := []rl.Rectangle{}
	for _, s := range v.stations {
		p := projectMercator(v.lat, v.lon, v.span, s.Latitude, s.Longitude, b)
		if !rl.CheckCollisionPointRec(p, b) {
			continue
		}
		selected := s.Source == v.selected
		drawAPRSIcon(p, s.Symbol, selected, time.Since(s.Received) > 30*time.Minute)
		label := rl.Rectangle{X: p.X + 18, Y: p.Y - 9, Width: float32(len(s.Source))*8 + 12, Height: 20}
		overlap := false
		for _, old := range labels {
			if rl.CheckCollisionRecs(label, old) {
				overlap = true
				break
			}
		}
		if !overlap || selected {
			drawMapCallout(s.Source, label.X, label.Y)
			labels = append(labels, label)
		}
	}
	rl.EndScissorMode()
	simpleui.DrawText(i18n.Source("text.00227ac3528d"), 24, 20, 24, colors.cyan)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.bf4a68fdefa8"), len(v.stations)), 310, 30, 12, colors.muted)
	v.details()
	simpleui.DrawText(i18n.Source("text.b296ad90963e"), 24, 775, 11, colors.muted)
}
func (v *aprsMap) details() {
	x := float32(1015)
	drawPanel(1005, 78, 335, 690)
	simpleui.DrawTextStyled(i18n.Source("text.e6cf8d6ec7a1"), x, 88, 15, simpleui.FontSemiBold, colors.orange)
	var station *aprs.MapStation
	for i := range v.stations {
		if v.stations[i].Source == v.selected {
			station = &v.stations[i]
			break
		}
	}
	if station == nil {
		simpleui.DrawText(i18n.Source("text.d449476944bd"), x, 125, 13, colors.muted)
		return
	}
	s := station
	if v.detailSource != s.Source {
		v.detailSource = s.Source
		v.detailScroll = 0
	}
	lines := []string{s.Source, i18n.Source("text.a10102cb7850") + aprsIconKind(s.Symbol), i18n.Source("text.c1c7851e2417") + s.Symbol, i18n.Source("text.62b2d218d65b") + s.Received.Local().Format("15:04:05"), i18n.Source("text.c60b53f018cd") + s.PositionTime.Local().Format("15:04:05"), fmt.Sprintf("%.5f, %.5f", s.Latitude, s.Longitude), i18n.Source("text.76f296118a80") + s.Speed, i18n.Source("text.4b2853bdeba4") + s.Course, i18n.Source("text.1c4f486cf5c7") + s.Altitude, i18n.Source("text.e4829bb8192b") + s.Temperature, i18n.Source("text.ec93d33ffe4b") + s.Humidity, i18n.Source("text.6abcf1bfc26e") + s.Pressure, i18n.Source("text.c8193388a6d4") + s.Wind, i18n.Source("text.3d0298f1e7ed") + s.Rain}
	lines = append(lines, "", i18n.Source("text.c0917a9dd342"), s.Telemetry, "", i18n.Source("text.742374013362"), s.Summary, "", i18n.Source("text.8cd073e74898"), s.Raw)
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, wrapAPRSDetails(line, 310, 16)...)
	}
	b := rl.Rectangle{X: 1015, Y: 125, Width: 315, Height: 620}
	if rl.CheckCollisionPointRec(simpleui.MousePosition(), b) {
		v.detailScroll -= rl.GetMouseWheelMove() * 60
	}
	v.detailScroll = float32(math.Max(0, math.Min(float64(v.detailScroll), math.Max(0, float64(len(wrapped)*28)-float64(b.Height)))))
	rl.BeginScissorMode(int32(b.X), int32(b.Y), int32(b.Width), int32(b.Height))
	for i, line := range wrapped {
		y := b.Y + float32(i)*28 - v.detailScroll
		if y+28 < b.Y || y > b.Y+b.Height {
			continue
		}
		simpleui.DrawTextStyled(line, x, y, 16, simpleui.FontRegular, rl.RayWhite)
	}
	rl.EndScissorMode()
	simpleui.DrawText(i18n.Source("text.ab7004e11964"), x, 749, 10, colors.muted)
}

func wrapAPRSDetails(text string, width float32, size int32) []string {
	text = i18n.Display(text)
	var lines []string
	line := ""
	for _, r := range text {
		next := line + string(r)
		if r == '\n' || (line != "" && simpleui.MeasureText(next, size).X > width) {
			lines = append(lines, strings.TrimSpace(line))
			line = ""
			if r == '\n' {
				continue
			}
		}
		line += string(r)
	}
	return append(lines, strings.TrimSpace(line))
}

// Common APRS symbols follow https://www.aprs.org/symbols-win.html.
// Unknown alternate symbols keep a generic marker and their exact code in details.
func aprsIconKind(symbol string) string {
	if len(symbol) != 2 {
		return i18n.Source("text.f328c354bbef")
	}
	table, code := symbol[0], symbol[1]
	if table != '/' {
		switch code {
		case '>':
			return i18n.Source("text.e4579a8b1ea4")
		case 's':
			return "Barco"
		case '_':
			return i18n.Source("text.62e084208a13")
		case '-':
			if table == '\\' {
				return i18n.Source("text.4b1601ee83fa")
			}
		}
		return i18n.Source("text.f328c354bbef")
	}
	switch code {
	case '>', 'j', 'k', 'u', 'v', 'U', 'a', 'f', 'R', '<', 'b':
		return i18n.Source("text.e4579a8b1ea4")
	case '-':
		return i18n.Source("text.4b1601ee83fa")
	case '_', 'W':
		return i18n.Source("text.62e084208a13")
	case '[':
		return "Persona"
	case 's', 'Y', 'C':
		return "Barco"
	case '^', '\'', 'X', 'g':
		return "Aeronave"
	case 'O':
		return "Globo"
	case '#', 'r', '`':
		return "Radio"
	}
	return i18n.Source("text.f328c354bbef")
}
func drawAPRSIcon(p rl.Vector2, symbol string, selected, stale bool) {
	c := rl.Color{R: 20, G: 65, B: 100, A: 255}
	if stale {
		c = rl.Gray
	}
	rl.DrawCircleV(p, 15, rl.Color{R: 20, G: 20, B: 20, A: 255})
	rl.DrawCircleV(p, 13, rl.RayWhite)
	if selected {
		rl.DrawCircleLines(int32(p.X), int32(p.Y), 20, colors.orange)
	}
	line := func(x1, y1, x2, y2 float32) {
		rl.DrawLineEx(rl.Vector2{X: p.X + x1, Y: p.Y + y1}, rl.Vector2{X: p.X + x2, Y: p.Y + y2}, 2, c)
	}
	switch aprsIconKind(symbol) {
	case "Vehículo":
		rl.DrawRectangle(int32(p.X-9), int32(p.Y-2), 18, 7, c)
		line(-6, -2, -3, -7)
		line(-3, -7, 5, -7)
		line(5, -7, 8, -2)
		rl.DrawCircle(int32(p.X-5), int32(p.Y+6), 3, c)
		rl.DrawCircle(int32(p.X+6), int32(p.Y+6), 3, c)
	case "Estación fija":
		line(-9, -1, 0, -9)
		line(0, -9, 9, -1)
		line(-6, -2, -6, 8)
		line(-6, 8, 6, 8)
		line(6, 8, 6, -2)
		line(0, 8, 0, 2)
	case "Meteorología":
		rl.DrawCircle(int32(p.X-4), int32(p.Y-3), 5, c)
		rl.DrawCircle(int32(p.X+3), int32(p.Y-5), 6, c)
		line(-7, 4, -9, 9)
		line(0, 4, -2, 9)
		line(7, 4, 5, 9)
	case "Persona":
		rl.DrawCircle(int32(p.X), int32(p.Y-8), 3, c)
		line(0, -4, 0, 3)
		line(-7, 0, 0, -2)
		line(0, -2, 7, 0)
		line(0, 3, -6, 10)
		line(0, 3, 6, 10)
	case "Barco":
		line(-10, 2, -5, 8)
		line(-5, 8, 5, 8)
		line(5, 8, 10, 2)
		line(-10, 2, 10, 2)
		line(0, 2, 0, -10)
		line(0, -10, 7, -1)
	case "Aeronave":
		line(0, -11, 0, 10)
		line(-10, 1, 0, -3)
		line(0, -3, 10, 1)
		line(-5, 9, 0, 6)
		line(0, 6, 5, 9)
	case "Globo":
		rl.DrawCircle(int32(p.X), int32(p.Y-4), 7, c)
		line(-4, 3, -2, 9)
		line(4, 3, 2, 9)
		line(-2, 9, 2, 9)
	case "Radio":
		line(0, -10, 0, 10)
		line(-7, -7, 0, 0)
		line(7, -7, 0, 0)
		line(-5, 10, 0, 0)
		line(5, 10, 0, 0)
	default:
		rl.DrawCircleV(p, 5, c)
	}
	if len(symbol) == 2 && symbol[0] != '/' && symbol[0] != '\\' {
		simpleui.DrawText(string(symbol[0]), p.X+8, p.Y-15, 10, rl.Black)
	}
}
