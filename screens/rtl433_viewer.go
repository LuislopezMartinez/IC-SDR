package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go-zero/internal/rtl433"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// RunRTL433Viewer runs in a second process because raylib owns one native
// window per process. The capture snapshot is refreshed live by the main app.
func RunRTL433Viewer(snapshotPath string) {
	simpleui.SetMode(1400, 760, simpleui.Fit)
	// The viewer is frequently maximized. Point presentation prevents the
	// complete text canvas from being bilinearly blurred during enlargement.
	simpleui.SetCanvasFilter(rl.FilterPoint)
	simpleui.SetTextScale(1.35)
	simpleui.SetTitle("IC-SDR · Capturas RTL_433")
	simpleui.SetMinimumSize(900, 520)
	v := &rtl433Viewer{path: snapshotPath, selected: -1}
	export := simpleui.NewButton("viewerExport", 1160, 18, 210, 44, "EXPORT CSV", 15)
	export.SetColors(colors.green, colors.border, colors.text)
	export.OnClick(func() {
		path, err := ExportRTL433CSV(v.events)
		if err != nil {
			v.feedback = "EXPORT ERROR"
		} else {
			v.feedback = "SAVED: " + filepath.Base(path)
		}
		v.feedbackUntil = time.Now().Add(3 * time.Second)
	})
	simpleui.Add(export)
	simpleui.Run(v.draw)
}

type rtl433Viewer struct {
	path             string
	events           []rtl433.Event
	selected, scroll int
	nextRead         time.Time
	feedback         string
	feedbackUntil    time.Time
}

func (v *rtl433Viewer) read() {
	if time.Now().Before(v.nextRead) {
		return
	}
	v.nextRead = time.Now().Add(250 * time.Millisecond)
	data, err := os.ReadFile(v.path)
	if err == nil {
		var events []rtl433.Event
		if json.Unmarshal(data, &events) == nil {
			v.events = events
			if v.selected >= len(events) {
				v.selected = len(events) - 1
			}
		}
	}
}
func (v *rtl433Viewer) draw() {
	v.read()
	if controlCopyPressed() && v.selected >= 0 && v.selected < len(v.events) {
		rl.SetClipboardText(rtl433ClipboardText(v.events[v.selected]))
		v.feedback = "FULL FRAME COPIED"
		v.feedbackUntil = time.Now().Add(2 * time.Second)
	}
	rl.DrawRectangle(0, 0, 1400, 760, colors.background)
	simpleui.DrawTextStyled("CAPTURAS RTL_433", 28, 24, 22, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%d devices · real-time updates", len(v.events)), 28, 54, 13, colors.muted)
	if time.Now().Before(v.feedbackUntil) {
		simpleui.DrawTextStyled(v.feedback, 850, 34, 13, simpleui.FontSemiBold, colors.green)
	}
	columns := []struct {
		x     float32
		title string
	}{{28, "DATE / TIME"}, {195, "FREQ. MHz"}, {300, "PROTOCOL"}, {405, "MODEL"}, {585, "TYPE"}, {705, "ID"}, {820, "CHAN"}, {915, "MOD"}, {980, "RSSI"}, {1050, "SNR"}, {1120, "DATA"}}
	rl.DrawRectangle(20, 82, 1360, 34, colors.panelAlt)
	for _, c := range columns {
		simpleui.DrawTextStyled(c.title, c.x, 89, 14, simpleui.FontSemiBold, colors.cyan)
	}
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 {
		v.scroll = min(max(v.scroll-int(wheel)*3, 0), max(len(v.events)-20, 0))
	}
	if rl.IsKeyPressed(rl.KeyDown) {
		v.selected = min(v.selected+1, len(v.events)-1)
		if v.selected >= v.scroll+20 {
			v.scroll = v.selected - 19
		}
	}
	if rl.IsKeyPressed(rl.KeyUp) {
		v.selected = max(v.selected-1, 0)
		if v.selected < v.scroll {
			v.scroll = v.selected
		}
	}
	mouse := simpleui.MousePosition()
	for row := 0; row < 20 && v.scroll+row < len(v.events); row++ {
		index := v.scroll + row
		e := v.events[index]
		y := 120 + float32(row)*24
		if index == v.selected {
			rl.DrawRectangle(20, int32(y-3), 1360, 23, rl.Color{R: 25, G: 83, B: 116, A: 230})
		} else if row%2 == 0 {
			rl.DrawRectangle(20, int32(y-3), 1360, 23, colors.panel)
		}
		bounds := rl.Rectangle{X: 20, Y: y - 3, Width: 1360, Height: 23}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mouse, bounds) {
			v.selected = index
		}
		simpleui.DrawTextStyled(e.Received.Format("2006-01-02 15:04:05"), 28, y-1, 13, simpleui.FontMono, colors.text)
		simpleui.DrawText(fmt.Sprintf("%.3f", e.FreqMHz), 195, y-1, 13, colors.text)
		simpleui.DrawText(fmt.Sprint(e.Protocol), 300, y-1, 13, colors.text)
		simpleui.DrawText(short(e.Model, 18), 405, y-1, 13, colors.text)
		simpleui.DrawText(short(e.Type, 11), 585, y-1, 13, colors.text)
		simpleui.DrawText(short(e.ID, 10), 705, y-1, 13, colors.text)
		simpleui.DrawText(short(e.Channel, 6), 820, y-1, 13, colors.text)
		simpleui.DrawText(e.Mod, 915, y-1, 13, colors.text)
		simpleui.DrawText(fmt.Sprintf("%.2f", e.RSSI), 980, y-1, 13, colors.text)
		simpleui.DrawText(fmt.Sprintf("%.2f", e.SNR), 1050, y-1, 13, colors.text)
		simpleui.DrawText(short(e.Summary, 27), 1120, y-1, 13, colors.text)
	}
	drawPanel(20, 610, 1360, 125)
	simpleui.DrawTextStyled("DETALLE JSON", 32, 620, 13, simpleui.FontSemiBold, colors.cyan)
	if v.selected >= 0 && v.selected < len(v.events) {
		drawWrapped(v.events[v.selected].Raw, 32, 647, 1335, 15, colors.text)
	} else {
		simpleui.DrawText("Select a capture to inspect all of its fields.", 32, 650, 13, colors.muted)
	}
}
