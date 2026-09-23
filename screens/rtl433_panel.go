package screens

import (
	"go-zero/internal/i18n"

	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-zero/internal/resources"
	"go-zero/internal/rtl433"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type RTL433Panel struct {
	screen         *MainScreen
	controls       []simpleui.Element
	presets        []*simpleui.Button
	widthButtons   []*simpleui.Button
	selected       int
	pressedRow     int
	targetHz       int64
	bandwidthHz    int
	snapshotPath   string
	feedback       string
	feedbackUntil  time.Time
	viewer         *exec.Cmd
	viewerDone     chan struct{}
	lastSnapshot   string
	pendingHz      int64
	pending        bool
	pendingApplyAt float64
}

func NewRTL433Panel(screen *MainScreen) *RTL433Panel {
	p := &RTL433Panel{screen: screen, targetHz: 433_920_000, bandwidthHz: 500_000, selected: -1, pressedRow: -1, snapshotPath: resources.WritablePath("cache", "rtl433-captures.json")}
	definitions := []struct {
		label string
		hz    int64
	}{{"315.000", 315_000_000}, {"433.920", 433_920_000}, {"868.300", 868_300_000}, {"915.000", 915_000_000}}
	for i, definition := range definitions {
		button := simpleui.NewButton("rtl433Preset"+strconv.Itoa(i), 40+float32(i)*130, 660, 118, 42, definition.label, uiControlFontSize)
		hz := definition.hz
		button.OnClick(func() { p.selectFrequency(hz, true) })
		p.presets = append(p.presets, button)
		p.controls = append(p.controls, button)
	}
	table := p.button("rtl433Table", 680, 660, 190, 42, i18n.Source("text.a5b86d91647b"), colors.blue)
	table.OnClick(p.openViewer)
	export := p.button("rtl433Export", 885, 660, 180, 42, i18n.Source("text.a15e7868d6b8"), colors.green)
	export.SetColors(colors.green, colors.border, colors.background)
	export.OnClick(p.exportCSV)
	clearButton := p.button("rtl433Clear", 1080, 660, 130, 42, i18n.Source("text.2aded7edd569"), colors.panelAlt)
	clearButton.SetColors(actionClearFill, colors.red, colors.text)
	clearButton.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.ClearRTL433Events()
		}
		p.selected = -1
		p.feedback = i18n.Source("text.3845e3443d3c")
		p.feedbackUntil = time.Now().Add(2 * time.Second)
	})
	p.controls = append(p.controls, table, export, clearButton)
	widths := []struct {
		label string
		hz    int
	}{{"250k", 250_000}, {"500k", 500_000}, {"1M", 1_000_000}, {"2M", 2_000_000}}
	for i, item := range widths {
		button := p.button("rtl433Width"+strconv.Itoa(i), 46+float32(i)*72, 742, 64, 28, item.label, colors.panelAlt)
		width := item.hz
		button.OnClick(func() { p.selectBandwidth(width) })
		p.widthButtons = append(p.widthButtons, button)
		p.controls = append(p.controls, button)
	}
	p.SetVisible(false)
	return p
}
func (p *RTL433Panel) button(id string, x, y, w, h float32, label string, color rl.Color) *simpleui.Button {
	b := simpleui.NewButton(id, x, y, w, h, label, uiControlFontSize)
	b.SetColors(color, colors.border, colors.text)
	return b
}
func (p *RTL433Panel) SetVisible(visible bool) {
	for _, c := range p.controls {
		c.SetVisible(visible)
	}
}

func (p *RTL433Panel) Enter() {
	// Decode the currently tuned frequency. Preset buttons remain the only
	// actions that retune the receiver.
	p.targetHz = p.screen.frequencyHz
	p.screen.rtl433FrequencyHz = p.targetHz
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureRTL433(true, p.targetHz, p.bandwidthHz)
	}
	p.stylePresets()
}
func (p *RTL433Panel) Leave() {
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureRTL433(false, 0)
	}
}

