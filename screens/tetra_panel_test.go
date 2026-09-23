package screens

import "testing"

func TestTETRAWavecomCaptureStaysOutsideUtilitiesSidebar(t *testing.T) {
	panel := NewTETRAPanel(&MainScreen{})
	bounds := panel.iqCapture.Bounds()
	if bounds.X < toolContentX {
		t.Fatalf("Wavecom IQ capture overlaps utilities sidebar: x=%v, content starts at %v", bounds.X, toolContentX)
	}
	if bounds.X+bounds.Width > toolContentRight {
		t.Fatalf("Wavecom IQ capture exceeds tool content: right=%v, limit=%v", bounds.X+bounds.Width, toolContentRight)
	}
	if bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
		t.Fatalf("Wavecom IQ capture is outside TETRA panel: bounds=%+v", bounds)
	}
}
