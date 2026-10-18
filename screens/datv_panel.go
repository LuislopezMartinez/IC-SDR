package screens

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"go-zero/internal/datv"
	"go-zero/internal/resources"
	"go-zero/internal/vlcembed"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const datvToolID = "DATV"

var datvSymbolRates = []int{33_000, 66_000, 125_000, 250_000, 333_000, 500_000, 1_000_000, 1_500_000}

type DATVPanel struct {
	screen                              *MainScreen
	controls                            []simpleui.Element
	standard, symbolRate, fec, polarity *simpleui.Dropdown
	decode, record                      *simpleui.Button
	targetHz                            int64
	bandwidthHz                         int64
	dragHandle                          int
	feedback                            string
	player                              *vlcembed.Player
}

func NewDATVPanel(screen *MainScreen) *DATVPanel {
	p := &DATVPanel{screen: screen, targetHz: screen.frequencyHz, bandwidthHz: 400_000, player: vlcembed.New()}
	p.standard = simpleui.NewDropdown("datvStandard", 370, toolY+45, 145, 36, "ESTÁNDAR", []string{"AUTO", "DVB-S2", "DVB-S"}, uiMinimumFontSize)
	p.standard.SetSelected(0)
	rates := make([]string, len(datvSymbolRates))
	for i, v := range datvSymbolRates {
		rates[i] = formatSymbolRate(v)
	}
	p.symbolRate = simpleui.NewDropdown("datvSymbolRate", 530, toolY+45, 145, 36, "SYMBOL RATE", rates, uiMinimumFontSize)
	p.symbolRate.SetSelected(4)
	p.symbolRate.OnChange(func(i int, _ string) { p.bandwidthHz = int64(float64(datvSymbolRates[i]) * 1.20) })
	p.fec = simpleui.NewDropdown("datvFEC", 690, toolY+45, 120, 36, "FEC", []string{"AUTO", "1/2", "2/3", "3/4", "5/6", "7/8"}, uiMinimumFontSize)
	p.fec.SetSelected(0)
	p.polarity = simpleui.NewDropdown("datvPolarity", 825, toolY+45, 130, 36, "ESPECTRO", []string{"NORMAL", "INVERTIDO"}, uiMinimumFontSize)
	p.polarity.SetSelected(0)
	p.decode = p.button("datvDecode", 370, toolY+103, 380, 43, "DECODIFICAR", colors.green, p.toggleDecode)
	p.record = p.button("datvRecord", 765, toolY+103, 190, 43, "GRABAR TRANSPORTE", colors.panelAlt, p.toggleRecord)
	p.controls = []simpleui.Element{p.standard, p.symbolRate, p.fec, p.polarity, p.decode, p.record}
	p.SetVisible(false)
	return p
}
func (p *DATVPanel) button(id string, x, y, w, h float32, label string, fill rl.Color, action func()) *simpleui.Button {
	b := simpleui.NewButton(id, x, y, w, h, label, uiMinimumFontSize)
	b.SetColors(fill, colors.border, colors.text)
	b.OnClick(action)
	return b
}
func (p *DATVPanel) SetVisible(v bool) {
	for _, c := range p.controls {
		c.SetVisible(v)
	}
	if p.player != nil && !v {
		p.player.SetBounds(0, 0, 1, 1, false)
	}
}
func (p *DATVPanel) Enter() {
	p.targetHz = p.screen.frequencyHz
	if p.bandwidthHz <= 0 {
		p.bandwidthHz = 400_000
	}
}
func (p *DATVPanel) Leave() { p.Stop() }

func (p *DATVPanel) toggleDecode() {
	if p.status().Running {
		p.Stop()
		p.feedback = "Decodificación detenida"
		return
	}
	p.start()
}

