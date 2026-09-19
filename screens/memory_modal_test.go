package screens

import (
	"encoding/json"
	"go-zero/internal/i18n"
	"os"
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
		memories: []MemoryEntry{{Name: "ORIGINAL", Description: "Canal de control", FrequencyHz: 391_662_500, Mode: "TETRA", FilterBandwidthHz: 25_000, StepHz: 12_500, Group: "TETRA"}},
		selected: 0, selectedGroup: "TODAS", pendingEditIndex: -1,
		path: filepath.Join(t.TempDir(), "memories.json"),
	}
	panel.editSelection()
	if panel.modal != "edit" || panel.memories[0].Name != "ORIGINAL" {
		t.Fatal("EDITAR debe abrir el diálogo sin modificar la memoria")
	}
	panel.pendingMemory.Name = "TETRA BCN"
	panel.pendingMemory.Description = "Servicio TETRA de Barcelona"
	panel.pendingMemory.FrequencyHz = 391_662_724
	panel.editField = 5
	panel.editBuffer = "6250"
	panel.commitEdit()
	got := panel.memories[0]
	if panel.modal != "" || got.Name != "TETRA BCN" || got.Description != "Servicio TETRA de Barcelona" || got.FrequencyHz != 391_662_724 || got.StepHz != 6250 {
		t.Fatalf("memoria editada incorrectamente: %#v", got)
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
		t.Fatalf("+ MEMORIA abrió modal %q en campo %d", panel.modal, panel.editField)
	}
	got := panel.pendingMemory
	if got.FrequencyHz != 391_662_500 || got.Mode != "TETRA" || got.FilterBandwidthHz != 25_000 || got.StepHz != 12_500 || got.Group != "TETRA" || !got.ScanEnabled {
		t.Fatalf("valores iniciales incorrectos: %#v", got)
	}
}

func TestMemoryCapturesAndRecallsActiveTool(t *testing.T) {
	mode := simpleui.NewDropdown("memoryToolMode", 0, 0, 100, 30, "", []string{"NFM"}, 12)
	mode.SetSelected(0)
	aprs := i18n.Source("text.4c4310fd27fd")
	tetraTool := i18n.Source("text.f69d86a86926")
	screen := &MainScreen{activeTool: aprs, frequencyHz: 144_800_000, centerFrequencyHz: 144_800_000, spanHz: 250_000, demodBandwidthHz: 12_500, tuningStepHz: 6_250, mode: mode}
	panel := &MemoryPanel{screen: screen, selectedGroup: "APRS", selected: -1, pendingEditIndex: -1, path: filepath.Join(t.TempDir(), "memories.json")}
	screen.memoryPanel = panel
	panel.openSaveModal()
	if panel.pendingMemory.Tool != aprs {
		t.Fatalf("new memory tool=%q want %q", panel.pendingMemory.Tool, aprs)
	}
	panel.editField = 0
	panel.editBuffer = "APRS 144.800"
	panel.commitCurrent()
	data, err := os.ReadFile(panel.path)
	if err != nil {
		t.Fatal(err)
	}
	var saved []MemoryEntry
	if json.Unmarshal(data, &saved) != nil || len(saved) != 1 || saved[0].Tool != aprs {
		t.Fatalf("stored memory lost tool: %s", data)
	}
	screen.activeTool = tetraTool
	panel.recall(saved[0])
	if screen.activeTool != aprs {
		t.Fatalf("recall selected %q want %q", screen.activeTool, aprs)
	}
}

func TestEditingMemoryUpdatesToolAndLegacyMemoryRemainsCompatible(t *testing.T) {
	tetraTool := i18n.Source("text.f69d86a86926")
	screen := &MainScreen{activeTool: tetraTool}
	panel := &MemoryPanel{screen: screen, memories: []MemoryEntry{{Name: "OLD", FrequencyHz: 420_800_000, Mode: "NFM", FilterBandwidthHz: 25_000, StepHz: 12_500}}, selected: 0, selectedGroup: "TODAS", pendingEditIndex: -1, path: filepath.Join(t.TempDir(), "memories.json")}
	panel.openEditModal()
	if panel.pendingMemory.Tool != tetraTool {
		t.Fatalf("edit did not capture active tool: %+v", panel.pendingMemory)
	}
	var legacy MemoryEntry
	if err := json.Unmarshal([]byte(`{"name":"LEGACY","frequencyHz":1000000,"mode":"AM","filterBandwidthHz":10000,"stepHz":1000,"group":"SIN GRUPO"}`), &legacy); err != nil || legacy.Tool != "" {
		t.Fatalf("legacy memory incompatible: %+v %v", legacy, err)
	}
}
