package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"image/color"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go-zero/internal/sstv"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type sstvPreview struct {
	texture  rl.Texture2D
	ready    bool
	sequence int
	width    int
	height   int
	lines    int
	pixels   []color.RGBA
}

type SSTVPanel struct {
	screen         *MainScreen
	controls       []simpleui.Element
	candidateModes [4]*simpleui.Dropdown
	auto, force    *simpleui.Button
	stop, restart  *simpleui.Button
	save, folder   *simpleui.Button
	previews       [5]sstvPreview
	status         sstv.Status
	feedback       string
	feedbackUntil  time.Time
}

func NewSSTVPanel(screen *MainScreen) *SSTVPanel {
	p := &SSTVPanel{screen: screen}
	for i := range p.candidateModes {
		i := i
		column, row := i%2, i/2
		x := float32(990 + column*286)
		y := float32(487 + row*137)
		p.candidateModes[i] = simpleui.NewDropdown(fmt.Sprintf("sstvCandidate%d", i), x+66, y, 196, 27, i18n.Source("text.ab06b3638fb5"), sstv.ValidModes, uiMinimumFontSize)
		p.candidateModes[i].SetSelected(sstvModeIndex(screen.sstvCandidateModes[i]))
		p.candidateModes[i].SetMaxVisibleItems(5)
		p.candidateModes[i].OnChange(func(_ int, mode string) {
			screen.sstvCandidateModes[i] = mode
			if screen.receiver != nil {
				screen.receiver.SetSSTVCandidateMode(i, mode)
			}
			screen.markSettingsDirty()
		})
	}
	p.auto = p.button("sstvAuto", 56, 803, 128, 38, i18n.Source("text.80e4cf374a8a"), colors.blue)
	p.force = p.button("sstvForce", 194, 803, 150, 38, "INICIAR SIN VIS", colors.orange)
	p.force.SetColors(colors.orange, colors.cyan, colors.background)
	p.stop = p.button("sstvStop", 354, 803, 128, 38, "CANCELAR", colors.red)
	p.restart = p.button("sstvRestart", 1364, 803, 176, 38, i18n.Source("text.d3eb9ea922cf"), colors.panelAlt)
	p.save = p.button("sstvSave", 560, 803, 148, 38, i18n.Source("text.65b42fff1e06"), colors.green)
	p.save.SetColors(colors.green, colors.cyan, colors.background)
	p.folder = p.button("sstvFolder", 718, 803, 190, 38, i18n.Source("text.f7aa514861ad"), colors.panelAlt)
	p.auto.OnClick(func() {
		screen.sstvAutomatic = true
		if screen.receiver != nil {
			screen.receiver.SetSSTVAutomatic(true)
		}
		p.flash(i18n.Source("text.5900e390166a"))
		screen.markSettingsDirty()
	})
	p.force.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.ForceSSTV()
		}
		p.flash(i18n.Source("text.0cba64183bac"))
	})
	p.stop.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.StopSSTVReceive()
		}
		p.flash(i18n.Source("text.3344f520783f"))
	})
	p.restart.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.RestartSSTV()
		}
		p.flash(i18n.Source("text.6367e9e8c696"))
	})
	p.save.OnClick(func() {
		if screen.receiver == nil {
			return
		}
		path, err := screen.receiver.SaveSSTVPartial()
		if err != nil {
			p.flash(err.Error())
			return
		}
		p.flash(i18n.Source("text.1dd5699b1655") + filepath.Base(path))
	})
	p.folder.OnClick(func() {
		if screen.receiver == nil {
			return
		}
		_ = exec.Command("explorer.exe", screen.receiver.SSTVOutputFolder()).Start()
	})
	p.controls = []simpleui.Element{p.auto, p.force, p.stop, p.restart, p.save, p.folder}
	for _, dropdown := range p.candidateModes {
		p.controls = append(p.controls, dropdown)
	}
	p.SetVisible(false)
	return p
}

func (p *SSTVPanel) button(id string, x, y, w, h float32, label string, background rl.Color) *simpleui.Button {
	b := simpleui.NewButton(id, x, y, w, h, label, uiControlFontSize)
	b.SetColors(background, colors.cyan, colors.text)
	return b
}

func sstvModeIndex(mode string) int {
	for i, candidate := range sstv.ValidModes {
		if candidate == mode {
			return i
		}
	}
	return 5
}

func (p *SSTVPanel) Enter() {
	if p.screen.receiver == nil {
		return
	}
	p.screen.receiver.ConfigureSSTV(true)
	p.screen.receiver.SetSSTVAutomatic(p.screen.sstvAutomatic)
	for i, mode := range p.screen.sstvCandidateModes[:len(p.candidateModes)] {
		p.screen.receiver.SetSSTVCandidateMode(i, mode)
	}
}

func (p *SSTVPanel) Leave() {
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureSSTV(false)
	}
}

func (p *SSTVPanel) SetVisible(visible bool) {
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
	for _, dropdown := range p.candidateModes {
		dropdown.SetVisible(visible)
	}
}

