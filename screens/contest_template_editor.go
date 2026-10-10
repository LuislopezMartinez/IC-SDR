package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go-zero/internal/contestlog"
	"go-zero/internal/i18n"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type ContestTemplateEditor struct {
	simpleui.BaseElement
	panel                                           *ContestLogPanel
	manager                                         *simpleui.Manager
	name, id, start, end, points, sentValue         *simpleui.TextField
	mode, duplicate, sentExchange, receivedExchange *simpleui.Button
	previous, next, fresh, clone, save              *simpleui.Button
	remove, close                                   *simpleui.Button
	preview                                         *simpleui.Button
	bandButtons                                     []*simpleui.Button
	open, deleting                                  bool
	selected                                        int
	status                                          string
	previewText                                     []string
	statusColor                                     rl.Color
}

func NewContestTemplateEditor(panel *ContestLogPanel) *ContestTemplateEditor {
	e := &ContestTemplateEditor{BaseElement: simpleui.NewBaseElement("contestTemplateEditor", 0, 0, designWidth, designHeight),
		panel: panel, manager: simpleui.NewManager(), selected: -1, statusColor: colors.muted}
	e.name = simpleui.NewTextField("templateName", 570, 245, 430, 42, i18n.Source("text.log0161"), 14)
	e.id = simpleui.NewTextField("templateID", 1018, 245, 300, 42, i18n.Source("text.log0162"), 14)
	e.start = simpleui.NewTextField("templateStart", 570, 330, 300, 42, "2026-10-03 06:00", 14)
	e.end = simpleui.NewTextField("templateEnd", 888, 330, 300, 42, "2026-10-04 06:00", 14)
	e.points = simpleui.NewTextField("templatePoints", 1206, 330, 112, 42, "1", 14)
	e.sentValue = simpleui.NewTextField("templateSentValue", 1155, 480, 163, 42, i18n.Source("text.log0182"), 13)
	for _, field := range []*simpleui.TextField{e.name, e.id, e.start, e.end, e.points, e.sentValue} {
		field.SetFont(simpleui.FontSemiBold)
	}
	e.id.SetFilter(func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' })
	e.points.SetFilter(func(r rune) bool { return unicode.IsDigit(r) })
	e.mode = simpleui.NewButton("templateMode", 570, 416, 205, 42, "", 12)
	e.mode.OnClick(func() { e.mode.SetLabel(cycleChoice(e.mode.Label(), []string{"SSB", "CW", "DIGI"})) })
	e.duplicate = simpleui.NewButton("templateDuplicateRule", 793, 416, 315, 42, "", 11)
	e.duplicate.OnClick(func() {
		if e.duplicate.Label() == i18n.Source("text.log0168") {
			e.duplicate.SetLabel(i18n.Source("text.log0169"))
		} else {
			e.duplicate.SetLabel(i18n.Source("text.log0168"))
		}
	})
	e.sentExchange = simpleui.NewButton("templateSentExchange", 570, 480, 270, 42, i18n.Source("text.log0179"), 11)
	e.sentExchange.OnClick(func() {
		fixed := e.sentExchange.Label() == i18n.Source("text.log0179")
		if fixed {
			e.sentExchange.SetLabel(i18n.Source("text.log0180"))
		} else {
			e.sentExchange.SetLabel(i18n.Source("text.log0179"))
		}
		e.sentValue.SetEnabled(fixed)
	})
	e.receivedExchange = simpleui.NewButton("templateReceivedExchange", 858, 480, 280, 42, i18n.Source("text.log0189"), 11)
	e.receivedExchange.OnClick(func() {
		values := receivedExchangeLabels()
		e.receivedExchange.SetLabel(cycleChoice(e.receivedExchange.Label(), values))
	})
	for index, band := range []string{"160M", "80M", "40M", "20M", "15M", "10M"} {
		band := band
		button := simpleui.NewButton("templateBand"+band, 570+float32(index)*120, 548, 108, 34, "[X] "+band, 11)
		button.OnClick(func() { e.toggleBand(band) })
		e.bandButtons = append(e.bandButtons, button)
	}
	e.previous = simpleui.NewButton("templatePrevious", 300, 245, 105, 38, i18n.Source("text.log0088"), 11)
	e.next = simpleui.NewButton("templateNext", 417, 245, 105, 38, i18n.Source("text.log0089"), 11)
	e.fresh = simpleui.NewButton("templateNew", 300, 315, 222, 38, i18n.Source("text.log0158"), 11)
	e.clone = simpleui.NewButton("templateClone", 300, 365, 222, 38, i18n.Source("text.log0159"), 11)
	e.preview = simpleui.NewButton("templatePreview", 300, 415, 222, 38, i18n.Source("text.log0197"), 11)
	e.preview.OnClick(e.previewTemplate)
	e.save = simpleui.NewButton("templateSave", 570, 630, 250, 44, i18n.Source("text.log0125"), 12)
	e.save.SetColors(actionStartFill, colors.green, colors.text)
	e.remove = simpleui.NewButton("templateDelete", 838, 630, 250, 44, i18n.Source("text.log0044"), 12)
	e.remove.SetColors(actionStopFill, colors.red, colors.text)
	e.close = simpleui.NewButton("templateClose", 1285, 134, 110, 40, i18n.Source("text.log0042"), 12)
	e.previous.OnClick(func() { e.move(-1) })
	e.next.OnClick(func() { e.move(1) })
	e.fresh.OnClick(e.newTemplate)
	e.clone.OnClick(e.cloneTemplate)
	e.save.OnClick(e.saveTemplate)
	e.remove.OnClick(e.deleteTemplate)
	e.close.OnClick(e.Close)
	controls := []simpleui.Element{e.name, e.id, e.start, e.end, e.points, e.sentValue, e.mode, e.duplicate, e.sentExchange, e.receivedExchange,
		e.previous, e.next, e.fresh, e.clone, e.preview, e.save, e.remove, e.close}
	for _, button := range e.bandButtons {
		controls = append(controls, button)
	}
	for _, control := range controls {
		e.manager.Add(control)
	}
	return e
}

func (e *ContestTemplateEditor) Open() {
	e.panel.reloadProfiles()
	e.open, e.deleting, e.status = true, false, ""
	e.selected = 0
	for index, profile := range e.panel.profiles {
		if profile.ID == e.panel.profileValue {
			e.selected = index
			break
		}
	}
	e.loadSelected()
}

func (e *ContestTemplateEditor) Close()                     { e.open = false }
func (e *ContestTemplateEditor) OverlayOpen() bool          { return e.open }
func (e *ContestTemplateEditor) Update(simpleui.Input) bool { return false }
func (e *ContestTemplateEditor) Draw()                      {}

func (e *ContestTemplateEditor) UpdateOverlay(input simpleui.Input) bool {
	if !e.open {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		e.Close()
		return true
	}
	e.manager.Update(input)
	return true
}

func (e *ContestTemplateEditor) DrawOverlay() {
	if !e.open {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 215})
	drawPanel(260, 105, 1180, 650)
	simpleui.DrawTextStyled(i18n.Source("text.log0155"), 300, 134, 21, simpleui.FontSemiBold, colors.cyan)
	simpleui.DrawText(i18n.Source("text.log0160"), 300, 164, 12, colors.muted)
	drawPanel(285, 205, 255, 505)
	current := 0
	if e.selected >= 0 {
		current = e.selected + 1
	}
	simpleui.DrawTextStyled(fmt.Sprintf("%d / %d", current, len(e.panel.profiles)), 362, 218, 14, simpleui.FontSemiBold, colors.cyan)
	labels := []struct {
		text string
		x, y float32
	}{
		{i18n.Source("text.log0161"), 570, 225}, {i18n.Source("text.log0162"), 1018, 225},
		{i18n.Source("text.log0163"), 570, 310}, {i18n.Source("text.log0164"), 888, 310},
		{i18n.Source("text.log0165"), 1206, 310}, {i18n.Source("text.log0166"), 570, 396},
		{i18n.Source("text.log0167"), 793, 396}, {i18n.Source("text.log0181"), 570, 460},
		{i18n.Source("text.log0196"), 858, 460}, {i18n.Source("text.log0182"), 1155, 460},
		{i18n.Source("text.log0199"), 570, 530},
	}
	for _, label := range labels {
		simpleui.DrawTextStyled(label.text, label.x, label.y, 11, simpleui.FontSemiBold, colors.muted)
	}
	simpleui.DrawText(i18n.Source("text.log0170"), 570, 600, 11, colors.muted)
	for index, line := range e.previewText {
		simpleui.DrawTextStyled(line, 300, 475+float32(index)*24, 11, simpleui.FontSemiBold, colors.text)
	}
	if e.status != "" {
		simpleui.DrawTextStyled(e.status, 570, 700, 12, simpleui.FontSemiBold, e.statusColor)
	}
	e.manager.Draw()
}

