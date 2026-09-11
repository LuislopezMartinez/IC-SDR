package screens

import (
	"encoding/json"
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/aircraft"
	"go-zero/simpleui"
	"math"
	"os"
	"time"
)

func RunAircraftMap(path string) {
	simpleui.SetMode(1360, 800, simpleui.Fit)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.25)
	simpleui.SetTitle("IC-SDR · Air traffic")
	simpleui.SetMinimumSize(900, 540)
	v := &aircraftMap{path: path, centerLat: 40.2, centerLon: -3.7, lonSpan: 14, selected: -1, tracks: make(map[string][]geoPoint)}
	center := simpleui.NewButton("flightCenter", 1040, 18, 145, 42, "CENTER TRAFFIC", 12)
	center.SetColors(colors.panelAlt, colors.border, colors.text)
	center.OnClick(func() { v.fit(); v.fitted = true })
	world := simpleui.NewButton("flightWorld", 1200, 18, 130, 42, "WORLD VIEW", 13)
	world.SetColors(colors.panelAlt, colors.border, colors.text)
	world.OnClick(func() { v.centerLat, v.centerLon, v.lonSpan = 15, 0, 260 })
	simpleui.Add(center)
	simpleui.Add(world)
	simpleui.Run(v.draw)
}

type aircraftMap struct {
	path                          string
	list                          []aircraft.Aircraft
	next                          time.Time
	centerLat, centerLon, lonSpan float64
	selected                      int
	fitted, dragging              bool
	dragOrigin, lastMouse         rl.Vector2
	tracks                        map[string][]geoPoint
	texture                       rl.Texture2D
}

