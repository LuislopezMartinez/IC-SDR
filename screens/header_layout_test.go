package screens

import (
	"testing"

	"go-zero/simpleui"
)

func TestSquelchAndHeaderSwitchLayout(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.CreateControls()
	if got := screen.squelchSwitch.Bounds(); got.Y != 24 || got.Height < 28 {
		t.Fatalf("squelch switch is not aligned in raised panel: %+v", got)
	}
	mem := screen.memViewSwitch.Bounds()
	vfo := screen.vfoModeSwitch.Bounds()
	if mem.Y != 159 || vfo.Y != 159 || mem.Height < 28 || vfo.Height < 28 {
		t.Fatalf("lower switches remain undersized: mem=%+v vfo=%+v", mem, vfo)
	}
	if mem.X+mem.Width > vfo.X {
		t.Fatalf("lower switches overlap: mem=%+v vfo=%+v", mem, vfo)
	}
	if label := screen.vfoModeSwitch.Label(); label != "CENTER" && label != "FIX" {
		t.Fatalf("unexpected VFO label: %q", screen.vfoModeSwitch.Label())
	}
	for name, control := range map[string]*simpleui.Button{"STEP": screen.step, "STEP −": screen.stepDown, "STEP +": screen.stepUp} {
		bounds := control.Bounds()
		if bounds.Y < frequencyPanelY || bounds.Y+bounds.Height > frequencyPanelY+frequencyPanelH {
			t.Fatalf("%s control is outside frequency panel: %+v", name, bounds)
		}
	}
	menu := screen.menuButton.Bounds()
	if menu.X != 24 || menu.Y != 16 || menu.X+menu.Width > screen.mode.Bounds().X || menu.Y+menu.Height > frequencyPanelY {
		t.Fatalf("menu is not prominent and separate from the dial: %+v", menu)
	}
	for name, control := range map[string]*simpleui.Button{"VIEW": screen.viewButton, "ESTILO": screen.themeButton} {
		bounds := control.Bounds()
		if bounds.Y < frequencyPanelY || bounds.Y+bounds.Height > frequencyPanelY+frequencyPanelH ||
			bounds.X < frequencyPanelX || bounds.X+bounds.Width > frequencyPanelX+frequencyPanelW {
			t.Fatalf("%s control is outside frequency panel: %+v", name, bounds)
		}
		if bounds.Y >= 205 {
			t.Fatalf("%s control remains in the window footer: %+v", name, bounds)
		}
	}
	if screen.viewButton.Bounds().X+screen.viewButton.Bounds().Width > screen.themeButton.Bounds().X {
		t.Fatal("view and style controls overlap")
	}
	rig := screen.rigButton.Bounds()
	if rig.X < frequencyPanelX || rig.X+rig.Width > frequencyDividerX || rig.Y < frequencyPanelY || rig.Y+rig.Height > frequencyPanelY+frequencyPanelH {
		t.Fatalf("RIG button is outside SPAN panel: %+v", rig)
	}
	if screen.mode.Bounds().X+screen.mode.Bounds().Width > screen.filter.Bounds().X ||
		screen.filter.Bounds().X+screen.filter.Bounds().Width > screen.band.Bounds().X ||
		screen.band.Bounds().X+screen.band.Bounds().Width > frequencyPanelX {
		t.Fatal("top-row controls overlap")
	}
}
