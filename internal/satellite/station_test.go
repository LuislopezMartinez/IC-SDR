package satellite

import (
	"math"
	"path/filepath"
	"testing"
)

func TestDefaultStationUsesCountryNotMadrid(t *testing.T) {
	au := DefaultStation("au")
	if au.Latitude >= 0 || au.Longitude < 100 {
		t.Fatalf("Australia default should be in the southern Pacific: %+v", au)
	}
	if IsLegacyMadridDefault(au) {
		t.Fatal("Australia default must not be Madrid")
	}
	es := DefaultStation("es")
	if !IsLegacyMadridDefault(es) {
		t.Fatalf("Spain default should remain Madrid: %+v", es)
	}
}

func TestLegacyMadridDetection(t *testing.T) {
	if !IsLegacyMadridDefault(Station{Name: "Madrid", Latitude: 40.4168, Longitude: -3.7038}) {
		t.Fatal("factory Madrid was not detected")
	}
	if IsLegacyMadridDefault(Station{Name: "Sydney", Latitude: -33.8688, Longitude: 151.2093}) {
		t.Fatal("Sydney was treated as Madrid")
	}
}

func TestResolveStationReplacesLegacyMadridForAustralia(t *testing.T) {
	path := filepath.Join(t.TempDir(), "satellite-station.json")
	if err := SaveStationFile(path, DefaultStation("es")); err != nil {
		t.Fatal(err)
	}
	got := ResolveStation(path, "au")
	if IsLegacyMadridDefault(got) || got.Name != "Sydney" {
		t.Fatalf("Madrid was not replaced for Australia: %+v", got)
	}
	saved, ok := LoadStationFile(path)
	if !ok || saved.Name != "Sydney" {
		t.Fatalf("updated station was not saved: %+v ok=%v", saved, ok)
	}
}

func TestResolveStationKeepsCustomLocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "satellite-station.json")
	custom := Station{Name: "Brisbane", Latitude: -27.4698, Longitude: 153.0251, AltitudeMeters: 27}
	if err := SaveStationFile(path, custom); err != nil {
		t.Fatal(err)
	}
	got := ResolveStation(path, "us")
	if got.Name != "Brisbane" || math.Abs(got.Latitude+27.4698) > 0.001 {
		t.Fatalf("custom location overwritten: %+v", got)
	}
}

func TestResolveStationWritesCountryDefaultWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "satellite-station.json")
	got := ResolveStation(path, "au")
	if got.Name != "Sydney" {
		t.Fatalf("missing file should use country default: %+v", got)
	}
	if _, ok := LoadStationFile(path); !ok {
		t.Fatal("country default was not saved")
	}
}

func TestResolveStationKeepsMadridWhenCountryUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "satellite-station.json")
	if err := SaveStationFile(path, DefaultStation("es")); err != nil {
		t.Fatal(err)
	}
	got := ResolveStation(path, "auto")
	if !IsLegacyMadridDefault(got) {
		t.Fatalf("unknown country overwrote Madrid: %+v", got)
	}
}
