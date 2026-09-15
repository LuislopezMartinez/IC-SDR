package screens

import (
	"go-zero/internal/i18n"

	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/aprs"
	"go-zero/internal/resources"
	"go-zero/simpleui"
)

type APRSPanel struct {
	screen                   *MainScreen
	controls                 []simpleui.Element
	view                     string
	buttons                  map[string]*simpleui.Button
	scroll, selected         int
	feedback                 string
	feedbackUntil            time.Time
	snapshotPath             string
	lastSnapshot             string
	viewer                   *exec.Cmd
	viewerDone               chan struct{}
	mapViewer                *exec.Cmd
	mapDone                  chan struct{}
	mapPath, lastMapSnapshot string
	stations                 map[string]aprs.MapStation
}

func NewAPRSPanel(screen *MainScreen) *APRSPanel {
	p := &APRSPanel{screen: screen, view: i18n.Source("text.74b8a8ece330"), buttons: map[string]*simpleui.Button{}, selected: -1, snapshotPath: resources.WritablePath("cache", "aprs-captures.json")}
	items := []struct {
		id, label string
		w         float32
		color     rl.Color
	}{{i18n.Source("text.74b8a8ece330"), i18n.Source("text.74b8a8ece330"), 120, colors.blue}, {i18n.Source("text.87073e5d8db0"), i18n.Source("text.87073e5d8db0"), 130, colors.panelAlt}, {i18n.Source("text.fe86cd5572c0"), i18n.Source("text.fe86cd5572c0"), 120, colors.panelAlt}, {i18n.Source("text.ddadd1fb4789"), i18n.Source("text.ddadd1fb4789"), 100, colors.panelAlt}, {i18n.Source("text.ac0562bba4a5"), i18n.Source("text.ac0562bba4a5"), 85, colors.panelAlt}}
	x := float32(40)
	for _, item := range items {
		b := simpleui.NewButton("aprs"+item.id, x, 660, item.w, 40, item.label, uiControlFontSize)
		b.SetColors(item.color, colors.border, colors.text)
		view := item.id
		b.OnClick(func() {
			p.view = view
			p.screen.aprsView = view
			p.scroll = 0
			p.selected = -1
			p.style()
			p.screen.markSettingsDirty()
		})
		p.buttons[item.id] = b
		p.controls = append(p.controls, b)
		x += item.w + 10
	}
	europe := simpleui.NewButton("aprsEurope", 650, 660, 185, 40, i18n.Source("text.6e4f4d0bfe82"), uiControlFontSize)
	europe.SetColors(rl.Color{R: 68, G: 49, B: 92, A: 255}, rl.Color{R: 175, G: 145, B: 245, A: 255}, colors.text)
	europe.OnClick(func() { p.tune(144_800_000) })
	popout := simpleui.NewButton("aprsPopout", 845, 660, 170, 40, i18n.Source("text.a5b86d91647b"), uiControlFontSize)
	popout.SetColors(colors.blue, colors.border, colors.text)
	popout.OnClick(p.openViewer)
	export := simpleui.NewButton("aprsExport", 1025, 660, 180, 40, i18n.Source("text.a15e7868d6b8"), uiControlFontSize)
	export.SetColors(colors.green, colors.border, colors.background)
	export.OnClick(p.export)
	clearButton := simpleui.NewButton("aprsClear", 1215, 660, 125, 40, i18n.Source("text.2aded7edd569"), uiControlFontSize)
	clearButton.SetColors(actionClearFill, colors.red, colors.text)
	clearButton.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.ClearAPRSPackets()
		}
		p.scroll, p.selected = 0, -1
		p.stations = make(map[string]aprs.MapStation)
		p.writeMapSnapshot(nil)
		p.say(i18n.Source("text.381b60f9a630"))
	})
	p.mapPath = resources.WritablePath("cache", "aprs-map.json")
	p.stations = make(map[string]aprs.MapStation)
	mapButton := simpleui.NewButton("aprsMap", 1350, 660, 200, 40, i18n.Source("text.19bc47933e09"), uiControlFontSize)
	mapButton.SetColors(colors.blue, colors.border, colors.text)
	mapButton.OnClick(p.openMap)
	p.controls = append(p.controls, europe, popout, export, clearButton, mapButton)
	p.SetVisible(false)
	return p
}
func (p *APRSPanel) tune(hz int64) {
	p.screen.frequencyHz, p.screen.centerFrequencyHz, p.screen.centerMode = hz, hz, true
	p.screen.bandCategory, p.screen.bandName = i18n.Source("text.4fae663ae96a"), "2 m"
	if p.screen.band != nil {
		p.screen.band.SetLabel(i18n.Source("text.d7cf93445b12"))
	}
	if p.screen.vfoModeSwitch != nil {
		p.screen.vfoModeSwitch.SetActive(false)
	}
	if p.screen.receiver != nil {
		p.screen.receiver.SetCenterFrequency(hz)
		p.screen.receiver.SetDemodulator(i18n.Source("text.0896d612d497"), hz, p.screen.demodBandwidthHz)
		p.screen.receiver.ConfigureAPRS(true, hz, p.screen.demodBandwidthHz)
	}
	p.screen.waterfall.Reset()
	p.screen.markSettingsDirty()
	p.say(i18n.Source("text.8b44cacab9a7"))
}
func (p *APRSPanel) style() {
	for id, b := range p.buttons {
		fill := colors.panelAlt
		if id == p.view {
			fill = rl.Color{R: 20, G: 100, B: 98, A: 255}
		}
		b.SetColors(fill, colors.cyan, colors.text)
	}
}
func (p *APRSPanel) SetVisible(v bool) {
	for _, c := range p.controls {
		c.SetVisible(v)
	}
	if v {
		p.style()
	}
}
func (p *APRSPanel) Enter() {
	p.screen.selectMode(i18n.Source("text.0896d612d497"))
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureAPRS(true, p.screen.frequencyHz, p.screen.demodBandwidthHz)
	}
}
func (p *APRSPanel) Leave() {
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureAPRS(false, 0, 0)
	}
}
func (p *APRSPanel) Close() {
	if p.mapViewer != nil && p.mapViewer.Process != nil {
		_ = p.mapViewer.Process.Kill()
	}
	p.Leave()
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}
func (p *APRSPanel) packets() []aprs.Packet {
	if p.screen.receiver == nil {
		return nil
	}
	return p.screen.receiver.APRSPackets()
}
func (p *APRSPanel) Tick() {
	if p.mapDone != nil {
		select {
		case <-p.mapDone:
			p.mapViewer, p.mapDone = nil, nil
		default:
		}
	}
	if p.screen.activeTool == i18n.Source("text.4c4310fd27fd") {
		p.writeMapSnapshot(p.packets())
	}
	if p.viewerDone != nil {
		select {
		case <-p.viewerDone:
			p.viewer, p.viewerDone = nil, nil
		default:
		}
	}
	if p.screen.activeTool != i18n.Source("text.4c4310fd27fd") || p.screen.viewMode != 1 || p.screen.overlayOpen() {
		return
	}
	p.writeSnapshot(p.packets())
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureAPRS(true, p.screen.frequencyHz, p.screen.demodBandwidthHz)
	}
	packets := p.filtered()
	visible := 5
	if p.view == i18n.Source("text.ddadd1fb4789") {
		return
	}
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 {
		p.scroll = min(max(p.scroll-int(wheel), 0), max(len(packets)-visible, 0))
	}
	if rl.IsKeyPressed(rl.KeyDown) {
		p.selected = min(p.selected+1, len(packets)-1)
	}
	if rl.IsKeyPressed(rl.KeyUp) {
		p.selected = max(p.selected-1, 0)
	}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		m := simpleui.MousePosition()
		for row := 0; row < visible && p.scroll+row < len(packets); row++ {
			if rl.CheckCollisionPointRec(m, rl.Rectangle{X: 282, Y: 742 + float32(row)*24, Width: 1258, Height: 24}) {
				p.selected = p.scroll + row
				simpleui.PlayActivationFeedback()
			}
		}
	}
}
func (p *APRSPanel) writeSnapshot(packets []aprs.Packet) {
	data, err := json.Marshal(packets)
	if err != nil {
		p.lastSnapshot = ""
		p.say(i18n.Source("text.f992375c8e2d"))
		return
	}
	value := string(data)
	if value == p.lastSnapshot {
		return
	}
	p.lastSnapshot = value
	if replaceLiveSnapshot(p.snapshotPath, data) != nil {
		p.lastSnapshot = ""
	}
}
func (p *APRSPanel) openViewer() {
	p.writeSnapshot(p.packets())
	if p.viewer != nil && p.viewer.Process != nil {
		focusRTL433Viewer(p.viewer.Process.Pid)
		p.say(i18n.Source("text.df8ed2b55cc5"))
		return
	}
	executable, err := os.Executable()
	if err != nil {
		p.say(i18n.Source("text.21deef5a70b9"))
		return
	}
	cmd := exec.Command(executable, "--aprs-viewer", p.snapshotPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = cmd.Start(); err != nil {
		p.say(i18n.Source("text.21deef5a70b9"))
		return
	}
	p.viewer = cmd
	p.viewerDone = make(chan struct{})
	done := p.viewerDone
	go func() { _ = cmd.Wait(); close(done) }()
	p.say(i18n.Source("text.f2d48920d980"))
}
func (p *APRSPanel) filtered() []aprs.Packet {
	packets := p.packets()
	if p.view == i18n.Source("text.fe86cd5572c0") {
		out := packets[:0]
		for _, packet := range packets {
			if packet.Type == i18n.Source("text.b194d92018d6") || packet.Type == i18n.Source("text.a2f1a6d79bfb") || packet.Type == i18n.Source("text.0a8e61fbc428") {
				out = append(out, packet)
			}
		}
		return out
	}
	if p.view == i18n.Source("text.87073e5d8db0") {
		seen := map[string]bool{}
		out := []aprs.Packet{}
		for _, packet := range packets {
			if !seen[packet.Source] {
				seen[packet.Source] = true
				out = append(out, packet)
			}
		}
		return out
	}
	return packets
}
func (p *APRSPanel) DrawPanel() {
	status := aprs.Status{State: i18n.Source("text.67b9e10a1cbd"), AudioLevel: -1}
	if p.screen.receiver != nil {
		status = p.screen.receiver.APRSStatus()
	}
	simpleui.DrawTextStyled(i18n.Source("text.89563b490222"), 40, 638, 16, simpleui.FontSemiBold, rl.Color{R: 55, G: 215, B: 195, A: 255})
	stateColor := colors.orange
	if status.State == i18n.Source("text.3ec713946605") {
		stateColor = colors.green
	} else if status.State == i18n.Source("text.d98ee0e5f939") {
		stateColor = colors.red
	}
	rl.DrawCircle(720, 646, 6, stateColor)
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.7d895cae283e"), status.State, float64(p.screen.frequencyHz)/1e6, levelText(status.AudioLevel), status.PacketCount, status.FrequencyErrorHz), 735, 638, 12, simpleui.FontSemiBold, colors.muted)
	drawPanel(40, 712, 220, 168)
	simpleui.DrawTextStyled(i18n.Source("text.92a8de300175"), 52, 721, 12, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.c84551016365"), map[bool]string{true: i18n.Source("text.5d2bd518ab17"), false: i18n.Source("text.761bca5bfba7")}[status.KISS]), 52, 745, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.78e70e4a8076"), status.Queued, status.Dropped), 52, 766, 12, colors.text)
	simpleui.DrawText(short(status.Detail, 28), 52, 787, 12, colors.muted)
	if p.view == i18n.Source("text.ddadd1fb4789") {
		p.drawRadar()
		return
	}
	p.drawTable(p.filtered())
	if time.Now().Before(p.feedbackUntil) {
		simpleui.DrawTextStyled(p.feedback, 1360, 672, 12, simpleui.FontSemiBold, colors.green)
	}
}
func levelText(v int) string {
	if v < 0 {
		return "—"
	}
	return strconv.Itoa(v)
}
func (p *APRSPanel) drawTable(packets []aprs.Packet) {
	drawPanel(275, 712, 1275, 168)
	headers := []struct {
		x float32
		s string
	}{{288, i18n.Source("text.e7563517a678")}, {365, i18n.Source("text.16d535b5d63f")}, {485, i18n.Source("text.cd4c8ecf7e4b")}, {590, i18n.Source("text.2adab83f1039")}, {700, i18n.Source("text.963552275d93")}, {900, i18n.Source("text.1f6b0a131de2")}, {1470, i18n.Source("text.ad2e66181fd9")}}
	for _, h := range headers {
		simpleui.DrawTextStyled(h.s, h.x, 718, 12, simpleui.FontSemiBold, colors.cyan)
	}
	if len(packets) == 0 {
		simpleui.DrawText(i18n.Source("text.f1f616d2befd"), 730, 760, 14, colors.muted)
		return
	}
	p.scroll = min(p.scroll, max(0, len(packets)-5))
	for row := 0; row < 5 && p.scroll+row < len(packets); row++ {
		index := p.scroll + row
		packet := packets[index]
		y := 746 + float32(row)*24
		if index == p.selected {
			rl.DrawRectangle(282, int32(y-4), 1258, 24, rl.Color{R: 25, G: 83, B: 116, A: 220})
		}
		typeColor := rl.Color{R: 190, G: 145, B: 235, A: 255}
		if packet.Type == i18n.Source("text.aad6faa6418d") {
			typeColor = colors.green
		} else if packet.Type == i18n.Source("text.0a244108837b") {
			typeColor = rl.Color{R: 75, G: 145, B: 255, A: 255}
		} else if packet.Type == i18n.Source("text.b194d92018d6") {
			typeColor = colors.orange
		}
		simpleui.DrawTextStyled(packet.Received.Format("15:04:05"), 288, y, 12, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(short(packet.Source, 13), 365, y, 12, simpleui.FontSemiBold, colors.cyan)
		simpleui.DrawText(packet.Type, 485, y, 12, typeColor)
		simpleui.DrawText(short(packet.Destination, 12), 590, y, 12, colors.text)
		path := packet.Path
		if path == "" {
			path = i18n.Source("text.3d46b2eae4e5")
		}
		drawAPRSCell(path, 700, y, 190, colors.text)
		info := packet.Summary
		if p.view == i18n.Source("text.ac0562bba4a5") {
			info = packet.Raw
		} else if p.view == i18n.Source("text.87073e5d8db0") && packet.Coordinates != "—" {
			info = packet.Coordinates + " · " + packet.Summary
		}
		drawAPRSCell(info, 900, y, 555, colors.text)
		simpleui.DrawText(levelText(packet.ReceiveLevel), 1470, y, 12, colors.text)
	}
	if p.selected >= 0 && p.selected < len(packets) {
		packet := packets[p.selected]
		drawAPRSCell(packet.Raw, 288, 861, 1250, colors.muted)
	}
}

func drawAPRSCell(text string, x, y, width float32, color rl.Color) {
	rl.BeginScissorMode(int32(x), int32(y), int32(width), 20)
	simpleui.DrawText(text, x, y, 12, color)
	rl.EndScissorMode()
}
func (p *APRSPanel) drawRadar() {
	drawPanel(275, 712, 1275, 94)
	packets := p.filteredPositions()
	if len(packets) == 0 {
		simpleui.DrawText(i18n.Source("text.ae39dbc196dd"), 680, 758, 14, colors.muted)
		return
	}
	minLat, maxLat, minLon, maxLon := packets[0].Latitude, packets[0].Latitude, packets[0].Longitude, packets[0].Longitude
	for _, v := range packets {
		minLat = min(minLat, v.Latitude)
		maxLat = max(maxLat, v.Latitude)
		minLon = min(minLon, v.Longitude)
		maxLon = max(maxLon, v.Longitude)
	}
	cx, cy := float32(910), float32(760)
	rl.DrawCircleLines(int32(cx), int32(cy), 37, colors.grid)
	rl.DrawCircleLines(int32(cx), int32(cy), 18, colors.grid)
	rl.DrawLine(int32(cx-45), int32(cy), int32(cx+45), int32(cy), colors.grid)
	rl.DrawLine(int32(cx), int32(cy-42), int32(cx), int32(cy+42), colors.grid)
	latSpan := max(maxLat-minLat, .01)
	lonSpan := max(maxLon-minLon, .01)
	for _, v := range packets {
		x := cx + float32((v.Longitude-(minLon+maxLon)/2)/lonSpan)*80
		y := cy - float32((v.Latitude-(minLat+maxLat)/2)/latSpan)*72
		rl.DrawCircle(int32(x), int32(y), 4, colors.green)
		simpleui.DrawText(v.Source, x+7, y-6, 12, colors.text)
	}
	simpleui.DrawText(i18n.Source("text.919fe96f83a4"), 290, 725, 12, colors.cyan)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.13ffc5803aa0"), len(packets)), 290, 748, 13, colors.text)
	simpleui.DrawText(i18n.Source("text.c2f24debc283"), 290, 772, 12, colors.muted)
}
func (p *APRSPanel) filteredPositions() []aprs.Packet {
	seen := map[string]bool{}
	out := []aprs.Packet{}
	for _, v := range p.packets() {
		if !seen[v.Source] && v.Coordinates != "—" {
			seen[v.Source] = true
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}
func (p *APRSPanel) say(s string) { p.feedback = s; p.feedbackUntil = time.Now().Add(3 * time.Second) }
func (p *APRSPanel) export() {
	path, err := exportAPRSCSV(p.filtered())
	if err != nil {
		p.say(i18n.Source("text.f020b3a6a762"))
	} else {
		p.say(i18n.Source("text.50e2734c33d7") + filepath.Base(path))
	}
}
func exportAPRSCSV(packets []aprs.Packet) (string, error) {
	dir := resources.WritablePath("exports", "aprs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "aprs-"+time.Now().Format("20060102-150405")+".csv")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, _ = f.Write([]byte{0xef, 0xbb, 0xbf})
	w := csv.NewWriter(f)
	w.Comma = ';'
	_ = w.Write(strings.Split("fecha;hora;indicativo;destino;ruta;tipo;simbolo;coordenadas;locator;rumbo;velocidad;altitud;destino_mensaje;id_mensaje;temperatura;humedad;presion;viento;lluvia;nivel;informacion;trama_raw", ";"))
	for _, p := range packets {
		_ = w.Write([]string{p.Received.Format("2006-01-02"), p.Received.Format("15:04:05"), p.Source, p.Destination, p.Path, p.Type, p.Symbol, p.Coordinates, p.Locator, p.Course, p.Speed, p.Altitude, p.MessageTarget, p.MessageID, p.Temperature, p.Humidity, p.Pressure, p.Wind, p.Rain, strconv.Itoa(p.ReceiveLevel), p.Summary, p.Raw})
	}
	w.Flush()
	return path, w.Error()
}
