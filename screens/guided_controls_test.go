package screens

import (
	"testing"

	"go-zero/simpleui"
)

func assertControlsInsideToolWorkspace(t *testing.T, controls []simpleui.Element) {
	t.Helper()
	for _, control := range controls {
		bounds := control.Bounds()
		if bounds.X < toolContentX || bounds.X+bounds.Width > toolContentRight || bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
			t.Fatalf("control %s outside guided tool workspace: %+v", control.ID(), bounds)
		}
	}
}

func TestGuidedWaterfallControlsFillWorkspaceWithoutOverflow(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.createWaterfallControls()
	compactToolControls(screen.waterfallControls)
	assertControlsInsideToolWorkspace(t, screen.waterfallControls)
	if bottom := screen.wfPaletteButton.Bounds().Y + screen.wfPaletteButton.Bounds().Height; bottom < toolY+toolH-45 {
		t.Fatalf("waterfall controls leave excessive lower space: bottom %.0f workspace %.0f", bottom, toolY+toolH)
	}
}

func TestGuidedFFTControlsFillWorkspaceWithoutOverflow(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewFFTDisplay(screen)
	compactToolControls(panel.controls)
	assertControlsInsideToolWorkspace(t, panel.controls)
	if bottom := panel.fftCeiling.Bounds().Y + panel.fftCeiling.Bounds().Height; bottom < toolY+toolH-30 {
		t.Fatalf("FFT scale controls leave excessive lower space: bottom %.0f workspace %.0f", bottom, toolY+toolH)
	}
}
