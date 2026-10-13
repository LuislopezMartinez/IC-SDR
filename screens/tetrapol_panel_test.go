package screens

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func assertRectangle(t *testing.T, got, want rl.Rectangle) {
	t.Helper()
	if got != want {
		t.Fatalf("bounds = %+v, want %+v", got, want)
	}
}

func TestTETRAPOLPanelUsesNarrowbandChannelStep(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.tuningStepHz = 1_000
	screen.waterfallVisible = true
	panel := NewTETRAPOLPanel(screen)

	panel.Enter()
	if got, want := screen.tuningStepHz, int64(12_500); got != want {
		t.Fatalf("TETRAPOL tuning step = %d, want %d", got, want)
	}
	if screen.waterfallVisible {
		t.Fatal("TETRAPOL panel left waterfall controls active")
	}
	if got, want := len(panel.controls), 12; got != want {
		t.Fatalf("TETRAPOL controls = %d, want %d", got, want)
	}
	if panel.start.Bounds().X < toolContentX {
		t.Fatalf("start button is hidden outside tool content: x=%.0f", panel.start.Bounds().X)
	}
	panel.Leave()
}

func TestTETRAPOLAFCIsDisabledWithoutCarrierEstimator(t *testing.T) {
	panel := NewTETRAPOLPanel(NewMainScreen(nil))
	if panel.autoCenter.Enabled() || panel.autoCenter.Active() {
		t.Fatal("AFC must remain disabled without a carrier-frequency estimator")
	}
}

func TestTETRAPOLVHFProfileProvidesTwoDocumentedRanges(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewTETRAPOLPanel(screen)
	panel.SetVisible(true)
	panel.setBand("VHF")

	if panel.bandType != "VHF" || screen.tetrapolBand != "VHF" {
		t.Fatalf("profile = %q/%q; want VHF", panel.bandType, screen.tetrapolBand)
	}
	if got := panel.presets[0].Label(); got != "68–88 MHz" {
		t.Fatalf("first VHF preset = %q", got)
	}
	if got := panel.presets[1].Label(); got != "150–174 MHz" {
		t.Fatalf("second VHF preset = %q", got)
	}
	if panel.presets[2].Visible() {
		t.Fatal("third preset remains visible for VHF")
	}
	panel.activatePreset(1)
	if screen.frequencyHz != 162_000_000 {
		t.Fatalf("VHF preset tuned %d; want 162000000", screen.frequencyHz)
	}
}

func TestTETRAPOLBandPresetTunesChannelRaster(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewTETRAPOLPanel(screen)
	panel.tune(420_000_000)
	if screen.frequencyHz != 420_000_000 || screen.centerFrequencyHz != 420_000_000 {
		t.Fatalf("preset tuning = %d/%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	if screen.spanHz != 250_000 || screen.tuningStepHz != 12_500 {
		t.Fatalf("preset raster = span %d step %d", screen.spanHz, screen.tuningStepHz)
	}
}

func TestTETRAPOLPresetRefreshKeepsCollapsedWorkspaceLayout(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewTETRAPOLPanel(screen)
	panel.SetVisible(true)
	screen.utilitiesCollapsed = true

	panel.setBand("UHF")
	assertRectangle(t, panel.presets[0].Bounds(), collapsedToolBounds(rl.Rectangle{X: 704, Y: toolY + 50, Width: 116, Height: 34}))
	assertRectangle(t, panel.presets[2].Bounds(), collapsedToolBounds(rl.Rectangle{X: 952, Y: toolY + 50, Width: 116, Height: 34}))

	panel.setBand("VHF")
	assertRectangle(t, panel.presets[0].Bounds(), collapsedToolBounds(rl.Rectangle{X: 704, Y: toolY + 50, Width: 178, Height: 34}))
	assertRectangle(t, panel.presets[1].Bounds(), collapsedToolBounds(rl.Rectangle{X: 890, Y: toolY + 50, Width: 178, Height: 34}))
}
