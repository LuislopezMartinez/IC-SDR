package screens

import (
	"fmt"
	"path/filepath"
	"time"

	"go-zero/internal/iqcapture"
	"go-zero/internal/resources"
	"go-zero/internal/tetrapol"
	"go-zero/internal/tetrapolruntime"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const tetrapolToolID = "TETRAPOL"

type tetrapolPreset struct {
	label string
	hz    int64
}

var tetrapolPresets = map[string][]tetrapolPreset{
	"VHF": {{"68–88 MHz", 78_000_000}, {"150–174 MHz", 162_000_000}},
	"UHF": {{"380–400 MHz", 390_000_000}, {"410–430 MHz", 420_000_000}, {"450–470 MHz", 460_000_000}},
}

type TETRAPOLPanel struct {
	screen                          *MainScreen
	controls                        []simpleui.Element
	start, events, capture          *simpleui.Button
	presets                         [3]*simpleui.Button
	channel, direction, band        *simpleui.Dropdown
	audio, follow, autoCenter       *simpleui.Switch
	enabled, visible, audioEnabled  bool
	followEnabled, following        bool
	controlFrequencyHz              int64
	lastChannelEvent                uint64
	pendingManualTune               bool
	restartAfter                    time.Time
	showEvents                      bool
	channelType, bandType, linkType string
	feedback                        string
	iqWriter                        *iqcapture.Writer
}

func NewTETRAPOLPanel(screen *MainScreen) *TETRAPOLPanel {
	band := screen.tetrapolBand
	if band != "VHF" {
		band = "UHF"
	}
	direction := screen.tetrapolDirection
	if direction != "UP" {
		direction = "DOWN"
	}
	panel := &TETRAPOLPanel{screen: screen, channelType: "CCH", bandType: band, linkType: direction, audioEnabled: true, followEnabled: true}
	panel.start = simpleui.NewButton("tetrapolStart", 374, toolY+45, 132, 38, "INICIAR", 14)
	panel.start.OnClick(func() { panel.enabled = !panel.enabled; panel.apply() })
	panel.controls = append(panel.controls, panel.start)
	panel.follow = simpleui.NewSwitch("tetrapolFollow", 518, toolY+45, 145, 38, "SEGUIR TCH", true, 11)
	panel.follow.OnChange(func(enabled bool) { panel.followEnabled = enabled })
	panel.controls = append(panel.controls, panel.follow)

	for index := range panel.presets {
		i := index
		button := simpleui.NewButton(fmt.Sprintf("tetrapolBand%d", index), 704+float32(index)*124, toolY+50, 116, 34, "", 11)
		button.OnClick(func() { panel.activatePreset(i) })
		panel.presets[index] = button
		panel.controls = append(panel.controls, button)
	}

	panel.channel = simpleui.NewDropdown("tetrapolChannel", 374, toolY+105, 132, 34, "CANAL", []string{"CCH", "TCH"}, 12)
	panel.channel.SetSelected(0)
	panel.channel.OnChange(func(_ int, value string) {
		panel.channelType = value
		panel.following = false
		if value == "CCH" {
			panel.controlFrequencyHz = panel.screen.frequencyHz
		}
		panel.restartIfRunning()
	})
	panel.direction = simpleui.NewDropdown("tetrapolDirection", 518, toolY+105, 145, 34, "DIRECCIÓN", []string{"DOWNLINK", "UPLINK"}, 11)
	if direction == "UP" {
		panel.direction.SetSelected(1)
	} else {
		panel.direction.SetSelected(0)
	}
	panel.direction.OnChange(func(index int, _ string) {
		panel.linkType = []string{"DOWN", "UP"}[index]
		panel.screen.tetrapolDirection = panel.linkType
		panel.screen.markSettingsDirty()
		panel.restartIfRunning()
	})
	panel.band = simpleui.NewDropdown("tetrapolProfile", 374, toolY+157, 132, 34, "PERFIL", []string{"UHF", "VHF"}, 12)
	if band == "VHF" {
		panel.band.SetSelected(1)
	} else {
		panel.band.SetSelected(0)
	}
	panel.band.OnChange(func(_ int, value string) { panel.setBand(value) })
	panel.audio = simpleui.NewSwitch("tetrapolAudio", 518, toolY+157, 145, 34, "AUDIO", true, 11)
	panel.audio.OnChange(func(enabled bool) {
		panel.audioEnabled = enabled
		if panel.screen.receiver != nil {
			panel.screen.receiver.SetTETRAPOLAudioEnabled(enabled)
		}
	})
	// FrequencyDeviationHz measures the average GMSK modulation deviation, not
	// a carrier error. Using it as AFC made the VFO drift continuously. Keep
	// the control visible but unavailable until the decoder exposes a genuine
	// carrier-frequency estimate.
	panel.autoCenter = simpleui.NewSwitch("tetrapolAFC", 704, toolY+157, 121, 30, "AFC", false, 11)
	panel.autoCenter.SetDisabledText("NO DISP.")
	panel.autoCenter.SetEnabled(false)
	panel.events = simpleui.NewButton("tetrapolEvents", 966, toolY+157, 105, 30, "EVENTOS", 11)
	panel.events.OnClick(func() {
		panel.showEvents = !panel.showEvents
		if panel.showEvents {
			panel.events.SetLabel("ESTADO")
		} else {
			panel.events.SetLabel("EVENTOS")
		}
	})
	panel.capture = simpleui.NewButton("tetrapolCapture", 835, toolY+157, 121, 30, "CAPTURAR IQ", 10)
	panel.capture.SetColors(colors.blue, colors.border, colors.text)
	panel.capture.OnClick(panel.toggleIQCapture)
	panel.controls = append(panel.controls, panel.channel, panel.direction, panel.band, panel.audio, panel.autoCenter, panel.capture, panel.events)
	panel.apply()
	panel.SetVisible(false)
	return panel
}

func (panel *TETRAPOLPanel) setBand(band string) {
	if band != "VHF" {
		band = "UHF"
	}
	panel.bandType = band
	panel.screen.tetrapolBand = band
	panel.refreshPresetControls()
	panel.screen.markSettingsDirty()
	panel.restartIfRunning()
}

func (panel *TETRAPOLPanel) refreshPresetControls() {
	presets := tetrapolPresets[panel.bandType]
	width := float32(116)
	if len(presets) == 2 {
		width = 178
	}
	for index, button := range panel.presets {
		shown := panel.visible && index < len(presets)
		button.SetVisible(shown)
		if index < len(presets) {
			button.SetLabel(presets[index].label)
			button.SetBounds(rl.Rectangle{X: 704 + float32(index)*(width+8), Y: toolY + 50, Width: width, Height: 34})
		}
	}
}

func (panel *TETRAPOLPanel) activatePreset(index int) {
	presets := tetrapolPresets[panel.bandType]
	if index < 0 || index >= len(presets) {
		return
	}
	preset := presets[index]
	panel.tune(preset.hz)
	panel.feedback = "Acceso " + preset.label
}

func (panel *TETRAPOLPanel) Enter() {
	panel.screen.setWaterfallControlsVisible(false)
	if panel.screen.mode != nil {
		for index, item := range panel.screen.mode.Items() {
			if item == tetrapolToolID {
				panel.screen.mode.SetSelected(index)
				break
			}
		}
	}
	panel.screen.savedMode = tetrapolToolID
	panel.screen.demodBandwidthHz = 12_500
	if panel.screen.receiver != nil {
		panel.screen.receiver.SetDemodulator(tetrapolToolID, panel.screen.frequencyHz, 12_500)
	}
	panel.screen.tuningStepHz = 12_500
	if panel.screen.stepSelector != nil {
		panel.screen.stepSelector.SetSelected(panel.screen.tuningStepHz)
	}
}

func (panel *TETRAPOLPanel) tune(hz int64) {
	panel.screen.frequencyHz, panel.screen.centerFrequencyHz = hz, hz
	panel.screen.spanHz, panel.screen.tuningStepHz = 250_000, 12_500
	panel.screen.updateBandForFrequency(hz)
	if panel.screen.stepSelector != nil {
		panel.screen.stepSelector.SetSelected(panel.screen.tuningStepHz)
	}
	if panel.screen.receiver != nil {
		panel.screen.receiver.SetCenterFrequency(hz)
		panel.screen.receiver.SetDemodulator(tetrapolToolID, hz, 12_500)
	}
	panel.screen.markSettingsDirty()
}

func (panel *TETRAPOLPanel) restartIfRunning() {
	if panel.enabled {
		panel.restart()
	}
}

func (panel *TETRAPOLPanel) restart() {
	if panel.screen.receiver == nil {
		return
	}
	_ = panel.screen.receiver.StopTETRAPOLRuntime()
	panel.lastChannelEvent = 0
	if err := panel.screen.receiver.StartTETRAPOLRuntimeConfigured(panel.channelType, panel.bandType, panel.linkType); err != nil {
		panel.enabled = false
		panel.feedback = err.Error()
	}
}

// resetAfterManualTune stops decoding immediately, then starts a fresh
// receive pipeline once the user has stopped moving the wheel. That prevents
// old-frame telemetry and channelizer history from being carried into the new
// channel while avoiding one process restart for every wheel tick.
func (panel *TETRAPOLPanel) resetAfterManualTune() {
	if !panel.enabled || panel.screen.receiver == nil {
		return
	}
	if !panel.pendingManualTune {
		_ = panel.screen.receiver.StopTETRAPOLRuntime()
		panel.feedback = "Reajustando TETRAPOL…"
	}
	panel.pendingManualTune = true
	panel.restartAfter = time.Now().Add(350 * time.Millisecond)
}

func (panel *TETRAPOLPanel) completeManualTuneReset(now time.Time) {
	if !panel.pendingManualTune || now.Before(panel.restartAfter) {
		return
	}
	panel.pendingManualTune = false
	panel.restart()
	if panel.enabled {
		panel.feedback = "Buscando sincronismo en la nueva frecuencia…"
	}
}

func (panel *TETRAPOLPanel) Tick() {
	if panel.screen.activeTool == tetrapolToolID {
		panel.completeManualTuneReset(time.Now())
	}
}

func (panel *TETRAPOLPanel) handleChannelFollow(telemetry tetrapolruntime.Telemetry) {
	if !panel.enabled || !panel.followEnabled || telemetry.ChannelEvent == 0 || telemetry.ChannelEvent == panel.lastChannelEvent {
		return
	}
	panel.lastChannelEvent = telemetry.ChannelEvent
	if telemetry.ChannelAction == "follow" && panel.channelType == "CCH" {
		controlHz := panel.screen.frequencyHz
		targetHz, err := tetrapolChannelFrequency(panel.bandType, controlHz, telemetry.ChannelID)
		if err != nil {
			panel.feedback = err.Error()
			return
		}
		panel.controlFrequencyHz = controlHz
		panel.following = true
		panel.channelType = "TCH"
		panel.channel.SetSelected(1)
		panel.tune(targetHz)
		panel.feedback = fmt.Sprintf("Siguiendo TCH %d en %.4f MHz", telemetry.ChannelID, float64(targetHz)/1e6)
		panel.restart()
	} else if telemetry.ChannelAction == "return" && panel.following && panel.controlFrequencyHz > 0 {
		controlHz := panel.controlFrequencyHz
		panel.following = false
		panel.channelType = "CCH"
		panel.channel.SetSelected(0)
		panel.tune(controlHz)
		panel.feedback = "Retorno automático al CCH"
		panel.restart()
	}
}

func (panel *TETRAPOLPanel) Leave() { panel.stopIQCapture(); panel.enabled = false; panel.apply() }
func (panel *TETRAPOLPanel) Close() { panel.Leave() }

func (panel *TETRAPOLPanel) toggleIQCapture() {
	if panel.iqWriter != nil {
		panel.stopIQCapture()
		panel.feedback = "Captura IQ TETRAPOL guardada."
		return
	}
	if panel.screen.receiver == nil {
		panel.feedback = "SDR no disponible para captura IQ."
		return
	}
	directory := resources.WritablePath("captures", "tetrapol")
	name := fmt.Sprintf("tetrapol_%s_%s_%s_%dHz_iq.wav", time.Now().Format("20060102-150405"), panel.bandType, panel.channelType, panel.screen.frequencyHz)
	writer, err := iqcapture.New(filepath.Join(directory, name), panel.screen.centerFrequencyHz, panel.screen.frequencyHz, panel.screen.receiver.SampleRate())
	if err != nil {
		panel.feedback = err.Error()
		return
	}
	panel.iqWriter = writer
	panel.screen.receiver.SetIQSink(writer.Write)
	panel.capture.SetLabel("DETENER IQ")
	panel.feedback = "Capturando IQ TETRAPOL a 128 kS/s."
}

func (panel *TETRAPOLPanel) stopIQCapture() {
	if panel.iqWriter == nil {
		return
	}
	if panel.screen.receiver != nil {
		panel.screen.receiver.SetIQSink(nil)
	}
	_ = panel.iqWriter.Close()
	panel.iqWriter = nil
	panel.capture.SetLabel("CAPTURAR IQ")
}

func (panel *TETRAPOLPanel) apply() {
	if panel.screen.receiver != nil {
		if panel.enabled {
			panel.screen.receiver.SetTETRAPOLAudioEnabled(panel.audioEnabled)
			if err := panel.screen.receiver.StartTETRAPOLRuntimeConfigured(panel.channelType, panel.bandType, panel.linkType); err != nil {
				panel.enabled = false
				panel.feedback = err.Error()
			}
		} else if err := panel.screen.receiver.StopTETRAPOLRuntime(); err != nil {
			panel.feedback = err.Error()
		}
	}
	if panel.enabled {
		panel.start.SetLabel("DETENER")
		panel.start.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		panel.start.SetLabel("INICIAR")
		panel.start.SetColors(actionStartFill, colors.green, colors.text)
	}
}

func (panel *TETRAPOLPanel) SetVisible(visible bool) {
	panel.visible = visible
	for _, control := range panel.controls {
		control.SetVisible(visible)
	}
	panel.refreshPresetControls()
}

func drawTETRAPOLGroup(bounds rl.Rectangle, title string) {
	fill := colors.panelAlt
	fill.A = 150
	rl.DrawRectangleRounded(bounds, .04, 6, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .04, 6, 1, colors.border)
	simpleui.DrawTextStyled(title, bounds.X+12, bounds.Y+8, 12, simpleui.FontSemiBold, colors.cyan)
}

func (panel *TETRAPOLPanel) DrawPanel() {
	if panel.screen.activeTool != tetrapolToolID {
		return
	}
	status := tetrapol.Status{}
	telemetry := tetrapolruntime.Telemetry{}
	runtimeState := "DETENIDO"
	if panel.screen.receiver != nil {
		status = panel.screen.receiver.TETRAPOLStatus()
		runtime := panel.screen.receiver.TETRAPOLRuntimeStatus()
		telemetry = runtime.Telemetry
		if runtime.Running {
			runtimeState = "ACTIVO"
		}
		if runtime.Error != "" {
			panel.feedback = runtime.Error
		}
	}
	panel.handleChannelFollow(telemetry)

	drawTETRAPOLGroup(rl.Rectangle{X: 360, Y: toolY + 5, Width: 315, Height: 202}, "CONTROL TETRAPOL")
	drawTETRAPOLGroup(rl.Rectangle{X: 686, Y: toolY + 5, Width: 400, Height: 202}, "SINTONÍA")
	drawTETRAPOLGroup(rl.Rectangle{X: 1097, Y: toolY + 5, Width: 495, Height: 202}, "ESTADO Y DECODIFICACIÓN")

	simpleui.DrawText("CANAL", 374, toolY+91, 10, colors.muted)
	simpleui.DrawText("DIRECCIÓN", 518, toolY+91, 10, colors.muted)
	simpleui.DrawText("PERFIL DE RED", 374, toolY+143, 10, colors.muted)
	simpleui.DrawText("RP-CELP", 518, toolY+143, 10, colors.muted)
	simpleui.DrawText("ACCESOS DE BANDA", 704, toolY+34, 10, colors.muted)
	simpleui.DrawText("PASO", 704, toolY+104, 10, colors.muted)
	simpleui.DrawTextStyled("12,5 kHz", 704, toolY+120, 15, simpleui.FontMono, colors.text)
	simpleui.DrawText("CANAL", 835, toolY+104, 10, colors.muted)
	simpleui.DrawTextStyled("12,5 kHz", 835, toolY+120, 15, simpleui.FontMono, colors.text)
	simpleui.DrawText("MODULACIÓN", 966, toolY+104, 10, colors.muted)
	simpleui.DrawTextStyled("GMSK 8 kBd", 966, toolY+120, 15, simpleui.FontMono, colors.text)
	simpleui.DrawText("AJUSTE AUTOMÁTICO", 704, toolY+143, 10, colors.muted)

	syncText, syncColor := "SYNC BUSCANDO", colors.orange
	if telemetry.Synchronized {
		syncText, syncColor = "SYNC BLOQUEADO", colors.green
	}
	frame := "—"
	if telemetry.FrameKnown {
		frame = fmt.Sprintf("%d", telemetry.FrameNumber)
	}
	voice := "INACTIVA"
	if telemetry.VoiceActive {
		voice = "ACTIVA"
	}
	if panel.showEvents {
		panel.drawEvents(telemetry)
	} else {
		syncBounds := rl.Rectangle{X: 1110, Y: toolY + 35, Width: 469, Height: 35}
		rl.DrawRectangleRounded(syncBounds, .25, 8, syncColor)
		drawCentered(syncText, syncBounds, 13, simpleui.EnsureTextContrast(colors.text, syncColor))
		items := []string{"TRAMA  " + frame, "TIPO  " + valueOrDash(telemetry.FrameType), "CANAL  " + valueOrDash(telemetry.LogicalChannel), "VOZ  " + voice}
		for index, value := range items {
			bounds := rl.Rectangle{X: 1110 + float32(index)*117, Y: toolY + 79, Width: 109, Height: 35}
			rl.DrawRectangleRounded(bounds, .14, 6, colors.panel)
			rl.DrawRectangleRoundedLinesEx(bounds, .14, 6, 1, colors.border)
			drawCentered(value, bounds, 10, colors.text)
		}
		scr := "—"
		if telemetry.SCRKnown {
			scr = fmt.Sprintf("%d", telemetry.SCR)
		}
		last := "—"
		if !telemetry.LastRecord.IsZero() {
			last = formatTETRAPOLAge(time.Since(telemetry.LastRecord))
		}
		integrity := "—"
		if telemetry.QualityKnown {
			integrity = fmt.Sprintf("%.0f%%", telemetry.QualityPercent)
		}
		simpleui.DrawText(fmt.Sprintf("Integridad CRC %s (5 s) · CRC %.1f/s · SCR %s · última %s", integrity, telemetry.CRCPerSecond, scr, last), 1112, toolY+126, 11, colors.text)
		asb := "—"
		if telemetry.ASBKnown {
			asb = fmt.Sprintf("%d%d", telemetry.ASB[0], telemetry.ASB[1])
		}
		cipher, cipherColor := "DESCONOCIDO · AUDIO BLOQUEADO", colors.orange
		if telemetry.CipherState == "clear" {
			cipher, cipherColor = "CLARO · AUDIO PERMITIDO", colors.green
		} else if telemetry.CipherState == "encrypted" {
			cipher, cipherColor = "CIFRADO · AUDIO BLOQUEADO", colors.red
		}
		simpleui.DrawText(fmt.Sprintf("ASB %s · offset %d · %s", asb, telemetry.RXOffset, cipher), 1112, toolY+149, 11, cipherColor)
		simpleui.DrawText(fmt.Sprintf("Runtime %s · %s/%s · nivel %.1f dBFS · desv. %.0f Hz · AFC no disponible", runtimeState, panel.bandType, panel.linkType, status.LevelDBFS, status.FrequencyDeviationHz), 1112, toolY+174, 10, colors.muted)
	}

	activity := rl.Rectangle{X: 360, Y: toolY + 216, Width: 1232, Height: 39}
	rl.DrawRectangleRounded(activity, .18, 6, colors.panelAlt)
	rl.DrawRectangleRoundedLinesEx(activity, .18, 6, 1, colors.border)
	simpleui.DrawTextStyled("ACTIVIDAD", 374, toolY+228, 11, simpleui.FontSemiBold, colors.cyan)
	barX := float32(452)
	for i := 0; i < 18; i++ {
		height := float32(4 + (i*7+int(telemetry.ValidFrames))%15)
		rl.DrawLineEx(rl.Vector2{X: barX + float32(i)*7, Y: toolY + 245}, rl.Vector2{X: barX + float32(i)*7, Y: toolY + 245 - height}, 2, colors.cyan)
	}
	summary := fmt.Sprintf("%s · trama %s (%s) · voz %s · %d válidas", syncText, frame, valueOrDash(telemetry.FrameType), voice, telemetry.ValidFrames)
	simpleui.DrawText(summary, 600, toolY+228, 11, colors.muted)
	if panel.feedback != "" {
		simpleui.DrawText(panel.feedback, 1240, toolY+228, 10, colors.red)
	}
}

func (panel *TETRAPOLPanel) drawEvents(telemetry tetrapolruntime.Telemetry) {
	events := telemetry.RecentEvents
	start := max(0, len(events)-6)
	if len(events) == 0 {
		simpleui.DrawText("Esperando eventos del runtime…", 1112, toolY+48, 11, colors.muted)
		return
	}
	for row, event := range events[start:] {
		y := toolY + 37 + float32(row)*26
		color := colors.text
		if event.Kind == "CRC" {
			color = colors.red
		} else if event.Kind == "VOICE" || event.Kind == "SCR" || (event.Kind == "CIPHER" && telemetry.CipherState == "clear") {
			color = colors.green
		} else if event.Kind == "CIPHER" {
			color = colors.red
		}
		simpleui.DrawTextStyled(event.Time.Format("15:04:05"), 1112, y, 10, simpleui.FontMono, colors.muted)
		simpleui.DrawText(event.Kind, 1172, y, 10, color)
		simpleui.DrawText(trimMemory(event.Summary, 49), 1224, y, 10, colors.text)
	}
}

func formatTETRAPOLAge(age time.Duration) string {
	if age < 0 {
		age = 0
	}
	if age < time.Second {
		return "ahora"
	}
	if age < time.Minute {
		return fmt.Sprintf("hace %ds", int(age.Seconds()))
	}
	return fmt.Sprintf("hace %dm", int(age.Minutes()))
}

func valueOrDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
