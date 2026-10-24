package screens

import (
	"fmt"
	"path/filepath"
	"time"

	"go-zero/internal/i18n"
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
	screen                                     *MainScreen
	controls                                   []simpleui.Element
	start, events, capture                     *simpleui.Button
	presets                                    [3]*simpleui.Button
	views                                      [4]*simpleui.Button
	gssiModes                                  [2]*simpleui.Button
	gssiDetectedAdd, gssiAllowedRemove         [3]*simpleui.Button
	gssiInput                                  *simpleui.TextField
	gssiManualAdd                              *simpleui.Button
	gssiControls                               []simpleui.Element
	channel, direction, band                   *simpleui.Dropdown
	audio, follow, autoCenter                  *simpleui.Switch
	enabled, visible, audioEnabled             bool
	followEnabled, following                   bool
	controlFrequencyHz                         int64
	lastChannelEvent                           uint64
	followedChannel                            int
	followStarted, lastTCHActivity             time.Time
	lastTCHAudioDecoded, lastTCHAudioConcealed uint64
	tchActivitySeen                            bool
	blockedChannel                             int
	blockedUntil                               time.Time
	followState                                string
	pendingManualTune                          bool
	restartAfter                               time.Time
	showEvents                                 bool
	activeView                                 int
	networkOpen, networkLive                   bool
	networkTab, networkOffset, networkPressed  int
	latestTelemetry, networkSnapshot           tetrapolruntime.Telemetry
	channelType, bandType, planType, linkType  string
	tchCipherState                             string
	feedback                                   string
	iqWriter                                   *iqcapture.Writer
	detectedGSSIs                              map[int]tetrapolruntime.GroupObservation
	detectedGSSIRows, allowedGSSIRows          []int
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
	panel := &TETRAPOLPanel{BaseElement: simpleui.NewBaseElement("tetrapolNetworkOverlay", 0, 0, designWidth, designHeight), screen: screen, channelType: "CCH", bandType: band, planType: planType, linkType: direction, audioEnabled: true, followEnabled: true, followedChannel: -1, blockedChannel: -1, followState: "CCH", networkLive: true, detectedGSSIs: make(map[int]tetrapolruntime.GroupObservation)}
	panel.start = simpleui.NewButton("tetrapolStart", 374, toolY+45, 132, 38, "INICIAR", 14)
	panel.start.OnClick(func() { panel.enabled = !panel.enabled; panel.apply() })
	panel.controls = append(panel.controls, panel.start)
	panel.follow = simpleui.NewSwitch("tetrapolFollow", 518, toolY+45, 145, 38, "SEGUIR TCH", true, 13)
	panel.follow.OnChange(func(enabled bool) { panel.followEnabled = enabled })
	panel.controls = append(panel.controls, panel.follow)

	for index := range panel.presets {
		i := index
		button := simpleui.NewButton(fmt.Sprintf("tetrapolBand%d", index), 704+float32(index)*124, toolY+50, 116, 34, "", 13)
		button.OnClick(func() { panel.activatePreset(i) })
		panel.presets[index] = button
		panel.controls = append(panel.controls, button)
	}

	panel.channel = simpleui.NewDropdown("tetrapolChannel", 374, toolY+105, 132, 34, "CANAL", []string{"CCH", "TCH"}, 13)
	panel.channel.SetSelected(0)
	panel.channel.OnChange(func(_ int, value string) {
		panel.channelType = value
		panel.following = false
		panel.tchCipherState = ""
		if value == "CCH" {
			panel.controlFrequencyHz = panel.screen.frequencyHz
		}
		panel.restartIfRunning()
	})
	panel.direction = simpleui.NewDropdown("tetrapolDirection", 518, toolY+105, 145, 34, "DIRECCIÓN", []string{"DOWNLINK", "UPLINK"}, 13)
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
	panel.band = simpleui.NewDropdown("tetrapolProfile", 374, toolY+157, 132, 34, "PERFIL", []string{"UHF 12,5", "UHF 10", "VHF"}, 13)
	panel.band.SetSelected(map[string]int{tetrapolPlanUHF125: 0, tetrapolPlanUHF10: 1, tetrapolPlanVHF: 2}[planType])
	panel.band.OnChange(func(index int, _ string) {
		panel.setPlan([]string{tetrapolPlanUHF125, tetrapolPlanUHF10, tetrapolPlanVHF}[index])
	})
	panel.audio = simpleui.NewSwitch("tetrapolAudio", 518, toolY+157, 145, 34, "AUDIO", true, 13)
	panel.audio.OnChange(func(enabled bool) {
		panel.audioEnabled = enabled
		if panel.screen.receiver != nil {
			panel.screen.receiver.SetTETRAPOLAudioEnabled(enabled)
		}
	})
	panel.autoCenter = simpleui.NewSwitch("tetrapolAFC", 704, toolY+157, 121, 30, "AFC", true, 13)
	panel.autoCenter.OnChange(func(enabled bool) {
		if panel.screen.receiver != nil {
			panel.screen.receiver.SetTETRAPOLAFCEnabled(enabled)
		}
	})
	panel.events = simpleui.NewButton("tetrapolEvents", 966, toolY+157, 105, 30, "RED", 13)
	panel.events.OnClick(panel.openNetwork)
	panel.capture = simpleui.NewButton("tetrapolCapture", 835, toolY+157, 121, 30, "CAPTURAR IQ", 11)
	panel.capture.SetColors(colors.blue, colors.border, colors.text)
	panel.capture.OnClick(panel.toggleIQCapture)
	panel.controls = append(panel.controls, panel.channel, panel.direction, panel.band, panel.audio, panel.autoCenter, panel.capture, panel.events)
	for index, label := range []string{"ESTADO", "RED", "LLAMADAS", "DIAG"} {
		i := index
		button := simpleui.NewButton(fmt.Sprintf("tetrapolView%d", index), 1110+float32(index)*116, toolY+9, 108, 23, label, 11)
		button.OnClick(func() { panel.setActiveView(i) })
		panel.views[index] = button
		panel.controls = append(panel.controls, button)
	}
	panel.createGSSIFilterControls()
	panel.setActiveView(0)
	panel.apply()
	panel.SetVisible(false)
	return panel
}