func (p *SSTVPanel) Tick() {
	if p.screen.activeTool != i18n.Source("text.820d4685bc9d") || p.screen.viewMode != 1 || p.screen.receiver == nil {
		return
	}
	p.status = p.screen.receiver.SSTVStatus()
	for _, dropdown := range p.candidateModes {
		dropdown.SetVisible(true)
	}
	for channel := range p.previews {
		frame, changed := p.screen.receiver.SSTVFrame(channel, p.previews[channel].sequence)
		if changed {
			p.updatePreview(channel, frame)
		}
	}
}

func (p *SSTVPanel) updatePreview(channel int, frame sstv.Frame) {
	preview := &p.previews[channel]
	preview.sequence, preview.lines = frame.Sequence, frame.Lines
	if frame.Width <= 0 || frame.Height <= 0 || len(frame.RGB) != frame.Width*frame.Height*3 {
		return
	}
	if !preview.ready || preview.width != frame.Width || preview.height != frame.Height {
		if preview.ready {
			rl.UnloadTexture(preview.texture)
		}
		img := rl.GenImageColor(frame.Width, frame.Height, rl.Black)
		preview.texture = rl.LoadTextureFromImage(img)
		rl.UnloadImage(img)
		rl.SetTextureFilter(preview.texture, rl.FilterBilinear)
		preview.ready, preview.width, preview.height = true, frame.Width, frame.Height
	}
	if cap(preview.pixels) < frame.Width*frame.Height {
		preview.pixels = make([]color.RGBA, frame.Width*frame.Height)
	} else {
		preview.pixels = preview.pixels[:frame.Width*frame.Height]
	}
	for i := range preview.pixels {
		j := i * 3
		preview.pixels[i] = color.RGBA{R: frame.RGB[j], G: frame.RGB[j+1], B: frame.RGB[j+2], A: 255}
	}
	rl.UpdateTexture(preview.texture, preview.pixels)
}

