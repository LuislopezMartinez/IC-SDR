package screens

import (
	"go-zero/internal/i18n"

	"fmt"
	"path/filepath"
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/simpleui"
)

const (
	utilitiesRight          = float32(350)
	utilitiesCollapsedRight = float32(64)
)

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
	hotkey    *simpleui.Button
	add       *simpleui.Button
	edit      *simpleui.Button
	remove    *simpleui.Button
	record    *simpleui.Button
	pause     *simpleui.Button
	skip      *simpleui.Button
	folder    *simpleui.Button
	format    *simpleui.Switch
	collapse  *simpleui.Button
	scanState *simpleui.Indicator
	memState  *simpleui.Indicator
	recState  *simpleui.Indicator
	expanded  map[simpleui.Element]rl.Rectangle
}

func NewUtilitiesSidebar(screen *MainScreen) *UtilitiesSidebar {
	p := &UtilitiesSidebar{screen: screen}
	button := func(id, label string, x, y, w, h float32, action func()) *simpleui.Button {
		b := simpleui.NewButton(id, x, y, w, h, label, uiBodyFontSize)
		b.SetColors(colors.panelAlt, colors.border, colors.text)
		b.OnClick(action)
		p.controls = append(p.controls, b)
		return b
	}
	p.scan = button("utilityScan", i18n.Source("text.7f23d98fbc9a"), 24, 377, 150, 34, screen.scanPanel.ToggleRunning)
	p.scanLayer = simpleui.NewSwitch("utilityScanLayer", 170, 275, 164, 28, i18n.Source("text.f96d81ccfe4b"), screen.scanOverlayVisible, uiBodyFontSize)
	p.scanLayer.SetTrackColors(colors.panelAlt, colors.green)
	p.scanLayer.OnChange(func(active bool) {
		screen.scanOverlayVisible = active
		screen.scanPanel.overlayVisible = active
		screen.markSettingsDirty()
	})
	p.controls = append(p.controls, p.scanLayer)
	p.scanRange = button("utilityScanRange", i18n.Source("text.bce5ee71842b"), 182, 377, 152, 34, func() {
		x := screen.centerFrequencyHz
		half := screen.spanHz / 2
		screen.scanPanel.minimumHz, screen.scanPanel.maximumHz = x-half+screen.spanHz/10, x+half-screen.spanHz/10
		screen.markSettingsDirty()
	})
	p.scanMode = button("utilityScanMode", i18n.Source("text.f9b36eae9bda"), 24, 336, 150, 31, func() {
		values := []string{i18n.Source("text.6ea56fae9eac"), i18n.Source("text.85135a165905"), i18n.Source("text.aacf94b7be62")}
		i := slices.Index(values, screen.scanPanel.resume)
		screen.scanPanel.resume = values[(i+1)%len(values)]
		screen.markSettingsDirty()
	})
	p.scanMem = button("utilityScanMemory", i18n.Source("text.0ab7ed3c35b1"), 182, 336, 152, 31, func() {
		screen.scanPanel.centerToMemory = !screen.scanPanel.centerToMemory
		screen.markSettingsDirty()
	})
	p.add = button("utilityMemoryAdd", i18n.Source("text.0692b8193af5"), 182, 433, 152, 31, screen.memoryPanel.openSaveModal)
	addGroup := button("utilityMemoryAddGroup", i18n.Source("text.6b5ffd7d7172"), 24, 433, 150, 31, screen.memoryPanel.openNewGroupModal)
	_ = addGroup
	p.groupPick = simpleui.NewDropdown("utilityMemoryFilter", 24, 470, 220, 32, i18n.Source("text.201f15dab8b3"), screen.memoryPanel.groups, 13)
	p.groupPick.OnChange(func(_ int, group string) {
		screen.memoryPanel.selectedGroup, screen.memoryPanel.selected, screen.memoryPanel.scrollOffset = group, -1, 0
	})
	p.controls = append(p.controls, p.groupPick)
	p.groupEdit = button("utilityMemoryGroupEdit", i18n.Source("text.762217dca4c2"), 250, 470, 84, 32, screen.memoryPanel.openGroupModal)
	p.previous = button("utilityMemoryPrevious", "←", 24, 728, 48, 31, func() { p.moveMemory(-1) })
	p.next = button("utilityMemoryNext", "→", 78, 728, 48, 31, func() { p.moveMemory(1) })
	p.hotkey = button("utilityMemoryHotkey", "TECLA F…", 132, 728, 96, 31, screen.memoryPanel.openHotkeyModal)
	p.hotkey.SetColors(rl.Color{R: 20, G: 85, B: 125, A: 255}, colors.blue, colors.text)
	p.recall = button("utilityMemoryRecall", i18n.Source("text.4089163c4819"), 234, 728, 100, 31, screen.memoryPanel.tuneSelected)
	p.edit = button("utilityMemoryEdit", i18n.Source("text.762217dca4c2"), 24, 765, 150, 31, screen.memoryPanel.editSelection)
	p.remove = button("utilityMemoryDelete", i18n.Source("text.b243b05a8aa4"), 182, 765, 152, 31, screen.memoryPanel.openDeleteModal)
	p.remove.SetColors(actionClearFill, colors.red, colors.text)
	p.record = button("utilityRecord", i18n.Source("text.31d59748e4d1"), 24, 858, 94, 32, screen.recorderPanel.ToggleRecording)
	p.record.SetColors(actionStopFill, colors.red, colors.text)
	p.pause = button("utilityPause", i18n.Source("text.0b03bdac33fa"), 124, 858, 78, 32, screen.recorderPanel.TogglePause)
	p.skip = button("utilitySkip", i18n.Source("text.a7056a455639"), 208, 858, 66, 32, screen.recorderPanel.ToggleSkipSilence)
	p.folder = button("utilityFolder", i18n.Source("text.d313ad93be2d"), 280, 858, 54, 32, screen.recorderPanel.OpenFolder)
	p.format = simpleui.NewSwitch("utilityRecordFormat", 205, 813, 129, 27, "MP3", screen.recorderFormat == recorderFormatMP3, uiBodyFontSize)
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
	p.collapse = simpleui.NewButton("utilityCollapse", 300, 218, 38, 30, "<", 18)
	p.collapse.SetColors(colors.panelAlt, colors.cyan, colors.text)
	p.collapse.OnClick(func() { screen.setUtilitiesCollapsed(!screen.utilitiesCollapsed) })
	p.controls = append(p.controls, p.collapse)
	p.scanState = simpleui.NewIndicator("utilityScanState", 50, 282, 10)
	p.scanState.SetColor(colors.green)
	p.memState = simpleui.NewIndicator("utilityMemoryState", 50, 338, 10)
	p.memState.SetColor(colors.blue)
	p.recState = simpleui.NewIndicator("utilityRecordState", 50, 394, 10)
	p.recState.SetColor(colors.red)
	for _, indicator := range []*simpleui.Indicator{p.scanState, p.memState, p.recState} {
		indicator.SetVisible(false)
		p.controls = append(p.controls, indicator)
	}
	p.expanded = make(map[simpleui.Element]rl.Rectangle, len(p.controls)-1)
	for _, control := range p.controls {
		if control != p.collapse && control != p.scanState && control != p.memState && control != p.recState {
			p.expanded[control] = control.Bounds()
		}
	}
	return p
}

