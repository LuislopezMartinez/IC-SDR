package screens

import (
	"path/filepath"
	"testing"

	"go-zero/simpleui"
)

func TestWebMemoryControlSaveTuneDeleteAndGroups(t *testing.T) {
	dir := t.TempDir()
	screen := &MainScreen{frequencyHz: 100_000_000, centerFrequencyHz: 100_000_000, spanHz: 250_000, centerMode: true}
	screen.mode = simpleui.NewDropdown("webMemoryMode", 0, 0, 100, 30, "MODE", []string{"AM", "NFM"}, 12)
	panel := &MemoryPanel{screen: screen, path: filepath.Join(dir, "memories.json"), groupColorsPath: filepath.Join(dir, "groups.json"), groupColors: map[string]string{}, groups: []string{"TODAS"}, selectedGroup: "TODAS", selected: -1}
	screen.memoryPanel = panel
	entry := MemoryEntry{Name: "PRUEBA", FrequencyHz: 100_050_000, Mode: "NFM", FilterBandwidthHz: 12_500, StepHz: 6_250, ScanEnabled: true, Group: "SIN GRUPO"}
	if err := (webControlCommand{Action: "memorySave", MemoryIndex: -1, Memory: &entry}).validate(); err != nil {
		t.Fatal(err)
	}
	screen.applyWebControl(webControlCommand{Action: "memorySave", MemoryIndex: -1, Memory: &entry})
	if len(panel.memories) != 1 || panel.memories[0].Name != "PRUEBA" {
		t.Fatalf("save: %+v", panel.memories)
	}
	screen.applyWebControl(webControlCommand{Action: "groupAdd", Group: "LOCAL"})
	if !panel.groupExists("LOCAL") {
		t.Fatal("group not added")
	}
	entry.Group, entry.Name = "LOCAL", "EDITADA"
	screen.applyWebControl(webControlCommand{Action: "memorySave", MemoryIndex: 0, MemoryName: "PRUEBA", Memory: &entry})
	if panel.memories[0].Name != "EDITADA" || panel.memories[0].Group != "LOCAL" {
		t.Fatal("memory not edited")
	}
	screen.applyWebControl(webControlCommand{Action: "memoryTune", MemoryIndex: 0, MemoryName: "EDITADA"})
	if screen.frequencyHz != entry.FrequencyHz {
		t.Fatalf("tuned to %d", screen.frequencyHz)
	}
	screen.applyWebControl(webControlCommand{Action: "groupRename", Group: "LOCAL", NewGroup: "NUEVO"})
	if panel.memories[0].Group != "NUEVO" {
		t.Fatal("group not renamed")
	}
	screen.applyWebControl(webControlCommand{Action: "memoryDelete", MemoryIndex: 0, MemoryName: "PRUEBA"})
	if len(panel.memories) != 1 {
		t.Fatal("stale name deleted memory")
	}
	screen.applyWebControl(webControlCommand{Action: "memoryDelete", MemoryIndex: 0, MemoryName: "EDITADA"})
	if len(panel.memories) != 0 {
		t.Fatal("memory not deleted")
	}
}
