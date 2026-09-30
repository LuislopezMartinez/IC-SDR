package voacap

import "testing"

func TestSolarSelectorsUseNewestObservation(t *testing.T) {
	cycle := newestCycle([]solarCycleRecord{{TimeTag: "2026-08", SSN: 80}, {TimeTag: "2026-09", SSN: 92}})
	flux := newestFlux([]solarFluxRecord{{TimeTag: "2026-09-28T20:00:00", Flux: 95}, {TimeTag: "2026-09-29T17:00:00", Flux: 92}})
	index := newestKIndex([]planetaryKRecord{{TimeTag: "2026-09-29T12:00:00", Kp: .67}, {TimeTag: "2026-09-29T15:00:00", Kp: 1.33}})
	if cycle.SSN != 92 || flux.Flux != 92 || index.Kp != 1.33 {
		t.Fatalf("newest observations = SSN %.0f, SFI %.0f, Kp %.2f", cycle.SSN, flux.Flux, index.Kp)
	}
}