func (p *SSTVPanel) DrawPanel() {
	stateColor := colors.orange
	if strings.Contains(p.status.State, i18n.Source("text.6b3cf57c7b13")) || p.status.Progress > 0 {
		stateColor = colors.green
	}
	simpleui.DrawTextStyled(i18n.Source("text.c46d72363117"), 40, 468, 15, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("FFT visible · el waterfall se oculta para ampliar la imagen", 286, 470, 11, colors.muted)

	mainCard := rl.Rectangle{X: 40, Y: 490, Width: 920, Height: 270}
	drawSSTVCard(mainCard, colors.cyan)
	simpleui.DrawTextStyled("IMAGEN PRINCIPAL · "+p.displayMode(), 56, 501, 14, simpleui.FontSemiBold, colors.cyan)
	drawSSTVBadge(rl.Rectangle{X: 760, Y: 497, Width: 180, Height: 25}, p.status.State, stateColor)
	p.drawImagePreview(rl.Rectangle{X: 55, Y: 530, Width: 890, Height: 216}, 0)

	for index := range p.candidateModes {
		p.drawCandidate(index)
	}

	drawSSTVControlGroup(rl.Rectangle{X: 40, Y: 776, Width: 458, Height: 75}, "RECEPCIÓN", colors.blue)
	drawSSTVControlGroup(rl.Rectangle{X: 520, Y: 776, Width: 408, Height: 75}, "IMAGEN", colors.orange)
	drawSSTVControlGroup(rl.Rectangle{X: 950, Y: 776, Width: 606, Height: 75}, "SERVICIO DEL DECODER", colors.green)

	drawSmallText(p.status.State+" · "+p.displayMode(), 56, 862, stateColor)
	drawSmallText(fmt.Sprintf(i18n.Source("text.1960c0d8a5da"), p.status.Progress, p.status.SyncPercent, p.status.Queued, p.status.Dropped), 430, 862, colors.text)
	bar := rl.Rectangle{X: 56, Y: 881, Width: 880, Height: 7}
	rl.DrawRectangleRec(bar, colors.grid)
	rl.DrawRectangleRec(rl.Rectangle{X: bar.X, Y: bar.Y, Width: bar.Width * float32(p.status.Progress) / 100, Height: bar.Height}, colors.green)
	tone := fmt.Sprintf(i18n.Source("text.c5e82496da50"), p.status.ToneFrequency, p.status.ToneLevel)
	drawSmallText(tone, 970, 862, colors.muted)
	if p.feedback != "" && time.Now().Before(p.feedbackUntil) {
		drawSmallText(p.feedback, 1210, 862, colors.cyan)
	}
}

func (p *SSTVPanel) displayMode() string {
	if p.status.Automatic {
		if p.status.Mode != "" {
			return p.status.Mode
		}
		return i18n.Source("text.80e4cf374a8a")
	}
	return i18n.Source("text.3d8fd46b21fc") + p.status.SelectedMode
}

func (p *SSTVPanel) drawImagePreview(bounds rl.Rectangle, channel int) {
	rl.DrawRectangleRec(bounds, rl.Color{R: 3, G: 6, B: 9, A: 255})
	rl.DrawRectangleLinesEx(bounds, 1, colors.border)
	preview := &p.previews[channel]
	if preview.ready {
		src := rl.Rectangle{X: 0, Y: 0, Width: float32(preview.width), Height: float32(preview.height)}
		dst := fitRectangle(bounds.X+2, bounds.Y+2, bounds.Width-4, bounds.Height-4, float32(preview.width), float32(preview.height))
		rl.DrawTexturePro(preview.texture, src, dst, rl.Vector2{}, 0, rl.White)
	} else {
		label := i18n.Source("text.b6285ba14fd2")
		size := simpleui.MeasureText(label, uiMinimumFontSize)
		simpleui.DrawText(label, bounds.X+(bounds.Width-size.X)/2, bounds.Y+(bounds.Height-size.Y)/2, uiMinimumFontSize, colors.muted)
	}
}

func (p *SSTVPanel) drawCandidate(index int) {
	column, row := index%2, index/2
	x, y := float32(980+column*286), float32(490+row*137)
	accent := colors.blue
	if p.status.CandidateProgress[index] > 0 {
		accent = colors.orange
	}
	if p.status.CandidateSync[index] >= 70 {
		accent = colors.green
	}
	bounds := rl.Rectangle{X: x, Y: y, Width: 276, Height: 130}
	drawSSTVCard(bounds, accent)
	simpleui.DrawTextStyled(fmt.Sprintf("RX%d", index+2), x+12, y+11, 12, simpleui.FontSemiBold, accent)
	p.drawImagePreview(rl.Rectangle{X: x + 12, Y: y + 43, Width: 126, Height: 75}, index+1)
	simpleui.DrawText(fmt.Sprintf("PROGRESO  %d%%", p.status.CandidateProgress[index]), x+150, y+52, 10, colors.text)
	drawSSTVMeter(rl.Rectangle{X: x + 150, Y: y + 69, Width: 108, Height: 6}, p.status.CandidateProgress[index], accent)
	simpleui.DrawText(fmt.Sprintf("SYNC  %d%%", p.status.CandidateSync[index]), x+150, y+83, 10, colors.text)
	drawSSTVMeter(rl.Rectangle{X: x + 150, Y: y + 100, Width: 108, Height: 6}, p.status.CandidateSync[index], accent)
	lines := p.previews[index+1].lines
	simpleui.DrawText(fmt.Sprintf("%d líneas", lines), x+150, y+111, 9, colors.muted)
}

func drawSSTVCard(bounds rl.Rectangle, accent rl.Color) {
	fill := mixColor(colors.panelAlt, accent, .06)
	rl.DrawRectangleRounded(bounds, .035, 7, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .035, 7, 1, mixColor(colors.border, accent, .5))
}

func drawSSTVControlGroup(bounds rl.Rectangle, title string, accent rl.Color) {
	drawSSTVCard(bounds, accent)
	simpleui.DrawTextStyled(title, bounds.X+14, bounds.Y+7, 11, simpleui.FontSemiBold, accent)
}

func drawSSTVBadge(bounds rl.Rectangle, label string, accent rl.Color) {
	fill := mixColor(colors.panelAlt, accent, .28)
	rl.DrawRectangleRounded(bounds, .45, 8, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .45, 8, 1, accent)
	drawCentered(trimMemory(label, 24), bounds, 10, simpleui.EnsureTextContrast(colors.text, fill))
}

func drawSSTVMeter(bounds rl.Rectangle, value int, accent rl.Color) {
	rl.DrawRectangleRounded(bounds, .5, 5, colors.grid)
	width := bounds.Width * float32(min(max(value, 0), 100)) / 100
	rl.DrawRectangleRounded(rl.Rectangle{X: bounds.X, Y: bounds.Y, Width: width, Height: bounds.Height}, .5, 5, accent)
}

func fitRectangle(x, y, w, h, sourceW, sourceH float32) rl.Rectangle {
	if sourceW <= 0 || sourceH <= 0 {
		return rl.Rectangle{X: x, Y: y, Width: w, Height: h}
	}
	scale := min(w/sourceW, h/sourceH)
	dw, dh := sourceW*scale, sourceH*scale
	return rl.Rectangle{X: x + (w-dw)/2, Y: y + (h-dh)/2, Width: dw, Height: dh}
}

func (p *SSTVPanel) channelMode(channel int) string {
	if channel > 0 && channel <= len(p.status.CandidateModes) {
		return p.status.CandidateModes[channel-1]
	}
	if p.status.Mode != "" {
		return p.status.Mode
	}
	return i18n.Source("text.80e4cf374a8a")
}

func (p *SSTVPanel) flash(message string) {
	p.feedback, p.feedbackUntil = message, time.Now().Add(3*time.Second)
}

func (p *SSTVPanel) Close() {
	for i := range p.previews {
		if p.previews[i].ready {
			rl.UnloadTexture(p.previews[i].texture)
			p.previews[i].ready = false
		}
	}
}