func (p *RTL433Panel) Close() {
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureRTL433(false, 0)
	}
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}
func (p *RTL433Panel) selectFrequency(hz int64, recenterCapture bool) {
	if hz < 1_000 {
		return
	}
	p.targetHz = hz
	p.screen.rtl433FrequencyHz = hz
	p.screen.frequencyHz = hz
	if recenterCapture || p.screen.centerMode {
		p.screen.centerFrequencyHz = hz
		p.screen.spanHz = max(p.screen.spanHz, max(int64(1_000_000), int64(p.bandwidthHz)))
	} else {
		p.screen.centerFrequencyHz = fixedCenterForRTL433(p.screen.centerFrequencyHz, hz, p.screen.spanHz)
	}
	if p.screen.receiver != nil {
		p.screen.receiver.SetCenterFrequency(p.screen.centerFrequencyHz)
		p.screen.receiver.SetDemodulator(p.screen.mode.SelectedText(), hz, p.screen.demodBandwidthHz)
		p.screen.receiver.ConfigureRTL433(true, hz, p.bandwidthHz)
	}
	if p.screen.waterfall != nil {
		p.screen.waterfall.Reset()
	}
	p.screen.markSettingsDirty()
	p.stylePresets()
	p.styleWidths()
	p.pending = false
}
func (p *RTL433Panel) selectBandwidth(width int) {
	p.bandwidthHz = width
	p.screen.rtl433BandwidthHz = width
	p.screen.spanHz = max(p.screen.spanHz, max(int64(1_000_000), int64(width)))
	p.selectFrequency(p.targetHz, false)
	p.feedback = i18n.Source("text.6a15345420ac") + rtl433WidthLabel(width)
	p.feedbackUntil = time.Now().Add(2 * time.Second)
}
func (p *RTL433Panel) styleWidths() {
	widths := []int{250_000, 500_000, 1_000_000, 2_000_000}
	for i, b := range p.widthButtons {
		fill := colors.panelAlt
		if widths[i] == p.bandwidthHz {
			fill = rl.Color{R: 68, G: 49, B: 92, A: 255}
		}
		b.SetColors(fill, rl.Color{R: 175, G: 145, B: 245, A: 255}, colors.text)
	}
}
func rtl433WidthLabel(width int) string {
	switch width {
	case 250_000:
		return i18n.Source("text.8b2aabf1dd4a")
	case 500_000:
		return i18n.Source("text.dcad5c981797")
	case 1_000_000:
		return i18n.Source("text.7a2ef656896a")
	case 2_000_000:
		return i18n.Source("text.6f129691b06f")
	}
	return fmt.Sprintf(i18n.Source("text.a47fa5c4f793"), width/1000)
}
func (p *RTL433Panel) stylePresets() {
	freqs := []int64{315_000_000, 433_920_000, 868_300_000, 915_000_000}
	for i, b := range p.presets {
		fill := colors.panelAlt
		if p.targetHz == freqs[i] {
			fill = rl.Color{R: 68, G: 49, B: 92, A: 255}
		}
		b.SetColors(fill, rl.Color{R: 160, G: 115, B: 225, A: 255}, colors.text)
	}
}

