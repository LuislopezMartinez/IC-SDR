package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go-zero/internal/contestlog"
	"go-zero/internal/i18n"
	"go-zero/internal/localtime"
	"go-zero/internal/resources"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const contestLogToolID = "CONTEST_LOG"

type ContestLogPanel struct {
	screen                                                                    *MainScreen
	controls                                                                  []simpleui.Element
	stationCall, operator, sentValue, location, workedCall                    *simpleui.TextField
	rstSent, rstReceived, serialReceived                                      *simpleui.TextField
	create, profile, editTemplates, power, save, export, openFolder           *simpleui.Button
	viewLog, editLast, finish, history                                        *simpleui.Button
	operatorCategory, assisted, categoryBand, transmitter                     *simpleui.Button
	viewer                                                                    *ContestLogViewer
	templateEditor                                                            *ContestTemplateEditor
	session                                                                   *contestlog.Session
	path, archiveDir, exportDir, backupDir, templateDir, status               string
	profiles                                                                  []contestlog.Profile
	statusColor                                                               rl.Color
	pendingQSO                                                                string
	profileValue                                                              string
	autoSentValue                                                             string
	workedIdentity                                                            contestlog.CallIdentity
	hasWorkedIdentity                                                         bool
	workedLocalMinute                                                         int64
	workedLocalLabel                                                          string
	identityCache                                                             map[string]contestlog.CallIdentity
	duplicatePreview, duplicateSignature                                      string
	duplicatePreviewLevel                                                     int
	operatorCategoryValue, assistedValue, categoryBandValue, transmitterValue string
}

type contestScoreProjection struct {
	current, projected contestlog.Score
}

