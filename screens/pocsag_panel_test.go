package screens

import "testing"

func TestPOCSAGPresetRecentersAndUsesNarrowFM(t *testing.T) {
	screen := &MainScreen{frequencyHz: 14_261_000, centerFrequencyHz: 14_261_000, spanHz: 100_000}
	panel := NewPOCSAGPanel(screen)
	panel.selectFrequency(169_625_000)
	if screen.frequencyHz != 169_625_000 || screen.centerFrequencyHz != 169_625_000 {
		t.Fatalf("preset did not retune and recenter: frequency=%d center=%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
	if screen.demodBandwidthHz != 12_500 {
		t.Fatalf("unexpected POCSAG bandwidth: %d", screen.demodBandwidthHz)
	}
	if screen.spanHz < 500_000 {
		t.Fatalf("preset did not expose enough spectrum: %d", screen.spanHz)
	}
}
