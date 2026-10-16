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

const (
	tetrapolTCHTimeout = 6 * time.Second
	tetrapolRearmDelay = 3 * time.Second
)

type tetrapolPreset struct {
	label string
	hz    int64
}

var tetrapolPresets = map[string][]tetrapolPreset{
	"VHF": {{"68–88 MHz", 78_000_000}, {"150–174 MHz", 162_000_000}},
	"UHF": {{"380–400 MHz", 390_000_000}, {"410–430 MHz", 420_000_000}, {"450–470 MHz", 460_000_000}},
}

type TETRAPOLPanel struct {
	simpleui.BaseElement
	screen                                    *MainScreen
	controls                                  []simpleui.Element
	start, events, capture                    *simpleui.Button
	presets                                   [3]*simpleui.Button
	channel, direction, band                  *simpleui.Dropdown
	audio, follow, autoCenter                 *simpleui.Switch
	enabled, visible, audioEnabled            bool
	followEnabled, following                  bool
	controlFrequencyHz                        int64
	lastChannelEvent                          uint64
	followedChannel                           int
	followStarted, lastTCHActivity            time.Time
	lastTCHRecord                             time.Time
	tchActivitySeen                           bool
	blockedChannel                            int
	blockedUntil                              time.Time
	followState                               string
	pendingManualTune                         bool
	restartAfter                              time.Time
	showEvents                                bool
	networkOpen, networkLive                  bool
	networkTab, networkOffset, networkPressed int
	latestTelemetry, networkSnapshot          tetrapolruntime.Telemetry
	channelType, bandType, planType, linkType string
	feedback                                  string
	iqWriter                                  *iqcapture.Writer
}

func NewTETRAPOLPanel(screen *MainScreen) *TETRAPOLPanel {
	planType := screen.tetrapolPlan
	if _, exists := tetrapolChannelPlans[planType]; !exists {
		if screen.tetrapolBand == "VHF" {
			planType = tetrapolPlanVHF
		} else {
			planType = tetrapolPlanUHF125
		}
	}
	plan := tetrapolPlan(planType)
	band := plan.band
	direction := screen.tetrapolDirection
	if direction != "UP" {
		direction = "DOWN"
	}
	panel := &TETRAPOLPanel{BaseElement: simpleui.NewBaseElement("tetrapolNetworkOverlay", 0, 0, designWidth, designHeight), screen: screen, channelType: "CCH", bandType: band, planType: planType, linkType: direction, audioEnabled: true, followEnabled: true, followedChannel: -1, blockedChannel: -1, followState: "CCH", networkLive: true}
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
	panel.band = simpleui.NewDropdown("tetrapolProfile", 374, toolY+157, 132, 34, "PERFIL", []string{"UHF 12,5", "UHF 10", "VHF"}, 11)
	panel.band.SetSelected(map[string]int{tetrapolPlanUHF125: 0, tetrapolPlanUHF10: 1, tetrapolPlanVHF: 2}[planType])
	panel.band.OnChange(func(index int, _ string) {
		panel.setPlan([]string{tetrapolPlanUHF125, tetrapolPlanUHF10, tetrapolPlanVHF}[index])
	})
	panel.audio = simpleui.NewSwitch("tetrapolAudio", 518, toolY+157, 145, 34, "AUDIO", true, 11)
	panel.audio.OnChange(func(enabled bool) {
		panel.audioEnabled = enabled
		if panel.screen.receiver != nil {
			panel.screen.receiver.SetTETRAPOLAudioEnabled(enabled)
		}
	})
	panel.autoCenter = simpleui.NewSwitch("tetrapolAFC", 704, toolY+157, 121, 30, "AFC", true, 11)
	panel.autoCenter.OnChange(func(enabled bool) {
		if panel.screen.receiver != nil {
			panel.screen.receiver.SetTETRAPOLAFCEnabled(enabled)
		}
	})
	panel.events = simpleui.NewButton("tetrapolEvents", 966, toolY+157, 105, 30, "RED", 11)
	panel.events.OnClick(panel.openNetwork)
	panel.capture = simpleui.NewButton("tetrapolCapture", 835, toolY+157, 121, 30, "CAPTURAR IQ", 10)
	panel.capture.SetColors(colors.blue, colors.border, colors.text)
	panel.capture.OnClick(panel.toggleIQCapture)
	panel.controls = append(panel.controls, panel.channel, panel.direction, panel.band, panel.audio, panel.autoCenter, panel.capture, panel.events)
	panel.apply()
	panel.SetVisible(false)
	return panel
}