func (panel *TETRAPOLPanel) setActiveView(index int) {
	if index < 0 || index >= len(panel.views) {
		return
	}
	panel.activeView = index
	for i, button := range panel.views {
		if button == nil {
			continue
		}
		if i == index {
			button.SetColors(colors.cyan, colors.cyan, colors.background)
		} else {
			button.SetColors(colors.panel, colors.border, colors.muted)
		}
	}
	panel.setGSSIFilterControlsVisible(panel.visible && index == 1)
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
	panel.tuneWithSpan(hz, true)
}

func (panel *TETRAPOLPanel) tunePreservingSpan(hz int64) {
	panel.tuneWithSpan(hz, false)
}

// tuneFollowChannel keeps the SDR hardware centered on the CCH whenever the
// assigned traffic channel is already inside the captured wideband IQ. Only
// the digital channelizer moves, which avoids USB retuning and stale CCH IQ.
// For distant channels it waits briefly for the hardware tune to complete
// before the native decoder is restarted.
func (panel *TETRAPOLPanel) tuneFollowChannel(hz int64) {
	step := tetrapolPlan(panel.planType).stepHz
	panel.screen.frequencyHz = hz
	panel.screen.tuningStepHz = step
	panel.screen.updateBandForFrequency(hz)
	if panel.screen.stepSelector != nil {
		panel.screen.stepSelector.SetSelected(step)
	}
	if panel.screen.receiver != nil {
		if panel.screen.receiver.CanTuneWithinCapture(hz, int(step)) {
			panel.screen.centerFrequencyHz = panel.screen.receiver.CenterFrequency()
			panel.screen.receiver.SetDemodulator(tetrapolToolID, hz, int(step))
		} else {
			panel.screen.centerFrequencyHz = hz
			panel.screen.receiver.SetCenterFrequencyAndWait(hz, 350*time.Millisecond)
			panel.screen.receiver.SetDemodulator(tetrapolToolID, hz, int(step))
		}
	}
	panel.screen.markSettingsDirty()
}

