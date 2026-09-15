package screens

import (
	"go-zero/internal/i18n"

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

	deviceLabel, rfLabel, ifLabel, ppmLabel, setpointLabel *simpleui.Label
	agc, biasT, rfNotch, dabNotch, iqCorrection            *simpleui.Switch
	rfGain, ifGain, ppm, setpoint                          *simpleui.Slider
	apply, cancel                                          *simpleui.Button
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
	modal.agc = simpleui.NewSwitch("settingsAGC", 500, 235, 300, 34, i18n.Source("text.20e0541e8b46"), false, 16)
	modal.rfLabel = label("settingsRFLabel", 500, 298, 360, i18n.Source("text.095f555474b3"), 14)
	modal.rfGain = simpleui.NewSlider("settingsRF", 500, 332, 360, 26, 0, 9, 0)
	modal.rfGain.SetStep(1)
	modal.ifLabel = label("settingsIFLabel", 500, 378, 360, i18n.Source("text.beb717ff2ec6"), 14)
	modal.ifGain = simpleui.NewSlider("settingsIF", 500, 412, 360, 26, 20, 59, 40)
	modal.ifGain.SetStep(1)
	modal.ppmLabel = label("settingsPPMLabel", 500, 458, 360, i18n.Source("text.dba49abd6613"), 14)
	modal.ppm = simpleui.NewSlider("settingsPPM", 500, 492, 360, 26, -100, 100, 0)
	modal.ppm.SetStep(.1)

	modal.biasT = simpleui.NewSwitch("settingsBiasT", 920, 235, 230, 34, i18n.Source("text.70ca67554bd7"), false, 14)
	modal.rfNotch = simpleui.NewSwitch("settingsRFNotch", 920, 285, 230, 34, i18n.Source("text.5acff90d01d7"), false, 14)
	modal.dabNotch = simpleui.NewSwitch("settingsDABNotch", 920, 335, 230, 34, i18n.Source("text.3d4899209b46"), false, 14)
	modal.iqCorrection = simpleui.NewSwitch("settingsIQCorrection", 920, 385, 230, 34, i18n.Source("text.031db0592584"), true, 14)
	modal.setpointLabel = label("settingsSetpointLabel", 920, 458, 230, i18n.Source("text.7a336a088dcf"), 13)
	modal.setpoint = simpleui.NewSlider("settingsSetpoint", 920, 492, 230, 26, -60, 0, -30)
	modal.setpoint.SetStep(1)

	modal.cancel = simpleui.NewButton("settingsCancel", 562, 566, 220, 46, i18n.Source("text.b1a5fe65d180"), 16)
	modal.apply = simpleui.NewButton("settingsApply", 818, 566, 220, 46, i18n.Source("text.3db550736ade"), 16)
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
		modal.deviceLabel, modal.agc, modal.rfLabel, modal.rfGain, modal.ifLabel, modal.ifGain,
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
	drawCentered(i18n.Source("text.67d049bf5898"), rl.Rectangle{X: 460, Y: 116, Width: 680, Height: 38}, 25, colors.text)
	rl.DrawLineEx(rl.Vector2{X: 465, Y: 195}, rl.Vector2{X: 1135, Y: 195}, 2, colors.border)
	if !modal.current.Available {
		drawCentered(i18n.Source("text.fc95502ea369"), rl.Rectangle{X: 500, Y: 300, Width: 600, Height: 40}, 18, colors.orange)
		modal.cancel.Draw()
		return
	}
	for _, control := range modal.controls {
		control.Draw()
	}
	drawCentered(i18n.Source("text.7d52529a55cb"), rl.Rectangle{X: 880, Y: 525, Width: 300, Height: 24}, 10, colors.orange)
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
	modal.deviceLabel.SetText(settings.Device + i18n.Source("text.b9b13a2d3caa") + settings.Driver)
	modal.agc.SetActive(settings.AGC)
	modal.biasT.SetActive(settings.BiasT)
	modal.rfNotch.SetActive(settings.RFNotch)
	modal.dabNotch.SetActive(settings.DABNotch)
	modal.iqCorrection.SetActive(settings.IQCorrection)
	modal.rfGain.SetValue(settings.RFGain)
	modal.ifGain.SetValue(settings.IFGain)
	modal.ppm.SetValue(settings.PPM)
	modal.setpoint.SetValue(float32(settings.AGCSetpoint))
	modal.ifGain.SetEnabled(settings.Available && !settings.AGC)
	for _, control := range modal.controls {
		control.SetVisible(settings.Available)
	}
	modal.cancel.SetVisible(true)
	modal.refreshLabels()
}

func (modal *SDRSettings) refreshLabels() {
	modal.rfLabel.SetText(fmt.Sprintf(i18n.Source("text.b369a319f723"), modal.current.RFGain))
	ifText := fmt.Sprintf(i18n.Source("text.c2bf6f633050"), modal.current.IFGain)
	if modal.current.AGC {
		ifText += i18n.Source("text.a90391bfa992")
	}
	modal.ifLabel.SetText(ifText)
	modal.ppmLabel.SetText(fmt.Sprintf(i18n.Source("text.4dcb606b34dd"), modal.current.PPM))
	modal.setpointLabel.SetText(fmt.Sprintf(i18n.Source("text.99f10ecbee15"), modal.current.AGCSetpoint))
}
