package screens

import (
	"fmt"
	"strings"

	"go-zero/internal/i18n"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const dmrHistoryRows = 15

var (
	dmrHistoryModal = rl.Rectangle{X: 90, Y: 70, Width: 1420, Height: 760}
	dmrHistoryTable = rl.Rectangle{X: 115, Y: 185, Width: 1370, Height: 480}
)

func (p *DMRPanel) openHistory() {
	p.historyOpen = true
	p.historyLive = true
	p.historyFilter = 0
	p.historySelected = -1
	p.historySeen = len(p.events)
	p.syncHistoryOffset()
}

func (p *DMRPanel) OverlayOpen() bool          { return p.historyOpen }
func (p *DMRPanel) Update(simpleui.Input) bool { return false }
func (p *DMRPanel) Draw()                      {}

func (p *DMRPanel) filteredHistoryIndices() []int {
	indices := make([]int, 0, len(p.events))
	for index, event := range p.events {
		if p.historyFilter == 1 && event.slot != "TS1" {
			continue
		}
		if p.historyFilter == 2 && event.slot != "TS2" {
			continue
		}
		indices = append(indices, index)
	}
	return indices
}

func (p *DMRPanel) syncHistoryOffset() {
	indices := p.filteredHistoryIndices()
	p.historyOffset = max(0, len(indices)-dmrHistoryRows)
	p.historySeen = len(p.events)
}

func dmrHistoryActionBounds(action int) rl.Rectangle {
	switch action {
	case 1:
		return rl.Rectangle{X: 1360, Y: 91, Width: 120, Height: 36}
	case 2:
		return rl.Rectangle{X: 115, Y: 137, Width: 120, Height: 36}
	case 3:
		return rl.Rectangle{X: 250, Y: 137, Width: 100, Height: 36}
	case 4:
		return rl.Rectangle{X: 360, Y: 137, Width: 90, Height: 36}
	case 5:
		return rl.Rectangle{X: 460, Y: 137, Width: 90, Height: 36}
	case 6:
		return rl.Rectangle{X: 1120, Y: 770, Width: 150, Height: 38}
	case 7:
		return rl.Rectangle{X: 1280, Y: 770, Width: 200, Height: 38}
	}
	return rl.Rectangle{}
}

func (p *DMRPanel) UpdateOverlay(input simpleui.Input) bool {
	if !p.historyOpen {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		p.historyOpen = false
		return true
	}
	indices := p.filteredHistoryIndices()
	maxOffset := max(0, len(indices)-dmrHistoryRows)
	if input.Over(dmrHistoryTable) && input.Wheel != 0 {
		p.historyLive = false
		p.historyOffset = min(max(p.historyOffset-int(input.Wheel)*3, 0), maxOffset)
	}
	if controlCopyPressed() && p.historySelected >= 0 && p.historySelected < len(p.events) {
		rl.SetClipboardText(dmrEventClipboardText(p.events[p.historySelected]))
	}
	if input.Pressed {
		p.historyPressed = 0
		for action := 1; action <= 7; action++ {
			if input.Over(dmrHistoryActionBounds(action)) {
				p.historyPressed = action
				return true
			}
		}
	}
	if input.Released {
		pressed := p.historyPressed
		p.historyPressed = 0
		if pressed != 0 && input.Over(dmrHistoryActionBounds(pressed)) {
			switch pressed {
			case 1:
				p.historyOpen = false
			case 2:
				p.historyLive = !p.historyLive
				if p.historyLive {
					p.syncHistoryOffset()
				}
			case 3, 4, 5:
				p.historyFilter = pressed - 3
				p.historyLive = true
				p.historySelected = -1
				p.syncHistoryOffset()
			case 6:
				if p.historySelected >= 0 && p.historySelected < len(p.events) {
					rl.SetClipboardText(dmrEventClipboardText(p.events[p.historySelected]))
				}
			case 7:
				p.clearEvents()
			}
			return true
		}
		if input.Over(dmrHistoryTable) && input.Pointer.Y >= dmrHistoryTable.Y+30 {
			row := int((input.Pointer.Y - (dmrHistoryTable.Y + 30)) / 30)
			position := p.historyOffset + row
			if row >= 0 && row < dmrHistoryRows && position >= 0 && position < len(indices) {
				p.historySelected = indices[position]
				p.historyLive = false
			}
		}
	}
	if p.historyLive {
		p.syncHistoryOffset()
	} else {
		p.historyOffset = min(max(p.historyOffset, 0), maxOffset)
	}
	return true
}

func dmrEventClipboardText(event dmrEvent) string {
	return strings.TrimSpace(fmt.Sprintf("%s\t%s\t%s\t%s", event.at, event.state, event.slot, event.detail))
}

func (p *DMRPanel) DrawOverlay() {
	if !p.historyOpen {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{R: 0, G: 0, B: 0, A: 210})
	rl.DrawRectangleRounded(dmrHistoryModal, .025, 8, colors.panel)
	rl.DrawRectangleRoundedLinesEx(dmrHistoryModal, .025, 8, 2, colors.cyan)
	simpleui.DrawTextStyled(i18n.Source("text.dmrHistoryTitle"), 115, 92, 21, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.dmrStoredEvents"), len(p.events)), 115, 117, 12, colors.muted)

	actions := []struct {
		id     int
		label  string
		active bool
		color  rl.Color
	}{
		{1, i18n.Source("text.dmrClose"), false, colors.border},
		{2, map[bool]string{true: i18n.Source("text.dmrPause"), false: i18n.Source("text.dmrLive")}[p.historyLive], p.historyLive, colors.border},
		{3, i18n.Source("text.dmrAll"), p.historyFilter == 0, colors.cyan},
		{4, "TS1", p.historyFilter == 1, colors.cyan},
		{5, "TS2", p.historyFilter == 2, colors.cyan},
		{6, i18n.Source("text.dmrCopy"), false, colors.border},
		{7, i18n.Source("text.dmrClearHistory"), false, colors.red},
	}
	for _, action := range actions {
		accent := action.color
		if action.active {
			accent = colors.green
		}
		drawModalAction(dmrHistoryActionBounds(action.id), action.label, accent, p.historyPressed == action.id)
	}

	rl.DrawRectangleRec(dmrHistoryTable, colors.background)
	rl.DrawRectangleLinesEx(dmrHistoryTable, 1, colors.border)
	rl.DrawRectangle(int32(dmrHistoryTable.X), int32(dmrHistoryTable.Y), int32(dmrHistoryTable.Width), 30, colors.panelAlt)
	simpleui.DrawText("HORA", 130, 194, 11, colors.cyan)
	simpleui.DrawText("EVENTO", 280, 194, 11, colors.cyan)
	simpleui.DrawText("SLOT", 450, 194, 11, colors.cyan)
	simpleui.DrawText("DETALLE", 550, 194, 11, colors.cyan)
	indices := p.filteredHistoryIndices()
	selection, _ := tableSelectionColors()
	for row := 0; row < dmrHistoryRows && p.historyOffset+row < len(indices); row++ {
		index := indices[p.historyOffset+row]
		event := p.events[index]
		y := float32(215 + row*30)
		if index == p.historySelected {
			rl.DrawRectangle(116, int32(y), 1368, 29, selection)
		} else if row%2 == 0 {
			rl.DrawRectangle(116, int32(y), 1368, 29, colors.panelAlt)
		}
		simpleui.DrawTextStyled(event.at, 130, y+7, 12, simpleui.FontMono, colors.muted)
		simpleui.DrawText(event.state, 280, y+7, 12, event.color)
		simpleui.DrawText(event.slot, 450, y+7, 12, colors.text)
		simpleui.DrawText(trimMemory(event.detail, 105), 550, y+7, 12, colors.text)
	}

	detail := i18n.Source("text.dmrSelectEvent")
	if p.historySelected >= 0 && p.historySelected < len(p.events) {
		detail = trimMemory(dmrEventClipboardText(p.events[p.historySelected]), 240)
	}
	drawPanel(115, 680, 1370, 70)
	simpleui.DrawTextStyled(i18n.Source("text.dmrDetail"), 130, 691, 11, simpleui.FontSemiBold, colors.cyan)
	drawWrapped(detail, 130, 713, 1335, 14, colors.text)

	newEvents := max(0, len(p.events)-p.historySeen)
	state := i18n.Source("text.dmrFollowing")
	if !p.historyLive {
		state = fmt.Sprintf(i18n.Source("text.dmrPausedNew"), newEvents)
	}
	simpleui.DrawText(state, 115, 782, 12, map[bool]rl.Color{true: colors.green, false: colors.orange}[p.historyLive])
}
