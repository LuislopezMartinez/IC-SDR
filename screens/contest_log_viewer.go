package screens

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go-zero/internal/contestlog"
	"go-zero/internal/i18n"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const contestViewerRows = 12

type contestHistoryEntry struct {
	session *contestlog.Session
	path    string
	active  bool
}

type ContestLogViewer struct {
	simpleui.BaseElement
	panel                                     *ContestLogPanel
	manager                                   *simpleui.Manager
	search, call, date, utc, frequency        *simpleui.TextField
	rstSent, rstReceived, serialReceived      *simpleui.TextField
	participantName, participantEmail         *simpleui.TextField
	participantAddress, participantCity       *simpleui.TextField
	participantCountry, participantClub       *simpleui.TextField
	close, save, remove, confirm, cancel      *simpleui.Button
	previous, next, reopen, archiveExport     *simpleui.Button
	participant, saveParticipant              *simpleui.Button
	nextIssue                                 *simpleui.Button
	contestIndex                              *simpleui.Button
	open, confirming, finishConfirm, archived bool
	participantMode, historyMode              bool
	selected, scroll                          int
	session                                   *contestlog.Session
	archivePath                               string
	archives                                  []string
	historyEntries                            []contestHistoryEntry
	archiveIndex                              int
	status                                    string
	statusColor                               rl.Color
	issueCursor                               int
}

