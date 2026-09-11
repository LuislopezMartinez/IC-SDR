package screens

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/ais"
	"go-zero/simpleui"
)

//go:embed assets/maps/ais-world.png
var aisWorldPNG []byte

func RunAISMap(path string) {
	simpleui.SetMode(1360, 800, simpleui.Fit)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.25)
	simpleui.SetTitle("IC-SDR · Mapa AIS")
	simpleui.SetMinimumSize(900, 540)
	v := &aisMap{path: path, centerLat: 40.2, centerLon: -3.7, lonSpan: 14, selected: -1, tracks: make(map[uint32][]geoPoint)}
	center := simpleui.NewButton("aisCenterFleet", 1040, 18, 145, 42, "CENTER FLEET", 13)
	center.SetColors(colors.panelAlt, colors.border, colors.text)
	center.OnClick(func() { v.fit(); v.fitted = true })
	world := simpleui.NewButton("aisWorld", 1200, 18, 130, 42, "WORLD VIEW", 13)
	world.SetColors(colors.panelAlt, colors.border, colors.text)
	world.OnClick(func() { v.centerLat, v.centerLon, v.lonSpan = 15, 0, 260 })
	simpleui.Add(center)
	simpleui.Add(world)
	simpleui.Run(v.draw)
}

type geoPoint struct{ lat, lon float64 }
type aisMap struct {
	path                          string
	vessels                       []ais.Vessel
	next                          time.Time
	centerLat, centerLon, lonSpan float64
	selected                      int
	selectedMMSI                  uint32
	fitted, dragging              bool
	dragOrigin, lastMouse         rl.Vector2
	tracks                        map[uint32][]geoPoint
	mapTexture                    rl.Texture2D
}

func (v *aisMap) ensureTexture() {
	if v.mapTexture.ID != 0 {
		return
	}
	img := rl.LoadImageFromMemory(".png", aisWorldPNG, int32(len(aisWorldPNG)))
	if img != nil && img.Data != nil {
		v.mapTexture = rl.LoadTextureFromImage(img)
		rl.UnloadImage(img)
		rl.SetTextureFilter(v.mapTexture, rl.FilterBilinear)
	}
}

func (v *aisMap) read() {
	if time.Now().Before(v.next) {
		return
	}
	v.next = time.Now().Add(250 * time.Millisecond)
	data, err := os.ReadFile(v.path)
	if err != nil {
		return
	}
	var vessels []ais.Vessel
	if json.Unmarshal(data, &vessels) != nil {
		return
	}
	selectedMMSI := v.selectedMMSI
	if selectedMMSI == 0 && v.selected >= 0 && v.selected < len(v.vessels) {
		selectedMMSI = v.vessels[v.selected].MMSI
	}
	v.vessels = vessels
	v.selected = -1
	if selectedMMSI != 0 {
		for i := range vessels {
			if vessels[i].MMSI == selectedMMSI {
				v.selected = i
				v.selectedMMSI = selectedMMSI
				break
			}
		}
	}
	for _, s := range vessels {
		if s.Latitude == nil || s.Longitude == nil {
			continue
		}
		p := geoPoint{*s.Latitude, *s.Longitude}
		track := v.tracks[s.MMSI]
		if len(track) == 0 || math.Abs(track[len(track)-1].lat-p.lat) > .00005 || math.Abs(track[len(track)-1].lon-p.lon) > .00005 {
			track = append(track, p)
			if len(track) > 80 {
				track = track[len(track)-80:]
			}
			v.tracks[s.MMSI] = track
		}
	}
	if !v.fitted && len(vessels) > 0 {
		v.fit()
		v.fitted = true
	}
}

func (v *aisMap) fit() {
	minLat, maxLat, minLon, maxLon := 90., -90., 180., -180.
	n := 0
	for _, s := range v.vessels {
		if s.Latitude == nil || s.Longitude == nil {
			continue
		}
		minLat = math.Min(minLat, *s.Latitude)
		maxLat = math.Max(maxLat, *s.Latitude)
		minLon = math.Min(minLon, *s.Longitude)
		maxLon = math.Max(maxLon, *s.Longitude)
		n++
	}
	if n == 0 {
		return
	}
	v.centerLat, v.centerLon = (minLat+maxLat)/2, (minLon+maxLon)/2
	v.lonSpan = math.Max(.35, math.Max((maxLon-minLon)*1.7, (maxLat-minLat)*2.6))
	v.clampView()
}

