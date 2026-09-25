package screens

import (
	"fmt"
	"go-zero/internal/i18n"
	"go-zero/internal/omnirig"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	rigOff = iota
	rigFollow
	rigBidirectional
)

const (
	rigFrequencyConfirmationToleranceHz = int64(25)
	rigFrequencyRetryInterval           = 500 * time.Millisecond
	rigFrequencyConfirmationTimeout     = 5 * time.Second
)

func (screen *MainScreen) cycleRigControl() {
	screen.rigControlMode = (screen.rigControlMode + 1) % 3
	screen.rigLastFrequencyHz = 0
	screen.rigLastMode = ""
	screen.rigLastProgramHz = screen.frequencyHz
	screen.rigLastProgramMode = screen.appModeForRig()
	screen.clearPendingRigFrequency()
	if screen.rigControlMode == rigOff {
		screen.setRigMuteApplied(false)
		screen.rigStatus = "Omni-Rig desactivado"
		if screen.rigClient != nil {
			screen.rigClient.Stop()
		}
	} else if screen.rigClient != nil {
		screen.rigStatus = "Iniciando Omni-Rig…"
		screen.rigClient.Start()
	}
	screen.updateRigButtonColor()
}

func (screen *MainScreen) updateRigButtonColor() {
	if screen.rigButton == nil {
		return
	}
	gray := rl.Color{R: 92, G: 101, B: 108, A: 255}
	switch screen.rigControlMode {
	case rigFollow:
		screen.rigButton.SetColors(colors.blue, rl.Color{R: 105, G: 190, B: 245, A: 255}, rl.White)
	case rigBidirectional:
		screen.rigButton.SetColors(colors.red, rl.Color{R: 255, G: 145, B: 145, A: 255}, rl.White)
	default:
		screen.rigButton.SetColors(gray, rl.Color{R: 135, G: 145, B: 152, A: 255}, rl.White)
	}
}

func (screen *MainScreen) updateRigControl() {
	if screen.rigControlMode == rigOff || screen.rigClient == nil {
		screen.setRigMuteApplied(false)
		return
	}
	state := screen.rigClient.State()
	if !state.Running && state.Error != "" {
		screen.setRigMuteApplied(false)
		screen.rigStatus = state.Error
		screen.rigControlMode = rigOff
		screen.updateRigButtonColor()
		return
	}
	if state.Status != "" {
		screen.rigStatus = state.Status
	}
	if !state.Online {
		screen.setRigMuteApplied(false)
		return
	}
	screen.updateRigTXMute(state)
	now := time.Now()
	programFrequencyChanged := screen.rigControlMode == rigBidirectional && screen.frequencyHz != screen.rigLastProgramHz
	if programFrequencyChanged {
		screen.rigPendingFrequencyHz = screen.frequencyHz
		screen.rigPendingSince = now
		screen.rigNextFrequencySend = time.Time{}
	}
	if screen.rigControlMode != rigBidirectional {
		screen.clearPendingRigFrequency()
	}
	if screen.rigPendingFrequencyHz > 0 {
		switch {
		case rigFrequencyConfirmed(state.FrequencyHz, screen.rigPendingFrequencyHz):
			screen.rigLastFrequencyHz = state.FrequencyHz
			screen.clearPendingRigFrequency()
		case now.Sub(screen.rigPendingSince) >= rigFrequencyConfirmationTimeout:
			screen.rigStatus = "La radio no confirmó la sintonía CAT"
			screen.clearPendingRigFrequency()
		case screen.rigNextFrequencySend.IsZero() || !now.Before(screen.rigNextFrequencySend):
			screen.rigClient.SetFrequency(screen.rigPendingFrequencyHz)
			screen.rigNextFrequencySend = now.Add(rigFrequencyRetryInterval)
		}
	}

	// While a program-originated CAT order is awaiting confirmation, an old
	// polled value must not immediately pull the application back to the radio.
	if screen.rigPendingFrequencyHz == 0 && state.FrequencyHz >= 100_000 && state.FrequencyHz <= 6_000_000_000 && state.FrequencyHz != screen.rigLastFrequencyHz {
		screen.rigLastFrequencyHz = state.FrequencyHz
		if state.FrequencyHz != screen.frequencyHz {
			screen.tuneFromRig(state.FrequencyHz)
		}
	}
	if state.Mode != "" && state.Mode != screen.rigLastMode {
		screen.rigLastMode = state.Mode
		screen.selectModeFromRig(state.Mode)
	}

	if screen.rigControlMode == rigBidirectional {
		mode := screen.appModeForRig()
		if mode != "" && mode != screen.rigLastProgramMode && mode != state.Mode {
			screen.rigClient.SetMode(mode)
		}
	}
	screen.rigLastProgramHz = screen.frequencyHz
	screen.rigLastProgramMode = screen.appModeForRig()
}

func (screen *MainScreen) clearPendingRigFrequency() {
	screen.rigPendingFrequencyHz = 0
	screen.rigPendingSince = time.Time{}
	screen.rigNextFrequencySend = time.Time{}
}

func rigFrequencyConfirmed(actual, wanted int64) bool {
	if actual <= 0 || wanted <= 0 {
		return false
	}
	difference := actual - wanted
	if difference < 0 {
		difference = -difference
	}
	return difference <= rigFrequencyConfirmationToleranceHz
}

