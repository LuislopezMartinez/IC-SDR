package screens

import (
	"go-zero/internal/i18n"

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
	p.autoCenter = simpleui.NewSwitch("dmrAutoCenter", 1280, 655, 260, 28, i18n.Source("text.a4c290b13a9d"), screen.dmrAutoCenter, uiMinimumFontSize)
	p.autoCenter.SetTrackColors(rl.Color{R: 51, G: 61, B: 70, A: 255}, colors.green)
	p.autoCenter.OnChange(func(value bool) {
		screen.dmrAutoCenter = value
		if screen.receiver != nil {
			screen.receiver.SetDMRAutoCenter(value)
		}
		screen.markSettingsDirty()
	})
	p.auto = p.button("dmrAuto", 1280, 699, 82, 42, i18n.Source("text.6ea56fae9eac"), colors.blue)
	p.ts1 = p.button("dmrTS1", 1370, 699, 80, 42, "TS1", colors.panelAlt)
	p.ts2 = p.button("dmrTS2", 1458, 699, 80, 42, "TS2", colors.panelAlt)
	p.resync = p.button("dmrResync", 1280, 754, 154, 48, i18n.Source("text.688ca3416da3"), colors.orange)
	p.resync.SetColors(colors.orange, colors.border, colors.background)
	p.clear = p.button("dmrClear", 1442, 754, 96, 48, i18n.Source("text.2aded7edd569"), colors.panelAlt)
	p.clear.SetColors(actionClearFill, colors.red, colors.text)
	p.auto.OnClick(func() { p.selectSlot(i18n.Source("text.6ea56fae9eac")) })
	p.ts1.OnClick(func() { p.selectSlot("TS1") })
	p.ts2.OnClick(func() { p.selectSlot("TS2") })
	p.resync.OnClick(func() {
		if screen.receiver != nil && screen.receiver.ResyncDMR() {
			p.add(i18n.Source("text.d621c1a7169f"), "--", i18n.Source("text.5278aa00d5e8"))
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
	p.add(i18n.Source("text.859e89a729c2"), slot, i18n.Source("text.ee1d8d5cb40e"))
	p.screen.markSettingsDirty()
	p.styleSlots()
}

func (p *DMRPanel) styleSlots() {
	for slot, button := range map[string]*simpleui.Button{i18n.Source("text.6ea56fae9eac"): p.auto, "TS1": p.ts1, "TS2": p.ts2} {
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
	if state == i18n.Source("text.a430e6d293d0") {
		color = colors.green
	} else if state == i18n.Source("text.d98ee0e5f939") {
		color = colors.red
	} else if state == i18n.Source("text.56f21695a650") {
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
			p.add(i18n.Source("text.c97c29c7a71b"), "TS1", status.Slot1)
		}
	}
	if status.Slot2 != p.lastTS2 {
		p.lastTS2 = status.Slot2
		if status.Slot2 != "--" {
			p.add(i18n.Source("text.c97c29c7a71b"), "TS2", status.Slot2)
		}
	}
}

func (p *DMRPanel) DrawPanel() {
	status := dmr.Status{State: i18n.Source("text.38cca6bea010"), Slot1: "--", Slot2: "--", ColorCode: -1}
	if p.screen.receiver != nil {
		status = p.screen.receiver.DMRStatus()
	}
	p.capture(status)
	simpleui.DrawTextStyled(i18n.Source("text.ae7216ed6f02"), 40, 642, 16, simpleui.FontSemiBold, rl.Color{R: 175, G: 145, B: 245, A: 255})
	drawPanel(38, 665, 210, 142)
	stateColor := colors.orange
	if status.State == i18n.Source("text.a430e6d293d0") {
		stateColor = colors.green
	} else if status.State == i18n.Source("text.c97c29c7a71b") {
		stateColor = colors.cyan
	} else if status.State == i18n.Source("text.d98ee0e5f939") {
		stateColor = colors.red
	}
	rl.DrawCircle(55, 688, 6, stateColor)
	simpleui.DrawTextStyled(status.State, 70, 677, 17, simpleui.FontSemiBold, stateColor)
	cc := "--"
	if status.ColorCode >= 0 {
		cc = fmt.Sprint(status.ColorCode)
	}
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.369660ea9c3f"), cc), 52, 711, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.33a5b9fa7e99"), map[bool]string{true: i18n.Source("text.52fc523baf24"), false: i18n.Source("text.31313da67e97")}[status.PLLLocked]), 52, 734, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.578917444543"), status.SyncQuality, status.InputLevel), 52, 757, 13, colors.text)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.0f28341d32b3"), status.Queued, status.Capacity, status.Dropped, p.screen.stats.AudioUnderflows), 52, 780, 12, colors.muted)
	p.drawSlot(262, 665, i18n.Source("text.7c7c98979e20"), status.Slot1, p.screen.dmrAudioSlot == "TS1")
	p.drawSlot(262, 738, i18n.Source("text.ce6d4ea6916a"), status.Slot2, p.screen.dmrAudioSlot == "TS2")
	p.drawEvents(530, 665, 730, 142)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.ca1ffacff316"), status.FrequencyErrorHz, status.AFCCorrectionHz, status.AFCState), 1280, 636, 12, colors.muted)
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
		simpleui.DrawText(i18n.Source("text.5b442db27f43"), x+126, y+9, 12, colors.green)
	}
}

func (p *DMRPanel) drawEvents(x, y, w, h float32) {
	drawPanel(x, y, w, h)
	simpleui.DrawTextStyled(i18n.Source("text.8db39183b07c"), x+12, y+8, 14, simpleui.FontSemiBold, colors.cyan)
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
