package voacap

import (
	"testing"
	"time"
)

func TestRadioBlackoutOnlyReducesConfidenceOnSunlitSide(t *testing.T) {
	noon := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	solar := SolarData{RScale: 2}
	day := AssessLiveConditions(0, 0, noon, solar)
	night := AssessLiveConditions(0, 180, noon, solar)
	if day.Confidence != "BAJA" || day.Reason != "ABSORCIÓN CAPA D" {
		t.Fatalf("day assessment = %+v", day)
	}
	if night.Confidence != "ALTA" {
		t.Fatalf("night assessment = %+v", night)
	}
}

func TestGeomagneticStormTargetsHighLatitudes(t *testing.T) {
	at := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	solar := SolarData{Kp: 7}
	if got := AssessLiveConditions(65, 0, at, solar); got.Confidence != "BAJA" {
		t.Fatalf("polar assessment = %+v", got)
	}
	if got := AssessLiveConditions(20, 0, at, solar); got.Confidence != "ALTA" {
		t.Fatalf("low-latitude assessment = %+v", got)
	}
}
