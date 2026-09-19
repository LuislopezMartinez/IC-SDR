package screens

import (
	"encoding/json"
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/i18n"
	"go-zero/internal/resources"
	"go-zero/internal/tetra"
	"go-zero/simpleui"
	"math"
	"os"
	"path/filepath"
	"time"
)

func (v *tetraViewer) events(signaling bool) []tetra.Message {
	out := []tetra.Message{}
	for _, m := range v.snapshot.Messages {
		if signaling != m.SDS {
			out = append(out, m)
		}
	}
	return out
}
func (v *tetraViewer) drawEvents(signaling bool) {
	events := v.events(signaling)
	m := simpleui.MousePosition()
	v.eventOffset = min(max(v.eventOffset, 0), max(0, len(events)-9))
	if rl.CheckCollisionPointRec(m, rl.Rectangle{X: 38, Y: 206, Width: 1325, Height: 305}) {
		v.eventOffset = min(max(v.eventOffset-int(rl.GetMouseWheelMove()), 0), max(0, len(events)-9))
	}
	title := i18n.Source("text.bb40197c6706")
	if !signaling {
		title = i18n.Source("text.930589e3a4d9")
	}
	simpleui.DrawTextStyled(title, 48, 167, 18, simpleui.FontSemiBold, colors.cyan)
	if distanceButton(i18n.Source("text.535ad08d8b0d"), rl.Rectangle{X: 1140, Y: 164, Width: 215, Height: 35}) {
		data, err := json.MarshalIndent(events, "", "  ")
		if err == nil {
			path := resources.WritablePath("exports", "tetra", fmt.Sprintf("events-%s.json", time.Now().Format("20060102-150405.000")))
			err = os.MkdirAll(filepath.Dir(path), 0755)
			if err == nil {
				err = os.WriteFile(path, data, 0644)
			}
		}
		if err != nil {
			v.exportStatus = i18n.Source("text.292ea49f103c")
		} else {
			v.exportStatus = i18n.Source("text.27d2334b9f25")
		}
	}
	columns := []struct {
		x float32
		s string
	}{{48, i18n.Source("text.e7563517a678")}, {190, i18n.Source("text.812b5403c47a")}, {355, i18n.Source("text.b5e4bb23195e")}, {505, i18n.Source("text.469580027ba3")}, {670, i18n.Source("text.9f882da226a8")}, {785, i18n.Source("text.f8a5e4dd4992")}, {920, i18n.Source("text.a9474bdd7d25")}, {1100, i18n.Source("text.c06d3a82731b")}}
	for _, c := range columns {
		simpleui.DrawTextStyled(c.s, c.x, 209, 11, simpleui.FontSemiBold, colors.cyan)
	}
	for i := v.eventOffset; i < min(len(events), v.eventOffset+9); i++ {
		e := events[i]
		y := float32(238 + (i-v.eventOffset)*30)
		b := rl.Rectangle{X: 38, Y: y - 3, Width: 1325, Height: 28}
		fill := colors.panel
		if v.selectedEvent != nil && v.selectedEvent.Time.Equal(e.Time) {
			fill = colors.blue
		}
		rl.DrawRectangleRec(b, fill)
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(m, b) {
			copy := e
			v.selectedEvent = &copy
			v.detailOffset = 0
		}
		carrier := "—"
		if e.Carrier != nil {
			carrier = fmt.Sprint(*e.Carrier)
		}
		call := "—"
		if e.CallID > 0 {
			call = fmt.Sprint(e.CallID)
		}
		texts := []string{viewerTime(e.Time), e.Kind, viewerSSI(e.AddressSSI), viewerSSI(e.PartySSI), call, carrier, fmt.Sprintf("%d / 0x%X", e.Slot, e.AssignedSlots), sondeClip(e.Text, 24)}
		for j, t := range texts {
			clip := rl.Rectangle{X: columns[j].x, Y: y - 2, Width: 125, Height: 28}
			if j == 0 {
				clip.Width = 135
			}
			if j == 1 {
				clip.Width = 155
			}
			if j == 7 {
				clip.Width = 255
			}
			rl.BeginScissorMode(int32(clip.X), int32(clip.Y), int32(clip.Width), int32(clip.Height))
			simpleui.DrawTextStyled(t, clip.X, y, 12, simpleui.FontMono, colors.text)
			rl.EndScissorMode()
		}
	}
	if v.selectedEvent == nil && len(events) > 0 {
		copy := events[0]
		v.selectedEvent = &copy
	}
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.b6a0fa6b7ce3"), len(events)), 48, 515, 12, colors.muted)
	drawPanel(38, 545, 1325, 176)
	if v.selectedEvent != nil {
		lines := tetraDetailLines(v.selectedEvent.Detail(), 1290)
		if rl.CheckCollisionPointRec(m, rl.Rectangle{X: 38, Y: 545, Width: 1325, Height: 176}) {
			v.detailOffset = min(max(v.detailOffset-int(rl.GetMouseWheelMove()), 0), max(0, len(lines)-6))
		}
		v.detailOffset = min(v.detailOffset, max(0, len(lines)-6))
		for i := v.detailOffset; i < min(len(lines), v.detailOffset+6); i++ {
			simpleui.DrawTextStyled(lines[i], 50, 555+float32(i-v.detailOffset)*25, 13, simpleui.FontMono, colors.text)
		}
	}
	if v.exportStatus != "" {
		simpleui.DrawText(v.exportStatus, 48, 727, 11, colors.green)
	}
}
func tetraDetailLines(text string, width float32) []string {
	out := []string{}
	line := ""
	for _, r := range i18n.Display(text) {
		next := line + string(r)
		if simpleui.MeasureTextStyled(next, 13, simpleui.FontMono).X > width && line != "" {
			out = append(out, line)
			line = string(r)
		} else {
			line = next
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}
func (v *tetraViewer) drawPositionMap() {
	b := rl.Rectangle{X: 48, Y: 220, Width: 900, Height: 480}
	positions := v.snapshot.Positions
	if len(positions) > 0 && !v.gpsFitted {
		lo, hi, lol, hil := 90., -90., 180., -180.
		for _, p := range positions {
			lo = math.Min(lo, p.Latitude)
			hi = math.Max(hi, p.Latitude)
			lol = math.Min(lol, p.Longitude)
			hil = math.Max(hil, p.Longitude)
		}
		v.gpsLat, v.gpsLon = (lo+hi)/2, (lol+hil)/2
		v.gpsSpan = math.Max(.1, math.Max((hil-lol)*1.6, (hi-lo)*2.6))
		v.gpsFitted = true
	}
	if v.gpsSpan == 0 {
		v.gpsLat, v.gpsLon, v.gpsSpan = 40.4, -3.7, 14
	}
	m := simpleui.MousePosition()
	if rl.CheckCollisionPointRec(m, b) {
		if w := rl.GetMouseWheelMove(); w != 0 {
			v.gpsLat, v.gpsLon, v.gpsSpan = zoomMap(v.gpsLat, v.gpsLon, v.gpsSpan, m, b, w)
		}
		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			v.gpsDragging = true
			v.gpsLast = m
		}
	}
	if v.gpsDragging && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		v.gpsLat, v.gpsLon = panMap(v.gpsLat, v.gpsLon, v.gpsSpan, rl.Vector2Subtract(m, v.gpsLast), b)
		v.gpsLast = m
	}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		v.gpsDragging = false
	}
	drawGeoMap(v.gpsLat, v.gpsLon, v.gpsSpan, b)
	rl.BeginScissorMode(int32(b.X), int32(b.Y), int32(b.Width), int32(b.Height))
	for _, p := range positions {
		q := projectMercator(v.gpsLat, v.gpsLon, v.gpsSpan, p.Latitude, p.Longitude, b)
		rl.DrawCircleV(q, 8, mapCalloutEdge)
		rl.DrawCircleV(q, 5, colors.cyan)
		drawMapCallout(fmt.Sprint(p.SSI), q.X+10, q.Y-8)
	}
	rl.EndScissorMode()
	if distanceButton(i18n.Source("text.ee139409c0cb"), rl.Rectangle{X: 680, Y: 171, Width: 268, Height: 34}) {
		v.gpsFitted = false
	}
	simpleui.DrawText(i18n.Source("text.6ec2a8e7cdac"), 48, 705, 9, colors.muted)
}