func NewContestLogViewer(panel *ContestLogPanel) *ContestLogViewer {
	v := &ContestLogViewer{BaseElement: simpleui.NewBaseElement("contestLogViewer", 0, 0, designWidth, designHeight),
		panel: panel, manager: simpleui.NewManager(), selected: -1, statusColor: colors.muted}
	v.search = simpleui.NewTextField("contestViewerSearch", 102, 137, 325, 40, i18n.Source("text.log0038"), 14)
	v.call = simpleui.NewTextField("contestViewerCall", 985, 249, 230, 40, i18n.Source("text.log0005"), 16)
	v.date = simpleui.NewTextField("contestViewerDate", 985, 326, 145, 38, i18n.Source("text.log0039"), 13)
	v.utc = simpleui.NewTextField("contestViewerUTC", 1142, 326, 100, 38, i18n.Source("text.log0040"), 13)
	v.frequency = simpleui.NewTextField("contestViewerFrequency", 1254, 326, 190, 38, i18n.Source("text.log0041"), 13)
	v.rstSent = simpleui.NewTextField("contestViewerRSTSent", 985, 401, 100, 38, i18n.Source("text.log0006"), 14)
	v.rstReceived = simpleui.NewTextField("contestViewerRSTReceived", 1097, 401, 100, 38, i18n.Source("text.log0007"), 14)
	v.serialReceived = simpleui.NewTextField("contestViewerSerialReceived", 1209, 401, 135, 38, i18n.Source("text.log0008"), 14)
	v.close = simpleui.NewButton("contestViewerClose", 1390, 91, 105, 40, i18n.Source("text.log0042"), 12)
	v.save = simpleui.NewButton("contestViewerSave", 985, 473, 220, 42, i18n.Source("text.log0043"), 12)
	v.save.SetColors(actionStartFill, colors.green, colors.text)
	v.remove = simpleui.NewButton("contestViewerDelete", 1217, 473, 220, 42, i18n.Source("text.log0044"), 12)
	v.remove.SetColors(actionStopFill, colors.red, colors.text)
	v.confirm = simpleui.NewButton("contestViewerConfirmDelete", 1035, 646, 210, 44, i18n.Source("text.log0045"), 12)
	v.confirm.SetColors(actionStopFill, colors.red, colors.text)
	v.cancel = simpleui.NewButton("contestViewerCancelDelete", 1257, 646, 160, 44, i18n.Source("text.log0046"), 12)
	v.previous = simpleui.NewButton("contestViewerPreviousArchive", 450, 137, 92, 40, i18n.Source("text.log0088"), 11)
	v.next = simpleui.NewButton("contestViewerNextArchive", 552, 137, 92, 40, i18n.Source("text.log0089"), 11)
	v.archiveExport = simpleui.NewButton("contestViewerArchiveExport", 654, 137, 170, 40, i18n.Source("text.log0012"), 11)
	v.reopen = simpleui.NewButton("contestViewerReopen", 834, 137, 170, 40, i18n.Source("text.log0090"), 11)
	v.participant = simpleui.NewButton("contestViewerParticipant", 1015, 137, 205, 40, i18n.Source("text.log0118"), 11)
	v.participant.OnClick(v.showParticipant)
	v.contestIndex = simpleui.NewButton("contestViewerContestIndex", 1230, 137, 205, 40, i18n.Source("text.log0141"), 11)
	v.contestIndex.OnClick(v.OpenHistory)
	v.participantName = simpleui.NewTextField("contestParticipantName", 985, 249, 220, 40, i18n.Source("text.log0119"), 13)
	v.participantEmail = simpleui.NewTextField("contestParticipantEmail", 1217, 249, 228, 40, i18n.Source("text.log0120"), 13)
	v.participantAddress = simpleui.NewTextField("contestParticipantAddress", 985, 326, 460, 40, i18n.Source("text.log0121"), 13)
	v.participantCity = simpleui.NewTextField("contestParticipantCity", 985, 401, 220, 40, i18n.Source("text.log0122"), 13)
	v.participantCountry = simpleui.NewTextField("contestParticipantCountry", 1217, 401, 228, 40, i18n.Source("text.log0123"), 13)
	v.participantClub = simpleui.NewTextField("contestParticipantClub", 985, 478, 220, 40, i18n.Source("text.log0124"), 13)
	v.saveParticipant = simpleui.NewButton("contestParticipantSave", 1217, 478, 228, 40, i18n.Source("text.log0125"), 12)
	v.saveParticipant.SetColors(actionStartFill, colors.green, colors.text)
	v.saveParticipant.OnClick(v.saveParticipantData)
	v.nextIssue = simpleui.NewButton("contestParticipantNextIssue", 1217, 575, 228, 40, i18n.Source("text.log0128"), 11)
	v.nextIssue.OnClick(v.goToNextIssue)

	callFilter := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '/' }
	digitFilter := func(r rune) bool { return unicode.IsDigit(r) }
	v.search.SetFilter(callFilter)
	v.call.SetFilter(callFilter)
	for _, field := range []*simpleui.TextField{v.search, v.call, v.date, v.utc, v.frequency, v.rstSent, v.rstReceived, v.serialReceived,
		v.participantName, v.participantEmail, v.participantAddress, v.participantCity, v.participantCountry, v.participantClub} {
		field.SetFont(simpleui.FontSemiBold)
	}
	for _, field := range []*simpleui.TextField{v.rstSent, v.rstReceived} {
		field.SetFilter(digitFilter)
	}
	v.search.OnChange(func(string) { v.scroll = 0 })
	v.close.OnClick(v.Close)
	v.save.OnClick(v.saveSelected)
	v.remove.OnClick(func() {
		if v.selected >= 0 {
			v.finishConfirm = false
			v.setConfirming(true)
		}
	})
	v.confirm.OnClick(func() {
		if v.finishConfirm {
			v.finishSession()
		} else {
			v.deleteSelected()
		}
	})
	v.cancel.OnClick(func() { v.setConfirming(false) })
	v.previous.OnClick(func() { v.openArchive(v.archiveIndex + 1) })
	v.next.OnClick(func() { v.openArchive(v.archiveIndex - 1) })
	v.archiveExport.OnClick(v.exportArchived)
	v.reopen.OnClick(v.reopenArchived)
	for _, control := range []simpleui.Element{v.search, v.call, v.date, v.utc, v.frequency, v.rstSent, v.rstReceived,
		v.serialReceived, v.close, v.save, v.remove, v.confirm, v.cancel, v.previous, v.next, v.archiveExport, v.reopen,
		v.participant, v.participantName, v.participantEmail, v.participantAddress, v.participantCity, v.participantCountry,
		v.participantClub, v.saveParticipant, v.nextIssue, v.contestIndex} {
		v.manager.Add(control)
	}
	v.setConfirming(false)
	v.refreshArchiveControls()
	v.setParticipantMode(false)
	return v
}

func (v *ContestLogViewer) Open(preferred int) {
	if v.panel.session == nil {
		return
	}
	v.session, v.archivePath, v.archived = v.panel.session, "", false
	v.open, v.status, v.scroll, v.historyMode = true, "", 0, false
	if preferred < 0 && len(v.session.QSOs) > 0 {
		preferred = len(v.session.QSOs) - 1
	}
	v.refreshArchiveControls()
	v.setParticipantMode(false)
	v.selectQSO(preferred)
	v.search.SetFocused(preferred < 0)
}

func (v *ContestLogViewer) OpenParticipant() {
	if v.panel.session == nil {
		return
	}
	v.Open(-1)
	v.showParticipant()
}