func (p *DATVPanel) Stop() {
	if p.screen.receiver != nil {
		p.screen.receiver.StopDATV()
	}
	if p.player != nil {
		p.player.Stop()
	}
	p.record.SetLabel("GRABAR TRANSPORTE")
}
func (p *DATVPanel) start() {
	if p.screen.receiver == nil {
		return
	}
	idx := p.symbolRate.SelectedIndex()
	if idx < 0 {
		idx = 4
	}
	x, y, w, h := p.physicalVideoBounds()
	if p.player != nil {
		if playerErr := p.player.Start(rl.GetWindowHandle(), x, y, w, h, "udp://@127.0.0.1:19540"); playerErr != nil {
			p.feedback = playerErr.Error()
		}
	}
	centerChanged := p.screen.positionVFOAt(p.targetHz)
	if centerChanged {
		p.screen.receiver.SetCenterFrequency(p.screen.centerFrequencyHz)
	}
	err := p.screen.receiver.StartDATV(datv.Settings{Standard: p.standard.SelectedText(), SymbolRate: datvSymbolRates[idx], BandwidthHz: p.bandwidthHz, OffsetHz: p.targetHz - p.screen.centerFrequencyHz, FEC: p.fec.SelectedText(), Inverted: p.polarity.SelectedIndex() == 1, StartPlayer: true})
	if err != nil {
		if p.player != nil {
			p.player.Stop()
		}
		p.feedback = err.Error()
	} else if p.feedback == "" {
		p.feedback = "LeanDVB iniciado; esperando sincronismo"
	}
}

func (p *DATVPanel) videoBounds() rl.Rectangle {
	return rl.Rectangle{X: 980, Y: toolY + 15, Width: 345, Height: 232}
}
func (p *DATVPanel) physicalVideoBounds() (int, int, int, int) {
	b := p.videoBounds()
	scale, offset := simpleui.Scale(), simpleui.Offset()
	return int(offset.X + b.X*scale.X), int(offset.Y + b.Y*scale.Y), int(b.Width * scale.X), int(b.Height * scale.Y)
}
func (p *DATVPanel) Tick() {
	if p.player == nil {
		return
	}
	x, y, w, h := p.physicalVideoBounds()
	p.player.SetBounds(x, y, w, h, p.screen.activeTool == datvToolID && p.screen.viewMode == 1 && !p.screen.overlayOpen())
}
func (p *DATVPanel) Close() {
	if p.player != nil {
		p.player.Close()
	}
}
func (p *DATVPanel) toggleRecord() {
	if p.screen.receiver == nil {
		return
	}
	s := p.screen.receiver.DATVStatus()
	if !s.Running {
		p.feedback = "Inicia primero la decodificación para grabar el transporte"
		return
	}
	path := ""
	if !s.Recording {
		path = resources.WritablePath("recordings", "datv", fmt.Sprintf("datv-%s.ts", time.Now().Format("20060102-150405")))
	}
	if err := p.screen.receiver.SetDATVRecording(path); err != nil {
		p.feedback = err.Error()
		return
	}
	if path == "" {
		p.record.SetLabel("GRABAR TRANSPORTE")
	} else {
		p.record.SetLabel("DETENER GRABACIÓN")
		p.feedback = "Guardando " + filepath.Base(path)
	}
}
func (p *DATVPanel) status() datv.Status {
	if p.screen.receiver == nil {
		return datv.Status{State: "sin receptor"}
	}
	return p.screen.receiver.DATVStatus()
}

