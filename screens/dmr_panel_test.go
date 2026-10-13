package screens

import (
	"fmt"
	"testing"
)

func TestDMRHistoryRetainsBoundedCompleteSession(t *testing.T) {
	panel := NewDMRPanel(NewMainScreen(nil))
	for index := 0; index < 540; index++ {
		panel.add("DATA", "TS1", fmt.Sprintf("event %d", index))
	}
	if got, want := len(panel.events), 500; got != want {
		t.Fatalf("retained DMR events = %d, want %d", got, want)
	}
	if got := panel.events[0].detail; got != "event 40" {
		t.Fatalf("oldest retained event = %q, want event 40", got)
	}
	if got := panel.events[len(panel.events)-1].detail; got != "event 539" {
		t.Fatalf("newest retained event = %q", got)
	}
}

func TestDMRPreviewPauseKeepsPositionAndCountsNewEvents(t *testing.T) {
	panel := NewDMRPanel(NewMainScreen(nil))
	panel.add("SEARCH", "--", "first")
	panel.togglePreviewPause()
	panel.add("DATA", "TS1", "second")
	panel.add("DATA", "TS2", "third")

	if !panel.previewPaused || panel.previewEnd != 1 || panel.previewNew != 2 {
		t.Fatalf("paused preview = paused %v end %d new %d", panel.previewPaused, panel.previewEnd, panel.previewNew)
	}
	panel.togglePreviewPause()
	if panel.previewPaused || panel.previewNew != 0 {
		t.Fatalf("resumed preview = paused %v new %d", panel.previewPaused, panel.previewNew)
	}
}

func TestDMRHistoryFiltersSlotsAndTracksNewEvents(t *testing.T) {
	panel := NewDMRPanel(NewMainScreen(nil))
	panel.add("DATA", "TS1", "one")
	panel.add("DATA", "TS2", "two")
	panel.add("SEARCH", "--", "three")
	panel.openHistory()

	panel.historyFilter = 1
	if got := panel.filteredHistoryIndices(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("TS1 history indices = %v", got)
	}
	panel.historyLive = false
	panel.add("DATA", "TS1", "four")
	if got := len(panel.events) - panel.historySeen; got != 1 {
		t.Fatalf("paused history new-event count = %d, want 1", got)
	}
}

func TestDMRControlsUseFullLowerWorkspaceWithoutOverflow(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewDMRPanel(screen)
	compactToolControls(panel.controls)
	for _, control := range panel.controls {
		bounds := control.Bounds()
		if bounds.X < toolContentX || bounds.X+bounds.Width > toolContentRight || bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
			t.Fatalf("DMR control %s outside lower workspace: %+v", control.ID(), bounds)
		}
	}
	if bottom := panel.clear.Bounds().Y + panel.clear.Bounds().Height; bottom < toolY+toolH-75 {
		t.Fatalf("DMR actions still leave excessive lower space: bottom %.0f workspace %.0f", bottom, toolY+toolH)
	}
}