func (v *ContestLogViewer) OpenFinish() {
	if v.panel.session == nil {
		return
	}
	v.Open(len(v.panel.session.QSOs) - 1)
	v.finishConfirm = true
	v.setConfirming(true)
}

func (v *ContestLogViewer) OpenHistory() {
	v.archives = v.panel.archivePaths()
	v.historyEntries = nil
	if v.panel.session != nil {
		v.historyEntries = append(v.historyEntries, contestHistoryEntry{session: v.panel.session, active: true})
	}
	for _, path := range v.archives {
		if session, err := contestlog.Load(path); err == nil {
			v.historyEntries = append(v.historyEntries, contestHistoryEntry{session: session, path: path})
		}
	}
	if len(v.historyEntries) == 0 {
		return
	}
	v.open, v.status, v.scroll, v.archived, v.historyMode = true, "", 0, false, true
	v.session, v.selected = nil, -1
	v.setConfirming(false)
	v.refreshArchiveControls()
	v.setParticipantMode(false)
}

func (v *ContestLogViewer) openArchive(index int) {
	if index < 0 || index >= len(v.archives) {
		return
	}
	session, err := contestlog.Load(v.archives[index])
	if err != nil {
		v.setStatus(err.Error(), colors.red)
		return
	}
	v.archiveIndex, v.archivePath, v.session, v.archived = index, v.archives[index], session, true
	v.historyMode = false
	v.scroll, v.selected = 0, -1
	if len(session.QSOs) > 0 {
		v.selectQSO(len(session.QSOs) - 1)
	}
	v.refreshArchiveControls()
	v.setParticipantMode(false)
}

func (v *ContestLogViewer) Close() {
	v.open = false
	v.setConfirming(false)
	for _, field := range []*simpleui.TextField{v.search, v.call, v.date, v.utc, v.frequency, v.rstSent, v.rstReceived, v.serialReceived} {
		field.SetFocused(false)
	}
	for _, field := range []*simpleui.TextField{v.participantName, v.participantEmail, v.participantAddress, v.participantCity, v.participantCountry, v.participantClub} {
		field.SetFocused(false)
	}
}

func (v *ContestLogViewer) OverlayOpen() bool          { return v.open }
func (v *ContestLogViewer) Update(simpleui.Input) bool { return false }
func (v *ContestLogViewer) Draw()                      {}

func (v *ContestLogViewer) UpdateOverlay(input simpleui.Input) bool {
	if !v.open {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		if v.confirming {
			v.setConfirming(false)
		} else {
			v.Close()
		}
		return true
	}
	v.manager.Update(input)
	table := rl.Rectangle{X: 95, Y: 207, Width: 820, Height: 510}
	if v.historyMode {
		maxScroll := max(0, len(v.historyEntries)-contestViewerRows)
		if input.Over(table) && input.Wheel != 0 {
			v.scroll = min(max(v.scroll-int(input.Wheel), 0), maxScroll)
		}
		if input.Released && input.Over(table) {
			row := int((input.Pointer.Y - 253) / 36)
			position := v.scroll + row
			if row >= 0 && row < contestViewerRows && position < len(v.historyEntries) {
				entry := v.historyEntries[position]
				if entry.active {
					v.Open(-1)
				} else {
					for index, path := range v.archives {
						if path == entry.path {
							v.openArchive(index)
							break
						}
					}
				}
			}
		}
		return true
	}
	indices := v.filteredIndices()
	maxScroll := max(0, len(indices)-contestViewerRows)
	if input.Over(table) && input.Wheel != 0 {
		v.scroll = min(max(v.scroll-int(input.Wheel), 0), maxScroll)
	}
	if !v.confirming && input.Released && input.Over(table) {
		row := int((input.Pointer.Y - 253) / 36)
		if row >= 0 && row < contestViewerRows {
			position := v.scroll + row
			if position < len(indices) {
				v.selectQSO(indices[position])
			}
		}
	}
	return true
}