func (p *DATVPanel) HandleSpectrumInput(mouse rl.Vector2, x, y, w, h float32) bool {
	if p.screen.activeTool != datvToolID || p.screen.viewMode != 1 {
		return false
	}
	inside := mouse.X >= x && mouse.X <= x+w && mouse.Y >= y && mouse.Y <= y+h
	left, right := p.selectionPixels(x, w)
	nearLeft := math.Abs(float64(mouse.X-left)) < 9
	nearRight := math.Abs(float64(mouse.X-right)) < 9
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && inside {
		switch {
		case nearLeft:
			p.dragHandle = -1
		case nearRight:
			p.dragHandle = 1
		default:
			p.dragHandle = 2
		}
	}
	if p.dragHandle == 0 {
		return false
	}
	if rl.IsMouseButtonDown(rl.MouseButtonLeft) || rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		hz := p.screen.centerFrequencyHz - p.screen.spanHz/2 + int64(math.Round(float64((mouse.X-x)/w)*float64(p.screen.spanHz)))
		switch p.dragHandle {
		case 2:
			p.targetHz = hz
		case -1:
			p.bandwidthHz = max(20_000, 2*(p.targetHz-hz))
		case 1:
			p.bandwidthHz = max(20_000, 2*(hz-p.targetHz))
		}
		p.bandwidthHz = min(p.bandwidthHz, p.screen.spanHz)
		p.syncNearestRate()
		p.screen.frequencyHz = p.targetHz
		p.screen.markSettingsDirty()
	}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		p.dragHandle = 0
	}
	return true
}
func (p *DATVPanel) selectionPixels(x, w float32) (float32, float32) {
	toX := func(hz int64) float32 {
		return x + float32(hz-(p.screen.centerFrequencyHz-p.screen.spanHz/2))/float32(p.screen.spanHz)*w
	}
	return toX(p.targetHz - p.bandwidthHz/2), toX(p.targetHz + p.bandwidthHz/2)
}
func (p *DATVPanel) syncNearestRate() {
	wanted := float64(p.bandwidthHz) / 1.20
	best := 0
	for i, v := range datvSymbolRates {
		if math.Abs(float64(v)-wanted) < math.Abs(float64(datvSymbolRates[best])-wanted) {
			best = i
		}
	}
	p.symbolRate.SetSelected(best)
}
func (p *DATVPanel) DrawSpectrumOverlay(x, y, w, h float32) {
	if p.screen.activeTool != datvToolID {
		return
	}
	left, right := p.selectionPixels(x, w)
	left = max(x, min(left, x+w))
	right = max(x, min(right, x+w))
	fill := rl.Color{R: 20, G: 185, B: 240, A: 34}
	rl.DrawRectangleRec(rl.Rectangle{X: left, Y: y, Width: max(right-left, float32(1)), Height: h}, fill)
	rl.DrawLineEx(rl.Vector2{X: left, Y: y}, rl.Vector2{X: left, Y: y + h}, 2, colors.cyan)
	rl.DrawLineEx(rl.Vector2{X: right, Y: y}, rl.Vector2{X: right, Y: y + h}, 2, colors.cyan)
	cx := (left + right) / 2
	rl.DrawLineEx(rl.Vector2{X: cx, Y: y}, rl.Vector2{X: cx, Y: y + h}, 1, colors.orange)
	simpleui.DrawTextStyled("DATV  "+formatSymbolRate(p.currentRate()), left+8, y+30, uiCaptionFontSize, simpleui.FontSemiBold, colors.cyan)
}
func (p *DATVPanel) currentRate() int {
	idx := p.symbolRate.SelectedIndex()
	if idx < 0 {
		return 333_000
	}
	return datvSymbolRates[idx]
}
func (p *DATVPanel) DrawPanel() {
	s := p.status()
	if s.Running {
		p.decode.SetLabel("DETENER")
		p.decode.SetColors(colors.red, colors.border, colors.text)
	} else {
		p.decode.SetLabel("DECODIFICAR")
		p.decode.SetColors(colors.green, colors.border, colors.text)
	}
	drawPanel(toolContentX, toolY, toolContentRight-toolContentX, toolH)
	simpleui.DrawTextStyled("DATV · LeanDVB", 370, toolY+8, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("Selección: %.6f MHz · ancho %.0f kHz", float64(p.targetHz)/1e6, float64(p.bandwidthHz)/1e3), 370, toolY+162, uiMinimumFontSize, colors.text)
	videoBounds := p.videoBounds()
	rl.DrawRectangleRec(rl.Rectangle{X: videoBounds.X - 3, Y: videoBounds.Y - 3, Width: videoBounds.Width + 6, Height: videoBounds.Height + 6}, colors.border)
	rl.DrawRectangleRec(videoBounds, rl.Black)
	stateColor := colors.orange
	if s.Locked && s.TSValid {
		stateColor = colors.green
	}
	if s.State == "error" || !s.Available {
		stateColor = colors.red
	}
	rl.DrawCircle(382, int32(toolY+190), 7, stateColor)
	simpleui.DrawTextStyled(strings.ToUpper(s.State), 399, toolY+179, uiMinimumFontSize, simpleui.FontSemiBold, stateColor)
	simpleui.DrawText(fmt.Sprintf("LOCK %s · MER %.1f", yesNo(s.Locked), s.MER), 540, toolY+179, uiMinimumFontSize, colors.text)
	simpleui.DrawText(fmt.Sprintf("TS %s · %.2f Mb/s", yesNo(s.TSValid), float64(s.Bitrate)/1e6), 690, toolY+179, uiMinimumFontSize, colors.text)
	if s.DecoderRate > 0 {
		simpleui.DrawText(fmt.Sprintf("Canalizador %.3f→%.3f MS/s · AFC %s %+.0f Hz", float64(s.InputRate)/1e6, float64(s.DecoderRate)/1e6, s.AFCState, s.AFCCorrectionHz), 370, toolY+207, uiCaptionFontSize, colors.muted)
	}
	if s.Running && s.AutoMode && !s.Locked && !s.TSValid {
		polarity := "normal"
		if s.SpectrumInverted {
			polarity = "invertido"
		}
		simpleui.DrawText(fmt.Sprintf("AUTO %d/%d · %s · %s · %s", s.AutoAttempt, s.AutoAttempts, s.Standard, formatSymbolRate(s.SymbolRate), polarity), 370, toolY+232, uiCaptionFontSize, colors.orange)
	}
	if s.Video != "" || s.Audio != "" {
		simpleui.DrawText("Vídeo "+datvFallback(s.Video, "—")+" · Audio "+datvFallback(s.Audio, "—"), 370, toolY+232, uiCaptionFontSize, colors.text)
	}
	p.drawConstellation(s)
	detail := p.feedback
	if s.Detail != "" {
		detail = s.Detail
	}
	if detail != "" && (s.State == "error" || !s.Running) {
		simpleui.DrawText(clipDAB(detail, 55), 370, toolY+232, uiCaptionFontSize, colors.red)
	}
}

