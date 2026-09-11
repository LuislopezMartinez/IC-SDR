package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/aprs"
	"go-zero/simpleui"
)

func RunAPRSViewer(path string) {
	simpleui.SetMode(1450, 760, simpleui.Fit)
	simpleui.SetCanvasFilter(rl.FilterPoint)
	simpleui.SetTextScale(1.35)
	simpleui.SetTitle("IC-SDR · Capturas APRS")
	simpleui.SetMinimumSize(920, 520)
	v := &aprsViewer{path: path, selected: -1}
	export := simpleui.NewButton("aprsViewerExport", 1210, 18, 210, 44, "EXPORT CSV", 15)
	export.SetColors(colors.green, colors.border, colors.text)
	export.OnClick(func() {
		saved, err := exportAPRSCSV(v.packets)
		if err != nil {
			v.feedback = "EXPORT ERROR"
		} else {
			v.feedback = "SAVED: " + filepath.Base(saved)
		}
		v.until = time.Now().Add(3 * time.Second)
	})
	simpleui.Add(export)
	simpleui.Run(v.draw)
}

type aprsViewer struct {
	path             string
	packets          []aprs.Packet
	scroll, selected int
	next             time.Time
	feedback         string
	until            time.Time
}

func (v *aprsViewer) read() {
	if time.Now().Before(v.next) {
		return
	}
	v.next = time.Now().Add(250 * time.Millisecond)
	data, err := os.ReadFile(v.path)
	if err == nil {
		var packets []aprs.Packet
		if json.Unmarshal(data, &packets) == nil {
			v.packets = packets
			if v.selected >= len(packets) {
				v.selected = len(packets) - 1
			}
		}
	}
}
func (v *aprsViewer) draw() {
	v.read()
	rl.DrawRectangle(0, 0, 1450, 760, colors.background)
	simpleui.DrawTextStyled("APRS CAPTURES", 28, 24, 22, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%d packets · table updated in real time", len(v.packets)), 28, 54, 13, colors.muted)
	if time.Now().Before(v.until) {
		simpleui.DrawTextStyled(v.feedback, 910, 35, 13, simpleui.FontSemiBold, colors.green)
	}
	headers := []struct {
		x float32
		s string
	}{{28, "DATE / TIME"}, {195, "CALLSIGN"}, {315, "TYPE"}, {420, "DESTINATION"}, {530, "PATH"}, {750, "POSITION"}, {950, "LEVEL"}, {1020, "INFORMATION"}}
	rl.DrawRectangle(20, 82, 1410, 34, colors.panelAlt)
	for _, h := range headers {
		simpleui.DrawTextStyled(h.s, h.x, 91, 13, simpleui.FontSemiBold, colors.cyan)
	}
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 {
		v.scroll = min(max(v.scroll-int(wheel)*3, 0), max(len(v.packets)-19, 0))
	}
	if rl.IsKeyPressed(rl.KeyDown) {
		v.selected = min(v.selected+1, len(v.packets)-1)
		if v.selected >= v.scroll+19 {
			v.scroll = v.selected - 18
		}
	}
	if rl.IsKeyPressed(rl.KeyUp) {
		v.selected = max(v.selected-1, 0)
		if v.selected < v.scroll {
			v.scroll = v.selected
		}
	}
	mouse := simpleui.MousePosition()
	for row := 0; row < 19 && v.scroll+row < len(v.packets); row++ {
		index := v.scroll + row
		p := v.packets[index]
		y := 121 + float32(row)*25
		if index == v.selected {
			rl.DrawRectangle(20, int32(y-3), 1410, 24, rl.Color{R: 25, G: 83, B: 116, A: 230})
		} else if row%2 == 0 {
			rl.DrawRectangle(20, int32(y-3), 1410, 24, colors.panel)
		}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mouse, rl.Rectangle{X: 20, Y: y - 3, Width: 1410, Height: 24}) {
			v.selected = index
		}
		simpleui.DrawTextStyled(p.Received.Format("2006-01-02 15:04:05"), 28, y, 12, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(short(p.Source, 13), 195, y, 12, simpleui.FontSemiBold, colors.cyan)
		simpleui.DrawText(p.Type, 315, y, 12, colors.green)
		simpleui.DrawText(short(p.Destination, 12), 420, y, 12, colors.text)
		route := p.Path
		if route == "" {
			route = "DIRECTO"
		}
		simpleui.DrawText(short(route, 24), 530, y, 12, colors.text)
		simpleui.DrawText(short(p.Coordinates, 22), 750, y, 12, colors.text)
		simpleui.DrawText(levelText(p.ReceiveLevel), 950, y, 12, colors.text)
		simpleui.DrawText(short(p.Summary, 49), 1020, y, 12, colors.text)
	}
	drawPanel(20, 610, 1410, 125)
	simpleui.DrawTextStyled("AX.25 / APRS FRAME", 32, 620, 13, simpleui.FontSemiBold, colors.cyan)
	if v.selected >= 0 && v.selected < len(v.packets) {
		p := v.packets[v.selected]
		drawWrapped(p.Raw, 32, 646, 1375, 13, colors.text)
		simpleui.DrawText(fmt.Sprintf("Symbol %s · Locator %s · Course %s · Speed %s · Altitude %s", p.Symbol, p.Locator, p.Course, p.Speed, p.Altitude), 32, 700, 12, colors.muted)
	} else {
		simpleui.DrawText("Select a packet to inspect the full frame.", 32, 650, 13, colors.muted)
	}
}
