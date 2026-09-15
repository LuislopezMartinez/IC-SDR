package screens

import (
	"context"
	"encoding/json"
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/distance"
	"go-zero/internal/i18n"
	"go-zero/internal/resources"
	"go-zero/simpleui"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type savedDistanceLocation struct {
	Name     string
	Lat, Lon float64
}
type distanceElevationResult struct {
	Generation int
	Values     []*float64
	Err        error
}
type distanceMap struct {
	lat, lon, span               float64
	pins                         [2]distance.Point
	drag                         int
	last                         rl.Vector2
	panel                        rl.Vector2
	collapsed, expanded, library bool
	manager                      *simpleui.Manager
	fields                       [3]*simpleui.TextField
	locationFields               [3]*simpleui.TextField
	locations                    []savedDistanceLocation
	selected, offset             int
	message                      string
	locationsPath                string
	elevations                   []*float64
	generation                   int
	due, lastQuery               time.Time
	loading                      bool
	cancel                       context.CancelFunc
	results                      chan distanceElevationResult
	cache                        map[string][]*float64
}

var distanceViewer *exec.Cmd
var distanceViewerDone chan struct{}

func OpenDistanceMap() error {
	if distanceViewer != nil && distanceViewer.Process != nil {
		select {
		case <-distanceViewerDone:
			distanceViewer = nil
		default:
			focusRTL433Viewer(distanceViewer.Process.Pid)
			return nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--distance-map")
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = cmd.Start(); err != nil {
		return err
	}
	distanceViewer = cmd
	done := make(chan struct{})
	distanceViewerDone = done
	go func() { _ = cmd.Wait(); close(done) }()
	return nil
}
func newDistanceMap() *distanceMap {
	v := &distanceMap{lat: 40.5, lon: -3.7, span: 2, pins: [2]distance.Point{{Lat: 40.4168, Lon: -3.7038}, {Lat: 40.65, Lon: -3.4}}, drag: -1, panel: rl.Vector2{X: 18, Y: 18}, selected: -1, manager: simpleui.NewManager(), results: make(chan distanceElevationResult, 8), cache: map[string][]*float64{}}
	for i, value := range []string{"144.8", "10", "10"} {
		f := simpleui.NewTextField(fmt.Sprintf("distanceRadio%d", i), 0, 0, 115, 34, "", 14)
		f.SetText(value)
		f.SetMaxLength(12)
		v.fields[i] = f
		v.manager.Add(f)
	}
	for i, placeholder := range []string{i18n.Source("text.562bb15757a8"), i18n.Source("text.b7a91311229d"), i18n.Source("text.dc8528ecd0fc")} {
		f := simpleui.NewTextField(fmt.Sprintf("distanceLocation%d", i), 0, 0, 150, 34, placeholder, 14)
		f.SetMaxLength(80)
		v.locationFields[i] = f
		v.manager.Add(f)
	}
	if data, err := os.ReadFile(resources.WritablePath("config", "distance-locations.json")); err == nil {
		_ = json.Unmarshal(data, &v.locations)
	}
	v.changed()
	return v
}
func RunDistanceMap() {
	simpleui.SetMode(1440, 900, simpleui.Stretch)
	simpleui.SetMinimumSize(960, 600)
	simpleui.SetTitle(i18n.Source("text.9d04fc08fba5"))
	simpleui.SetTextScale(1.15)
	v := newDistanceMap()
	defer func() {
		if v.cancel != nil {
			v.cancel()
		}
	}()
	simpleui.Run(v.draw)
}
func (v *distanceMap) changed() {
	v.generation++
	v.elevations = nil
	v.loading = false
	if v.cancel != nil {
		v.cancel()
	}
	v.due = time.Now().Add(900 * time.Millisecond)
}
func (v *distanceMap) query() {
	for {
		select {
		case r := <-v.results:
			if r.Generation == v.generation {
				v.loading = false
				v.elevations = r.Values
				if r.Err != nil {
					v.message = i18n.Source("text.dd9aecfab1e3")
				} else {
					v.message = ""
					if len(v.cache) >= 256 {
						v.cache = map[string][]*float64{}
					}
					v.cache[v.key()] = r.Values
				}
			}
		default:
			goto drained
		}
	}
drained:
	if v.loading || v.due.IsZero() || time.Now().Before(v.due) || v.drag >= 0 {
		return
	}
	if e, ok := v.cache[v.key()]; ok {
		v.elevations = e
		v.due = time.Time{}
		return
	}
	if time.Since(v.lastQuery) < time.Second {
		return
	}
	v.due = time.Time{}
	v.loading = true
	v.lastQuery = time.Now()
	gen := v.generation
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	v.cancel = cancel
	points := distance.Samples(v.pins[0], v.pins[1], 81)
	go func() {
		defer cancel()
		e, err := distance.Elevations(ctx, &http.Client{Timeout: 12 * time.Second}, points)
		select {
		case v.results <- distanceElevationResult{gen, e, err}:
		case <-ctx.Done():
			if err != nil {
				select {
				case v.results <- distanceElevationResult{gen, nil, err}:
				default:
				}
			}
		}
	}()
}
func (v *distanceMap) key() string {
	return fmt.Sprintf("%.6f,%.6f/%.6f,%.6f", v.pins[0].Lat, v.pins[0].Lon, v.pins[1].Lat, v.pins[1].Lon)
}
func (v *distanceMap) panelBounds() rl.Rectangle {
	h := float32(310)
	if v.expanded {
		h = 745
	}
	if v.collapsed {
		h = 44
	}
	return rl.Rectangle{X: v.panel.X, Y: v.panel.Y, Width: 430, Height: h}
}
func (v *distanceMap) layout() {
	for i, f := range v.fields {
		f.SetVisible(v.expanded && !v.collapsed)
		f.SetBounds(rl.Rectangle{X: v.panel.X + 14 + float32(i)*134, Y: v.panel.Y + 352, Width: 120, Height: 34})
	}
	for i, f := range v.locationFields {
		f.SetVisible(v.library)
		f.SetBounds(rl.Rectangle{X: 490 + float32(i)*167, Y: 477, Width: 157, Height: 36})
	}
}
func distanceButton(label string, b rl.Rectangle) bool {
	rl.DrawRectangleRounded(b, .12, 4, colors.blue)
	textColor := simpleui.EnsureTextContrast(colors.text, colors.blue)
	size := int32(16)
	if simpleui.MeasureTextStyled(label, size, simpleui.FontSemiBold).X <= b.Width-12 {
		drawCenteredStyled(label, b, size, simpleui.FontSemiBold, textColor)
	} else {
		words := strings.Fields(i18n.Display(label))
		split := len(words) / 2
		if split > 0 {
			top, bottom := b, b
			top.Height = b.Height / 2
			bottom.Y += top.Height
			bottom.Height = top.Height
			drawCenteredStyled(strings.Join(words[:split], " "), top, size, simpleui.FontSemiBold, textColor)
			drawCenteredStyled(strings.Join(words[split:], " "), bottom, size, simpleui.FontSemiBold, textColor)
		} else {
			drawCenteredStyled(label, b, size, simpleui.FontSemiBold, textColor)
		}
	}
	return rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(simpleui.MousePosition(), b)
}
func (v *distanceMap) input(b rl.Rectangle) {
	m := simpleui.MousePosition()
	pb := v.panelBounds()
	lib := rl.Rectangle{X: 475, Y: 60, Width: 535, Height: 550}
	over := rl.CheckCollisionPointRec(m, pb) || (v.library && rl.CheckCollisionPointRec(m, lib))
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		v.last = m
		if rl.CheckCollisionPointRec(m, rl.Rectangle{X: pb.X, Y: pb.Y, Width: pb.Width - 45, Height: 40}) {
			v.drag = 3
		} else if !over {
			v.drag = 2
			for i, p := range v.pins {
				q := projectMercator(v.lat, v.lon, v.span, p.Lat, p.Lon, b)
				if rl.Vector2Distance(m, rl.Vector2{X: q.X, Y: q.Y - 23}) < 25 {
					v.drag = i
					break
				}
			}
		}
	}
	if v.drag >= 0 && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		d := rl.Vector2Subtract(m, v.last)
		switch v.drag {
		case 0, 1:
			lat, lon := unprojectMercator(v.lat, v.lon, v.span, m, b)
			v.pins[v.drag] = distance.Point{Lat: math.Max(-mercatorMaxLat, math.Min(mercatorMaxLat, lat)), Lon: lon}
			v.changed()
		case 2:
			v.lat, v.lon = panMap(v.lat, v.lon, v.span, d, b)
		case 3:
			v.panel.X = math32Clamp(v.panel.X+d.X, 0, 1440-430)
			v.panel.Y = math32Clamp(v.panel.Y+d.Y, 0, 900-v.panelBounds().Height)
		}
		v.last = m
	}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		v.drag = -1
	}
	if w := rl.GetMouseWheelMove(); w != 0 && !over {
		v.lat, v.lon, v.span = zoomMap(v.lat, v.lon, v.span, m, b, w)
	}
	v.lat, v.lon, v.span = clampMapView(v.lat, v.lon, v.span, b)
	v.layout()
	v.manager.Update(simpleui.Input{Pointer: m, PointerInCanvas: true, Pressed: rl.IsMouseButtonPressed(rl.MouseButtonLeft), Down: rl.IsMouseButtonDown(rl.MouseButtonLeft), Released: rl.IsMouseButtonReleased(rl.MouseButtonLeft)})
}
func math32Clamp(x, lo, hi float32) float32 { return min(max(x, lo), hi) }
func (v *distanceMap) draw() {
	b := rl.Rectangle{Width: 1440, Height: 900}
	v.input(b)
	v.query()
	drawGeoMap(v.lat, v.lon, v.span, b)
	path := distance.Samples(v.pins[0], v.pins[1], 81)
	for i := 1; i < len(path); i++ {
		if math.Abs(path[i].Lon-path[i-1].Lon) > 180 {
			continue
		}
		a, z := path[i-1], path[i]
		rl.DrawLineEx(projectMercator(v.lat, v.lon, v.span, a.Lat, a.Lon, b), projectMercator(v.lat, v.lon, v.span, z.Lat, z.Lon, b), 3, colors.orange)
	}
	for i, p := range v.pins {
		q := projectMercator(v.lat, v.lon, v.span, p.Lat, p.Lon, b)
		c := colors.cyan
		if i == 1 {
			c = colors.orange
		}
		rl.DrawLineEx(q, rl.Vector2{X: q.X, Y: q.Y - 23}, 4, mapCalloutEdge)
		rl.DrawCircleV(rl.Vector2{X: q.X, Y: q.Y - 23}, 15, mapCalloutEdge)
		rl.DrawCircleV(rl.Vector2{X: q.X, Y: q.Y - 23}, 12, c)
		simpleui.DrawText(string(rune('A'+i)), q.X-5, q.Y-31, 16, mapCalloutText)
	}
	v.drawPanel()
	if v.library {
		v.drawLibrary()
	}
	v.layout()
	v.manager.Draw()
	simpleui.DrawText(i18n.Source("text.6ec2a8e7cdac"), 12, 880, 10, mapCalloutText)
}
func (v *distanceMap) line(s string, y float32) {
	simpleui.DrawText(s, v.panel.X+14, v.panel.Y+y, 14, colors.text)
}
func (v *distanceMap) drawPanel() {
	pb := v.panelBounds()
	drawPanel(pb.X, pb.Y, pb.Width, pb.Height)
	v.line(i18n.Source("text.0ecb9f4b22e5"), 13)
	toggle := "-"
	if v.collapsed {
		toggle = "+"
	}
	if distanceButton(toggle, rl.Rectangle{X: pb.X + 383, Y: pb.Y + 6, Width: 40, Height: 30}) {
		v.collapsed = !v.collapsed
	}
	if v.collapsed {
		return
	}
	d := distance.Distance(v.pins[0], v.pins[1])
	v.line(fmt.Sprintf(i18n.Source("text.5a98d3f6e20b"), d/1000, d), 53)
	v.line(fmt.Sprintf(i18n.Source("text.f4de56d37c48"), d/1852), 78)
	if d > .01 {
		v.line(fmt.Sprintf(i18n.Source("text.7dd118dc2bb8"), distance.Bearing(v.pins[0], v.pins[1]), distance.Bearing(v.pins[1], v.pins[0])), 103)
	} else {
		v.line(i18n.Source("text.2ab7f9b10595"), 103)
	}
	for i, p := range v.pins {
		alt := i18n.Source("text.b781f6bd9b98")
		if len(v.elevations) > 0 {
			j := 0
			if i == 1 {
				j = len(v.elevations) - 1
			}
			if v.elevations[j] != nil {
				alt = fmt.Sprintf("%.1f m", *v.elevations[j])
			}
		}
		v.line(fmt.Sprintf("%c: %.5f, %.5f · %s", 'A'+i, p.Lat, p.Lon, distance.Locator(p)), 135+float32(i)*45)
		v.line(i18n.Source("text.b8aee5a60063")+alt, 156+float32(i)*45)
	}
	if len(v.elevations) > 0 && v.elevations[0] != nil && v.elevations[len(v.elevations)-1] != nil {
		v.line(fmt.Sprintf(i18n.Source("text.d2317ec4971d"), *v.elevations[len(v.elevations)-1]-*v.elevations[0]), 225)
	} else if v.loading {
		v.line(i18n.Source("text.fbe6aec0af8c"), 225)
	} else {
		v.line(i18n.Source("text.619f531b1747"), 225)
	}
	if distanceButton(i18n.Source("text.2e6f0b5cef98"), rl.Rectangle{X: pb.X + 12, Y: pb.Y + 261, Width: 135, Height: 42}) {
		v.expanded = !v.expanded
		v.panel.Y = min(v.panel.Y, 900-v.panelBounds().Height)
	}
	if distanceButton(i18n.Source("text.f2f6d7256e7e"), rl.Rectangle{X: pb.X + 154, Y: pb.Y + 261, Width: 125, Height: 42}) {
		v.library = !v.library
	}
	if distanceButton(i18n.Source("text.a9254c5f8128"), rl.Rectangle{X: pb.X + 286, Y: pb.Y + 261, Width: 130, Height: 42}) {
		delete(v.cache, v.key())
		v.changed()
	}
	if !v.expanded {
		return
	}
	v.line(i18n.Source("text.160042442991"), 326)
	vals := [3]float64{}
	valid := true
	for i, f := range v.fields {
		n, err := strconv.ParseFloat(strings.ReplaceAll(f.Text(), ",", "."), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			valid = false
		}
		vals[i] = n
	}
	if vals[0] <= 0 {
		valid = false
	}
	if !valid || d <= .01 {
		v.line(i18n.Source("text.0e86f222faae"), 402)
		return
	}
	v.line(fmt.Sprintf(i18n.Source("text.9ec1bd436665"), distance.FSPL(d, vals[0])), 402)
	r := distance.Evaluate(v.elevations, d, vals[0], vals[1], vals[2])
	if r.Valid {
		status := i18n.Source("text.f690038d32b2")
		if !r.Clear {
			status = i18n.Source("text.65074d5af894")
		}
		v.line(i18n.Source("text.da53e0117f18")+status, 429)
		v.line(fmt.Sprintf(i18n.Source("text.d3aff3ba6afd"), r.MinLOS), 454)
		v.line(fmt.Sprintf(i18n.Source("text.e925afbba5e7"), r.MinFresnel), 479)
		v.line(fmt.Sprintf(i18n.Source("text.1e1721d0b2fb"), r.Radius), 504)
	} else {
		v.line(i18n.Source("text.16db741d1425"), 429)
	}
	v.profile(rl.Rectangle{X: pb.X + 18, Y: pb.Y + 543, Width: 394, Height: 130}, d, vals)
	v.line(i18n.Source("text.5a1bf7f02c98"), 686)
	v.line(i18n.Source("text.68eafa7ce00e"), 711)
}
func (v *distanceMap) profile(b rl.Rectangle, d float64, vals [3]float64) {
	rl.BeginScissorMode(int32(b.X), int32(b.Y), int32(b.Width), int32(b.Height))
	defer rl.EndScissorMode()
	rl.DrawRectangleRec(b, colors.panelAlt)
	if len(v.elevations) < 2 {
		simpleui.DrawText(i18n.Source("text.53922a46cc1f"), b.X+8, b.Y+50, 13, colors.muted)
		return
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, e := range v.elevations {
		if e != nil {
			t := float64(i) / float64(len(v.elevations)-1)
			h := *e + d*d*t*(1-t)/(2*distance.EarthRadius*4/3)
			lo = math.Min(lo, h)
			hi = math.Max(hi, h)
		}
	}
	if math.IsInf(lo, 0) {
		return
	}
	if v.elevations[0] != nil {
		hi = math.Max(hi, *v.elevations[0]+vals[1])
	}
	if v.elevations[len(v.elevations)-1] != nil {
		hi = math.Max(hi, *v.elevations[len(v.elevations)-1]+vals[2])
	}
	span := math.Max(10, hi-lo) * 1.15
	pt := func(i int, h float64) rl.Vector2 {
		return rl.Vector2{X: b.X + float32(i)*b.Width/float32(len(v.elevations)-1), Y: b.Y + b.Height - 10 - float32((h-lo)/span)*(b.Height-25)}
	}
	for i := 1; i < len(v.elevations); i++ {
		a, z := v.elevations[i-1], v.elevations[i]
		if a == nil || z == nil {
			continue
		}
		ta, tz := float64(i-1)/float64(len(v.elevations)-1), float64(i)/float64(len(v.elevations)-1)
		rl.DrawLineEx(pt(i-1, *a+d*d*ta*(1-ta)/(2*distance.EarthRadius*4/3)), pt(i, *z+d*d*tz*(1-tz)/(2*distance.EarthRadius*4/3)), 2, colors.green)
	}
	if a, z := v.elevations[0], v.elevations[len(v.elevations)-1]; a != nil && z != nil {
		rl.DrawLineEx(pt(0, *a+vals[1]), pt(len(v.elevations)-1, *z+vals[2]), 2, colors.orange)
		for i := 1; i < len(v.elevations); i++ {
			t0, t1 := float64(i-1)/float64(len(v.elevations)-1), float64(i)/float64(len(v.elevations)-1)
			h0 := *a + vals[1] + (*z+vals[2]-*a-vals[1])*t0 - .6*math.Sqrt((299.792458/vals[0])*d*t0*(1-t0))
			h1 := *a + vals[1] + (*z+vals[2]-*a-vals[1])*t1 - .6*math.Sqrt((299.792458/vals[0])*d*t1*(1-t1))
			rl.DrawLineEx(pt(i-1, h0), pt(i, h1), 1, colors.cyan)
		}

	}
	simpleui.DrawText(fmt.Sprintf("%.0f–%.0f m · %.1f km", lo, hi, d/1000), b.X+5, b.Y+3, 11, colors.text)
	simpleui.DrawText(i18n.Source("text.e9aa69efa6e1"), b.X+5, b.Y+18, 10, colors.text)
}
func (v *distanceMap) persist() {
	data, err := json.MarshalIndent(v.locations, "", "  ")
	if err == nil {
		p := v.locationsPath
		if p == "" {
			p = resources.WritablePath("config", "distance-locations.json")
		}
		err = os.MkdirAll(filepath.Dir(p), 0755)
		if err == nil {
			err = replaceLiveSnapshot(p, data)
		}
	}
	if err != nil {
		v.message = i18n.Source("text.a1bd63755b51")
	} else {
		v.message = i18n.Source("text.2ab402b105ed")
	}
}
func (v *distanceMap) locationInput() (savedDistanceLocation, bool) {
	name := strings.TrimSpace(v.locationFields[0].Text())
	lat, e1 := strconv.ParseFloat(strings.ReplaceAll(v.locationFields[1].Text(), ",", "."), 64)
	lon, e2 := strconv.ParseFloat(strings.ReplaceAll(v.locationFields[2].Text(), ",", "."), 64)
	ok := name != "" && e1 == nil && e2 == nil && !math.IsNaN(lat) && !math.IsNaN(lon) && math.Abs(lat) <= 85.05112878 && math.Abs(lon) <= 180
	if !ok {
		v.message = i18n.Source("text.977e43259b61")
	}
	return savedDistanceLocation{name, lat, lon}, ok
}
func (v *distanceMap) drawLibrary() {
	drawPanel(475, 60, 535, 550)
	simpleui.DrawText(i18n.Source("text.2e5b92fe7ec8"), 490, 77, 19, colors.cyan)
	if distanceButton(i18n.Source("text.aeccae342e4b"), rl.Rectangle{X: 920, Y: 72, Width: 75, Height: 30}) {
		v.library = false
	}
	m := simpleui.MousePosition()
	if rl.CheckCollisionPointRec(m, rl.Rectangle{X: 490, Y: 120, Width: 505, Height: 330}) {
		v.offset = min(max(v.offset-int(rl.GetMouseWheelMove()), 0), max(0, len(v.locations)-7))
	}
	for i := v.offset; i < min(len(v.locations), v.offset+7); i++ {
		l := v.locations[i]
		b := rl.Rectangle{X: 490, Y: 120 + float32(i-v.offset)*46, Width: 505, Height: 40}
		c := colors.panelAlt
		if i == v.selected {
			c = colors.blue
		}
		rl.DrawRectangleRec(b, c)
		simpleui.DrawText(sondeClip(l.Name, 34), b.X+6, b.Y+4, 14, colors.text)
		simpleui.DrawText(fmt.Sprintf("%.5f, %.5f", l.Lat, l.Lon), b.X+6, b.Y+23, 11, colors.muted)
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(m, b) {
			v.selected = i
			v.locationFields[0].SetText(l.Name)
			v.locationFields[1].SetText(fmt.Sprintf("%.6f", l.Lat))
			v.locationFields[2].SetText(fmt.Sprintf("%.6f", l.Lon))
		}
	}
	simpleui.DrawText(i18n.Source("text.1bcd6293cc90"), 490, 453, 13, colors.text)
	labels := []string{i18n.Source("text.7542a5e800f9"), i18n.Source("text.5fcb58c53538"), i18n.Source("text.c9894cf002f9"), i18n.Source("text.30ba4618f876"), i18n.Source("text.22e776c4b59d")}
	buttonX := float32(490)
	widths := []float32{78, 146, 84, 82, 82}
	for i, s := range labels {
		b := rl.Rectangle{X: buttonX, Y: 527, Width: widths[i], Height: 38}
		buttonX += widths[i] + 7
		if distanceButton(s, b) {
			if i == 2 {
				if v.selected >= 0 && v.selected < len(v.locations) {
					v.locations = append(v.locations[:v.selected], v.locations[v.selected+1:]...)
					v.selected = -1
					v.offset = 0
					v.persist()
				}
				continue
			}
			l, ok := v.locationInput()
			if !ok {
				continue
			}
			switch i {
			case 0:
				v.locations = append(v.locations, l)
				v.selected = len(v.locations) - 1
				v.offset = max(0, len(v.locations)-7)
				v.persist()
			case 1:
				if v.selected >= 0 && v.selected < len(v.locations) {
					v.locations[v.selected] = l
					v.persist()
				} else {
					v.message = i18n.Source("text.117cf2fe3cb6")
				}
			case 3, 4:
				v.pins[i-3] = distance.Point{Lat: l.Lat, Lon: l.Lon}
				v.lat, v.lon = l.Lat, l.Lon
				v.changed()
			}
		}
	}
	if distanceButton(i18n.Source("text.18191652edba"), rl.Rectangle{X: 490, Y: 570, Width: 115, Height: 34}) {
		v.locationFields[1].SetText(fmt.Sprintf("%.6f", v.pins[0].Lat))
		v.locationFields[2].SetText(fmt.Sprintf("%.6f", v.pins[0].Lon))
	}
	if distanceButton(i18n.Source("text.8c0adb91a90d"), rl.Rectangle{X: 615, Y: 570, Width: 115, Height: 34}) {
		v.locationFields[1].SetText(fmt.Sprintf("%.6f", v.pins[1].Lat))
		v.locationFields[2].SetText(fmt.Sprintf("%.6f", v.pins[1].Lon))
	}
	simpleui.DrawText(sondeClip(v.message, 32), 745, 578, 11, colors.muted)
}
