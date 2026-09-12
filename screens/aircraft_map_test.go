package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/aircraft"
	"go-zero/simpleui"
)

func TestAircraftMapProjectionRoundTrip(t *testing.T) {
	v := aircraftMap{centerLat: 40, centerLon: -4, lonSpan: 12}
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	p := v.project(41.2, -2.3, b)
	lat, lon := v.unproject(p, b)
	if abs64(lat-41.2) > 1e-5 || abs64(lon+2.3) > 1e-5 {
		t.Fatalf("projection mismatch %.6f %.6f", lat, lon)
	}
}

func TestAircraftMapFitsAfterPositionsArrive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aircraft.json")
	v := &aircraftMap{path: path, centerLat: 40.2, centerLon: -3.7, lonSpan: 14, selected: -1, tracks: make(map[string][]geoPoint)}
	withoutPosition, _ := json.Marshal([]aircraft.Aircraft{{ICAO: "7C6B28", Callsign: "QFA123"}})
	if err := os.WriteFile(path, withoutPosition, 0644); err != nil {
		t.Fatal(err)
	}
	v.read()
	if v.fitted {
		t.Fatal("view must stay unlocked until an aircraft has a position")
	}
	if v.centerLat != 40.2 || v.centerLon != -3.7 {
		t.Fatalf("view moved without positions: %.4f %.4f", v.centerLat, v.centerLon)
	}

	lat, lon := -33.86, 151.21
	withPosition, _ := json.Marshal([]aircraft.Aircraft{{ICAO: "7C6B28", Callsign: "QFA123", Latitude: &lat, Longitude: &lon}})
	if err := os.WriteFile(path, withPosition, 0644); err != nil {
		t.Fatal(err)
	}
	v.next = time.Time{}
	v.read()
	if !v.fitted {
		t.Fatal("expected the map to fit once a position arrived")
	}
	if abs64(v.centerLat-lat) > 0.01 || abs64(v.centerLon-lon) > 0.01 {
		t.Fatalf("map did not center on traffic: lat=%.4f lon=%.4f", v.centerLat, v.centerLon)
	}
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	p := v.project(lat, lon, b)
	if !rl.CheckCollisionPointRec(p, b) {
		t.Fatalf("positioned aircraft is off the map: %+v", p)
	}
}

func TestAircraftMapLabelClickAndZoomFromDetailsPanel(t *testing.T) {
	lat, lon := 40.48, -3.57
	v := aircraftMap{centerLat: lat, centerLon: lon, lonSpan: 2, selected: -1, list: []aircraft.Aircraft{{ICAO: "3451A2", Latitude: &lat, Longitude: &lon}}}
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	p := v.project(lat, lon, b)
	v.selectAt(rl.Vector2{X: p.X + 120, Y: p.Y}, b)
	if v.selected != 0 {
		t.Fatal("aircraft label was not clickable")
	}
	before := v.lonSpan
	v.zoomAt(rl.Vector2{X: 1200, Y: 400}, b, 1)
	if v.lonSpan >= before {
		t.Fatal("wheel over details panel did not zoom aircraft map")
	}
}

func TestAircraftMapKeepsSelectionWhenListReorders(t *testing.T) {
	lat, lon := 40.48, -3.57
	otherLat, otherLon := 41.0, -4.0
	v := aircraftMap{
		centerLat: lat, centerLon: lon, lonSpan: 2, selected: -1,
		tracks: make(map[string][]geoPoint),
		list: []aircraft.Aircraft{
			{ICAO: "AAAAAA", Latitude: &lat, Longitude: &lon},
			{ICAO: "BBBBBB", Latitude: &otherLat, Longitude: &otherLon},
		},
	}
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	p := v.project(otherLat, otherLon, b)
	v.selectAt(p, b)
	if v.selectedICAO != "BBBBBB" || v.selected != 1 {
		t.Fatalf("selected ICAO=%q index=%d", v.selectedICAO, v.selected)
	}
	v.list = []aircraft.Aircraft{
		{ICAO: "CCCCCC", Latitude: &lat, Longitude: &lon},
		{ICAO: "AAAAAA", Latitude: &lat, Longitude: &lon},
		{ICAO: "BBBBBB", Latitude: &otherLat, Longitude: &otherLon},
	}
	v.restoreSelection()
	if v.selected != 2 || v.list[v.selected].ICAO != "BBBBBB" {
		t.Fatalf("selection lost after LastSeen reorder: index=%d", v.selected)
	}
}

func TestAircraftMarkerColorsAreDark(t *testing.T) {
	for name, c := range map[string]rl.Color{
		"1090":  aircraftMarker1090,
		"978":   aircraftMarker978,
		"stale": aircraftMarkerStale,
		"label": mapCalloutText,
	} {
		if int(c.R)+int(c.G)+int(c.B) > 180 {
			t.Fatalf("%s marker/label is too light for the street map: %+v", name, c)
		}
	}
}

func TestAircraftMapRender(t *testing.T) {
	dir := os.Getenv("AIRCRAFT_MAP_RENDER_DIR")
	if dir == "" {
		t.Skip("visual QA not requested")
	}
	_ = os.MkdirAll(dir, 0755)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1360, 800, "Aircraft QA")
	defer rl.CloseWindow()
	simpleui.SetTextScale(1.25)
	lat, lon, spd, trk, vr := 40.48, -3.57, 265., 215., -640.
	alt := 11800
	list := []aircraft.Aircraft{{ICAO: "3451A2", Callsign: "IBE3174", Latitude: &lat, Longitude: &lon, Altitude: &alt, Speed: &spd, Track: &trk, VerticalRate: &vr, Source: aircraft.Mode1090, LastSeen: time.Now(), Messages: 284}}
	data, _ := json.Marshal(list)
	path := filepath.Join(dir, "aircraft.json")
	_ = os.WriteFile(path, data, 0644)
	v := &aircraftMap{path: path, centerLat: 40.4, centerLon: -3.7, lonSpan: 4, selected: 0, tracks: map[string][]geoPoint{"3451A2": {{40.9, -2.8}, {40.7, -3.1}, {40.48, -3.57}}}}
	canvas := rl.LoadRenderTexture(1360, 800)
	defer rl.UnloadRenderTexture(canvas)
	rl.BeginTextureMode(canvas)
	v.draw()
	rl.EndTextureMode()
	img := rl.LoadImageFromTexture(canvas.Texture)
	defer rl.UnloadImage(img)
	rl.ImageFlipVertical(img)
	if !rl.ExportImage(*img, filepath.Join(dir, "aircraft-map.png")) {
		t.Fatal("render failed")
	}
}
