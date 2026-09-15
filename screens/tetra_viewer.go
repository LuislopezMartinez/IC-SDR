package screens

import (
	"go-zero/internal/i18n"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/tetra"
	"go-zero/simpleui"
)

var tetraTabs = []string{i18n.Source("text.65cbe1e19791"), i18n.Source("text.a0511f3fa0fb"), i18n.Source("text.195f5fd6e9c6"), i18n.Source("text.36d9977c457c"), i18n.Source("text.fe86cd5572c0"), i18n.Source("text.176c7866b945"), i18n.Source("text.b29ec97662f8")}

func RunTETRAViewer(path, settingsPath string) {
	simpleui.SetMode(1400, 780, simpleui.Fit)
	simpleui.SetInitialWindowSize(700, 390)
	// The viewer is commonly kept beside the receiver at a reduced size. Use
	// larger glyphs and smooth canvas reduction so text remains readable near
	// the minimum window dimensions.
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.55)
	simpleui.SetTitle(i18n.Source("text.6f4cc5cbc29c"))
	simpleui.SetMinimumSize(700, 390)
	v := &tetraViewer{path: path, settingsPath: settingsPath}
	simpleui.Run(v.draw)
}

type tetraViewer struct {
	path                  string
	settingsPath          string
	snapshot              tetra.LiveSnapshot
	tab                   int
	nextRead              time.Time
	nextSettingsRead      time.Time
	topmost, topmostKnown bool
}

func (v *tetraViewer) read() {
	if time.Now().Before(v.nextRead) {
		return
	}
	v.nextRead = time.Now().Add(200 * time.Millisecond)
	if data, err := os.ReadFile(v.path); err == nil {
		var s tetra.LiveSnapshot
		if json.Unmarshal(data, &s) == nil {
			v.snapshot = s
		}
	}
}

func (v *tetraViewer) readSettings() {
	if time.Now().Before(v.nextSettingsRead) {
		return
	}
	v.nextSettingsRead = time.Now().Add(200 * time.Millisecond)
	data, err := os.ReadFile(v.settingsPath)
	if err != nil {
		return
	}
	var settings tetraViewerSettings
	if json.Unmarshal(data, &settings) != nil || (v.topmostKnown && settings.Topmost == v.topmost) {
		return
	}
	v.topmost, v.topmostKnown = settings.Topmost, true
	if v.topmost {
		rl.SetWindowState(rl.FlagWindowTopmost)
	} else {
		rl.ClearWindowState(rl.FlagWindowTopmost)
	}
}
func (v *tetraViewer) draw() {
	v.read()
	v.readSettings()
	rl.DrawRectangle(0, 0, 1400, 780, colors.background)
	simpleui.DrawTextStyled(i18n.Source("text.260548a7d2d7"), 28, 22, 24, simpleui.FontSemiBold, colors.cyan)
	state := v.snapshot.Status.State
	if state == "" {
		state = i18n.Source("text.426adab4482d")
	}
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.b5bce57093ca"), float64(v.snapshot.FrequencyHz)/1e6, state, viewerTime(v.snapshot.Updated)), 28, 56, 13, colors.muted)
	mouse := simpleui.MousePosition()
	for i, name := range tetraTabs {
		b := rl.Rectangle{X: 24 + float32(i)*193, Y: 88, Width: 179, Height: 45}
		fill := colors.panelAlt
		if i == v.tab {
			fill = colors.blue
		}
		rl.DrawRectangleRounded(b, .12, 7, fill)
		rl.DrawRectangleRoundedLinesEx(b, .12, 7, 1, colors.border)
		drawCentered(name, b, 14, colors.text)
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mouse, b) {
			v.tab = i
		}
	}
	drawPanel(24, 150, 1352, 600)
	switch v.tab {
	case 0:
		v.drawNetwork()
	case 1:
		v.drawNeighbours()
	case 2:
		v.drawGroups()
	case 3:
		v.drawUsers()
	case 4:
		v.drawMessages()
	case 5:
		v.drawGPS()
	case 6:
		v.drawConsole()
	}
}

