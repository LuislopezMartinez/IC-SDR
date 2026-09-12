package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/tetra"
	"go-zero/simpleui"
)

var tetraTabs = []string{"NET", "CELLS", "GROUPS", "USERS", "MESSAGES", "GPS", "CONSOLE"}

func RunTETRAViewer(path, settingsPath string) {
	simpleui.SetMode(1400, 780, simpleui.Fit)
	simpleui.SetInitialWindowSize(700, 390)
	// The viewer is commonly kept beside the receiver at a reduced size. Use
	// larger glyphs and smooth canvas reduction so text remains readable near
	// the minimum window dimensions.
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.55)
	simpleui.SetTitle("IC-SDR · TETRA Monitor")
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
	simpleui.DrawTextStyled("TETRA MONITOR", 28, 22, 24, simpleui.FontSemiBold, colors.cyan)
	state := v.snapshot.Status.State
	if state == "" {
		state = "WAITING FOR DATA"
	}
	simpleui.DrawText(fmt.Sprintf("%.6f MHz · %s · updated %s", float64(v.snapshot.FrequencyHz)/1e6, state, viewerTime(v.snapshot.Updated)), 28, 56, 13, colors.muted)
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
	simpleui.DrawTextStyled("NEIGHBOUR CELLS · D-NWRK-BROADCAST", 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	if len(v.snapshot.Neighbours) == 0 {
		simpleui.DrawText("Waiting for neighbour cell announcements…", 48, 235, 14, colors.muted)
		return
	}
	columns := []struct {
		x    float32
		text string
	}{{48, "CELL"}, {145, "FREQUENCY"}, {350, "CARRIER"}, {480, "MCC"}, {560, "MNC"}, {650, "LA"}, {750, "SYNC"}, {850, "SERVICE"}, {990, "VOICE"}, {1090, "UPDATED"}}
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
		frequency := "UNKNOWN"
		if n.FrequencyHz > 0 {
			frequency = fmt.Sprintf("%.6f MHz", float64(n.FrequencyHz)/1e6)
		}
		syncText, syncColor := "NO", colors.muted
		if n.Synchronized {
			syncText, syncColor = "YES", colors.green
		}
		voiceText := "—"
		if n.Voice {
			voiceText = "TETRA"
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
	simpleui.DrawText("Click a cell with a known frequency to tune it.", 48, 704, 12, colors.muted)
}
func (v *tetraViewer) drawGroups() {
	simpleui.DrawTextStyled("CMCE CALLS AND DESTINATIONS", 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	if len(v.snapshot.Calls) > 0 {
		for _, column := range []struct {
			x float32
			t string
		}{{48, "CALL ID"}, {180, "SSI / DESTINATION"}, {390, "TS"}, {465, "USAGE"}, {570, "STATUS"}, {830, "CIPHER"}, {1010, "LAST EVENT"}} {
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
			activity := "ENDED"
			if call.Active {
				stateColor, activity = colors.green, "ACTIVE"
			}
			cipher := "CLEAR"
			cipherColor := colors.green
			if call.Encrypted {
				cipher, cipherColor = "CIPHER", colors.orange
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
	simpleui.DrawText("Observed destinations from clear signalling; waiting for a confirmed Call ID.", 48, 207, 12, colors.muted)
	if len(v.snapshot.Groups) == 0 {
		simpleui.DrawText("Waiting for D-SETUP, D-CONNECT or D-TX GRANTED…", 48, 260, 14, colors.muted)
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
		simpleui.DrawText(fmt.Sprintf("%d events", g.Calls), 790, y, 13, colors.text)
		simpleui.DrawText(viewerTime(g.LastSeen), 1020, y, 13, colors.text)
	}
}
func (v *tetraViewer) drawMessages() {
	simpleui.DrawTextStyled("CMCE / SDS EVENTS", 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	if len(v.snapshot.Messages) == 0 {
		simpleui.DrawText("Waiting for unencrypted CMCE signalling…", 48, 235, 14, colors.muted)
		return
	}
	columns := []struct {
		x    float32
		text string
	}{{48, T("TIME")}, {150, T("TYPE")}, {315, "SSI"}, {425, "PARTY SSI"}, {550, "TS"}, {590, T("PROTOCOL")}, {755, T("ENC.")}, {805, T("CONTENT / DIAGNOSIS")}}
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
		protocol := "CMCE"
		if m.SDS {
			protocol = m.ProtocolName
			if protocol == "" {
				protocol = fmt.Sprintf("SDS %d", m.SDSProtocol)
			}
			protocol = fmt.Sprintf("T%d · %s", m.SDSDataType, protocol)
		}
		detail := m.Text
		if m.RawHex != "" && !m.Recognized {
			detail += " · RAW " + m.RawHex
		}
		slot := "—"
		if m.Slot > 0 {
			slot = fmt.Sprintf("%d", m.Slot)
		}
		simpleui.DrawText(viewerTime(m.Time), 48, y, 12, colors.muted)
		cipher, cipherColor := "NO", colors.green
		if m.Encrypted {
			cipher, cipherColor = T("YES"), colors.red
		}
		simpleui.DrawTextStyled(sondeClip(m.Kind, 21), 150, y, 12, simpleui.FontSemiBold, kindColor)
		simpleui.DrawTextStyled(viewerSSI(m.AddressSSI), 315, y, 12, simpleui.FontMono, colors.text)
		simpleui.DrawTextStyled(viewerSSI(m.PartySSI), 425, y, 12, simpleui.FontMono, colors.text)
		simpleui.DrawText(slot, 550, y, 12, colors.text)
		simpleui.DrawText(sondeClip(protocol, 20), 590, y, 12, kindColor)
		simpleui.DrawText(cipher, 755, y, 12, cipherColor)
		simpleui.DrawText(sondeClip(detail, 68), 805, y, 12, colors.text)
	}
	simpleui.DrawText(fmt.Sprintf("%d %s", len(v.snapshot.Messages), T("events kept · orange = SDS still unrecognized")), 48, 716, 12, colors.muted)
}

func viewerSSI(ssi uint32) string {
	if ssi == 0 {
		return "—"
	}
	return fmt.Sprintf("%08d", ssi)
}
func (v *tetraViewer) drawUsers() {
	simpleui.DrawTextStyled("USERS DETECTED · MAC-RESOURCE", 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	cols := []struct {
		x     float32
		label string
	}{{48, "SSI"}, {260, "TYPE"}, {430, "TIMESLOT"}, {590, "CIPHER"}, {760, "SEEN"}, {940, "LAST ACTIVITY"}}
	for _, c := range cols {
		simpleui.DrawTextStyled(c.label, c.x, 220, 13, simpleui.FontSemiBold, colors.cyan)
	}
	if len(v.snapshot.Users) == 0 {
		simpleui.DrawText("Waiting for MAC-RESOURCE with valid SCH/F CRC…", 48, 270, 14, colors.muted)
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
		kind := map[uint8]string{1: "SSI", 3: "USSI", 4: "SMI", 5: "SSI+EVENT", 6: "SSI+USAGE", 7: "SMI+EVENT"}[u.AddressType]
		enc, color := "CLEAR", colors.green
		if u.Encrypted {
			enc, color = "CIPHER", colors.orange
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
	simpleui.DrawTextStyled("SERVING CELL AND NET", 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	if !s.System.Valid {
		simpleui.DrawText("Waiting for a valid BSCH block…", 48, 220, 15, colors.muted)
		return
	}
	cards := []struct{ label, value string }{{"MCC", fmt.Sprint(s.System.MCC)}, {"MNC", fmt.Sprint(s.System.MNC)}, {"COLOR", fmt.Sprint(s.System.ColourCode)}, {"TIMESLOT", fmt.Sprint(s.System.Timeslot)}, {"FRAME", fmt.Sprint(s.System.Frame)}, {"MULTIFRAME", fmt.Sprint(s.System.Multiframe)}}
	for i, c := range cards {
		x := 48 + float32(i%3)*425
		y := 220 + float32(i/3)*130
		drawPanel(x, y, 390, 100)
		simpleui.DrawText(c.label, x+18, y+15, 12, colors.muted)
		simpleui.DrawTextStyled(c.value, x+18, y+43, 30, simpleui.FontMono, colors.text)
	}
	simpleui.DrawText(fmt.Sprintf("SYNC %d · NTS1 %d · CRC failures %d · quality %.0f%%", s.SyncHits, s.NormalBursts, s.BSCHFailures, s.Quality), 48, 505, 14, colors.green)
	if s.Network.Valid {
		simpleui.DrawText(fmt.Sprintf("SYSINFO · CARRIER %d · BAND %d · DL %.6f MHz · UL %.6f MHz · LA %d · SERVICES %03X", s.Network.MainCarrier, s.Network.FrequencyBand, float64(s.Network.DownlinkHz)/1e6, float64(s.Network.UplinkHz)/1e6, s.Network.LocationArea, s.Network.ServiceDetails), 48, 530, 13, colors.cyan)
	}
	simpleui.DrawTextStyled("ACTIVITY BY TIMESLOT", 48, 565, 14, simpleui.FontSemiBold, colors.cyan)
	maxBursts := uint64(1)
	for _, n := range s.SlotBursts {
		if n > maxBursts {
			maxBursts = n
		}
	}
	for i, n := range s.SlotBursts {
		x := 48 + float32(i)*305
		simpleui.DrawText(fmt.Sprintf("TS%d  %d", i+1, n), x, 600, 13, colors.text)
		w := float32(n) * 250 / float32(maxBursts)
		rl.DrawRectangle(int32(x), 630, int32(w), 18, colors.green)
		rl.DrawRectangleLines(int32(x), 630, 250, 18, colors.border)
	}
}
func (v *tetraViewer) drawEmpty(title string, count int, hint string) {
	simpleui.DrawTextStyled(title, 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(fmt.Sprintf("%d items", count), 48, 212, 14, colors.text)
	simpleui.DrawText(hint, 48, 260, 14, colors.muted)
	simpleui.DrawText("The window already receives the live snapshot and will keep this tab for the next protocol phase.", 48, 292, 13, colors.muted)
}
func (v *tetraViewer) drawGPS() {
	simpleui.DrawTextStyled("GPS / LIP POSITIONS", 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
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
	simpleui.DrawText(fmt.Sprintf("%d positions", len(v.snapshot.Positions)), 990, 230, 15, colors.text)
	if len(v.snapshot.Positions) == 0 {
		simpleui.DrawText("Waiting for LIP messages", 990, 270, 13, colors.muted)
		return
	}
	for i, p := range v.snapshot.Positions {
		if i >= 10 {
			break
		}
		y := 270 + float32(i)*42
		simpleui.DrawTextStyled(fmt.Sprintf("SSI %08d", p.SSI), 990, y, 14, simpleui.FontMono, colors.cyan)
		simpleui.DrawText(fmt.Sprintf("%.5f, %.5f · %.0f km/h · %.0f° · ±%.0f m", p.Latitude, p.Longitude, p.SpeedKmh, p.Heading, p.AccuracyM), 990, y+19, 12, colors.text)
	}
}
func (v *tetraViewer) drawConsole() {
	s := v.snapshot.Status
	simpleui.DrawTextStyled("TECHNICAL CONSOLE", 48, 175, 18, simpleui.FontSemiBold, colors.cyan)
	lines := []string{fmt.Sprintf("State               %s", s.State), fmt.Sprintf("Input/channel       %.0f / %.0f S/s", s.InputRate, s.OutputRate), fmt.Sprintf("Level/quality/AFC   %.1f dBFS / %.1f%% / %+.1f Hz", s.LevelDBFS, s.Quality, s.FrequencyErrorHz), fmt.Sprintf("Timing phase/error  %d / %.3f rad", s.TimingPhase, s.TimingError), fmt.Sprintf("BER / FER           %.2f%% / %.1f%%", s.BER, s.FER), fmt.Sprintf("Audio decision      %s", s.LastAudioDecision), fmt.Sprintf("Reject E/U/I/D      %d / %d / %d / %d", s.AudioRejectedEncrypted, s.AudioRejectedUnselected, s.AudioRejectedInactive, s.AudioRejectedDamaged), fmt.Sprintf("SYNC / NTS          %d / %d", s.SyncHits, s.NormalBursts), fmt.Sprintf("TS activity         %d / %d / %d / %d", s.SlotBursts[0], s.SlotBursts[1], s.SlotBursts[2], s.SlotBursts[3]), fmt.Sprintf("SCH valid/fail      %d / %d", s.SCHValid, s.SCHCRCFailures), fmt.Sprintf("MAC R/F/B/S         %d / %d / %d / %d", s.MACPDUTypes[0], s.MACPDUTypes[1], s.MACPDUTypes[2], s.MACPDUTypes[3]), fmt.Sprintf("RESOURCE/reject     %d / %d", s.MACResources, s.MACRejected), fmt.Sprintf("Alloc/cipher        %d / %d", s.MACChannelAlloc, s.MACEncrypted), fmt.Sprintf("LLC types 0..7      %d %d %d %d %d %d %d %d", s.LLCTypes[0], s.LLCTypes[1], s.LLCTypes[2], s.LLCTypes[3], s.LLCTypes[4], s.LLCTypes[5], s.LLCTypes[6], s.LLCTypes[7]), fmt.Sprintf("LLC types 8..15     %d %d %d %d %d %d %d %d", s.LLCTypes[8], s.LLCTypes[9], s.LLCTypes[10], s.LLCTypes[11], s.LLCTypes[12], s.LLCTypes[13], s.LLCTypes[14], s.LLCTypes[15]), fmt.Sprintf("Fragments/joined    %d / %d", s.LLCFragments, s.LLCReassembled), fmt.Sprintf("MLE MM/CMCE/SNDCP   %d / %d / %d", s.MLEProtocols[1], s.MLEProtocols[2], s.MLEProtocols[4]), fmt.Sprintf("LLC non-CMCE/reject %d / %d", s.LLCNonCMCE, s.LLCRejected), fmt.Sprintf("CMCE events         %d", s.CMCEEvents)}
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
