package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"path/filepath"
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/simpleui"
)

const utilitiesRight = float32(350)

// UtilitiesSidebar keeps the three operational tools visible while a decoder
// remains selected in the lower workspace.
type UtilitiesSidebar struct {
	screen    *MainScreen
	controls  []simpleui.Element
	scan      *simpleui.Button
	scanLayer *simpleui.Switch
	scanRange *simpleui.Button
	scanMode  *simpleui.Button
	scanMem   *simpleui.Button
	groupPick *simpleui.Dropdown
	groupEdit *simpleui.Button
	previous  *simpleui.Button
	next      *simpleui.Button
	recall    *simpleui.Button
	add       *simpleui.Button
	edit      *simpleui.Button
	remove    *simpleui.Button
	record    *simpleui.Button
	pause     *simpleui.Button
	skip      *simpleui.Button
	folder    *simpleui.Button
	format    *simpleui.Switch
}

func NewUtilitiesSidebar(screen *MainScreen) *UtilitiesSidebar {
	p := &UtilitiesSidebar{screen: screen}
	button := func(id, label string, x, y, w, h float32, action func()) *simpleui.Button {
		b := simpleui.NewButton(id, x, y, w, h, label, 11)
		b.SetColors(colors.panelAlt, colors.border, colors.text)
		b.OnClick(action)
		p.controls = append(p.controls, b)
		return b
	}
	p.scan = button("utilityScan", i18n.Source("text.7f23d98fbc9a"), 24, 339, 150, 34, screen.scanPanel.ToggleRunning)
	p.scanLayer = simpleui.NewSwitch("utilityScanLayer", 170, 237, 164, 28, i18n.Source("text.f96d81ccfe4b"), true, 11)
	p.scanLayer.SetTrackColors(colors.panelAlt, colors.green)
	p.scanLayer.OnChange(func(active bool) { screen.scanPanel.overlayVisible = active })
	p.controls = append(p.controls, p.scanLayer)
	p.scanRange = button("utilityScanRange", i18n.Source("text.bce5ee71842b"), 182, 339, 152, 34, func() {
		x := screen.centerFrequencyHz
		half := screen.spanHz / 2
		screen.scanPanel.minimumHz, screen.scanPanel.maximumHz = x-half+screen.spanHz/10, x+half-screen.spanHz/10
		screen.markSettingsDirty()
	})
	p.scanMode = button("utilityScanMode", i18n.Source("text.f9b36eae9bda"), 24, 298, 150, 31, func() {
		values := []string{i18n.Source("text.6ea56fae9eac"), i18n.Source("text.85135a165905"), i18n.Source("text.aacf94b7be62")}
		i := slices.Index(values, screen.scanPanel.resume)
		screen.scanPanel.resume = values[(i+1)%len(values)]
		screen.markSettingsDirty()
	})
	p.scanMem = button("utilityScanMemory", i18n.Source("text.0ab7ed3c35b1"), 182, 298, 152, 31, func() {
		screen.scanPanel.centerToMemory = !screen.scanPanel.centerToMemory
		screen.markSettingsDirty()
	})
	p.add = button("utilityMemoryAdd", i18n.Source("text.0692b8193af5"), 182, 405, 152, 31, screen.memoryPanel.openSaveModal)
	addGroup := button("utilityMemoryAddGroup", i18n.Source("text.6b5ffd7d7172"), 24, 405, 150, 31, screen.memoryPanel.openNewGroupModal)
	_ = addGroup
	p.groupPick = simpleui.NewDropdown("utilityMemoryFilter", 24, 442, 220, 32, i18n.Source("text.201f15dab8b3"), screen.memoryPanel.groups, 13)
	p.groupPick.OnChange(func(_ int, group string) {
		screen.memoryPanel.selectedGroup, screen.memoryPanel.selected, screen.memoryPanel.scrollOffset = group, -1, 0
	})
	p.controls = append(p.controls, p.groupPick)
	p.groupEdit = button("utilityMemoryGroupEdit", i18n.Source("text.762217dca4c2"), 250, 442, 84, 32, screen.memoryPanel.openGroupModal)
	p.previous = button("utilityMemoryPrevious", "◄", 24, 700, 48, 31, func() { p.moveMemory(-1) })
	p.next = button("utilityMemoryNext", "►", 78, 700, 48, 31, func() { p.moveMemory(1) })
	p.recall = button("utilityMemoryRecall", i18n.Source("text.4089163c4819"), 182, 700, 152, 31, screen.memoryPanel.tuneSelected)
	p.edit = button("utilityMemoryEdit", i18n.Source("text.762217dca4c2"), 24, 737, 150, 31, screen.memoryPanel.editSelection)
	p.remove = button("utilityMemoryDelete", i18n.Source("text.b243b05a8aa4"), 182, 737, 152, 31, screen.memoryPanel.openDeleteModal)
	p.remove.SetColors(actionClearFill, colors.red, colors.text)
	p.record = button("utilityRecord", i18n.Source("text.31d59748e4d1"), 24, 836, 94, 32, screen.recorderPanel.ToggleRecording)
	p.record.SetColors(actionStopFill, colors.red, colors.text)
	p.pause = button("utilityPause", i18n.Source("text.0b03bdac33fa"), 124, 836, 78, 32, screen.recorderPanel.TogglePause)
	p.skip = button("utilitySkip", i18n.Source("text.a7056a455639"), 208, 836, 66, 32, screen.recorderPanel.ToggleSkipSilence)
	p.folder = button("utilityFolder", i18n.Source("text.d313ad93be2d"), 280, 836, 54, 32, screen.recorderPanel.OpenFolder)
	p.format = simpleui.NewSwitch("utilityRecordFormat", 205, 796, 129, 27, "MP3", screen.recorderFormat == recorderFormatMP3, 11)
	p.format.SetTrackColors(colors.panelAlt, colors.green)
	p.format.OnChange(func(mp3 bool) {
		if screen.recorder.State().Recording {
			p.format.SetActive(screen.recorderFormat == recorderFormatMP3)
			return
		}
		screen.recorderFormat = recorderFormatWAV
		if mp3 {
			screen.recorderFormat = recorderFormatMP3
		}
		screen.recorder.SetFormat(screen.recorderFormat)
		screen.markSettingsDirty()
	})
	p.controls = append(p.controls, p.format)
	return p
}