func NewContestLogPanel(screen *MainScreen) *ContestLogPanel {
	p := &ContestLogPanel{screen: screen, path: resources.WritablePath("logbook", "oceania-dx-ssb-2026.json"), identityCache: make(map[string]contestlog.CallIdentity),
		archiveDir: resources.WritablePath("logbook", "archive"), exportDir: resources.WritablePath("exports", "cabrillo"),
		backupDir: resources.WritablePath("logbook", "backups"), templateDir: resources.WritablePath("logbook", "templates"), statusColor: colors.muted}
	p.reloadProfiles()
	p.stationCall = simpleui.NewTextField("contestStationCall", 382, toolY+58, 160, 40, i18n.Source("text.log0001"), 16)
	p.operator = simpleui.NewTextField("contestOperator", 552, toolY+58, 145, 40, i18n.Source("text.log0002"), 13)
	p.sentValue = simpleui.NewTextField("contestSentValue", 707, toolY+58, 90, 40, "CQ zone", 13)
	p.location = simpleui.NewTextField("contestLocation", 807, toolY+58, 90, 40, "Location", 11)
	p.location.SetText("DX")
	p.power = simpleui.NewButton("contestPower", 907, toolY+58, 120, 40, i18n.Source("text.log0003"), 11)
	p.power.OnClick(p.cyclePower)
	p.create = simpleui.NewButton("contestCreate", 1037, toolY+58, 181, 40, i18n.Source("text.log0004"), 13)
	p.create.SetColors(actionStartFill, colors.green, colors.text)
	p.create.OnClick(p.createSession)
	p.profile = simpleui.NewButton("contestProfile", 382, toolY+29, 836, 24, "", 11)
	p.profile.OnClick(p.cycleProfile)
	p.editTemplates = simpleui.NewButton("contestEditTemplates", 1230, toolY+29, 327, 24, i18n.Source("text.log0155"), 10)
	p.editTemplates.OnClick(func() { p.templateEditor.Open() })

	p.workedCall = simpleui.NewTextField("contestWorkedCall", 382, toolY+51, 190, 40, i18n.Source("text.log0005"), 17)
	p.rstSent = simpleui.NewTextField("contestRSTSent", 632, toolY+56, 52, 35, "59", 15)
	p.rstReceived = simpleui.NewTextField("contestRSTReceived", 744, toolY+56, 52, 35, "59", 15)
	p.serialReceived = simpleui.NewTextField("contestSerialReceived", 856, toolY+56, 130, 35, "000", 15)
	p.save = simpleui.NewButton("contestSave", 998, toolY+51, 150, 40, i18n.Source("text.log0009"), 12)
	p.save.SetColors(actionStartFill, colors.green, colors.text)
	p.save.OnClick(p.saveQSO)
	p.viewLog = simpleui.NewButton("contestViewLog", 382, toolY+99, 190, 32, i18n.Source("text.log0010"), 11)
	p.viewLog.OnClick(func() { p.viewer.Open(-1) })
	p.editLast = simpleui.NewButton("contestEditLast", 584, toolY+99, 180, 32, i18n.Source("text.log0011"), 11)
	p.editLast.OnClick(func() {
		if p.session != nil && len(p.session.QSOs) > 0 {
			p.viewer.Open(len(p.session.QSOs) - 1)
		}
	})
	p.finish = simpleui.NewButton("contestFinish", 776, toolY+99, 180, 32, i18n.Source("text.log0085"), 11)
	p.finish.SetColors(colors.panelAlt, colors.orange, colors.text)
	p.finish.OnClick(func() { p.viewer.OpenFinish() })
	p.history = simpleui.NewButton("contestHistory", 1377, toolY+99, 180, 32, i18n.Source("text.log0141"), 11)
	p.history.OnClick(func() { p.viewer.OpenHistory() })
	p.operatorCategoryValue, p.assistedValue, p.categoryBandValue, p.transmitterValue = "SINGLE-OP", "NON-ASSISTED", "ALL", "ONE"
	p.profileValue = contestlog.CQWWSSB
	p.refreshProfileLabel()
	p.operatorCategory = simpleui.NewButton("contestOperatorCategory", 382, toolY+108, 165, 32, "", 10)
	p.assisted = simpleui.NewButton("contestAssisted", 557, toolY+108, 165, 32, "", 10)
	p.categoryBand = simpleui.NewButton("contestCategoryBand", 732, toolY+108, 145, 32, "", 10)
	p.transmitter = simpleui.NewButton("contestTransmitter", 887, toolY+108, 175, 32, "", 10)
	p.operatorCategory.OnClick(func() {
		p.operatorCategoryValue = cycleChoice(p.operatorCategoryValue, []string{"SINGLE-OP", "MULTI-OP"})
		p.refreshConfigLabels()
	})
	p.assisted.OnClick(func() {
		p.assistedValue = cycleChoice(p.assistedValue, []string{"NON-ASSISTED", "ASSISTED"})
		p.refreshConfigLabels()
	})
	p.categoryBand.OnClick(func() {
		choices := []string{"ALL"}
		for _, profile := range p.profiles {
			if profile.ID == p.profileValue {
				choices = append(choices, profile.AllowedBands...)
				break
			}
		}
		p.categoryBandValue = cycleChoice(p.categoryBandValue, choices)
		p.refreshConfigLabels()
	})
	p.transmitter.OnClick(func() {
		p.transmitterValue = cycleChoice(p.transmitterValue, []string{"ONE", "TWO", "UNLIMITED"})
		p.refreshConfigLabels()
	})
	p.refreshConfigLabels()
	p.export = simpleui.NewButton("contestExport", 1270, toolY+51, 175, 40, i18n.Source("text.log0205"), 12)
	p.export.SetColors(colors.blue, colors.cyan, colors.text)
	p.export.OnClick(func() { p.viewer.Open(-1) })
	p.openFolder = simpleui.NewButton("contestOpenFolder", 1457, toolY+51, 110, 40, i18n.Source("text.log0013"), 9)
	p.openFolder.OnClick(func() { openExplorerPath(p.exportDir) })

	callFilter := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '/' }
	digitFilter := func(r rune) bool { return unicode.IsDigit(r) }
	for _, field := range []*simpleui.TextField{p.stationCall, p.operator, p.workedCall} {
		field.SetFilter(callFilter)
		field.SetMaxLength(16)
		field.SetFont(simpleui.FontSemiBold)
	}
	p.sentValue.SetFilter(digitFilter)
	p.sentValue.SetMaxLength(2)
	p.stationCall.OnChange(func(string) { p.autofillCQZone() })
	p.sentValue.OnChange(func(value string) {
		if value != p.autoSentValue {
			p.autoSentValue = ""
		}
	})
	p.location.SetFilter(func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' })
	p.location.SetMaxLength(8)
	for _, field := range []*simpleui.TextField{p.rstSent, p.rstReceived} {
		field.SetFilter(digitFilter)
		field.SetMaxLength(4)
		field.SetFont(simpleui.FontSemiBold)
	}
	p.workedCall.OnSubmit(func(string) { p.serialReceived.SetFocused(true) })
	p.workedCall.OnChange(func(value string) {
		p.updateWorkedIdentity(value)
		p.duplicateSignature = ""
		p.updateDuplicatePreview()
	})
	p.serialReceived.OnSubmit(func(string) { p.saveQSO() })
	p.controls = []simpleui.Element{p.stationCall, p.operator, p.sentValue, p.location, p.power, p.create, p.profile, p.editTemplates, p.workedCall, p.rstSent,
		p.rstReceived, p.serialReceived, p.save, p.viewLog, p.editLast, p.finish, p.history, p.operatorCategory,
		p.assisted, p.categoryBand, p.transmitter, p.export, p.openFolder}
	if loaded, err := contestlog.Load(p.path); err == nil {
		p.session = loaded
	} else if !os.IsNotExist(err) {
		p.setStatus(fmt.Sprintf(i18n.Source("text.log0070"), err.Error()), colors.red)
	}
	p.rstSent.SetText("59")
	p.rstReceived.SetText("59")
	p.refreshExchangeField()
	p.viewer = NewContestLogViewer(p)
	p.templateEditor = NewContestTemplateEditor(p)
	p.refreshVisibility(false)
	return p
}

func (p *ContestLogPanel) cycleProfile() {
	profiles := p.profiles
	current := p.profileValue
	for index, profile := range profiles {
		if profile.ID == current {
			p.profileValue = profiles[(index+1)%len(profiles)].ID
			p.categoryBandValue = "ALL"
			p.refreshProfileLabel()
			p.refreshConfigLabels()
			p.refreshSentValueVisibility()
			return
		}
	}
	p.profileValue = contestlog.CQWWSSB
	p.refreshProfileLabel()
	p.refreshSentValueVisibility()
}

func (p *ContestLogPanel) reloadProfiles() {
	profiles, err := contestlog.LoadProfiles(p.templateDir)
	if err != nil || len(profiles) == 0 {
		profiles = contestlog.Profiles()
	}
	p.profiles = profiles
	found := false
	for _, profile := range profiles {
		found = found || profile.ID == p.profileValue
	}
	if !found {
		p.profileValue = contestlog.CQWWSSB
	}
}

