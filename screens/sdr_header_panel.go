package screens

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/sdr"
	"go-zero/simpleui"
)

type SDRHeaderPanel struct {
	receiver                                          *sdr.Receiver
	controls                                          []simpleui.Element
	current                                           sdr.HardwareSettings
	nextSync                                          float64
	onChanged                                         func()
	status, rfLabel, ifLabel, ppmLabel, setpointLabel *simpleui.Label
	agc, biasT, iqCorrection, rfNotch, dabNotch       *simpleui.Switch
	rfGain, ifGain, ppm, setpoint                     *simpleui.Slider
	antenna                                           [3]*simpleui.Button
	dcSpike                                           *simpleui.Switch
}

func NewSDRHeaderPanel(receiver *sdr.Receiver, onChanged func()) *SDRHeaderPanel {
	p := &SDRHeaderPanel{receiver: receiver, onChanged: onChanged}
	label := func(id string, x, y, w float32) *simpleui.Label {
		v := simpleui.NewLabel(id, x, y, w, 18, "", uiMinimumFontSize)
		v.SetAlignment(simpleui.AlignCenter)
		return v
	}
	p.status = label("headerSDRStatus", 1268, 5, 304)
	p.agc = simpleui.NewSwitch("headerSDRAGC", 1270, 27, 92, 27, "AGC", false, 12)
	p.biasT = simpleui.NewSwitch("headerSDRBiasT", 1370, 27, 92, 27, "BIAS-T", false, 12)
	p.iqCorrection = simpleui.NewSwitch("headerSDRIQ", 1470, 27, 100, 27, "IQ", true, 12)
	p.rfLabel = label("headerSDRRFLabel", 1268, 57, 145)
	p.ifLabel = label("headerSDRIFLabel", 1420, 57, 150)
	p.rfGain = simpleui.NewSlider("headerSDRRF", 1272, 76, 137, 18, 0, 9, 0)
	p.rfGain.SetStep(1)
	p.ifGain = simpleui.NewSlider("headerSDRIF", 1424, 76, 142, 18, 20, 59, 40)
	p.ifGain.SetStep(1)
	p.ppmLabel = label("headerSDRPPMLabel", 1268, 101, 145)
	p.setpointLabel = label("headerSDRSetLabel", 1420, 101, 150)
	p.ppm = simpleui.NewSlider("headerSDRPPM", 1272, 120, 137, 18, -100, 100, 0)
	p.ppm.SetStep(.1)
	p.setpoint = simpleui.NewSlider("headerSDRSetpoint", 1424, 120, 142, 18, -60, 0, -30)
	p.setpoint.SetStep(1)
	p.rfNotch = simpleui.NewSwitch("headerSDRRFNotch", 1270, 174, 140, 24, "RF NOTCH", false, 11)
	p.dabNotch = simpleui.NewSwitch("headerSDRDABNotch", 1420, 174, 150, 24, "DAB NOTCH", false, 11)
	p.dcSpike = simpleui.NewSwitch("headerSDRDC", 1268, 148, 98, 24, "DC SPIKE", true, 11)
	for index := range p.antenna {
		i := index
		p.antenna[i] = simpleui.NewButton("headerSDRAntenna"+fmt.Sprintf("%d", i), 1268+float32(i)*102, 148, 98, 24, []string{"A", "B", "C"}[i], 12)
		p.antenna[i].OnClick(func() {
			if i >= len(p.current.Antennas) {
				return
			}
			p.current.Antenna = p.current.Antennas[i]
			p.submit()
			p.refresh()
		})
	}
	p.agc.OnChange(func(v bool) { p.current.AGC = v; p.submit(); p.refresh() })
	p.biasT.OnChange(func(v bool) { p.current.BiasT = v; p.submit() })
	p.iqCorrection.OnChange(func(v bool) {
		if sdr.ProfileFor(p.current.Driver) == sdr.ProfileRTLSDR {
			p.current.DigitalAGC = v
		} else {
			p.current.IQCorrection = v
		}
		p.submit()
	})
	p.rfNotch.OnChange(func(v bool) {
		if sdr.ProfileFor(p.current.Driver) == sdr.ProfileRTLSDR {
			p.current.OffsetTuning = v
		} else {
			p.current.RFNotch = v
		}
		p.submit()
	})
	p.dabNotch.OnChange(func(v bool) {
		if sdr.ProfileFor(p.current.Driver) == sdr.ProfileRTLSDR {
			p.current.IQSwap = v
		} else {
			p.current.DABNotch = v
		}
		p.submit()
	})
	p.dcSpike.OnChange(func(v bool) {
		if p.receiver != nil {
			p.receiver.SetRemoveDCSpike(v)
		}
		if p.onChanged != nil {
			p.onChanged()
		}
	})
	p.rfGain.OnChange(func(v float32) { p.current.RFGain = v; p.refreshLabels() })
	p.rfGain.OnRelease(func(float32) { p.submit() })
	p.ifGain.OnChange(func(v float32) {
		if sdr.ProfileFor(p.current.Driver) == sdr.ProfileRTLSDR {
			p.current.DirectSampling = int(math.Round(float64(v)))
		} else {
			p.current.IFGain = v
		}
		p.refreshLabels()
	})
	p.ifGain.OnRelease(func(float32) { p.submit() })
	p.ppm.OnChange(func(v float32) { p.current.PPM = v; p.refreshLabels() })
	p.ppm.OnRelease(func(float32) { p.submit() })
	p.setpoint.OnChange(func(v float32) { p.current.AGCSetpoint = int(math.Round(float64(v))); p.refreshLabels() })
	p.setpoint.OnRelease(func(float32) { p.submit() })
	p.controls = []simpleui.Element{p.status, p.agc, p.biasT, p.iqCorrection, p.rfLabel, p.ifLabel, p.rfGain, p.ifGain, p.ppmLabel, p.setpointLabel, p.ppm, p.setpoint, p.antenna[0], p.antenna[1], p.antenna[2], p.dcSpike, p.rfNotch, p.dabNotch}
	p.sync()
	return p
}

