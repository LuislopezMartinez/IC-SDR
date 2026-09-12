package screens

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/satellite"
)

func TestSatelliteMapCatalogSearchFiltersNameNORADAndGroup(t *testing.T) {
	v := &satelliteMap{snapshot: satellite.Snapshot{Satellites: []satellite.State{
		{Name: "ISS (ZARYA)", NORAD: 25544, Group: "Space stations"},
		{Name: "QO-100", NORAD: 43700, Group: "Amateur radio"},
	}}}
	for _, query := range []string{"zarya", "25544", "space"} {
		v.query = query
		_, groups := v.grouped()
		if len(groups["Space stations"]) != 1 || len(groups["Amateur radio"]) != 0 {
			t.Fatalf("query %q returned %#v", query, groups)
		}
	}
}

func TestWorldProjectRoundTrip(t *testing.T) {
	b := rl.Rectangle{X: 352, Y: 68, Width: 1072, Height: 556}
	points := []struct{ lat, lon float64 }{
		{-33.8688, 151.2093},
		{40.4168, -3.7038},
		{0, 0},
		{51.5074, -0.1278},
	}
	for _, p := range points {
		got := worldProject(p.lat, p.lon, b)
		lat, lon := worldUnproject(got, b)
		if math.Abs(lat-p.lat) > 0.02 || math.Abs(lon-p.lon) > 0.02 {
			t.Fatalf("round-trip %.4f,%.4f -> %.4f,%.4f", p.lat, p.lon, lat, lon)
		}
	}
}