func (p *ContestLogPanel) refreshProfileLabel() {
	for _, profile := range p.profiles {
		if profile.ID == p.profileValue {
			label := fmt.Sprintf(i18n.Source("text.log0156"), profile.Name, profile.Mode)
			if profile.OfficialScoring {
				label += i18n.Source("text.log0157")
			}
			p.profile.SetLabel(label)
			return
		}
	}
	p.profile.SetLabel(i18n.Source("text.log0154"))
}

func (p *ContestLogPanel) selectedProfile() (contestlog.Profile, bool) {
	for _, profile := range p.profiles {
		if profile.ID == p.profileValue {
			return profile, true
		}
	}
	return contestlog.Profile{}, false
}

func (p *ContestLogPanel) refreshSentValueVisibility() {
	if p.sentValue == nil {
		return
	}
	profile, ok := p.selectedProfile()
	p.sentValue.SetVisible(ok && profile.SentExchange == "FIXED")
	if ok && (profile.ID == contestlog.CQWWSSB || profile.ID == contestlog.CQWWCW || profile.ID == contestlog.JIDXSSB) {
		p.operatorCategoryValue, p.transmitterValue = "SINGLE-OP", "ONE"
		p.refreshConfigLabels()
		p.autofillCQZone()
	}
}

func (p *ContestLogPanel) autofillCQZone() {
	profile, ok := p.selectedProfile()
	if !ok || (profile.ID != contestlog.CQWWSSB && profile.ID != contestlog.CQWWCW && profile.ID != contestlog.JIDXSSB) {
		return
	}
	current := strings.TrimSpace(p.sentValue.Text())
	if current != "" && current != p.autoSentValue {
		return
	}
	zone, ok := contestlog.CQZoneForCall(p.stationCall.Text())
	if !ok {
		if current == p.autoSentValue {
			p.sentValue.SetText("")
			p.autoSentValue = ""
		}
		return
	}
	p.autoSentValue = strconv.Itoa(zone)
	p.sentValue.SetText(p.autoSentValue)
}

func (p *ContestLogPanel) updateWorkedIdentity(call string) {
	p.workedIdentity, p.hasWorkedIdentity = contestlog.LookupCallIdentity(call)
	p.workedLocalMinute, p.workedLocalLabel = -1, ""
}

func (p *ContestLogPanel) updateDuplicatePreview() {
	if p.session == nil {
		p.clearDuplicatePreview()
		return
	}
	call := contestlog.NormalizeCall(p.workedCall.Text())
	band := contestlog.BandForFrequency(p.screen.frequencyHz)
	signature := fmt.Sprintf("%s|%s|%d", call, band, len(p.session.QSOs))
	if signature == p.duplicateSignature {
		return
	}
	p.duplicateSignature = signature
	if call == "" {
		p.clearDuplicatePreview()
		return
	}
	type prior struct {
		band string
		when time.Time
	}
	byBand := map[string]time.Time{}
	duplicate := false
	for _, qso := range p.session.QSOs {
		if contestlog.NormalizeCall(qso.WorkedCall) != call {
			continue
		}
		if previous, ok := byBand[qso.Band]; !ok || qso.Time.After(previous) {
			byBand[qso.Band] = qso.Time
		}
		if p.session.DuplicateRule == "CONTEST" || (band != "" && qso.Band == band) {
			duplicate = true
		}
	}
	if len(byBand) == 0 {
		p.clearDuplicatePreview()
		return
	}
	order := []string{"160M", "80M", "40M", "20M", "15M", "10M"}
	contacts := make([]prior, 0, len(byBand))
	for _, candidate := range order {
		if when, ok := byBand[candidate]; ok {
			contacts = append(contacts, prior{band: candidate, when: when})
		}
	}
	details := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		details = append(details, fmt.Sprintf("%s %s UTC", contact.band, contact.when.UTC().Format("15:04")))
	}
	if duplicate {
		p.duplicatePreviewLevel = 2
		p.duplicatePreview = i18n.Source("text.log0102") + " · " + strings.Join(details, " · ")
		p.workedCall.SetBorderColor(colors.red)
		p.save.SetColors(colors.panelAlt, colors.red, colors.text)
		return
	}
	p.duplicatePreviewLevel = 1
	p.duplicatePreview = "OTRA BANDA · " + strings.Join(details, " · ")
	p.workedCall.SetBorderColor(colors.orange)
	p.save.SetColors(actionStartFill, colors.green, colors.text)
}

func (p *ContestLogPanel) clearDuplicatePreview() {
	p.duplicatePreview, p.duplicatePreviewLevel = "", 0
	p.workedCall.ClearBorderColor()
	p.save.SetColors(actionStartFill, colors.green, colors.text)
}

func (p *ContestLogPanel) receivedExchangeLabel() string {
	if p.session == nil {
		return i18n.Source("text.log0111")
	}
	if p.session.Contest == contestlog.JIDXSSB {
		return i18n.Source("text.log0223")
	}
	keys := map[string]string{"SERIAL": "text.log0111", "CQ_ZONE": "text.log0183", "ITU_ZONE": "text.log0184",
		"SECTION": "text.log0185", "LOCATOR": "text.log0186", "NUMBER": "text.log0187", "TEXT": "text.log0188"}
	if key := keys[p.session.ReceivedExchange]; key != "" {
		return i18n.Source(key)
	}
	return i18n.Source("text.log0111")
}

