package screens

import (
	"path/filepath"
	"testing"
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
	if len(panel.profiles) != 3 || panel.profiles[2].PointsPerQSO != 2 {
		t.Fatalf("saved template was not reloaded: %+v", panel.profiles)
	}
}
