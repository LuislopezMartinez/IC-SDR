package screens

import (
	"fmt"
	"strings"
	"time"

	"go-zero/internal/dmr"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type dmrEvent struct {
	at, state, slot, detail string
	color                   rl.Color
}

type DMRPanel struct {
	screen                        *MainScreen
	controls                      []simpleui.Element
	autoCenter                    *simpleui.Switch
	auto, ts1, ts2, resync, clear *simpleui.Button
	events                        []dmrEvent
	lastState, lastTS1, lastTS2   string
}

func NewDMRPanel(screen *MainScreen) *DMRPanel {
	p := &DMRPanel{screen: screen}
	p.autoCenter = simpleui.NewSwitch("dmrAutoCenter", 1280, 655, 260, 28, "AUTOMATIC CORRECTION", screen.dmrAutoCenter, uiMinimumFontSize)
	p.autoCenter.SetTrackColors(rl.Color{R: 51, G: 61, B: 70, A: 255}, colors.green)
	p.autoCenter.OnChange(func(value bool) {
		screen.dmrAutoCenter = value
		if screen.receiver != nil {
			screen.receiver.SetDMRAutoCenter(value)
		}
		screen.markSettingsDirty()
	})
	p.auto = p.button("dmrAuto", 1280, 699, 82, 42, "AUTO", colors.blue)
	p.ts1 = p.button("dmrTS1", 1370, 699, 80, 42, "TS1", colors.panelAlt)
	p.ts2 = p.button("dmrTS2", 1458, 699, 80, 42, "TS2", colors.panelAlt)
	p.resync = p.button("dmrResync", 1280, 754, 154, 48, "RESYNC", colors.orange)
	p.resync.SetColors(colors.orange, colors.border, colors.background)
	p.clear = p.button("dmrClear", 1442, 754, 96, 48, "CLEAR", colors.panelAlt)
	p.clear.SetColors(actionClearFill, colors.red, colors.text)
	p.auto.OnClick(func() { p.selectSlot("AUTO") })
	p.ts1.OnClick(func() { p.selectSlot("TS1") })
	p.ts2.OnClick(func() { p.selectSlot("TS2") })
	p.resync.OnClick(func() {
		if screen.receiver != nil && screen.receiver.ResyncDMR() {
			p.add("SYSTEM", "--", "Manual resync requested")
		}
	})
	p.clear.OnClick(func() { p.events = nil })
	p.controls = []simpleui.Element{p.autoCenter, p.auto, p.ts1, p.ts2, p.resync, p.clear}
	p.SetVisible(false)
	return p
}

func (p *DMRPanel) button(id string, x, y, w, h float32, label string, color rl.Color) *simpleui.Button {
	b := simpleui.NewButton(id, x, y, w, h, label, uiControlFontSize)
	b.SetColors(color, colors.border, colors.text)
	return b
}

func (p *DMRPanel) selectSlot(slot string) {
	p.screen.dmrAudioSlot = slot
	if p.screen.receiver != nil {
		p.screen.receiver.SetDMRAudioSlot(slot)
	}
	p.add("AUDIO", slot, "Selected audio slot")
	p.screen.markSettingsDirty()
	p.styleSlots()
}

func (p *DMRPanel) styleSlots() {
	for slot, button := range map[string]*simpleui.Button{"AUTO": p.auto, "TS1": p.ts1, "TS2": p.ts2} {
		background := colors.panelAlt
		if slot == p.screen.dmrAudioSlot {
			background = colors.blue
		}
		button.SetColors(background, colors.cyan, colors.text)
	}
}

func (p *DMRPanel) SetVisible(visible bool) {
	for _, control := range p.controls {
		control.SetVisible(visible)
	}
	if visible {
		p.styleSlots()
	}
}

func (p *DMRPanel) add(state, slot, detail string) {
	color := colors.cyan
	if state == "VOICE" {
		color = colors.green
	} else if state == "ERROR" {
		color = colors.red
	} else if state == "SEARCH" {
		color = colors.orange
	}
	p.events = append(p.events, dmrEvent{time.Now().Format("15:04:05"), state, slot, detail, color})
	if len(p.events) > 80 {
		p.events = p.events[len(p.events)-80:]
	}
}

func (p *DMRPanel) capture(status dmr.Status) {
	if status.State != p.lastState {
		p.lastState = status.State
		p.add(status.State, "--", status.Detail)
	}
	if status.Slot1 != p.lastTS1 {
		p.lastTS1 = status.Slot1
		if status.Slot1 != "--" {
			p.add("DATA", "TS1", status.Slot1)
		}
	}
	if status.Slot2 != p.lastTS2 {
		p.lastTS2 = status.Slot2
		if status.Slot2 != "--" {
			p.add("DATA", "TS2", status.Slot2)
		}
	}
}

func (p *DMRPanel) DrawPanel() {
	status := dmr.Status{State: "OFF", Slot1: "--", Slot2: "--", ColorCode: -1}
	if p.screen.receiver != nil {
		status = p.screen.receiver.DMRStatus()
	}
	p.capture(status)
	simpleui.DrawTextStyled("DMR MONITOR", 40, 642, 16, simpleui.FontSemiBold, rl.Color{R: 175, G: 145, B: 245, A: 255})
	drawPanel(38, 665, 210, 142)
	stateColor := colors.orange
	if status.State == "VOICE" {
		stateColor = colors.green
	} else if status.State == "DATA" {
		stateColor = colors.cyan
	} else if status.State == "ERROR" {
		stateColor = colors.red
	}
	rl.DrawCircle(55, 688, 6, stateColor)
	simpleui.DrawTextStyled(status.State, 70, 677, 17, simpleui.FontSemiBold, stateColor)
	cc := "--"
	if status.ColorCode >= 0 {
		cc = fmt.Sprint(status.ColorCode)
	}
	simpleui.DrawText(fmt.Sprintf("COLOR CODE  %s", cc), 52, 711, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("PLL  %s", map[bool]string{true: "LOCKED", false: "NO LOCK"}[status.PLLLocked]), 52, 734, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("SYNC %d   LEVEL %d", status.SyncQuality, status.InputLevel), 52, 757, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf("QUEUE %d/%d   DROP %d   UND %d", status.Queued, status.Capacity, status.Dropped, p.screen.stats.AudioUnderflows), 52, 780, 12, colors.muted)
	p.drawSlot(262, 665, "TIME SLOT 1", status.Slot1, p.screen.dmrAudioSlot == "TS1")
	p.drawSlot(262, 738, "TIME SLOT 2", status.Slot2, p.screen.dmrAudioSlot == "TS2")
	p.drawEvents(530, 665, 730, 142)
	simpleui.DrawText(fmt.Sprintf("ERR %+.0f Hz   AFC %+.0f Hz   %s", status.FrequencyErrorHz, status.AFCCorrectionHz, status.AFCState), 1280, 636, 12, colors.muted)
}

func (p *DMRPanel) drawSlot(x, y float32, title, value string, selected bool) {
	drawPanel(x, y, 250, 69)
	simpleui.DrawText(title, x+12, y+9, 13, colors.cyan)
	if strings.TrimSpace(value) == "" {
		value = "--"
	}
	if len(value) > 30 {
		value = value[:27] + "..."
	}
	simpleui.DrawText(value, x+12, y+34, 14, colors.text)
	if selected {
		simpleui.DrawText("SELECTED AUDIO", x+126, y+9, 12, colors.green)
	}
}

func (p *DMRPanel) drawEvents(x, y, w, h float32) {
	drawPanel(x, y, w, h)
	simpleui.DrawTextStyled("DMR ACTIVITY", x+12, y+8, 14, simpleui.FontSemiBold, colors.cyan)
	start := max(0, len(p.events)-5)
	for row, event := range p.events[start:] {
		ry := y + 35 + float32(row)*20
		if row%2 == 0 {
			rl.DrawRectangle(int32(x+6), int32(ry-2), int32(w-12), 19, colors.panelAlt)
		}
		simpleui.DrawTextStyled(event.at, x+12, ry, 12, simpleui.FontMono, colors.muted)
		simpleui.DrawText(event.state, x+82, ry, 12, event.color)
		simpleui.DrawText(event.slot, x+155, ry, 12, colors.text)
		detail := event.detail
		if len(detail) > 64 {
			detail = detail[:61] + "..."
		}
		simpleui.DrawText(detail, x+205, ry, 12, colors.text)
	}
}