func (p *ContestLogPanel) refreshExchangeField() {
	if p.serialReceived == nil {
		return
	}
	p.serialReceived.SetPlaceholder(p.receivedExchangeLabel())
	if p.session != nil && (p.session.ContestMode == "CW" || p.session.ContestMode == "DIGI") {
		p.rstSent.SetText("599")
		p.rstReceived.SetText("599")
	}
}

func (p *ContestLogPanel) cyclePower() {
	current := strings.TrimPrefix(p.power.Label(), i18n.Source("text.log0014"))
	next := map[string]string{"LOW": "HIGH", "HIGH": "QRP", "QRP": "LOW"}[current]
	if next == "" {
		next = "LOW"
	}
	p.power.SetLabel(i18n.Source("text.log0014") + next)
}

func cycleChoice(current string, choices []string) string {
	for index, value := range choices {
		if value == current {
			return choices[(index+1)%len(choices)]
		}
	}
	return choices[0]
}

func (p *ContestLogPanel) refreshConfigLabels() {
	p.operatorCategory.SetLabel(fmt.Sprintf(i18n.Source("text.log0098"), p.operatorCategoryValue))
	p.assisted.SetLabel(fmt.Sprintf(i18n.Source("text.log0099"), p.assistedValue))
	p.categoryBand.SetLabel(fmt.Sprintf(i18n.Source("text.log0100"), p.categoryBandValue))
	p.transmitter.SetLabel(fmt.Sprintf(i18n.Source("text.log0101"), p.transmitterValue))
}

func (p *ContestLogPanel) createSession() {
	power := strings.TrimPrefix(p.power.Label(), i18n.Source("text.log0014"))
	var selected contestlog.Profile
	for _, profile := range p.profiles {
		if profile.ID == p.profileValue {
			selected = profile
			break
		}
	}
	session, err := contestlog.NewSessionFromProfile(selected, p.stationCall.Text(), p.operator.Text(), contestlog.Config{
		Power: power, OperatorCategory: p.operatorCategoryValue, Assisted: p.assistedValue,
		Band: p.categoryBandValue, Transmitter: p.transmitterValue, SentValue: p.sentValue.Text(), Location: p.location.Text()}, time.Now())
	if err != nil {
		p.setStatus(err.Error(), colors.red)
		return
	}
	if err := p.saveActive(session); err != nil {
		p.setStatus(fmt.Sprintf(i18n.Source("text.log0015"), err.Error()), colors.red)
		return
	}
	p.session = session
	p.refreshExchangeField()
	p.pendingQSO = ""
	p.save.SetLabel(i18n.Source("text.log0009"))
	p.setStatus(i18n.Source("text.log0016"), colors.green)
	p.refreshVisibility(true)
	p.workedCall.SetFocused(true)
}

func (p *ContestLogPanel) saveQSO() {
	exchange := strings.TrimSpace(p.serialReceived.Text())
	if exchange == "" {
		p.setStatus(i18n.Source("text.log0017"), colors.orange)
		return
	}
	serialNumber, _ := strconv.Atoi(exchange)
	mode := p.screen.savedMode
	if p.screen.mode != nil {
		mode = p.screen.mode.SelectedText()
	}
	entry := contestlog.Entry{Time: time.Now(), FrequencyHz: p.screen.frequencyHz, Mode: mode,
		WorkedCall: p.workedCall.Text(), RSTSent: p.rstSent.Text(), RSTReceived: p.rstReceived.Text(), SerialReceived: serialNumber, ExchangeReceived: exchange}
	if contestlog.NormalizeCall(entry.WorkedCall) == "" {
		p.setStatus(i18n.Source("text.log0077"), colors.red)
		return
	}
	if contestlog.BandForFrequency(entry.FrequencyHz) == "" {
		p.setStatus(i18n.Source("text.log0079"), colors.red)
		return
	}
	signature := fmt.Sprintf("%s|%d|%s|%s|%s|%s", contestlog.NormalizeCall(entry.WorkedCall), entry.FrequencyHz, mode, entry.RSTSent, entry.RSTReceived, entry.ExchangeReceived)
	warnings := p.session.Warnings(entry)
	if len(warnings) > 0 && p.pendingQSO != signature {
		labels := make([]string, 0, len(warnings))
		for _, warning := range warnings {
			switch warning {
			case contestlog.WarningDuplicate:
				labels = append(labels, i18n.Source("text.log0102"))
			case contestlog.WarningMode:
				labels = append(labels, i18n.Source("text.log0103"))
			case contestlog.WarningTime:
				labels = append(labels, i18n.Source("text.log0104"))
			case contestlog.WarningBand:
				labels = append(labels, i18n.Source("text.log0198"))
			}
		}
		p.pendingQSO = signature
		p.save.SetLabel(i18n.Source("text.log0105"))
		p.setStatus(fmt.Sprintf(i18n.Source("text.log0106"), strings.Join(labels, " · ")), colors.orange)
		return
	}
	qso, err := p.session.Add(entry)
	if err != nil {
		p.setStatus(err.Error(), colors.red)
		return
	}
	if err := p.saveActive(p.session); err != nil {
		p.session.QSOs = p.session.QSOs[:len(p.session.QSOs)-1]
		p.session.NextSerial--
		p.setStatus(fmt.Sprintf(i18n.Source("text.log0018"), err.Error()), colors.red)
		return
	}
	message := fmt.Sprintf(i18n.Source("text.log0019"), qso.SerialSent, qso.WorkedCall, qso.Band)
	color := colors.green
	if qso.Duplicate {
		message += i18n.Source("text.log0020")
		color = colors.orange
	}
	p.setStatus(message, color)
	p.workedCall.SetText("")
	p.updateWorkedIdentity("")
	p.duplicateSignature = ""
	p.updateDuplicatePreview()
	p.serialReceived.SetText("")
	p.pendingQSO = ""
	p.save.SetLabel(i18n.Source("text.log0009"))
	p.workedCall.SetFocused(true)
}