func (v *tetraViewer) drawNeighbours() {
	simpleui.DrawTextStyled(i18n.Source("text.791102c24e6b"), 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	if len(v.snapshot.Neighbours) == 0 {
		simpleui.DrawText(i18n.Source("text.4f8abf156b0a"), 48, 235, 14, colors.muted)
		return
	}
	columns := []struct {
		x    float32
		text string
	}{{48, i18n.Source("text.0d3cfaccd1ec")}, {145, i18n.Source("text.1fdfd3b541f0")}, {350, i18n.Source("text.bb7cb16379da")}, {480, i18n.Source("text.7038db1aaa2f")}, {560, i18n.Source("text.87e93a77615b")}, {650, "LA"}, {750, i18n.Source("text.7dcad6823810")}, {850, i18n.Source("text.d83eaf7c2000")}, {990, i18n.Source("text.f5337c8f577c")}, {1090, i18n.Source("text.79d63323a528")}}
	for _, column := range columns {
		simpleui.DrawTextStyled(column.text, column.x, 215, 12, simpleui.FontSemiBold, colors.cyan)
	}
	for i, n := range v.snapshot.Neighbours {
		if i >= 15 {
			break
		}
		y := 247 + float32(i)*29
		if i%2 == 0 {
			rl.DrawRectangle(38, int32(y-3), 1300, 27, colors.panel)
		}
		frequency := i18n.Source("text.fd13f355202c")
		if n.FrequencyHz > 0 {
			frequency = fmt.Sprintf(i18n.Source("text.5c87fd270c6b"), float64(n.FrequencyHz)/1e6)
		}
		syncText, syncColor := "NO", colors.muted
		if n.Synchronized {
			syncText, syncColor = "SÍ", colors.green
		}
		voiceText := "—"
		if n.Voice {
			voiceText = i18n.Source("text.f69d86a86926")
		}
		simpleui.DrawText(fmt.Sprintf("%d", n.CellID), 48, y, 13, colors.text)
		simpleui.DrawTextStyled(frequency, 145, y, 13, simpleui.FontMono, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d", n.Carrier), 350, y, 13, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d", n.MCC), 480, y, 13, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d", n.MNC), 560, y, 13, colors.text)
		simpleui.DrawText(fmt.Sprintf("%d", n.LocationArea), 650, y, 13, colors.text)
		simpleui.DrawText(syncText, 750, y, 13, syncColor)
		simpleui.DrawText(fmt.Sprintf("%d", n.ServiceLevel), 850, y, 13, colors.text)
		simpleui.DrawText(voiceText, 990, y, 13, colors.text)
		simpleui.DrawText(viewerTime(n.LastSeen), 1090, y, 13, colors.text)
		row := rl.Rectangle{X: 38, Y: y - 4, Width: 1300, Height: 28}
		if n.FrequencyHz > 0 && rl.CheckCollisionPointRec(simpleui.MousePosition(), row) {
			rl.DrawRectangleRoundedLinesEx(row, .08, 5, 1, colors.cyan)
			if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
				data, _ := json.Marshal(tetraViewerCommand{TuneHz: n.FrequencyHz})
				_ = os.WriteFile(filepath.Join(filepath.Dir(v.path), "tetra-command.json"), data, 0644)
			}
		}
	}
	simpleui.DrawText(i18n.Source("text.b0e3de64803a"), 48, 704, 12, colors.muted)
}
func (v *tetraViewer) drawGroups() {
	simpleui.DrawTextStyled(i18n.Source("text.0a5a30ad01ba"), 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	if len(v.snapshot.Calls) > 0 {
		for _, column := range []struct {
			x float32
			t string
		}{{48, i18n.Source("text.180fdf7b7d31")}, {180, i18n.Source("text.d2220d5369b9")}, {390, "TS"}, {465, i18n.Source("text.93b5b57d722d")}, {570, i18n.Source("text.f16fe7d4376e")}, {830, i18n.Source("text.f932c4ddaf86")}, {1010, i18n.Source("text.79d0a812e99e")}} {
			simpleui.DrawTextStyled(column.t, column.x, 214, 12, simpleui.FontSemiBold, colors.cyan)
		}
		for i, call := range v.snapshot.Calls {
			if i >= 15 {
				break
			}
			y := 246 + float32(i)*29
			if i%2 == 0 {
				rl.DrawRectangle(38, int32(y-3), 1300, 27, colors.panel)
			}
			stateColor := colors.muted
			activity := i18n.Source("text.a5ce935500e7")
			if call.Active {
				stateColor, activity = colors.green, i18n.Source("text.71ea479e9102")
			}
			cipher := i18n.Source("text.33f29205b18a")
			cipherColor := colors.green
			if call.Encrypted {
				cipher, cipherColor = i18n.Source("text.f932c4ddaf86"), colors.orange
			}
			simpleui.DrawTextStyled(fmt.Sprintf("%d", call.ID), 48, y, 13, simpleui.FontMono, colors.text)
			simpleui.DrawTextStyled(fmt.Sprintf("%08d", call.SSI), 180, y, 13, simpleui.FontMono, colors.text)
			simpleui.DrawText(fmt.Sprintf("TS%d", call.Slot), 390, y, 13, colors.text)
			simpleui.DrawText(fmt.Sprintf("%d", call.UsageMarker), 465, y, 13, colors.text)
			simpleui.DrawText(activity+" · "+call.State, 570, y, 12, stateColor)
			simpleui.DrawText(cipher, 830, y, 12, cipherColor)
			simpleui.DrawText(viewerTime(call.LastSeen), 1010, y, 12, colors.text)
		}
		return
	}
	simpleui.DrawText(i18n.Source("text.62d3d4f30cb3"), 48, 207, 12, colors.muted)
	if len(v.snapshot.Groups) == 0 {
		simpleui.DrawText(i18n.Source("text.d10b4f1fe55b"), 48, 260, 14, colors.muted)
		return
	}
	for i, g := range v.snapshot.Groups {
		if i >= 15 {
			break
		}
		y := 246 + float32(i)*29
		if i%2 == 0 {
			rl.DrawRectangle(38, int32(y-3), 1300, 27, colors.panel)
		}
		simpleui.DrawTextStyled(fmt.Sprintf("%08d", g.ID), 48, y, 14, simpleui.FontMono, colors.text)
		simpleui.DrawText(g.Name, 270, y, 13, colors.text)
		simpleui.DrawText(g.LastEvent, 530, y, 13, colors.cyan)
		simpleui.DrawText(fmt.Sprintf(i18n.Source("text.63f0c0bf904a"), g.Calls), 790, y, 13, colors.text)
		simpleui.DrawText(viewerTime(g.LastSeen), 1020, y, 13, colors.text)
	}
}
func (v *tetraViewer) drawMessages() {
	simpleui.DrawTextStyled(i18n.Source("text.eb7c941c9b26"), 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	if len(v.snapshot.Messages) == 0 {
		simpleui.DrawText(i18n.Source("text.f3e59cc5de24"), 48, 235, 14, colors.muted)
		return
	}
	columns := []struct {
		x    float32
		text string
	}{{48, i18n.Source("text.e7563517a678")}, {150, i18n.Source("text.cd4c8ecf7e4b")}, {315, i18n.Source("text.b5e4bb23195e")}, {425, i18n.Source("text.f6fa48470b25")}, {550, "TS"}, {590, i18n.Source("text.5cf99be8ab9b")}, {755, i18n.Source("text.acf101dfac95")}, {805, i18n.Source("text.fc473ebb1982")}}
	for _, column := range columns {
		simpleui.DrawTextStyled(column.text, column.x, 210, 11, simpleui.FontSemiBold, colors.cyan)
	}
	for i, m := range v.snapshot.Messages {
		if i >= 16 {
			break
		}
		y := 239 + float32(i)*29
		if i%2 == 0 {
			rl.DrawRectangle(38, int32(y-3), 1300, 26, colors.panel)
		}
		kindColor := colors.cyan
		if m.SDS && !m.Recognized {
			kindColor = colors.orange
		}
		protocol := i18n.Source("text.04cd1c188336")
		if m.SDS {
			protocol = m.ProtocolName
			if protocol == "" {
				protocol = fmt.Sprintf(i18n.Source("text.1b9a4bc520d6"), m.SDSProtocol)
			}
			protocol = fmt.Sprintf(i18n.Source("text.800f1919b226"), m.SDSDataType, protocol)
		}
		detail := m.Text
		if m.RawHex != "" && !m.Recognized {
			detail += i18n.Source("text.2425e30b828f") + m.RawHex
		}
		slot := "—"
		if m.Slot > 0 {
			slot = fmt.Sprintf("%d", m.Slot)
		}
		simpleui.DrawText(viewerTime(m.Time), 48, y, 12, colors.muted)
		cipher, cipherColor := "NO", colors.green
		if m.Encrypted {
			cipher, cipherColor = "SÍ", colors.red
		}
		simpleui.DrawTextStyled(sondeClip(m.Kind, 21), 150, y, 12, simpleui.FontSemiBold, kindColor)
		simpleui.DrawTextStyled(viewerSSI(m.AddressSSI), 315, y, 12, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(viewerSSI(m.PartySSI), 425, y, 12, simpleui.FontMono, colors.text)
		simpleui.DrawText(slot, 550, y, 12, colors.text)
		simpleui.DrawText(sondeClip(protocol, 20), 590, y, 12, kindColor)
		simpleui.DrawText(cipher, 755, y, 12, cipherColor)
		simpleui.DrawText(sondeClip(detail, 68), 805, y, 12, colors.text)
	}
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.0703a92da9f7"), len(v.snapshot.Messages)), 48, 716, 12, colors.muted)
}

func viewerSSI(ssi uint32) string {
	if ssi == 0 {
		return "—"
	}
	return fmt.Sprintf("%08d", ssi)
}
func (v *tetraViewer) drawUsers() {
	simpleui.DrawTextStyled(i18n.Source("text.794ff66835e4"), 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	cols := []struct {
		x     float32
		label string
	}{{48, i18n.Source("text.b5e4bb23195e")}, {260, i18n.Source("text.cd4c8ecf7e4b")}, {430, i18n.Source("text.6336c11a4448")}, {590, i18n.Source("text.f932c4ddaf86")}, {760, i18n.Source("text.64eea6ea2ef9")}, {940, i18n.Source("text.96835131c7ed")}}
	for _, c := range cols {
		simpleui.DrawTextStyled(c.label, c.x, 220, 13, simpleui.FontSemiBold, colors.cyan)
	}
	if len(v.snapshot.Users) == 0 {
		simpleui.DrawText(i18n.Source("text.83ba63dfc31d"), 48, 270, 14, colors.muted)
		return
	}
	for i, u := range v.snapshot.Users {
		if i >= 16 {
			break
		}
		y := 254 + float32(i)*28
		if i%2 == 0 {
			rl.DrawRectangle(38, int32(y-3), 1300, 26, colors.panel)
		}
		kind := map[uint8]string{1: i18n.Source("text.b5e4bb23195e"), 3: i18n.Source("text.729992f8dd90"), 4: i18n.Source("text.bb262a5ac6f8"), 5: i18n.Source("text.b4a242fc3adf"), 6: i18n.Source("text.30b0cdddeee0"), 7: i18n.Source("text.ed4d41c10d82")}[u.AddressType]
		enc, color := i18n.Source("text.33f29205b18a"), colors.green
		if u.Encrypted {
			enc, color = i18n.Source("text.f932c4ddaf86"), colors.orange
		}
		simpleui.DrawTextStyled(fmt.Sprintf("%08d", u.SSI), 48, y, 14, simpleui.FontMono, colors.text)
		simpleui.DrawText(kind, 260, y, 13, colors.text)
		simpleui.DrawText(fmt.Sprintf("TS%d", u.Slot), 430, y, 13, colors.text)
		simpleui.DrawText(enc, 590, y, 13, color)
		simpleui.DrawText(fmt.Sprint(u.Seen), 760, y, 13, colors.text)
		simpleui.DrawText(viewerTime(u.LastSeen), 940, y, 13, colors.text)
	}
}
func (v *tetraViewer) drawNetwork() {
	s := v.snapshot.Status
	simpleui.DrawTextStyled(i18n.Source("text.f6d795acddf4"), 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	if !s.System.Valid {
		simpleui.DrawText(i18n.Source("text.5bd1ac212b9d"), 48, 220, 15, colors.muted)
		return
	}
	cards := []struct{ label, value string }{{i18n.Source("text.7038db1aaa2f"), fmt.Sprint(s.System.MCC)}, {i18n.Source("text.87e93a77615b"), fmt.Sprint(s.System.MNC)}, {i18n.Source("text.41081d8e7363"), fmt.Sprint(s.System.ColourCode)}, {i18n.Source("text.6336c11a4448"), fmt.Sprint(s.System.Timeslot)}, {i18n.Source("text.c0fcf1e75f3a"), fmt.Sprint(s.System.Frame)}, {i18n.Source("text.4eb553abebc0"), fmt.Sprint(s.System.Multiframe)}}
	for i, c := range cards {
		x := 48 + float32(i%3)*425
		y := 220 + float32(i/3)*130
		drawPanel(x, y, 390, 100)
		simpleui.DrawText(c.label, x+18, y+15, 12, colors.muted)
		simpleui.DrawTextStyled(c.value, x+18, y+43, 30, simpleui.FontMono, colors.text)
	}
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.deac85b853d6"), s.SyncHits, s.NormalBursts, s.BSCHFailures, s.Quality), 48, 505, 14, colors.green)
	if s.Network.Valid {
		simpleui.DrawText(fmt.Sprintf(i18n.Source("text.70fd46370dd2"), s.Network.MainCarrier, s.Network.FrequencyBand, float64(s.Network.DownlinkHz)/1e6, float64(s.Network.UplinkHz)/1e6, s.Network.LocationArea, s.Network.ServiceDetails), 48, 530, 13, colors.cyan)
	}
	simpleui.DrawTextStyled(i18n.Source("text.9d3a84ffa048"), 48, 565, 14, simpleui.FontSemiBold, colors.cyan)
	maxBursts := uint64(1)
	for _, n := range s.SlotBursts {
		if n > maxBursts {
			maxBursts = n
		}
	}
	for i, n := range s.SlotBursts {
		x := 48 + float32(i)*305
		simpleui.DrawText(fmt.Sprintf(i18n.Source("text.e12f090b8cc4"), i+1, n), x, 600, 13, colors.text)
		w := float32(n) * 250 / float32(maxBursts)
		rl.DrawRectangle(int32(x), 630, int32(w), 18, colors.green)
		rl.DrawRectangleLines(int32(x), 630, 250, 18, colors.border)
	}
}
func (v *tetraViewer) drawEmpty(title string, count int, hint string) {
	simpleui.DrawTextStyled(title, 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.b14c22355248"), count), 48, 212, 14, colors.text)
	simpleui.DrawText(hint, 48, 260, 14, colors.muted)
	simpleui.DrawText(i18n.Source("text.6297edc5ca93"), 48, 292, 13, colors.muted)
}
func (v *tetraViewer) drawGPS() {
	simpleui.DrawTextStyled(i18n.Source("text.e08d65caa6d5"), 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	mapBox := rl.Rectangle{X: 48, Y: 220, Width: 900, Height: 480}
	rl.DrawRectangleRec(mapBox, rl.Color{R: 8, G: 17, B: 24, A: 255})
	rl.DrawRectangleLinesEx(mapBox, 1, colors.border)
	for i := 1; i < 8; i++ {
		x := mapBox.X + mapBox.Width*float32(i)/8
		rl.DrawLine(int32(x), int32(mapBox.Y), int32(x), int32(mapBox.Y+mapBox.Height), colors.grid)
	}
	for i := 1; i < 6; i++ {
		y := mapBox.Y + mapBox.Height*float32(i)/6
		rl.DrawLine(int32(mapBox.X), int32(y), int32(mapBox.X+mapBox.Width), int32(y), colors.grid)
	}
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.d57b21258ad8"), len(v.snapshot.Positions)), 990, 230, 15, colors.text)
	if len(v.snapshot.Positions) == 0 {
		simpleui.DrawText(i18n.Source("text.98de653665fb"), 990, 270, 13, colors.muted)
		return
	}
	for i, p := range v.snapshot.Positions {
		if i >= 10 {
			break
		}
		y := 270 + float32(i)*42
		simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.6bb3cf0cf824"), p.SSI), 990, y, 14, simpleui.FontMono, colors.cyan)
		simpleui.DrawText(fmt.Sprintf(i18n.Source("text.76cbe80026b1"), p.Latitude, p.Longitude, p.SpeedKmh, p.Heading, p.AccuracyM), 990, y+19, 12, colors.text)
	}
}
func (v *tetraViewer) drawConsole() {
	s := v.snapshot.Status
	simpleui.DrawTextStyled(i18n.Source("text.08401e46df11"), 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	lines := []string{fmt.Sprintf(i18n.Source("text.5e1dd5e12944"), s.State), fmt.Sprintf(i18n.Source("text.e005e1039069"), s.InputRate, s.OutputRate), fmt.Sprintf(i18n.Source("text.df413186cf40"), s.LevelDBFS, s.Quality, s.FrequencyErrorHz), fmt.Sprintf(i18n.Source("text.e0c7cc10f878"), s.TimingPhase, s.TimingError), fmt.Sprintf(i18n.Source("text.d59c8264e9a1"), s.BER, s.FER), fmt.Sprintf(i18n.Source("text.1e61c98fc7cd"), s.LastAudioDecision), fmt.Sprintf(i18n.Source("text.4d92d778ba74"), s.AudioRejectedEncrypted, s.AudioRejectedUnselected, s.AudioRejectedInactive, s.AudioRejectedDamaged), fmt.Sprintf(i18n.Source("text.2c030f3a5bab"), s.SyncHits, s.NormalBursts), fmt.Sprintf(i18n.Source("text.783b84029c2e"), s.SlotBursts[0], s.SlotBursts[1], s.SlotBursts[2], s.SlotBursts[3]), fmt.Sprintf(i18n.Source("text.3c4602200dea"), s.SCHValid, s.SCHCRCFailures), fmt.Sprintf(i18n.Source("text.904d9f317863"), s.MACPDUTypes[0], s.MACPDUTypes[1], s.MACPDUTypes[2], s.MACPDUTypes[3]), fmt.Sprintf(i18n.Source("text.7ffbad9e01a1"), s.MACResources, s.MACRejected), fmt.Sprintf(i18n.Source("text.fe17af399ef4"), s.MACChannelAlloc, s.MACEncrypted), fmt.Sprintf(i18n.Source("text.55cb224e4649"), s.LLCTypes[0], s.LLCTypes[1], s.LLCTypes[2], s.LLCTypes[3], s.LLCTypes[4], s.LLCTypes[5], s.LLCTypes[6], s.LLCTypes[7]), fmt.Sprintf(i18n.Source("text.e3eea1cdd702"), s.LLCTypes[8], s.LLCTypes[9], s.LLCTypes[10], s.LLCTypes[11], s.LLCTypes[12], s.LLCTypes[13], s.LLCTypes[14], s.LLCTypes[15]), fmt.Sprintf(i18n.Source("text.70dd11cfa1d9"), s.LLCFragments, s.LLCReassembled), fmt.Sprintf(i18n.Source("text.76d31f2b8627"), s.MLEProtocols[1], s.MLEProtocols[2], s.MLEProtocols[4]), fmt.Sprintf(i18n.Source("text.ad3903b0ea87"), s.LLCNonCMCE, s.LLCRejected), fmt.Sprintf(i18n.Source("text.181c066d0f1c"), s.CMCEEvents)}
	for i, line := range lines {
		simpleui.DrawTextStyled(line, 58, 216+float32(i)*29, 15, simpleui.FontMono, colors.text)
	}
}
func viewerTime(t time.Time) string {
	if t.IsZero() {
		return "--:--:--"
	}
	return t.Format("15:04:05.000")
}