func (e *ContestTemplateEditor) move(delta int) {
	if len(e.panel.profiles) == 0 {
		return
	}
	e.selected = (e.selected + delta + len(e.panel.profiles)) % len(e.panel.profiles)
	e.loadSelected()
}

func (e *ContestTemplateEditor) loadSelected() {
	if e.selected < 0 || e.selected >= len(e.panel.profiles) {
		return
	}
	p := e.panel.profiles[e.selected]
	e.name.SetText(p.Name)
	e.id.SetText(p.ID)
	e.mode.SetLabel(p.Mode)
	e.points.SetText(strconv.Itoa(p.PointsPerQSO))
	e.sentValue.SetText(p.SentValue)
	e.start.SetText(formatTemplateTime(p.Start))
	e.end.SetText(formatTemplateTime(p.End))
	if p.DuplicateRule == "CONTEST" {
		e.duplicate.SetLabel(i18n.Source("text.log0169"))
	} else {
		e.duplicate.SetLabel(i18n.Source("text.log0168"))
	}
	if p.SentExchange == "FIXED" {
		e.sentExchange.SetLabel(i18n.Source("text.log0180"))
	} else {
		e.sentExchange.SetLabel(i18n.Source("text.log0179"))
	}
	e.receivedExchange.SetLabel(receivedExchangeLabel(p.ReceivedExchange))
	e.setBands(p.AllowedBands)
	for _, field := range []*simpleui.TextField{e.name, e.id, e.start, e.end, e.points} {
		field.SetEnabled(!p.BuiltIn)
	}
	e.sentValue.SetEnabled(!p.BuiltIn && p.SentExchange == "FIXED")
	e.mode.SetEnabled(!p.BuiltIn)
	e.duplicate.SetEnabled(!p.BuiltIn)
	e.sentExchange.SetEnabled(!p.BuiltIn)
	e.receivedExchange.SetEnabled(!p.BuiltIn)
	for _, button := range e.bandButtons {
		button.SetEnabled(!p.BuiltIn)
	}
	e.save.SetEnabled(!p.BuiltIn)
	e.remove.SetEnabled(!p.BuiltIn)
	e.clone.SetEnabled(true)
	e.deleting, e.status, e.previewText = false, "", nil
	e.remove.SetLabel(i18n.Source("text.log0044"))
}

