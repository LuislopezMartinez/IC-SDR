package screens

import "testing"

func TestApplyTuningMovesFrequencyCenterAndBandTogether(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.bandCategory, screen.bandName = "HAM", "20 m"
	screen.frequencyHz, screen.centerFrequencyHz = 14_200_000, 14_200_000
	screen.spanHz, screen.demodBandwidthHz, screen.centerMode = 500_000, 12_500, true

	screen.applyTuning(tuningRequest{FrequencyHz: 145_600_000, Center: tuningKeepVisible, UpdateBand: true})

	if screen.frequencyHz != 145_600_000 || screen.centerFrequencyHz != 145_600_000 || screen.bandName != "2 m" {
		t.Fatalf("atomic tuning state = %d/%d/%s", screen.frequencyHz, screen.centerFrequencyHz, screen.bandName)
	}
}

func TestBandProfileFrequencyRejectsFrequencyFromAnotherBand(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.bandCategory, screen.bandName = "HAM", "20 m"
	screen.bandDisplayProfiles = map[string]bandDisplayProfile{
		bandDisplayProfileKey("HAM", "20 m"): {LastFrequencyHz: 7_100_000},
	}
	if got := screen.bandProfileFrequency(14_200_000); got != 14_200_000 {
		t.Fatalf("foreign profile frequency accepted: %d", got)
	}
}
