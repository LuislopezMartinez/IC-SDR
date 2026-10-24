package screens

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-zero/internal/contestlog"
)

func TestContestLogControlsStayInsideToolWorkspace(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	for _, control := range panel.controls {
		bounds := control.Bounds()
		if bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
			t.Fatalf("control %s at y %.0f..%.0f is outside tool workspace %.0f..%.0f", control.ID(), bounds.Y, bounds.Y+bounds.Height, toolY, toolY+toolH)
		}
		if bounds.X < toolContentX || bounds.X+bounds.Width > toolContentRight {
			t.Fatalf("control %s at x %.0f..%.0f is outside tool workspace %.0f..%.0f", control.ID(), bounds.X, bounds.X+bounds.Width, toolContentX, toolContentRight)
		}
	}
}

func TestContestLogRowsFitCompletelyInsidePanel(t *testing.T) {
	headerY := toolY + 138
	tableBottom := toolY + toolH - 7
	rows := contestLogVisibleRows(headerY, tableBottom)
	if rows != 4 {
		t.Fatalf("visible rows = %d, want 4 complete rows", rows)
	}
	lastBaseline := headerY + 24 + float32(rows-1)*25
	if lastBaseline+17 > tableBottom {
		t.Fatalf("last row extends below table: %.0f > %.0f", lastBaseline+17, tableBottom)
	}
}

func TestSimpleContestTemplateEditorSavesAndSelectsTemplate(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	panel.templateDir = filepath.Join(t.TempDir(), "templates")
	panel.reloadProfiles()
	editor := panel.templateEditor
	editor.Open()
	editor.newTemplate()
	editor.name.SetText("Concurso local")
	editor.id.SetText("LOCAL-SSB")
	editor.start.SetText("")
	editor.end.SetText("")
	editor.points.SetText("2")
	editor.saveTemplate()
	if panel.profileValue != "LOCAL-SSB" {
		t.Fatalf("saved template was not selected: %s", panel.profileValue)
	}
	if len(panel.profiles) != len(contestlog.Profiles())+1 {
		t.Fatalf("saved template was not reloaded: %+v", panel.profiles)
	}
	found := false
	for _, profile := range panel.profiles {
		found = found || profile.ID == "LOCAL-SSB" && profile.PointsPerQSO == 2
	}
	if !found {
		t.Fatalf("saved template was not reloaded: %+v", panel.profiles)
	}
}

func TestCQZoneAutofillUpdatesAutomaticValueAndPreservesManualOverride(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	panel.profileValue = contestlog.CQWWSSB
	panel.stationCall.SetText("EA1ABC")
	panel.autofillCQZone()
	if got := panel.sentValue.Text(); got != "14" {
		t.Fatalf("EA1 zone = %q, want 14", got)
	}
	panel.stationCall.SetText("EA8ABC")
	panel.autofillCQZone()
	if got := panel.sentValue.Text(); got != "33" {
		t.Fatalf("EA8 zone = %q, want 33", got)
	}
	panel.sentValue.SetText("14")
	panel.autoSentValue = ""
	panel.stationCall.SetText("EA8ABC")
	panel.autofillCQZone()
	if got := panel.sentValue.Text(); got != "14" {
		t.Fatalf("manual zone was overwritten: %q", got)
	}
}

func TestWorkedCallIdentityUpdatesAndClears(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	panel.updateWorkedIdentity("EA8ABC")
	if !panel.hasWorkedIdentity || panel.workedIdentity.Entity != "Canary Islands" || panel.workedIdentity.CQZone != 33 {
		t.Fatalf("unexpected identity: %+v, %v", panel.workedIdentity, panel.hasWorkedIdentity)
	}
	panel.updateWorkedIdentity("")
	if panel.hasWorkedIdentity {
		t.Fatalf("empty call retained identity: %+v", panel.workedIdentity)
	}
}

func TestWorkedIdentityProvidesApproximateLocalTime(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	panel.updateWorkedIdentity("EA8ABC")
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	if got := panel.workedLocalTime(now); got != "13:00" {
		t.Fatalf("Canary Islands local time = %q, want 13:00", got)
	}
	if panel.workedLocalMinute != now.Unix()/60 {
		t.Fatal("local time minute was not cached")
	}
	panel.updateWorkedIdentity("")
	if got := panel.workedLocalTime(now); got != "" {
		t.Fatalf("unknown identity local time = %q", got)
	}
}

func TestContestLogCachesRowIdentity(t *testing.T) {
	panel := NewContestLogPanel(&MainScreen{})
	identity, ok := panel.cachedCallIdentity("EA8ABC")
	if !ok || identity.Entity != "Canary Islands" {
		t.Fatalf("unexpected cached identity: %+v, %v", identity, ok)
	}
	if _, exists := panel.identityCache["EA8ABC"]; !exists {
		t.Fatal("resolved identity was not cached")
	}
}

func TestDuplicatePreviewDistinguishesCurrentAndOtherBands(t *testing.T) {
	screen := &MainScreen{frequencyHz: 14_200_000}
	panel := NewContestLogPanel(screen)
	panel.session = &contestlog.Session{DuplicateRule: "BAND", QSOs: []contestlog.QSO{{
		WorkedCall: "K1ABC", Band: "20M", Time: time.Date(2026, 10, 24, 13, 4, 0, 0, time.UTC),
	}}}
	panel.workedCall.SetText("k1abc")
	panel.duplicateSignature = ""
	panel.updateDuplicatePreview()
	if panel.duplicatePreviewLevel != 2 || !strings.Contains(panel.duplicatePreview, "20M 13:04 UTC") {
		t.Fatalf("same-band duplicate preview = %q, level %d", panel.duplicatePreview, panel.duplicatePreviewLevel)
	}
	screen.frequencyHz = 7_100_000
	panel.updateDuplicatePreview()
	if panel.duplicatePreviewLevel != 1 || !strings.Contains(panel.duplicatePreview, "20M 13:04 UTC") {
		t.Fatalf("other-band preview = %q, level %d", panel.duplicatePreview, panel.duplicatePreviewLevel)
	}
}