func (screen *MainScreen) updateRigTXMute(state omnirig.State) {
	if !screen.rigMuteOnTX || !state.TXReadable || !state.Online {
		screen.rigTXSince = time.Time{}
		screen.setRigMuteApplied(false)
		return
	}
	if !state.Transmitting {
		screen.rigTXSince = time.Time{}
		screen.setRigMuteApplied(false)
		return
	}
	if screen.rigTXSince.IsZero() {
		screen.rigTXSince = time.Now()
		return
	}
	if time.Since(screen.rigTXSince) >= 75*time.Millisecond {
		screen.setRigMuteApplied(true)
	}
}

func (screen *MainScreen) setRigMuteApplied(applied bool) {
	if screen.rigMuteApplied == applied {
		return
	}
	screen.rigMuteApplied = applied
	if !applied {
		screen.rigTXSince = time.Time{}
	}
	screen.refreshMuteState()
}

func (screen *MainScreen) refreshMuteState() {
	effective := screen.muted || screen.rigMuteApplied
	if screen.audioPlayer != nil {
		screen.audioPlayer.SetMuted(effective)
	}
	if screen.volumeSlider != nil {
		screen.volumeSlider.SetEnabled(!effective)
	}
	if screen.muteSwitch != nil {
		screen.muteSwitch.SetActive(effective)
		screen.muteSwitch.SetEnabled(!screen.rigMuteApplied)
		if screen.rigMuteApplied {
			screen.muteSwitch.SetLabel("RIG → MUTE")
			screen.muteSwitch.SetTrackColors(colors.panelAlt, colors.red)
		} else {
			screen.muteSwitch.SetLabel(i18n.Source("text.699ea8f5b381"))
			screen.muteSwitch.ClearTrackColors()
		}
	}
	if screen.volumeLabel != nil {
		if screen.rigMuteApplied {
			screen.volumeLabel.SetText("RIG → MUTE")
			screen.volumeLabel.SetColor(colors.red)
		} else if screen.muted {
			screen.volumeLabel.SetText(i18n.Source("text.03b0c21a17ef"))
			screen.volumeLabel.SetColor(colors.red)
		} else {
			screen.volumeLabel.SetText(fmt.Sprintf(i18n.Source("text.594875f2c60c"), screen.volume))
			screen.volumeLabel.SetColor(colors.cyan)
		}
	}
}

func (screen *MainScreen) tuneFromRig(hz int64) {
	if screen.scanPanel != nil && screen.scanPanel.running {
		screen.scanPanel.Stop()
	}
	previousFrequency, previousCenter := screen.frequencyHz, screen.centerFrequencyHz
	screen.frequencyHz = hz
	if screen.centerMode || screen.spanHz <= 0 {
		screen.centerFrequencyHz = hz
	} else {
		halfSpan := screen.spanHz / 2
		guard := max(int64(screen.demodBandwidthHz), screen.spanHz/20)
		guard = min(guard, screen.spanHz*2/5)
		if hz < screen.centerFrequencyHz-halfSpan+guard {
			screen.centerFrequencyHz = hz + halfSpan - guard
		} else if hz > screen.centerFrequencyHz+halfSpan-guard {
			screen.centerFrequencyHz = hz - halfSpan + guard
		}
		screen.centerFrequencyHz = max(screen.centerFrequencyHz, halfSpan)
	}
	if screen.receiver != nil {
		if screen.centerFrequencyHz != previousCenter {
			screen.receiver.SetCenterFrequency(screen.centerFrequencyHz)
		}
		screen.receiver.SetDemodulator(screen.receiverDemodMode(), hz, screen.demodBandwidthHz)
	}
	if screen.centerFrequencyHz != previousCenter && screen.waterfall != nil {
		screen.waterfall.Reset()
	}
	screen.resetTETRAAfterManualSpectrumTune(previousFrequency)
	screen.markSettingsDirty()
}

func (screen *MainScreen) appModeForRig() string {
	if screen.mode == nil {
		return ""
	}
	switch screen.mode.SelectedText() {
	case "AM":
		return omnirig.ModeAM
	case "CW":
		return omnirig.ModeCW
	case i18n.Source("text.61f0acff1735"):
		return omnirig.ModeUSB
	case i18n.Source("text.6323db4948ad"):
		return omnirig.ModeLSB
	case i18n.Source("text.0896d612d497"), i18n.Source("text.6b742bac3eb4"):
		return omnirig.ModeFM
	default:
		return ""
	}
}

func (screen *MainScreen) selectModeFromRig(mode string) {
	var wanted string
	switch mode {
	case omnirig.ModeAM:
		wanted = "AM"
	case omnirig.ModeCW:
		wanted = "CW"
	case omnirig.ModeUSB:
		wanted = i18n.Source("text.61f0acff1735")
	case omnirig.ModeLSB:
		wanted = i18n.Source("text.6323db4948ad")
	case omnirig.ModeFM:
		wanted = i18n.Source("text.0896d612d497")
	default:
		return
	}
	if screen.mode == nil || screen.mode.SelectedText() == wanted {
		return
	}
	for index, item := range screen.mode.Items() {
		if item == wanted {
			screen.mode.SetSelected(index)
			screen.applyModeSelection(wanted)
			return
		}
	}
}

func (screen *MainScreen) applyModeSelection(mode string) {
	screen.savedMode = mode
	if screen.filterSelector != nil {
		screen.selectFilter(screen.filterSelector.Current(mode))
	}
	if mode == i18n.Source("text.2604864ce4d3") && screen.activeTool != i18n.Source("text.7a1580c49e45") {
		screen.selectTool(i18n.Source("text.93239b223632"))
	} else if mode != i18n.Source("text.2604864ce4d3") && screen.activeTool == i18n.Source("text.93239b223632") {
		screen.selectTool(i18n.Source("text.a42c60257b01"))
	}
	screen.markSettingsDirty()
}
