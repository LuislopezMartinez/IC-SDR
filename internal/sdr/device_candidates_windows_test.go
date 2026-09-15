//go:build windows

package sdr

import "testing"

func TestDeviceCandidatesFallBackFromSpecificRSPToRTLSDR(t *testing.T) {
	candidates := deviceCandidates(Config{Driver: "sdrplay", Serial: "RSP-SERIAL"})
	if len(candidates) != 3 {
		t.Fatalf("got %d candidates, want 3", len(candidates))
	}
	if candidates[0].Driver != "sdrplay" || candidates[0].Serial != "RSP-SERIAL" {
		t.Fatalf("specific RSP must be tried first: %+v", candidates)
	}
	if candidates[1].Driver != "sdrplay" || candidates[1].Serial != "" {
		t.Fatalf("any RSP must be tried second: %+v", candidates)
	}
	if candidates[2].Driver != "rtlsdr" || candidates[2].Serial != "" {
		t.Fatalf("RTL-SDR fallback missing: %+v", candidates)
	}
}

func TestDeviceCandidatesDoNotRequireRSPForRTLSDR(t *testing.T) {
	candidates := deviceCandidates(Config{Driver: "rtlsdr"})
	if len(candidates) != 2 || candidates[0].Driver != "rtlsdr" || candidates[1].Driver != "sdrplay" {
		t.Fatalf("unexpected RTL-SDR candidates: %+v", candidates)
	}
}

func TestSavedRTLSerialFallsBackToOtherConnectedRadio(t *testing.T) {
	candidates := deviceCandidates(Config{Driver: "rtlsdr", Serial: "missing"})
	if len(candidates) != 3 || candidates[0].Serial != "missing" || candidates[1].Driver != "rtlsdr" || candidates[1].Serial != "" || candidates[2].Driver != "sdrplay" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
}

func TestHackRFSerialDoesNotFallBackToAnotherHackRF(t *testing.T) {
	candidates := deviceCandidates(Config{Driver: "hackrf", Serial: "missing"})
	if len(candidates) != 2 || candidates[0].Driver != "hackrf" || candidates[0].Serial != "missing" || candidates[1].Driver != "rtlsdr" {
		t.Fatalf("unexpected HackRF candidates: %+v", candidates)
	}
}
