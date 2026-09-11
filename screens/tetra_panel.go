package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/resources"
	"go-zero/internal/tetra"
	"go-zero/simpleui"
)

type TETRAPanel struct {
	screen                                                      *MainScreen
	controls                                                    []simpleui.Element
	start                                                       *simpleui.Button
	autoCenterSwitch                                            *simpleui.Switch
	topmostSwitch                                               *simpleui.Switch
	listenSelector                                              *simpleui.Dropdown
	clearOnlySwitch                                             *simpleui.Switch
	enabled                                                     bool
	autoCenter                                                  bool
	nextCenter                                                  time.Time
	nextSnapshot                                                time.Time
	centerError                                                 float64
	centerFiltered                                              float64
	centerStable                                                int
	feedback                                                    string
	snapshotPath, commandPath, viewerSettingsPath, lastSnapshot string
	viewer                                                      *exec.Cmd
	viewerDone                                                  chan struct{}
	listenSlot                                                  int
	clearOnly                                                   bool
	viewerTopmost                                               bool
}

func NewTETRAPanel(screen *MainScreen) *TETRAPanel {
	p := &TETRAPanel{screen: screen, snapshotPath: resources.WritablePath("cache", "tetra-live.json"), commandPath: resources.WritablePath("cache", "tetra-command.json"), viewerSettingsPath: resources.WritablePath("settings", "tetra-viewer.json"), clearOnly: true}
	p.loadViewerSettings()
	button := func(id, label string, x, w float32, action func()) *simpleui.Button {
		b := simpleui.NewButton(id, x, toolY+28, w, 34, label, uiControlFontSize)
		b.SetColors(colors.panelAlt, colors.border, colors.text)
		b.OnClick(action)
		p.controls = append(p.controls, b)
		return b
	}
	p.start = button("tetraStart", "INICIAR", 360, 94, func() { p.enabled = !p.enabled; p.apply() })
	clear := button("tetraClear", "LIMPIAR", 462, 95, func() {
		if screen.receiver != nil {
			screen.receiver.ClearTETRA()
		}
		p.feedback = "CONTADORES REINICIADOS"
	})
	clear.SetColors(actionClearFill, colors.red, colors.text)
	p.listenSelector = simpleui.NewDropdown("tetraListenSlot", 1120, toolY+137, 175, 34, "ESCUCHA", []string{"AUTO", "TS1", "TS2", "TS3", "TS4"}, 13)
	p.listenSelector.SetSelected(0)
	p.listenSelector.OnChange(func(index int, _ string) { p.listenSlot = index; p.applyAudioPolicy() })
	p.controls = append(p.controls, p.listenSelector)
	p.clearOnlySwitch = simpleui.NewSwitch("tetraClearAudioOnly", 990, toolY+28, 175, 34, "SOLO SIN CIFRAR", true, 12)
	p.clearOnlySwitch.OnChange(func(active bool) { p.clearOnly = active; p.applyAudioPolicy() })
	p.clearOnlySwitch.SetTrackColors(colors.red, colors.green)
	p.controls = append(p.controls, p.clearOnlySwitch)
	p.autoCenter = true
	p.autoCenterSwitch = simpleui.NewSwitch("tetraAutoCenter", 1175, toolY+28, 140, 34, "AUTO CENTRO", true, 12)
	p.autoCenterSwitch.OnChange(func(active bool) { p.autoCenter = active })
	p.autoCenterSwitch.SetTrackColors(colors.panelAlt, colors.green)
	p.controls = append(p.controls, p.autoCenterSwitch)
	viewer := button("tetraViewer", "DATA", 1325, 80, p.openViewer)
	viewer.SetColors(colors.blue, colors.border, colors.text)
	p.topmostSwitch = simpleui.NewSwitch("tetraViewerTopmost", 1415, toolY+28, 140, 34, "1er PLANO", p.viewerTopmost, 12)
	p.topmostSwitch.SetTrackColors(colors.panelAlt, colors.green)
	p.topmostSwitch.OnChange(func(active bool) {
		p.viewerTopmost = active
		p.writeViewerSettings()
		if active {
			p.feedback = "VENTANA TETRA · SIEMPRE EN PRIMER PLANO"
		} else {
			p.feedback = "VENTANA TETRA · PRIMER PLANO DESACTIVADO"
		}
	})
	p.controls = append(p.controls, p.topmostSwitch)
	for i, b := range []struct {
		name string
		hz   int64
	}{{"380–400", tetra.DefaultFrequencyHz}, {"410–430", 420_000_000}, {"450–470", 460_000_000}} {
		bb := b
		btn := button(fmt.Sprintf("tetraBand%d", i), bb.name, 565+float32(i)*142, 130, func() { p.tune(bb.hz); p.feedback = "BANDA " + bb.name + " MHz" })
		btn.SetColors(colors.blue, colors.border, colors.text)
	}
	p.SetVisible(false)
	return p
}