func (p *ContestLogPanel) exportADIFSession(session *contestlog.Session) (string, error) {
	if session == nil {
		return "", fmt.Errorf("%s", i18n.Source("text.log0076"))
	}
	name := contestExportBaseName(session) + ".adi"
	path := filepath.Join(p.exportDir, name)
	if err := contestlog.ExportADIF(path, session); err != nil {
		return "", err
	}
	return path, nil
}

func (p *ContestLogPanel) exportSession(session *contestlog.Session) (string, error) {
	if session == nil {
		return "", fmt.Errorf("%s", i18n.Source("text.log0076"))
	}
	name := contestExportBaseName(session) + ".log"
	path := filepath.Join(p.exportDir, name)
	if err := contestlog.ExportCabrillo(path, session); err != nil {
		return "", err
	}
	return path, nil
}

func contestExportBaseName(session *contestlog.Session) string {
	if session == nil {
		return "contest"
	}
	parts := []string{session.StationCall, session.Contest}
	if !session.CreatedAt.IsZero() {
		parts = append(parts, session.CreatedAt.UTC().Format("20060102"))
	}
	var name strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.Join(parts, "-")) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			name.WriteRune(r)
			dash = false
		} else if !dash && name.Len() > 0 {
			name.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(name.String(), "-")
}

func (p *ContestLogPanel) archivePaths() []string {
	paths, _ := filepath.Glob(filepath.Join(p.archiveDir, "*.json"))
	slices.Sort(paths)
	slices.Reverse(paths)
	return paths
}

func (p *ContestLogPanel) finishSession() error {
	if p.session == nil {
		return fmt.Errorf("%s", i18n.Source("text.log0076"))
	}
	if issues := p.session.ValidateSubmission(); len(issues) > 0 {
		return fmt.Errorf(i18n.Source("text.log0117"), len(issues))
	}
	snapshot := *p.session
	snapshot.QSOs = append([]contestlog.QSO(nil), p.session.QSOs...)
	snapshot.Close(time.Now())
	if _, err := p.exportSession(&snapshot); err != nil {
		return err
	}
	stamp := snapshot.ClosedAt.UTC().Format("20060102-150405")
	call := strings.NewReplacer("/", "-", "\\", "-").Replace(snapshot.StationCall)
	archivePath := filepath.Join(p.archiveDir, stamp+"-"+call+".json")
	if err := contestlog.Save(archivePath, &snapshot); err != nil {
		return err
	}
	if err := os.Remove(p.path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(archivePath)
		return err
	}
	p.stationCall.SetText(snapshot.StationCall)
	p.operator.SetText(snapshot.Operator)
	p.session = nil
	p.pendingQSO = ""
	p.save.SetLabel(i18n.Source("text.log0009"))
	p.refreshVisibility(true)
	return nil
}

func (p *ContestLogPanel) reopenArchive(path string) error {
	if p.session != nil {
		return fmt.Errorf("%s", i18n.Source("text.log0087"))
	}
	session, err := contestlog.Load(path)
	if err != nil {
		return err
	}
	session.Reopen()
	if err := p.saveActive(session); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		_ = os.Remove(p.path)
		return err
	}
	p.session = session
	p.pendingQSO = ""
	p.refreshExchangeField()
	p.save.SetLabel(i18n.Source("text.log0009"))
	p.refreshVisibility(true)
	return nil
}

func (p *ContestLogPanel) saveActive(session *contestlog.Session) error {
	if data, err := os.ReadFile(p.path); err == nil {
		if err := os.MkdirAll(p.backupDir, 0o755); err == nil {
			name := "contest-" + time.Now().UTC().Format("20060102-150405.000000000") + ".json"
			_ = os.WriteFile(filepath.Join(p.backupDir, name), data, 0o644)
			backups, _ := filepath.Glob(filepath.Join(p.backupDir, "contest-*.json"))
			slices.Sort(backups)
			for len(backups) > 20 {
				_ = os.Remove(backups[0])
				backups = backups[1:]
			}
		}
	}
	return contestlog.Save(p.path, session)
}

func (p *ContestLogPanel) setStatus(message string, color rl.Color) {
	p.status, p.statusColor = message, color
}
func (p *ContestLogPanel) Enter() {
	if p.session != nil {
		p.duplicateSignature = ""
		p.updateDuplicatePreview()
		p.workedCall.SetFocused(true)
	}
}
func (p *ContestLogPanel) Leave() {
	for _, f := range []*simpleui.TextField{p.stationCall, p.operator, p.workedCall, p.rstSent, p.rstReceived, p.serialReceived} {
		f.SetFocused(false)
	}
}
func (p *ContestLogPanel) Close() {
	p.Leave()
	if p.viewer != nil {
		p.viewer.Close()
	}
	if p.templateEditor != nil {
		p.templateEditor.Close()
	}
}
func (p *ContestLogPanel) SetVisible(visible bool) { p.refreshVisibility(visible) }
func (p *ContestLogPanel) Tick()                   { p.updateDuplicatePreview() }