func (p *UtilitiesSidebar) applyCollapsedState(collapsed bool) {
	for _, indicator := range []*simpleui.Indicator{p.scanState, p.memState, p.recState} {
		indicator.SetVisible(false)
	}
	for control, bounds := range p.expanded {
		control.SetBounds(bounds)
		control.SetVisible(!collapsed)
	}
	if collapsed {
		for control, bounds := range map[simpleui.Element]rl.Rectangle{
			p.scan:   {X: 14, Y: 276, Width: 42, Height: 44},
			p.recall: {X: 14, Y: 332, Width: 42, Height: 44},
			p.record: {X: 14, Y: 388, Width: 42, Height: 44},
		} {
			control.SetBounds(bounds)
			control.SetVisible(true)
		}
		p.scanState.SetBounds(rl.Rectangle{X: 48, Y: 279, Width: 10, Height: 10})
		p.memState.SetBounds(rl.Rectangle{X: 48, Y: 335, Width: 10, Height: 10})
		p.recState.SetBounds(rl.Rectangle{X: 48, Y: 391, Width: 10, Height: 10})
		for _, indicator := range []*simpleui.Indicator{p.scanState, p.memState, p.recState} {
			indicator.SetVisible(true)
		}
		p.collapse.SetBounds(rl.Rectangle{X: 14, Y: 222, Width: 42, Height: 40})
		p.collapse.SetLabel(">")
	} else {
		p.collapse.SetBounds(rl.Rectangle{X: 300, Y: 218, Width: 38, Height: 30})
		p.collapse.SetLabel("<")
	}
}

