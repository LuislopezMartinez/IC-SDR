package screens

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestMercatorRoundTripKnownCities(t *testing.T) {
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	points := []struct{ lat, lon float64 }{
		{40.4168, -3.7038},   // Madrid
		{-33.8688, 151.2093}, // Sydney
		{51.5074, -0.1278},   // London
		{40.7128, -74.0060},  // New York
		{35.6762, 139.6503},  // Tokyo
	}
	for _, p := range points {
		centerLat, centerLon, lonSpan := p.lat, p.lon, 2.0
		proj := projectMercator(centerLat, centerLon, lonSpan, p.lat, p.lon, b)
		if math.Abs(float64(proj.X-(b.X+b.Width/2))) > 0.6 || math.Abs(float64(proj.Y-(b.Y+b.Height/2))) > 0.6 {
			t.Fatalf("city %.4f,%.4f was not at the view center: %+v", p.lat, p.lon, proj)
		}
		lat, lon := unprojectMercator(centerLat, centerLon, lonSpan, proj, b)
		if math.Abs(lat-p.lat) > 1e-6 || math.Abs(wrapLongitude(lon-p.lon)) > 1e-6 {
			t.Fatalf("round-trip %.4f,%.4f -> %.8f,%.8f", p.lat, p.lon, lat, lon)
		}
	}
}

func TestMercatorZoomKeepsAnchor(t *testing.T) {
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	centerLat, centerLon, lonSpan := 40.4, -3.7, 8.0
	anchor := rl.Vector2{X: b.X + 220, Y: b.Y + 180}
	lat, lon := unprojectMercator(centerLat, centerLon, lonSpan, anchor, b)
	centerLat, centerLon, lonSpan = zoomMap(centerLat, centerLon, lonSpan, anchor, b, 1)
	got := projectMercator(centerLat, centerLon, lonSpan, lat, lon, b)
	if math.Abs(float64(got.X-anchor.X)) > 0.75 || math.Abs(float64(got.Y-anchor.Y)) > 0.75 {
		t.Fatalf("zoom moved the map under the cursor: %+v vs %+v", got, anchor)
	}
}

func TestMapZoomMatchesWebMercatorTileScale(t *testing.T) {
	z := mapZoom(360, 256)
	if math.Abs(z-2) > 1e-9 {
		t.Fatalf("world view zoom = %v, want 2", z)
	}
	z = mapZoom(360/math.Exp2(10), 256)
	if math.Abs(z-10) > 1e-9 {
		t.Fatalf("z10 view zoom = %v, want 10", z)
	}
}
