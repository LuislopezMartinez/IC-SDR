package screens

import (
	"math"
	"strings"
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

func TestClampTextureSrcStaysInsideImage(t *testing.T) {
	tex := rl.Texture2D{Width: 100, Height: 50}
	got := clampTextureSrc(rl.Rectangle{X: -10, Y: -4, Width: 140, Height: 80}, tex)
	if got.X != 0 || got.Y != 0 || got.Width != 100 || got.Height != 50 {
		t.Fatalf("unclamped source %+v", got)
	}
}

func TestIsPNGRejectsHTML(t *testing.T) {
	if isPNG([]byte("<html>not a tile</html>")) {
		t.Fatal("HTML accepted as a PNG tile")
	}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 1, 2, 3}
	if !isPNG(png) {
		t.Fatal("PNG signature rejected")
	}
}

func TestTileImageTypeAcceptsJPEG(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0x10}
	if tileImageType(jpeg) != ".jpg" {
		t.Fatal("JPEG tile rejected")
	}
	if tileImageType([]byte("API KEY REQUIRED")) != "" {
		t.Fatal("text accepted as a map tile")
	}
}

func TestMapTileURLsAreKeyless(t *testing.T) {
	urls := mapTileURLs(tileKey{z: 10, x: 512, y: 340})
	if len(urls) < 2 {
		t.Fatal("need a primary tile host and a fallback")
	}
	for _, url := range urls {
		lower := strings.ToLower(url)
		if strings.Contains(lower, "carto") || strings.Contains(lower, "apikey") || strings.Contains(lower, "api_key") {
			t.Fatalf("tile URL still needs a key or Carto: %s", url)
		}
	}
	if urls[0] != "https://tile.openstreetmap.de/10/512/340.png" {
		t.Fatalf("primary tile URL = %s", urls[0])
	}
	if urls[1] != "https://server.arcgisonline.com/ArcGIS/rest/services/World_Street_Map/MapServer/tile/10/340/512" {
		t.Fatalf("fallback tile URL = %s", urls[1])
	}
}