func (v *ContestLogViewer) DrawOverlay() {
	if !v.open {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{R: 0, G: 0, B: 0, A: 210})
	drawPanel(72, 68, 1456, 750)
	title := i18n.Source("text.log0047")
	if v.session != nil {
		title = fmt.Sprintf(title, v.session.Contest)
	}
	if v.historyMode {
		title = i18n.Source("text.log0142")
	}
	simpleui.DrawTextStyled(title, 100, 91, 20, simpleui.FontSemiBold, colors.cyan)
	count := 0
	if v.historyMode {
		count = len(v.historyEntries)
	} else if v.session != nil {
		count = len(v.session.QSOs)
	}
	countText := fmt.Sprintf(i18n.Source("text.log0048"), count)
	if v.historyMode {
		countText = fmt.Sprintf(i18n.Source("text.log0143"), count)
	}
	simpleui.DrawText(countText, 100, 116, 12, colors.muted)
	if v.archived && v.session != nil && v.session.ClosedAt != nil {
		simpleui.DrawText(fmt.Sprintf(i18n.Source("text.log0091"), v.archiveIndex+1, len(v.archives), v.session.ClosedAt.UTC().Format("2006-01-02 15:04")), 1030, 116, 11, colors.orange)
	}
	if v.historyMode {
		v.drawHistoryTable()
		v.drawHistoryHelp()
	} else {
		v.drawTable()
		v.drawSummary()
	}
	if v.historyMode {
		// The index uses the full overlay; no QSO editor is drawn.
	} else if v.participantMode {
		v.drawParticipant()
	} else {
		v.drawEditor()
	}
	v.manager.Draw()
}

func (v *ContestLogViewer) drawHistoryTable() {
	rl.DrawRectangleRounded(rl.Rectangle{X: 94, Y: 194, Width: 822, Height: 548}, .025, 6, colors.background)
	headers := []struct {
		text string
		x    float32
	}{{i18n.Source("text.log0144"), 113}, {i18n.Source("text.log0005"), 270}, {i18n.Source("text.log0145"), 390},
		{i18n.Source("text.log0146"), 665}, {i18n.Source("text.log0147"), 730}, {i18n.Source("text.log0033"), 810}}
	for _, header := range headers {
		simpleui.DrawTextStyled(header.text, header.x, 216, 12, simpleui.FontSemiBold, colors.muted)
	}
	for row := 0; row < contestViewerRows && v.scroll+row < len(v.historyEntries); row++ {
		entry := v.historyEntries[v.scroll+row]
		session := entry.session
		y := float32(253 + row*36)
		background := colors.panelAlt
		if row%2 != 0 {
			background = colors.panel
		}
		rl.DrawRectangleRounded(rl.Rectangle{X: 103, Y: y - 7, Width: 804, Height: 31}, .04, 4, background)
		state, stateColor := i18n.Source("text.log0148"), colors.muted
		if entry.active {
			state, stateColor = i18n.Source("text.log0149"), colors.green
		}
		values := []struct {
			text string
			x    float32
		}{{session.CreatedAt.UTC().Format("2006-01-02 15:04"), 113}, {session.StationCall, 270}, {session.Contest, 390},
			{strconv.Itoa(len(session.QSOs)), 665}, {strconv.Itoa(session.Score().Total), 730}}
		for _, value := range values {
			simpleui.DrawTextStyled(value.text, value.x, y, 14, simpleui.FontSemiBold, colors.text)
		}
		simpleui.DrawTextStyled(state, 810, y, 12, simpleui.FontSemiBold, stateColor)
	}
}

