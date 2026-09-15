package screens

import (
	"encoding/json"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDistanceMapRender(t *testing.T) {
	dir := os.Getenv("DISTANCE_MAP_RENDER_DIR")
	if dir == "" {
		t.Skip("visual QA not requested")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1440, 900, "Distance map QA")
	defer rl.CloseWindow()
	v := newDistanceMap()
	v.expanded = true
	e := make([]*float64, 81)
	for i := range e {
		h := 500. + float64(i)*1.3
		if i > 30 && i < 50 {
			h += 100
		}
		e[i] = &h
	}
	v.elevations = e
	v.due = v.due.AddDate(1, 0, 0)
	canvas := rl.LoadRenderTexture(1440, 900)
	defer rl.UnloadRenderTexture(canvas)
	os.MkdirAll(dir, 0755)
	for _, name := range []string{"distance-map.png", "distance-locations.png"} {
		rl.BeginTextureMode(canvas)
		v.draw()
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(canvas.Texture)
		rl.ImageFlipVertical(img)
		if !rl.ExportImage(*img, filepath.Join(dir, name)) {
			t.Fatal("render failed")
		}
		rl.UnloadImage(img)
		v.library = true
	}
	if v.cancel != nil {
		v.cancel()
	}
}

func TestDistanceLocationsPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "locations.json")
	v := &distanceMap{locationsPath: path, locations: []savedDistanceLocation{{Name: "Casa", Lat: 40.4168, Lon: -3.7038}}}
	v.persist()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored []savedDistanceLocation
	if json.Unmarshal(data, &stored) != nil || len(stored) != 1 || stored[0].Name != "Casa" {
		t.Fatal("location not saved")
	}
	v.locations[0].Name = "Antena"
	v.persist()
	data, _ = os.ReadFile(path)
	json.Unmarshal(data, &stored)
	if stored[0].Name != "Antena" {
		t.Fatal("location edit not saved")
	}
	v.locations = nil
	v.persist()
	data, _ = os.ReadFile(path)
	json.Unmarshal(data, &stored)
	if len(stored) != 0 {
		t.Fatal("location deletion not saved")
	}
}