func (p *TETRAPanel) tune(hz int64) {
	s := p.screen
	p.centerError, p.centerFiltered, p.centerStable = 0, 0, 0
	s.frequencyHz = hz
	s.centerFrequencyHz = hz
	s.spanHz = 250_000
	s.tuningStepHz = 12_500
	s.centerMode = true
	s.updateBandForFrequency(hz)
	if s.stepSelector != nil {
		s.stepSelector.SetSelected(s.tuningStepHz)
	}
	if s.vfoModeSwitch != nil {
		s.vfoModeSwitch.SetActive(false)
	}
	if s.receiver != nil {
		s.receiver.SetCenterFrequency(hz)
		s.receiver.SetDemodulator("TETRA", hz, 25_000)
	}
	s.waterfall.Reset()
	s.markSettingsDirty()
}
func (p *TETRAPanel) Enter() {
	s := p.screen
	for i, item := range s.mode.Items() {
		if item == "TETRA" {
			s.mode.SetSelected(i)
			break
		}
	}
	s.savedMode = "TETRA"
	if s.filterSelector != nil {
		s.selectFilter(s.filterSelector.SelectPreset("TETRA", 0))
	} else {
		s.demodBandwidthHz = 25_000
	}
	s.tuningStepHz = 12_500
	if s.stepSelector != nil {
		s.stepSelector.SetSelected(s.tuningStepHz)
	}
}
func (p *TETRAPanel) Leave() {
	p.enabled = false
	p.apply()
}
func (p *TETRAPanel) apply() {
	if p.enabled {
		// Keep the RF capture fixed while the narrow TETRA VFO and AFC make
		// sub-bin corrections; repeated hardware retunes would break timing.
		p.screen.centerMode = false
		if p.screen.vfoModeSwitch != nil {
			p.screen.vfoModeSwitch.SetActive(true)
		}
	}
	if p.screen.receiver != nil {
		p.screen.receiver.ConfigureTETRA(p.enabled)
		p.screen.receiver.SetTETRAAudioPolicy(p.listenSlot, p.clearOnly)
	}
	if p.enabled {
		p.start.SetLabel("DETENER")
		p.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.start.SetLabel("INICIAR")
		p.start.SetColors(actionStartFill, colors.green, colors.text)
	}
}
func (p *TETRAPanel) applyAudioPolicy() {
	if p.screen.receiver != nil {
		p.screen.receiver.SetTETRAAudioPolicy(p.listenSlot, p.clearOnly)
	}
	mode := "AUTO"
	if p.listenSlot > 0 {
		mode = fmt.Sprintf("TS%d", p.listenSlot)
	}
	p.feedback = "ESCUCHA " + mode
	if p.clearOnly {
		p.feedback += " · SOLO ABIERTA"
	} else {
		p.feedback += " · INCLUYE CIFRADA"
	}
}
func (p *TETRAPanel) Close() {
	p.enabled = false
	p.apply()
	if p.viewer != nil && p.viewer.Process != nil {
		_ = p.viewer.Process.Kill()
	}
}
func (p *TETRAPanel) Tick() {
	p.readViewerCommand()
	p.writeSnapshot()
	if p.viewerDone != nil {
		select {
		case <-p.viewerDone:
			p.viewer = nil
			p.viewerDone = nil
		default:
		}
	}
	if !p.enabled || !p.autoCenter || p.screen.activeTool != "TETRA" || time.Now().Before(p.nextCenter) {
		return
	}
	p.nextCenter = time.Now().Add(time.Second)
	s := p.screen
	if s.receiver == nil {
		return
	}
	status := s.receiver.TETRAStatus()
	// Like the SDR# TETRA plugin, use phase error from the demodulator instead
	// of the spectral centroid. Adjacent carriers cannot pull this estimate.
	if status.LastSync.IsZero() || time.Since(status.LastSync) > 1500*time.Millisecond || status.Quality < 35 {
		p.centerStable = 0
		p.centerFiltered = 0
		return
	}
	err := float64(status.FrequencyErrorHz)
	p.centerError = err
	if p.centerStable == 0 {
		p.centerFiltered = err
	} else {
		p.centerFiltered = p.centerFiltered*.90 + err*.10
	}
	p.centerStable++
	if math.Abs(p.centerFiltered) <= 200 {
		return
	}
	// A valid TETRA channel should already be inside the 25 kHz passband.
	// Reject implausible estimates and cap one retune to half a channel step.
	if math.Abs(p.centerFiltered) > 6000 {
		p.centerStable = 0
		p.centerFiltered = 0
		return
	}
	p.centerStable = 0
	correction := math.Max(-2500, math.Min(2500, p.centerFiltered))
	next := int64(math.Round(float64(s.frequencyHz) + correction))
	next = (next / 10) * 10
	if next == s.frequencyHz {
		return
	}
	s.frequencyHz = next
	if s.receiver != nil {
		s.receiver.SetDemodulator("TETRA", next, s.demodBandwidthHz)
	}
	p.centerFiltered = 0
	s.markSettingsDirty()
}