func (panel *TETRAPOLPanel) tuneWithSpan(hz int64, resetSpan bool) {
	step := tetrapolPlan(panel.planType).stepHz
	panel.screen.frequencyHz, panel.screen.centerFrequencyHz = hz, hz
	if resetSpan {
		panel.screen.spanHz = 250_000
	}
	panel.screen.tuningStepHz = step
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
	if err := panel.screen.receiver.StartTETRAPOLRuntimeConfiguredWithCipher(panel.channelType, panel.bandType, panel.linkType, panel.tchCipherState); err != nil {
		panel.enabled = false
		panel.feedback = err.Error()
	}
}

// resetAfterManualTune stops decoding immediately, then starts a fresh
// receive pipeline once the user has stopped moving the wheel. That prevents
// old-frame telemetry and channelizer history from being carried into the new
// channel while avoiding one process restart for every wheel tick.
func (panel *TETRAPOLPanel) resetAfterManualTune() {
	panel.resetSessionForManualTune()
	if !panel.enabled {
		return
	}
	if panel.screen.receiver == nil {
		return
	}
	if !panel.pendingManualTune {
		// A manual FFT tune starts a different listening session. Unlike an
		// automatic CCH/TCH transition, no network information from the old
		// frequency is valid on the new one.
		panel.screen.receiver.ClearTETRAPOLNetworkState()
		_ = panel.screen.receiver.StopTETRAPOLRuntime()
		panel.feedback = "Reajustando TETRAPOL…"
	}
	panel.pendingManualTune = true
	panel.restartAfter = time.Now().Add(350 * time.Millisecond)
}

