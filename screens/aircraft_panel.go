package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go-zero/internal/aircraft"
	"go-zero/internal/resources"
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
	p.mode1090 = button("air1090", "1090 ADS-B", 40, 160, func() { p.selectMode(aircraft.Mode1090) })
	p.mode978 = button("air978", "978 UAT", 215, 140, func() { p.selectMode(aircraft.Mode978) })
	p.start = button("airStart", "START", 375, 140, func() { p.enabled = !p.enabled; p.apply() })
	mapButton := button("airMap", T("OPEN MAP"), 535, 175, p.openMap)
	mapButton.SetColors(colors.blue, colors.border, colors.text)
	clearButton := button("airClear", T("CLEAR"), 730, 130, func() {
		if screen.receiver != nil {
			screen.receiver.ClearAircraft()
		}
		p.feedback = T("List cleared")
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
	demodMode := "ADS-B"
	if p.mode == aircraft.Mode978 {
		demodMode = "UAT"
	}
	p.screen.selectMode(demodMode)
}
func (p *AircraftPanel) tune() {
	s := p.screen
	hz := p.frequency()
	demodMode := "ADS-B"
	if p.mode == aircraft.Mode978 {
		demodMode = "UAT"
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
		lat, lon, ok := loadAircraftReceiverRef()
		p.screen.receiver.SetAircraftReference(lat, lon, ok)
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
		p.start.SetLabel(T("STOP"))
		p.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.start.SetLabel(T("START"))
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
		p.feedback = "MAP ALREADY OPEN"
		return
	}
	exe, err := os.Executable()
	if err != nil {
		p.feedback = "ERROR OPENING MAP"
		return
	}
	cmd := exec.Command(exe, "--aircraft-map", p.snapshotPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = cmd.Start(); err != nil {
		p.feedback = "ERROR OPENING MAP"
		return
	}
	p.viewer = cmd
	p.viewerDone = make(chan struct{})
	done := p.viewerDone
	go func() { _ = cmd.Wait(); close(done) }()
	p.feedback = "MAP OPENED"
}
func (p *AircraftPanel) Close() {
	p.enabled = false
	p.apply()
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}
func (p *AircraftPanel) DrawPanel() {
	status := aircraft.Status{State: "NO RECEIVER"}
	var list []aircraft.Aircraft
	if p.screen.receiver != nil {
		status = p.screen.receiver.AircraftStatus()
		list = p.screen.receiver.Aircraft()
	}
	simpleui.DrawText(fmt.Sprintf("AIR SURVEILLANCE · %s · %.3f MHz · %s · %d aircraft · %d messages", p.mode, float64(p.frequency())/1e6, status.State, len(list), status.Messages), 40, toolY+7, 12, colors.cyan)
	if status.Error != "" {
		simpleui.DrawText(sondeClip(status.Error, 72), 880, toolY+38, 12, colors.red)
	} else if status.Detail != "" {
		simpleui.DrawText(sondeClip(status.Detail, 72), 880, toolY+38, 12, colors.muted)
	} else {
		simpleui.DrawText(T("1090: ADS-B/Mode S worldwide · 978 UAT: mainly USA"), 880, toolY+38, 12, colors.muted)
	}
	cols := []struct {
		x    float32
		name string
	}{{40, "FLIGHT / ICAO"}, {260, "SOURCE"}, {375, "LAST"}, {480, "ALT ft"}, {590, "SPD kt"}, {700, "TRACK"}, {810, "V/S fpm"}, {950, "LATITUDE"}, {1080, "LONGITUDE"}}
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
		simpleui.DrawText(T("Select a band and press START. OPEN MAP shows positions, altitude and trails in another window."), 40, toolY+110, 13, colors.muted)
	}
	simpleui.DrawText(sondeClip(p.feedback+"  "+T("Local reception from the SDR · no external tracking services"), 150), 40, toolY+174, 12, colors.muted)
}

func loadAircraftReceiverRef() (float64, float64, bool) {
	data, err := os.ReadFile(resources.WritablePath("config", "satellite-station.json"))
	if err != nil {
		return 0, 0, false
	}
	var station struct {
		Latitude, Longitude float64
	}
	if json.Unmarshal(data, &station) != nil {
		return 0, 0, false
	}
	if station.Latitude == 0 && station.Longitude == 0 {
		return 0, 0, false
	}
	if station.Latitude < -90 || station.Latitude > 90 || station.Longitude < -180 || station.Longitude > 180 {
		return 0, 0, false
	}
	return station.Latitude, station.Longitude, true
}
