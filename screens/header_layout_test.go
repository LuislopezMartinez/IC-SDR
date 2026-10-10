package screens

import (
	"testing"

	"go-zero/simpleui"
)

func TestSquelchAndHeaderSwitchLayout(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.utilitiesCollapsed = false
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
	quickLeft, quickRight := float32(130), float32(516)
	for name, control := range map[string]simpleui.Element{
		"mode": screen.mode, "filter": screen.filter, "band": screen.band,
	} {
		bounds := control.Bounds()
		if bounds.X < quickLeft || bounds.X+bounds.Width > quickRight || bounds.Y < 8 || bounds.Y+bounds.Height > 88 {
			t.Fatalf("%s escaped quick-tuning group: %+v", name, bounds)
		}
	}
	if frequencyPanelW <= 290 {
		t.Fatal("frequency readout must remain the most prominent header panel")
	}
	if screen.sdrHeader.deviceSelect.Bounds().X <= frequencyPanelX+frequencyPanelW {
		t.Fatal("advanced hardware controls overlap the primary frequency region")
	}

	toolControl := screen.audioPanel.controls[0]
	expandedBounds := toolControl.Bounds()
	scannerWasRunning := screen.scanPanel.running
	screen.setUtilitiesCollapsed(true)
	if x, _, _, _ := screen.spectrumGeometry(); x != collapsedToolContentX {
		t.Fatalf("spectrum did not use collapsed workspace: x=%.0f", x)
	}
	if !screen.utilitiesSidebar.scan.Visible() || screen.utilitiesSidebar.scanLayer.Visible() {
		t.Fatal("compact rail did not preserve only its primary scanner action")
	}
	if screen.scanPanel.running != scannerWasRunning {
		t.Fatal("collapsing utilities changed scanner operation")
	}
	if toolControl.Bounds().Width <= expandedBounds.Width {
		t.Fatal("active tool did not receive the released sidebar width")
	}
	screen.setUtilitiesCollapsed(false)
	restored := toolControl.Bounds()
	if restored.X != expandedBounds.X || restored.Width != expandedBounds.Width {
		t.Fatalf("tool bounds were not restored: got %+v want %+v", restored, expandedBounds)
	}
}
