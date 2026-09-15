package sdr

import "testing"

func TestRSPDxAntennaMatching(t *testing.T) {
	for _, name := range []string{"RSPdx", "RSP-DX", "SDRplay RSPdx-R2"} {
		if !IsRSPDx(name) {
			t.Fatalf("expected RSP-Dx detection for %q", name)
		}
	}
	if IsRSPDx("RSP1A") {
		t.Fatal("RSP1A must not show RSP-Dx antenna controls")
	}
	available := []string{"Antenna A", "Antenna B", "Antenna C"}
	if got := MatchAntenna("B", available); got != "Antenna B" {
		t.Fatalf("matched antenna = %q", got)
	}
	if got := MatchAntenna("unknown", available); got != "" {
		t.Fatalf("unknown antenna = %q", got)
	}
}