func (panel *TETRAPOLPanel) setBand(band string) {
	if band == "VHF" {
		panel.setPlan(tetrapolPlanVHF)
	} else {
		panel.setPlan(tetrapolPlanUHF125)
	}
}

func (panel *TETRAPOLPanel) setPlan(planID string) {
	plan := tetrapolPlan(planID)
	panel.planType, panel.bandType = plan.id, plan.band
	panel.screen.tetrapolPlan, panel.screen.tetrapolBand = plan.id, plan.band
	panel.screen.tuningStepHz, panel.screen.demodBandwidthHz = plan.stepHz, int(plan.stepHz)
	if panel.screen.stepSelector != nil {
		panel.screen.stepSelector.SetSelected(plan.stepHz)
	}
	if panel.screen.receiver != nil {
		panel.screen.receiver.SetDemodulator(tetrapolToolID, panel.screen.frequencyHz, int(plan.stepHz))
	}
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
			bounds := rl.Rectangle{X: 704 + float32(index)*(width+8), Y: toolY + 50, Width: width, Height: 34}
			// Presets change width when the VHF/UHF profile changes. Rebuilding
			// their bounds in design coordinates used to undo the horizontal
			// remapping applied while the utilities rail is collapsed, leaving
			// these buttons shifted into the status card.
			if panel.screen.utilitiesCollapsed {
				bounds = collapsedToolBounds(bounds)
			}
			button.SetBounds(bounds)
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
	step := tetrapolPlan(panel.planType).stepHz
	panel.screen.demodBandwidthHz = int(step)
	if panel.screen.receiver != nil {
		panel.screen.receiver.SetDemodulator(tetrapolToolID, panel.screen.frequencyHz, int(step))
	}
	panel.screen.tuningStepHz = step
	if panel.screen.stepSelector != nil {
		panel.screen.stepSelector.SetSelected(panel.screen.tuningStepHz)
	}
}

