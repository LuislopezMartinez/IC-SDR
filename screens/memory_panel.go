package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go-zero/internal/resources"
	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	memoryPanelTop    = float32(638)
	memoryPanelH      = float32(182)
	memoryHeaderY     = float32(648)
	memoryFirstRowY   = float32(672)
	memoryRowH        = float32(24)
	memoryVisibleRows = 6
)

type MemoryEntry struct {
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	FrequencyHz       int64  `json:"frequencyHz"`
	Mode              string `json:"mode"`
	FilterBandwidthHz int    `json:"filterBandwidthHz"`
	StepHz            int64  `json:"stepHz"`
	ScanEnabled       bool   `json:"scanEnabled"`
	Group             string `json:"group"`
	Priority          bool   `json:"priority"`
	CTCSSHz           string `json:"ctcssHz,omitempty"`
	DCSCode           string `json:"dcsCode,omitempty"`
}

type MemoryPanel struct {
	simpleui.BaseElement
	screen                                                                     *MainScreen
	controls                                                                   []simpleui.Element
	memories                                                                   []MemoryEntry
	groups                                                                     []string
	selectedGroup                                                              string
	selected                                                                   int
	scrollOffset                                                               int
	lastClicked                                                                int
	lastClickAt                                                                float64
	markersVisible, compact, onlyActive                                        bool
	path                                                                       string
	groupColorsPath                                                            string
	groupColors                                                                map[string]string
	viewSwitch                                                                 *simpleui.Switch
	add, addGroup, tune, edit, duplicate, move, remove, markerView, activeOnly *simpleui.Button
	modal                                                                      string
	modalPressed                                                               int
	pendingMemory                                                              MemoryEntry
	pendingDeleteIndex                                                         int
	duplicateNames                                                             []string
	pendingGroup                                                               string
	pendingGroupColor                                                          int
	pendingOriginalGroup                                                       string
	pendingGroupScan                                                           bool
	pendingGroupPriority                                                       bool
	pendingMoveGroup                                                           string
	pendingEditIndex                                                           int
	editField                                                                  int
	editBuffer                                                                 string
	editError                                                                  string
}

var memoryGroupPalette = []rl.Color{
	{R: 45, G: 195, B: 225, A: 255}, {R: 70, G: 205, B: 135, A: 255},
	{R: 238, G: 156, B: 24, A: 255}, {R: 235, G: 70, B: 75, A: 255},
	{R: 170, G: 125, B: 235, A: 255}, {R: 70, G: 145, B: 245, A: 255},
	{R: 235, G: 105, B: 180, A: 255}, {R: 205, G: 210, B: 220, A: 255},
}

func NewMemoryPanel(screen *MainScreen) *MemoryPanel {
	p := &MemoryPanel{BaseElement: simpleui.NewBaseElement("memoryModal", 0, 0, designWidth, designHeight), screen: screen, selected: -1, lastClicked: -1, selectedGroup: "ALL", markersVisible: screen.memoryViewEnabled, path: resources.WritablePath("config", "memories.json"), groupColorsPath: resources.WritablePath("config", "memory-groups.json"), groupColors: map[string]string{}, pendingDeleteIndex: -1, pendingEditIndex: -1}
	p.load()
	p.loadGroupColors()
	p.rebuildGroups()
	p.viewSwitch = simpleui.NewSwitch("memoryViewSwitch", 1355, 638, 98, 25, "MEM VIEW", screen.memoryViewEnabled, 9)
	p.viewSwitch.SetTrackColors(rl.Color{R: 51, G: 61, B: 70, A: 255}, colors.blue)
	p.viewSwitch.OnChange(screen.setMemoryView)
	p.activeOnly = simpleui.NewButton("memoryOnlyActive", 1462, 638, 98, 25, "ALL", 9)
	p.addGroup = simpleui.NewButton("memoryAddGroup", 41, 793, 182, 25, "+ NEW GROUP", 11)
	p.add = simpleui.NewButton("memoryAdd", 1355, 665, 126, 25, "+ MEMORY", 12)
	p.markerView = simpleui.NewButton("memoryMarkerView", 1487, 665, 73, 25, "VIEW B", 9)
	p.tune = simpleui.NewButton("memoryTune", 1355, 692, 205, 25, "TUNE", 12)
	p.edit = simpleui.NewButton("memoryEdit", 1355, 719, 205, 25, "EDIT", 12)
	p.duplicate = simpleui.NewButton("memoryDuplicate", 1355, 746, 205, 25, "DUPLICATE", 12)
	p.move = simpleui.NewButton("memoryMove", 1355, 773, 205, 25, "MOVE", 12)
	p.remove = simpleui.NewButton("memoryDelete", 1355, 800, 205, 25, "DELETE", 12)
	p.add.SetColors(rl.Color{R: 20, G: 105, B: 70, A: 255}, colors.green, colors.text)
	p.addGroup.SetColors(rl.Color{R: 28, G: 62, B: 88, A: 255}, colors.blue, colors.text)
	p.tune.SetColors(rl.Color{R: 20, G: 85, B: 125, A: 255}, colors.cyan, colors.text)
	p.edit.SetColors(rl.Color{R: 28, G: 62, B: 88, A: 255}, colors.blue, colors.text)
	p.duplicate.SetColors(rl.Color{R: 28, G: 62, B: 88, A: 255}, colors.blue, colors.text)
	p.move.SetColors(rl.Color{R: 105, G: 67, B: 18, A: 255}, colors.orange, colors.text)
	p.remove.SetColors(rl.Color{R: 112, G: 31, B: 35, A: 255}, colors.red, colors.text)
	p.add.OnClick(p.openSaveModal)
	p.addGroup.OnClick(p.openNewGroupModal)
	p.tune.OnClick(p.tuneSelected)
	p.edit.OnClick(p.editSelection)
	p.duplicate.OnClick(p.duplicateSelected)
	p.move.OnClick(p.moveSelectedToNextGroup)
	p.remove.OnClick(p.openDeleteModal)
	p.markerView.OnClick(func() {
		p.compact = !p.compact
		if p.compact {
			p.markerView.SetLabel("VIEW B")
		} else {
			p.markerView.SetLabel("VIEW A")
		}
	})
	p.activeOnly.OnClick(func() {
		p.onlyActive = !p.onlyActive
		if p.onlyActive {
			p.activeOnly.SetLabel("ACTIVE")
		} else {
			p.activeOnly.SetLabel("ALL")
		}
	})
	p.controls = []simpleui.Element{p.viewSwitch, p.addGroup, p.add, p.tune, p.edit, p.duplicate, p.move, p.remove, p.markerView, p.activeOnly}
	p.SetVisible(false)
	return p
}

func (p *MemoryPanel) loadGroupColors() {
	data, err := os.ReadFile(p.groupColorsPath)
	if err == nil {
		_ = json.Unmarshal(data, &p.groupColors)
	}
	if p.groupColors == nil {
		p.groupColors = map[string]string{}
	}
	if p.normalizeGroupColorKeys() {
		p.saveGroupColors()
	}
}
func (p *MemoryPanel) saveGroupColors() {
	_ = os.MkdirAll(filepath.Dir(p.groupColorsPath), 0755)
	data, err := json.MarshalIndent(p.groupColors, "", "  ")
	if err == nil {
		_ = os.WriteFile(p.groupColorsPath, data, 0644)
	}
}
func colorHex(c rl.Color) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }
func parseColorHex(value string, fallback rl.Color) rl.Color {
	var r, g, b uint8
	if _, err := fmt.Sscanf(value, "#%02X%02X%02X", &r, &g, &b); err == nil {
		return rl.Color{R: r, G: g, B: b, A: 255}
	}
	return fallback
}
func (p *MemoryPanel) groupColor(group string) rl.Color {
	if group == "ALL" || group == "UNGROUPED" {
		return colors.cyan
	}
	return parseColorHex(p.groupColors[group], colors.cyan)
}
func (p *MemoryPanel) editSelection() {
	if p.selectedGroup != "ALL" && p.selected < 0 {
		p.openGroupModal()
		return
	}
	p.openEditModal()
}

