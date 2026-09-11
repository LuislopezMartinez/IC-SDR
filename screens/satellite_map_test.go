package screens

import (
	"testing"

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
