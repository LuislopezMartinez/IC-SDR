package screens

import (
	"path/filepath"
	"testing"

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