func (panel *TETRAPOLPanel) tune(hz int64) {
	step := tetrapolPlan(panel.planType).stepHz
	panel.screen.frequencyHz, panel.screen.centerFrequencyHz = hz, hz
	panel.screen.spanHz, panel.screen.tuningStepHz = 250_000, step
	panel.screen.updateBandForFrequency(hz)
	if panel.screen.stepSelector != nil {
		panel.screen.stepSelector.SetSelected(panel.screen.tuningStepHz)
	}
	if panel.screen.receiver != nil {
		panel.screen.receiver.SetCenterFrequency(hz)
		panel.screen.receiver.SetDemodulator(tetrapolToolID, hz, int(step))
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
	panel.handleChannelFollowAt(telemetry, time.Now())
}

func (panel *TETRAPOLPanel) handleChannelFollowAt(telemetry tetrapolruntime.Telemetry, now time.Time) {
	if !panel.enabled || !panel.followEnabled {
		return
	}
	if panel.following {
		if telemetry.ChannelAction == "return" && telemetry.ChannelEvent != 0 && telemetry.ChannelEvent != panel.lastChannelEvent {
			panel.lastChannelEvent = telemetry.ChannelEvent
			panel.returnToControl("Fin de comunicación · retorno automático al CCH", now)
			return
		}
		freshRecord := !telemetry.LastRecord.IsZero() && telemetry.LastRecord.After(panel.lastTCHRecord)
		if telemetry.VoiceActive || freshRecord || (telemetry.Synchronized && !panel.tchActivitySeen) {
			panel.tchActivitySeen = true
			panel.followState = "TCH ACTIVO"
			panel.lastTCHActivity = now
			if freshRecord {
				panel.lastTCHRecord = telemetry.LastRecord
			}
		}
		lastActivity := panel.followStarted
		if panel.tchActivitySeen {
			lastActivity = panel.lastTCHActivity
		}
		if !lastActivity.IsZero() && now.Sub(lastActivity) >= tetrapolTCHTimeout {
			reason := "TCH sin actividad · retorno al CCH"
			if !panel.tchActivitySeen {
				reason = "TCH sin sincronismo · retorno al CCH"
			}
			panel.returnToControl(reason, now)
		}
		return
	}
	if telemetry.ChannelEvent == 0 || telemetry.ChannelEvent == panel.lastChannelEvent {
		return
	}
	panel.lastChannelEvent = telemetry.ChannelEvent
	if telemetry.ChannelAction == "follow" && panel.channelType == "CCH" {
		if telemetry.ChannelID == panel.blockedChannel && now.Before(panel.blockedUntil) {
			panel.feedback = fmt.Sprintf("TCH %d esperando rearme", telemetry.ChannelID)
			return
		}
		controlHz := panel.screen.frequencyHz
		targetHz, err := tetrapolChannelFrequencyForPlan(panel.planType, controlHz, telemetry.ChannelID)
		if err != nil {
			panel.feedback = err.Error()
			return
		}
		panel.controlFrequencyHz = controlHz
		panel.following = true
		panel.followedChannel = telemetry.ChannelID
		panel.followStarted = now
		panel.lastTCHActivity = time.Time{}
		panel.lastTCHRecord = time.Time{}
		panel.tchActivitySeen = false
		panel.followState = "ADQUIRIENDO TCH"
		panel.channelType = "TCH"
		panel.channel.SetSelected(1)
		panel.tune(targetHz)
		panel.feedback = fmt.Sprintf("Siguiendo TCH %d en %.4f MHz", telemetry.ChannelID, float64(targetHz)/1e6)
		panel.restart()
	}
}

func (panel *TETRAPOLPanel) returnToControl(reason string, now time.Time) {
	if panel.controlFrequencyHz <= 0 {
		return
	}
	controlHz := panel.controlFrequencyHz
	panel.blockedChannel = panel.followedChannel
	panel.blockedUntil = now.Add(tetrapolRearmDelay)
	panel.following = false
	panel.followedChannel = -1
	panel.followStarted, panel.lastTCHActivity, panel.lastTCHRecord = time.Time{}, time.Time{}, time.Time{}
	panel.tchActivitySeen = false
	panel.followState = "CCH"
	panel.channelType = "CCH"
	panel.channel.SetSelected(0)
	panel.tune(controlHz)
	panel.feedback = reason
	panel.restart()
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
			panel.screen.receiver.SetTETRAPOLAFCEnabled(panel.autoCenter.Active())
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
	panel.BaseElement.SetVisible(visible)
	if !visible {
		panel.networkOpen = false
	}
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
	panel.latestTelemetry = telemetry

	drawTETRAPOLGroup(rl.Rectangle{X: 360, Y: toolY + 5, Width: 315, Height: 202}, "CONTROL TETRAPOL")
	drawTETRAPOLGroup(rl.Rectangle{X: 686, Y: toolY + 5, Width: 400, Height: 202}, "SINTONÍA")
	drawTETRAPOLGroup(rl.Rectangle{X: 1097, Y: toolY + 5, Width: 495, Height: 202}, "ESTADO Y DECODIFICACIÓN")

	simpleui.DrawText("CANAL", 374, toolY+91, 10, colors.muted)
	simpleui.DrawText("DIRECCIÓN", 518, toolY+91, 10, colors.muted)
	simpleui.DrawText("PERFIL DE RED", 374, toolY+143, 10, colors.muted)
	simpleui.DrawText("RP-CELP", 518, toolY+143, 10, colors.muted)
	simpleui.DrawText("ACCESOS DE BANDA", 704, toolY+34, 10, colors.muted)
	simpleui.DrawText("PASO", 704, toolY+104, 10, colors.muted)
	stepLabel := fmt.Sprintf("%.1f kHz", float64(tetrapolPlan(panel.planType).stepHz)/1_000)
	simpleui.DrawTextStyled(stepLabel, 704, toolY+120, 15, simpleui.FontMono, colors.text)
	simpleui.DrawText("CANAL", 835, toolY+104, 10, colors.muted)
	simpleui.DrawTextStyled(stepLabel, 835, toolY+120, 15, simpleui.FontMono, colors.text)
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
		simpleui.DrawText(fmt.Sprintf("Runtime %s · %s · %s/%s · nivel %.1f dBFS · AFC %s %+.0f Hz · error %+.0f Hz", runtimeState, panel.followState, panel.bandType, panel.linkType, status.LevelDBFS, status.AFCState, status.CarrierOffsetHz, status.AFCErrorHz), 1112, toolY+174, 10, colors.muted)
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
	network := fmt.Sprintf("red %d CH / %d grupos", len(telemetry.Channels), len(telemetry.Groups))
	if telemetry.CODOPKnown {
		network += fmt.Sprintf(" · CODOP %02X", telemetry.LastCODOP)
	}
	summary := fmt.Sprintf("%s · trama %s (%s) · voz %s · %d válidas · audio %d / PLC %d · %s", syncText, frame, valueOrDash(telemetry.FrameType), voice, telemetry.ValidFrames, telemetry.AudioDecoded, telemetry.AudioConcealed, network)
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
