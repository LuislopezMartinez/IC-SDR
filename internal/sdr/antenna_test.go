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

func TestRSPDuoAntennaSelection(t *testing.T) {
	for _, name := range []string{"RSPduo", "RSP-DUO", "SDRplay RSP duo"} {
		if !IsRSPDuo(name) || !HasSelectableAntennas(name) {
			t.Fatalf("expected RSPduo detection for %q", name)
		}
	}
	if IsRSPDuo("RSPdx") {
		t.Fatal("RSP-Dx must not be detected as RSPduo")
	}
	available := []string{"Tuner 1 50 ohm", "Tuner 1 Hi-Z", "Tuner 2 50 ohm"}
	wantLabels := []string{"T1 50Ω", "T1 Hi-Z", "T2 50Ω"}
	for index, antenna := range available {
		if got := AntennaShortLabel(antenna); got != wantLabels[index] {
			t.Fatalf("short label for %q = %q, want %q", antenna, got, wantLabels[index])
		}
		if got := MatchAntenna(wantLabels[index], available); got != antenna {
			t.Fatalf("matched antenna for %q = %q, want %q", wantLabels[index], got, antenna)
		}
	}
}