func (v *ContestLogViewer) drawHistoryHelp() {
	simpleui.DrawTextStyled(i18n.Source("text.log0150"), 970, 194, 17, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(i18n.Source("text.log0151"), 970, 229, 12, colors.text)
	simpleui.DrawText(i18n.Source("text.log0152"), 970, 255, 11, colors.muted)
}

func (v *ContestLogViewer) drawSummary() {
	if v.session == nil {
		return
	}
	score := v.session.Score()
	text := fmt.Sprintf(i18n.Source("text.log0112"), score.ValidQSOs, score.Points, score.Multipliers, score.Total)
	simpleui.DrawTextStyled(text, 102, 758, 13, simpleui.FontSemiBold, colors.cyan)
	band := func(name string) string {
		value := score.Bands[name]
		return fmt.Sprintf("%s %dP/%dM", name, value.Points, value.Multipliers)
	}
	simpleui.DrawTextStyled(strings.Join([]string{band("160M"), band("80M"), band("40M"), band("20M"), band("15M"), band("10M")}, "  ·  "), 102, 781, 11, simpleui.FontSemiBold, colors.muted)
}

func (v *ContestLogViewer) drawTable() {
	rl.DrawRectangleRounded(rl.Rectangle{X: 94, Y: 194, Width: 822, Height: 548}, .025, 6, colors.background)
	headers := []struct {
		text string
		x    float32
	}{{i18n.Source("text.log0029"), 113}, {i18n.Source("text.log0030"), 235}, {i18n.Source("text.log0005"), 310},
		{i18n.Source("text.log0049"), 490}, {i18n.Source("text.log0050"), 625}, {i18n.Source("text.log0033"), 842}}
	for _, header := range headers {
		simpleui.DrawTextStyled(header.text, header.x, 216, 12, simpleui.FontSemiBold, colors.muted)
	}
	indices := v.filteredIndices()
	for row := 0; row < contestViewerRows && v.scroll+row < len(indices); row++ {
		index := indices[v.scroll+row]
		q := v.session.QSOs[index]
		y := float32(253 + row*36)
		background := colors.panelAlt
		if index == v.selected {
			background = blendRGBA(colors.panelAlt, colors.blue, .45)
		}
		if row%2 != 0 && index != v.selected {
			background = colors.panel
		}
		rl.DrawRectangleRounded(rl.Rectangle{X: 103, Y: y - 7, Width: 804, Height: 31}, .06, 4, background)
		values := []struct {
			text string
			x    float32
		}{
			{q.Time.UTC().Format("02-01 15:04"), 113}, {q.Band, 235}, {q.WorkedCall, 310},
			{fmt.Sprintf("%s/%s", q.RSTSent, q.SentExchangeText()), 490},
			{fmt.Sprintf("%s/%s", q.RSTReceived, q.ReceivedExchangeText()), 625},
		}
		for _, value := range values {
			simpleui.DrawTextStyled(value.text, value.x, y, 14, simpleui.FontSemiBold, colors.text)
		}
		state, stateColor := i18n.Source("text.log0073"), colors.green
		if q.Duplicate {
			state, stateColor = i18n.Source("text.log0074"), colors.orange
		} else if len(q.Warnings) > 0 {
			state, stateColor = i18n.Source("text.log0108"), colors.orange
		}
		simpleui.DrawTextStyled(state, 842, y, 12, simpleui.FontSemiBold, stateColor)
	}
	if len(indices) == 0 {
		simpleui.DrawText(i18n.Source("text.log0051"), 113, 265, 13, colors.muted)
	}
	if len(indices) > contestViewerRows {
		simpleui.DrawText(fmt.Sprintf(i18n.Source("text.log0052"), v.scroll+1, min(v.scroll+contestViewerRows, len(indices)), len(indices)), 112, 715, 11, colors.muted)
	}
}

func (v *ContestLogViewer) drawEditor() {
	simpleui.DrawTextStyled(i18n.Source("text.log0053"), 970, 194, 17, simpleui.FontSemiBold, colors.cyan)
	if v.selected < 0 || v.session == nil || v.selected >= len(v.session.QSOs) {
		simpleui.DrawText(i18n.Source("text.log0054"), 970, 226, 12, colors.muted)
		return
	}
	q := v.session.QSOs[v.selected]
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.log0055"), q.SerialSent, q.Band, q.ID), 970, 222, 10, colors.muted)
	for _, label := range []struct {
		text string
		x, y float32
	}{
		{i18n.Source("text.log0005"), 985, 235}, {i18n.Source("text.log0056"), 985, 309}, {i18n.Source("text.log0057"), 1142, 309}, {i18n.Source("text.log0041"), 1254, 309},
		{i18n.Source("text.log0058"), 985, 384}, {i18n.Source("text.log0059"), 1097, 384}, {v.receivedExchangeLabel(), 1209, 384}} {
		simpleui.DrawText(label.text, label.x, label.y, 10, colors.muted)
	}
	if v.status != "" {
		simpleui.DrawText(v.status, 985, 535, 11, v.statusColor)
	}
	if v.confirming {
		drawPanel(965, 550, 500, 152)
		message := i18n.Source("text.log0061")
		if v.finishConfirm {
			message = i18n.Source("text.log0092")
		}
		simpleui.DrawText(message, 985, 561, 11, colors.orange)
		if v.finishConfirm && v.session != nil {
			score := v.session.Score()
			simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.log0113"), score.Points, score.Multipliers, score.Total), 985, 587, 13, simpleui.FontSemiBold, colors.cyan)
			simpleui.DrawText(fmt.Sprintf(i18n.Source("text.log0114"), score.Duplicates, score.Warnings, score.NonScoring, score.Incomplete), 985, 612, 11, colors.orange)
		}
	}
}

func (v *ContestLogViewer) receivedExchangeLabel() string {
	if v.session == nil {
		return i18n.Source("text.log0060")
	}
	keys := map[string]string{"SERIAL": "text.log0060", "CQ_ZONE": "text.log0183", "ITU_ZONE": "text.log0184",
		"SECTION": "text.log0185", "LOCATOR": "text.log0186", "NUMBER": "text.log0187", "TEXT": "text.log0188"}
	if key := keys[v.session.ReceivedExchange]; key != "" {
		return i18n.Source(key)
	}
	return i18n.Source("text.log0060")
}

func (v *ContestLogViewer) drawParticipant() {
	simpleui.DrawTextStyled(i18n.Source("text.log0118"), 970, 194, 17, simpleui.FontSemiBold, colors.cyan)
	for _, label := range []struct {
		text string
		x, y float32
	}{
		{i18n.Source("text.log0119"), 985, 233}, {i18n.Source("text.log0120"), 1217, 233},
		{i18n.Source("text.log0121"), 985, 310}, {i18n.Source("text.log0122"), 985, 385},
		{i18n.Source("text.log0123"), 1217, 385}, {i18n.Source("text.log0124"), 985, 462}} {
		simpleui.DrawText(label.text, label.x, label.y, 10, colors.muted)
	}
	if v.session != nil {
		issues := v.session.ValidateSubmission()
		color := colors.green
		message := i18n.Source("text.log0126")
		if len(issues) > 0 {
			color = colors.orange
			message = fmt.Sprintf(i18n.Source("text.log0116"), len(issues))
		}
		simpleui.DrawTextStyled(message, 985, 540, 12, simpleui.FontSemiBold, color)
		if len(issues) > 0 {
			simpleui.DrawText(v.issueSummary(issues), 985, 560, 10, colors.muted)
		}
		if v.status != "" {
			simpleui.DrawText(v.status, 985, 630, 11, v.statusColor)
		}
	}
}

func (v *ContestLogViewer) issueSummary(issues []contestlog.ValidationIssue) string {
	labels := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, issue := range issues {
		label := validationIssueLabel(issue.Code)
		if issue.QSOIndex >= 0 {
			label = fmt.Sprintf("QSO %03d: %s", issue.QSOIndex+1, label)
		}
		if !seen[label] {
			labels, seen[label] = append(labels, label), true
		}
		if len(labels) == 3 {
			break
		}
	}
	result := strings.Join(labels, " · ")
	if len(issues) > len(labels) {
		result += fmt.Sprintf(i18n.Source("text.log0137"), len(issues)-len(labels))
	}
	return result
}

func validationIssueLabel(code string) string {
	keys := map[string]string{
		"no_session": "text.log0129", "station_call": "text.log0130", "name": "text.log0131",
		"email": "text.log0132", "address": "text.log0133", "country": "text.log0134",
		"no_qsos": "text.log0135", "qso_incomplete": "text.log0136", "qso_order": "text.log0138",
		"qso_band": "text.log0139",
	}
	if key, ok := keys[code]; ok {
		return i18n.Source(key)
	}
	return code
}

func (v *ContestLogViewer) filteredIndices() []int {
	if v.session == nil {
		return nil
	}
	query := contestlog.NormalizeCall(v.search.Text())
	indices := make([]int, 0, len(v.session.QSOs))
	for index := len(v.session.QSOs) - 1; index >= 0; index-- {
		if query == "" || strings.Contains(v.session.QSOs[index].WorkedCall, query) {
			indices = append(indices, index)
		}
	}
	return indices
}

func (v *ContestLogViewer) selectQSO(index int) {
	v.setParticipantMode(false)
	if v.session == nil || index < 0 || index >= len(v.session.QSOs) {
		v.selected = -1
		return
	}
	v.selected = index
	q := v.session.QSOs[index]
	v.call.SetText(q.WorkedCall)
	v.date.SetText(q.Time.UTC().Format("2006-01-02"))
	v.utc.SetText(q.Time.UTC().Format("15:04"))
	v.frequency.SetText(strconv.FormatFloat(float64(q.FrequencyHz)/1e6, 'f', 6, 64))
	v.rstSent.SetText(q.RSTSent)
	v.rstReceived.SetText(q.RSTReceived)
	v.serialReceived.SetText(q.ReceivedExchangeText())
	v.status = ""
}

func (v *ContestLogViewer) showParticipant() {
	if v.session == nil || v.archived {
		return
	}
	v.participantName.SetText(v.session.Name)
	v.participantEmail.SetText(v.session.Email)
	v.participantAddress.SetText(v.session.Address)
	v.participantCity.SetText(v.session.City)
	v.participantCountry.SetText(v.session.Country)
	v.participantClub.SetText(v.session.Club)
	v.issueCursor = 0
	v.setParticipantMode(true)
}

func (v *ContestLogViewer) setParticipantMode(enabled bool) {
	v.participantMode = enabled && !v.archived
	for _, control := range []simpleui.Element{v.call, v.date, v.utc, v.frequency, v.rstSent, v.rstReceived, v.serialReceived} {
		control.SetVisible(!v.participantMode && !v.historyMode)
	}
	for _, control := range []simpleui.Element{v.participantName, v.participantEmail, v.participantAddress, v.participantCity, v.participantCountry, v.participantClub, v.saveParticipant, v.nextIssue} {
		control.SetVisible(v.participantMode && !v.historyMode)
	}
	v.nextIssue.SetEnabled(v.participantMode && !v.historyMode && v.hasQSOIssues())
	v.save.SetVisible(!v.participantMode && !v.archived && !v.historyMode)
	v.remove.SetVisible(!v.participantMode && !v.archived && !v.historyMode)
	v.participant.SetVisible(!v.archived && !v.historyMode)
	v.contestIndex.SetVisible(!v.historyMode)
	v.search.SetVisible(!v.historyMode)
}

func (v *ContestLogViewer) hasQSOIssues() bool {
	if v.session == nil {
		return false
	}
	for _, issue := range v.session.ValidateSubmission() {
		if issue.QSOIndex >= 0 {
			return true
		}
	}
	return false
}

func (v *ContestLogViewer) goToNextIssue() {
	if v.session == nil {
		return
	}
	var qsoIssues []contestlog.ValidationIssue
	for _, issue := range v.session.ValidateSubmission() {
		if issue.QSOIndex >= 0 {
			qsoIssues = append(qsoIssues, issue)
		}
	}
	if len(qsoIssues) == 0 {
		return
	}
	issue := qsoIssues[v.issueCursor%len(qsoIssues)]
	v.issueCursor++
	v.search.SetText("")
	v.selectQSO(issue.QSOIndex)
	for position, index := range v.filteredIndices() {
		if index == issue.QSOIndex {
			v.scroll = min(position, max(0, len(v.session.QSOs)-contestViewerRows))
			break
		}
	}
	v.setStatus(fmt.Sprintf(i18n.Source("text.log0140"), issue.QSOIndex+1, validationIssueLabel(issue.Code)), colors.orange)
}

func (v *ContestLogViewer) saveParticipantData() {
	if v.session == nil || v.archived {
		return
	}
	previous := *v.session
	v.session.Name = strings.TrimSpace(v.participantName.Text())
	v.session.Email = strings.TrimSpace(v.participantEmail.Text())
	v.session.Address = strings.TrimSpace(v.participantAddress.Text())
	v.session.City = strings.TrimSpace(v.participantCity.Text())
	v.session.Country = strings.TrimSpace(v.participantCountry.Text())
	v.session.Club = strings.TrimSpace(v.participantClub.Text())
	if err := v.panel.saveActive(v.session); err != nil {
		*v.session = previous
		v.setStatus(fmt.Sprintf(i18n.Source("text.log0018"), err.Error()), colors.red)
		return
	}
	v.setStatus(i18n.Source("text.log0127"), colors.green)
}

func (v *ContestLogViewer) saveSelected() {
	if v.selected < 0 || v.session == nil || v.archived {
		return
	}
	when, err := time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(v.date.Text())+" "+strings.TrimSpace(v.utc.Text()), time.UTC)
	if err != nil {
		v.setStatus(i18n.Source("text.log0062"), colors.red)
		return
	}
	mhz, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v.frequency.Text()), ",", "."), 64)
	if err != nil {
		v.setStatus(i18n.Source("text.log0063"), colors.red)
		return
	}
	exchangeReceived := strings.TrimSpace(v.serialReceived.Text())
	if exchangeReceived == "" {
		v.setStatus(i18n.Source("text.log0064"), colors.red)
		return
	}
	serialReceived, _ := strconv.Atoi(exchangeReceived)
	snapshot := append([]contestlog.QSO(nil), v.session.QSOs...)
	updated, err := v.session.Update(v.selected, contestlog.Entry{Time: when, FrequencyHz: int64(mhz*1e6 + .5), Mode: "PH",
		WorkedCall: v.call.Text(), RSTSent: v.rstSent.Text(), RSTReceived: v.rstReceived.Text(), SerialReceived: serialReceived, ExchangeReceived: exchangeReceived})
	if err != nil {
		v.setStatus(err.Error(), colors.red)
		return
	}
	if err := v.panel.saveActive(v.session); err != nil {
		v.session.QSOs = snapshot
		v.setStatus(fmt.Sprintf(i18n.Source("text.log0018"), err.Error()), colors.red)
		return
	}
	v.selectQSO(v.selected)
	v.setStatus(fmt.Sprintf(i18n.Source("text.log0065"), updated.WorkedCall), colors.green)
	v.panel.setStatus(i18n.Source("text.log0066"), colors.green)
}

