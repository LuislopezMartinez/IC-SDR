package worldclock

import (
	"path/filepath"
	"testing"
	"time"
)

func TestResolvePrefersCityInPinnedTimezone(t *testing.T) {
	city, km := Resolve(40.42, -3.70, "Europe/Madrid")
	if city.Name != "Madrid" || km > 5 {
		t.Fatalf("unexpected city: %+v at %.1f km", city, km)
	}
}

func TestNormalizeUsesExactPinTimezone(t *testing.T) {
	location := Normalize(Location{Lat: 51.5074, Lon: -0.1278}, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	if location.Zone != "Europe/London" || location.City != "Londres" {
		t.Fatalf("unexpected normalized location: %+v", location)
	}
}

func TestRemotePinDoesNotClaimADistantCity(t *testing.T) {
	location := Normalize(Location{Lat: 0, Lon: -140}, time.Now().UTC())
	if location.City == "Honolulu" || location.Country != "" {
		t.Fatalf("remote ocean pin received a misleading city: %+v", location)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clocks.json")
	want := Defaults()
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got := Load(path)
	if len(got.Locations) != 5 || got.Locations[4].City != "Sídney" || got.Locations[4].Zone == "" {
		t.Fatalf("unexpected config: %+v", got)
	}
}