func (p *UtilitiesSidebar) moveMemory(delta int) {
	indices := p.screen.memoryPanel.filteredIndices()
	p.screen.memoryPanel.moveSelection(indices, delta)
}

func (p *UtilitiesSidebar) Draw() {
	drawPanel(8, 215, utilitiesRight-8, 685)
	p.drawSection(225, 158, i18n.Source("text.31d2326366e4"), colors.cyan)
	p.drawSection(393, 385, i18n.Source("text.59fe2aae29ee"), colors.blue)
	p.drawSection(788, 102, i18n.Source("text.be637709464d"), colors.red)

	scan := p.screen.scanPanel
	if scan.running {
		p.scan.SetLabel(i18n.Source("text.42a572b1399e"))
		p.scan.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.scan.SetLabel(i18n.Source("text.7f23d98fbc9a"))
		p.scan.SetColors(actionStartFill, colors.green, colors.text)
	}
	simpleui.DrawTextStyled(scan.displayStatus(), 24, 258, 12, simpleui.FontSemiBold, func() rl.Color {
		if scan.running {
			return colors.green
		}
		return colors.muted
	}())
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.9ec780e23eee"), float64(scan.minimumHz)/1e6, float64(scan.maximumHz)/1e6, p.screen.squelchThreshold), 24, 278, 11, colors.muted)
	p.scanMode.SetLabel(i18n.Source("text.aa06f8a27805") + scan.resume)
	if scan.centerToMemory {
		p.scanMem.SetLabel(i18n.Source("text.6e2930c883e0"))
	} else {
		p.scanMem.SetLabel(i18n.Source("text.43f36ce02413"))
	}

	memory := p.screen.memoryPanel
	canEditGroup := memory.selectedGroup != "" && memory.selectedGroup != i18n.Source("text.201f15dab8b3") && memory.selectedGroup != i18n.Source("text.445a7d952a48")
	p.groupEdit.SetEnabled(canEditGroup)
	if !slices.Equal(p.groupPick.Items(), memory.groups) {
		p.groupPick.SetItems(memory.groups)
	}
	for i, group := range memory.groups {
		if group == memory.selectedGroup && p.groupPick.SelectedIndex() != i {
			p.groupPick.SetSelected(i)
		}
	}
	p.drawMemoryTable(memory)

	state := p.screen.recorder.State()
	status, statusColor := i18n.Source("text.d78afe9b19e4"), colors.muted
	if state.Recording {
		status, statusColor = i18n.Source("text.edd74dc734e6"), colors.red
		if state.Paused {
			status, statusColor = i18n.Source("text.033d2e9017bb"), colors.orange
		}
		p.record.SetLabel(i18n.Source("text.42a572b1399e"))
	} else {
		p.record.SetLabel(i18n.Source("text.31d59748e4d1"))
	}
	p.pause.SetEnabled(state.Recording)
	p.format.SetEnabled(!state.Recording)
	p.format.SetActive(state.Format == recorderFormatMP3)
	p.format.SetLabel(state.Format)
	if state.Paused {
		p.pause.SetLabel(i18n.Source("text.fad8cbb4bb22"))
	} else {
		p.pause.SetLabel(i18n.Source("text.0b03bdac33fa"))
	}
	if state.SkipSquelchSilence {
		p.skip.SetLabel(i18n.Source("text.1391629a9ebe"))
	} else {
		p.skip.SetLabel(i18n.Source("text.625060d3de2a"))
	}
	rl.DrawCircle(28, 817, 5, statusColor)
	simpleui.DrawTextStyled(status+"  "+formatRecordingDuration(state.DurationSeconds), 40, 809, 12, simpleui.FontMono, colors.text)
	if len(state.RecentFiles) > 0 {
		simpleui.DrawText(trimMemory(filepath.Base(state.RecentFiles[0]), 31), 24, 873, 10, colors.muted)
	}
}