func (p *UtilitiesSidebar) moveMemory(delta int) {
	indices := p.screen.memoryPanel.filteredIndices()
	p.screen.memoryPanel.moveSelection(indices, delta)
}

func (p *UtilitiesSidebar) Draw() {
	if p.screen.utilitiesCollapsed {
		p.drawCollapsed()
		return
	}
	drawPanel(8, 215, utilitiesRight-8, 685)
	rl.DrawRectangleRec(rl.Rectangle{X: 9, Y: 216, Width: utilitiesRight - 10, Height: 39}, colors.panelAlt)
	simpleui.DrawTextStyled(i18n.Source("text.utilitiesTitle"), 24, 227, uiMinimumFontSize, simpleui.FontSemiBold, colors.text)
	p.drawSection(263, 150, i18n.Source("text.31d2326366e4"), colors.cyan)
	p.drawSection(421, 375, i18n.Source("text.59fe2aae29ee"), colors.blue)
	p.drawSection(801, 91, i18n.Source("text.be637709464d"), colors.red)

	scan := p.screen.scanPanel
	if scan.running {
		p.scan.SetLabel(i18n.Source("text.42a572b1399e"))
		p.scan.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.scan.SetLabel(i18n.Source("text.7f23d98fbc9a"))
		p.scan.SetColors(actionStartFill, colors.green, colors.text)
	}
	simpleui.DrawTextStyled(scan.displayStatus(), 24, 296, uiBodyFontSize, simpleui.FontSemiBold, func() rl.Color {
		if scan.running {
			return colors.green
		}
		return colors.muted
	}())
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.9ec780e23eee"), float64(scan.minimumHz)/1e6, float64(scan.maximumHz)/1e6, p.screen.squelchThreshold), 24, 316, uiCaptionFontSize, colors.muted)
	p.scanMode.SetLabel(i18n.Source("text.aa06f8a27805") + scan.resume)
	if scan.centerToMemory {
		p.scanMem.SetLabel(i18n.Source("text.6e2930c883e0"))
	} else {
		p.scanMem.SetLabel(i18n.Source("text.43f36ce02413"))
	}

	memory := p.screen.memoryPanel
	canEditGroup := memory.selectedGroup != "" && memory.selectedGroup != i18n.Source("text.201f15dab8b3") && memory.selectedGroup != i18n.Source("text.445a7d952a48")
	p.groupEdit.SetEnabled(canEditGroup)
	p.hotkey.SetEnabled(memory.selected >= 0 && memory.selected < len(memory.memories))
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
	rl.DrawCircle(28, 835, 5, statusColor)
	simpleui.DrawTextStyled(status+"  "+formatRecordingDuration(state.DurationSeconds), 40, 827, uiBodyFontSize, simpleui.FontMono, colors.text)
	if len(state.RecentFiles) > 0 {
		simpleui.DrawText(trimMemory(filepath.Base(state.RecentFiles[0]), 31), 24, 842, uiCaptionFontSize, colors.muted)
	}
	if pointer := simpleui.MousePosition(); rl.CheckCollisionPointRec(pointer, p.collapse.Bounds()) {
		p.drawTooltip(i18n.Source("text.utilitiesCollapse"), utilitiesRight+8, pointer.Y-16)
	}
}

func (p *UtilitiesSidebar) drawCollapsed() {
	drawPanel(8, 215, utilitiesCollapsedRight-8, 685)
	scan := p.screen.scanPanel
	if scan.running {
		p.scan.SetLabel("STOP")
		p.scan.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.scan.SetLabel("SCAN")
		p.scan.SetColors(colors.panelAlt, colors.cyan, colors.text)
	}
	p.recall.SetLabel("MEM")
	p.recall.SetEnabled(p.screen.memoryPanel.selected >= 0)
	p.recall.SetColors(colors.panelAlt, colors.blue, colors.text)

	state := p.screen.recorder.State()
	if state.Recording {
		p.record.SetLabel("STOP")
		p.record.SetColors(actionStopFill, colors.red, colors.text)
	} else {
		p.record.SetLabel("REC")
		p.record.SetColors(colors.panelAlt, colors.red, colors.text)
	}
	p.scanState.SetActive(scan.running)
	p.memState.SetActive(p.screen.memoryPanel.selected >= 0)
	p.recState.SetActive(state.Recording)
	p.drawCompactTooltip()
}

