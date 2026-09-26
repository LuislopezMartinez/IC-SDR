package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"strings"
	"time"

	"go-zero/internal/digitalvoice"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type DigitalVoicePanel struct {
	screen                               *MainScreen
	controls                             []simpleui.Element
	start, allModes                      *simpleui.Button
	modeButtons                          map[string]*simpleui.Button
	enabledModes                         map[string]bool
	skipEncrypted, followCall, nfmBypass *simpleui.Switch
	lastError                            string
}

var digitalDetectionModes = []string{i18n.Source("text.ade0cbd42252"), "P25 I", i18n.Source("text.9e7aa85fb23c"), i18n.Source("text.09558c27a7a1"), i18n.Source("text.8ffa43c306ee"), i18n.Source("text.4fcb50bfd35b"), i18n.Source("text.ef1fb8875c5e"), "dPMR", i18n.Source("text.996308552c50"), "M17", i18n.Source("text.63c78e1bf13c")}

func NewDigitalVoicePanel(screen *MainScreen) *DigitalVoicePanel {
	p := &DigitalVoicePanel{screen: screen, modeButtons: make(map[string]*simpleui.Button), enabledModes: make(map[string]bool)}
	p.start = simpleui.NewButton("digitalVoiceStart", 378, 530, 148, 38, i18n.Source("text.7f23d98fbc9a"), 14)
	p.start.SetColors(actionStartFill, colors.green, colors.text)
	p.start.OnClick(p.toggle)
	p.allModes = simpleui.NewButton("digitalDetectAll", 538, 530, 112, 38, i18n.Source("text.1facd1ff1ab7"), 13)
	p.allModes.SetColors(colors.blue, colors.cyan, colors.text)
	p.allModes.OnClick(func() {
		for _, mode := range digitalDetectionModes {
			p.enabledModes[mode] = true
		}
		if p.screen.receiver != nil && p.screen.receiver.DigitalVoiceStatus().Running {
			p.screen.receiver.StopDigitalVoice()
		}
		p.styleModeButtons("")
	})
	p.skipEncrypted = simpleui.NewSwitch("digitalVoiceEncrypted", 1070, 535, 200, 28, i18n.Source("text.42d506b4220f"), true, 12)
	p.skipEncrypted.SetTrackColors(colors.panelAlt, colors.green)
	p.followCall = simpleui.NewSwitch("digitalVoiceFollow", 1280, 535, 190, 28, i18n.Source("text.10f775fe15af"), false, 12)
	p.followCall.SetTrackColors(colors.panelAlt, colors.blue)
	p.nfmBypass = simpleui.NewSwitch("digitalVoiceNFMBypass", 820, 535, 220, 28, "BYPASS NFM", true, 12)
	p.nfmBypass.SetTrackColors(colors.panelAlt, colors.green)
	p.nfmBypass.OnChange(func(enabled bool) {
		if p.screen.receiver != nil {
			p.screen.receiver.SetDigitalVoiceBypass(enabled)
		}
	})
	p.controls = []simpleui.Element{p.start, p.allModes, p.nfmBypass, p.skipEncrypted, p.followCall}
	x := float32(538)
	for _, mode := range digitalDetectionModes {
		label := strings.ReplaceAll(strings.ReplaceAll(mode, i18n.Source("text.f8f49fde8af0"), "N"), i18n.Source("text.1a2f630a99ac"), "")
		width := max(float32(len([]rune(label))*7+18), 48)
		button := simpleui.NewButton("digitalDetect"+strings.ReplaceAll(mode, " ", ""), x, 578, width, 27, label, 11)
		selectedMode := mode
		button.OnClick(func() { p.toggleMode(selectedMode) })
		p.modeButtons[mode], p.enabledModes[mode] = button, true
		p.controls = append(p.controls, button)
		x += width + 7
	}
	p.SetVisible(false)
	return p
}

func (p *DigitalVoicePanel) toggle() {
	if p.screen.receiver == nil {
		return
	}
	status := p.screen.receiver.DigitalVoiceStatus()
	if status.Running {
		p.screen.receiver.StopDigitalVoice()
		return
	}
	if err := p.screen.receiver.StartDigitalVoice(p.backendMode()); err != nil {
		p.lastError = err.Error()
	}
}

func (p *DigitalVoicePanel) toggleMode(mode string) {
	if !p.enabledModes[mode] {
		p.enabledModes[mode] = true
	} else {
		count := 0
		for _, enabled := range p.enabledModes {
			if enabled {
				count++
			}
		}
		if count > 1 {
			p.enabledModes[mode] = false
		}
	}
	if p.screen.receiver != nil && p.screen.receiver.DigitalVoiceStatus().Running {
		p.screen.receiver.StopDigitalVoice()
	}
	p.styleModeButtons("")
}