func (v *aircraftMap) read() {
	if time.Now().Before(v.next) {
		return
	}
	v.next = time.Now().Add(250 * time.Millisecond)
	data, err := os.ReadFile(v.path)
	if err != nil {
		return
	}
	var list []aircraft.Aircraft
	if json.Unmarshal(data, &list) != nil {
		return
	}
	v.list = list
	for _, a := range list {
		if a.Latitude == nil || a.Longitude == nil {
			continue
		}
		p := geoPoint{*a.Latitude, *a.Longitude}
		t := v.tracks[a.ICAO]
		if len(t) == 0 || math.Abs(t[len(t)-1].lat-p.lat) > .0001 || math.Abs(t[len(t)-1].lon-p.lon) > .0001 {
			t = append(t, p)
			if len(t) > 100 {
				t = t[len(t)-100:]
			}
			v.tracks[a.ICAO] = t
		}
	}
	if !v.fitted && len(list) > 0 {
		v.fit()
		v.fitted = true
	}
}
func (v *aircraftMap) fit() {
	minLat, maxLat, minLon, maxLon := 90., -90., 180., -180.
	n := 0
	for _, a := range v.list {
		if a.Latitude == nil || a.Longitude == nil {
			continue
		}
		minLat = math.Min(minLat, *a.Latitude)
		maxLat = math.Max(maxLat, *a.Latitude)
		minLon = math.Min(minLon, *a.Longitude)
		maxLon = math.Max(maxLon, *a.Longitude)
		n++
	}
	if n == 0 {
		return
	}
	v.centerLat, v.centerLon = (minLat+maxLat)/2, (minLon+maxLon)/2
	v.lonSpan = math.Max(.8, math.Max((maxLon-minLon)*1.6, (maxLat-minLat)*2.5))
	v.clamp()
}
func (v *aircraftMap) latSpan(b rl.Rectangle) float64 { return v.lonSpan * float64(b.Height/b.Width) }
func (v *aircraftMap) clamp() {
	v.lonSpan = math.Max(.1, math.Min(260, v.lonSpan))
	ls := v.lonSpan * 690 / 970
	v.centerLat = math.Max(-90+ls/2, math.Min(90-ls/2, v.centerLat))
	v.centerLon = math.Max(-180+v.lonSpan/2, math.Min(180-v.lonSpan/2, v.centerLon))
}
func (v *aircraftMap) project(lat, lon float64, b rl.Rectangle) rl.Vector2 {
	ls := v.latSpan(b)
	return rl.Vector2{X: b.X + float32((lon-v.centerLon+v.lonSpan/2)/v.lonSpan)*b.Width, Y: b.Y + float32((v.centerLat+ls/2-lat)/ls)*b.Height}
}
func (v *aircraftMap) unproject(p rl.Vector2, b rl.Rectangle) (float64, float64) {
	ls := v.latSpan(b)
	return v.centerLat + ls/2 - float64((p.Y-b.Y)/b.Height)*ls, v.centerLon - v.lonSpan/2 + float64((p.X-b.X)/b.Width)*v.lonSpan
}
func (v *aircraftMap) background(b rl.Rectangle) {
	if v.texture.ID == 0 {
		img := rl.LoadImageFromMemory(".png", aisWorldPNG, int32(len(aisWorldPNG)))
		if img != nil && img.Data != nil {
			v.texture = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
			rl.SetTextureFilter(v.texture, rl.FilterBilinear)
		}
	}
	if v.texture.ID == 0 {
		return
	}
	ls := v.latSpan(b)
	src := rl.Rectangle{X: float32((v.centerLon-v.lonSpan/2+180)/360) * float32(v.texture.Width), Y: float32((90-v.centerLat-ls/2)/180) * float32(v.texture.Height), Width: float32(v.lonSpan/360) * float32(v.texture.Width), Height: float32(ls/180) * float32(v.texture.Height)}
	rl.DrawTexturePro(v.texture, src, b, rl.Vector2{}, 0, rl.White)
}
func (v *aircraftMap) input(b rl.Rectangle) {
	m := rl.GetMousePosition()
	inside := rl.CheckCollisionPointRec(m, b)
	if w := rl.GetMouseWheelMove(); w != 0 {
		v.zoomAt(m, b, w)
	}
	if inside && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		v.dragging = true
		v.dragOrigin, v.lastMouse = m, m
	}
	if v.dragging && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		d := rl.Vector2Subtract(m, v.lastMouse)
		v.centerLon -= float64(d.X) / float64(b.Width) * v.lonSpan
		v.centerLat += float64(d.Y) / float64(b.Height) * v.latSpan(b)
		v.lastMouse = m
		v.clamp()
	}
	if v.dragging && rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		if rl.Vector2Distance(v.dragOrigin, m) < 10 {
			v.selectAt(m, b)
		}
		v.dragging = false
	}
}
func (v *aircraftMap) zoomAt(m rl.Vector2, b rl.Rectangle, wheel float32) {
	anchor := m
	if !rl.CheckCollisionPointRec(anchor, b) {
		anchor = rl.Vector2{X: b.X + b.Width/2, Y: b.Y + b.Height/2}
	}
	lat, lon := v.unproject(anchor, b)
	v.lonSpan *= math.Pow(.78, float64(wheel))
	v.clamp()
	lat2, lon2 := v.unproject(anchor, b)
	v.centerLat += lat - lat2
	v.centerLon += lon - lon2
	v.clamp()
}
func (v *aircraftMap) selectAt(m rl.Vector2, b rl.Rectangle) {
	best, dist := -1, float32(1e9)
	for i, a := range v.list {
		if a.Latitude == nil || a.Longitude == nil {
			continue
		}
		p := v.project(*a.Latitude, *a.Longitude, b)
		d := rl.Vector2Distance(p, m)
		markerHit := d <= 34
		labelHit := m.X >= p.X-8 && m.X <= p.X+175 && m.Y >= p.Y-20 && m.Y <= p.Y+20
		if (markerHit || labelHit) && d < dist {
			best, dist = i, d
		}
	}
	if best >= 0 {
		v.selected = best
	}
}
func (v *aircraftMap) grid(b rl.Rectangle) {
	ls := v.latSpan(b)
	for i := 0; i <= 10; i++ {
		x := b.X + b.Width*float32(i)/10
		rl.DrawLine(int32(x), int32(b.Y), int32(x), int32(b.Y+b.Height), rl.Color{R: 90, G: 150, B: 170, A: 38})
		simpleui.DrawText(fmt.Sprintf("%.2f°", v.centerLon-v.lonSpan/2+v.lonSpan*float64(i)/10), x+2, b.Y+b.Height-18, 9, colors.muted)
	}
	for i := 0; i <= 8; i++ {
		y := b.Y + b.Height*float32(i)/8
		rl.DrawLine(int32(b.X), int32(y), int32(b.X+b.Width), int32(y), rl.Color{R: 90, G: 150, B: 170, A: 38})
		simpleui.DrawText(fmt.Sprintf("%.2f°", v.centerLat+ls/2-ls*float64(i)/8), b.X+3, y+2, 9, colors.muted)
	}
}
func (v *aircraftMap) draw() {
	v.read()
	rl.ClearBackground(rl.Color{R: 6, G: 12, B: 19, A: 255})
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	v.input(b)
	v.background(b)
	v.grid(b)
	rl.DrawRectangleLinesEx(b, 2, colors.border)
	for _, t := range v.tracks {
		for i := 1; i < len(t); i++ {
			rl.DrawLineEx(v.project(t[i-1].lat, t[i-1].lon, b), v.project(t[i].lat, t[i].lon, b), 1.5, rl.Color{R: 196, G: 120, B: 255, A: 120})
		}
	}
	for i, a := range v.list {
		if a.Latitude == nil || a.Longitude == nil {
			continue
		}
		p := v.project(*a.Latitude, *a.Longitude, b)
		if !rl.CheckCollisionPointRec(p, b) {
			continue
		}
		c := colors.cyan
		if a.Source == aircraft.Mode978 {
			c = colors.orange
		}
		if time.Since(a.LastSeen) > 60*time.Second {
			c = colors.muted
		}
		angle := float32(0)
		if a.Track != nil {
			angle = float32(*a.Track)
		}
		size := float32(12)
		if i == v.selected {
			size = 16
			rl.DrawCircleLines(int32(p.X), int32(p.Y), 22, colors.orange)
		}
		drawAircraftSymbol(p, size, angle, c)
		label := a.Callsign
		if label == "" {
			label = a.ICAO
		}
		alt := ""
		if a.Altitude != nil {
			alt = fmt.Sprintf(" · %d ft", *a.Altitude)
		}
		simpleui.DrawText(label+alt, p.X+13, p.Y-8, 10, colors.text)
	}
	simpleui.DrawText("LIVE AIR TRAFFIC", 24, 20, 24, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%d aircraft · cyan 1090 · orange 978 · drag and use the wheel", len(v.list)), 340, 29, 13, colors.muted)
	v.details()
	simpleui.DrawText("Natural Earth · positions received directly by radio", 1015, 742, 9, colors.muted)
}
func drawAircraftSymbol(p rl.Vector2, size, angle float32, c rl.Color) {
	r := float64(angle) * math.Pi / 180
	rot := func(x, y float32) rl.Vector2 {
		return rl.Vector2{X: p.X + x*float32(math.Cos(r)) - y*float32(math.Sin(r)), Y: p.Y + x*float32(math.Sin(r)) + y*float32(math.Cos(r))}
	}
	nose := rot(0, -size)
	left := rot(-size*.7, size*.75)
	tail := rot(0, size*.35)
	right := rot(size*.7, size*.75)
	rl.DrawTriangle(nose, left, tail, c)
	rl.DrawTriangle(nose, tail, right, c)
}
func (v *aircraftMap) details() {
	x := float32(1015)
	simpleui.DrawText("DETALLE DE AERONAVE", x, 88, 14, colors.orange)
	if v.selected < 0 || v.selected >= len(v.list) {
		simpleui.DrawText("Click an aircraft", x, 125, 13, colors.muted)
		return
	}
	a := v.list[v.selected]
	val := func(p *float64, f string) string {
		if p == nil {
			return "--"
		}
		return fmt.Sprintf(f, *p)
	}
	alt := "--"
	if a.Altitude != nil {
		alt = fmt.Sprintf("%d ft", *a.Altitude)
	}
	lines := []struct{ l, v string }{{"FLIGHT", a.Callsign}, {"ICAO", a.ICAO}, {"SOURCE", a.Source}, {"LATITUDE", val(a.Latitude, "%.6f°")}, {"LONGITUDE", val(a.Longitude, "%.6f°")}, {"ALTITUDE", alt}, {"SPEED", val(a.Speed, "%.0f kt")}, {"TRACK", val(a.Track, "%.1f°")}, {"VERTICAL SPEED", val(a.VerticalRate, "%.0f ft/min")}, {"SQUAWK", a.Squawk}, {"CATEGORY", a.Category}, {"MESSAGES", fmt.Sprintf("%d", a.Messages)}, {"UPDATED", time.Since(a.LastSeen).Round(time.Second).String() + " ago"}}
	for i, z := range lines {
		y := float32(125 + i*42)
		simpleui.DrawText(z.l, x, y, 9, colors.muted)
		if z.v == "" {
			z.v = "--"
		}
		simpleui.DrawText(sondeClip(z.v, 29), x, y+15, 13, colors.text)
	}
}
