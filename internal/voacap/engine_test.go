package voacap

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPortableEngineProducesWorldGrid(t *testing.T) {
	if os.Getenv("GOZERO_VOACAP_INTEGRATION") != "1" {
		t.Skip("set GOZERO_VOACAP_INTEGRATION=1 to run the official engine")
	}
	if runtime.GOOS != "windows" {
		t.Skip("the official portable engine is a Windows binary")
	}
	root := filepath.Join("..", "..", "DATA", "tools", "voacap", "runtime")
	settings := Defaults()
	settings.Bands = map[string]bool{"20 m": true}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	prediction := Calculate(ctx, root, settings)
	if prediction.Error != "" {
		t.Fatalf("portable VOACAP failed: %s", prediction.Error)
	}
	if len(prediction.Cells) != 961 {
		t.Fatalf("got %d grid cells, want 961", len(prediction.Cells))
	}
	if _, ok := prediction.Cells[0].Reliability["20 m"]; !ok {
		t.Fatal("20 m reliability is missing")
	}
}

func TestPortableEngineProduces24HourRoute(t *testing.T) {
	if os.Getenv("GOZERO_VOACAP_INTEGRATION") != "1" {
		t.Skip("set GOZERO_VOACAP_INTEGRATION=1 to run the official engine")
	}
	if runtime.GOOS != "windows" {
		t.Skip("the official portable engine is a Windows binary")
	}
	root := filepath.Join("..", "..", "DATA", "tools", "voacap", "runtime")
	settings := Defaults()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	prediction := CalculateRoute(ctx, root, settings, RouteLocation{Latitude: 40.4168, Longitude: -3.7038}, RouteLocation{Latitude: 40.7128, Longitude: -74.0060})
	if prediction.Error != "" {
		t.Fatalf("portable VOACAP route failed: %s", prediction.Error)
	}
	if len(prediction.Reliability) != len(AmateurBands) || len(prediction.Reliability["20 m"]) != 24 {
		t.Fatalf("route matrix has %d bands and %d hours", len(prediction.Reliability), len(prediction.Reliability["20 m"]))
	}
	if prediction.Reliability["80 m"][1] == prediction.Reliability["60 m"][1] {
		t.Fatal("route reliability columns were not parsed as separate requested frequencies")
	}
	if prediction.DistanceKM < 5_000 || prediction.Azimuth < 250 {
		t.Fatalf("unexpected route geometry: %.0f km, %.0f degrees", prediction.DistanceKM, prediction.Azimuth)
	}
}

func TestStageRuntimeRecreatesPortableWorkingDirectories(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "bin_win"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "bin_win", "placeholder"), []byte("runtime"), 0o644); err != nil {
		t.Fatal(err)
	}
	staged, cleanup, err := stageRuntime(source)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, directory := range []string{"run", "areadata", "area_inv"} {
		info, statErr := os.Stat(filepath.Join(staged, directory))
		if statErr != nil || !info.IsDir() {
			t.Fatalf("staged runtime is missing %s: %v", directory, statErr)
		}
	}
}
