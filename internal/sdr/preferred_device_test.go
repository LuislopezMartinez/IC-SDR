package sdr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreferredDeviceAcceptsAirspy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sdr-device.json")
	if err := os.WriteFile(path, []byte(`{"Driver":"airspy","Serial":"A1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadPreferredDevice(path)
	if got.Driver != "airspy" || got.Serial != "A1" {
		t.Fatalf("unexpected preferred device: %+v", got)
	}
}
