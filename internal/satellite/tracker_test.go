package satellite

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseTLEAndVisibilitySnapshot(t *testing.T) {
	const tle = `ISS (ZARYA)
1 25544U 98067A   25250.50000000  .00010000  00000-0  18000-3 0  9999
2 25544  51.6400 120.0000 0005000  40.0000 320.0000 15.50000000123456`
	list, err := parseTLE(strings.NewReader(tle), "Space stations")
	if err != nil || len(list) != 1 {
		t.Fatalf("parseTLE() = %d satellites, %v", len(list), err)
	}
	if list[0].NORAD != 25544 || math.Abs(list[0].Elements.MeanMotion-15.5) > 1e-6 {
		t.Fatalf("unexpected satellite: %+v", list[0])
	}
	applyKnownSignals(&list[0])
	tracker := &Tracker{satellites: list, station: Station{Name: "Madrid", Latitude: 40.4168, Longitude: -3.7038}, selected: 25544}
	snapshot := tracker.Snapshot(time.Date(2025, 9, 7, 12, 0, 0, 0, time.UTC))
	if len(snapshot.Satellites) != 1 || len(snapshot.Satellites[0].Trajectory) != 28 {
		t.Fatalf("snapshot missing satellite trajectory: %+v", snapshot)
	}
	state := snapshot.Satellites[0]
	if state.Latitude < -90 || state.Latitude > 90 || state.Longitude < -180 || state.Longitude > 180 || state.AltitudeKM < 100 {
		t.Fatalf("invalid propagated position: %+v", state)
	}
}

func TestFallbackContainsISSAndQO100(t *testing.T) {
	seen := map[int]bool{}
	for _, sat := range fallbackCatalog() {
		seen[sat.NORAD] = true
	}
	if !seen[25544] || !seen[43700] {
		t.Fatalf("fallback catalog = %v", seen)
	}
}

func TestPredictPassReturnsOrderedApproachData(t *testing.T) {
	iss := fallbackCatalog()[0]
	station := Station{Name: "Madrid", Latitude: 40.4168, Longitude: -3.7038, AltitudeMeters: 657}
	prediction := predictPass(iss.Elements, station, iss.Elements.Epoch)
	if !prediction.Found || prediction.Continuous {
		t.Fatalf("expected an ISS pass, got %+v", prediction)
	}
	if prediction.AOS.After(prediction.TCA) || prediction.TCA.After(prediction.LOS) {
		t.Fatalf("pass events are not ordered: %+v", prediction)
	}
	if prediction.MinRangeKM <= 0 || prediction.MaxElevation < 0 {
		t.Fatalf("invalid closest approach: %+v", prediction)
	}
}

func TestPredictPassClassifiesGeostationaryVisibility(t *testing.T) {
	qo100 := fallbackCatalog()[1]
	station := Station{Name: "Madrid", Latitude: 40.4168, Longitude: -3.7038, AltitudeMeters: 657}
	prediction := predictPass(qo100.Elements, station, qo100.Elements.Epoch)
	if !prediction.Found || !prediction.Continuous {
		t.Fatalf("expected continuous geostationary state, got %+v", prediction)
	}
}
