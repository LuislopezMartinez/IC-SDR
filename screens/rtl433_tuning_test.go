package screens

import "testing"

func TestCenteredClickMovesVFOAndCaptureTogether(t *testing.T) {
	screen := &MainScreen{frequencyHz: 433_925_000, centerFrequencyHz: 433_925_000, spanHz: 1_000_000, tuningStepHz: 1_000, centerMode: true}
	screen.tuneAtFraction(.575) // 434.000 MHz on a 1 MHz display.
	if screen.frequencyHz != 434_000_000 || screen.centerFrequencyHz != 434_000_000 {
		t.Fatalf("CENTER click tuned=%d center=%d", screen.frequencyHz, screen.centerFrequencyHz)
	}
}

func TestRTL433FixedTuneKeepsCaptureStationary(t *testing.T) {
	screen := &MainScreen{frequencyHz: 433_925_000, centerFrequencyHz: 433_925_000, spanHz: 1_000_000, tuningStepHz: 1_000, centerMode: false}
	panel := &RTL433Panel{screen: screen, targetHz: screen.frequencyHz, bandwidthHz: 500_000}
	panel.selectFrequency(434_075_000, false)
	if screen.frequencyHz != 434_075_000 || screen.centerFrequencyHz != 433_925_000 || screen.centerMode {
		t.Fatalf("RTL_433 FIX tuned=%d center=%d mode=%v", screen.frequencyHz, screen.centerFrequencyHz, screen.centerMode)
	}
}

func TestRTL433CenterTuneRecentersAndPreservesMode(t *testing.T) {
	screen := &MainScreen{frequencyHz: 433_925_000, centerFrequencyHz: 433_925_000, spanHz: 1_000_000, tuningStepHz: 1_000, centerMode: true}
	panel := &RTL433Panel{screen: screen, targetHz: screen.frequencyHz, bandwidthHz: 500_000}
	panel.selectFrequency(434_075_000, false)
	if screen.frequencyHz != 434_075_000 || screen.centerFrequencyHz != 434_075_000 || !screen.centerMode {
		t.Fatalf("RTL_433 CENTER tuned=%d center=%d mode=%v", screen.frequencyHz, screen.centerFrequencyHz, screen.centerMode)
	}
}

func TestRTL433FixedCenterPansOnlyWhenDecoderWouldLeaveCapture(t *testing.T) {
	if got := fixedCenterForRTL433(433_000_000, 433_400_000, 1_000_000, 500_000); got != 433_150_000 {
		t.Fatalf("minimal capture pan = %d, want 433150000", got)
	}
}
