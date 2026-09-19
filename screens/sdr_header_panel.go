package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"math"
	"sync/atomic"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/sdr"
	"go-zero/simpleui"
)

type SDRHeaderPanel struct {
	receiver                                    *sdr.Receiver
	controls                                    []simpleui.Element
	current                                     sdr.HardwareSettings
	devices                                     []sdr.DeviceOption
	nextSync, nextList                          float64
	listing, selecting                          atomic.Bool
	onChanged                                   func()
	deviceSelect                                *simpleui.Dropdown
	rfLabel, ifLabel, ppmLabel, setpointLabel   *simpleui.Label
	agc, biasT, iqCorrection, rfNotch, dabNotch *simpleui.Switch
	rfGain, ifGain, ppm, setpoint               *simpleui.Slider
	antenna                                     [3]*simpleui.Button
}

func NewSDRHeaderPanel(receiver *sdr.Receiver, onChanged func()) *SDRHeaderPanel {
	p := &SDRHeaderPanel{receiver: receiver, onChanged: onChanged}
	label := func(id string, x, y, w float32) *simpleui.Label {
		v := simpleui.NewLabel(id, x, y, w, 18, "", uiMinimumFontSize)
		v.SetAlignment(simpleui.AlignCenter)
		return v
	}
	p.deviceSelect = simpleui.NewDropdown("headerSDRDevice", 1268, 5, 304, 22, i18n.Source("text.33af30ac8d27"), []string{i18n.Source("text.74f2b39784ef")}, 11)
	p.deviceSelect.SetMaxVisibleItems(6)
	p.deviceSelect.OnChange(func(index int, _ string) {
		if p.receiver == nil || index < 0 || index >= len(p.devices) || !p.selecting.CompareAndSwap(false, true) {
			return
		}
		option := p.devices[index]
		go func() { defer p.selecting.Store(false); _ = p.receiver.SelectDevice(option.Driver, option.Serial) }()
	})
	p.agc = simpleui.NewSwitch("headerSDRAGC", 1270, 27, 92, 27, i18n.Source("text.20e0541e8b46"), false, 12)
	p.biasT = simpleui.NewSwitch("headerSDRBiasT", 1370, 27, 92, 27, i18n.Source("text.70ca67554bd7"), false, 12)
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
	p.rfNotch = simpleui.NewSwitch("headerSDRRFNotch", 1270, 150, 140, 24, i18n.Source("text.5acff90d01d7"), false, 11)
	p.dabNotch = simpleui.NewSwitch("headerSDRDABNotch", 1420, 150, 150, 24, i18n.Source("text.3d4899209b46"), false, 11)
	for index := range p.antenna {
		i := index
		p.antenna[i] = simpleui.NewButton(fmt.Sprintf("headerSDRAntenna%d", i), 1268+float32(i)*102, 148, 98, 24, i18n.Source("text.417966420c63")+[]string{"A", "B", "C"}[i], 11)
		p.antenna[i].SetVisible(false)
		p.antenna[i].OnClick(func() {
			if i >= len(p.current.Antennas) {
				return
			}
			p.current.Antenna = p.current.Antennas[i]
			p.submit()
			p.refresh()
		})
	}
	p.agc.OnChange(func(v bool) {
		if p.current.Driver == "hackrf" {
			p.current.ExternalAmp = v
		} else {
			p.current.AGC = v
		}
		p.submit()
		p.refresh()
	})
	p.biasT.OnChange(func(v bool) { p.current.BiasT = v; p.submit() })
	p.iqCorrection.OnChange(func(v bool) {
		if p.current.Driver == "rtlsdr" {
			p.current.DigitalAGC = v
		} else {
			p.current.IQCorrection = v
		}
		p.submit()
	})
	p.rfNotch.OnChange(func(v bool) {
		if p.current.Driver == "rtlsdr" {
			p.current.OffsetTuning = v
		} else {
			p.current.RFNotch = v
		}
		p.submit()
	})
	p.dabNotch.OnChange(func(v bool) {
		if p.current.Driver == "rtlsdr" {
			p.current.IQSwap = v
		} else {
			p.current.DABNotch = v
		}
		p.submit()
	})
	p.rfGain.OnChange(func(v float32) { p.current.RFGain = v; p.refreshLabels() })
	p.rfGain.OnRelease(func(float32) { p.submit() })
	p.ifGain.OnChange(func(v float32) {
		if p.current.Driver == "rtlsdr" {
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
	p.controls = []simpleui.Element{p.deviceSelect, p.agc, p.biasT, p.iqCorrection, p.rfLabel, p.ifLabel, p.rfGain, p.ifGain, p.ppmLabel, p.setpointLabel, p.ppm, p.setpoint, p.antenna[0], p.antenna[1], p.antenna[2], p.rfNotch, p.dabNotch}
	p.sync()
	return p
}

func (p *SDRHeaderPanel) Tick() {
	now := rl.GetTime()
	if p.receiver != nil && !p.selecting.Load() && now >= p.nextList && p.listing.CompareAndSwap(false, true) {
		p.nextList = now + 8
		go func() { defer p.listing.Store(false); _ = p.receiver.ListDevices() }()
	}
	if p.receiver != nil && !rl.IsMouseButtonDown(rl.MouseButtonLeft) && rl.GetTime() >= p.nextSync {
		p.sync()
		p.nextSync = rl.GetTime() + .25
	}
}
func (p *SDRHeaderPanel) DrawBackground() { drawPanel(1264, 4, 312, 198) }
func (p *SDRHeaderPanel) sync() {
	if p.receiver != nil {
		p.current = p.receiver.HardwareSettings()
		// Never replace the rows while the user is clicking the popup. Device
		// discovery is asynchronous and used to make a freshly found HackRF
		// move underneath the pointer between press and release.
		if !p.deviceSelect.Open() {
			p.syncDevices()
		}
	}
	p.refresh()
}

func sameSDRDevices(a, b []sdr.DeviceOption) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func (p *SDRHeaderPanel) syncDevices() {
	list := p.receiver.CachedDevices()
	p.devices = list
	if len(list) == 0 {
		p.deviceSelect.SetItems([]string{i18n.Source("text.74f2b39784ef")})
		p.deviceSelect.SetSelected(-1)
		return
	}
	labels := make([]string, len(list))
	selected := -1
	preferred := p.receiver.SelectedDevice()
	for index, option := range list {
		labels[index] = sdr.FormatDeviceLabel(option)
		if option.Driver == p.current.Driver && (preferred.Serial == "" || option.Serial == preferred.Serial) {
			selected = index
		}
	}
	p.deviceSelect.SetItems(labels)
	p.deviceSelect.SetSelected(selected)
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
	rtl := s.Driver == "rtlsdr"
	hackrf := s.Driver == "hackrf"
	hasAntennas := s.Available && sdr.IsRSPDx(s.Device) && len(s.Antennas) > 1
	p.agc.SetActive(s.AGC)
	p.agc.SetLabel(i18n.Source("text.20e0541e8b46"))
	p.biasT.SetActive(s.BiasT)
	p.iqCorrection.SetActive(s.IQCorrection)
	p.rfNotch.SetActive(s.RFNotch)
	p.dabNotch.SetActive(s.DABNotch)
	if rtl {
		p.iqCorrection.SetLabel(i18n.Source("text.bfc861f154f6"))
		p.rfNotch.SetLabel(i18n.Source("text.82047faf5f26"))
		p.dabNotch.SetLabel(i18n.Source("text.0d2f9ea039e6"))
		p.iqCorrection.SetActive(s.DigitalAGC)
		p.rfNotch.SetActive(s.OffsetTuning)
		p.dabNotch.SetActive(s.IQSwap)
		p.rfGain.SetRange(0, 49.6)
		p.rfGain.SetStep(.1)
		p.ifGain.SetRange(0, 2)
		p.ifGain.SetStep(1)
	} else if hackrf {
		p.agc.SetLabel("AMP")
		p.agc.SetActive(s.ExternalAmp)
		p.rfGain.SetRange(0, 40)
		p.rfGain.SetStep(8)
		p.ifGain.SetRange(0, 62)
		p.ifGain.SetStep(2)
	} else {
		p.iqCorrection.SetLabel("IQ")
		p.rfNotch.SetLabel(i18n.Source("text.5acff90d01d7"))
		p.dabNotch.SetLabel(i18n.Source("text.3d4899209b46"))
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
	p.deviceSelect.SetEnabled(true)
	for i, button := range p.antenna {
		button.SetVisible(hasAntennas && i < len(s.Antennas))
		if i < len(s.Antennas) {
			label := sdr.AntennaShortLabel(s.Antennas[i])
			button.SetLabel(i18n.Source("text.417966420c63") + label)
			if sdr.MatchAntenna(s.Antenna, s.Antennas) == s.Antennas[i] {
				button.SetColors(colors.green, colors.green, colors.text)
			} else {
				button.ClearColors()
			}
		}
	}
	bottomY := float32(150)
	if hasAntennas {
		bottomY = 175
	}
	for _, control := range []*simpleui.Switch{p.rfNotch, p.dabNotch} {
		bounds := control.Bounds()
		bounds.Y = bottomY
		control.SetBounds(bounds)
		control.SetVisible(!hackrf)
	}
	if hackrf {
		for _, control := range []simpleui.Element{p.iqCorrection, p.ppm, p.setpoint, p.rfNotch, p.dabNotch} {
			control.SetEnabled(false)
		}
	}
	if hackrf {
		p.agc.SetEnabled(s.Available)
		p.biasT.SetEnabled(s.Available)
		p.ifGain.SetEnabled(s.Available)
	} else if rtl {
		p.ifGain.SetEnabled(s.Available)
		p.setpoint.SetEnabled(false)
	} else {
		p.ifGain.SetEnabled(s.Available && !s.AGC)
	}
	p.rfGain.SetEnabled(s.Available && (hackrf || (!s.AGC && (!rtl || s.DirectSampling == 0))))
	p.refreshLabels()
}
func (p *SDRHeaderPanel) refreshLabels() {
	p.ppmLabel.SetText(fmt.Sprintf(i18n.Source("text.f2033d5208fc"), p.current.PPM))
	if p.current.Driver == "hackrf" {
		p.rfLabel.SetText(fmt.Sprintf("LNA  %.0f dB", p.current.RFGain))
		p.ifLabel.SetText(fmt.Sprintf("VGA  %.0f dB", p.current.IFGain))
		p.ppmLabel.SetText(i18n.Source("text.6f41a0c5df05"))
		p.setpointLabel.SetText(i18n.Source("text.eb6ef01999d1"))
		return
	}
	if p.current.Driver == "rtlsdr" {
		p.rfLabel.SetText(fmt.Sprintf(i18n.Source("text.fdcfa6474a00"), p.current.RFGain))
		direct := []string{i18n.Source("text.d11dd64cc044"), i18n.Source("text.236470f480c2"), i18n.Source("text.feface07d305")}
		mode := min(max(p.current.DirectSampling, 0), 2)
		p.ifLabel.SetText(direct[mode])
		p.setpointLabel.SetText(i18n.Source("text.eb6ef01999d1"))
		return
	}
	p.rfLabel.SetText(fmt.Sprintf(i18n.Source("text.c8a345893149"), p.current.RFGain))
	if p.current.AGC {
		p.ifLabel.SetText(i18n.Source("text.4c13d2f948d8"))
	} else {
		p.ifLabel.SetText(fmt.Sprintf(i18n.Source("text.9a31f847fa9d"), p.current.IFGain))
	}
	p.setpointLabel.SetText(fmt.Sprintf(i18n.Source("text.90f23be3b348"), p.current.AGCSetpoint))
}