func (panel *TETRAPOLPanel) resetSessionForManualTune() {
	panel.following = false
	panel.followedChannel = -1
	panel.controlFrequencyHz = 0
	panel.lastChannelEvent = 0
	panel.followStarted, panel.lastTCHActivity = time.Time{}, time.Time{}
	panel.lastTCHAudioDecoded, panel.lastTCHAudioConcealed = 0, 0
	panel.tchActivitySeen = false
	panel.blockedChannel = -1
	panel.blockedUntil = time.Time{}
	panel.tchCipherState = ""
	panel.latestTelemetry = tetrapolruntime.Telemetry{}
	panel.networkSnapshot = tetrapolruntime.Telemetry{}
	clear(panel.detectedGSSIs)
	panel.detectedGSSIRows = panel.detectedGSSIRows[:0]
	panel.networkLive = true
	panel.networkOffset = 0
	if panel.channelType == "TCH" {
		panel.followState = "TCH MANUAL"
	} else {
		panel.followState = "CCH"
	}
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

func (panel *TETRAPOLPanel) reconcileAcquisitionFeedback(telemetry tetrapolruntime.Telemetry) {
	if !telemetry.Synchronized {
		return
	}
	if panel.feedback == "Buscando sincronismo en la nueva frecuencia…" || panel.feedback == "Reajustando TETRAPOL…" {
		panel.feedback = ""
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
		if telemetry.ChannelAction == "return" && telemetry.ChannelEvent != 0 {
			panel.lastChannelEvent = telemetry.ChannelEvent
			panel.returnToControl("Fin de comunicación · retorno automático al CCH", now)
			return
		}
		audioProgress := telemetry.AudioDecoded > panel.lastTCHAudioDecoded ||
			(telemetry.AudioDecoded > 0 && telemetry.AudioConcealed > panel.lastTCHAudioConcealed)
		panel.lastTCHAudioDecoded = max(panel.lastTCHAudioDecoded, telemetry.AudioDecoded)
		panel.lastTCHAudioConcealed = max(panel.lastTCHAudioConcealed, telemetry.AudioConcealed)
		if telemetry.VoiceActive || audioProgress || (telemetry.Synchronized && !panel.tchActivitySeen) {
			panel.tchActivitySeen = true
			panel.followState = "TCH ACTIVO"
			panel.lastTCHActivity = now
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
		if telemetry.CipherState == "encrypted" {
			panel.feedback = fmt.Sprintf("TCH %d cifrado · llamada ignorada", telemetry.ChannelID)
			return
		}
		if panel.screen.tetrapolGSSIFilter {
			groupID, known := tetrapolFollowGroupID(telemetry)
			if !known {
				panel.feedback = fmt.Sprintf("TCH %d ignorado · GSSI sin identificar", telemetry.ChannelID)
				return
			}
			if !containsTETRAPOLGSSI(panel.screen.tetrapolAllowedGSSIs, groupID) {
				panel.feedback = fmt.Sprintf("GSSI %d ignorado · no autorizado", groupID)
				return
			}
		}
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
		panel.tchCipherState = telemetry.CipherState
		panel.followedChannel = telemetry.ChannelID
		panel.followStarted = now
		panel.lastTCHActivity = time.Time{}
		panel.lastTCHAudioDecoded, panel.lastTCHAudioConcealed = 0, 0
		panel.tchActivitySeen = false
		panel.followState = "ADQUIRIENDO TCH"
		panel.channelType = "TCH"
		panel.channel.SetSelected(1)
		if panel.screen.receiver != nil {
			panel.screen.receiver.SetTETRAPOLFollowContext(telemetry.CellID, telemetry.ChannelID)
		}
		panel.tuneFollowChannel(targetHz)
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
	panel.tchCipherState = ""
	panel.followedChannel = -1
	panel.followStarted, panel.lastTCHActivity = time.Time{}, time.Time{}
	panel.lastTCHAudioDecoded, panel.lastTCHAudioConcealed = 0, 0
	panel.tchActivitySeen = false
	panel.followState = "CCH"
	panel.channelType = "CCH"
	panel.channel.SetSelected(0)
	if panel.screen.receiver != nil {
		panel.screen.receiver.SetTETRAPOLFollowContext("", -1)
	}
	panel.tuneFollowChannel(controlHz)
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
	panel.setGSSIFilterControlsVisible(visible && panel.activeView == 1)
	panel.refreshPresetControls()
}

func drawTETRAPOLGroup(bounds rl.Rectangle, title string) {
	fill := colors.panelAlt
	fill.A = 150
	rl.DrawRectangleRounded(bounds, .04, 6, fill)
	rl.DrawRectangleRoundedLinesEx(bounds, .04, 6, 1, colors.border)
	simpleui.DrawTextStyled(title, bounds.X+12, bounds.Y+8, 14, simpleui.FontSemiBold, colors.cyan)
}

func tetrapolAudioCipherPresentation(state string) (string, rl.Color) {
	switch state {
	case "clear":
		return "AUDIO ABIERTO", colors.green
	case "encrypted":
		return "AUDIO CIFRADO", colors.red
	default:
		return "SIN IDENTIFICAR", colors.orange
	}
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
			if runtime.Backend != "" {
				runtimeState += " · " + runtime.Backend
			}
		}
		if runtime.Error != "" {
			panel.feedback = runtime.Error
		}
	}
	if panel.pendingManualTune {
		// Stop() leaves the last snapshot readable until the replacement runtime
		// starts.  Do not display or act on that old frequency's frames during
		// the debounce interval.
		telemetry = tetrapolruntime.Telemetry{}
		runtimeState = "REINICIANDO"
	} else {
		panel.handleChannelFollow(telemetry)
	}
	panel.reconcileAcquisitionFeedback(telemetry)
	panel.latestTelemetry = telemetry
	panel.observeGSSIGroups(telemetry)
	panel.drawReadableHeader(status, telemetry)

	drawTETRAPOLGroup(rl.Rectangle{X: 360, Y: toolY + 5, Width: 315, Height: 202}, "CONTROL TETRAPOL")
	drawTETRAPOLGroup(rl.Rectangle{X: 686, Y: toolY + 5, Width: 400, Height: 202}, "SINTONÍA")
	drawTETRAPOLGroup(rl.Rectangle{X: 1097, Y: toolY + 5, Width: 495, Height: 202}, "")

	simpleui.DrawText("CANAL", 374, toolY+91, 12, colors.text)
	simpleui.DrawText("DIRECCIÓN", 518, toolY+91, 12, colors.text)
	simpleui.DrawText("PERFIL DE RED", 374, toolY+143, 12, colors.text)
	simpleui.DrawText("RP-CELP", 518, toolY+143, 12, colors.text)
	simpleui.DrawText("ACCESOS DE BANDA", 704, toolY+34, 12, colors.text)
	simpleui.DrawText("PASO", 704, toolY+104, 12, colors.text)
	stepLabel := fmt.Sprintf("%.1f kHz", float64(tetrapolPlan(panel.planType).stepHz)/1_000)
	simpleui.DrawTextStyled(stepLabel, 704, toolY+120, 17, simpleui.FontMono, colors.text)
	simpleui.DrawText("CANAL", 835, toolY+104, 12, colors.text)
	simpleui.DrawTextStyled(stepLabel, 835, toolY+120, 17, simpleui.FontMono, colors.text)
	simpleui.DrawText("MODULACIÓN", 966, toolY+104, 12, colors.text)
	simpleui.DrawTextStyled("GMSK 8 kBd", 966, toolY+120, 17, simpleui.FontMono, colors.text)
	simpleui.DrawText("AJUSTE AUTOMÁTICO", 704, toolY+143, 12, colors.text)

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
	if panel.activeView == 1 {
		panel.drawGSSIFilter()
	} else if panel.showEvents {
		panel.drawEvents(telemetry)
	} else if panel.activeView == 2 {
		panel.drawConversationSummary(telemetry)
	} else if panel.activeView == 3 {
		panel.drawDiagnostics(status, telemetry, runtimeState)
	} else {
		syncBounds := rl.Rectangle{X: 1110, Y: toolY + 39, Width: 293, Height: 32}
		rl.DrawRectangleRounded(syncBounds, .25, 8, syncColor)
		drawCentered(syncText, syncBounds, 15, simpleui.EnsureTextContrast(colors.text, syncColor))
		audioLabel, audioColor := tetrapolAudioCipherPresentation(telemetry.CipherState)
		if telemetry.LogicalChannel == "CCH" {
			audioLabel, audioColor = "CONTROL · SIN AUDIO", colors.blue
		}
		audioBounds := rl.Rectangle{X: 1411, Y: toolY + 39, Width: 168, Height: 32}
		rl.DrawRectangleRounded(audioBounds, .25, 8, audioColor)
		drawCentered(audioLabel, audioBounds, 12, simpleui.EnsureTextContrast(colors.text, audioColor))
		items := []string{
			i18n.Source("text.tetrapolFrame") + "  " + frame,
			i18n.Display("TIPO") + "  " + valueOrDash(telemetry.FrameType),
			i18n.Display("CANAL") + "  " + valueOrDash(telemetry.LogicalChannel),
			i18n.Display("VOZ") + "  " + i18n.Display(voice),
		}
		for index, value := range items {
			bounds := rl.Rectangle{X: 1110 + float32(index)*117, Y: toolY + 79, Width: 109, Height: 35}
			rl.DrawRectangleRounded(bounds, .14, 6, colors.panel)
			rl.DrawRectangleRoundedLinesEx(bounds, .14, 6, 1, colors.border)
			drawCentered(value, bounds, 12, colors.text)
		}
		scr := "—"
		if telemetry.SCRKnown {
			scr = fmt.Sprintf("%d", telemetry.SCR)
		}
		last := "—"
		if !telemetry.LastRecord.IsZero() {
			last = formatTETRAPOLAge(time.Since(telemetry.LastRecord))
		}
		integrity := "MIDIENDO"
		if telemetry.QualityKnown {
			integrity = fmt.Sprintf("%.0f%% (%d tramas)", telemetry.QualityPercent, telemetry.QualitySamples)
		}
		drawTETRAPOLMetric("CRC", integrity, 1112, 1145, toolY+126, 12, colors.text)
		drawTETRAPOLMetric("ERR/s", fmt.Sprintf("%.1f", telemetry.CRCPerSecond), 1280, 1325, toolY+126, 12, colors.text)
		drawTETRAPOLMetric("SCR", scr, 1385, 1420, toolY+126, 12, colors.text)
		drawTETRAPOLMetric("ÚLT.", last, 1470, 1510, toolY+126, 12, colors.text)
		asb := "—"
		if telemetry.ASBKnown {
			asb = fmt.Sprintf("%d%d", telemetry.ASB[0], telemetry.ASB[1])
		}
		cipher, cipherColor := "DESCONOCIDO · AUDIO ACTIVO", colors.orange
		if telemetry.CipherState == "clear" {
			cipher, cipherColor = "CLARO · AUDIO PERMITIDO", colors.green
		} else if telemetry.CipherState == "encrypted" {
			cipher, cipherColor = "CIFRADO · AUDIO BLOQUEADO", colors.red
		}
		if telemetry.LogicalChannel == "CCH" {
			cipher, cipherColor = "CIFRADO N/A · AUDIO SOLO EN TCH", colors.cyan
		}
		drawTETRAPOLMetric("ASB", asb, 1112, 1145, toolY+149, 12, cipherColor)
		drawTETRAPOLMetric("OFFSET", fmt.Sprintf("%d", telemetry.RXOffset), 1200, 1255, toolY+149, 12, cipherColor)
		simpleui.DrawTextStyled(cipher, 1320, toolY+149, 12, simpleui.FontSemiBold, cipherColor)
		drawTETRAPOLMetric("RUNTIME", runtimeState, 1112, 1175, toolY+174, 11, colors.text)
		drawTETRAPOLMetric("ESTADO", panel.followState, 1250, 1302, toolY+174, 11, colors.text)
		drawTETRAPOLMetric("RED", panel.bandType+"/"+panel.linkType, 1390, 1420, toolY+174, 11, colors.text)
		drawTETRAPOLMetric("AFC", fmt.Sprintf("%+.0f/%+.0f", telemetry.AFCFrequencyHz, telemetry.AFCErrorHz), 1510, 1538, toolY+174, 11, colors.text)
	}

	activity := rl.Rectangle{X: 360, Y: toolY + 216, Width: 1232, Height: 39}
	rl.DrawRectangleRounded(activity, .18, 6, colors.panelAlt)
	rl.DrawRectangleRoundedLinesEx(activity, .18, 6, 1, colors.border)
	simpleui.DrawTextStyled("ACTIVIDAD", 374, toolY+228, 13, simpleui.FontSemiBold, colors.cyan)
	barX := float32(452)
	activityPhase := int((telemetry.IQSamples / 256) % 15)
	for i := 0; i < 18; i++ {
		height := float32(4 + (i*7+activityPhase)%15)
		rl.DrawLineEx(rl.Vector2{X: barX + float32(i)*7, Y: toolY + 245}, rl.Vector2{X: barX + float32(i)*7, Y: toolY + 245 - height}, 2, colors.cyan)
	}
	simpleui.DrawTextStyled(map[bool]string{true: "SYNC OK", false: "BUSCANDO"}[telemetry.Synchronized], 600, toolY+228, 12, simpleui.FontSemiBold, syncColor)
	drawTETRAPOLMetric("TRAMA", frame, 700, 748, toolY+228, 11, colors.text)
	drawTETRAPOLMetric("VOZ", voice, 805, 840, toolY+228, 11, colors.text)
	drawTETRAPOLMetric("VÁL.", fmt.Sprintf("%d", telemetry.ValidFrames), 925, 965, toolY+228, 11, colors.text)
	drawTETRAPOLMetric("AUDIO", fmt.Sprintf("%d", telemetry.AudioDecoded), 1040, 1085, toolY+228, 11, colors.text)
	drawTETRAPOLMetric("PLC", fmt.Sprintf("%d", telemetry.AudioConcealed), 1150, 1180, toolY+228, 11, colors.text)
	drawTETRAPOLMetric("RED", fmt.Sprintf("%d/%d", len(telemetry.Channels), len(telemetry.Groups)), 1235, 1265, toolY+228, 11, colors.text)
	codop := "—"
	if telemetry.CODOPKnown {
		codop = fmt.Sprintf("%02X", telemetry.LastCODOP)
	}
	drawTETRAPOLMetric("CODOP", codop, 1335, 1385, toolY+228, 11, colors.text)
	if panel.feedback != "" {
		feedbackWidth := simpleui.MeasureText(panel.feedback, 10).X
		feedbackX := activity.X + activity.Width - 14 - feedbackWidth
		simpleui.DrawText(panel.feedback, feedbackX, toolY+228, 10, colors.red)
	}
}

func drawTETRAPOLMetric(label, value string, labelX, valueX, y float32, size int32, color rl.Color) {
	simpleui.DrawTextStyled(label, labelX, y, size, simpleui.FontSemiBold, colors.muted)
	simpleui.DrawTextStyled(value, valueX, y, size, simpleui.FontMono, color)
}

func tetrapolNativeAFCState(telemetry tetrapolruntime.Telemetry) string {
	if telemetry.NativeRate <= 0 {
		return "—"
	}
	if telemetry.AFCLocked {
		return "LOCKED"
	}
	if telemetry.AFCMode != "" {
		return telemetry.AFCMode
	}
	return "ACQ"
}

func (panel *TETRAPOLPanel) drawReadableHeader(status tetrapol.Status, telemetry tetrapolruntime.Telemetry) {
	y := float32(548)
	syncText, syncColor := "BUSCANDO SINCRONISMO", colors.orange
	if telemetry.Synchronized {
		syncText, syncColor = "SINCRONISMO BLOQUEADO", colors.green
	}
	syncBounds := rl.Rectangle{X: 370, Y: y + 8, Width: 330, Height: 56}
	rl.DrawRectangleRounded(syncBounds, .16, 7, syncColor)
	drawCentered(syncText, syncBounds, 16, simpleui.EnsureTextContrast(colors.text, syncColor))

	channel := valueOrDash(telemetry.LogicalChannel)
	protocol := "ESPERANDO DATOS DE RED"
	if telemetry.CODOPKnown {
		protocol = fmt.Sprintf("CODOP %02X", telemetry.LastCODOP)
		if telemetry.LastCODOP == 0x90 {
			protocol = "D_SYSTEM_INFO"
		}
	}
	drawTETRAPOLMetric("CANAL", channel, 725, 790, y+8, 16, colors.cyan)
	drawTETRAPOLMetric("SCR", fmt.Sprintf("%d", telemetry.SCR), 880, 925, y+8, 16, colors.cyan)
	simpleui.DrawTextStyled(protocol, 725, y+36, 14, simpleui.FontSemiBold, colors.text)
	drawTETRAPOLMetric("TSDU", fmt.Sprintf("%d", telemetry.TSDUCount), 965, 1015, y+36, 14, colors.text)
	drawTETRAPOLMetric("VÁLIDAS", fmt.Sprintf("%d", telemetry.ValidFrames), 1080, 1145, y+36, 14, colors.text)

	drawTETRAPOLMetric("SEÑAL", fmt.Sprintf("%6.1f dBFS", status.LevelDBFS), 1250, 1310, y+8, 15, colors.text)
	drawTETRAPOLMetric("AFC", tetrapolNativeAFCState(telemetry), 1250, 1290, y+36, 13, colors.text)
	drawTETRAPOLMetric("CORR.", fmt.Sprintf("%+5.0f Hz", telemetry.AFCFrequencyHz), 1390, 1440, y+36, 13, colors.text)
	drawTETRAPOLMetric("ERR.", fmt.Sprintf("%+4.0f", telemetry.AFCErrorHz), 1530, 1570, y+36, 13, colors.text)
}

func (panel *TETRAPOLPanel) drawConversationSummary(telemetry tetrapolruntime.Telemetry) {
	simpleui.DrawTextStyled("LLAMADA ACTUAL", 1112, toolY+45, 13, simpleui.FontSemiBold, colors.cyan)
	state, color := tetrapolAudioCipherPresentation(telemetry.CipherState)
	if telemetry.VoiceActive {
		state = "VOZ CLARA ACTIVA"
		color = colors.green
	}
	badge := rl.Rectangle{X: 1110, Y: toolY + 65, Width: 469, Height: 38}
	rl.DrawRectangleRounded(badge, .18, 6, color)
	drawCentered(state, badge, 13, simpleui.EnsureTextContrast(colors.text, color))
	drawTETRAPOLMetric("CANAL", valueOrDash(telemetry.LogicalChannel), 1112, 1170, toolY+119, 14, colors.text)
	drawTETRAPOLMetric("AUDIO", fmt.Sprintf("%d", telemetry.AudioDecoded), 1260, 1315, toolY+119, 14, colors.text)
	drawTETRAPOLMetric("PLC", fmt.Sprintf("%d", telemetry.AudioConcealed), 1410, 1445, toolY+119, 14, colors.text)
	simpleui.DrawText("El historial se completará con las nuevas sesiones TCH.", 1112, toolY+146, 12, colors.text)
}

func (panel *TETRAPOLPanel) drawDiagnostics(status tetrapol.Status, telemetry tetrapolruntime.Telemetry, runtimeState string) {
	nativeTitle := "MOTOR NATIVO X64"
	if telemetry.NativeRate > 0 {
		nativeTitle = fmt.Sprintf("MOTOR NATIVO X64 · %.0f kS/s · %.2f SPS", telemetry.NativeRate/1_000, telemetry.NativeSPS)
	}
	simpleui.DrawTextStyled(nativeTitle, 1112, toolY+45, 13, simpleui.FontSemiBold, colors.cyan)
	sync := "BUSCANDO"
	if telemetry.Synchronized {
		sync = "BLOQUEADO"
	}
	drawTETRAPOLMetric("RUNTIME", runtimeState, 1112, 1190, toolY+70, 13, colors.text)
	drawTETRAPOLMetric("IQ DROP", fmt.Sprintf("%d · Q%d", telemetry.IQDropped, telemetry.IQQueued), 1390, 1460, toolY+70, 12, colors.text)
	drawTETRAPOLMetric("SYNC", sync, 1112, 1190, toolY+93, 13, colors.text)
	physicalSync := "NO"
	if telemetry.PhysicalSync {
		physicalSync = "SÍ"
	}
	drawTETRAPOLMetric("FÍSICO", physicalSync, 1280, 1340, toolY+93, 12, colors.text)
	drawTETRAPOLMetric("SCR", fmt.Sprintf("%d", telemetry.SCR), 1390, 1430, toolY+93, 13, colors.text)
	drawTETRAPOLMetric("VÁLIDAS", fmt.Sprintf("%d", telemetry.ValidFrames), 1112, 1190, toolY+116, 13, colors.text)
	drawTETRAPOLMetric("COR%", fmt.Sprintf("%d", telemetry.CorrelationDropPercent), 1280, 1330, toolY+116, 12, colors.text)
	drawTETRAPOLMetric("CRC", fmt.Sprintf("%d", telemetry.CRCFailures), 1410, 1450, toolY+116, 13, colors.text)
	drawTETRAPOLMetric("AUDIO", fmt.Sprintf("%d", telemetry.AudioDecoded), 1112, 1190, toolY+139, 13, colors.text)
	drawTETRAPOLMetric("PLC", fmt.Sprintf("%d", telemetry.AudioConcealed), 1280, 1320, toolY+139, 13, colors.text)
	drawTETRAPOLMetric("AUD DROP", fmt.Sprintf("%d", telemetry.AudioLongGaps), 1400, 1475, toolY+139, 12, colors.text)
	drawTETRAPOLMetric("IQ", fmt.Sprintf("%6.1f dBFS", status.LevelDBFS), 1112, 1150, toolY+162, 12, colors.text)
	drawTETRAPOLMetric("AFC", tetrapolNativeAFCState(telemetry), 1290, 1330, toolY+162, 12, colors.text)
	drawTETRAPOLMetric("CORR.", fmt.Sprintf("%+5.0f Hz", telemetry.AFCFrequencyHz), 1430, 1480, toolY+162, 12, colors.text)
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
		simpleui.DrawTextStyled(event.Time.Format("15:04:05"), 1112, y, 11, simpleui.FontMono, colors.text)
		simpleui.DrawText(event.Kind, 1178, y, 11, color)
		simpleui.DrawText(trimMemory(event.Summary, 43), 1238, y, 11, colors.text)
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