func (p *SDRHeaderPanel) Tick() {
	if p.receiver != nil && !rl.IsMouseButtonDown(rl.MouseButtonLeft) && rl.GetTime() >= p.nextSync {
		p.sync()
		p.nextSync = rl.GetTime() + .25
	}
}
func (p *SDRHeaderPanel) DrawBackground() { drawPanel(1264, 4, 312, 206) }
func (p *SDRHeaderPanel) sync() {
	if p.receiver != nil {
		p.current = p.receiver.HardwareSettings()
	}
	p.refresh()
}
func (p *SDRHeaderPanel) submit() {
	if p.receiver != nil && p.current.Available {
		p.receiver.ApplyHardwareSettings(p.current)
		if p.onChanged != nil {
			p.onChanged()
		}
	}
}
func (p *SDRHeaderPanel) refresh() {
	s := p.current
	profile := sdr.ProfileFor(s.Driver)
	rtl := profile == sdr.ProfileRTLSDR
	generic := profile == sdr.ProfileGeneric
	status := T("SDR · NO DEVICE")
	if s.Available {
		status = fmt.Sprintf("SDR %s · %s", s.Device, s.Driver)
	}
	p.status.SetText(status)
	p.agc.SetActive(s.AGC)
	p.biasT.SetActive(s.BiasT)
	p.iqCorrection.SetActive(s.IQCorrection)
	p.rfNotch.SetActive(s.RFNotch)
	p.dabNotch.SetActive(s.DABNotch)
	if p.receiver != nil {
		p.dcSpike.SetActive(p.receiver.RemoveDCSpike())
	}
	switch {
	case rtl:
		p.iqCorrection.SetLabel(T("D-AGC"))
		p.rfNotch.SetLabel(T("OFFSET"))
		p.dabNotch.SetLabel(T("IQ SWAP"))
		p.iqCorrection.SetActive(s.DigitalAGC)
		p.rfNotch.SetActive(s.OffsetTuning)
		p.dabNotch.SetActive(s.IQSwap)
		p.rfGain.SetRange(0, 49.6)
		p.rfGain.SetStep(.1)
		p.ifGain.SetRange(0, 2)
		p.ifGain.SetStep(1)
	case generic:
		p.iqCorrection.SetLabel(T("IQ"))
		p.rfNotch.SetLabel(T("RF NOTCH"))
		p.dabNotch.SetLabel(T("DAB NOTCH"))
		p.rfGain.SetRange(0, 116)
		p.rfGain.SetStep(.5)
		p.ifGain.SetRange(0, 80)
		p.ifGain.SetStep(.5)
	default:
		p.iqCorrection.SetLabel(T("IQ"))
		p.rfNotch.SetLabel(T("RF NOTCH"))
		p.dabNotch.SetLabel(T("DAB NOTCH"))
		p.rfGain.SetRange(0, 9)
		p.rfGain.SetStep(1)
		p.ifGain.SetRange(20, 59)
		p.ifGain.SetStep(1)
	}
	p.rfGain.SetValue(s.RFGain)
	if rtl {
		p.ifGain.SetValue(float32(s.DirectSampling))
	} else {
		p.ifGain.SetValue(s.IFGain)
	}
	p.ppm.SetValue(s.PPM)
	p.setpoint.SetValue(float32(s.AGCSetpoint))
	for _, c := range p.controls {
		c.SetEnabled(s.Available)
	}
	p.status.SetEnabled(true)
	switch {
	case rtl:
		p.ifGain.SetEnabled(s.Available)
		p.setpoint.SetEnabled(false)
	case generic:
		p.ifGain.SetEnabled(false)
		p.setpoint.SetEnabled(false)
		p.iqCorrection.SetEnabled(false)
		p.rfNotch.SetEnabled(false)
		p.dabNotch.SetEnabled(false)
	default:
		p.ifGain.SetEnabled(s.Available && !s.AGC)
	}
	p.rfGain.SetEnabled(s.Available && !s.AGC && (!rtl || s.DirectSampling == 0))
	p.dcSpike.SetEnabled(true)
	p.refreshAntennaButtons()
	p.refreshLabels()
}