func (p *RTL433Panel) Tick() {
	remoteLocked := p.screen.webServer != nil && p.screen.webServer.RemoteActive()
	if remoteLocked {
		p.pending = false
	}
	if p.viewerDone != nil {
		select {
		case <-p.viewerDone:
			p.viewer, p.viewerDone = nil, nil
		default:
		}
	}
	if p.screen.activeTool == i18n.Source("text.8be70e7cb2c4") && p.pending {
		if rl.IsKeyPressed(rl.KeyEscape) {
			p.pending = false
			p.feedback = i18n.Source("text.c140aaefea05")
			p.feedbackUntil = time.Now().Add(2 * time.Second)
		} else if rl.GetTime() >= p.pendingApplyAt {
			p.selectFrequency(p.pendingHz, false)
			p.feedback = i18n.Source("text.76ed2f1acd7c")
			p.feedbackUntil = time.Now().Add(2 * time.Second)
		}
	}
	// Frequency digits and the header wheel are shared controls. Convert any
	// change they make into the same delayed preview used by the FFT gestures.
	if p.screen.activeTool == i18n.Source("text.8be70e7cb2c4") && !remoteLocked && !p.pending && p.screen.frequencyHz != p.targetHz {
		candidate := p.screen.frequencyHz
		p.screen.frequencyHz = p.targetHz
		if p.screen.receiver != nil {
			p.screen.receiver.SetCenterFrequency(p.targetHz)
			p.screen.receiver.SetDemodulator(p.screen.mode.SelectedText(), p.targetHz, p.screen.demodBandwidthHz)
		}
		p.previewFrequency(candidate)
	}
	if p.screen.activeTool != i18n.Source("text.8be70e7cb2c4") || p.screen.viewMode != 1 || p.screen.overlayOpen() {
		p.pressedRow = -1
		return
	}
	events := p.events()
	p.writeSnapshot(events)
	if controlCopyPressed() && p.selected >= 0 && p.selected < len(events) {
		rl.SetClipboardText(rtl433ClipboardText(events[p.selected]))
		p.feedback = i18n.Source("text.9ea4cb382871")
		p.feedbackUntil = time.Now().Add(2 * time.Second)
	}
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		p.pressedRow = rtl433TableRowAt(simpleui.MousePosition(), len(events))
	}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		if row := rtl433TableRowAt(simpleui.MousePosition(), len(events)); row >= 0 && row == p.pressedRow {
			p.selected = row
			simpleui.PlayActivationFeedback()
		}
		p.pressedRow = -1
	}
}

func fixedCenterForRTL433(centerHz, targetHz, spanHz int64) int64 {
	if spanHz <= 0 {
		return targetHz
	}
	// FIX describes the RF capture position, independently of the bandwidth
	// requested by rtl_433. Using half of the decoder width as an edge guard
	// made narrow spans recentre on every tune because the guard consumed the
	// entire visible interval. Keep the capture stationary while the selected
	// carrier remains visible, and pan only the minimum amount once it leaves.
	halfSpan := spanHz / 2
	if targetHz < centerHz-halfSpan {
		return targetHz + halfSpan
	}
	if targetHz > centerHz+halfSpan {
		return targetHz - halfSpan
	}
	return centerHz
}

func rtl433TableRowAt(mouse rl.Vector2, count int) int {
	for row := range min(count, 4) {
		bounds := rl.Rectangle{X: 357, Y: 744 + float32(row)*14, Width: 840, Height: 14}
		if mouse.X >= bounds.X && mouse.X < bounds.X+bounds.Width && mouse.Y >= bounds.Y && mouse.Y < bounds.Y+bounds.Height {
			return row
		}
	}
	return -1
}

func controlCopyPressed() bool {
	control := rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)
	return control && rl.IsKeyPressed(rl.KeyC)
}

func rtl433ClipboardText(event rtl433.Event) string {
	if event.Raw != "" {
		return event.Raw
	}
	data, err := json.Marshal(event)
	if err != nil {
		return ""
	}
	return string(data)
}

func (p *RTL433Panel) previewFrequency(hz int64) {
	step := max(p.screen.activeTuningStepHz(), int64(1))
	hz = max(int64(math.Round(float64(hz)/float64(step)))*step, 1_000)
	p.pendingHz, p.pending, p.pendingApplyAt = hz, true, rl.GetTime()+.4
}

