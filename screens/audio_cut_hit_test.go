package screens

import (
	"math"
	"testing"
)

func TestCompactToolXRoundTrip(t *testing.T) {
	for _, x := range []float32{legacyToolX, 42, 818, 824, 1063, 1554, legacyToolX + legacyToolWidth} {
		got := expandToolX(compactToolX(x))
		if math.Abs(float64(got-x)) > 0.05 {
			t.Fatalf("legacy %.1f compacted to %.1f expanded to %.1f", x, compactToolX(x), got)
		}
	}
}

func TestAudioCutHitUsesCompactedCoordinates(t *testing.T) {
	legacyLow := audioSpectrumXForHz(100)
	visualLow := compactToolX(legacyLow)
	if visualLow >= audioSpectrumX && visualLow <= audioSpectrumX+22 {
		t.Fatalf("visible low-cut x %.1f still sits on the unscaled hit box; clicks on the line would already work", visualLow)
	}
	if nearestAudioCutHandle(expandToolX(visualLow), 100, 4000) != 1 {
		t.Fatalf("clicking the visible low-cut line at %.1f did not grab LOW", visualLow)
	}
	if nearestAudioCutHandle(legacyLow, 100, 4000) != 1 {
		t.Fatal("legacy low-cut x should still identify LOW after expandToolX")
	}

	legacyHigh := audioSpectrumXForHz(4000)
	visualHigh := compactToolX(legacyHigh)
	if nearestAudioCutHandle(expandToolX(visualHigh), 100, 4000) != 2 {
		t.Fatalf("clicking the visible high-cut line at %.1f did not grab HIGH", visualHigh)
	}
	if nearestAudioCutHandle(visualHigh, 100, 4000) == 2 {
		t.Fatal("an unscaled click on the visible high-cut x should not be treated as a hit; that was the old left-of-line bug")
	}
}

func TestNearestAudioCutHandlePrefersCloserLine(t *testing.T) {
	mid := (audioSpectrumXForHz(100) + audioSpectrumXForHz(4000)) / 2
	if nearestAudioCutHandle(mid, 100, 4000) != 0 {
		t.Fatal("clicking the passband centre should not grab a cut")
	}
	if nearestAudioCutHandle(audioSpectrumXForHz(3900), 100, 4000) != 2 {
		t.Fatal("click nearer the high cut should grab HIGH")
	}
}