func (p *DigitalVoicePanel) backendMode() string {
	selected := make([]string, 0, len(digitalDetectionModes))
	for _, mode := range digitalDetectionModes {
		if p.enabledModes[mode] {
			selected = append(selected, mode)
		}
	}
	if len(selected) == 1 {
		return selected[0]
	}
	if len(selected) == len(digitalDetectionModes) {
		return i18n.Source("text.5f08cf2bcce3")
	}
	return strings.Join(selected, "|")
}

func (p *DigitalVoicePanel) styleModeButtons(detected string) {
	selected := 0
	for mode, button := range p.modeButtons {
		if p.enabledModes[mode] {
			selected++
		}
		if !p.enabledModes[mode] {
			button.SetColors(colors.panelAlt, colors.border, colors.muted)
		} else if modeMatchesDetection(mode, detected) {
			button.SetColors(blendRGBA(colors.panelAlt, colors.green, .38), colors.green, colors.text)
		} else {
			button.SetColors(blendRGBA(colors.panelAlt, colors.blue, .22), colors.cyan, colors.text)
		}
	}
	if selected == len(digitalDetectionModes) {
		p.allModes.SetColors(colors.blue, colors.cyan, colors.text)
	} else {
		p.allModes.SetColors(colors.panelAlt, colors.border, colors.muted)
	}
}

func (p *DigitalVoicePanel) selectedModeCount() int {
	count := 0
	for _, enabled := range p.enabledModes {
		if enabled {
			count++
		}
	}
	return count
}

func modeMatchesDetection(mode, detected string) bool {
	mode, detected = strings.ToUpper(mode), strings.ToUpper(detected)
	if strings.HasPrefix(mode, i18n.Source("text.da9d217abd2b")) {
		return strings.HasPrefix(detected, i18n.Source("text.da9d217abd2b"))
	}
	return mode == detected || (mode == i18n.Source("text.63c78e1bf13c") && detected == "X2")
}

func (p *DigitalVoicePanel) Enter() {
	if p.screen.mode != nil {
		for i, value := range p.screen.mode.Items() {
			if value == i18n.Source("text.0896d612d497") {
				p.screen.mode.SetSelected(i)
				break
			}
		}
	}
	p.screen.savedMode = i18n.Source("text.0896d612d497")
	if p.screen.receiver != nil {
		p.screen.receiver.SetDigitalVoiceBypass(p.nfmBypass.Active())
		p.screen.receiver.SetDemodulator(i18n.Source("text.3ae4feb8250d"), p.screen.frequencyHz, p.screen.demodBandwidthHz)
	}
}

func (p *DigitalVoicePanel) Leave() {
	if p.screen.receiver != nil {
		p.screen.receiver.StopDigitalVoice()
	}
}

func (p *DigitalVoicePanel) Close() { p.Leave() }

func (p *DigitalVoicePanel) SetVisible(visible bool) {
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
}

func (p *DigitalVoicePanel) Tick() {
	if p.screen.receiver == nil {
		return
	}
	status := p.screen.receiver.DigitalVoiceStatus()
	if status.Running {
		p.start.SetLabel(i18n.Source("text.42a572b1399e"))
		p.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.start.SetLabel(i18n.Source("text.7f23d98fbc9a"))
		p.start.SetColors(actionStartFill, colors.green, colors.text)
	}
	p.styleModeButtons(status.Protocol)
}