func (p *ContestLogPanel) refreshVisibility(panelVisible bool) {
	creating := p.session == nil
	for _, control := range []simpleui.Element{p.stationCall, p.operator, p.location, p.power, p.create, p.profile, p.editTemplates, p.operatorCategory, p.assisted, p.categoryBand, p.transmitter} {
		control.SetVisible(panelVisible && creating)
	}
	profile, ok := p.selectedProfile()
	p.sentValue.SetVisible(panelVisible && creating && ok && profile.SentExchange == "FIXED")
	singleOperator := ok && (profile.ID == contestlog.CQWWSSB || profile.ID == contestlog.CQWWCW || profile.ID == contestlog.JIDXSSB)
	p.operatorCategory.SetEnabled(!singleOperator)
	p.transmitter.SetEnabled(!singleOperator)
	if p.history != nil {
		p.history.SetVisible(panelVisible)
		p.history.SetEnabled(p.session != nil || len(p.archivePaths()) > 0)
	}
	for _, control := range []simpleui.Element{p.workedCall, p.rstSent, p.rstReceived, p.serialReceived, p.save, p.viewLog, p.editLast, p.finish, p.export, p.openFolder} {
		control.SetVisible(panelVisible && !creating)
	}
}

func (p *ContestLogPanel) DrawPanel() {
	drawPanel(toolContentX, toolY, toolContentRight-toolContentX, toolH)
	simpleui.DrawTextStyled(i18n.Source("text.log0023"), 378, toolY+12, 16, simpleui.FontSemiBold, colors.cyan)
	if p.session == nil {
		simpleui.DrawText(i18n.Source("text.log0024"), 382, toolY+149, 11, colors.muted)
		if p.status != "" {
			simpleui.DrawText(p.status, 382, toolY+169, 11, p.statusColor)
		}
		return
	}
	band := contestlog.BandForFrequency(p.screen.frequencyHz)
	if band == "" {
		band = i18n.Source("text.log0025")
	}
	summary := fmt.Sprintf(i18n.Source("text.log0026"), p.session.StationCall, p.session.Contest, len(p.session.QSOs), p.session.NextSerial)
	if p.session.SentExchange == "FIXED" {
		summary = fmt.Sprintf(i18n.Source("text.log0225"), p.session.StationCall, p.session.Contest, len(p.session.QSOs))
	}
	simpleui.DrawText(fitContestLabel(summary, 218, 12), 382, toolY+34, 12, colors.text)
	p.drawScorePanel(time.Now())
	p.drawWorkedIdentity()
	simpleui.DrawText(fmt.Sprintf(i18n.Source("text.log0027"), float64(p.screen.frequencyHz)/1e6, band, time.Now().UTC().Format("15:04:05")), 1370, toolY+34, 11, colors.cyan)
	serialBadge := rl.Rectangle{X: 1160, Y: toolY + 51, Width: 100, Height: 40}
	badgeFill := blendRGBA(colors.panelAlt, colors.orange, .35)
	rl.DrawRectangleRounded(serialBadge, .16, 8, badgeFill)
	rl.DrawRectangleRoundedLinesEx(serialBadge, .16, 8, 1.5, colors.orange)
	sentValue := strconv.Itoa(p.session.NextSerial)
	if p.session.SentExchange == "FIXED" {
		sentValue = p.session.SentValue
	}
	serialText := fmt.Sprintf(i18n.Source("text.log0224"), sentValue)
	serialSize := simpleui.MeasureTextStyled(serialText, 13, simpleui.FontSemiBold)
	simpleui.DrawTextStyled(serialText, serialBadge.X+(serialBadge.Width-serialSize.X)/2, serialBadge.Y+(serialBadge.Height-serialSize.Y)/2, 13, simpleui.FontSemiBold, colors.orange)
	fieldLabels := []struct {
		text string
		x    float32
	}{{i18n.Source("text.log0109"), 632}, {i18n.Source("text.log0110"), 744}, {p.receivedExchangeLabel(), 856}}
	for _, label := range fieldLabels {
		simpleui.DrawTextStyled(label.text, label.x, toolY+64, 10, simpleui.FontSemiBold, colors.muted)
	}
	if p.duplicatePreview != "" {
		color := colors.orange
		if p.duplicatePreviewLevel == 2 {
			color = colors.red
		}
		simpleui.DrawTextStyled(p.duplicatePreview, 970, toolY+108, 12, simpleui.FontSemiBold, color)
	} else if p.status != "" {
		simpleui.DrawTextStyled(p.status, 970, toolY+108, 12, simpleui.FontSemiBold, p.statusColor)
	}
	p.drawRows()
}

