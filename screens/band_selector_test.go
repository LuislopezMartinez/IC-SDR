package screens

import (
	"testing"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestBandCatalogMatchesOriginalCategories(t *testing.T) {
	restoreDefaultLocaleForTests()
	if len(bandsByCategory["HAM"]) != 15 || len(bandsByCategory["COMMERCIAL"]) != 13 || len(bandsByCategory["ISM"]) != 6 {
		t.Fatalf("unexpected band counts: HAM=%d COMMERCIAL=%d ISM=%d",
			len(bandsByCategory["HAM"]), len(bandsByCategory["COMMERCIAL"]), len(bandsByCategory["ISM"]))
	}
}

func TestPMR446Definition(t *testing.T) {
	restoreDefaultLocaleForTests()
	band := bandsByCategory["ISM"][1]
	if band.Name != "PMR446" || band.FrequencyHz != 446_006_250 || band.SpanHz != 500_000 {
		t.Fatalf("unexpected PMR446 definition: %+v", band)
	}
}

func TestBandSelectorSelectsPMR446(t *testing.T) {
	restoreDefaultLocaleForTests()
	var selected BandDefinition
	selector := NewBandSelector("HAM", "20 m", func(band BandDefinition) { selected = band })
	selector.Open()
	selector.UpdateOverlay(simpleui.Input{Pointer: rl.Vector2{X: 872, Y: 174}, Released: true, PointerInCanvas: true})
	selector.UpdateOverlay(simpleui.Input{Pointer: rl.Vector2{X: 414, Y: 257}, Released: true, PointerInCanvas: true})
	if selected.Name != "PMR446" || selected.FrequencyHz != 446_006_250 {
		t.Fatalf("PMR446 selection was not emitted: %+v", selected)
	}
	if selector.OverlayOpen() {
		t.Fatal("selector must close after choosing a band")
	}
}

func TestPMR446RecommendsNFM(t *testing.T) {
	restoreDefaultLocaleForTests()
	if mode := recommendedModeForBand(bandsByCategory["ISM"][1]); mode != "NFM" {
		t.Fatalf("PMR446 recommended mode: got %s, want NFM", mode)
	}
}