type tetraViewerCommand struct {
	TuneHz int64 `json:"tuneHz"`
}

type tetraViewerSettings struct {
	Topmost bool `json:"topmost"`
}

func (p *TETRAPanel) loadViewerSettings() {
	data, err := os.ReadFile(p.viewerSettingsPath)
	if err != nil {
		return
	}
	var settings tetraViewerSettings
	if json.Unmarshal(data, &settings) == nil {
		p.viewerTopmost = settings.Topmost
	}
}

func (p *TETRAPanel) writeViewerSettings() {
	data, err := json.Marshal(tetraViewerSettings{Topmost: p.viewerTopmost})
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p.viewerSettingsPath), 0755)
	_ = os.WriteFile(p.viewerSettingsPath, data, 0644)
}

func (p *TETRAPanel) readViewerCommand() {
	data, err := os.ReadFile(p.commandPath)
	if err != nil {
		return
	}
	_ = os.Remove(p.commandPath)
	var command tetraViewerCommand
	if json.Unmarshal(data, &command) != nil || command.TuneHz < 100_000_000 || command.TuneHz > 1_500_000_000 {
		return
	}
	p.tune(command.TuneHz)
	if p.screen.receiver != nil {
		p.screen.receiver.ClearTETRA()
	}
	p.feedback = fmt.Sprintf("CELDA VECINA · %.6f MHz · DATOS REINICIADOS", float64(command.TuneHz)/1e6)
	p.nextSnapshot = time.Time{}
}