// HandleSpectrumInput owns tuning gestures while RTL_433 is selected so the
// decoder is restarted only after the user pauses, not for every wheel notch.
func (p *RTL433Panel) HandleSpectrumInput(mouse rl.Vector2, x, y, w, h float32) bool {
	if p.screen.activeTool != i18n.Source("text.8be70e7cb2c4") || p.screen.overlayOpen() || mouse.X < x || mouse.X > x+w || mouse.Y < y || mouse.Y > y+h {
		return false
	}
	if steps := wheelSteps(rl.GetMouseWheelMove()); steps != 0 {
		base := p.targetHz
		if p.pending {
			base = p.pendingHz
		}
		p.previewFrequency(base + steps*p.screen.activeTuningStepHz())
	}
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		fraction := min(max((mouse.X-x)/w, 0), 1)
		hz := p.screen.centerFrequencyHz - p.screen.spanHz/2 + int64(math.Round(float64(fraction)*float64(p.screen.spanHz)))
		p.previewFrequency(hz)
	}
	return true
}
func (p *RTL433Panel) events() []rtl433.Event {
	if p.screen.receiver == nil {
		return nil
	}
	return p.screen.receiver.RTL433Events()
}

func (p *RTL433Panel) DrawPanel() {
	status := rtl433.Status{State: i18n.Source("text.dc2c64e7e3ca")}
	if p.screen.receiver != nil {
		status = p.screen.receiver.RTL433Status()
	}
	simpleui.DrawTextStyled(i18n.Source("text.89c1cd146225"), 40, 638, 16, simpleui.FontSemiBold, rl.Color{R: 175, G: 145, B: 245, A: 255})
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.e86583683f38"), status.State, status.SampleRate/1000, status.Events, status.Dropped), 700, 638, 12, simpleui.FontSemiBold, colors.muted)
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.d3fc384b301d"), float64(p.targetHz)/1e6), 1230, 666, 14, simpleui.FontSemiBold, colors.orange)
	drawPanel(40, 716, 300, 90)
	simpleui.DrawTextStyled(i18n.Source("text.f1a93e4d1c90"), 52, 719, 12, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(rtl433WidthLabel(p.bandwidthHz), 52, 781, 12, colors.text)
	events := p.events()
	drawPanel(350, 716, 855, 90)
	headers := []struct {
		x float32
		t string
	}{{365, i18n.Source("text.e7563517a678")}, {440, i18n.Source("text.5a5b4d7c0e3c")}, {650, "ID"}, {745, i18n.Source("text.4e89bb9f11b4")}, {825, i18n.Source("text.dba4b22edbcd")}, {920, i18n.Source("text.99bf85f9fe95")}, {1025, i18n.Source("text.50d30c134163")}}
	for _, h := range headers {
		simpleui.DrawTextStyled(h.t, h.x, 721, 12, simpleui.FontSemiBold, colors.cyan)
	}
	for row, e := range events[:min(len(events), 4)] {
		y := 746 + float32(row)*14
		if row == p.selected {
			rl.DrawRectangle(357, int32(y-2), 840, 15, rl.Color{R: 25, G: 83, B: 116, A: 220})
		}
		simpleui.DrawTextStyled(e.Received.Format("15:04:05"), 365, y, 12, simpleui.FontMono, colors.text)
		simpleui.DrawText(short(e.Model+" / "+e.Type, 23), 440, y, 12, colors.text)
		simpleui.DrawText(short(e.ID, 10), 650, y, 12, colors.text)
		simpleui.DrawText(short(e.Channel, 7), 745, y, 12, colors.text)
		simpleui.DrawText(fmt.Sprintf("%.3f", e.FreqMHz), 825, y, 12, colors.text)
		simpleui.DrawText(fmt.Sprintf("%.1f/%.1f", e.RSSI, e.SNR), 920, y, 12, colors.text)
		simpleui.DrawText(short(e.Summary, 18), 1025, y, 12, colors.text)
	}
	if p.selected >= 0 && p.selected < len(events) {
		drawPanel(1220, 716, 330, 90)
		simpleui.DrawTextStyled(i18n.Source("text.ac84910ab13b"), 1232, 722, 12, simpleui.FontSemiBold, colors.cyan)
		drawWrapped(short(events[p.selected].Raw, 150), 1232, 746, 305, 12, colors.text)
	}
	if time.Now().Before(p.feedbackUntil) {
		simpleui.DrawTextStyled(p.feedback, 1230, 692, 12, simpleui.FontSemiBold, colors.green)
	}
}

// DrawSpectrumOverlay makes the exact RF interval submitted to rtl_433
// visible in every FFT layout while this decoder owns the active tool.
func (p *RTL433Panel) DrawSpectrumOverlay(x, y, w, h float32) {
	if p.screen.activeTool != i18n.Source("text.8be70e7cb2c4") || p.bandwidthHz <= 0 {
		return
	}
	accent := rl.Color{R: 175, G: 125, B: 245, A: 255}
	drawRange := func(center int64, fillAlpha uint8, lineWidth float32, lineColor rl.Color) (float32, float32) {
		displayLow := p.screen.centerFrequencyHz - p.screen.spanHz/2
		low := center - int64(p.bandwidthHz)/2
		high := center + int64(p.bandwidthHz)/2
		left := max(x, x+w*float32(low-displayLow)/float32(p.screen.spanHz))
		right := min(x+w, x+w*float32(high-displayLow)/float32(p.screen.spanHz))
		if right <= left {
			return left, right
		}
		rl.DrawRectangle(int32(left), int32(y+1), int32(right-left), int32(h-53), rl.Color{R: 95, G: 55, B: 145, A: fillAlpha})
		rl.DrawLineEx(rl.Vector2{X: left, Y: y + 1}, rl.Vector2{X: left, Y: y + h - 52}, lineWidth, lineColor)
		rl.DrawLineEx(rl.Vector2{X: right, Y: y + 1}, rl.Vector2{X: right, Y: y + h - 52}, lineWidth, lineColor)
		return left, right
	}
	alpha := uint8(32)
	lineColor := accent
	if p.pending {
		alpha = 12
		lineColor = rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 105}
	}
	left, right := drawRange(p.targetHz, alpha, 2, lineColor)
	if right <= left {
		return
	}
	label := i18n.Source("text.bb34b5fe6a67") + rtl433WidthLabel(p.bandwidthHz)
	tw := simpleui.MeasureTextStyled(label, 12, simpleui.FontSemiBold).X
	// The decoder owns the first overlay lane. The tuning plate uses the lane
	// immediately below it, so both labels remain readable even when centered
	// on exactly the same frequency.
	labelX := min(max((left+right-tw)/2, x+120), x+w-tw-8)
	rl.DrawRectangleRounded(rl.Rectangle{X: labelX - 6, Y: y + 4, Width: tw + 12, Height: 20}, .2, 5, rl.Color{R: 18, G: 11, B: 28, A: 225})
	simpleui.DrawTextStyled(label, labelX, y+7, 12, simpleui.FontSemiBold, accent)
	if p.pending {
		futureLeft, futureRight := drawRange(p.pendingHz, 62, 3, rl.Color{R: 215, G: 180, B: 255, A: 255})
		cursorX := (futureLeft + futureRight) / 2
		rl.DrawLineEx(rl.Vector2{X: cursorX, Y: y + 1}, rl.Vector2{X: cursorX, Y: y + h - 52}, 2, colors.orange)
		delta := p.pendingHz - p.targetHz
		sign := "+"
		if delta < 0 {
			sign = ""
		}
		rangeLow := float64(p.pendingHz-int64(p.bandwidthHz)/2) / 1e6
		rangeHigh := float64(p.pendingHz+int64(p.bandwidthHz)/2) / 1e6
		preview := fmt.Sprintf(i18n.Source("text.6ecc2602ed52"), rangeLow, rangeHigh, sign, formatStep(delta))
		pw := simpleui.MeasureTextStyled(preview, 12, simpleui.FontSemiBold).X
		px := min(max(cursorX-pw/2, x+8), x+w-pw-8)
		// A third lane is reserved for the delayed retune preview.
		rl.DrawRectangleRounded(rl.Rectangle{X: px - 7, Y: y + 81, Width: pw + 14, Height: 22}, .2, 5, rl.Color{R: 35, G: 20, B: 45, A: 235})
		simpleui.DrawTextStyled(preview, px, y+85, 12, simpleui.FontSemiBold, rl.Color{R: 225, G: 195, B: 255, A: 255})
	}
}
func short(s string, n int) string {
	s = i18n.Display(s)
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
func drawWrapped(s string, x, y, w float32, size int32, color rl.Color) {
	s = i18n.Display(s)
	words := strings.Fields(s)
	line := ""
	for _, word := range words {
		next := strings.TrimSpace(line + " " + word)
		if simpleui.MeasureText(next, size).X > w {
			simpleui.DrawText(line, x, y, size, color)
			y += float32(size + 3)
			line = word
		} else {
			line = next
		}
		if y > 795 {
			break
		}
	}
	if line != "" && y <= 795 {
		simpleui.DrawText(line, x, y, size, color)
	}
}

func (p *RTL433Panel) writeSnapshot(events []rtl433.Event) {
	data, _ := json.Marshal(events)
	current := string(data)
	if current == p.lastSnapshot {
		return
	}
	p.lastSnapshot = current
	if replaceLiveSnapshot(p.snapshotPath, data) != nil {
		p.lastSnapshot = ""
	}
}
func (p *RTL433Panel) openViewer() {
	p.writeSnapshot(p.events())
	if p.viewer != nil && p.viewer.Process != nil {
		focusRTL433Viewer(p.viewer.Process.Pid)
		p.feedback = i18n.Source("text.df8ed2b55cc5")
		p.feedbackUntil = time.Now().Add(2 * time.Second)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		p.feedback = i18n.Source("text.21deef5a70b9")
		return
	}
	cmd := exec.Command(executable, "--rtl433-viewer", p.snapshotPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = cmd.Start(); err != nil {
		p.feedback = i18n.Source("text.21deef5a70b9")
	} else {
		p.viewer = cmd
		p.viewerDone = make(chan struct{})
		done := p.viewerDone
		go func() { _ = cmd.Wait(); close(done) }()
		p.feedback = i18n.Source("text.f2d48920d980")
	}
	p.feedbackUntil = time.Now().Add(2 * time.Second)
}
func (p *RTL433Panel) exportCSV() {
	path, err := ExportRTL433CSV(p.events())
	if err != nil {
		p.feedback = i18n.Source("text.f020b3a6a762")
	} else {
		p.feedback = i18n.Source("text.50e2734c33d7") + filepath.Base(path)
	}
	p.feedbackUntil = time.Now().Add(3 * time.Second)
}

func ExportRTL433CSV(events []rtl433.Event) (string, error) {
	dir := resources.WritablePath("exports", "rtl433")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "rtl433-"+time.Now().Format("20060102-150405")+".csv")
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, _ = file.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(file)
	writer.Comma = ';'
	_ = writer.Write([]string{"fecha", "hora", "frecuencia_MHz", "protocolo", "modelo", "tipo", "id", "canal", "modulacion", i18n.Source("text.3f3c5df3dada"), i18n.Source("text.33663bd5c0bf"), "resumen", i18n.Source("text.db1a21a0bc2e")})
	for _, e := range events {
		_ = writer.Write([]string{e.Received.Format("2006-01-02"), e.Received.Format("15:04:05"), fmt.Sprintf("%.6f", e.FreqMHz), strconv.Itoa(e.Protocol), e.Model, e.Type, e.ID, e.Channel, e.Mod, fmt.Sprintf("%.3f", e.RSSI), fmt.Sprintf("%.3f", e.SNR), e.Summary, e.Raw})
	}
	writer.Flush()
	return path, writer.Error()
}
