package screens

import (
	"fmt"
	"strings"
	"time"

	"go-zero/internal/digitalvoice"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type DigitalVoicePanel struct {
	screen                    *MainScreen
	controls                  []simpleui.Element
	start, allModes           *simpleui.Button
	modeButtons               map[string]*simpleui.Button
	enabledModes              map[string]bool
	skipEncrypted, followCall *simpleui.Switch
	lastError                 string
}

var digitalDetectionModes = []string{"DMR", "P25 I", "P25 II", "NXDN 48", "NXDN 96", "D-STAR", "YSF", "dPMR", "PROVOICE", "M17", "X2-TDMA"}

func NewDigitalVoicePanel(screen *MainScreen) *DigitalVoicePanel {
	p := &DigitalVoicePanel{screen: screen, modeButtons: make(map[string]*simpleui.Button), enabledModes: make(map[string]bool)}
	p.start = simpleui.NewButton("digitalVoiceStart", 378, 530, 148, 38, T("START"), 14)
	p.start.SetColors(actionStartFill, colors.green, colors.text)
	p.start.OnClick(p.toggle)
	p.allModes = simpleui.NewButton("digitalDetectAll", 538, 530, 112, 38, T("ALL"), 13)
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
	p.skipEncrypted = simpleui.NewSwitch("digitalVoiceEncrypted", 1070, 535, 200, 28, T("SKIP ENCRYPTED"), true, 12)
	p.skipEncrypted.SetTrackColors(colors.panelAlt, colors.green)
	p.followCall = simpleui.NewSwitch("digitalVoiceFollow", 1280, 535, 190, 28, T("FOLLOW CALL"), false, 12)
	p.followCall.SetTrackColors(colors.panelAlt, colors.blue)
	p.controls = []simpleui.Element{p.start, p.allModes, p.skipEncrypted, p.followCall}
	x := float32(538)
	for _, mode := range digitalDetectionModes {
		label := strings.ReplaceAll(strings.ReplaceAll(mode, "NXDN ", "N"), "-TDMA", "")
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
		return "AUTO · ALL"
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
	if strings.HasPrefix(mode, "NXDN") {
		return strings.HasPrefix(detected, "NXDN")
	}
	return mode == detected || (mode == "X2-TDMA" && detected == "X2")
}

func (p *DigitalVoicePanel) Enter() {
	if p.screen.mode != nil {
		for i, value := range p.screen.mode.Items() {
			if value == "NFM" {
				p.screen.mode.SetSelected(i)
				break
			}
		}
	}
	p.screen.savedMode = "NFM"
	p.screen.demodBandwidthHz = max(p.screen.demodBandwidthHz, 12_500)
	if p.screen.receiver != nil {
		p.screen.receiver.SetDemodulator("DIGITAL AUTO", p.screen.frequencyHz, p.screen.demodBandwidthHz)
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
		p.start.SetLabel(T("STOP"))
		p.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.start.SetLabel(T("START"))
		p.start.SetColors(actionStartFill, colors.green, colors.text)
	}
	p.styleModeButtons(status.Protocol)
}

func (p *DigitalVoicePanel) DrawPanel() {
	status := digitalvoice.Status{State: T("UNAVAILABLE"), InputDBFS: -60}
	if p.screen.receiver != nil {
		status = p.screen.receiver.DigitalVoiceStatus()
	}
	x, y, w := float32(360), float32(488), float32(1232)
	drawPanel(x, y, w, 338)
	simpleui.DrawTextStyled(T("DIGITAL AUTO DECODER"), x+18, y+13, 16, simpleui.FontSemiBold, colors.cyan)
	stateColor := colors.orange
	if status.State == "DECODING" {
		stateColor = colors.green
	} else if status.State == "ERROR" || !status.Available {
		stateColor = colors.red
	}
	rl.DrawCircle(int32(x+275), int32(y+23), 6, stateColor)
	simpleui.DrawTextStyled(status.State, x+288, y+14, 13, simpleui.FontSemiBold, stateColor)
	simpleui.DrawTextStyled(fmt.Sprintf("%d %s", p.selectedModeCount(), T("MODES ACTIVE")), x+306, y+52, 12, simpleui.FontSemiBold, colors.muted)

	simpleui.DrawTextStyled(T("DETECTION MODES"), x+178, y+67, 12, simpleui.FontSemiBold, colors.muted)

	call := rl.Rectangle{X: x + 18, Y: y + 124, Width: 490, Height: 112}
	drawPanel(call.X, call.Y, call.Width, call.Height)
	protocol := status.Protocol
	if protocol == "" {
		protocol = T("WAITING FOR SIGNAL")
	}
	simpleui.DrawTextStyled(protocol+slotSuffix(status.Slot), call.X+15, call.Y+10, 19, simpleui.FontSemiBold, stateColor)
	voice := T("NO VOICE")
	if status.VoiceActive {
		voice = T("CLEAR VOICE")
	}
	if status.Encrypted {
		voice = T("ENCRYPTED VOICE")
	}
	simpleui.DrawTextStyled(voice, call.X+310, call.Y+12, 13, simpleui.FontSemiBold, func() rl.Color {
		if status.Encrypted {
			return colors.red
		}
		return colors.green
	}())
	simpleui.DrawText(fmt.Sprintf("%s   %s", T("TG / DEST"), fallback(status.Target)), call.X+15, call.Y+43, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("%s %s", T("RADIO / SRC"), fallback(status.Source)), call.X+15, call.Y+66, 13, colors.text)
	detail := status.Detail
	if p.lastError != "" {
		detail = p.lastError
	}
	simpleui.DrawText(trimDigital(detail, 62), call.X+15, call.Y+90, 10, colors.muted)

	metrics := rl.Rectangle{X: x + 522, Y: y + 124, Width: 252, Height: 112}
	drawPanel(metrics.X, metrics.Y, metrics.Width, metrics.Height)
	simpleui.DrawTextStyled(T("SIGNAL QUALITY"), metrics.X+14, metrics.Y+10, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%s     %.1f dBFS", T("LEVEL"), status.InputDBFS), metrics.X+14, metrics.Y+38, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("SNR       %.1f dB", status.SNR), metrics.X+14, metrics.Y+61, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("BER       %.2f %%", status.BER), metrics.X+14, metrics.Y+84, 13, colors.text)

	context := rl.Rectangle{X: x + 788, Y: y + 124, Width: 426, Height: 112}
	drawPanel(context.X, context.Y, context.Width, context.Height)
	simpleui.DrawTextStyled(T("PROTOCOL DATA"), context.X+14, context.Y+10, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText("SLOT  "+fallback(status.Slot)+"    CC  "+fallback(status.ColorCode)+"    NAC  "+fallback(status.NAC), context.X+14, context.Y+38, 12, colors.text)
	simpleui.DrawText("RAN   "+fallback(status.RAN)+"    "+T("SYSTEM")+"  "+fallback(status.System)+"    "+T("SITE")+"  "+fallback(status.Site), context.X+14, context.Y+61, 12, colors.text)
	duration := "--:--"
	if status.VoiceActive && !status.StartedAt.IsZero() {
		duration = time.Since(status.StartedAt).Truncate(time.Second).String()
	}
	simpleui.DrawText(T("DURATION")+"  "+duration+"    "+T("AUDIO")+"  "+voice, context.X+14, context.Y+84, 12, colors.text)

	p.drawActivity(x+18, y+247, w-36, 76, status.Events)
}

func (p *DigitalVoicePanel) drawActivity(x, y, w, h float32, events []digitalvoice.Event) {
	drawPanel(x, y, w, h)
	simpleui.DrawTextStyled(T("RECENT ACTIVITY"), x+12, y+6, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(T("TIME"), x+12, y+28, 11, colors.muted)
	simpleui.DrawText(T("MODE"), x+82, y+28, 11, colors.muted)
	simpleui.DrawText("SLOT", x+172, y+28, 11, colors.muted)
	simpleui.DrawText(T("SOURCE"), x+242, y+28, 11, colors.muted)
	simpleui.DrawText(T("DEST / DETAIL"), x+350, y+28, 11, colors.muted)
	if len(events) == 0 {
		simpleui.DrawText(T("Waiting for a digital transmission"), x+12, y+50, 12, colors.muted)
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
