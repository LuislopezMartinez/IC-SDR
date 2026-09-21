package screens

import (
	"fmt"

	"go-zero/internal/omnirig"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const omniRigToolID = "OMNIRIG"

// OmniRigPanel exposes the operational CAT controls that Omni-Rig publishes
// through COM. Hardware-profile and serial-port editing remains behind the
// advanced button because the Omni-Rig API does not expose those fields.
type OmniRigPanel struct {
	screen   *MainScreen
	controls []simpleui.Element
	off      *simpleui.Button
	follow   *simpleui.Button
	bidir    *simpleui.Button
	apply    *simpleui.Button
	advanced *simpleui.Button
	rig1     *simpleui.Button
	rig2     *simpleui.Button
	muteOnTX *simpleui.Switch
	modes    map[string]*simpleui.Button
}

func NewOmniRigPanel(screen *MainScreen) *OmniRigPanel {
	panel := &OmniRigPanel{screen: screen, modes: make(map[string]*simpleui.Button)}
	// These are deliberately large: switching the physical radio is a primary
	// action and must remain obvious at a glance and easy to hit on touchscreens.
	panel.rig1 = simpleui.NewButton("omnirigRig1", 1200, toolY+48, 180, 176, "RIG 1 · SIN CONFIGURAR", 11)
	panel.rig2 = simpleui.NewButton("omnirigRig2", 1390, toolY+48, 180, 176, "RIG 2 · SIN CONFIGURAR", 11)
	panel.rig1.OnClick(func() { panel.selectRig(1) })
	panel.rig2.OnClick(func() { panel.selectRig(2) })
	panel.controls = append(panel.controls, panel.rig1, panel.rig2)
	panel.off = simpleui.NewButton("omnirigOff", 374, toolY+48, 105, 36, "APAGADO", 11)
	panel.follow = simpleui.NewButton("omnirigFollow", 489, toolY+48, 145, 36, "SEGUIR RADIO", 11)
	panel.bidir = simpleui.NewButton("omnirigBidir", 644, toolY+48, 145, 36, "BIDIRECCIONAL", 10)
	panel.off.OnClick(func() { panel.setControlMode(rigOff) })
	panel.follow.OnClick(func() { panel.setControlMode(rigFollow) })
	panel.bidir.OnClick(func() { panel.setControlMode(rigBidirectional) })
	panel.controls = append(panel.controls, panel.off, panel.follow, panel.bidir)

	panel.apply = simpleui.NewButton("omnirigApplyFrequency", 799, toolY+48, 156, 36, "ENVIAR SINTONÍA", 10)
	panel.apply.OnClick(func() {
		panel.ensureRunning()
		screen.rigClient.SetFrequency(screen.frequencyHz)
		if mode := screen.appModeForRig(); mode != "" {
			screen.rigClient.SetMode(mode)
		}
	})
	panel.advanced = simpleui.NewButton("omnirigAdvanced", 965, toolY+48, 205, 36, "CONFIG. AVANZADA", 10)
	panel.advanced.OnClick(func() {
		panel.ensureRunning()
		screen.rigClient.ShowDialog(true)
	})
	panel.controls = append(panel.controls, panel.apply, panel.advanced)

	for index, mode := range []string{omnirig.ModeAM, omnirig.ModeFM, omnirig.ModeUSB, omnirig.ModeLSB, omnirig.ModeCW} {
		selectedMode := mode
		button := simpleui.NewButton("omnirigMode"+mode, 374+float32(index)*112, toolY+165, 102, 34, mode, 11)
		button.OnClick(func() { panel.ensureRunning(); screen.rigClient.SetMode(selectedMode) })
		panel.modes[mode] = button
		panel.controls = append(panel.controls, button)
	}
	panel.muteOnTX = simpleui.NewSwitch("omnirigMuteOnTX", 950, toolY+165, 230, 34, "MUTE AUDIO EN TX", screen.rigMuteOnTX, 11)
	panel.muteOnTX.OnChange(func(enabled bool) {
		screen.rigMuteOnTX = enabled
		if !enabled {
			screen.setRigMuteApplied(false)
		}
		screen.markSettingsDirty()
	})
	panel.controls = append(panel.controls, panel.muteOnTX)
	panel.SetVisible(false)
	return panel
}

func (panel *OmniRigPanel) selectRig(number int) {
	if panel.screen.rigClient == nil {
		return
	}
	panel.screen.rigLastFrequencyHz = 0
	panel.screen.rigLastMode = ""
	panel.screen.rigLastProgramHz = panel.screen.frequencyHz
	panel.screen.rigLastProgramMode = panel.screen.appModeForRig()
	panel.screen.setRigMuteApplied(false)
	panel.screen.rigClient.SelectRig(number)
	if panel.screen.rigControlMode != rigOff {
		panel.screen.rigStatus = fmt.Sprintf("Cambiando a RIG %d…", number)
		panel.screen.rigClient.Start()
	}
}

func (panel *OmniRigPanel) ensureRunning() {
	if panel.screen.rigControlMode == rigOff {
		panel.setControlMode(rigFollow)
	} else if panel.screen.rigClient != nil {
		panel.screen.rigClient.Start()
	}
}

func (panel *OmniRigPanel) setControlMode(mode int) {
	if panel.screen.rigControlMode == mode {
		return
	}
	panel.screen.rigControlMode = (mode + 2) % 3
	panel.screen.cycleRigControl()
}

func (panel *OmniRigPanel) SetVisible(visible bool) {
	for _, control := range panel.controls {
		control.SetVisible(visible)
	}
}

func (panel *OmniRigPanel) DrawPanel() {
	state := panel.screen.rigClient.State()
	panel.rig1.SetLabel(rigSelectorLabel(1, state.Rig1))
	panel.rig2.SetLabel(rigSelectorLabel(2, state.Rig2))
	panel.styleRigSelector(panel.rig1, state.SelectedRig == 1, state.Rig1)
	panel.styleRigSelector(panel.rig2, state.SelectedRig == 2, state.Rig2)
	statusColor := colors.orange
	if state.Online {
		statusColor = colors.green
	} else if state.Error != "" {
		statusColor = colors.red
	}
	drawSmallText("CONTROL OMNIRIG · RIG 1", 376, toolY+14, colors.cyan)
	drawSmallText("ESTADO", 806, toolY+112, colors.muted)
	status := state.Status
	if status == "" {
		status = panel.screen.rigStatus
	}
	if status == "" {
		status = "DETENIDO"
	}
	simpleui.DrawText(status, 806, toolY+133, 13, statusColor)
	drawSmallText("RADIO", 376, toolY+112, colors.muted)
	rigType := state.RigType
	if rigType == "" {
		rigType = "SIN CONFIGURAR"
	}
	simpleui.DrawText(rigType, 376, toolY+133, 14, colors.text)
	drawSmallText("FRECUENCIA CAT", 526, toolY+112, colors.muted)
	frequency := "—"
	if state.FrequencyHz > 0 {
		frequency = fmt.Sprintf("%.6f MHz", float64(state.FrequencyHz)/1e6)
	}
	simpleui.DrawText(frequency, 526, toolY+133, 17, colors.text)
	drawSmallText("MODO CAT", 700, toolY+112, colors.muted)
	mode := state.Mode
	if mode == "" {
		mode = "—"
	}
	simpleui.DrawText(mode, 700, toolY+133, 14, colors.text)
	drawSmallText("PTT", 1040, toolY+112, colors.muted)
	ptt := "NO DISPONIBLE"
	pttColor := colors.muted
	if state.TXReadable {
		ptt, pttColor = "RX", colors.green
		if state.Transmitting {
			ptt, pttColor = "TX", colors.red
		}
	}
	simpleui.DrawText(ptt, 1040, toolY+133, 14, pttColor)
	simpleui.DrawText("La configuración avanzada se muestra solo al solicitarla; cerrar esa ventana no detiene CAT.", 376, toolY+222, 11, colors.muted)

	panel.off.SetColors(colors.panelAlt, colors.border, colors.text)
	panel.follow.SetColors(colors.panelAlt, colors.border, colors.text)
	panel.bidir.SetColors(colors.panelAlt, colors.border, colors.text)
	switch panel.screen.rigControlMode {
	case rigFollow:
		panel.follow.SetColors(colors.blue, colors.border, rl.White)
	case rigBidirectional:
		panel.bidir.SetColors(colors.red, colors.border, rl.White)
	default:
		panel.off.SetColors(rl.Color{R: 92, G: 101, B: 108, A: 255}, colors.border, rl.White)
	}
}

func rigSelectorLabel(number int, rig omnirig.RigSummary) string {
	model := rig.RigType
	if model == "" || model == "NONE" {
		model = "SIN CONFIGURAR"
	}
	return fmt.Sprintf("RIG %d · %s", number, model)
}

func (panel *OmniRigPanel) styleRigSelector(button *simpleui.Button, selected bool, rig omnirig.RigSummary) {
	background := colors.panelAlt
	border := colors.border
	if rig.Transmitting {
		border = colors.red
	} else if rig.Online {
		border = colors.green
	} else if rig.RigType != "" && rig.RigType != "NONE" {
		border = colors.orange
	}
	if selected {
		background = colors.blue
	}
	button.SetColors(background, border, colors.text)
}
