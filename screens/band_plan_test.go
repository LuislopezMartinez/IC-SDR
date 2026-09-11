package screens

import (
	"testing"

	"go-zero/simpleui"
)

func TestFindBandRangeForRecalledMemory(t *testing.T) {
	band, found := findBandRange(446_081_250, "HAM")
	if !found || band.category != "ISM" || band.name != "PMR446" {
		t.Fatalf("unexpected PMR band: %+v found=%v", band, found)
	}
	band, found = findBandRange(438_412_500, "ISM")
	if !found || band.category != "HAM" || band.name != "70 cm" {
		t.Fatalf("unexpected DMR band: %+v found=%v", band, found)
	}
}

func TestOutOfBandUsesCompactButtonLabel(t *testing.T) {
	screen := NewMainScreen(nil)
	screen.band = simpleui.NewButton("band", 0, 0, 150, 48, "", 14)
	screen.updateBandForFrequency(75_000_000)
	if got := screen.band.Label(); got != "BAND OUT" {
		t.Fatalf("out-of-band label = %q", got)
	}
	if screen.bandName != "OUT OF BAND" {
		t.Fatalf("internal band name changed to %q", screen.bandName)
	}
}

func TestFindBandRangePrefersCurrentOverlap(t *testing.T) {
	band, found := findBandRange(433_920_000, "ISM")
	if !found || band.category != "ISM" || band.name != "433 MHz" {
		t.Fatalf("preferred overlap not retained: %+v found=%v", band, found)
	}
}
