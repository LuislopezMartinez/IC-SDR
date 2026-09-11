package screens

import (
	"path/filepath"
	"testing"
)

func TestMemoryGroupColorPersistsSeparately(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	panel := &MemoryPanel{groupColorsPath: path, groupColors: map[string]string{}, pendingGroup: "DMR", pendingGroupColor: 3}
	panel.commitGroupColor()
	loaded := &MemoryPanel{groupColorsPath: path}
	loaded.loadGroupColors()
	color := loaded.groupColor("DMR")
	want := memoryGroupPalette[3]
	if color != want {
		t.Fatalf("color=%+v want %+v", color, want)
	}
}
func TestUnknownGroupKeepsDefaultCyan(t *testing.T) {
	panel := &MemoryPanel{groupColors: map[string]string{}}
	if got := panel.groupColor("NUEVO"); got != colors.cyan {
		t.Fatalf("fallback=%+v", got)
	}
}

func TestCreateEmptyMemoryGroupPersists(t *testing.T) {
	dir := t.TempDir()
	panel := &MemoryPanel{groupColorsPath: filepath.Join(dir, "groups.json"), groupColors: map[string]string{}, selectedGroup: "ALL"}
	panel.rebuildGroups()
	panel.pendingGroup = "Emergencias"
	panel.pendingGroupColor = 3
	panel.commitNewGroup()
	if panel.selectedGroup != "Emergencias" || !panel.groupExists("emergencias") {
		t.Fatalf("new group was not selected or indexed: %+v", panel.groups)
	}
	loaded := &MemoryPanel{groupColorsPath: panel.groupColorsPath, groupColors: map[string]string{}, selectedGroup: "ALL"}
	loaded.loadGroupColors()
	loaded.rebuildGroups()
	if !loaded.groupExists("EMERGENCIAS") {
		t.Fatalf("empty group did not survive reload: %+v", loaded.groups)
	}
}

func TestEditGroupRenamesAndAppliesSharedFlags(t *testing.T) {
	dir := t.TempDir()
	panel := &MemoryPanel{
		memories:    []MemoryEntry{{Name: "A", Group: "OLD"}, {Name: "B", Group: "OLD", Priority: true}},
		groupColors: map[string]string{"OLD": "#000000"}, groupColorsPath: filepath.Join(dir, "groups.json"), path: filepath.Join(dir, "memories.json"),
		selectedGroup: "OLD", pendingOriginalGroup: "OLD", pendingGroup: "NEW", pendingGroupColor: 2, pendingGroupScan: true,
	}
	panel.commitGroupColor()
	if panel.selectedGroup != "NEW" || panel.memories[0].Group != "NEW" || panel.memories[1].Group != "NEW" || !panel.memories[0].ScanEnabled || panel.memories[1].Priority {
		t.Fatalf("group edit was not applied: %+v", panel)
	}
	if _, old := panel.groupColors["OLD"]; old || panel.groupColor("NEW") != memoryGroupPalette[2] {
		t.Fatalf("group color was not moved: %+v", panel.groupColors)
	}
}

func TestEditGroupCanReturnToNoMove(t *testing.T) {
	panel := &MemoryPanel{
		groups:               []string{"ALL", "OLD", "DESTINATION"},
		pendingOriginalGroup: "OLD",
	}
	panel.moveEditedGroup()
	if panel.pendingMoveGroup == "" {
		t.Fatal("the first click must select a destination")
	}
	for panel.pendingMoveGroup != "" {
		panel.moveEditedGroup()
	}
	if panel.pendingMoveGroup != "" {
		t.Fatal("the selector must be able to return to KEEP GROUP")
	}
}

func TestEditGroupMovesMemoriesIntoExistingGroup(t *testing.T) {
	dir := t.TempDir()
	panel := &MemoryPanel{
		memories: []MemoryEntry{
			{Name: "A", Group: "OLD"},
			{Name: "B", Group: "DESTINATION"},
		},
		groupColors:          map[string]string{"OLD": "#000000", "DESTINATION": "#123456"},
		groupColorsPath:      filepath.Join(dir, "groups.json"),
		path:                 filepath.Join(dir, "memories.json"),
		selectedGroup:        "OLD",
		pendingOriginalGroup: "OLD",
		pendingGroup:         "OLD",
		pendingMoveGroup:     "DESTINATION",
		pendingGroupScan:     true,
	}
	panel.commitGroupColor()
	if panel.memories[0].Group != "DESTINATION" || panel.selectedGroup != "DESTINATION" {
		t.Fatalf("memories were not moved to the existing group: %+v", panel)
	}
	if panel.groupColors["DESTINATION"] != "#123456" {
		t.Fatal("moving memories must not replace the destination group color")
	}
}
