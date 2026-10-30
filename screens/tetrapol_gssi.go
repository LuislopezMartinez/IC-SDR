package screens

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"go-zero/internal/tetrapolruntime"
	"go-zero/simpleui"
)

const tetrapolMaximumGSSI = 99_999_999

func normalizeTETRAPOLGSSIs(values []int) []int {
	seen := make(map[int]bool, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value < 0 || value > tetrapolMaximumGSSI || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func containsTETRAPOLGSSI(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func tetrapolFollowGroupID(telemetry tetrapolruntime.Telemetry) (int, bool) {
	for _, channel := range telemetry.Channels {
		if channel.ChannelID == telemetry.ChannelID && channel.GroupID >= 0 {
			return channel.GroupID, true
		}
	}
	for _, group := range telemetry.Groups {
		if group.ChannelID == telemetry.ChannelID && group.GroupID >= 0 {
			return group.GroupID, true
		}
	}
	if len(telemetry.Groups) == 1 && telemetry.Groups[0].GroupID >= 0 {
		return telemetry.Groups[0].GroupID, true
	}
	return 0, false
}

func (panel *TETRAPOLPanel) createGSSIFilterControls() {
	for index, label := range []string{"TODOS", "SELECCIONADOS"} {
		i := index
		button := simpleui.NewButton(fmt.Sprintf("tetrapolGSSIMode%d", index), 1270+float32(index)*126, toolY+38, 120, 23, label, 10)
		button.OnClick(func() {
			panel.screen.tetrapolGSSIFilter = i == 1
			panel.refreshGSSIModeColors()
			panel.screen.markSettingsDirty()
		})
		panel.gssiModes[index] = button
		panel.gssiControls = append(panel.gssiControls, button)
	}
	for index := 0; index < 3; index++ {
		i := index
		add := simpleui.NewButton(fmt.Sprintf("tetrapolGSSIAdd%d", index), 1358, toolY+78+float32(index)*27, 27, 22, "+", 13)
		add.OnClick(func() {
			if i < len(panel.detectedGSSIRows) {
				panel.addAllowedGSSI(panel.detectedGSSIRows[i])
			}
		})
		remove := simpleui.NewButton(fmt.Sprintf("tetrapolGSSIRemove%d", index), 1550, toolY+78+float32(index)*27, 27, 22, "X", 10)
		remove.OnClick(func() {
			if i < len(panel.allowedGSSIRows) {
				panel.removeAllowedGSSI(panel.allowedGSSIRows[i])
			}
		})
		panel.gssiDetectedAdd[index], panel.gssiAllowedRemove[index] = add, remove
		panel.gssiControls = append(panel.gssiControls, add, remove)
	}
	panel.gssiInput = simpleui.NewTextField("tetrapolGSSIInput", 1438, toolY+166, 72, 24, "GSSI", 11)
	panel.gssiInput.SetMaxLength(8)
	panel.gssiInput.SetFilter(unicode.IsDigit)
	panel.gssiInput.OnSubmit(func(string) { panel.addManualGSSI() })
	panel.gssiManualAdd = simpleui.NewButton("tetrapolGSSIManualAdd", 1517, toolY+166, 60, 24, "+ AÑADIR", 9)
	panel.gssiManualAdd.OnClick(panel.addManualGSSI)
	panel.gssiControls = append(panel.gssiControls, panel.gssiInput, panel.gssiManualAdd)
	panel.controls = append(panel.controls, panel.gssiControls...)
	panel.refreshGSSIModeColors()
}

func (panel *TETRAPOLPanel) refreshGSSIModeColors() {
	for index, button := range panel.gssiModes {
		active := (index == 1) == panel.screen.tetrapolGSSIFilter
		if active {
			button.SetColors(colors.cyan, colors.cyan, colors.background)
		} else {
			button.SetColors(colors.panel, colors.border, colors.muted)
		}
	}
}

func (panel *TETRAPOLPanel) setGSSIFilterControlsVisible(visible bool) {
	for _, control := range panel.gssiControls {
		control.SetVisible(visible)
	}
}

func (panel *TETRAPOLPanel) addAllowedGSSI(groupID int) {
	if groupID < 0 || groupID > tetrapolMaximumGSSI {
		panel.feedback = "GSSI fuera de rango"
		return
	}
	panel.screen.tetrapolAllowedGSSIs = normalizeTETRAPOLGSSIs(append(panel.screen.tetrapolAllowedGSSIs, groupID))
	panel.screen.tetrapolGSSIFilter = true
	panel.refreshGSSIModeColors()
	panel.feedback = fmt.Sprintf("GSSI %d añadido a permitidos", groupID)
	panel.screen.markSettingsDirty()
}

func (panel *TETRAPOLPanel) removeAllowedGSSI(groupID int) {
	values := panel.screen.tetrapolAllowedGSSIs[:0]
	for _, value := range panel.screen.tetrapolAllowedGSSIs {
		if value != groupID {
			values = append(values, value)
		}
	}
	panel.screen.tetrapolAllowedGSSIs = normalizeTETRAPOLGSSIs(values)
	panel.feedback = fmt.Sprintf("GSSI %d eliminado", groupID)
	panel.screen.markSettingsDirty()
}

func (panel *TETRAPOLPanel) addManualGSSI() {
	value, err := strconv.Atoi(strings.TrimSpace(panel.gssiInput.Text()))
	if err != nil {
		panel.feedback = "Introduce un GSSI válido"
		return
	}
	panel.addAllowedGSSI(value)
	panel.gssiInput.SetText("")
}

func (panel *TETRAPOLPanel) observeGSSIGroups(telemetry tetrapolruntime.Telemetry) {
	for _, group := range telemetry.Groups {
		if group.GroupID >= 0 {
			panel.detectedGSSIs[group.GroupID] = group
		}
	}
}

func (panel *TETRAPOLPanel) refreshGSSIRows() {
	panel.detectedGSSIRows = panel.detectedGSSIRows[:0]
	for groupID := range panel.detectedGSSIs {
		panel.detectedGSSIRows = append(panel.detectedGSSIRows, groupID)
	}
	sort.Slice(panel.detectedGSSIRows, func(i, j int) bool {
		left, right := panel.detectedGSSIs[panel.detectedGSSIRows[i]], panel.detectedGSSIs[panel.detectedGSSIRows[j]]
		if left.LastSeen.Equal(right.LastSeen) {
			return left.GroupID < right.GroupID
		}
		return left.LastSeen.After(right.LastSeen)
	})
	if len(panel.detectedGSSIRows) > 3 {
		panel.detectedGSSIRows = panel.detectedGSSIRows[:3]
	}
	panel.allowedGSSIRows = append(panel.allowedGSSIRows[:0], panel.screen.tetrapolAllowedGSSIs...)
	if len(panel.allowedGSSIRows) > 3 {
		panel.allowedGSSIRows = panel.allowedGSSIRows[:3]
	}
	for index := 0; index < 3; index++ {
		panel.gssiDetectedAdd[index].SetEnabled(index < len(panel.detectedGSSIRows) && !containsTETRAPOLGSSI(panel.screen.tetrapolAllowedGSSIs, panel.detectedGSSIRows[index]))
		panel.gssiAllowedRemove[index].SetEnabled(index < len(panel.allowedGSSIRows))
	}
}

func (panel *TETRAPOLPanel) drawGSSIFilter() {
	panel.refreshGSSIRows()
	simpleui.DrawTextStyled("FILTRO GSSI", 1112, toolY+40, 12, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled("DETECTADOS", 1112, toolY+65, 10, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled("PERMITIDOS", 1402, toolY+65, 10, simpleui.FontSemiBold, colors.cyan)
	for row := 0; row < 3; row++ {
		y := toolY + 82 + float32(row)*27
		if row < len(panel.detectedGSSIRows) {
			groupID := panel.detectedGSSIRows[row]
			group := panel.detectedGSSIs[groupID]
			state, color := "DESCONOCIDO", colors.orange
			if group.CipherState == "clear" {
				state, color = "CLARO", colors.green
			} else if group.CipherState == "encrypted" {
				state, color = "CIFRADO", colors.red
			}
			simpleui.DrawTextStyled(fmt.Sprintf("%d", groupID), 1112, y, 11, simpleui.FontMono, colors.text)
			simpleui.DrawTextStyled(state, 1240, y, 10, simpleui.FontSemiBold, color)
		} else {
			simpleui.DrawText("—", 1112, y, 11, colors.muted)
		}
		if row < len(panel.allowedGSSIRows) {
			simpleui.DrawTextStyled("OK", 1402, y, 11, simpleui.FontSemiBold, colors.green)
			simpleui.DrawTextStyled(fmt.Sprintf("GSSI %d", panel.allowedGSSIRows[row]), 1422, y, 11, simpleui.FontMono, colors.text)
		} else {
			simpleui.DrawText("—", 1402, y, 11, colors.muted)
		}
	}
	simpleui.DrawText("GSSI", 1402, toolY+172, 10, colors.muted)
	mode := "TODOS LOS GRUPOS"
	modeColor := colors.muted
	if panel.screen.tetrapolGSSIFilter {
		mode = fmt.Sprintf("SOLO SELECCIONADOS · %d PERMITIDOS", len(panel.screen.tetrapolAllowedGSSIs))
		modeColor = colors.green
	}
	simpleui.DrawTextStyled(mode, 1112, toolY+178, 10, simpleui.FontSemiBold, modeColor)
}