func (p *ContestLogPanel) scoreProjection(now time.Time) (contestScoreProjection, bool) {
	if p.session == nil || (p.session.Contest != contestlog.CQWWSSB && p.session.Contest != contestlog.CQWWCW && p.session.Contest != contestlog.JIDXSSB) || !p.hasWorkedIdentity {
		return contestScoreProjection{}, false
	}
	zone, err := strconv.Atoi(strings.TrimSpace(p.serialReceived.Text()))
	maximum := 40
	if p.session.Contest == contestlog.JIDXSSB {
		maximum = 50
		if !contestlog.IsJapaneseContestCall(p.workedCall.Text()) {
			return contestScoreProjection{}, false
		}
	}
	if err != nil || zone < 1 || zone > maximum || contestlog.BandForFrequency(p.screen.frequencyHz) == "" {
		return contestScoreProjection{}, false
	}
	mode := p.screen.savedMode
	if p.screen.mode != nil {
		mode = p.screen.mode.SelectedText()
	}
	entry := contestlog.Entry{Time: now, FrequencyHz: p.screen.frequencyHz, Mode: mode, WorkedCall: p.workedCall.Text(),
		RSTSent: p.rstSent.Text(), RSTReceived: p.rstReceived.Text(), SerialReceived: zone, ExchangeReceived: strconv.Itoa(zone)}
	preview := *p.session
	preview.QSOs = append([]contestlog.QSO(nil), p.session.QSOs...)
	qso, err := preview.Add(entry)
	if err != nil || qso.Duplicate || len(qso.Warnings) > 0 {
		return contestScoreProjection{}, false
	}
	return contestScoreProjection{current: p.session.Score(), projected: preview.Score()}, true
}

func contestProjectionFlashOn(now time.Time) bool {
	return (now.UnixMilli()/450)%2 == 0
}

func (p *ContestLogPanel) drawScorePanel(now time.Time) {
	const x, width, rowHeight = float32(605), float32(315), float32(22)
	const scoreFontSize = int32(12)
	y := toolY + 5
	violet := rl.Color{R: 169, G: 92, B: 255, A: 255}
	lavender := rl.Color{R: 205, G: 174, B: 255, A: 255}
	current := p.session.Score()
	currentBounds := rl.Rectangle{X: x, Y: y, Width: width, Height: rowHeight}
	currentFill := blendRGBA(colors.panelAlt, colors.blue, .42)
	rl.DrawRectangleRounded(currentBounds, .22, 6, currentFill)
	rl.DrawRectangleRoundedLinesEx(currentBounds, .22, 6, 1.5, colors.cyan)
	simpleui.DrawTextStyled("ACTUAL", x+7, y+5, scoreFontSize, simpleui.FontSemiBold, colors.cyan)
	currentText := fmt.Sprintf("PTS %d · MULT %d · TOTAL %d", current.Points, current.Multipliers, current.Total)
	simpleui.DrawTextStyled(fitContestLabel(currentText, 250, scoreFontSize), x+60, y+5, scoreFontSize, simpleui.FontSemiBold, colors.text)

	projection, ok := p.scoreProjection(now)
	if !ok {
		return
	}
	pendingY := y + rowHeight + 1
	pendingBounds := rl.Rectangle{X: x, Y: pendingY, Width: width, Height: rowHeight}
	pendingFill := blendRGBA(colors.panelAlt, violet, .3)
	rl.DrawRectangleRounded(pendingBounds, .22, 6, pendingFill)
	rl.DrawRectangleRoundedLinesEx(pendingBounds, .22, 6, 1.5, violet)
	pointDelta := projection.projected.Points - projection.current.Points
	multiplierDelta := projection.projected.Multipliers - projection.current.Multipliers
	prefix := fmt.Sprintf("AL GUARDAR +%d PTS · +%d MULT ·", pointDelta, multiplierDelta)
	total := fmt.Sprintf("TOTAL %d", projection.projected.Total)
	totalWidth := simpleui.MeasureTextStyled(total, scoreFontSize, simpleui.FontSemiBold).X
	totalBounds := rl.Rectangle{X: x + width - totalWidth - 12, Y: pendingY + 2, Width: totalWidth + 8, Height: rowHeight - 4}
	prefixWidth := totalBounds.X - (x + 7) - 4
	simpleui.DrawTextStyled(fitContestLabel(prefix, prefixWidth, scoreFontSize), x+7, pendingY+5, scoreFontSize, simpleui.FontSemiBold, lavender)
	flash := contestProjectionFlashOn(now)
	totalColor := lavender
	totalBorder := violet
	if flash {
		totalColor = rl.Color{R: 250, G: 240, B: 255, A: 255}
		totalBorder = rl.Color{R: 220, G: 150, B: 255, A: 255}
		rl.DrawRectangleRounded(rl.Rectangle{X: totalBounds.X - 2, Y: totalBounds.Y - 2, Width: totalBounds.Width + 4, Height: totalBounds.Height + 4}, .3, 6,
			blendRGBA(pendingFill, violet, .32))
	}
	rl.DrawRectangleRoundedLinesEx(totalBounds, .3, 6, 1.2, totalBorder)
	simpleui.DrawTextStyled(total, totalBounds.X+4, pendingY+5, scoreFontSize, simpleui.FontSemiBold, totalColor)
}

func (p *ContestLogPanel) drawWorkedIdentity() {
	if !p.hasWorkedIdentity {
		return
	}
	if local := p.workedLocalTime(time.Now()); local != "" {
		badge := rl.Rectangle{X: 930, Y: toolY + 5, Width: 245, Height: 22}
		background := blendRGBA(colors.panelAlt, colors.blue, .48)
		rl.DrawRectangleRounded(badge, .24, 6, background)
		rl.DrawRectangleRoundedLinesEx(badge, .24, 6, 1, colors.cyan)
		text := "HORA LOCAL ≈ " + local
		size := simpleui.MeasureTextStyled(text, 11, simpleui.FontSemiBold)
		simpleui.DrawTextStyled(text, badge.X+(badge.Width-size.X)/2, badge.Y+(badge.Height-size.Y)/2, 11, simpleui.FontSemiBold,
			simpleui.EnsureTextContrast(colors.text, background))
	}
	drawFT8RegionFlag(p.workedIdentity.Entity, 930, toolY+32)
	label := fmt.Sprintf("%s · CQ %d · %s", p.workedIdentity.Entity, p.workedIdentity.CQZone, p.workedIdentity.Continent)
	label = fitContestLabel(label, 395, 11)
	simpleui.DrawTextStyled(label, 954, toolY+31, 11, simpleui.FontSemiBold, colors.cyan)
}

