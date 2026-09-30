package voacap

import "testing"

func TestDefaultsSelectUsefulHFBands(t *testing.T) {
	settings := Defaults()
	if !settings.Bands["20 m"] || !settings.Bands["15 m"] || settings.Mode != "SSB" || settings.PowerW != 100 {
		t.Fatalf("unexpected defaults: %+v", settings)
	}
}

func TestNormalizeRejectsInvalidPropagationSettings(t *testing.T) {
	settings := Settings{Latitude: 100, Longitude: -200, Mode: "FM", PowerW: -1, UTCHour: 40}
	settings.Normalize()
	if settings.Latitude != 0 || settings.Longitude != 0 || settings.Mode != "SSB" || settings.PowerW != 100 || settings.UTCHour != 23 {
		t.Fatalf("settings were not normalized: %+v", settings)
	}
}