func (p *DATVPanel) drawConstellation(s datv.Status) {
	box := rl.Rectangle{X: 815, Y: toolY + 153, Width: 145, Height: 82}
	rl.DrawRectangleRec(box, colors.background)
	rl.DrawRectangleLinesEx(box, 1, colors.border)
	cx, cy := box.X+box.Width/2, box.Y+box.Height/2
	rl.DrawLine(int32(box.X), int32(cy), int32(box.X+box.Width), int32(cy), colors.grid)
	rl.DrawLine(int32(cx), int32(box.Y), int32(cx), int32(box.Y+box.Height), colors.grid)
	for _, point := range s.Points {
		x := cx + point.I*(box.Width*.42)
		y := cy - point.Q*(box.Height*.42)
		rl.DrawCircle(int32(x), int32(y), 2, rl.Color{R: 30, G: 205, B: 240, A: 175})
	}
	label := datvFallback(s.Constellation, "CONSTELACIÓN")
	simpleui.DrawTextStyled(label, box.X+5, box.Y+4, 10, simpleui.FontSemiBold, colors.muted)
}
func formatSymbolRate(v int) string {
	if v >= 1_000_000 {
		return fmt.Sprintf("%.1f MS/s", float64(v)/1e6)
	}
	return fmt.Sprintf("%d kS/s", v/1000)
}
func yesNo(v bool) string {
	if v {
		return "SÍ"
	}
	return "NO"
}
func datvFallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