func (p *MemoryPanel) openEditModal() {
	if p.selected < 0 || p.selected >= len(p.memories) {
		return
	}
	p.pendingEditIndex, p.pendingMemory = p.selected, p.memories[p.selected]
	p.modal, p.modalPressed, p.editField, p.editError = "edit", 0, 0, ""
	p.loadEditBuffer()
}

func (p *MemoryPanel) loadEditBuffer() {
	switch p.editField {
	case 0:
		p.editBuffer = p.pendingMemory.Name
	case 1:
		p.editBuffer = fmt.Sprintf("%.6f", float64(p.pendingMemory.FrequencyHz)/1e6)
	case 4:
		p.editBuffer = strconv.Itoa(p.pendingMemory.FilterBandwidthHz)
	case 5:
		p.editBuffer = strconv.FormatInt(p.pendingMemory.StepHz, 10)
	case 6:
		p.editBuffer = p.pendingMemory.Description
	default:
		p.editBuffer = ""
	}
}

func (p *MemoryPanel) storeEditBuffer() bool {
	p.editError = ""
	v := strings.TrimSpace(p.editBuffer)
	switch p.editField {
	case 0:
		if v == "" {
			p.editError = "Name cannot be empty."
			return false
		}
		p.pendingMemory.Name = v
	case 1:
		mhz, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		if err != nil || mhz <= 0 {
			p.editError = "Enter a valid frequency in MHz."
			return false
		}
		p.pendingMemory.FrequencyHz = int64(mhz*1e6 + .5)
	case 4:
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			p.editError = "Filter width must be greater than zero."
			return false
		}
		p.pendingMemory.FilterBandwidthHz = n
	case 5:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			p.editError = "Step must be greater than zero."
			return false
		}
		p.pendingMemory.StepHz = n
	case 6:
		p.pendingMemory.Description = v
	}
	return true
}

func (p *MemoryPanel) commitEdit() {
	if p.pendingEditIndex < 0 || p.pendingEditIndex >= len(p.memories) || !p.storeEditBuffer() {
		return
	}
	p.memories[p.pendingEditIndex] = p.pendingMemory
	p.selected = p.pendingEditIndex
	p.rebuildGroups()
	p.save()
	p.closeModal()
}
func (p *MemoryPanel) openGroupModal() {
	if p.selectedGroup == "" || p.selectedGroup == "ALL" || p.selectedGroup == "UNGROUPED" {
		return
	}
	p.pendingOriginalGroup, p.pendingGroup, p.pendingMoveGroup = p.selectedGroup, p.selectedGroup, ""
	p.pendingGroupScan, p.pendingGroupPriority = true, false
	for _, memory := range p.memories {
		if memoryGroup(memory) == p.selectedGroup {
			p.pendingGroupScan = p.pendingGroupScan && memory.ScanEnabled
			p.pendingGroupPriority = p.pendingGroupPriority || memory.Priority
		}
	}
	p.pendingGroupColor = 0
	current := p.groupColor(p.selectedGroup)
	best := int64(1 << 62)
	for i, c := range memoryGroupPalette {
		d := int64(absInt(int(c.R)-int(current.R)) + absInt(int(c.G)-int(current.G)) + absInt(int(c.B)-int(current.B)))
		if d < best {
			best = int64(d)
			p.pendingGroupColor = i
		}
	}
	p.modal, p.modalPressed = "group", 0
}
func (p *MemoryPanel) commitGroupColor() {
	name := strings.TrimSpace(p.pendingGroup)
	original := p.pendingOriginalGroup
	if original == "" {
		original = name
	}
	if name == "" || strings.EqualFold(name, "ALL") || strings.EqualFold(name, "UNGROUPED") {
		p.editError = "Enter a valid group name."
		return
	}
	if p.pendingMoveGroup == "" && !strings.EqualFold(name, p.pendingOriginalGroup) && p.groupExists(name) {
		p.editError = "A group with that name already exists."
		return
	}
	destination := name
	if p.pendingMoveGroup != "" {
		destination = p.pendingMoveGroup
	}
	for i := range p.memories {
		if memoryGroup(p.memories[i]) == original {
			p.memories[i].Group, p.memories[i].ScanEnabled, p.memories[i].Priority = destination, p.pendingGroupScan, p.pendingGroupPriority
		}
	}
	delete(p.groupColors, original)
	if p.pendingMoveGroup == "" {
		p.groupColors[destination] = colorHex(memoryGroupPalette[p.pendingGroupColor])
	}
	p.selectedGroup = destination
	p.save()
	p.rebuildGroups()
	p.saveGroupColors()
	p.closeModal()
}

func (p *MemoryPanel) deleteEditedGroup() {
	for i := range p.memories {
		if memoryGroup(p.memories[i]) == p.pendingOriginalGroup {
			p.memories[i].Group = "UNGROUPED"
		}
	}
	delete(p.groupColors, p.pendingOriginalGroup)
	p.selectedGroup, p.selected = "ALL", -1
	p.save()
	p.saveGroupColors()
	p.rebuildGroups()
	p.closeModal()
}

func (p *MemoryPanel) moveEditedGroup() {
	targets := []string{"", "UNGROUPED"}
	for _, group := range p.groups {
		if group != "ALL" && group != p.pendingOriginalGroup && group != "UNGROUPED" {
			targets = append(targets, group)
		}
	}
	p.pendingMoveGroup = cycleString(p.pendingMoveGroup, targets)
}

func (p *MemoryPanel) openNewGroupModal() {
	p.pendingGroup = ""
	p.pendingOriginalGroup, p.pendingMoveGroup = "", ""
	p.pendingGroupColor = 0
	p.modal, p.modalPressed = "new-group", 0
}