func (p *UtilitiesSidebar) drawMemoryTable(memory *MemoryPanel) {
	x, y, w, rowH := float32(24), float32(481), float32(310), float32(29)
	rl.DrawRectangleLinesEx(rl.Rectangle{X: x, Y: y, Width: w, Height: 213}, 1, colors.border)
	simpleui.DrawTextStyled(i18n.Source("text.af1acd1a0fa8"), x+7, y+8, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(i18n.Source("text.1ce0cee583e6"), x+92, y+8, 13, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled("MHz", x+215, y+8, 13, simpleui.FontSemiBold, colors.cyan)
	indices := memory.filteredIndices()
	memory.scrollOffset = min(max(memory.scrollOffset, 0), max(len(indices)-6, 0))
	end := min(memory.scrollOffset+6, len(indices))
	for row, index := range indices[memory.scrollOffset:end] {
		m := memory.memories[index]
		yy := y + 32 + float32(row)*rowH
		groupText, rowText := memory.groupColor(memoryGroup(m)), colors.text
		if index == memory.selected {
			selection, selectionText := tableSelectionColors()
			rl.DrawRectangleRec(rl.Rectangle{X: x + 1, Y: yy - 4, Width: w - 2, Height: rowH}, selection)
			groupText, rowText = selectionText, selectionText
		}
		simpleui.DrawText(trimMemory(memoryGroup(m), 9), x+7, yy, 13, groupText)
		simpleui.DrawText(trimMemory(m.Name, 14), x+92, yy, 13, rowText)
		simpleui.DrawText(fmt.Sprintf("%.5f", float64(m.FrequencyHz)/1e6), x+215, yy, 13, rowText)
	}
}

func (p *UtilitiesSidebar) UpdateInput() {
	if p.screen.webServer != nil && p.screen.webServer.RemoteActive() {
		return
	}
	m := p.screen.memoryPanel
	mouse := simpleui.MousePosition()
	indices := m.filteredIndices()
	if mouse.X >= 24 && mouse.X <= 334 && mouse.Y >= 513 && mouse.Y < 687 {
		if wheel := rl.GetMouseWheelMove(); wheel != 0 {
			m.scrollOffset -= int(wheel)
			m.scrollOffset = min(max(m.scrollOffset, 0), max(len(indices)-6, 0))
		}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
			row := int((mouse.Y - 513) / 29)
			pos := m.scrollOffset + row
			if pos >= 0 && pos < len(indices) {
				index := indices[pos]
				now := rl.GetTime()
				m.selected = index
				if m.lastClicked == index && now-m.lastClickAt <= .42 {
					m.tuneSelected()
					m.lastClicked = -1
				} else {
					m.lastClicked, m.lastClickAt = index, now
				}
			}
		}
	}
}

func (p *UtilitiesSidebar) drawSection(y, height float32, title string, accent rl.Color) {
	rl.DrawRectangleRoundedLinesEx(rl.Rectangle{X: 15, Y: y, Width: utilitiesRight - 22, Height: height}, .05, 6, 1, colors.border)
	simpleui.DrawTextStyled(title, 24, y+8, 12, simpleui.FontSemiBold, accent)
}
