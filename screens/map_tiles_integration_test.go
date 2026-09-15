package screens

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
	"testing"
)

func TestUpdatedMapsMatchTileProjection(t *testing.T) {
	b := rl.Rectangle{X: 352, Y: 68, Width: 1072, Height: 556}
	for _, city := range []geoPoint{{40.4168, -3.7038}, {-33.8688, 151.2093}, {65, 20}} {
		p := worldProject(city.lat, city.lon, b)
		expected := projectMercator(0, 0, 360, city.lat, city.lon, b)
		if rl.Vector2Distance(p, expected) > .01 {
			t.Fatal("satellite marker differs from tile projection")
		}
		lat, lon := worldUnproject(p, b)
		if math.Abs(lat-city.lat) > 1e-4 || math.Abs(lon-city.lon) > 1e-4 {
			t.Fatal("station click does not match marker")
		}
		v := aisMap{centerLat: city.lat, centerLon: city.lon, lonSpan: 12}
		p = v.project(city.lat+1, city.lon+1, b)
		expected = projectMercator(v.centerLat, v.centerLon, v.lonSpan, city.lat+1, city.lon+1, b)
		if rl.Vector2Distance(p, expected) > .01 {
			t.Fatal("AIS marker differs from tile projection")
		}
	}
}
