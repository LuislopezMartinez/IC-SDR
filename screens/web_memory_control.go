package screens

import "go-zero/internal/i18n"

import "strings"

func validMemoryGroup(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && len(name) <= 40 && !strings.EqualFold(name, i18n.Source("text.201f15dab8b3")) && !strings.EqualFold(name, i18n.Source("text.445a7d952a48"))
}

func errInvalidMemory(memory MemoryEntry) bool {
	if name := strings.TrimSpace(memory.Name); name == "" || len(name) > 80 {
		return true
	}
	if memory.FrequencyHz < 100_000 || memory.FrequencyHz > 6_000_000_000 || memory.FilterBandwidthHz < 1 || memory.FilterBandwidthHz > 2_048_000 || memory.StepHz < 1 || memory.StepHz > 1_000_000 {
		return true
	}
	if len(memory.Description) > 500 || len(memory.Group) > 40 || len(memory.CTCSSHz) > 24 || len(memory.DCSCode) > 24 {
		return true
	}
	if memory.Group == "" || strings.EqualFold(memory.Group, i18n.Source("text.201f15dab8b3")) {
		return true
	}
	for _, mode := range []string{"AM", i18n.Source("text.0896d612d497"), i18n.Source("text.6b742bac3eb4"), i18n.Source("text.61f0acff1735"), i18n.Source("text.6323db4948ad"), "CW", i18n.Source("text.2604864ce4d3"), i18n.Source("text.7866f9f32e66"), i18n.Source("text.72c048cb5100"), i18n.Source("text.f69d86a86926")} {
		if memory.Mode == mode {
			return false
		}
	}
	return true
}

func (screen *MainScreen) applyWebMemoryControl(command webControlCommand) {
	panel := screen.memoryPanel
	if panel == nil {
		return
	}
	validSelection := command.MemoryIndex >= 0 && command.MemoryIndex < len(panel.memories) && panel.memories[command.MemoryIndex].Name == command.MemoryName
	switch command.Action {
	case "memoryTune":
		if !validSelection {
			return
		}
		if screen.scanPanel != nil && screen.scanPanel.running {
			screen.scanPanel.Stop()
		}
		panel.selected = command.MemoryIndex
		panel.tuneSelected()
	case "memoryDelete":
		if !validSelection {
			return
		}
		panel.memories = append(panel.memories[:command.MemoryIndex], panel.memories[command.MemoryIndex+1:]...)
		panel.selected = -1
		panel.rebuildGroups()
		panel.save()
	case "memorySave":
		if command.Memory == nil || errInvalidMemory(*command.Memory) {
			return
		}
		memory := *command.Memory
		memory.Name = strings.TrimSpace(memory.Name)
		memory.Group = strings.TrimSpace(memory.Group)
		if command.MemoryIndex < 0 {
			panel.memories = append(panel.memories, memory)
			panel.selected = len(panel.memories) - 1
		} else {
			if !validSelection {
				return
			}
			panel.memories[command.MemoryIndex] = memory
			panel.selected = command.MemoryIndex
		}
		panel.rebuildGroups()
		panel.save()
	case "groupAdd":
		group := strings.TrimSpace(command.Group)
		if !validMemoryGroup(group) || panel.groupExists(group) {
			return
		}
		if panel.groupColors == nil {
			panel.groupColors = map[string]string{}
		}
		panel.groupColors[group] = colorHex(memoryGroupPalette[0])
		panel.saveGroupColors()
		panel.rebuildGroups()
		panel.selectedGroup = group
	case "groupRename":
		original, updated := strings.TrimSpace(command.Group), strings.TrimSpace(command.NewGroup)
		if !validMemoryGroup(original) || !validMemoryGroup(updated) || !panel.groupExists(original) || (!strings.EqualFold(original, updated) && panel.groupExists(updated)) {
			return
		}
		for index := range panel.memories {
			if memoryGroup(panel.memories[index]) == original {
				panel.memories[index].Group = updated
			}
		}
		if color, ok := panel.groupColors[original]; ok {
			delete(panel.groupColors, original)
			panel.groupColors[updated] = color
			panel.saveGroupColors()
		}
		panel.selectedGroup = updated
		panel.rebuildGroups()
		panel.save()
	}
}
