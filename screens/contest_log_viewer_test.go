package screens

import (
	"os"
	"path/filepath"
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
