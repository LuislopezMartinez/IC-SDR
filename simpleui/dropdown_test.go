package simpleui

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestDropdownCopiesItemsAndSelects(t *testing.T) {
	items := []string{"AM", "FM", "USB"}
	dropdown := NewDropdown("mode", 0, 0, 200, 40, "MODE", items, 16)
	items[0] = "changed externally"
	dropdown.SetSelected(0)
	if dropdown.SelectedText() != "AM" {
		t.Fatalf("got %q, want copied item AM", dropdown.SelectedText())
	}
}

func TestDropdownNotifiesOnlyChangedSelection(t *testing.T) {
	dropdown := NewDropdown("mode", 0, 0, 200, 40, "MODE", []string{"AM", "FM"}, 16)
	changes := 0
	dropdown.OnChange(func(index int, value string) {
		changes++
		if index != 1 || value != "FM" {
			t.Fatalf("unexpected selection %d %q", index, value)
		}
	})
	dropdown.choose(1, true)
	dropdown.choose(1, true)
	if changes != 1 {
		t.Fatalf("got %d changes, want 1", changes)
	}
}

func TestDropdownOverlaySelectsClickedItem(t *testing.T) {
	dropdown := NewDropdown("mode", 10, 10, 200, 40, "MODE", []string{"AM", "FM", "USB"}, 16)
	dropdown.setOpen(true)
	popup := dropdown.popupBounds()
	pointer := rl.Vector2{X: popup.X + 20, Y: popup.Y + dropdown.itemHeight + 5}
	dropdown.UpdateOverlay(Input{Pointer: pointer, PointerInCanvas: true, Pressed: true})
	if dropdown.SelectedText() != "FM" || dropdown.Open() {
		t.Fatalf("selection=%q open=%v", dropdown.SelectedText(), dropdown.Open())
	}
}

func TestDropdownClickOutsideClosesAndConsumes(t *testing.T) {
	dropdown := NewDropdown("mode", 10, 10, 200, 40, "MODE", []string{"AM"}, 16)
	dropdown.setOpen(true)
	consumed := dropdown.UpdateOverlay(Input{Pointer: rl.Vector2{X: 500, Y: 500}, PointerInCanvas: true, Pressed: true})
	if dropdown.Open() || !consumed {
		t.Fatalf("open=%v consumed=%v", dropdown.Open(), consumed)
	}
}

func TestManagerKeepsPointerBlockedWhenDropdownSelectionClosesOverlay(t *testing.T) {
	manager := NewManager()
	dropdown := NewDropdown("mode", 10, 10, 200, 40, "MODE", []string{"AM", "FM"}, 16)
	manager.Add(dropdown)
	dropdown.setOpen(true)
	popup := dropdown.popupBounds()
	manager.Update(Input{Pointer: rl.Vector2{X: popup.X + 10, Y: popup.Y + 10}, PointerInCanvas: true, Pressed: true, Down: true})
	if dropdown.Open() {
		t.Fatal("selection did not close dropdown")
	}
	if !manager.PointerInputBlocked() {
		t.Fatal("popup selection click was allowed to reach controls underneath")
	}
	manager.Update(Input{PointerInCanvas: true, Down: true})
	if !manager.PointerInputBlocked() {
		t.Fatal("pointer was unblocked before the popup gesture was released")
	}
	manager.Update(Input{PointerInCanvas: true, Released: true})
	if !manager.PointerInputBlocked() {
		t.Fatal("popup release was allowed to reach controls underneath")
	}
	manager.Update(Input{PointerInCanvas: true})
	if manager.PointerInputBlocked() {
		t.Fatal("pointer remained blocked after the popup gesture")
	}
}

func TestDropdownOpensAboveWhenNeeded(t *testing.T) {
	dropdown := NewDropdown("mode", 10, float32(runtime.height)-50, 200, 40, "MODE", []string{"AM", "FM"}, 16)
	dropdown.setOpen(true)
	if dropdown.popupBounds().Y >= dropdown.Bounds().Y {
		t.Fatal("dropdown did not open above the control")
	}
}

func TestDisabledDropdownDoesNotOpen(t *testing.T) {
	dropdown := NewDropdown("mode", 10, 10, 200, 40, "MODE", []string{"AM"}, 16)
	dropdown.SetEnabled(false)
	dropdown.setOpen(true)
	if dropdown.Open() {
		t.Fatal("disabled dropdown opened")
	}
}
