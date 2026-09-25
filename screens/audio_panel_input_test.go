package screens

import "testing"

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
