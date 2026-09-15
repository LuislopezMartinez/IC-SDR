package screens

import (
	"testing"

	"go-zero/internal/aircraft"
	"go-zero/simpleui"
)

func TestAircraftToolSelectsMatchingDemodulator(t *testing.T) {
	s := NewMainScreen(nil)
	s.mode = simpleui.NewDropdown("mode", 0, 0, 100, 40, "MODE", []string{"AM", "NFM", "ADS-B", "UAT"}, 12)
	s.filterSelector = NewFilterSelector(s.selectFilter)
	s.filter = simpleui.NewButton("filter", 0, 0, 100, 40, "FILTER", 12)
	s.band = simpleui.NewButton("band", 0, 0, 100, 40, "BAND", 12)
	s.bandSelector = NewBandSelector("COMMERCIAL", "ADS-B 1090", nil)
	p := NewAircraftPanel(s)

	s.frequencyHz = 433_920_000
	s.centerFrequencyHz = 433_920_000
	p.Enter()
	if got := s.mode.SelectedText(); got != "ADS-B" {
		t.Fatalf("1090 mode = %q, want ADS-B", got)
	}
	if s.frequencyHz != 433_920_000 || s.centerFrequencyHz != 433_920_000 {
		t.Fatalf("opening aircraft tool changed tuning to %d / %d", s.frequencyHz, s.centerFrequencyHz)
	}
	p.selectMode(aircraft.Mode1090)
	if s.frequencyHz != aircraft.Frequency1090Hz || s.centerFrequencyHz != aircraft.Frequency1090Hz {
		t.Fatalf("clicking active 1090 button did not tune: %d / %d", s.frequencyHz, s.centerFrequencyHz)
	}

	p.selectMode(aircraft.Mode978)
	if got := s.mode.SelectedText(); got != "UAT" {
		t.Fatalf("978 mode = %q, want UAT", got)
	}
	if s.frequencyHz != aircraft.Frequency978Hz || s.demodBandwidthHz != 2_000_000 {
		t.Fatalf("978 tuning = %d Hz / %d Hz BW", s.frequencyHz, s.demodBandwidthHz)
	}
}