func (p *MemoryPanel) groupExists(name string) bool {
	for _, group := range p.groups {
		if strings.EqualFold(strings.TrimSpace(group), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func (p *MemoryPanel) commitNewGroup() {
	name := strings.TrimSpace(p.pendingGroup)
	if name == "" || strings.EqualFold(name, "ALL") || strings.EqualFold(name, "UNGROUPED") || p.groupExists(name) {
		return
	}
	p.groupColors[name] = colorHex(memoryGroupPalette[p.pendingGroupColor])
	p.saveGroupColors()
	p.rebuildGroups()
	p.selectedGroup, p.selected, p.scrollOffset = name, -1, 0
	p.closeModal()
}

func (p *MemoryPanel) SetVisible(visible bool) {
	for _, c := range p.controls {
		c.SetVisible(visible)
	}
	if visible && p.edit != nil {
		if p.selected >= 0 && p.selected < len(p.memories) {
			p.edit.SetLabel("EDIT MEMORY")
			p.edit.SetEnabled(true)
		} else if p.selectedGroup != "ALL" {
			p.edit.SetLabel("EDIT GROUP")
			p.edit.SetEnabled(true)
		} else {
			p.edit.SetLabel("EDIT MEMORY")
			p.edit.SetEnabled(false)
		}
	}
}

func (p *MemoryPanel) SetMarkersVisible(visible bool) {
	p.markersVisible = visible
	if p.viewSwitch != nil {
		p.viewSwitch.SetActive(visible)
	}
}

func (p *MemoryPanel) load() {
	data, err := os.ReadFile(p.path)
	if err == nil && json.Unmarshal(data, &p.memories) == nil {
		if p.normalizeMemoryModes() || p.normalizeMemoryGroups() {
			p.save()
		}
		p.rebuildGroups()
		return
	}
	data, err = os.ReadFile(resources.Path("data", "ic-sdr-settings.json"))
	if err != nil {
		return
	}
	var legacy struct {
		ScanMemories []struct {
			Name        string `json:"name"`
			FrequencyHz int64  `json:"frequencyHz"`
			Mode        string `json:"mode"`
			FilterIndex int    `json:"filterIndex"`
			StepHz      int64  `json:"stepHz"`
			ScanEnabled bool   `json:"scanEnabled"`
			Group       string `json:"group"`
			Priority    bool   `json:"priority"`
		} `json:"scanMemories"`
	}
	if json.Unmarshal(data, &legacy) != nil {
		return
	}
	for _, m := range legacy.ScanMemories {
		catalog := filterCatalog[filterMode(m.Mode)]
		bw := 0
		if m.FilterIndex >= 0 && m.FilterIndex < len(catalog) {
			bw = catalog[m.FilterIndex].BandwidthHz
		}
		p.memories = append(p.memories, MemoryEntry{Name: m.Name, FrequencyHz: m.FrequencyHz, Mode: m.Mode, FilterBandwidthHz: bw, StepHz: m.StepHz, ScanEnabled: m.ScanEnabled, Group: m.Group, Priority: m.Priority})
	}
	p.normalizeMemoryModes()
	p.normalizeMemoryGroups()
	p.rebuildGroups()
}

// Older IC-SDR settings could contain a DMR-labelled channel stored as NFM.
// Treat explicit DMR names/groups as digital memories and persist the repair.
func (p *MemoryPanel) normalizeMemoryModes() bool {
	changed := false
	for index := range p.memories {
		memory := &p.memories[index]
		mode := strings.ToUpper(strings.TrimSpace(memory.Mode))
		looksDMR := strings.Contains(strings.ToUpper(memory.Name), "DMR") || strings.Contains(strings.ToUpper(memory.Group), "DMR")
		if mode == "DMR" || mode == "DMR BETA" || (mode == "NFM" && looksDMR) {
			if memory.Mode != "DMR BETA" {
				memory.Mode, changed = "DMR BETA", true
			}
			if memory.FilterBandwidthHz < 8_000 || memory.FilterBandwidthHz > 18_000 {
				memory.FilterBandwidthHz, changed = 12_500, true
			}
		}
	}
	return changed
}

func englishMemoryGroup(name string) string {
	switch strings.TrimSpace(name) {
	case "SIN GRUPO":
		return "UNGROUPED"
	case "TODAS":
		return "ALL"
	case "AEREA BCN":
		return "AIR BCN"
	case "MARITIMA VHF":
		return "MARINE VHF"
	case "MIL AEREA BCN":
		return "MIL AIR BCN"
	case "REPETIDORES":
		return "REPEATERS"
	case "APRS ESP":
		return "APRS SPAIN"
	default:
		return name
	}
}

func (p *MemoryPanel) normalizeMemoryGroups() bool {
	changed := false
	for index := range p.memories {
		if next := englishMemoryGroup(p.memories[index].Group); next != p.memories[index].Group {
			p.memories[index].Group = next
			changed = true
		}
	}
	if next := englishMemoryGroup(p.selectedGroup); next != p.selectedGroup {
		p.selectedGroup = next
		changed = true
	}
	return changed
}

func (p *MemoryPanel) normalizeGroupColorKeys() bool {
	changed := false
	next := map[string]string{}
	for group, color := range p.groupColors {
		name := englishMemoryGroup(group)
		if name != group {
			changed = true
		}
		if _, exists := next[name]; !exists {
			next[name] = color
		}
	}
	if changed {
		p.groupColors = next
	}
	return changed
}

func (p *MemoryPanel) save() {
	_ = os.MkdirAll(filepath.Dir(p.path), 0755)
	data, _ := json.MarshalIndent(p.memories, "", "  ")
	_ = os.WriteFile(p.path, data, 0644)
}
func (p *MemoryPanel) rebuildGroups() {
	seen := map[string]bool{}
	p.groups = []string{"ALL"}
	custom := make([]string, 0, len(p.groupColors))
	for group := range p.groupColors {
		if group != "" && group != "ALL" && group != "UNGROUPED" {
			custom = append(custom, group)
		}
	}
	sort.Strings(custom)
	for _, group := range custom {
		seen[group] = true
		p.groups = append(p.groups, group)
	}
	for _, m := range p.memories {
		g := memoryGroup(m)
		if !seen[g] {
			seen[g] = true
			p.groups = append(p.groups, g)
		}
	}
	if p.selectedGroup == "" {
		p.selectedGroup = "ALL"
	}
	found := p.selectedGroup == "ALL"
	for _, group := range p.groups {
		found = found || group == p.selectedGroup
	}
	if !found {
		p.selectedGroup = "ALL"
	}
}

func (p *MemoryPanel) openSaveModal() {
	name := fmt.Sprintf("MEM %02d", len(p.memories)+1)
	group := "UNGROUPED"
	if p.selectedGroup != "" && p.selectedGroup != "ALL" {
		group = p.selectedGroup
	}
	p.pendingMemory = MemoryEntry{Name: name, FrequencyHz: p.screen.frequencyHz, Mode: p.screen.mode.SelectedText(), FilterBandwidthHz: p.screen.demodBandwidthHz, StepHz: p.screen.tuningStepHz, ScanEnabled: true, Group: group}
	if p.screen.subtonePanel != nil {
		status := p.screen.subtonePanel.status()
		if status.Detected && status.Kind == "CTCSS" {
			p.pendingMemory.CTCSSHz = status.Value
		}
		if status.Detected && status.Kind == "DCS" {
			p.pendingMemory.DCSCode = status.Value
		}
	}
	p.duplicateNames = p.memoriesAtFrequency(p.pendingMemory.FrequencyHz)
	p.modal, p.modalPressed, p.editField, p.editError = "create", 0, 0, ""
	p.loadEditBuffer()
}

func (p *MemoryPanel) commitCurrent() {
	if !p.storeEditBuffer() {
		return
	}
	p.duplicateNames = p.memoriesAtFrequency(p.pendingMemory.FrequencyHz)
	p.memories = append(p.memories, p.pendingMemory)
	p.selected = len(p.memories) - 1
	p.rebuildGroups()
	p.save()
	if p.screen.scanPanel != nil {
		p.screen.scanPanel.ShowMemorySaved(p.pendingMemory.Name)
	}
	p.closeModal()
}

func (p *MemoryPanel) memoriesAtFrequency(frequencyHz int64) []string {
	names := []string{}
	for _, memory := range p.memories {
		if memory.FrequencyHz == frequencyHz {
			names = append(names, memory.Name)
		}
	}
	return names
}
func (p *MemoryPanel) tuneSelected() {
	if p.selected >= 0 && p.selected < len(p.memories) {
		p.recall(p.memories[p.selected])
	}
}

// updateSelectedFromVFO applies the receiver's current tuning parameters to
// the selected memory while preserving its name, group and scan flags.
func (p *MemoryPanel) updateSelectedFromVFO() {
	if p.selected < 0 || p.selected >= len(p.memories) {
		return
	}
	m := &p.memories[p.selected]
	m.FrequencyHz = p.screen.frequencyHz
	m.Mode = p.screen.mode.SelectedText()
	m.FilterBandwidthHz = p.screen.demodBandwidthHz
	m.StepHz = p.screen.tuningStepHz
	p.save()
}

// moveSelectedToNextGroup provides a compact group move action until the full
// memory editor modal is added. Each press advances through the known groups.
func (p *MemoryPanel) moveSelectedToNextGroup() {
	if p.selected < 0 || p.selected >= len(p.memories) {
		return
	}
	groups := []string{}
	if len(p.groups) > 1 {
		groups = p.groups[1:]
	}
	if len(groups) == 0 {
		groups = []string{"UNGROUPED"}
	}
	current := p.memories[p.selected].Group
	next := 0
	for i, group := range groups {
		if group == current {
			next = (i + 1) % len(groups)
			break
		}
	}
	p.memories[p.selected].Group = groups[next]
	p.rebuildGroups()
	p.save()
}
func (p *MemoryPanel) duplicateSelected() {
	if p.selected < 0 || p.selected >= len(p.memories) {
		return
	}
	m := p.memories[p.selected]
	m.Name += " COPY"
	p.memories = append(p.memories, m)
	p.selected = len(p.memories) - 1
	p.save()
}
func (p *MemoryPanel) openDeleteModal() {
	if p.selected < 0 || p.selected >= len(p.memories) {
		return
	}
	p.pendingDeleteIndex = p.selected
	p.modal, p.modalPressed = "delete", 0
}

func (p *MemoryPanel) confirmDelete() {
	if p.pendingDeleteIndex < 0 || p.pendingDeleteIndex >= len(p.memories) {
		p.closeModal()
		return
	}
	p.memories = append(p.memories[:p.pendingDeleteIndex], p.memories[p.pendingDeleteIndex+1:]...)
	p.selected = -1
	p.rebuildGroups()
	p.save()
	p.closeModal()
}

func (p *MemoryPanel) closeModal() {
	p.modal, p.modalPressed = "", 0
	p.pendingDeleteIndex = -1
	p.duplicateNames = nil
	p.pendingGroup = ""
	p.pendingEditIndex, p.editField, p.editBuffer, p.editError = -1, 0, "", ""
}

func (p *MemoryPanel) Update(simpleui.Input) bool { return false }
func (p *MemoryPanel) Draw()                      {}
func (p *MemoryPanel) OverlayOpen() bool          { return p.modal != "" }

func (p *MemoryPanel) modalCancelBounds() rl.Rectangle {
	if p.memoryEditorOpen() {
		return rl.Rectangle{X: 520, Y: 715, Width: 220, Height: 52}
	}
	if p.modal == "group" {
		return rl.Rectangle{X: 520, Y: 650, Width: 220, Height: 52}
	}
	return rl.Rectangle{X: 520, Y: 510, Width: 220, Height: 52}
}

func (p *MemoryPanel) modalConfirmBounds() rl.Rectangle {
	if p.memoryEditorOpen() {
		return rl.Rectangle{X: 860, Y: 715, Width: 220, Height: 52}
	}
	if p.modal == "group" {
		return rl.Rectangle{X: 860, Y: 650, Width: 220, Height: 52}
	}
	return rl.Rectangle{X: 860, Y: 510, Width: 220, Height: 52}
}

func (p *MemoryPanel) UpdateOverlay(input simpleui.Input) bool {
	if p.modal == "" {
		return false
	}
	if rl.IsKeyPressed(rl.KeyEscape) {
		p.closeModal()
		return true
	}
	if p.modal == "new-group" || p.modal == "group" {
		if rl.IsKeyPressed(rl.KeyBackspace) && len([]rune(p.pendingGroup)) > 0 {
			runes := []rune(p.pendingGroup)
			p.pendingGroup = string(runes[:len(runes)-1])
		}
		for character := rl.GetCharPressed(); character > 0; character = rl.GetCharPressed() {
			if character >= 32 && character != 127 && len([]rune(p.pendingGroup)) < 24 {
				p.pendingGroup += string(character)
			}
		}
		if rl.IsKeyPressed(rl.KeyEnter) {
			if p.modal == "group" {
				p.commitGroupColor()
			} else {
				p.commitNewGroup()
			}
			return true
		}
	}
	if p.memoryEditorOpen() {
		editable := p.editField == 0 || p.editField == 1 || p.editField == 4 || p.editField == 5 || p.editField == 6
		if editable && rl.IsKeyPressed(rl.KeyBackspace) && len([]rune(p.editBuffer)) > 0 {
			r := []rune(p.editBuffer)
			p.editBuffer = string(r[:len(r)-1])
		}
		for ch := rl.GetCharPressed(); ch > 0; ch = rl.GetCharPressed() {
			validNumber := ch >= '0' && ch <= '9' || ch == '.' || ch == ','
			limit := 32
			if p.editField == 6 {
				limit = 160
			}
			textField := p.editField == 0 || p.editField == 6
			if ch >= 32 && ch != 127 && len([]rune(p.editBuffer)) < limit && (textField || editable && validNumber) {
				p.editBuffer += string(ch)
			}
		}
		if rl.IsKeyPressed(rl.KeyTab) && p.storeEditBuffer() {
			p.editField = (p.editField + 1) % 7
			p.loadEditBuffer()
		}
		if rl.IsKeyPressed(rl.KeyEnter) {
			if p.modal == "create" {
				p.commitCurrent()
			} else {
				p.commitEdit()
			}
			return true
		}
	}
	if input.Pressed {
		p.modalPressed = 0
		if p.modal == "group" || p.modal == "new-group" {
			for i, bounds := range p.groupColorBounds() {
				if input.Over(bounds) {
					p.pendingGroupColor = i
					p.modalPressed = 10 + i
					return true
				}
			}
			if p.modal == "group" {
				if input.Over(rl.Rectangle{X: 500, Y: 450, Width: 285, Height: 42}) {
					p.pendingGroupScan = !p.pendingGroupScan
					return true
				}
				if input.Over(rl.Rectangle{X: 815, Y: 450, Width: 285, Height: 42}) {
					p.pendingGroupPriority = !p.pendingGroupPriority
					return true
				}
				if input.Over(rl.Rectangle{X: 500, Y: 520, Width: 285, Height: 48}) {
					p.moveEditedGroup()
					return true
				}
				if input.Over(rl.Rectangle{X: 815, Y: 520, Width: 285, Height: 48}) {
					p.deleteEditedGroup()
					return true
				}
			}
		}
		if p.memoryEditorOpen() {
			for i, bounds := range p.editFieldBounds() {
				if input.Over(bounds) {
					if !p.storeEditBuffer() {
						return true
					}
					p.editField = i
					if i == 2 {
						p.cycleEditGroup()
					} else if i == 3 {
						p.cycleEditMode()
					}
					p.loadEditBuffer()
					return true
				}
			}
			if input.Over(rl.Rectangle{X: 500, Y: 620, Width: 285, Height: 42}) {
				p.pendingMemory.ScanEnabled = !p.pendingMemory.ScanEnabled
				return true
			}
			if input.Over(rl.Rectangle{X: 815, Y: 620, Width: 285, Height: 42}) {
				p.pendingMemory.Priority = !p.pendingMemory.Priority
				return true
			}
		}
		if input.Over(p.modalCancelBounds()) {
			p.modalPressed = 1
		} else if input.Over(p.modalConfirmBounds()) {
			p.modalPressed = 2
		}
	}
	if input.Released {
		if p.modalPressed == 1 && input.Over(p.modalCancelBounds()) {
			simpleui.PlayActivationFeedback()
			p.closeModal()
		} else if p.modalPressed == 2 && input.Over(p.modalConfirmBounds()) {
			simpleui.PlayActivationFeedback()
			if p.modal == "create" {
				p.commitCurrent()
			} else if p.modal == "group" {
				p.commitGroupColor()
			} else if p.modal == "new-group" {
				p.commitNewGroup()
			} else if p.modal == "edit" {
				p.commitEdit()
			} else {
				p.confirmDelete()
			}
		}
		p.modalPressed = 0
	}
	return true
}

func (p *MemoryPanel) DrawOverlay() {
	if p.modal == "" {
		return
	}
	rl.DrawRectangle(0, 0, int32(designWidth), int32(designHeight), rl.Color{A: 220})
	modal := rl.Rectangle{X: 440, Y: 260, Width: 720, Height: 330}
	if p.memoryEditorOpen() {
		modal = rl.Rectangle{X: 440, Y: 105, Width: 720, Height: 690}
	} else if p.modal == "group" {
		modal = rl.Rectangle{X: 440, Y: 170, Width: 720, Height: 560}
	}
	rl.DrawRectangleRounded(modal, .035, 8, colors.panel)
	accent := colors.green
	title := T("SAVE MEMORY")
	confirm := T("SAVE")
	if p.modal == "create" && len(p.duplicateNames) > 0 {
		confirm = T("SAVE DUPLICATE")
	}
	if p.modal == "delete" {
		accent, title, confirm = colors.red, T("DELETE MEMORY"), T("DELETE")
	} else if p.modal == "group" {
		accent, title, confirm = memoryGroupPalette[p.pendingGroupColor], T("EDIT GROUP"), T("SAVE CHANGES")
	} else if p.modal == "new-group" {
		accent, title, confirm = memoryGroupPalette[p.pendingGroupColor], T("NEW GROUP"), T("CREATE GROUP")
	}
	if p.modal == "edit" {
		accent, title, confirm = colors.cyan, T("EDIT MEMORY"), T("SAVE CHANGES")
	} else if p.modal == "create" {
		accent, title, confirm = colors.green, T("NEW MEMORY"), T("SAVE MEMORY")
	}
	rl.DrawRectangleRoundedLinesEx(modal, .035, 8, 2, accent)
	titleBounds := rl.Rectangle{X: 470, Y: 286, Width: 660, Height: 34}
	if p.memoryEditorOpen() {
		titleBounds.Y = 126
	} else if p.modal == "group" {
		titleBounds.Y = 192
	}
	drawCentered(title, titleBounds, 22, accent)
	if p.memoryEditorOpen() {
		p.drawEditModalContent()
	} else if p.modal == "group" {
		p.drawGroupModalContent()
	} else if p.modal == "new-group" {
		p.drawNewGroupModalContent()
	} else {
		p.drawDeleteModalContent()
	}
	drawModalAction(p.modalCancelBounds(), T("CANCEL"), colors.border, p.modalPressed == 1)
	drawModalAction(p.modalConfirmBounds(), confirm, accent, p.modalPressed == 2)
}

func (p *MemoryPanel) memoryEditorOpen() bool { return p.modal == "edit" || p.modal == "create" }

func (p *MemoryPanel) editFieldBounds() []rl.Rectangle {
	return []rl.Rectangle{
		{X: 500, Y: 205, Width: 285, Height: 52}, {X: 815, Y: 205, Width: 285, Height: 52},
		{X: 500, Y: 305, Width: 285, Height: 52}, {X: 815, Y: 305, Width: 285, Height: 52},
		{X: 500, Y: 405, Width: 285, Height: 52}, {X: 815, Y: 405, Width: 285, Height: 52},
		{X: 500, Y: 515, Width: 600, Height: 58},
	}
}

func (p *MemoryPanel) editGroups() []string {
	result := []string{"UNGROUPED"}
	for _, group := range p.groups {
		if group != "" && group != "ALL" && group != "UNGROUPED" {
			result = append(result, group)
		}
	}
	return result
}

func cycleString(current string, values []string) string {
	if len(values) == 0 {
		return current
	}
	for i, value := range values {
		if strings.EqualFold(value, current) {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}

func (p *MemoryPanel) cycleEditGroup() {
	p.pendingMemory.Group = cycleString(p.pendingMemory.Group, p.editGroups())
}
func (p *MemoryPanel) cycleEditMode() {
	modes := []string{"AM", "NFM", "WFM", "USB", "LSB", "CW", "RAW", "DMR BETA", "TETRA", "ADS-B", "UAT"}
	if p.screen != nil && p.screen.mode != nil && len(p.screen.mode.Items()) > 0 {
		modes = p.screen.mode.Items()
	}
	p.pendingMemory.Mode = cycleString(p.pendingMemory.Mode, modes)
}

func (p *MemoryPanel) drawEditModalContent() {
	labels := []string{"NAME", "FREQUENCY (MHz)", "GROUP (CLICK TO CHANGE)", "MODE (CLICK TO CHANGE)", "FILTER (Hz)", "STEP (Hz)", "DESCRIPTION"}
	values := []string{p.pendingMemory.Name, fmt.Sprintf("%.6f", float64(p.pendingMemory.FrequencyHz)/1e6), p.pendingMemory.Group + "   >", p.pendingMemory.Mode + "   >", strconv.Itoa(p.pendingMemory.FilterBandwidthHz), strconv.FormatInt(p.pendingMemory.StepHz, 10), trimMemory(p.pendingMemory.Description, 72)}
	for i, bounds := range p.editFieldBounds() {
		simpleui.DrawText(labels[i], bounds.X, bounds.Y-22, 13, colors.muted)
		border := colors.border
		if i == p.editField {
			border = colors.cyan
			if i == 0 || i == 1 || i == 4 || i == 5 || i == 6 {
				values[i] = trimMemory(p.editBuffer, 72) + "|"
			}
		}
		rl.DrawRectangleRounded(bounds, .1, 6, simpleui.CurrentTheme().InputBackground)
		rl.DrawRectangleRoundedLinesEx(bounds, .1, 6, 2, border)
		simpleui.DrawText(values[i], bounds.X+14, bounds.Y+16, 17, colors.text)
	}
	drawEditToggle := func(bounds rl.Rectangle, label string, active bool) {
		accent := colors.border
		state := "OFF"
		if active {
			accent, state = colors.green, "ON"
		}
		rl.DrawRectangleRounded(bounds, .12, 6, colors.panelAlt)
		rl.DrawRectangleRoundedLinesEx(bounds, .12, 6, 2, accent)
		drawCentered(label+"  "+state, bounds, 14, colors.text)
	}
	drawEditToggle(rl.Rectangle{X: 500, Y: 620, Width: 285, Height: 42}, "INCLUDE IN SCANNER", p.pendingMemory.ScanEnabled)
	drawEditToggle(rl.Rectangle{X: 815, Y: 620, Width: 285, Height: 42}, "PRIORITY", p.pendingMemory.Priority)
	if p.editError != "" {
		drawCentered(p.editError, rl.Rectangle{X: 490, Y: 675, Width: 620, Height: 22}, 13, colors.red)
	} else if p.modal == "create" && len(p.memoriesAtFrequency(p.pendingMemory.FrequencyHz)) > 0 {
		drawCentered(T("Note: another memory already exists on this frequency."), rl.Rectangle{X: 490, Y: 675, Width: 620, Height: 22}, 13, colors.orange)
	}
}

func (p *MemoryPanel) groupColorBounds() []rl.Rectangle {
	if p.modal == "group" {
		bounds := make([]rl.Rectangle, len(memoryGroupPalette))
		for i := range bounds {
			bounds[i] = rl.Rectangle{X: 570 + float32(i)*64, Y: 350, Width: 42, Height: 42}
		}
		return bounds
	}
	bounds := make([]rl.Rectangle, len(memoryGroupPalette))
	for i := range bounds {
		bounds[i] = rl.Rectangle{X: 500 + float32(i%4)*150, Y: 374 + float32(i/4)*75, Width: 118, Height: 48}
	}
	return bounds
}
func (p *MemoryPanel) drawGroupModalContent() {
	simpleui.DrawText(T("GROUP NAME"), 500, 252, 14, colors.muted)
	field := rl.Rectangle{X: 500, Y: 278, Width: 600, Height: 48}
	rl.DrawRectangleRounded(field, .1, 6, simpleui.CurrentTheme().InputBackground)
	rl.DrawRectangleRoundedLinesEx(field, .1, 6, 2, colors.cyan)
	simpleui.DrawText(p.pendingGroup+"|", 514, 293, 17, colors.text)
	simpleui.DrawText(T("COLOR"), 500, 362, 14, colors.muted)
	for i, bounds := range p.groupColorBounds() {
		c := memoryGroupPalette[i]
		rl.DrawCircle(int32(bounds.X+bounds.Width/2), int32(bounds.Y+bounds.Height/2), 14, c)
		border, width := colors.text, float32(1)
		if i == p.pendingGroupColor {
			border = rl.White
			width = 3
		}
		rl.DrawCircleLines(int32(bounds.X+bounds.Width/2), int32(bounds.Y+bounds.Height/2), 18, border)
		_ = width
	}
	count := 0
	for _, memory := range p.memories {
		if memoryGroup(memory) == p.pendingOriginalGroup {
			count++
		}
	}
	simpleui.DrawTextStyled(fmt.Sprintf("%d %s", count, T("MEMORIES")), 500, 412, 15, simpleui.FontSemiBold, colors.text)
	drawGroupToggle := func(bounds rl.Rectangle, label string, active bool) {
		accent, state := colors.border, "OFF"
		if active {
			accent, state = colors.green, "ON"
		}
		rl.DrawRectangleRounded(bounds, .12, 6, colors.panelAlt)
		rl.DrawRectangleRoundedLinesEx(bounds, .12, 6, 2, accent)
		drawCentered(label+"  "+state, bounds, 14, colors.text)
	}
	drawGroupToggle(rl.Rectangle{X: 500, Y: 450, Width: 285, Height: 42}, "INCLUDE IN SCANNER", p.pendingGroupScan)
	drawGroupToggle(rl.Rectangle{X: 815, Y: 450, Width: 285, Height: 42}, "PRIORITY", p.pendingGroupPriority)
	moveLabel := "KEEP GROUP"
	if p.pendingMoveGroup != "" {
		moveLabel = "MOVE TO: " + p.pendingMoveGroup
	}
	drawModalAction(rl.Rectangle{X: 500, Y: 520, Width: 285, Height: 48}, moveLabel, colors.border, false)
	drawModalAction(rl.Rectangle{X: 815, Y: 520, Width: 285, Height: 48}, "DELETE GROUP", colors.red, false)
	if p.editError != "" {
		drawCentered(p.editError, rl.Rectangle{X: 490, Y: 605, Width: 620, Height: 22}, 13, colors.red)
	}
}

func (p *MemoryPanel) drawNewGroupModalContent() {
	field := rl.Rectangle{X: 500, Y: 330, Width: 600, Height: 38}
	rl.DrawRectangleRounded(field, .12, 6, simpleui.CurrentTheme().InputBackground)
	rl.DrawRectangleRoundedLinesEx(field, .12, 6, 2, memoryGroupPalette[p.pendingGroupColor])
	name := p.pendingGroup
	if name == "" {
		name = "Type the group name…"
		simpleui.DrawText(name, field.X+12, field.Y+10, 14, colors.muted)
	} else {
		simpleui.DrawText(name+"|", field.X+12, field.Y+10, 14, colors.text)
	}
	for i, bounds := range p.groupColorBounds() {
		c := memoryGroupPalette[i]
		rl.DrawRectangleRounded(bounds, .18, 6, c)
		width := float32(1)
		if i == p.pendingGroupColor {
			width = 4
		}
		rl.DrawRectangleRoundedLinesEx(bounds, .18, 6, width, colors.text)
	}
	message := "Type a name and pick a color. Press Enter to create."
	if p.groupExists(p.pendingGroup) || strings.EqualFold(strings.TrimSpace(p.pendingGroup), "ALL") || strings.EqualFold(strings.TrimSpace(p.pendingGroup), "UNGROUPED") {
		message = "That name already exists or is reserved."
	}
	drawCentered(message, rl.Rectangle{X: 490, Y: 488, Width: 620, Height: 18}, 12, colors.muted)
}

func (p *MemoryPanel) drawSaveModalContent() {
	drawCentered(p.pendingMemory.Name, rl.Rectangle{X: 490, Y: 338, Width: 620, Height: 28}, 18, colors.text)
	detail := fmt.Sprintf("%.6f MHz  ·  %s  ·  %s", float64(p.pendingMemory.FrequencyHz)/1e6, p.pendingMemory.Mode, formatFilterBandwidth(p.pendingMemory.FilterBandwidthHz))
	drawCentered(detail, rl.Rectangle{X: 490, Y: 374, Width: 620, Height: 28}, 16, colors.cyan)
	if len(p.duplicateNames) > 0 {
		drawCentered(T("THIS FREQUENCY IS ALREADY STORED"), rl.Rectangle{X: 490, Y: 420, Width: 620, Height: 28}, 16, colors.orange)
		drawCentered("Existing: "+strings.Join(p.duplicateNames, ", "), rl.Rectangle{X: 490, Y: 452, Width: 620, Height: 24}, 13, colors.text)
		drawCentered(T("A second memory will be saved on the same frequency."), rl.Rectangle{X: 490, Y: 478, Width: 620, Height: 22}, 12, colors.muted)
	}
}

func (p *MemoryPanel) drawDeleteModalContent() {
	if p.pendingDeleteIndex < 0 || p.pendingDeleteIndex >= len(p.memories) {
		return
	}
	memory := p.memories[p.pendingDeleteIndex]
	drawCentered(memory.Name, rl.Rectangle{X: 490, Y: 352, Width: 620, Height: 32}, 20, colors.text)
	drawCentered(fmt.Sprintf("%.6f MHz  ·  %s", float64(memory.FrequencyHz)/1e6, memory.Mode), rl.Rectangle{X: 490, Y: 394, Width: 620, Height: 28}, 16, colors.cyan)
	drawCentered(T("This action cannot be undone."), rl.Rectangle{X: 490, Y: 448, Width: 620, Height: 24}, 14, colors.muted)
}

func drawModalAction(bounds rl.Rectangle, label string, accent rl.Color, pressed bool) {
	background := colors.panelAlt
	if pressed {
		background = accent
	}
	rl.DrawRectangleRounded(bounds, .12, 7, background)
	rl.DrawRectangleRoundedLinesEx(bounds, .12, 7, 2, accent)
	drawCentered(label, bounds, 16, simpleui.EnsureTextContrast(colors.text, background))
}

func (p *MemoryPanel) recall(m MemoryEntry) {
	mode := strings.ToUpper(strings.TrimSpace(m.Mode))
	if mode == "DMR" {
		mode = "DMR BETA"
	}
	if mode == "NFM" && (strings.Contains(strings.ToUpper(m.Name), "DMR") || strings.Contains(strings.ToUpper(m.Group), "DMR")) {
		mode = "DMR BETA"
	}
	if mode == "DMR BETA" {
		// A manual recall owns the workspace, unlike a scanner recall, and should
		// expose decoder state and slot activity immediately.
		p.screen.selectTool("DMR_MONITOR")
	}
	for i, value := range p.screen.mode.Items() {
		if value == mode {
			p.screen.mode.SetSelected(i)
			p.screen.savedMode = mode
			break
		}
	}
	if m.FilterBandwidthHz > 0 {
		catalog := filterCatalog[filterMode(mode)]
		preset := catalog[0]
		best := absInt(preset.BandwidthHz - m.FilterBandwidthHz)
		for _, candidate := range catalog {
			if d := absInt(candidate.BandwidthHz - m.FilterBandwidthHz); d < best {
				best, preset = d, candidate
			}
		}
		p.screen.selectFilter(preset)
	}
	if m.StepHz > 0 {
		p.screen.tuningStepHz = m.StepHz
		if p.screen.stepSelector != nil {
			p.screen.stepSelector.SetSelected(m.StepHz)
		}
	}
	p.screen.frequencyHz = m.FrequencyHz
	p.screen.updateBandForFrequency(m.FrequencyHz)
	half := p.screen.spanHz / 2
	if p.screen.centerMode || m.FrequencyHz < p.screen.centerFrequencyHz-half || m.FrequencyHz > p.screen.centerFrequencyHz+half {
		p.screen.centerFrequencyHz = m.FrequencyHz
		if p.screen.receiver != nil {
			p.screen.receiver.SetCenterFrequency(m.FrequencyHz)
		}
	}
	if p.screen.receiver != nil {
		p.screen.receiver.SetDemodulator(mode, m.FrequencyHz, p.screen.demodBandwidthHz)
	}
	p.screen.markSettingsDirty()
}

func (p *MemoryPanel) DrawPanel() {
	drawPanel(30, memoryPanelTop, 205, memoryPanelH)
	drawColumnHeader("GROUPS", 42, memoryHeaderY, colors.cyan)
	for i, g := range p.groups {
		if i >= 6 {
			break
		}
		y := memoryFirstRowY + float32(i)*21
		groupColor := p.groupColor(g)
		countColor := colors.muted
		if g == p.selectedGroup {
			selection, selectionText := tableSelectionColors()
			rl.DrawRectangleRounded(rl.Rectangle{X: 36, Y: y - 5, Width: 192, Height: 20}, .15, 4, selection)
			groupColor, countColor = selectionText, selectionText
		}
		rl.DrawCircle(44, int32(y+4), 4, groupColor)
		drawMemoryText(g, 54, y, groupColor)
		count := 0
		for _, m := range p.memories {
			if g == "ALL" || memoryGroup(m) == g {
				count++
			}
		}
		drawMemoryText(fmt.Sprintf("%d", count), 205, y, countColor)
	}
	drawPanel(245, memoryPanelTop, 1100, memoryPanelH)
	headers := []string{"ACT", "NAME", "FREQUENCY", "MODE", "FILTER", "STEP", "PRI"}
	xs := []float32{255, 300, 600, 800, 900, 1030, 1180}
	for i, h := range headers {
		drawColumnHeader(h, xs[i], memoryHeaderY, colors.text)
	}
	rl.DrawLineEx(rl.Vector2{X: 251, Y: 663}, rl.Vector2{X: 1338, Y: 663}, 1, colors.border)
	indices := p.filteredIndices()
	p.clampScroll(len(indices))
	end := min(p.scrollOffset+memoryVisibleRows, len(indices))
	for row, index := range indices[p.scrollOffset:end] {
		m := p.memories[index]
		yy := memoryFirstRowY + float32(row)*memoryRowH
		groupColor := p.groupColor(memoryGroup(m))
		rowText, activeColor, starColor := colors.text, colors.cyan, colors.orange
		if index == p.selected {
			selection, selectionText := tableSelectionColors()
			rl.DrawRectangle(248, int32(yy-3), 1094, 22, selection)
			groupColor, rowText, activeColor, starColor = selectionText, selectionText, selectionText, selectionText
		}
		rl.DrawRectangle(248, int32(yy-3), 4, 22, groupColor)
		active := "□"
		if m.ScanEnabled {
			active = "■"
		}
		star := ""
		if m.Priority {
			star = "★"
		}
		drawMemoryText(active, 257, yy, activeColor)
		drawMemoryText(trimMemory(m.Name, 20), 300, yy, groupColor)
		drawMemoryText(fmt.Sprintf("%.6f", float64(m.FrequencyHz)/1e6), 600, yy, rowText)
		drawMemoryText(m.Mode, 800, yy, rowText)
		drawMemoryText(formatFilterBandwidth(m.FilterBandwidthHz), 900, yy, rowText)
		drawMemoryText(formatStep(m.StepHz), 1030, yy, rowText)
		drawMemoryText(star, 1180, yy, starColor)
	}
	if len(indices) > memoryVisibleRows {
		trackY, trackH := float32(668), float32(145)
		thumbH := max(float32(22), trackH*memoryVisibleRows/float32(len(indices)))
		thumbY := trackY + (trackH-thumbH)*float32(p.scrollOffset)/float32(len(indices)-memoryVisibleRows)
		rl.DrawRectangleRounded(rl.Rectangle{X: 1337, Y: trackY, Width: 4, Height: trackH}, 1, 4, colors.grid)
		rl.DrawRectangleRounded(rl.Rectangle{X: 1337, Y: thumbY, Width: 4, Height: thumbH}, 1, 4, colors.cyan)
	}
}

func (p *MemoryPanel) Tick() {
	if p.modal != "" || p.screen.activeTool != "MEMORIES" || p.screen.viewMode != 1 {
		return
	}
	mouse := simpleui.MousePosition()
	indices := p.filteredIndices()
	p.clampScroll(len(indices))
	if mouse.X >= 245 && mouse.X <= 1345 && mouse.Y >= 665 && mouse.Y < 820 {
		if wheel := rl.GetMouseWheelMove(); wheel != 0 {
			p.scrollOffset -= int(wheel)
			p.clampScroll(len(indices))
		}
	}
	if rl.IsKeyPressed(rl.KeyDown) || rl.IsKeyPressedRepeat(rl.KeyDown) {
		p.moveSelection(indices, 1)
	}
	if rl.IsKeyPressed(rl.KeyUp) || rl.IsKeyPressedRepeat(rl.KeyUp) {
		p.moveSelection(indices, -1)
	}
	if rl.IsKeyPressed(rl.KeyHome) && len(indices) > 0 {
		p.selected = indices[0]
		p.ensureSelectionVisible(indices)
	}
	if rl.IsKeyPressed(rl.KeyEnd) && len(indices) > 0 {
		p.selected = indices[len(indices)-1]
		p.ensureSelectionVisible(indices)
	}
	if rl.IsKeyPressed(rl.KeyEnter) {
		p.tuneSelected()
	}
	if !rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		return
	}
	if mouse.X >= 30 && mouse.X <= 235 && mouse.Y >= memoryFirstRowY-5 && mouse.Y < memoryFirstRowY-5+6*21 {
		row := int((mouse.Y - (memoryFirstRowY - 5)) / 21)
		if row >= 0 && row < len(p.groups) && row < 6 {
			p.selectedGroup = p.groups[row]
			p.scrollOffset = 0
			p.selected = -1
			if p.selectedGroup == "ALL" {
				p.edit.SetLabel("EDIT MEMORY")
				p.edit.SetEnabled(false)
			} else {
				p.edit.SetLabel("EDIT GROUP")
				p.edit.SetEnabled(true)
			}
			return
		}
	}
	if mouse.X >= 245 && mouse.X <= 1345 && mouse.Y >= 668 && mouse.Y < 820 {
		wanted := int((mouse.Y - (memoryFirstRowY - 4)) / memoryRowH)
		visibleRow := p.scrollOffset + wanted
		if visibleRow >= 0 && visibleRow < len(indices) {
			i := indices[visibleRow]
			p.selected = i
			p.edit.SetLabel("EDIT MEMORY")
			p.edit.SetEnabled(true)
			if mouse.X < 290 {
				p.memories[i].ScanEnabled = !p.memories[i].ScanEnabled
				p.lastClicked = -1
				p.save()
			} else if mouse.X >= 1160 && mouse.X <= 1240 {
				p.memories[i].Priority = !p.memories[i].Priority
				p.lastClicked = -1
				p.save()
			} else {
				now := rl.GetTime()
				if p.lastClicked == i && now-p.lastClickAt <= .42 {
					p.recall(p.memories[i])
					p.lastClicked = -1
				} else {
					p.lastClicked, p.lastClickAt = i, now
				}
			}
		}
	}
}

func (p *MemoryPanel) filteredIndices() []int {
	indices := make([]int, 0, len(p.memories))
	for i, memory := range p.memories {
		groupMatches := p.selectedGroup == "ALL" || memoryGroup(memory) == p.selectedGroup
		if groupMatches && (!p.onlyActive || memory.ScanEnabled) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (p *MemoryPanel) clampScroll(count int) {
	p.scrollOffset = min(max(p.scrollOffset, 0), max(count-memoryVisibleRows, 0))
}

func (p *MemoryPanel) moveSelection(indices []int, delta int) {
	if len(indices) == 0 {
		return
	}
	position := -1
	for i, index := range indices {
		if index == p.selected {
			position = i
			break
		}
	}
	if position < 0 {
		position = 0
	} else {
		position = min(max(position+delta, 0), len(indices)-1)
	}
	p.selected = indices[position]
	p.ensureSelectionVisible(indices)
}

func (p *MemoryPanel) ensureSelectionVisible(indices []int) {
	for position, index := range indices {
		if index != p.selected {
			continue
		}
		if position < p.scrollOffset {
			p.scrollOffset = position
		} else if position >= p.scrollOffset+memoryVisibleRows {
			p.scrollOffset = position - memoryVisibleRows + 1
		}
		break
	}
	p.clampScroll(len(indices))
}

func drawMemoryText(text string, x, y float32, color rl.Color) {
	simpleui.DrawTextStyled(text, x, y-2, 12, simpleui.FontRegular, color)
}

func drawColumnHeader(text string, x, y float32, color rl.Color) {
	simpleui.DrawTextStyled(text, x, y-2, 11, simpleui.FontSemiBold, color)
}

func memoryGroup(memory MemoryEntry) string {
	if strings.TrimSpace(memory.Group) == "" {
		return "UNGROUPED"
	}
	return memory.Group
}

func (p *MemoryPanel) DrawMarkers(x, y, w, h float32) {
	// MEM VIEW is the sole authority for marker visibility. The scanner may
	// use memories for tuning without forcing their labels back onto the FFT.
	if !p.markersVisible {
		return
	}
	visible := p.visibleMarkerIndices()
	low := p.screen.centerFrequencyHz - p.screen.spanHz/2
	laneRight := []float32{x - 20, x - 20, x - 20, x - 20}
	for order, index := range visible {
		m := p.memories[index]
		mx := x + w*float32(m.FrequencyHz-low)/float32(p.screen.spanHz)
		color := p.groupColor(memoryGroup(m))
		if !m.ScanEnabled {
			color = blendRGBA(color, colors.muted, .65)
		}
		current := p.screen.scanPanel != nil && p.screen.scanPanel.running && p.screen.scanPanel.currentMemory == index
		if current {
			rl.DrawLineEx(rl.Vector2{X: mx, Y: y + 25}, rl.Vector2{X: mx, Y: y + h - 53}, 4, colors.green)
		}
		// Stop before the band-plan ribbon; memory markers never draw through it.
		rl.DrawLineEx(rl.Vector2{X: mx, Y: y + 26}, rl.Vector2{X: mx, Y: y + h - 53}, 1, color)
		label := fmt.Sprintf("%d", order+1)
		if !p.compact {
			label = trimMemory(m.Name, 12)
		}
		tw := simpleui.MeasureTextStyled(label, 11, simpleui.FontSemiBold).X
		left := min(max(mx-tw/2-6, x+52), x+w-tw-15)
		lane := -1
		for candidate := range laneRight {
			if left > laneRight[candidate]+5 {
				lane = candidate
				break
			}
		}
		if lane < 0 {
			continue // the frequency line remains visible; hover still reveals it
		}
		// Four dedicated memory lanes sit above the band-plan ribbon.
		labelY := y + h - 76 - float32(lane)*22
		bounds := rl.Rectangle{X: left, Y: labelY, Width: tw + 12, Height: 19}
		labelBackground := colors.panel
		labelBackground.A = 242
		rl.DrawRectangleRounded(bounds, .2, 4, labelBackground)
		rl.DrawRectangleRoundedLinesEx(bounds, .2, 4, 1, color)
		simpleui.DrawTextStyled(label, left+6, labelY+3, 11, simpleui.FontSemiBold, simpleui.EnsureTextContrast(color, labelBackground))
		laneRight[lane] = left + tw + 12
	}
}

func (p *MemoryPanel) DrawMarkerTooltip(x, y, w, h float32) bool {
	if !p.markersVisible || !simpleui.MouseInViewport() {
		return false
	}
	mouse := simpleui.MousePosition()
	if mouse.X < x || mouse.X > x+w || mouse.Y < y+24 || mouse.Y > y+h-53 {
		return false
	}
	low := p.screen.centerFrequencyHz - p.screen.spanHz/2
	hovered := -1
	nearest := float32(9)
	for _, index := range p.visibleMarkerIndices() {
		mx := x + w*float32(p.memories[index].FrequencyHz-low)/float32(p.screen.spanHz)
		if distance := absFloat32(mouse.X - mx); distance < nearest {
			hovered, nearest = index, distance
		}
	}
	if hovered < 0 {
		return false
	}
	m := p.memories[hovered]
	cardW, cardH := float32(390), float32(142)
	cardX := mouse.X + 16
	if cardX+cardW > x+w-5 {
		cardX = mouse.X - cardW - 16
	}
	cardX = min(max(cardX, x+52), x+w-cardW-5)
	cardY := min(max(mouse.Y-cardH/2, y+31), y+h-cardH-58)
	card := rl.Rectangle{X: cardX, Y: cardY, Width: cardW, Height: cardH}
	cardBackground := colors.panel
	cardBackground.A = 250
	rl.DrawRectangleRounded(card, .08, 5, cardBackground)
	groupColor := p.groupColor(memoryGroup(m))
	rl.DrawRectangleRoundedLinesEx(card, .08, 5, 2, groupColor)
	mainText := simpleui.EnsureTextContrast(colors.text, cardBackground)
	accentText := simpleui.EnsureTextContrast(groupColor, cardBackground)
	simpleui.DrawTextStyled(trimMemory(m.Name, 30), cardX+14, cardY+10, 17, simpleui.FontSemiBold, mainText)
	simpleui.DrawTextStyled(fmt.Sprintf("%.6f MHz   %s", float64(m.FrequencyHz)/1e6, m.Mode), cardX+14, cardY+39, 15, simpleui.FontMono, accentText)
	detail := fmt.Sprintf("FILTER %s   STEP %s", formatFilterBandwidth(m.FilterBandwidthHz), formatStep(m.StepHz))
	if m.CTCSSHz != "" {
		detail += "   CTCSS " + m.CTCSSHz
	} else if m.DCSCode != "" {
		detail += "   DCS " + m.DCSCode
	}
	simpleui.DrawTextStyled(detail, cardX+14, cardY+66, 13, simpleui.FontRegular, mainText)
	description := strings.TrimSpace(m.Description)
	if description == "" {
		description = "No description"
	}
	simpleui.DrawTextStyled(trimMemory(description, 51), cardX+14, cardY+91, 13, simpleui.FontRegular, mainText)
	status := "SCAN INACTIVE"
	if m.ScanEnabled {
		status = "SCAN ACTIVE"
	}
	priority := ""
	if m.Priority {
		priority = "  ·  PRIORITY"
	}
	simpleui.DrawTextStyled(memoryGroup(m)+"  ·  "+status+priority, cardX+14, cardY+118, 12, simpleui.FontSemiBold, accentText)
	return true
}

func (p *MemoryPanel) visibleMarkerIndices() []int {
	visible := []int{}
	low := p.screen.centerFrequencyHz - p.screen.spanHz/2
	high := low + p.screen.spanHz
	for i, memory := range p.memories {
		// The group and active-only choices belong to the memory table. The FFT
		// layer is controlled exclusively by MEM VIEW and therefore includes
		// every stored memory that falls inside the displayed span.
		if memory.FrequencyHz >= low && memory.FrequencyHz <= high {
			visible = append(visible, i)
		}
	}
	sort.Slice(visible, func(i, j int) bool { return p.memories[visible[i]].FrequencyHz < p.memories[visible[j]].FrequencyHz })
	return visible
}
func (p *MemoryPanel) HandleMarkerClick(point rl.Vector2, x, y, w, h float32) bool {
	if !p.markersVisible || point.Y < y+24 || point.Y > y+h-53 {
		return false
	}
	low := p.screen.centerFrequencyHz - p.screen.spanHz/2
	for _, i := range p.visibleMarkerIndices() {
		m := p.memories[i]
		mx := x + w*float32(m.FrequencyHz-low)/float32(p.screen.spanHz)
		if absFloat32(point.X-mx) <= 8 {
			p.selected = i
			p.recall(m)
			return true
		}
	}
	return false
}
func trimMemory(value string, n int) string {
	r := []rune(strings.TrimSpace(value))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func absFloat32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
