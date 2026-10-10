package screens

import (
	"fmt"
	"strings"

	"go-zero/internal/dab"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const dabToolID = "DAB_DAB_PLUS"

type dabChannel struct {
	name string
	hz   int64
}

var dabBandIIIChannels = []dabChannel{
	{"5A", 174_928_000}, {"5B", 176_640_000}, {"5C", 178_352_000}, {"5D", 180_064_000},
	{"6A", 181_936_000}, {"6B", 183_648_000}, {"6C", 185_360_000}, {"6D", 187_072_000},
	{"7A", 188_928_000}, {"7B", 190_640_000}, {"7C", 192_352_000}, {"7D", 194_064_000},
	{"8A", 195_936_000}, {"8B", 197_648_000}, {"8C", 199_360_000}, {"8D", 201_072_000},
	{"9A", 202_928_000}, {"9B", 204_640_000}, {"9C", 206_352_000}, {"9D", 208_064_000},
	{"10A", 209_936_000}, {"10B", 211_648_000}, {"10C", 213_360_000}, {"10D", 215_072_000},
	{"11A", 216_928_000}, {"11B", 218_640_000}, {"11C", 220_352_000}, {"11D", 222_064_000},
	{"12A", 223_936_000}, {"12B", 225_648_000}, {"12C", 227_360_000}, {"12D", 229_072_000},
	{"13A", 230_784_000}, {"13B", 232_496_000}, {"13C", 234_208_000}, {"13D", 235_776_000},
	{"13E", 237_488_000}, {"13F", 239_200_000},
}

type DABPanel struct {
	screen       *MainScreen
	controls     []simpleui.Element
	enable       *simpleui.Switch
	channel      *simpleui.Dropdown
	previous     *simpleui.Button
	tune         *simpleui.Button
	next         *simpleui.Button
	services     [3]*simpleui.Button
	serviceIDs   [3]uint32
	channelIndex int
	feedback     string
	analogMode   string
	analogBW     int
}

func NewDABPanel(screen *MainScreen) *DABPanel {
	p := &DABPanel{screen: screen, channelIndex: 27}
	p.enable = simpleui.NewSwitch("dabEnabled", 370, toolY+32, 190, 30, "DECODER", false, uiMinimumFontSize)
	p.enable.SetTrackColors(colors.panelAlt, colors.green)
	p.enable.OnChange(func(active bool) {
		if active {
			p.tuneSelected()
		}
		if screen.receiver != nil {
			if err := screen.receiver.ConfigureDAB(active, 0); err != nil {
				p.feedback = err.Error()
				p.enable.SetActive(false)
				p.restoreAnalogAudio()
				return
			}
		}
		if !active {
			p.restoreAnalogAudio()
		}
	})
	items := make([]string, len(dabBandIIIChannels))
	for index, channel := range dabBandIIIChannels {
		items[index] = channel.name
	}
	p.channel = simpleui.NewDropdown("dabChannel", 370, toolY+78, 190, 34, "CANAL", items, uiControlFontSize)
	p.channel.SetSelected(p.channelIndex)
	p.channel.OnChange(func(index int, _ string) { p.channelIndex = index })
	p.previous = p.button("dabPrevious", 370, toolY+174, 42, 42, "<", colors.panelAlt, func() { p.changeChannel(-1) })
	p.tune = p.button("dabTune", 420, toolY+174, 130, 42, "SINTONIZAR", colors.blue, p.tuneSelected)
	p.next = p.button("dabNext", 558, toolY+174, 42, 42, ">", colors.panelAlt, func() { p.changeChannel(1) })
	for index := range p.services {
		i := index
		p.services[index] = p.button(fmt.Sprintf("dabService%d", index), 850, toolY+55+float32(index)*47, 365, 39, "—", colors.panelAlt, func() { p.selectService(i) })
	}
	p.controls = []simpleui.Element{p.enable, p.channel, p.previous, p.tune, p.next}
	for _, service := range p.services {
		p.controls = append(p.controls, service)
	}
	p.SetVisible(false)
	return p
}

func (p *DABPanel) button(id string, x, y, w, h float32, label string, fill rl.Color, action func()) *simpleui.Button {
	b := simpleui.NewButton(id, x, y, w, h, label, uiBodyFontSize)
	b.SetColors(fill, colors.border, colors.text)
	b.OnClick(action)
	return b
}

func (p *DABPanel) SetVisible(visible bool) {
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
}
func (p *DABPanel) Enter() {
	if p.screen.mode == nil {
		return
	}
	currentMode := p.screen.mode.SelectedText()
	if currentMode != "DAB+" {
		p.analogMode = currentMode
		p.analogBW = p.screen.demodBandwidthHz
	}
	if p.analogMode == "" {
		p.analogMode = "NFM"
		p.analogBW = 12_500
	}
	for index, item := range p.screen.mode.Items() {
		if item == "DAB+" {
			p.screen.mode.SetSelected(index)
			break
		}
	}
	p.screen.savedMode = "DAB+"
	if p.screen.filterSelector != nil {
		preset := p.screen.filterSelector.Current("DAB+")
		p.screen.demodBandwidthHz = preset.BandwidthHz
		if p.screen.filter != nil {
			p.screen.filter.SetLabel(filterButtonLabel(preset))
		}
	}
}
func (p *DABPanel) Leave() {
	p.enable.SetActive(false)
	if p.screen.receiver != nil {
		_ = p.screen.receiver.ConfigureDAB(false, 0)
	}
}
func (p *DABPanel) changeChannel(delta int) {
	p.channelIndex = (p.channelIndex + delta + len(dabBandIIIChannels)) % len(dabBandIIIChannels)
	p.channel.SetSelected(p.channelIndex)
	p.tuneSelected()
}
func (p *DABPanel) tuneSelected() {
	channel := dabBandIIIChannels[p.channelIndex]
	s := p.screen
	restart := false
	if s.receiver != nil {
		restart = s.receiver.DABStatus().Running
		if restart {
			_ = s.receiver.ConfigureDAB(false, 0)
		}
	}
	s.frequencyHz, s.centerFrequencyHz, s.spanHz = channel.hz, channel.hz, 2_048_000
	s.updateBandForFrequency(channel.hz)
	if s.receiver != nil {
		s.receiver.SetCenterFrequency(channel.hz)
		if !p.enable.Active() {
			s.receiver.SetDemodulator(p.analogMode, channel.hz, max(p.analogBW, 1_000))
		}
		if restart {
			if err := s.receiver.ConfigureDAB(true, 0); err != nil {
				p.feedback = err.Error()
				p.enable.SetActive(false)
				p.restoreAnalogAudio()
			}
		}
	}
	if s.waterfall != nil {
		s.waterfall.Reset()
	}
	s.markSettingsDirty()
}
func (p *DABPanel) restoreAnalogAudio() {
	if p.screen.receiver == nil {
		return
	}
	mode := p.analogMode
	if mode == "" || mode == "DAB+" {
		mode = "NFM"
	}
	p.screen.receiver.SetDemodulator(mode, p.screen.frequencyHz, max(p.analogBW, 1_000))
	if p.screen.audioPlayer != nil {
		p.screen.audioPlayer.ResetPlayback()
	}
}
func (p *DABPanel) selectService(index int) {
	if index < 0 || index >= len(p.serviceIDs) || p.serviceIDs[index] == 0 || p.screen.receiver == nil {
		return
	}
	if err := p.screen.receiver.SelectDABService(p.serviceIDs[index]); err != nil {
		p.feedback = err.Error()
	}
}

func (p *DABPanel) status() dab.Status {
	if p.screen.receiver == nil {
		return dab.Status{State: "sin receptor"}
	}
	return p.screen.receiver.DABStatus()
}

func (p *DABPanel) DrawPanel() {
	status := p.status()
	for index := range p.services {
		label, id := "—", uint32(0)
		if index < len(status.Services) {
			label, id = status.Services[index].Name, status.Services[index].ID
		}
		if strings.TrimSpace(label) == "" {
			label = fmt.Sprintf("Servicio %08X", id)
		}
		p.services[index].SetLabel(label)
		p.serviceIDs[index] = id
	}
	drawPanel(toolContentX, toolY, toolContentRight-toolContentX, toolH)
	simpleui.DrawTextStyled("DAB / DAB+", 370, toolY+8, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	channel := dabBandIIIChannels[p.channelIndex]
	simpleui.DrawTextStyled(fmt.Sprintf("%.3f MHz", float64(channel.hz)/1e6), 380, toolY+127, 22, simpleui.FontMono, colors.text)
	rl.DrawLineEx(rl.Vector2{X: 610, Y: toolY + 15}, rl.Vector2{X: 610, Y: toolY + 230}, 1, colors.border)
	simpleui.DrawTextStyled("ESTADO", 635, toolY+8, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	stateColor := colors.orange
	if status.Synced {
		stateColor = colors.green
	}
	if status.State == "error" || !status.Available {
		stateColor = colors.red
	}
	rl.DrawCircle(647, int32(toolY+51), 7, stateColor)
	state := strings.ToUpper(status.State)
	if state == "" {
		state = "PREPARADO"
	}
	simpleui.DrawTextStyled(state, 665, toolY+39, uiMinimumFontSize, simpleui.FontSemiBold, stateColor)
	simpleui.DrawText(fmt.Sprintf("SNR  %.1f dB", status.SNR), 635, toolY+76, uiMinimumFontSize, colors.text)
	simpleui.DrawText("ENSEMBLE", 635, toolY+111, uiCaptionFontSize, colors.muted)
	ensemble := status.Ensemble
	if ensemble == "" {
		ensemble = "—"
	}
	simpleui.DrawTextStyled(ensemble, 635, toolY+133, uiMinimumFontSize, simpleui.FontSemiBold, colors.text)
	if p.feedback != "" {
		simpleui.DrawText(clipDAB(p.feedback, 28), 635, toolY+183, uiCaptionFontSize, colors.red)
	}
	rl.DrawLineEx(rl.Vector2{X: 825, Y: toolY + 15}, rl.Vector2{X: 825, Y: toolY + 230}, 1, colors.border)
	simpleui.DrawTextStyled("SERVICIOS", 850, toolY+8, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	rl.DrawLineEx(rl.Vector2{X: 1240, Y: toolY + 15}, rl.Vector2{X: 1240, Y: toolY + 230}, 1, colors.border)
	simpleui.DrawTextStyled("AHORA SUENA", 1265, toolY+8, uiSectionFontSize, simpleui.FontSemiBold, colors.cyan)
	service := status.Service
	if service == "" {
		service = "Sin servicio seleccionado"
	}
	simpleui.DrawTextStyled(clipDAB(service, 24), 1265, toolY+47, 18, simpleui.FontSemiBold, colors.text)
	simpleui.DrawText("DAB/DAB+ · audio digital", 1265, toolY+78, uiBodyFontSize, colors.muted)
	simpleui.DrawText(fmt.Sprintf("CORRECCIÓN   %+.0f Hz", status.FrequencyErrorHz), 1265, toolY+108, uiCaptionFontSize, colors.text)
	simpleui.DrawText(fmt.Sprintf("COLA IQ      %d / %d", status.InputQueued, status.InputCapacity), 1265, toolY+130, uiCaptionFontSize, colors.text)
	dropColor := colors.muted
	if status.Dropped > 0 {
		dropColor = colors.orange
	}
	simpleui.DrawText(fmt.Sprintf("PÉRDIDAS      %d", status.Dropped), 1265, toolY+152, uiCaptionFontSize, dropColor)
	audioRate := "—"
	if status.AudioSampleRate > 0 {
		audioRate = fmt.Sprintf("%d kHz · %d ch", status.AudioSampleRate/1000, status.AudioChannels)
	}
	simpleui.DrawText("AUDIO         "+audioRate, 1265, toolY+174, uiCaptionFontSize, colors.text)
	runtimeText := "DETENIDO"
	runtimeColor := colors.muted
	if status.Running {
		runtimeText, runtimeColor = "ACTIVO", colors.green
	}
	simpleui.DrawText("RUNTIME       "+runtimeText, 1265, toolY+196, uiCaptionFontSize, runtimeColor)
}
func clipDAB(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit-3] + "..."
}