func (e *ContestTemplateEditor) newTemplate() {
	e.selected = -1
	e.name.SetText("")
	e.id.SetText("")
	e.start.SetText("")
	e.end.SetText("")
	e.points.SetText("1")
	e.sentValue.SetText("")
	e.mode.SetLabel("SSB")
	e.duplicate.SetLabel(i18n.Source("text.log0168"))
	e.sentExchange.SetLabel(i18n.Source("text.log0179"))
	e.receivedExchange.SetLabel(i18n.Source("text.log0189"))
	e.setBands([]string{"160M", "80M", "40M", "20M", "15M", "10M"})
	for _, field := range []*simpleui.TextField{e.name, e.id, e.start, e.end, e.points} {
		field.SetEnabled(true)
	}
	e.mode.SetEnabled(true)
	e.duplicate.SetEnabled(true)
	e.sentExchange.SetEnabled(true)
	e.receivedExchange.SetEnabled(true)
	for _, button := range e.bandButtons {
		button.SetEnabled(true)
	}
	e.sentValue.SetEnabled(false)
	e.save.SetEnabled(true)
	e.remove.SetEnabled(false)
	e.status = i18n.Source("text.log0171")
	e.statusColor = colors.cyan
	e.name.SetFocused(true)
}

func (e *ContestTemplateEditor) cloneTemplate() {
	if e.selected < 0 || e.selected >= len(e.panel.profiles) {
		return
	}
	p := e.panel.profiles[e.selected]
	e.newTemplate()
	e.name.SetText(p.Name + " " + i18n.Source("text.log0172"))
	e.id.SetText(p.ID + "-COPY")
	e.start.SetText(formatTemplateTime(p.Start))
	e.end.SetText(formatTemplateTime(p.End))
	e.points.SetText(strconv.Itoa(p.PointsPerQSO))
	e.mode.SetLabel(p.Mode)
	e.sentValue.SetText(p.SentValue)
	if p.SentExchange == "FIXED" {
		e.sentExchange.SetLabel(i18n.Source("text.log0180"))
		e.sentValue.SetEnabled(true)
	}
	e.receivedExchange.SetLabel(receivedExchangeLabel(p.ReceivedExchange))
	e.setBands(p.AllowedBands)
	if p.DuplicateRule == "CONTEST" {
		e.duplicate.SetLabel(i18n.Source("text.log0169"))
	}
}