func (v *aisMap) latSpan(b rl.Rectangle) float64 { return v.lonSpan * float64(b.Height/b.Width) }
func (v *aisMap) clampView() {
	v.lonSpan = math.Max(.08, math.Min(360, v.lonSpan))
	latSpan := v.lonSpan * 690 / 970
	if latSpan > 180 {
		v.lonSpan = 180 * 970 / 690
		latSpan = 180
	}
	v.centerLat = math.Max(-90+latSpan/2, math.Min(90-latSpan/2, v.centerLat))
	v.centerLon = math.Max(-180+v.lonSpan/2, math.Min(180-v.lonSpan/2, v.centerLon))
}
func (v *aisMap) project(lat, lon float64, b rl.Rectangle) rl.Vector2 {
	latSpan := v.latSpan(b)
	return rl.Vector2{X: b.X + float32((lon-(v.centerLon-v.lonSpan/2))/v.lonSpan)*b.Width, Y: b.Y + float32(((v.centerLat+latSpan/2)-lat)/latSpan)*b.Height}
}
func (v *aisMap) unproject(p rl.Vector2, b rl.Rectangle) (float64, float64) {
	latSpan := v.latSpan(b)
	return v.centerLat + latSpan/2 - float64((p.Y-b.Y)/b.Height)*latSpan, v.centerLon - v.lonSpan/2 + float64((p.X-b.X)/b.Width)*v.lonSpan
}

func (v *aisMap) drawMap(b rl.Rectangle) {
	v.ensureTexture()
	if v.mapTexture.ID == 0 {
		rl.DrawRectangleRec(b, rl.Color{R: 7, G: 31, B: 48, A: 255})
		return
	}
	latSpan := v.latSpan(b)
	src := rl.Rectangle{X: float32((v.centerLon-v.lonSpan/2+180)/360) * float32(v.mapTexture.Width), Y: float32((90-(v.centerLat+latSpan/2))/180) * float32(v.mapTexture.Height), Width: float32(v.lonSpan/360) * float32(v.mapTexture.Width), Height: float32(latSpan/180) * float32(v.mapTexture.Height)}
	rl.DrawTexturePro(v.mapTexture, src, b, rl.Vector2{}, 0, rl.White)
}

func (v *aisMap) input(b rl.Rectangle) {
	m := rl.GetMousePosition()
	inside := rl.CheckCollisionPointRec(m, b)
	if wheel := rl.GetMouseWheelMove(); wheel != 0 {
		v.zoomAt(m, b, wheel)
	}
	if inside && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		v.dragging = true
		v.dragOrigin = m
		v.lastMouse = m
	}
	if v.dragging && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		d := rl.Vector2Subtract(m, v.lastMouse)
		v.centerLon -= float64(d.X) / float64(b.Width) * v.lonSpan
		v.centerLat += float64(d.Y) / float64(b.Height) * v.latSpan(b)
		v.lastMouse = m
		v.clampView()
	}
	if v.dragging && rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		if rl.Vector2Distance(v.dragOrigin, m) < 10 {
			v.selectAt(m, b)
		}
		v.dragging = false
	}
}
func (v *aisMap) zoomAt(m rl.Vector2, b rl.Rectangle, wheel float32) {
	anchor := m
	if !rl.CheckCollisionPointRec(anchor, b) {
		anchor = rl.Vector2{X: b.X + b.Width/2, Y: b.Y + b.Height/2}
	}
	lat, lon := v.unproject(anchor, b)
	v.lonSpan *= math.Pow(.78, float64(wheel))
	v.clampView()
	lat2, lon2 := v.unproject(anchor, b)
	v.centerLat += lat - lat2
	v.centerLon += lon - lon2
	v.clampView()
}
func (v *aisMap) selectAt(m rl.Vector2, b rl.Rectangle) {
	best, bestD := -1, float32(1e9)
	for i, s := range v.vessels {
		if s.Latitude == nil || s.Longitude == nil {
			continue
		}
		p := v.project(*s.Latitude, *s.Longitude, b)
		d := rl.Vector2Distance(p, m)
		// The marker and its text form one generous click target. This remains
		// usable at high DPI and when several reports are close together.
		markerHit := d <= 32
		labelHit := m.X >= p.X-8 && m.X <= p.X+155 && m.Y >= p.Y-18 && m.Y <= p.Y+18
		if (markerHit || labelHit) && d < bestD {
			best, bestD = i, d
		}
	}
	if best >= 0 {
		v.selected = best
		v.selectedMMSI = v.vessels[best].MMSI
	}
}

