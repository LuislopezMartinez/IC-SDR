package screens

import "testing"

func TestToolMenuWheelMovesWithinCurrentCategory(t *testing.T) {
	menu := NewToolMenu("", nil)
	menu.setCategory(1)
	indices := menu.indicesForRow(1)
	if len(indices) < 2 {
		t.Fatal("test category needs at least two items")
	}

	if !menu.moveFocusByWheel(-1) || menu.focusIndex != indices[1] {
		t.Fatalf("wheel down did not move to next item: got %d, want %d", menu.focusIndex, indices[1])
	}
	if !menu.moveFocusByWheel(1) || menu.focusIndex != indices[0] {
		t.Fatalf("wheel up did not move to previous item: got %d, want %d", menu.focusIndex, indices[0])
	}
	if menu.category != 1 {
		t.Fatalf("wheel changed category: got %d, want 1", menu.category)
	}
}

func TestToolMenuWheelWrapsAndIgnoresZero(t *testing.T) {
	menu := NewToolMenu("", nil)
	menu.setCategory(0)
	indices := menu.indicesForRow(0)
	menu.focusIndex = indices[0]

	if !menu.moveFocusByWheel(1) || menu.focusIndex != indices[len(indices)-1] {
		t.Fatalf("wheel up did not wrap to last item: got %d, want %d", menu.focusIndex, indices[len(indices)-1])
	}
	previous := menu.focusIndex
	if menu.moveFocusByWheel(0) || menu.focusIndex != previous {
		t.Fatal("zero wheel movement changed the focused item")
	}
}
