package screens

import (
	"go-zero/internal/i18n"

	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go-zero/internal/aircraft"
	"go-zero/internal/resources"
	"go-zero/internal/satellite"
	"go-zero/simpleui"
)

type AircraftPanel struct {
	screen                     *MainScreen
	controls                   []simpleui.Element
	mode1090, mode978, start   *simpleui.Button
	mode                       string
	enabled                    bool
	snapshotPath, lastSnapshot string
	viewer                     *exec.Cmd
	viewerDone                 chan struct{}
	feedback                   string
}

func NewAircraftPanel(screen *MainScreen) *AircraftPanel {
	p := &AircraftPanel{screen: screen, mode: aircraft.Mode1090, snapshotPath: filepath.Join(resources.WritablePath("cache"), "aircraft.json")}
	button := func(id, label string, x, w float32, action func()) *simpleui.Button {
		b := simpleui.NewButton(id, x, toolY+28, w, 34, label, uiControlFontSize)
		b.SetColors(colors.panelAlt, colors.border, colors.text)
		b.OnClick(action)
		p.controls = append(p.controls, b)
		return b
	}
	p.mode1090 = button("air1090", i18n.Source("text.edcb1e8e7a35"), 40, 160, func() { p.selectMode(aircraft.Mode1090) })
	p.mode978 = button("air978", i18n.Source("text.5779a6ff204c"), 215, 140, func() { p.selectMode(aircraft.Mode978) })
	p.start = button("airStart", i18n.Source("text.7f23d98fbc9a"), 375, 140, func() { p.enabled = !p.enabled; p.apply() })
	mapButton := button("airMap", i18n.Source("text.19bc47933e09"), 535, 175, p.openMap)
	mapButton.SetColors(colors.blue, colors.border, colors.text)
	clearButton := button("airClear", i18n.Source("text.2aded7edd569"), 730, 130, func() {
		if screen.receiver != nil {
			screen.receiver.ClearAircraft()
		}
		p.feedback = i18n.Source("text.1a7e9d00cb45")
	})
	clearButton.SetColors(actionClearFill, colors.red, colors.text)
	p.SetVisible(false)
	p.style()
	return p
}
func (p *AircraftPanel) SetVisible(v bool) {
	for _, c := range p.controls {
		c.SetVisible(v)
	}
}
func (p *AircraftPanel) frequency() int64 {
	if p.mode == aircraft.Mode978 {
		return aircraft.Frequency978Hz
	}
	return aircraft.Frequency1090Hz
}
func (p *AircraftPanel) selectMode(mode string) {
	if p.mode == mode {
		p.tune()
		return
	}
	was := p.enabled
	p.enabled = false
	p.apply()
	p.mode = mode
	p.tune()
	p.enabled = was
	p.apply()
	p.style()
}
func (p *AircraftPanel) Enter() {
	// The 1090/978 buttons are the explicit tuning controls. Merely opening
	// this tool keeps the current band and frequency untouched.
	demodMode := i18n.Source("text.7866f9f32e66")
	if p.mode == aircraft.Mode978 {
		demodMode = i18n.Source("text.72c048cb5100")
	}
	p.screen.selectMode(demodMode)
	p.setReceiverReference()
}

