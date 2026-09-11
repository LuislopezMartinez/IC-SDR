package screens

import (
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
	candidateModes [3]*simpleui.Dropdown
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
		x := float32(40 + (i+1)*238)
		p.candidateModes[i] = simpleui.NewDropdown(fmt.Sprintf("sstvCandidate%d", i), x+44, toolY+34, 182, 28, "MODE", sstv.ValidModes, uiMinimumFontSize)
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
	p.auto = p.button("sstvAuto", 1166, 657, 116, 36, "AUTO VIS", colors.blue)
	p.force = p.button("sstvForce", 1292, 657, 126, 36, "FORCE RX", colors.orange)
	p.force.SetColors(colors.orange, colors.cyan, colors.background)
	p.stop = p.button("sstvStop", 1428, 657, 128, 36, "STOP", colors.red)
	p.restart = p.button("sstvRestart", 1006, 703, 128, 36, "RESTART", colors.panelAlt)
	p.save = p.button("sstvSave", 1144, 703, 128, 36, "SAVE PNG", colors.green)
	p.save.SetColors(colors.green, colors.cyan, colors.background)
	p.folder = p.button("sstvFolder", 1282, 703, 274, 36, "OPEN FOLDER", colors.panelAlt)
	p.auto.OnClick(func() {
		screen.sstvAutomatic = true
		if screen.receiver != nil {
			screen.receiver.SetSSTVAutomatic(true)
		}
		p.flash("Automatic VIS detection")
		screen.markSettingsDirty()
	})
	p.force.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.ForceSSTV()
		}
		p.flash("Forced reception: 4 candidates")
	})
	p.stop.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.StopSSTVReceive()
		}
		p.flash("Reception stopped; searching for VIS")
	})
	p.restart.OnClick(func() {
		if screen.receiver != nil {
			screen.receiver.RestartSSTV()
		}
		p.flash("Decoder restarted")
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
		p.flash("Saved: " + filepath.Base(path))
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
	p.screen.sstvAutomatic = true
	p.screen.receiver.SetSSTVAutomatic(true)
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
	if p.screen.activeTool != "SSTV" || p.screen.viewMode != 1 || p.screen.receiver == nil {
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
	drawSmallText("SSTV · IMAGE RECEPTION", 40, toolY+10, colors.cyan)
	channels := []int{0, 1, 2, 3}
	p.drawAutomaticDecoderHeader()
	for i := range p.candidateModes {
		x := float32(40 + (i+1)*238)
		simpleui.DrawTextStyled(fmt.Sprintf("RX%d", i+2), x+5, toolY+42, 12, simpleui.FontSemiBold, colors.cyan)
	}
	for i, channel := range channels {
		p.drawPreview(i, channel)
	}
	stateColor := colors.orange
	if strings.Contains(p.status.State, "IMAGE") || p.status.Progress > 0 {
		stateColor = colors.green
	}
	drawSmallText(p.status.State+" · "+p.displayMode(), 1006, 751, stateColor)
	drawSmallText(fmt.Sprintf("PROGRESS %d%%   SYNC %d%%   QUEUE %d   DROP %d", p.status.Progress, p.status.SyncPercent, p.status.Queued, p.status.Dropped), 1006, 774, colors.text)
	bar := rl.Rectangle{X: 1006, Y: 799, Width: 550, Height: 10}
	rl.DrawRectangleRec(bar, colors.grid)
	rl.DrawRectangleRec(rl.Rectangle{X: bar.X, Y: bar.Y, Width: bar.Width * float32(p.status.Progress) / 100, Height: bar.Height}, colors.green)
	tone := fmt.Sprintf("TONE %.0f Hz  %.0f dB", p.status.ToneFrequency, p.status.ToneLevel)
	drawSmallText(tone, 1006, 812, colors.muted)
	if p.feedback != "" && time.Now().Before(p.feedbackUntil) {
		drawSmallText(p.feedback, 1240, 812, colors.cyan)
	}
}

func (p *SSTVPanel) displayMode() string {
	if p.status.Automatic {
		if p.status.Mode != "" {
			return p.status.Mode
		}
		return "AUTO VIS"
	}
	return "MANUAL " + p.status.SelectedMode
}

func (p *SSTVPanel) drawPreview(slot, channel int) {
	x, y, w, h := float32(40+slot*238), toolY+66, float32(226), float32(126)
	rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y, Width: w, Height: h}, rl.Color{R: 3, G: 6, B: 9, A: 255})
	rl.DrawRectangleLinesEx(rl.Rectangle{X: x, Y: y, Width: w, Height: h}, 1, colors.border)
	preview := &p.previews[channel]
	if preview.ready {
		src := rl.Rectangle{X: 0, Y: 0, Width: float32(preview.width), Height: float32(preview.height)}
		dst := fitRectangle(x+2, y+2, w-4, h-4, float32(preview.width), float32(preview.height))
		rl.DrawTexturePro(preview.texture, src, dst, rl.Vector2{}, 0, rl.White)
	} else {
		simpleui.DrawText("WAITING FOR IMAGE", x+28, y+82, uiMinimumFontSize, colors.muted)
	}
}

func (p *SSTVPanel) drawAutomaticDecoderHeader() {
	bounds := rl.Rectangle{X: 40, Y: toolY + 34, Width: 226, Height: 28}
	rl.DrawRectangleRounded(bounds, .12, 6, colors.panelAlt)
	rl.DrawRectangleRoundedLinesEx(bounds, .12, 6, 1, colors.cyan)
	drawCentered("RX1 · AUTO VIS", bounds, uiMinimumFontSize, colors.cyan)
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
	return "AUTO VIS"
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