func (p *DigitalVoicePanel) DrawPanel() {
	status := digitalvoice.Status{State: i18n.Source("text.67b9e10a1cbd"), InputDBFS: -60}
	if p.screen.receiver != nil {
		status = p.screen.receiver.DigitalVoiceStatus()
	}
	x, y, w := float32(360), float32(488), float32(1232)
	drawPanel(x, y, w, 338)
	// Keep the three independent listening policies visually grouped. These
	// cards are drawn before the UI elements, so their switches remain fully
	// interactive while the labels no longer float over the main panel.
	drawPanel(800, 521, 250, 55)
	drawPanel(1058, 521, 220, 55)
	drawPanel(1268, 521, 214, 55)
	simpleui.DrawTextStyled(i18n.Source("text.0efd19a09068"), x+18, y+13, 16, simpleui.FontSemiBold, colors.cyan)
	stateColor := colors.orange
	if status.State == i18n.Source("text.c6d93a7e3862") {
		stateColor = colors.green
	} else if status.State == i18n.Source("text.d98ee0e5f939") || !status.Available {
		stateColor = colors.red
	}
	rl.DrawCircle(int32(x+275), int32(y+23), 6, stateColor)
	simpleui.DrawTextStyled(status.State, x+288, y+14, 13, simpleui.FontSemiBold, stateColor)
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.94576db15561"), p.selectedModeCount()), x+306, y+52, 12, simpleui.FontSemiBold, colors.muted)
	// This heading used to share the ALL button's coordinates and was partly
	// hidden behind it. The free left side of the mode row is its natural slot.
	simpleui.DrawTextStyled(i18n.Source("text.d8f75115c06d"), x+18, y+96, 12, simpleui.FontSemiBold, colors.muted)

	call := rl.Rectangle{X: x + 18, Y: y + 124, Width: 490, Height: 112}
	drawPanel(call.X, call.Y, call.Width, call.Height)
	protocol := status.Protocol
	if protocol == "" {
		protocol = i18n.Source("text.d274cad88fc7")
	}
	simpleui.DrawTextStyled(protocol+slotSuffix(status.Slot), call.X+15, call.Y+10, 19, simpleui.FontSemiBold, stateColor)
	voice := i18n.Source("text.4d2e8ca34469")
	if status.VoiceActive {
		voice = i18n.Source("text.c418d63f8b13")
	}
	if status.Encrypted {
		voice = i18n.Source("text.8d3f66fe97d2")
	}
	simpleui.DrawTextStyled(voice, call.X+310, call.Y+12, 13, simpleui.FontSemiBold, func() rl.Color {
		if status.Encrypted {
			return colors.red
		}
		return colors.green
	}())
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.d40f44d16794"), fallback(status.Target)), call.X+15, call.Y+43, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.8ee436ff807a"), fallback(status.Source)), call.X+15, call.Y+66, 13, colors.text)
	detail := status.Detail
	if p.lastError != "" {
		detail = p.lastError
	}
	simpleui.DrawText(trimDigital(detail, 62), call.X+15, call.Y+90, 10, colors.muted)

	metrics := rl.Rectangle{X: x + 522, Y: y + 124, Width: 252, Height: 112}
	drawPanel(metrics.X, metrics.Y, metrics.Width, metrics.Height)
	simpleui.DrawTextStyled(i18n.Source("text.7ce16744f4e7"), metrics.X+14, metrics.Y+10, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.6bc6699e4c5c"), status.InputDBFS), metrics.X+14, metrics.Y+38, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.38137d08a579"), status.SNR), metrics.X+14, metrics.Y+61, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.eda21833bc6e"), status.BER), metrics.X+14, metrics.Y+84, 13, colors.text)

	context := rl.Rectangle{X: x + 788, Y: y + 124, Width: 426, Height: 112}
	drawPanel(context.X, context.Y, context.Width, context.Height)
	simpleui.DrawTextStyled(i18n.Source("text.4a444df712c4"), context.X+14, context.Y+10, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(i18n.Source("text.8aa75e7708c6")+fallback(status.Slot)+"    CC  "+fallback(status.ColorCode)+i18n.Source("text.cbd17bc64986")+fallback(status.NAC), context.X+14, context.Y+38, 12, colors.text)
	simpleui.DrawText(i18n.Source("text.3d2b56dc4923")+fallback(status.RAN)+i18n.Source("text.cd3eee1922c5")+fallback(status.System)+i18n.Source("text.1d226d27511e")+fallback(status.Site), context.X+14, context.Y+61, 12, colors.text)
	duration := "--:--"
	if status.VoiceActive && !status.StartedAt.IsZero() {
		duration = time.Since(status.StartedAt).Truncate(time.Second).String()
	}
	simpleui.DrawText(i18n.Source("text.1d833b90d9c3")+duration+i18n.Source("text.f9b479570a6b")+voice, context.X+14, context.Y+84, 12, colors.text)

	p.drawActivity(x+18, y+247, w-36, 76, status.Events)
}

func (p *DigitalVoicePanel) drawActivity(x, y, w, h float32, events []digitalvoice.Event) {
	drawPanel(x, y, w, h)
	simpleui.DrawTextStyled(i18n.Source("text.414e69ec8567"), x+12, y+6, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(i18n.Source("text.e7563517a678"), x+12, y+28, 11, colors.muted)
	simpleui.DrawText(i18n.Source("text.ab06b3638fb5"), x+82, y+28, 11, colors.muted)
	simpleui.DrawText(i18n.Source("text.12dd6f102b80"), x+172, y+28, 11, colors.muted)
	simpleui.DrawText(i18n.Source("text.9a965b631371"), x+242, y+28, 11, colors.muted)
	simpleui.DrawText(i18n.Source("text.4d33cd7f8416"), x+350, y+28, 11, colors.muted)
	if len(events) == 0 {
		simpleui.DrawText(i18n.Source("text.19706ce22b79"), x+12, y+50, 12, colors.muted)
		return
	}
	e := events[len(events)-1]
	simpleui.DrawText(e.At, x+12, y+50, 12, colors.text)
	simpleui.DrawText(e.Protocol, x+82, y+50, 12, colors.text)
	simpleui.DrawText(fallback(e.Slot), x+172, y+50, 12, colors.text)
	simpleui.DrawText(fallback(e.Source), x+242, y+50, 12, colors.text)
	simpleui.DrawText(trimDigital(fallback(e.Target)+" · "+e.Detail, 85), x+350, y+50, 12, colors.text)
}

func fallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "--"
	}
	return value
}
func slotSuffix(slot string) string {
	if strings.TrimSpace(slot) == "" {
		return ""
	}
	return " · " + slot
}
func trimDigital(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit-3] + "..."
	}
	return value
}