func (v *ContestLogViewer) deleteSelected() {
	if v.selected < 0 || v.session == nil || v.archived {
		return
	}
	snapshot := append([]contestlog.QSO(nil), v.session.QSOs...)
	removed, err := v.session.Delete(v.selected)
	if err != nil {
		v.setStatus(err.Error(), colors.red)
		return
	}
	if err := v.panel.saveActive(v.session); err != nil {
		v.session.QSOs = snapshot
		v.setStatus(fmt.Sprintf(i18n.Source("text.log0067"), err.Error()), colors.red)
		return
	}
	v.setConfirming(false)
	v.selected = -1
	if len(v.session.QSOs) > 0 {
		v.selectQSO(len(v.session.QSOs) - 1)
	}
	v.setStatus(fmt.Sprintf(i18n.Source("text.log0068"), removed.WorkedCall), colors.orange)
	v.panel.setStatus(i18n.Source("text.log0069"), colors.orange)
}

func (v *ContestLogViewer) setConfirming(confirming bool) {
	v.confirming = confirming
	if !confirming {
		v.finishConfirm = false
	}
	if v.finishConfirm {
		v.confirm.SetLabel(i18n.Source("text.log0093"))
	} else {
		v.confirm.SetLabel(i18n.Source("text.log0045"))
	}
	v.confirm.SetVisible(confirming)
	v.cancel.SetVisible(confirming)
	v.save.SetEnabled(!confirming)
	v.remove.SetEnabled(!confirming)
}

