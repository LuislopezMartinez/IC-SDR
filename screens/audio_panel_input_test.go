package screens

import (
	"math"
	"testing"
)

func TestAudioGraphPointerMatchesCompactedDrawing(t *testing.T) {
	for _, legacyX := range []float32{42, 237, 818, 1063, 1554} {
		drawnX := toolContentX + (legacyX-legacyToolX)*toolContentScaleX
		got := legacyToolPointerX(drawnX)
		if math.Abs(float64(got-legacyX)) > 0.001 {
			t.Fatalf("drawn X %.3f maps to %.3f, want %.3f", drawnX, got, legacyX)
		}
	}
}