func (p *SDRHeaderPanel) refreshAntennaButtons() {
	ports := p.current.Antennas
	show := p.current.Available && len(ports) >= 2
	selected := sdr.AntennaShortLabel(p.current.Antenna)
	theme := simpleui.CurrentTheme()
	for index, button := range p.antenna {
		if !show || index >= len(ports) {
			button.SetVisible(false)
			button.SetEnabled(false)
			continue
		}
		button.SetVisible(true)
		button.SetEnabled(true)
		button.SetLabel(sdr.AntennaShortLabel(ports[index]))
		if sdr.AntennaShortLabel(ports[index]) == selected {
			button.SetColors(theme.ControlPressed, theme.Accent, theme.Text)
		} else {
			button.ClearColors()
		}
	}
	rowY := float32(148)
	if show {
		rowY = 174
	}
	p.dcSpike.SetBounds(rl.Rectangle{X: 1268, Y: rowY, Width: 98, Height: 24})
	p.rfNotch.SetBounds(rl.Rectangle{X: 1370, Y: rowY, Width: 92, Height: 24})
	p.dabNotch.SetBounds(rl.Rectangle{X: 1466, Y: rowY, Width: 98, Height: 24})
}
func (p *SDRHeaderPanel) refreshLabels() {
	p.ppmLabel.SetText(fmt.Sprintf("%s  %+.1f", T("PPM"), p.current.PPM))
	switch sdr.ProfileFor(p.current.Driver) {
	case sdr.ProfileRTLSDR:
		p.rfLabel.SetText(fmt.Sprintf("%s  %.1f dB", T("TUNER"), p.current.RFGain))
		direct := []string{T("DIRECT · OFF"), T("DIRECT · I"), T("DIRECT · Q")}
		mode := min(max(p.current.DirectSampling, 0), 2)
		p.ifLabel.SetText(direct[mode])
		p.setpointLabel.SetText(T("SETPOINT · N/A"))
	case sdr.ProfileGeneric:
		p.rfLabel.SetText(fmt.Sprintf("%s  %.1f dB", T("GAIN"), p.current.RFGain))
		p.ifLabel.SetText(T("IFGR · N/A"))
		p.setpointLabel.SetText(T("SETPOINT · N/A"))
	default:
		p.rfLabel.SetText(fmt.Sprintf("%s  %.0f", T("RF / LNA"), p.current.RFGain))
		if p.current.AGC {
			p.ifLabel.SetText(T("IFGR · AGC"))
		} else {
			p.ifLabel.SetText(fmt.Sprintf("%s  %.0f dB", T("IFGR"), p.current.IFGain))
		}
		p.setpointLabel.SetText(fmt.Sprintf("%s  %d dB", T("SETPOINT"), p.current.AGCSetpoint))
	}
}

func (p *SDRHeaderPanel) refreshLocale() {
	p.agc.SetLabel(T("AGC"))
	p.biasT.SetLabel(T("BIAS-T"))
	p.dcSpike.SetLabel(T("DC SPIKE"))
	p.refresh()
}
