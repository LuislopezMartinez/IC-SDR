package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"go-zero/internal/ais"
	"go-zero/internal/resources"
	"go-zero/simpleui"
)

type AISPanel struct {
	screen                     *MainScreen
	controls                   []simpleui.Element
	start                      *simpleui.Button
	enabled                    bool
	snapshotPath, lastSnapshot string
	viewer                     *exec.Cmd
	viewerDone                 chan struct{}
	feedback                   string
}

func NewAISPanel(screen *MainScreen) *AISPanel {
	p := &AISPanel{screen: screen, snapshotPath: filepath.Join(resources.WritablePath("cache"), "ais-vessels.json")}
	button := func(id, label string, x, w float32, action func()) *simpleui.Button {
		b := simpleui.NewButton(id, x, toolY+28, w, 34, label, uiControlFontSize)
		b.SetColors(colors.panelAlt, colors.border, colors.text)
		b.OnClick(action)
		p.controls = append(p.controls, b)
		return b
	}
	p.start = button("aisStart", "START", 40, 145, func() {
		if !p.enabled {
			p.tuneAIS()
		}
		p.enabled = !p.enabled
		p.apply()
	})
	mapButton := button("aisMap", "OPEN MAP", 205, 180, p.openMap)
	mapButton.SetColors(colors.blue, colors.border, colors.text)
	clearButton := button("aisClear", "CLEAR", 405, 135, func() {
		if screen.receiver != nil {
			screen.receiver.ClearAIS()
		}
		p.feedback = "Lista limpiada"
	})
	clearButton.SetColors(actionClearFill, colors.red, colors.text)
	p.apply()
	p.SetVisible(false)
	return p
}

func (p *AISPanel) SetVisible(v bool) {
	for _, c := range p.controls {
		c.SetVisible(v)
	}
}
func (p *AISPanel) Enter() {
	// Opening a tool must never alter the RF tuning. AIS tunes only when the
	// user explicitly presses START.
}
func (p *AISPanel) tuneAIS() {
	s := p.screen
	s.draggingSpectrum = false
	s.setFrequencyDigitExponent(-1)
	s.frequencyHz = ais.CenterFrequencyHz
	s.centerFrequencyHz = ais.CenterFrequencyHz
	s.centerMode = true
	s.spanHz = 250_000
	s.updateBandForFrequency(ais.CenterFrequencyHz)
	if s.vfoModeSwitch != nil {
		s.vfoModeSwitch.SetActive(false)
	}
	if s.receiver != nil {
		s.receiver.SetCenterFrequency(ais.CenterFrequencyHz)
		s.receiver.SetDemodulator("NFM", ais.CenterFrequencyHz, 50_000)
	}
	s.waterfall.Reset()
}
func (p *AISPanel) Leave() {
	p.enabled = false
	p.apply()
}
func (p *AISPanel) apply() {
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureAIS(p.enabled)
		if p.enabled && !p.screen.receiver.AISStatus().Running {
			p.enabled = false
		}
	}
	if p.enabled {
		p.start.SetLabel("STOP")
		p.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.start.SetLabel("START")
		p.start.SetColors(actionStartFill, colors.green, colors.text)
	}
}
func (p *AISPanel) Close() {
	p.enabled = false
	p.apply()
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}
func (p *AISPanel) Tick() {
	if p.screen.receiver == nil {
		return
	}
	vessels := p.screen.receiver.AISVessels()
	p.writeSnapshot(vessels)
	if p.enabled && !p.screen.receiver.AISStatus().Running {
		p.enabled = false
		p.start.SetLabel("START")
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
func (p *AISPanel) writeSnapshot(v []ais.Vessel) {
	sort.Slice(v, func(i, j int) bool { return v[i].LastSeen.After(v[j].LastSeen) })
	data, _ := json.Marshal(v)
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
func (p *AISPanel) openMap() {
	if p.screen.receiver != nil {
		p.writeSnapshot(p.screen.receiver.AISVessels())
	}
	if p.viewer != nil && p.viewer.Process != nil {
		focusRTL433Viewer(p.viewer.Process.Pid)
		p.feedback = "MAP ALREADY OPEN"
		return
	}
	executable, err := os.Executable()
	if err != nil {
		p.feedback = "ERROR OPENING MAP"
		return
	}
	cmd := exec.Command(executable, "--ais-map", p.snapshotPath)
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
func (p *AISPanel) DrawPanel() {
	status := ais.Status{State: "NO RECEIVER"}
	var vessels []ais.Vessel
	if p.screen.receiver != nil {
		status = p.screen.receiver.AISStatus()
		vessels = p.screen.receiver.AISVessels()
	}
	simpleui.DrawText(fmt.Sprintf("MARITIME AIS · 161.975 / 162.025 MHz · %s · %d ships · %d messages", status.State, len(vessels), status.Messages), 40, toolY+7, 12, colors.cyan)
	if status.Error != "" {
		simpleui.DrawText(sondeClip(status.Error, 100), 570, toolY+38, 12, colors.red)
	} else {
		simpleui.DrawText("Simultaneous reception of both AIS channels", 570, toolY+38, 12, colors.muted)
	}
	cols := []struct {
		x    float32
		name string
	}{{40, "SHIP / MMSI"}, {310, "LAST"}, {410, "LATITUDE"}, {535, "LONGITUDE"}, {665, "VEL. kn"}, {770, "TRACK"}, {880, "STATUS"}, {1090, "DESTINATION"}}
	for _, c := range cols {
		simpleui.DrawText(c.name, c.x, toolY+78, 12, colors.muted)
	}
	sort.Slice(vessels, func(i, j int) bool { return vessels[i].LastSeen.After(vessels[j].LastSeen) })
	for i, v := range vessels {
		if i >= 3 {
			break
		}
		name := v.Name
		if name == "" {
			name = fmt.Sprintf("MMSI %09d", v.MMSI)
		}
		lat, lon, speed, course := "--", "--", "--", "--"
		if v.Latitude != nil {
			lat = fmt.Sprintf("%.5f", *v.Latitude)
		}
		if v.Longitude != nil {
			lon = fmt.Sprintf("%.5f", *v.Longitude)
		}
		if v.Speed != nil {
			speed = fmt.Sprintf("%.1f", *v.Speed)
		}
		if v.Course != nil {
			course = fmt.Sprintf("%.0f°", *v.Course)
		}
		values := []string{name, v.LastSeen.Local().Format("15:04:05"), lat, lon, speed, course, v.StatusText, v.Destination}
		for j, value := range values {
			simpleui.DrawText(sondeClip(value, 24), cols[j].x, toolY+101+float32(i)*23, 12, colors.text)
		}
	}
	if len(vessels) == 0 {
		simpleui.DrawText("Press START to decode AIS from the SDR receiver. OPEN MAP shows positions in another window.", 40, toolY+110, 13, colors.muted)
	}
	simpleui.DrawText(sondeClip(p.feedback+"  AIS-catcher · local map with no Internet connection", 150), 40, toolY+174, 12, colors.muted)
}