func (v *aisMap) drawGrid(b rl.Rectangle) {
	latSpan := v.latSpan(b)
	for i := 0; i <= 10; i++ {
		x := b.X + b.Width*float32(i)/10
		rl.DrawLine(int32(x), int32(b.Y), int32(x), int32(b.Y+b.Height), rl.Color{R: 80, G: 145, B: 160, A: 40})
		lon := v.centerLon - v.lonSpan/2 + v.lonSpan*float64(i)/10
		simpleui.DrawText(fmt.Sprintf("%.2f°", lon), x+2, b.Y+b.Height-18, 9, colors.muted)
	}
	for i := 0; i <= 8; i++ {
		y := b.Y + b.Height*float32(i)/8
		rl.DrawLine(int32(b.X), int32(y), int32(b.X+b.Width), int32(y), rl.Color{R: 80, G: 145, B: 160, A: 40})
		lat := v.centerLat + latSpan/2 - latSpan*float64(i)/8
		simpleui.DrawText(fmt.Sprintf("%.2f°", lat), b.X+3, y+2, 9, colors.muted)
	}
}
func (v *aisMap) drawScale(b rl.Rectangle) {
	nmPerPixel := v.lonSpan * 60 * math.Max(.15, math.Cos(v.centerLat*math.Pi/180)) / float64(b.Width)
	target := nmPerPixel * 140
	pow := math.Pow(10, math.Floor(math.Log10(target)))
	scale := pow
	for _, m := range []float64{1, 2, 5, 10} {
		if m*pow <= target {
			scale = m * pow
		}
	}
	w := float32(scale / nmPerPixel)
	x, y := b.X+25, b.Y+b.Height-42
	rl.DrawLineEx(rl.Vector2{X: x, Y: y}, rl.Vector2{X: x + w, Y: y}, 3, colors.text)
	rl.DrawLine(int32(x), int32(y-5), int32(x), int32(y+5), colors.text)
	rl.DrawLine(int32(x+w), int32(y-5), int32(x+w), int32(y+5), colors.text)
	simpleui.DrawText(fmt.Sprintf("%.0f NM", scale), x, y-22, 10, colors.text)
}

func (v *aisMap) draw() {
	v.read()
	rl.ClearBackground(rl.Color{R: 6, G: 12, B: 19, A: 255})
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	v.input(b)
	v.drawMap(b)
	v.drawGrid(b)
	rl.DrawRectangleLinesEx(b, 2, colors.border)
	for _, track := range v.tracks {
		for i := 1; i < len(track); i++ {
			rl.DrawLineEx(v.project(track[i-1].lat, track[i-1].lon, b), v.project(track[i].lat, track[i].lon, b), 1.5, rl.Color{R: 80, G: 210, B: 225, A: 105})
		}
	}
	for i, s := range v.vessels {
		if s.Latitude == nil || s.Longitude == nil {
			continue
		}
		p := v.project(*s.Latitude, *s.Longitude, b)
		if !rl.CheckCollisionPointRec(p, b) {
			continue
		}
		c := colors.cyan
		if time.Since(s.LastSeen) > 5*time.Minute {
			c = colors.muted
		}
		heading := float32(0)
		if s.Course != nil {
			heading = float32(*s.Course)
		}
		radius := float32(10)
		if i == v.selected {
			radius = 14
			rl.DrawCircleLines(int32(p.X), int32(p.Y), 20, colors.orange)
		}
		rl.DrawPoly(p, 3, radius, heading, c)
		name := s.Name
		if name == "" {
			name = fmt.Sprintf("%09d", s.MMSI)
		}
		simpleui.DrawText(sondeClip(name, 19), p.X+12, p.Y-7, 10, colors.text)
	}
	v.drawScale(b)
	simpleui.DrawText("MAPA AIS EN VIVO", 24, 20, 24, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%d ships with signal · drag to pan · wheel to zoom", len(v.vessels)), 310, 29, 13, colors.muted)
	v.drawDetails()
	simpleui.DrawText("Natural Earth · built-in cartography · no Internet connection", 1015, 742, 9, colors.muted)
}

func (v *aisMap) drawDetails() {
	x := float32(1015)
	simpleui.DrawText("DETALLE DEL BARCO", x, 88, 14, colors.orange)
	if v.selected < 0 || v.selected >= len(v.vessels) {
		simpleui.DrawText("Click a ship on the map", x, 125, 13, colors.muted)
		simpleui.DrawText("to inspect its details.", x, 148, 13, colors.muted)
		return
	}
	s := v.vessels[v.selected]
	name := s.Name
	if name == "" {
		name = "UNNAMED"
	}
	value := func(p *float64, format string) string {
		if p == nil {
			return "--"
		}
		return fmt.Sprintf(format, *p)
	}
	age := time.Since(s.LastSeen).Round(time.Second)
	lines := []struct{ label, val string }{{"NAME", name}, {"MMSI", fmt.Sprintf("%09d", s.MMSI)}, {"CALLSIGN", s.Callsign}, {"TYPE", s.ShipTypeText}, {"STATUS", s.StatusText}, {"LATITUDE", value(s.Latitude, "%.6f°")}, {"LONGITUDE", value(s.Longitude, "%.6f°")}, {"SPEED", value(s.Speed, "%.1f kn")}, {"COG", value(s.Course, "%.1f°")}, {"HDG", value(s.Heading, "%.0f°")}, {"DESTINATION", s.Destination}, {"MESSAGES", fmt.Sprintf("%d", s.Messages)}, {"UPDATED", age.String() + " ago"}}
	for i, line := range lines {
		y := float32(125 + i*42)
		simpleui.DrawText(line.label, x, y, 9, colors.muted)
		val := line.val
		if val == "" {
			val = "--"
		}
		simpleui.DrawText(sondeClip(val, 29), x, y+15, 13, colors.text)
	}
}
