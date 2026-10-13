package screens

import (
	"math"
	"testing"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestAudioPanelUsesFullNativeToolArea(t *testing.T) {
	panel := NewAudioPanel(NewMainScreen(nil))
	for _, control := range panel.controls {
		bounds := control.Bounds()
		if bounds.X < toolContentX || bounds.X+bounds.Width > toolContentRight || bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
			t.Fatalf("audio control outside tool workspace: %+v", bounds)
		}
	}
	if bottom := panel.notchReset.Bounds().Y + panel.notchReset.Bounds().Height; bottom < toolY+toolH-30 {
		t.Fatalf("audio panel still leaves excessive unused height: bottom %.0f workspace %.0f", bottom, toolY+toolH)
	}
}

func TestAudioAutoNotchSelectsNarrowPeak(t *testing.T) {
	screen := NewMainScreen(nil)
	panel := NewAudioPanel(screen)
	for index := range panel.preNotchSpectrum {
		panel.preNotchSpectrum[index] = -70
	}
	peak := 9 // about 1.5 kHz at 16 kHz / 95 bins
	panel.preNotchSpectrum[peak] = -15
	panel.autoNotch()
	if !panel.notchEnabled || panel.notchFrequencyHz < 1400 || panel.notchFrequencyHz > 1650 {
		t.Fatalf("auto notch did not select peak: enabled=%v frequency=%d", panel.notchEnabled, panel.notchFrequencyHz)
	}
}

func TestNotchInteractionUsesUpperHalfForTuningAndLowerForWidth(t *testing.T) {
	for _, test := range []struct {
		y    float32
		want int
	}{{683, 0}, {684, 1}, {753, 1}, {754, 2}, {824, 2}, {825, 0}} {
		if got := notchInteractionZone(test.y, 684, 140); got != test.want {
			t.Fatalf("zone at y %.0f = %d, want %d", test.y, got, test.want)
		}
	}
}

func TestAudioGraphPointerTracksCollapsedWorkspaceTransform(t *testing.T) {
	for _, legacyX := range []float32{380, 520, 660, 962, 1267, 1572} {
		visualX := collapsedToolContentX + (legacyX-toolContentX)*collapsedWorkspaceScaleX()
		mapped := toolPointerForLayout(rl.Vector2{X: visualX, Y: 740}, true)
		if math.Abs(float64(mapped.X-legacyX)) > .01 || mapped.Y != 740 {
			t.Fatalf("collapsed pointer %.2f mapped to %+v, want x %.2f", visualX, mapped, legacyX)
		}
	}
	plain := rl.Vector2{X: 1111, Y: 700}
	if got := toolPointerForLayout(plain, false); got != plain {
		t.Fatalf("expanded pointer changed: got %+v want %+v", got, plain)
	}
}

func TestNotchFeedbackBadgeFollowsThemeContrast(t *testing.T) {
	previous := colors
	t.Cleanup(func() { colors = previous })

	colors = lightPalette
	lightBackground, lightText := notchFeedbackBadgeColors()
	if want := simpleui.EnsureTextContrast(lightPalette.text, lightBackground); lightText != want {
		t.Fatalf("light badge text = %#v, want contrast-adjusted %#v", lightText, want)
	}
	if lightText == lightBackground {
		t.Fatal("light badge text and background must be distinct")
	}

	colors = darkPalette
	darkBackground, darkText := notchFeedbackBadgeColors()
	if lightBackground == darkBackground {
		t.Fatal("notch badge background did not follow the active theme")
	}
	if want := simpleui.EnsureTextContrast(darkPalette.text, darkBackground); darkText != want {
		t.Fatalf("dark badge text = %#v, want contrast-adjusted %#v", darkText, want)
	}
}

func TestNRMeterInactiveSegmentsKeepTheirHue(t *testing.T) {
	live := rl.Color{R: 40, G: 210, B: 120, A: 255}
	dim := nrMeterDimColor(live, .20)
	if dim.R == 0 || dim.G == 0 || dim.B == 0 {
		t.Fatalf("inactive NR segment lost its hue: %#v", dim)
	}
	if dim.R >= live.R || dim.G >= live.G || dim.B >= live.B || dim.A != 255 {
		t.Fatalf("inactive NR segment is not a dark opaque variant: live=%#v dim=%#v", live, dim)
	}
}
