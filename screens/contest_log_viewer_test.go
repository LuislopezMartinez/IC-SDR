package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-zero/internal/contestlog"
)

func TestContestLogViewerEditsAndDeletesPersistently(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	root := t.TempDir()
	panel.path = filepath.Join(root, "contest.json")
	panel.backupDir = filepath.Join(root, "backups")
	panel.session, _ = contestlog.NewOceaniaDXSSB("EA1ABC", "", "LOW", time.Now())
	_, err := panel.session.Add(contestlog.Entry{Time: time.Date(2026, 10, 3, 6, 0, 0, 0, time.UTC), FrequencyHz: 14_200_000,
		WorkedCall: "VK2AAA", RSTSent: "59", RSTReceived: "59", SerialReceived: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := contestlog.Save(panel.path, panel.session); err != nil {
		t.Fatal(err)
	}

	viewer := panel.viewer
	viewer.Open(0)
	viewer.call.SetText("ZL1BBB")
	viewer.frequency.SetText("7.100")
	viewer.serialReceived.SetText("27")
	viewer.saveSelected()
	backups, _ := filepath.Glob(filepath.Join(panel.backupDir, "contest-*.json"))
	if len(backups) != 1 {
		t.Fatalf("automatic backups = %v", backups)
	}
	loaded, err := contestlog.Load(panel.path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.QSOs[0].WorkedCall != "ZL1BBB" || loaded.QSOs[0].Band != "40M" || loaded.QSOs[0].SerialReceived != 27 {
		t.Fatalf("edited QSO was not persisted: %+v", loaded.QSOs[0])
	}

	viewer.deleteSelected()
	loaded, err = contestlog.Load(panel.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.QSOs) != 0 || loaded.NextSerial != 2 {
		t.Fatalf("delete must persist without reusing sent serial: %+v", loaded)
	}
}

func TestContestLifecycleArchivesExportsAndReopens(t *testing.T) {
	root := t.TempDir()
	panel := NewContestLogPanel(&MainScreen{})
	panel.path = filepath.Join(root, "active.json")
	panel.archiveDir = filepath.Join(root, "archive")
	panel.exportDir = filepath.Join(root, "exports")
	panel.backupDir = filepath.Join(root, "backups")
	panel.session, _ = contestlog.NewOceaniaDXSSB("EA1ABC", "", "LOW", time.Now())
	panel.session.Name, panel.session.Email = "Luis", "luis@example.com"
	panel.session.Address, panel.session.Country = "Calle Radio 1", "Spain"
	_, err := panel.session.Add(contestlog.Entry{Time: time.Now(), FrequencyHz: 14_200_000, WorkedCall: "VK2AAA", SerialReceived: 7})
	if err != nil {
		t.Fatal(err)
	}
	if err := contestlog.Save(panel.path, panel.session); err != nil {
		t.Fatal(err)
	}

	if err := panel.finishSession(); err != nil {
		t.Fatal(err)
	}
	if panel.session != nil {
		t.Fatal("finished contest remained active")
	}
	if _, err := os.Stat(panel.path); !os.IsNotExist(err) {
		t.Fatalf("active file still exists: %v", err)
	}
	archives := panel.archivePaths()
	if len(archives) != 1 {
		t.Fatalf("archives = %v", archives)
	}
	archived, err := contestlog.Load(archives[0])
	if err != nil || !archived.Closed() {
		t.Fatalf("archive is not closed: %+v, %v", archived, err)
	}
	logs, _ := filepath.Glob(filepath.Join(panel.exportDir, "*.log"))
	if len(logs) != 1 {
		t.Fatalf("Cabrillo exports = %v", logs)
	}

	if err := panel.reopenArchive(archives[0]); err != nil {
		t.Fatal(err)
	}
	if panel.session == nil || panel.session.Closed() || panel.session.NextSerial != 2 {
		t.Fatalf("bad reopened session: %+v", panel.session)
	}
	if _, err := os.Stat(archives[0]); !os.IsNotExist(err) {
		t.Fatalf("archive was not consumed: %v", err)
	}
}

func TestContestIndexIncludesActiveAndArchivedSessions(t *testing.T) {
	root := t.TempDir()
	panel := NewContestLogPanel(&MainScreen{})
	panel.archiveDir = filepath.Join(root, "archive")
	panel.session, _ = contestlog.NewOceaniaDXSSB("EA1ACTIVE", "", "LOW", time.Now())
	archived, _ := contestlog.NewOceaniaDXSSB("EA1OLD", "", "LOW", time.Now().Add(-24*time.Hour))
	archived.Close(time.Now().Add(-23 * time.Hour))
	archivePath := filepath.Join(panel.archiveDir, "old.json")
	if err := contestlog.Save(archivePath, archived); err != nil {
		t.Fatal(err)
	}

	panel.viewer.OpenHistory()
	if !panel.viewer.historyMode || len(panel.viewer.historyEntries) != 2 {
		t.Fatalf("contest index = mode %v, entries %+v", panel.viewer.historyMode, panel.viewer.historyEntries)
	}
	if !panel.viewer.historyEntries[0].active || panel.viewer.historyEntries[1].path != archivePath {
		t.Fatalf("active/archive ordering is wrong: %+v", panel.viewer.historyEntries)
	}
}

func TestSelectedContestAlwaysShowsBothExportFormats(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	panel.session, _ = contestlog.NewOceaniaDXSSB("EA1ABC", "", "LOW", time.Now())
	viewer := panel.viewer
	viewer.Open(-1)
	if !viewer.archiveExport.Visible() || !viewer.archiveADIF.Visible() {
		t.Fatal("active contest does not show both export buttons")
	}
	viewer.historyMode = true
	viewer.refreshArchiveControls()
	if viewer.archiveExport.Visible() || viewer.archiveADIF.Visible() {
		t.Fatal("contest index should not show export buttons before a contest is selected")
	}
}

func TestContestExportNamesIncludeCallContestAndDate(t *testing.T) {
	session := &contestlog.Session{StationCall: "EA3IWW/P", Contest: contestlog.CQWWSSB,
		CreatedAt: time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC)}
	if got, want := contestExportBaseName(session), "ea3iww-p-cq-ww-ssb-20261025"; got != want {
		t.Fatalf("export basename = %q, want %q", got, want)
	}
}

func TestSelectedContestReportsTheExportedFormat(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	panel.exportDir = t.TempDir()
	panel.session, _ = contestlog.NewSession(contestlog.CQWWSSB, "EA3IWW", "EA3IWW",
		contestlog.Config{Power: "LOW", SentValue: "14", Location: "DX"}, time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC))
	panel.session.Name, panel.session.Email = "Luis", "luis@example.com"
	panel.session.Address, panel.session.Country = "Calle Radio 1", "Spain"
	_, _ = panel.session.Add(contestlog.Entry{Time: time.Date(2026, 10, 25, 8, 5, 0, 0, time.UTC), FrequencyHz: 28_435_000,
		Mode: "USB", WorkedCall: "7Z7A", RSTSent: "59", RSTReceived: "59", ExchangeReceived: "21"})
	viewer := panel.viewer
	viewer.Open(0)
	viewer.exportArchived()
	if !strings.Contains(viewer.status, "Cabrillo") || !strings.Contains(viewer.status, ".log") {
		t.Fatalf("Cabrillo export status = %q", viewer.status)
	}
	viewer.exportArchivedADIF()
	if !strings.Contains(viewer.status, "ADIF") || !strings.Contains(viewer.status, ".adi") {
		t.Fatalf("ADIF export status = %q", viewer.status)
	}
}

func TestExportValidationNavigatesToExactMissingData(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	panel.exportDir = t.TempDir()
	panel.session, _ = contestlog.NewOceaniaDXSSB("EA1ABC", "", "LOW", time.Now())
	panel.session.QSOs = []contestlog.QSO{{Time: time.Now().UTC(), FrequencyHz: 14_200_000, Band: "20M", Mode: "PH",
		StationCall: "EA1ABC", WorkedCall: "VK2AAA", RSTSent: "59", RSTReceived: "59", SerialSent: 1,
		ExchangeSent: "001", SerialReceived: 1, ExchangeReceived: "001"}}
	viewer := panel.viewer
	viewer.Open(0)
	viewer.exportArchived()
	if !viewer.participantMode || !viewer.participantName.Focused() || !strings.Contains(viewer.status, "falta el nombre") {
		t.Fatalf("Cabrillo validation did not navigate to participant name: mode=%v status=%q", viewer.participantMode, viewer.status)
	}

	panel.session.QSOs[0].FrequencyHz = 0
	viewer.setParticipantMode(false)
	viewer.exportArchivedADIF()
	if viewer.selected != 0 || !viewer.frequency.Focused() || !strings.Contains(viewer.status, "falta la frecuencia") {
		t.Fatalf("ADIF validation did not navigate to QSO frequency: selected=%d status=%q", viewer.selected, viewer.status)
	}
}

func TestFinishValidationExplainsExactBlockerAndKeepsContestOpen(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	panel.session, _ = contestlog.NewOceaniaDXSSB("EA1ABC", "", "LOW", time.Now())
	panel.session.QSOs = []contestlog.QSO{{Time: time.Now().UTC(), FrequencyHz: 14_200_000, Band: "20M", Mode: "PH",
		StationCall: "EA1ABC", WorkedCall: "VK2AAA", RSTSent: "59", RSTReceived: "59", SerialSent: 1,
		ExchangeSent: "001", SerialReceived: 1, ExchangeReceived: "001"}}
	viewer := panel.viewer
	viewer.OpenFinish()
	viewer.finishSession()
	if panel.session == nil || !viewer.participantMode || !viewer.participantName.Focused() {
		t.Fatal("blocked finish did not preserve the contest and navigate to the missing participant field")
	}
	if !strings.Contains(viewer.status, "No se puede finalizar el concurso") || !strings.Contains(viewer.status, "falta el nombre") {
		t.Fatalf("finish blocker is not explicit: %q", viewer.status)
	}
}