func (v *ContestLogViewer) refreshArchiveControls() {
	v.previous.SetVisible(v.archived && !v.historyMode)
	v.next.SetVisible(v.archived && !v.historyMode)
	v.archiveExport.SetVisible(v.archived && !v.historyMode)
	v.reopen.SetVisible(v.archived && !v.historyMode)
	v.reopen.SetEnabled(v.archived && v.panel.session == nil)
	v.participant.SetVisible(!v.archived && !v.historyMode)
	v.contestIndex.SetVisible(!v.historyMode)
	v.previous.SetEnabled(v.archived && v.archiveIndex+1 < len(v.archives))
	v.next.SetEnabled(v.archived && v.archiveIndex > 0)
	v.save.SetVisible(!v.archived && !v.historyMode)
	v.remove.SetVisible(!v.archived && !v.historyMode)
	for _, field := range []*simpleui.TextField{v.call, v.date, v.utc, v.frequency, v.rstSent, v.rstReceived, v.serialReceived} {
		field.SetEnabled(!v.archived)
	}
}

func (v *ContestLogViewer) finishSession() {
	if v.session != nil {
		if issues := v.session.ValidateSubmission(); len(issues) > 0 {
			v.setConfirming(false)
			v.setStatus(fmt.Sprintf(i18n.Source("text.log0117"), len(issues)), colors.orange)
			v.showParticipant()
			return
		}
	}
	if err := v.panel.finishSession(); err != nil {
		v.setStatus(fmt.Sprintf(i18n.Source("text.log0094"), err.Error()), colors.red)
		v.setConfirming(false)
		return
	}
	v.Close()
	v.panel.setStatus(i18n.Source("text.log0095"), colors.green)
}

func (v *ContestLogViewer) exportArchived() {
	path, err := v.panel.exportSession(v.session)
	if err != nil {
		v.setStatus(fmt.Sprintf(i18n.Source("text.log0021"), err.Error()), colors.red)
		return
	}
	v.setStatus(fmt.Sprintf(i18n.Source("text.log0022"), filepath.Base(path)), colors.green)
}

func (v *ContestLogViewer) reopenArchived() {
	if err := v.panel.reopenArchive(v.archivePath); err != nil {
		v.setStatus(fmt.Sprintf(i18n.Source("text.log0096"), err.Error()), colors.red)
		return
	}
	v.Close()
	v.panel.setStatus(i18n.Source("text.log0097"), colors.green)
	v.panel.workedCall.SetFocused(true)
}

func (v *ContestLogViewer) setStatus(message string, color rl.Color) {
	v.status, v.statusColor = message, color
}