func (p *AircraftPanel) setReceiverReference() {
	if p.screen.receiver == nil {
		return
	}
	path := resources.WritablePath("config", "satellite-station.json")
	data, err := os.ReadFile(path)
	if err != nil {
		p.screen.receiver.SetAircraftReference(0, 0, false)
		return
	}
	var station satellite.Station
	if json.Unmarshal(data, &station) != nil || station.Latitude < -90 || station.Latitude > 90 || station.Longitude < -180 || station.Longitude > 180 || (station.Latitude == 0 && station.Longitude == 0) {
		p.screen.receiver.SetAircraftReference(0, 0, false)
		return
	}
	p.screen.receiver.SetAircraftReference(station.Latitude, station.Longitude, true)
}
func (p *AircraftPanel) tune() {
	s := p.screen
	hz := p.frequency()
	demodMode := i18n.Source("text.7866f9f32e66")
	if p.mode == aircraft.Mode978 {
		demodMode = i18n.Source("text.72c048cb5100")
	}
	s.selectMode(demodMode)
	s.draggingSpectrum = false
	s.setFrequencyDigitExponent(-1)
	s.frequencyHz = hz
	s.centerFrequencyHz = hz
	s.centerMode = true
	s.spanHz = 2_000_000
	s.updateBandForFrequency(hz)
	if s.vfoModeSwitch != nil {
		s.vfoModeSwitch.SetActive(false)
	}
	if s.receiver != nil {
		s.receiver.SetCenterFrequency(hz)
		s.receiver.SetDemodulator(demodMode, hz, s.demodBandwidthHz)
	}
	s.waterfall.Reset()
}
func (p *AircraftPanel) Leave() {
	p.enabled = false
	p.apply()
}
func (p *AircraftPanel) apply() {
	if p.screen.receiver != nil {
		p.setReceiverReference()
		p.screen.receiver.ConfigureAircraft(p.enabled, p.mode)
		if p.enabled && !p.screen.receiver.AircraftStatus().Running {
			p.enabled = false
		}
	}
	p.style()
}
func (p *AircraftPanel) style() {
	if p.mode == aircraft.Mode1090 {
		p.mode1090.SetColors(colors.blue, colors.border, colors.text)
		p.mode978.SetColors(colors.panelAlt, colors.border, colors.text)
	} else {
		p.mode1090.SetColors(colors.panelAlt, colors.border, colors.text)
		p.mode978.SetColors(colors.blue, colors.border, colors.text)
	}
	if p.enabled {
		p.start.SetLabel(i18n.Source("text.42a572b1399e"))
		p.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.start.SetLabel(i18n.Source("text.7f23d98fbc9a"))
		p.start.SetColors(actionStartFill, colors.green, colors.text)
	}
}
func (p *AircraftPanel) Tick() {
	if p.screen.receiver == nil {
		return
	}
	list := p.screen.receiver.Aircraft()
	p.writeSnapshot(list)
	if p.enabled && !p.screen.receiver.AircraftStatus().Running {
		p.enabled = false
		p.style()
	}
	if p.viewerDone != nil {
		select {
		case <-p.viewerDone:
			p.viewer = nil
			p.viewerDone = nil
		default:
		}
	}
}
func (p *AircraftPanel) writeSnapshot(list []aircraft.Aircraft) {
	data, _ := json.Marshal(list)
	current := string(data)
	if current == p.lastSnapshot {
		return
	}
	p.lastSnapshot = current
	_ = os.MkdirAll(filepath.Dir(p.snapshotPath), 0755)
	if replaceLiveSnapshot(p.snapshotPath, data) != nil {
		p.lastSnapshot = ""
	}
}
func (p *AircraftPanel) openMap() {
	if p.screen.receiver != nil {
		p.writeSnapshot(p.screen.receiver.Aircraft())
	}
	if p.viewer != nil && p.viewer.Process != nil {
		focusRTL433Viewer(p.viewer.Process.Pid)
		p.feedback = i18n.Source("text.d45ae247faf6")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		p.feedback = i18n.Source("text.590655f681fb")
		return
	}
	cmd := exec.Command(exe, "--aircraft-map", p.snapshotPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = cmd.Start(); err != nil {
		p.feedback = i18n.Source("text.590655f681fb")
		return
	}
	p.viewer = cmd
	p.viewerDone = make(chan struct{})
	done := p.viewerDone
	go func() { _ = cmd.Wait(); close(done) }()
	p.feedback = i18n.Source("text.60ac967b38ed")
}
func (p *AircraftPanel) Close() {
	p.enabled = false
	p.apply()
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}
func (p *AircraftPanel) DrawPanel() {
	status := aircraft.Status{State: i18n.Source("text.810e0d52136b")}
	var list []aircraft.Aircraft
	if p.screen.receiver != nil {
		status = p.screen.receiver.AircraftStatus()
		list = p.screen.receiver.Aircraft()
	}
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.b8b3f8c0bf9d"), p.mode, float64(p.frequency())/1e6, status.State, len(list), status.Messages), 40, toolY+7, 12, colors.cyan)
	if status.Error != "" {
		simpleui.DrawText(sondeClip(status.Error, 72), 880, toolY+38, 12, colors.red)
	} else {
		simpleui.DrawText(i18n.Source("text.20ef88737e26"), 880, toolY+38, 12, colors.muted)
	}
	cols := []struct {
		x    float32
		name string
	}{{40, i18n.Source("text.ab556fe26885")}, {260, i18n.Source("text.f0417f5218f5")}, {375, i18n.Source("text.a1f6986b3cd0")}, {480, i18n.Source("text.5743f7dd78c6")}, {590, i18n.Source("text.48b9f0f98bd8")}, {700, i18n.Source("text.6cbb45c7f467")}, {810, i18n.Source("text.a87d437284ec")}, {950, i18n.Source("text.5b99241b86d1")}, {1080, i18n.Source("text.b0ddd4ea3459")}}
	for _, c := range cols {
		simpleui.DrawText(c.name, c.x, toolY+78, 12, colors.muted)
	}
	for i, a := range list {
		if i >= 3 {
			break
		}
		name := a.Callsign
		if name == "" {
			name = a.ICAO
		}
		f := func(v *float64, format string) string {
			if v == nil {
				return "--"
			}
			return fmt.Sprintf(format, *v)
		}
		alt := "--"
		if a.Altitude != nil {
			alt = fmt.Sprintf("%d", *a.Altitude)
		}
		values := []string{name + " / " + a.ICAO, a.Source, a.LastSeen.Local().Format("15:04:05"), alt, f(a.Speed, "%.0f"), f(a.Track, "%.0f°"), f(a.VerticalRate, "%.0f"), f(a.Latitude, "%.5f"), f(a.Longitude, "%.5f")}
		for j, value := range values {
			simpleui.DrawText(sondeClip(value, 20), cols[j].x, toolY+101+float32(i)*23, 12, colors.text)
		}
	}
	if len(list) == 0 {
		simpleui.DrawText(i18n.Source("text.c9ee8539ff8d"), 40, toolY+110, 13, colors.muted)
	}
	simpleui.DrawText(sondeClip(p.feedback+i18n.Source("text.72bc437f7de7"), 150), 40, toolY+174, 12, colors.muted)
}
