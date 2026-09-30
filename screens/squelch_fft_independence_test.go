package screens

import (
	"testing"

	"go-zero/simpleui"
)

func TestFFTDisplayRangeDoesNotMoveSquelchControl(t *testing.T) {
	screen := &MainScreen{
		squelchThreshold:  -100,
		spectrumMinimumDB: -40,
		spectrumMaximumDB: 0,
		squelchSlider:     simpleui.NewSlider("testSquelch", 0, 0, 100, 20, squelchMinimumDB, squelchMaximumDB, -100),
		squelchLabel:      simpleui.NewLabel("testSquelchLabel", 0, 0, 100, 20, "", 10),
	}
	screen.syncSquelchControl()
	before := screen.squelchSlider.Value()
	screen.spectrumMinimumDB, screen.spectrumMaximumDB = -130, -20
	screen.syncSquelchControl()
	if screen.squelchThreshold != -100 || screen.squelchSlider.Value() != before {
		t.Fatalf("FFT range moved squelch: threshold=%d slider=%g, want -100", screen.squelchThreshold, screen.squelchSlider.Value())
	}
}
