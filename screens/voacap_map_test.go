package screens

import (
	"math"
	"testing"

	"go-zero/internal/voacap"
)

func TestVOACAPReliabilityAtUsesSelectedBands(t *testing.T) {
	viewer := &voacapMap{snapshot: voacap.Snapshot{
		Settings: voacap.Settings{Bands: map[string]bool{"20 m": true}},
		Prediction: voacap.Prediction{Cells: []voacap.Cell{
			{Latitude: 10, Longitude: 20, Reliability: map[string]float64{"20 m": 73, "40 m": 95}},
		}},
	}}

	got, ok := viewer.reliabilityAt(10, 20)
	if !ok || math.Abs(got-73) > 0.001 {
		t.Fatalf("reliabilityAt() = %v, %v; want 73, true", got, ok)
	}
}

func TestVOACAPReliabilityAtInterpolatesNearbyCells(t *testing.T) {
	viewer := &voacapMap{snapshot: voacap.Snapshot{
		Settings: voacap.Settings{Bands: map[string]bool{"20 m": true}},
		Prediction: voacap.Prediction{Cells: []voacap.Cell{
			{Latitude: 0, Longitude: -1, Reliability: map[string]float64{"20 m": 40}},
			{Latitude: 0, Longitude: 1, Reliability: map[string]float64{"20 m": 80}},
		}},
	}}

	got, ok := viewer.reliabilityAt(0, 0)
	if !ok || math.Abs(got-60) > 0.001 {
		t.Fatalf("reliabilityAt() = %v, %v; want 60, true", got, ok)
	}
}

func TestSolarSSNRoundsToVOACAPSelectorStep(t *testing.T) {
	if got := solarSSN(81.32); got != 80 {
		t.Fatalf("solarSSN(81.32) = %d; want 80", got)
	}
	if got := solarSSN(303); got != 300 {
		t.Fatalf("solarSSN(303) = %d; want 300", got)
	}
}