func (e *ContestTemplateEditor) profileFromFields() (contestlog.Profile, error) {
	points, err := strconv.Atoi(e.points.Text())
	if err != nil {
		return contestlog.Profile{}, fmt.Errorf("%s", i18n.Source("text.log0173"))
	}
	start, end := time.Time{}, time.Time{}
	if strings.TrimSpace(e.start.Text()) != "" || strings.TrimSpace(e.end.Text()) != "" {
		start, err = time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(e.start.Text()), time.UTC)
		if err != nil {
			return contestlog.Profile{}, fmt.Errorf("%s", i18n.Source("text.log0174"))
		}
		end, err = time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(e.end.Text()), time.UTC)
		if err != nil {
			return contestlog.Profile{}, fmt.Errorf("%s", i18n.Source("text.log0174"))
		}
	}
	id := strings.ToUpper(strings.TrimSpace(e.id.Text()))
	duplicate := "BAND"
	if e.duplicate.Label() == i18n.Source("text.log0169") {
		duplicate = "CONTEST"
	}
	sent := "SERIAL"
	if e.sentExchange.Label() == i18n.Source("text.log0180") {
		sent = "FIXED"
	}
	return contestlog.Profile{ID: id, Name: strings.TrimSpace(e.name.Text()), CabrilloName: id, Mode: e.mode.Label(),
		Start: start, End: end, DuplicateRule: duplicate, PointsPerQSO: points, SentExchange: sent,
		SentValue: strings.TrimSpace(e.sentValue.Text()), ReceivedExchange: receivedExchangeCode(e.receivedExchange.Label()),
		AllowedBands: e.selectedBands()}, nil
}

func (e *ContestTemplateEditor) saveTemplate() {
	oldID := ""
	if e.selected >= 0 && e.selected < len(e.panel.profiles) && !e.panel.profiles[e.selected].BuiltIn {
		oldID = e.panel.profiles[e.selected].ID
	}
	p, err := e.profileFromFields()
	if err == nil {
		for _, builtIn := range contestlog.Profiles() {
			if builtIn.ID == p.ID {
				err = fmt.Errorf("%s", i18n.Source("text.log0175"))
				break
			}
		}
	}
	if err == nil {
		err = contestlog.SaveProfile(e.panel.templateDir, p)
	}
	if err != nil {
		e.status, e.statusColor = err.Error(), colors.red
		return
	}
	if oldID != "" && oldID != p.ID {
		_ = contestlog.DeleteProfile(e.panel.templateDir, oldID)
	}
	e.panel.profileValue = p.ID
	e.panel.reloadProfiles()
	e.panel.refreshProfileLabel()
	for index, profile := range e.panel.profiles {
		if profile.ID == p.ID {
			e.selected = index
			break
		}
	}
	e.loadSelected()
	e.status, e.statusColor = i18n.Source("text.log0176"), colors.green
}