func (p *TETRAPanel) resetAfterManualTune() {
	p.centerError, p.centerFiltered, p.centerStable = 0, 0, 0
	p.nextCenter = time.Now().Add(time.Second)
	if p.screen.receiver != nil {
		p.screen.receiver.ClearTETRA()
	}
	p.feedback = "NUEVA SINTONÍA · DATOS TETRA REINICIADOS"
	p.nextSnapshot = time.Time{}
	p.writeSnapshot()
}
func (p *TETRAPanel) writeSnapshot() {
	if p.screen.receiver == nil {
		return
	}
	if time.Now().Before(p.nextSnapshot) {
		return
	}
	p.nextSnapshot = time.Now().Add(200 * time.Millisecond)
	snap := p.screen.receiver.TETRALiveSnapshot(p.screen.frequencyHz)
	data, _ := json.Marshal(snap)
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
func (p *TETRAPanel) openViewer() {
	p.writeSnapshot()
	if p.viewer != nil && p.viewer.Process != nil {
		focusRTL433Viewer(p.viewer.Process.Pid)
		p.feedback = "VENTANA TETRA YA ABIERTA"
		return
	}
	exe, err := os.Executable()
	if err != nil {
		p.feedback = "ERROR AL ABRIR VENTANA"
		return
	}
	p.writeViewerSettings()
	cmd := exec.Command(exe, "--tetra-viewer", p.snapshotPath, p.viewerSettingsPath)
	cmd.SysProcAttr = rtl433ViewerProcessAttributes()
	if err = cmd.Start(); err != nil {
		p.feedback = "ERROR AL ABRIR VENTANA"
		return
	}
	p.viewer = cmd
	p.viewerDone = make(chan struct{})
	done := p.viewerDone
	go func() { _ = cmd.Wait(); close(done) }()
	p.feedback = "VENTANA TETRA ABIERTA"
}
func (p *TETRAPanel) SetVisible(v bool) {
	for _, c := range p.controls {
		c.SetVisible(v)
	}
}

func (p *TETRAPanel) DrawPanel() {
	status := tetra.Status{State: "SIN RECEPTOR"}
	if p.screen.receiver != nil {
		status = p.screen.receiver.TETRAStatus()
	}
	simpleui.DrawTextStyled("TETRA · π/4-DQPSK · 25 kHz", toolContentX, toolY+7, 14, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled("ESTADO AUDIO", 1120, toolY+76, 12, simpleui.FontSemiBold, colors.muted)
	for i, encrypted := range status.SlotEncrypted {
		x := float32(1139 + i*39)
		indicator := colors.muted
		if status.SlotTraffic[i] == 1 {
			indicator = colors.orange
			if encrypted == 0 {
				indicator = colors.green
			} else if encrypted == 1 {
				indicator = colors.red
			}
		}
		rl.DrawCircle(int32(x), int32(toolY+105), 10, indicator)
		rl.DrawCircleLines(int32(x), int32(toolY+105), 11, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d", i+1), x-4, toolY+98, 12, colors.background)
		simpleui.DrawText(fmt.Sprintf("TS%d", i+1), x-12, toolY+119, 12, colors.text)
	}
	if status.ActiveAudioSlot > 0 {
		label, color := fmt.Sprintf("TS OBJETIVO TS%d · ESPERANDO TCH", status.ActiveAudioSlot), colors.orange
		if status.AudioFrames > 0 && time.Since(status.LastAudio) < time.Second {
			label, color = fmt.Sprintf("REPRODUCIENDO TS%d", status.ActiveAudioSlot), colors.green
		}
		simpleui.DrawText(label, 1120, toolY+174, 12, color)
	}
	if !status.VoiceCodecReady {
		simpleui.DrawText("CÓDEC: "+status.VoiceCodecError, 1310, toolY+174, 12, colors.red)
	}
	// Draw each live value in its own fixed column. A single formatted string
	// shifts every field whenever a signed value gains or loses a digit.
	drawTETRAStatusField("ESTADO", status.State, toolContentX, toolY+72, 185)
	drawTETRAStatusField("NIVEL", fmt.Sprintf("%6.1f dBFS", status.LevelDBFS), 555, toolY+72, 125)
	drawTETRAStatusField("CALIDAD", fmt.Sprintf("%3.0f %%", status.Quality), 690, toolY+72, 115)
	drawTETRAStatusField("AFC", fmt.Sprintf("%+6.0f Hz", status.FrequencyErrorHz), 815, toolY+72, 120)
	drawTETRAStatusField("CENTRO", fmt.Sprintf("%+6.0f Hz", p.centerError), 945, toolY+72, 145)
	simpleui.DrawText(fmt.Sprintf("18 ksym/s · BER %5.2f%% · FER %5.1f%% · SYNC %d · NTS %d · AACH %d/%d · SCH %d/%d · MAC %d · CMCE %d", status.BER, status.FER, status.SyncHits, status.NormalBursts, status.AACHValid, status.AACHRejected, status.SCHValid, status.SCHCRCFailures, status.MACResources, status.CMCEEvents), toolContentX, toolY+99, 13, colors.muted)
	if !status.LastSync.IsZero() {
		simpleui.DrawText("ÚLTIMA SINCRONÍA  "+status.LastSync.Format("15:04:05"), toolContentX, toolY+124, 13, colors.green)
	}
	if status.System.Valid {
		simpleui.DrawText(fmt.Sprintf("RED  MCC %d · MNC %d · COLOR %d · TS %d · FN %d · MF %d", status.System.MCC, status.System.MNC, status.System.ColourCode, status.System.Timeslot, status.System.Frame, status.System.Multiframe), toolContentX, toolY+148, 13, colors.green)
	}
	constellation := rl.Rectangle{X: 1310, Y: toolY + 72, Width: 150, Height: 108}
	rl.DrawRectangleRec(constellation, colors.background)
	rl.DrawRectangleLinesEx(constellation, 1, colors.border)
	cx, cy := constellation.X+constellation.Width/2, constellation.Y+constellation.Height/2
	rl.DrawLine(int32(constellation.X), int32(cy), int32(constellation.X+constellation.Width), int32(cy), colors.grid)
	rl.DrawLine(int32(cx), int32(constellation.Y), int32(cx), int32(constellation.Y+constellation.Height), colors.grid)
	for _, pt := range status.Constellation {
		rl.DrawCircle(int32(cx+pt.I*42), int32(cy-pt.Q*42), 2, rl.Color{R: 45, G: 195, B: 225, A: 170})
	}
	simpleui.DrawText("CONSTELACIÓN", 1468, toolY+82, 12, colors.muted)
	msg := p.feedback
	if msg == "" {
		msg = "Selecciona la subbanda y ajusta el canal en pasos de 25 kHz"
	}
	simpleui.DrawText(msg, toolContentX, toolY+174, 12, colors.muted)
}

func drawTETRAStatusField(label, value string, x, y, width float32) {
	simpleui.DrawTextStyled(label, x, y, 11, simpleui.FontSemiBold, colors.muted)
	valueX := x + 58
	// Clip unexpectedly long state text to its reserved column so it can never
	// invade the next live field.
	maxChars := int((width - 58) / 7)
	if maxChars > 1 && len([]rune(value)) > maxChars {
		runes := []rune(value)
		value = string(runes[:maxChars-1]) + "…"
	}
	simpleui.DrawText(value, valueX, y-1, 13, colors.text)
}