func (p *ContestLogPanel) workedLocalTime(now time.Time) string {
	if !p.hasWorkedIdentity {
		return ""
	}
	minute := now.UTC().Unix() / 60
	if minute == p.workedLocalMinute {
		return p.workedLocalLabel
	}
	p.workedLocalMinute = minute
	info, ok := localtime.At(now, p.workedIdentity.Latitude, p.workedIdentity.Longitude)
	if !ok {
		p.workedLocalLabel = ""
		return ""
	}
	p.workedLocalLabel = info.Time.Format("15:04")
	return p.workedLocalLabel
}

func fitContestLabel(label string, width float32, fontSize int32) string {
	if simpleui.MeasureTextStyled(label, fontSize, simpleui.FontSemiBold).X <= width {
		return label
	}
	runes := []rune(label)
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if simpleui.MeasureTextStyled(candidate, fontSize, simpleui.FontSemiBold).X <= width {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return ""
}

func (p *ContestLogPanel) drawRows() {
	y := toolY + 138
	tableTop := y - 7
	tableBottom := toolY + toolH - 7
	visibleRows := contestLogVisibleRows(y, tableBottom)
	rows := make([]contestlog.QSO, 0, visibleRows)
	for index := len(p.session.QSOs) - 1; index >= 0 && len(rows) < visibleRows; index-- {
		rows = append(rows, p.session.QSOs[index])
	}
	rl.DrawRectangleRounded(rl.Rectangle{X: 376, Y: tableTop, Width: 1188, Height: tableBottom - tableTop}, .025, 5, colors.background)
	rl.BeginScissorMode(376, int32(tableTop), 1188, int32(tableBottom-tableTop))
	defer rl.EndScissorMode()
	headers := []struct {
		text string
		x    float32
	}{{i18n.Source("text.log0029"), 382}, {i18n.Source("text.log0030"), 443}, {i18n.Source("text.log0005"), 510}, {"DXCC", 685},
		{i18n.Source("text.log0031"), 900}, {i18n.Source("text.log0032"), 1050}, {i18n.Source("text.log0033"), 1422}}
	for _, header := range headers {
		simpleui.DrawTextStyled(header.text, header.x, y, 11, simpleui.FontSemiBold, colors.muted)
	}
	score := p.session.Score()
	simpleui.DrawTextStyled(fmt.Sprintf(i18n.Source("text.log0115"), score.Points, score.Multipliers, score.Total), 1200, y, 11, simpleui.FontSemiBold, colors.cyan)
	for i, q := range rows {
		rowY := y + 24 + float32(i)*25
		if i%2 == 0 {
			rl.DrawRectangle(376, int32(rowY-6), 1188, 23, colors.panelAlt)
		}
		state, stateColor := i18n.Source("text.log0034"), colors.green
		if q.Duplicate {
			state, stateColor = i18n.Source("text.log0035"), colors.orange
		} else if len(q.Warnings) > 0 {
			state, stateColor = i18n.Source("text.log0108"), colors.orange
		}
		values := []struct {
			text string
			x    float32
		}{
			{q.Time.UTC().Format("15:04"), 382}, {q.Band, 443}, {q.WorkedCall, 510},
			{fmt.Sprintf("%s/%s", q.RSTSent, q.SentExchangeText()), 900},
			{fmt.Sprintf("%s/%s", q.RSTReceived, q.ReceivedExchangeText()), 1050},
		}
		for _, value := range values {
			simpleui.DrawTextStyled(value.text, value.x, rowY, 12, simpleui.FontSemiBold, colors.text)
		}
		if identity, ok := p.cachedCallIdentity(q.WorkedCall); ok {
			drawFT8RegionFlag(identity.Entity, 685, rowY+1)
			name := fitContestLabel(identity.Entity, 180, 11)
			simpleui.DrawTextStyled(name, 709, rowY, 11, simpleui.FontSemiBold, colors.text)
		}
		simpleui.DrawTextStyled(state, 1422, rowY, 11, simpleui.FontSemiBold, stateColor)
	}
	if len(rows) == 0 {
		simpleui.DrawText(i18n.Source("text.log0036"), 382, y+35, 11, colors.muted)
	}
}

func contestLogVisibleRows(headerY, tableBottom float32) int {
	const rowStep = float32(25)
	// The first baseline is 24 px below the header. Reserve 17 px below
	// every baseline for the glyphs and striped-row background.
	room := tableBottom - (headerY + 24 + 17)
	if room < 0 {
		return 0
	}
	return int(room/rowStep) + 1
}

func (p *ContestLogPanel) cachedCallIdentity(call string) (contestlog.CallIdentity, bool) {
	call = contestlog.NormalizeCall(call)
	if call == "" {
		return contestlog.CallIdentity{}, false
	}
	if identity, ok := p.identityCache[call]; ok {
		return identity, true
	}
	identity, ok := contestlog.LookupCallIdentity(call)
	if ok {
		p.identityCache[call] = identity
	}
	return identity, ok
}