func (e *ContestTemplateEditor) deleteTemplate() {
	if e.selected < 0 || e.selected >= len(e.panel.profiles) || e.panel.profiles[e.selected].BuiltIn {
		return
	}
	if !e.deleting {
		e.deleting = true
		e.remove.SetLabel(i18n.Source("text.log0177"))
		return
	}
	if err := contestlog.DeleteProfile(e.panel.templateDir, e.panel.profiles[e.selected].ID); err != nil {
		e.status, e.statusColor = err.Error(), colors.red
		return
	}
	e.panel.profileValue = contestlog.OceaniaDXSSB
	e.panel.reloadProfiles()
	e.panel.refreshProfileLabel()
	e.selected = 0
	e.loadSelected()
	e.status, e.statusColor = i18n.Source("text.log0178"), colors.orange
}

func formatTemplateTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format("2006-01-02 15:04")
}

func receivedExchangeLabels() []string {
	return []string{i18n.Source("text.log0189"), i18n.Source("text.log0190"), i18n.Source("text.log0191"),
		i18n.Source("text.log0192"), i18n.Source("text.log0193"), i18n.Source("text.log0194"), i18n.Source("text.log0195")}
}

func receivedExchangeLabel(code string) string {
	keys := map[string]string{"SERIAL": "text.log0189", "CQ_ZONE": "text.log0190", "ITU_ZONE": "text.log0191",
		"SECTION": "text.log0192", "LOCATOR": "text.log0193", "NUMBER": "text.log0194", "TEXT": "text.log0195"}
	if key := keys[code]; key != "" {
		return i18n.Source(key)
	}
	return i18n.Source("text.log0189")
}

func receivedExchangeCode(label string) string {
	codes := []string{"SERIAL", "CQ_ZONE", "ITU_ZONE", "SECTION", "LOCATOR", "NUMBER", "TEXT"}
	for index, candidate := range receivedExchangeLabels() {
		if label == candidate {
			return codes[index]
		}
	}
	return "SERIAL"
}

func (e *ContestTemplateEditor) setBands(selected []string) {
	set := map[string]bool{}
	for _, band := range selected {
		set[band] = true
	}
	if len(set) == 0 {
		for _, band := range []string{"160M", "80M", "40M", "20M", "15M", "10M"} {
			set[band] = true
		}
	}
	for _, button := range e.bandButtons {
		band := strings.TrimPrefix(strings.TrimPrefix(button.Label(), "[X] "), "[ ] ")
		prefix := "[ ] "
		if set[band] {
			prefix = "[X] "
		}
		button.SetLabel(prefix + band)
	}
}

func (e *ContestTemplateEditor) selectedBands() []string {
	var bands []string
	for _, button := range e.bandButtons {
		if strings.HasPrefix(button.Label(), "[X]") {
			bands = append(bands, strings.TrimPrefix(button.Label(), "[X] "))
		}
	}
	return bands
}

func (e *ContestTemplateEditor) toggleBand(band string) {
	for _, button := range e.bandButtons {
		if strings.HasSuffix(button.Label(), band) {
			if strings.HasPrefix(button.Label(), "[X]") {
				if len(e.selectedBands()) > 1 {
					button.SetLabel("[ ] " + band)
				}
			} else {
				button.SetLabel("[X] " + band)
			}
			return
		}
	}
}

func (e *ContestTemplateEditor) previewTemplate() {
	profile, err := e.profileFromFields()
	if err != nil || contestlog.ValidateProfile(profile) != nil {
		e.previewText = []string{i18n.Source("text.log0200")}
		return
	}
	mode := map[string]string{"SSB": "PH", "CW": "CW", "DIGI": "DG"}[profile.Mode]
	sent := "001"
	if profile.SentExchange == "FIXED" {
		sent = strings.ToUpper(profile.SentValue)
	}
	received := map[string]string{"SERIAL": "014", "CQ_ZONE": "14", "ITU_ZONE": "37", "SECTION": "BI", "LOCATOR": "IN80", "NUMBER": "25", "TEXT": "ABC"}[profile.ReceivedExchange]
	e.previewText = []string{i18n.Source("text.log0201"), "EA1ABC 59/" + sent, "EA2XYZ 59/" + received,
		fmt.Sprintf("QSO: 14200 %s ...", mode), fmt.Sprintf(i18n.Source("text.log0202"), profile.PointsPerQSO)}
}
