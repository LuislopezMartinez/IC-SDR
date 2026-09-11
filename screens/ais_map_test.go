package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"go-zero/internal/ais"
	"go-zero/simpleui"
)

func TestAISMapProjectionRoundTrip(t *testing.T) {
	v := aisMap{centerLat: 40, centerLon: -4, lonSpan: 12}
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	p := v.project(43.36, -8.41, b)
	lat, lon := v.unproject(p, b)
	if abs64(lat-43.36) > 1e-5 || abs64(lon+8.41) > 1e-5 {
		t.Fatalf("projection mismatch: %.6f %.6f", lat, lon)
	}
}

func TestAISMapLabelClickAndZoomFromDetailsPanel(t *testing.T) {
	lat, lon := 43.36, -8.41
	v := aisMap{centerLat: 43.36, centerLon: -8.41, lonSpan: 2, selected: -1, vessels: []ais.Vessel{{MMSI: 224123456, Latitude: &lat, Longitude: &lon}}}
	b := rl.Rectangle{X: 20, Y: 78, Width: 970, Height: 690}
	p := v.project(lat, lon, b)
	v.selectAt(rl.Vector2{X: p.X + 100, Y: p.Y}, b)
	if v.selected != 0 {
		t.Fatal("ship label was not clickable")
	}
	before := v.lonSpan
	v.zoomAt(rl.Vector2{X: 1200, Y: 400}, b, 1)
	if v.lonSpan >= before {
		t.Fatal("wheel over details panel did not zoom AIS map")
	}
}

func TestAISMapKeepsSelectedVesselWhenSnapshotOrderChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vessels.json")
	lat, lon := 43.36, -8.41
	v := aisMap{
		path: path, selected: 0, selectedMMSI: 224123456,
		vessels: []ais.Vessel{{MMSI: 224123456, Name: "SELECTED", Latitude: &lat, Longitude: &lon}},
		tracks:  make(map[uint32][]geoPoint),
	}
	updated := []ais.Vessel{
		{MMSI: 224999999, Name: "NEW", Latitude: &lat, Longitude: &lon},
		{MMSI: 224123456, Name: "SELECTED", Latitude: &lat, Longitude: &lon},
	}
	data, _ := json.Marshal(updated)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	v.read()
	if v.selected != 1 || v.vessels[v.selected].MMSI != 224123456 {
		t.Fatalf("selection moved after reorder: index=%d mmsi=%d", v.selected, v.vessels[v.selected].MMSI)
	}
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func TestAISMapRender(t *testing.T) {
	dir := os.Getenv("AIS_MAP_RENDER_DIR")
	if dir == "" {
		t.Skip("visual QA not requested")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1360, 800, "AIS map QA")
	defer rl.CloseWindow()
	simpleui.SetTextScale(1.25)
	lat1, lon1, speed1, course1, heading1 := 43.36, -8.41, 12.4, 238.0, 240.0
	lat2, lon2, speed2, course2 := 43.31, -8.50, 6.2, 91.0
	vessels := []ais.Vessel{{MMSI: 224123456, Name: "GALICIA STAR", Callsign: "EABC", ShipTypeText: "Cargo", StatusText: "Under way", Latitude: &lat1, Longitude: &lon1, Speed: &speed1, Course: &course1, Heading: &heading1, Destination: "A CORUÑA", LastSeen: time.Now(), Messages: 143}, {MMSI: 224654321, Name: "MAR BLUE", Latitude: &lat2, Longitude: &lon2, Speed: &speed2, Course: &course2, LastSeen: time.Now(), Messages: 44}}
	data, _ := json.Marshal(vessels)
	path := filepath.Join(dir, "vessels.json")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(path, data, 0644)
	v := &aisMap{path: path, centerLat: 43.34, centerLon: -8.45, lonSpan: .45, selected: 0, tracks: map[uint32][]geoPoint{224123456: {{43.39, -8.34}, {43.37, -8.38}, {43.36, -8.41}}}}
	canvas := rl.LoadRenderTexture(1360, 800)
	defer rl.UnloadRenderTexture(canvas)
	rl.BeginTextureMode(canvas)
	v.draw()
	rl.EndTextureMode()
	img := rl.LoadImageFromTexture(canvas.Texture)
	defer rl.UnloadImage(img)
	rl.ImageFlipVertical(img)
	if !rl.ExportImage(*img, filepath.Join(dir, "ais-map.png")) {
		t.Fatal("map export failed")
	}
}
