package screens

import (
	"time"

	"go-zero/internal/dsp"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type SubtonePanel struct {
	screen        *MainScreen
	mode          string
	modeButton    *simpleui.Button
	save          *simpleui.Button
	feedback      string
	feedbackUntil time.Time
}

func NewSubtonePanel(screen *MainScreen) *SubtonePanel {
	p := &SubtonePanel{screen: screen, mode: screen.subtoneMode}
	if p.mode == "" {
		p.mode = "AUTO"
	}
	p.modeButton = simpleui.NewButton("subtoneMode", 538, 43, 88, 34, p.mode, 12)
	p.save = simpleui.NewButton("subtoneSave", 840, 43, 106, 34, "TO MEMORY", 12)
	p.modeButton.SetColors(colors.panelAlt, colors.cyan, colors.text)
	p.save.SetColors(colors.panelAlt, colors.green, colors.text)
	p.modeButton.OnClick(p.cycleMode)
	p.save.OnClick(p.saveMemory)
	if screen.receiver != nil {
		screen.receiver.SetSubtoneMode(p.mode)
	}
	return p
}
func (p *SubtonePanel) controls() []simpleui.Element {
	return []simpleui.Element{p.modeButton, p.save}
}
func (p *SubtonePanel) status() dsp.SubtoneStatus {
	if p.screen.receiver == nil {
		return dsp.SubtoneStatus{Mode: p.mode}
	}
	return p.screen.receiver.SubtoneStatus()
}
func (p *SubtonePanel) cycleMode() {
	modes := []string{"AUTO", "CTCSS", "DCS", "OFF"}
	for i, m := range modes {
		if m == p.mode {
			p.mode = modes[(i+1)%len(modes)]
			break
		}
	}
	p.modeButton.SetLabel(p.mode)
	p.screen.subtoneMode = p.mode
	if p.screen.receiver != nil {
		p.screen.receiver.SetSubtoneMode(p.mode)
	}
	p.screen.markSettingsDirty()
}
func (p *SubtonePanel) saveMemory() {
	s := p.status()
	if !s.Detected {
		p.feedback = "NO SUBTONE YET"
		p.feedbackUntil = time.Now().Add(2 * time.Second)
		return
	}
	if p.screen.memoryPanel != nil {
		p.screen.memoryPanel.openSaveModal()
		p.feedback = "SUBTONE ADDED TO MEMORY"
		p.feedbackUntil = time.Now().Add(2 * time.Second)
	}
}
func (p *SubtonePanel) Draw() {
	drawPanel(526, 16, 430, 72)
	simpleui.DrawTextStyled("DETECTOR DE TONOS", 538, 20, 12, simpleui.FontSemiBold, colors.cyan)
	s := p.status()
	result := "SEARCHING…"
	color := colors.muted
	if p.mode == "OFF" {
		result = "DESACTIVADO"
	} else if p.screen.mode.SelectedText() != "NFM" {
		result = "ONLY AVAILABLE IN NFM"
	} else if s.Detected {
		result = s.Kind + "  " + s.Value
		color = colors.green
	}
	// Keep the detected value in its own header column so it never overlaps the title.
	simpleui.DrawTextStyled(result, 720, 20, 14, simpleui.FontSemiBold, color)
	rl.DrawRectangleRounded(rl.Rectangle{X: 638, Y: 65, Width: 184, Height: 7}, 1, 4, colors.grid)
	rl.DrawRectangleRounded(rl.Rectangle{X: 638, Y: 65, Width: 184 * min(max(s.Confidence, 0), 1), Height: 7}, 1, 4, color)
	enabled := p.screen.mode.SelectedText() == "NFM" && p.mode != "OFF"
	p.save.SetEnabled(enabled && s.Detected)
	if time.Now().Before(p.feedbackUntil) {
		simpleui.DrawTextStyled(p.feedback, 638, 46, 10, simpleui.FontSemiBold, colors.orange)
	}
}