func (p *UtilitiesSidebar) drawCompactTooltip() {
	pointer := simpleui.MousePosition()
	label := ""
	for _, item := range []struct {
		control simpleui.Element
		text    string
	}{
		{p.collapse, i18n.Source("text.utilitiesExpand")},
		{p.scan, i18n.Source("text.utilitiesScan")},
		{p.recall, i18n.Source("text.utilitiesMemory")},
		{p.record, i18n.Source("text.utilitiesRecord")},
	} {
		if rl.CheckCollisionPointRec(pointer, item.control.Bounds()) {
			label = item.text
			break
		}
	}
	if label == "" {
		return
	}
	p.drawTooltip(label, utilitiesCollapsedRight+8, pointer.Y-16)
}

func (p *UtilitiesSidebar) drawTooltip(label string, x, y float32) {
	textSize := simpleui.MeasureTextStyled(label, uiBodyFontSize, simpleui.FontSemiBold)
	bounds := rl.Rectangle{X: x, Y: y, Width: textSize.X + 24, Height: 32}
	background := colors.panel
	background.A = 248
	rl.DrawRectangleRounded(bounds, .16, 6, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .16, 6, 1, colors.border)
	simpleui.DrawTextStyled(label, bounds.X+12, bounds.Y+(bounds.Height-textSize.Y)/2, uiBodyFontSize, simpleui.FontSemiBold, simpleui.EnsureTextContrast(colors.text, background))
}

func (p *UtilitiesSidebar) drawMemoryTable(memory *MemoryPanel) {
	x, y, w, rowH := float32(24), float32(509), float32(310), float32(29)
	rl.DrawRectangleLinesEx(rl.Rectangle{X: x, Y: y, Width: w, Height: 213}, 1, colors.border)
	simpleui.DrawTextStyled("KEY", x+7, y+8, uiCaptionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(i18n.Source("text.af1acd1a0fa8"), x+45, y+8, uiCaptionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled(i18n.Source("text.1ce0cee583e6"), x+110, y+8, uiCaptionFontSize, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawTextStyled("MHz", x+230, y+8, uiCaptionFontSize, simpleui.FontSemiBold, colors.cyan)
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
		simpleui.DrawText(memoryHotkeyLabel(m.Hotkey), x+7, yy, uiBodyFontSize, colors.blue)
		simpleui.DrawText(trimMemory(memoryGroup(m), 7), x+45, yy, uiBodyFontSize, groupText)
		simpleui.DrawText(trimMemory(m.Name, 13), x+110, yy, uiBodyFontSize, rowText)
		simpleui.DrawText(fmt.Sprintf("%.5f", float64(m.FrequencyHz)/1e6), x+230, yy, uiBodyFontSize, rowText)
	}
}

func (p *UtilitiesSidebar) UpdateInput() {
	if p.screen.utilitiesCollapsed {
		return
	}
	if p.screen.webServer != nil && p.screen.webServer.RemoteActive() {
		return
	}
	m := p.screen.memoryPanel
	mouse := simpleui.MousePosition()
	indices := m.filteredIndices()
	if mouse.X >= 24 && mouse.X <= 334 && mouse.Y >= 541 && mouse.Y < 715 {
		if wheel := rl.GetMouseWheelMove(); wheel != 0 {
			m.scrollOffset -= int(wheel)
			m.scrollOffset = min(max(m.scrollOffset, 0), max(len(indices)-6, 0))
		}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
			row := int((mouse.Y - 541) / 29)
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
	bounds := rl.Rectangle{X: 15, Y: y, Width: utilitiesRight - 22, Height: height}
	rl.DrawRectangleRoundedLinesEx(bounds, .05, 6, 1, colors.border)
	textSize := simpleui.MeasureTextStyled(title, uiMinimumFontSize, simpleui.FontSemiBold)
	plate := rl.Rectangle{X: 22, Y: y - 8, Width: textSize.X + 16, Height: textSize.Y + 4}
	rl.DrawRectangleRec(plate, colors.panel)
	simpleui.DrawTextStyled(title, plate.X+8, plate.Y+2, uiMinimumFontSize, simpleui.FontSemiBold, accent)
}
