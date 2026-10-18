package screens

import (
	"testing"

	"go-zero/internal/i18n"
	"go-zero/simpleui"
)

func TestSatelliteTuneSelectsSignalDemodulator(t *testing.T) {
	screen := NewMainScreen(nil)
	nfm := i18n.Source("text.0896d612d497")
	screen.mode = simpleui.NewDropdown("mode", 0, 0, 100, 40, "MODE", []string{"AM", nfm, "USB"}, 12)
	screen.filterSelector = NewFilterSelector(screen.selectFilter)
	screen.filter = simpleui.NewButton("filter", 0, 0, 100, 40, "FILTER", 12)
	screen.mode.SetSelected(0)

	panel := &SatellitePanel{screen: screen}
	if got := panel.selectSignalMode("FM"); got != nfm {
		t.Fatalf("selected receiver mode = %q, want %q", got, nfm)
	}

	if got := screen.mode.SelectedText(); got != nfm {
		t.Fatalf("satellite FM mode = %q, want %q", got, nfm)
	}
}
