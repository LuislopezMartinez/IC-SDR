package sdr

import (
	"path/filepath"
	"testing"
)

func TestPreferredDeviceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "sdr-device.json")
	want := DeviceOption{Driver: "rtlsdr", Serial: "00000042"}
	if err := SavePreferredDevice(path, want); err != nil {
		t.Fatal(err)
	}
	got := LoadPreferredDevice(path)
	if got.Driver != want.Driver || got.Serial != want.Serial {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMergeDeviceOptionsKeepsUniqueDriverAndSerial(t *testing.T) {
	options := mergeDeviceOptions([]DeviceOption{{Driver: "rtlsdr", Serial: "A"}}, []DeviceOption{{Driver: "rtlsdr", Serial: "A"}, {Driver: "rtlsdr", Serial: "B"}})
	if len(options) != 2 || options[0].Serial != "A" || options[1].Serial != "B" {
		t.Fatalf("unexpected devices: %+v", options)
	}
}
