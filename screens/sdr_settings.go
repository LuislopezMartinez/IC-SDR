package screens

import (
	"fmt"
	"math"

	"go-zero/internal/sdr"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// SDRSettings is a modal editor for the controls exported by the active receiver.
type SDRSettings struct {
	simpleui.BaseElement
	receiver *sdr.Receiver
	open     bool
	nextSync float64
	original sdr.HardwareSettings
	current  sdr.HardwareSettings
	controls []simpleui.Element

	deviceLabel, rfLabel, ifLabel, ppmLabel, setpointLabel, antennaLabel *simpleui.Label
	agc, biasT, rfNotch, dabNotch, iqCorrection                          *simpleui.Switch
	rfGain, ifGain, ppm, setpoint                                        *simpleui.Slider
	antenna                                                              [3]*simpleui.Button
	apply, cancel                                                        *simpleui.Button
}

func NewSDRSettings(receiver *sdr.Receiver) *SDRSettings {
	modal := &SDRSettings{
		BaseElement: simpleui.NewBaseElement("sdrSettingsOverlay", 0, 0, designWidth, designHeight),
		receiver:    receiver,
	}
	modal.createControls()
	return modal
}

func (modal *SDRSettings) createControls() {
	label := func(id string, x, y, width float32, text string, size int32) *simpleui.Label {
		result := simpleui.NewLabel(id, x, y, width, 24, text, size)
		result.SetAlignment(simpleui.AlignCenter)
		return result
	}
	modal.deviceLabel = label("settingsDevice", 500, 158, 600, "", 14)
	modal.antennaLabel = label("settingsAntennaLabel", 500, 190, 180, "ANTENNA", 13)
	for index := range modal.antenna {
		i := index
		modal.antenna[i] = simpleui.NewButton("settingsAntenna"+fmt.Sprintf("%d", i), 690+float32(i)*140, 186, 128, 32, []string{"A", "B", "C"}[i], 16)
		modal.antenna[i].OnClick(func() {
			if i >= len(modal.current.Antennas) {
				return
			}
			modal.current.Antenna = modal.current.Antennas[i]
			modal.submit()
			modal.refresh()
		})
	}
	modal.agc = simpleui.NewSwitch("settingsAGC", 500, 235, 300, 34, "AGC", false, 16)
	modal.rfLabel = label("settingsRFLabel", 500, 298, 360, "LNA / RFGR", 14)
	modal.rfGain = simpleui.NewSlider("settingsRF", 500, 332, 360, 26, 0, 9, 0)
	modal.rfGain.SetStep(1)
	modal.ifLabel = label("settingsIFLabel", 500, 378, 360, "IFGR", 14)
	modal.ifGain = simpleui.NewSlider("settingsIF", 500, 412, 360, 26, 20, 59, 40)
	modal.ifGain.SetStep(1)
	modal.ppmLabel = label("settingsPPMLabel", 500, 458, 360, "FREQUENCY CORRECTION", 14)
	modal.ppm = simpleui.NewSlider("settingsPPM", 500, 492, 360, 26, -100, 100, 0)
	modal.ppm.SetStep(.1)

	modal.biasT = simpleui.NewSwitch("settingsBiasT", 920, 235, 230, 34, "BIAS-T", false, 14)
	modal.rfNotch = simpleui.NewSwitch("settingsRFNotch", 920, 285, 230, 34, "RF NOTCH", false, 14)
	modal.dabNotch = simpleui.NewSwitch("settingsDABNotch", 920, 335, 230, 34, "DAB NOTCH", false, 14)
	modal.iqCorrection = simpleui.NewSwitch("settingsIQCorrection", 920, 385, 230, 34, "IQ CORRECTION", true, 14)
	modal.setpointLabel = label("settingsSetpointLabel", 920, 458, 230, "AGC SETPOINT", 13)
	modal.setpoint = simpleui.NewSlider("settingsSetpoint", 920, 492, 230, 26, -60, 0, -30)
	modal.setpoint.SetStep(1)

	modal.cancel = simpleui.NewButton("settingsCancel", 562, 566, 220, 46, "CANCEL", 16)
	modal.apply = simpleui.NewButton("settingsApply", 818, 566, 220, 46, "APPLY", 16)
	modal.cancel.OnClick(func() { modal.set(modal.original); modal.Close() })
	modal.apply.OnClick(modal.Close)

	modal.agc.OnChange(func(value bool) { modal.current.AGC = value; modal.refresh(); modal.submit() })
	modal.biasT.OnChange(func(value bool) { modal.current.BiasT = value; modal.submit() })
	modal.rfNotch.OnChange(func(value bool) { modal.current.RFNotch = value; modal.submit() })
	modal.dabNotch.OnChange(func(value bool) { modal.current.DABNotch = value; modal.submit() })
	modal.iqCorrection.OnChange(func(value bool) { modal.current.IQCorrection = value; modal.submit() })
	modal.rfGain.OnChange(func(value float32) { modal.current.RFGain = value; modal.refreshLabels() })
	modal.rfGain.OnRelease(func(float32) { modal.submit() })
	modal.ifGain.OnChange(func(value float32) { modal.current.IFGain = value; modal.refreshLabels() })
	modal.ifGain.OnRelease(func(float32) { modal.submit() })
	modal.ppm.OnChange(func(value float32) { modal.current.PPM = value; modal.refreshLabels() })
	modal.ppm.OnRelease(func(float32) { modal.submit() })
	modal.setpoint.OnChange(func(value float32) {
		modal.current.AGCSetpoint = int(math.Round(float64(value)))
		modal.refreshLabels()
	})
	modal.setpoint.OnRelease(func(float32) { modal.submit() })

	modal.controls = []simpleui.Element{
		modal.deviceLabel, modal.antennaLabel, modal.antenna[0], modal.antenna[1], modal.antenna[2],
		modal.agc, modal.rfLabel, modal.rfGain, modal.ifLabel, modal.ifGain,
		modal.ppmLabel, modal.ppm, modal.biasT, modal.rfNotch, modal.dabNotch,
		modal.iqCorrection, modal.setpointLabel, modal.setpoint, modal.cancel, modal.apply,
	}
}

func (modal *SDRSettings) Open() {
	if modal.receiver != nil {
		modal.original = modal.receiver.HardwareSettings()
	}
	modal.current = modal.original
	modal.open = true
	modal.nextSync = rl.GetTime() + .10
	modal.refresh()
}

func (modal *SDRSettings) Close()                     { modal.open = false }
func (modal *SDRSettings) OverlayOpen() bool          { return modal.open }
func (modal *SDRSettings) Update(simpleui.Input) bool { return false }
func (modal *SDRSettings) Draw()                      {}

func (modal *SDRSettings) UpdateOverlay(input simpleui.Input) bool {
	if !modal.open {
		return false
	}
	if modal.receiver != nil && !input.Down && rl.GetTime() >= modal.nextSync {
		modal.current = modal.receiver.HardwareSettings()
		modal.refresh()
		modal.nextSync = rl.GetTime() + .10
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		modal.set(modal.original)
		modal.Close()
		return true
	}
	for index := len(modal.controls) - 1; index >= 0; index-- {
		control := modal.controls[index]
		if control.Visible() && control.Enabled() && control.Update(input) {
			return true
		}
	}
	return true
}

func (modal *SDRSettings) DrawOverlay() {
	if !modal.open {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 205})
	panel := rl.Rectangle{X: 420, Y: 100, Width: 760, Height: 550}
	rl.DrawRectangleRounded(panel, .025, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(panel, .025, 8, 2, colors.border)
	rl.DrawRectangleRounded(rl.Rectangle{X: 420, Y: 100, Width: 10, Height: 550}, .5, 8, colors.blue)
	drawCentered(T("SDR SETTINGS"), rl.Rectangle{X: 460, Y: 116, Width: 680, Height: 38}, 25, colors.text)
	rl.DrawLineEx(rl.Vector2{X: 465, Y: 195}, rl.Vector2{X: 1135, Y: 195}, 2, colors.border)
	if !modal.current.Available {
		drawCentered(T("No physical receiver is active."), rl.Rectangle{X: 500, Y: 300, Width: 600, Height: 40}, 18, colors.orange)
		modal.cancel.SetLabel(T("CANCEL"))
		modal.cancel.Draw()
		return
	}
	for _, control := range modal.controls {
		control.Draw()
	}
	drawCentered(T("BIAS-T SUPPLIES VOLTAGE ON THE ANTENNA CONNECTOR"), rl.Rectangle{X: 880, Y: 525, Width: 300, Height: 24}, 10, colors.orange)
}

func (modal *SDRSettings) set(settings sdr.HardwareSettings) {
	modal.current = settings
	modal.refresh()
	modal.submit()
}

func (modal *SDRSettings) submit() {
	if modal.receiver != nil && modal.current.Available {
		modal.receiver.ApplyHardwareSettings(modal.current)
	}
}

func (modal *SDRSettings) refresh() {
	settings := modal.current
	profile := sdr.ProfileFor(settings.Driver)
	rtl := profile == sdr.ProfileRTLSDR
	generic := profile == sdr.ProfileGeneric
	modal.deviceLabel.SetText(settings.Device + "   DRIVER " + settings.Driver)
	modal.agc.SetActive(settings.AGC)
	modal.biasT.SetActive(settings.BiasT)
	modal.rfNotch.SetActive(settings.RFNotch)
	modal.dabNotch.SetActive(settings.DABNotch)
	modal.iqCorrection.SetActive(settings.IQCorrection)
	modal.rfGain.SetValue(settings.RFGain)
	modal.ifGain.SetValue(settings.IFGain)
	modal.ppm.SetValue(settings.PPM)
	modal.setpoint.SetValue(float32(settings.AGCSetpoint))
	switch {
	case rtl:
		modal.rfGain.SetRange(0, 49.6)
		modal.ifGain.SetEnabled(false)
		modal.setpoint.SetEnabled(false)
		modal.rfNotch.SetEnabled(false)
		modal.dabNotch.SetEnabled(false)
		modal.iqCorrection.SetEnabled(false)
	case generic:
		modal.rfGain.SetRange(0, 80)
		modal.ifGain.SetEnabled(false)
		modal.setpoint.SetEnabled(false)
		modal.rfNotch.SetEnabled(false)
		modal.dabNotch.SetEnabled(false)
		modal.iqCorrection.SetEnabled(false)
	default:
		modal.rfGain.SetRange(0, 9)
		modal.ifGain.SetEnabled(settings.Available && !settings.AGC)
		modal.setpoint.SetEnabled(settings.Available)
		modal.rfNotch.SetEnabled(settings.Available)
		modal.dabNotch.SetEnabled(settings.Available)
		modal.iqCorrection.SetEnabled(settings.Available)
	}
	modal.rfGain.SetEnabled(settings.Available && !settings.AGC)
	showAntenna := settings.Available && len(settings.Antennas) >= 2
	modal.antennaLabel.SetVisible(showAntenna)
	selected := sdr.AntennaShortLabel(settings.Antenna)
	theme := simpleui.CurrentTheme()
	for index, button := range modal.antenna {
		if !showAntenna || index >= len(settings.Antennas) {
			button.SetVisible(false)
			button.SetEnabled(false)
			continue
		}
		button.SetVisible(true)
		button.SetEnabled(true)
		button.SetLabel(sdr.AntennaShortLabel(settings.Antennas[index]))
		if sdr.AntennaShortLabel(settings.Antennas[index]) == selected {
			button.SetColors(theme.ControlPressed, theme.Accent, theme.Text)
		} else {
			button.ClearColors()
		}
	}
	for _, control := range modal.controls {
		if control == modal.antennaLabel || control == modal.antenna[0] || control == modal.antenna[1] || control == modal.antenna[2] {
			continue
		}
		control.SetVisible(settings.Available)
	}
	modal.cancel.SetVisible(true)
	modal.refreshLabels()
}

func (modal *SDRSettings) refreshLabels() {
	switch sdr.ProfileFor(modal.current.Driver) {
	case sdr.ProfileRTLSDR:
		modal.rfLabel.SetText(fmt.Sprintf("%s   %.1f dB", T("TUNER"), modal.current.RFGain))
		modal.ifLabel.SetText(T("IFGR · N/A"))
		modal.setpointLabel.SetText(T("AGC SETPOINT · N/A"))
	case sdr.ProfileGeneric:
		modal.rfLabel.SetText(fmt.Sprintf("%s   %.1f dB", T("GAIN"), modal.current.RFGain))
		modal.ifLabel.SetText(T("IFGR · N/A"))
		modal.setpointLabel.SetText(T("AGC SETPOINT · N/A"))
	default:
		modal.rfLabel.SetText(fmt.Sprintf("%s   STATUS %.0f", T("LNA / RFGR"), modal.current.RFGain))
		ifText := fmt.Sprintf("%s   %.0f dB", T("IFGR"), modal.current.IFGain)
		if modal.current.AGC {
			ifText += "   (" + T("AGC CONTROLLED") + ")"
		}
		modal.ifLabel.SetText(ifText)
		modal.setpointLabel.SetText(fmt.Sprintf("%s   %d dB", T("AGC SETPOINT"), modal.current.AGCSetpoint))
	}
	modal.ppmLabel.SetText(fmt.Sprintf("%s   %.1f ppm", T("FREQUENCY CORRECTION"), modal.current.PPM))
}

func (modal *SDRSettings) refreshLocale() {
	modal.agc.SetLabel(T("AGC"))
	modal.biasT.SetLabel(T("BIAS-T"))
	modal.antennaLabel.SetText(T("ANTENNA"))
	modal.rfNotch.SetLabel(T("RF NOTCH"))
	modal.dabNotch.SetLabel(T("DAB NOTCH"))
	modal.iqCorrection.SetLabel(T("IQ CORRECTION"))
	modal.cancel.SetLabel(T("CANCEL"))
	modal.apply.SetLabel(T("APPLY"))
	modal.refreshLabels()
}
