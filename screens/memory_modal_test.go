package screens

import (
	"path/filepath"
	"testing"

	"go-zero/simpleui"
)

func TestMemoryDuplicateDetectionUsesExactFrequency(t *testing.T) {
	panel := &MemoryPanel{memories: []MemoryEntry{
		{Name: "PMR-01", FrequencyHz: 446_006_250},
		{Name: "PMR-01 BIS", FrequencyHz: 446_006_250},
		{Name: "PMR-02", FrequencyHz: 446_018_750},
	}}
	names := panel.memoriesAtFrequency(446_006_250)
	if len(names) != 2 || names[0] != "PMR-01" || names[1] != "PMR-01 BIS" {
		t.Fatalf("duplicate names = %#v", names)
	}
	if names := panel.memoriesAtFrequency(446_006_251); len(names) != 0 {
		t.Fatalf("nearby frequency incorrectly treated as duplicate: %#v", names)
	}
}

func TestDeleteMemoryRequiresConfirmation(t *testing.T) {
	panel := &MemoryPanel{
		memories: []MemoryEntry{{Name: "KEEP", FrequencyHz: 1000}},
		selected: 0,
		path:     filepath.Join(t.TempDir(), "memories.json"),
	}
	panel.openDeleteModal()
	if len(panel.memories) != 1 || panel.modal != "delete" {
		t.Fatal("opening delete confirmation modified the memory")
	}
	panel.confirmDelete()
	if len(panel.memories) != 0 || panel.modal != "" {
		t.Fatal("confirmed delete did not remove the memory")
	}
}

func TestEditMemoryUsesDialogAndCommitsExplicitValues(t *testing.T) {
	panel := &MemoryPanel{
		memories: []MemoryEntry{{Name: "ORIGINAL", Description: "Control channel", FrequencyHz: 391_662_500, Mode: "TETRA", FilterBandwidthHz: 25_000, StepHz: 12_500, Group: "TETRA"}},
		selected: 0, selectedGroup: "ALL", pendingEditIndex: -1,
		path: filepath.Join(t.TempDir(), "memories.json"),
	}
	panel.editSelection()
	if panel.modal != "edit" || panel.memories[0].Name != "ORIGINAL" {
		t.Fatal("EDIT must open the dialog without changing the memory")
	}
	panel.pendingMemory.Name = "TETRA BCN"
	panel.pendingMemory.Description = "Barcelona TETRA service"
	panel.pendingMemory.FrequencyHz = 391_662_724
	panel.editField = 5
	panel.editBuffer = "6250"
	panel.commitEdit()
	got := panel.memories[0]
	if panel.modal != "" || got.Name != "TETRA BCN" || got.Description != "Barcelona TETRA service" || got.FrequencyHz != 391_662_724 || got.StepHz != 6250 {
		t.Fatalf("memory edited incorrectly: %#v", got)
	}
}

func TestAddMemoryOpensEditableFormWithCurrentReceiverValues(t *testing.T) {
	screen := &MainScreen{
		frequencyHz: 391_662_500, demodBandwidthHz: 25_000, tuningStepHz: 12_500,
		mode: simpleui.NewDropdown("testMemoryMode", 0, 0, 1, 1, "", []string{"TETRA"}, 10),
	}
	screen.mode.SetSelected(0)
	panel := &MemoryPanel{screen: screen, selectedGroup: "TETRA", pendingEditIndex: -1, path: filepath.Join(t.TempDir(), "memories.json")}
	panel.openSaveModal()
	if panel.modal != "create" || panel.editField != 0 {
		t.Fatalf("+ MEMORY opened modal %q on field %d", panel.modal, panel.editField)
	}
	got := panel.pendingMemory
	if got.FrequencyHz != 391_662_500 || got.Mode != "TETRA" || got.FilterBandwidthHz != 25_000 || got.StepHz != 12_500 || got.Group != "TETRA" || !got.ScanEnabled {
		t.Fatalf("incorrect initial values: %#v", got)
	}
}
