package sdr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeviceCandidatesFallBackFromSpecificRSPToRTLSDR(t *testing.T) {
	candidates := deviceCandidates(Config{Driver: "sdrplay", Serial: "RSP-SERIAL"}, nil)
	if len(candidates) < 3 {
		t.Fatalf("got %d candidates, want at least 3", len(candidates))
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
	if !containsDriver(candidates, "hackrf") || !containsDriver(candidates, "airspy") {
		t.Fatalf("probe list missing HackRF/Airspy: %+v", driversOf(candidates))
	}
}

func TestDeviceCandidatesPreferEnumeratedHardware(t *testing.T) {
	candidates := deviceCandidates(
		Config{Driver: "sdrplay"},
		[]soapyIdentity{{Driver: "hackrf", Serial: "ABC", Label: "HackRF One"}},
	)
	if candidates[0].Driver != "sdrplay" {
		t.Fatalf("requested driver must stay first: %+v", candidates)
	}
	if !containsDriverSerial(candidates, "hackrf", "ABC") {
		t.Fatalf("enumerated HackRF missing: %+v", candidates)
	}
	if containsDriver(candidates, "lime") {
		t.Fatalf("unused probe drivers should not run when hardware was enumerated: %+v", driversOf(candidates))
	}
}

func TestDeviceCandidatesDoNotRequireRSPForRTLSDR(t *testing.T) {
	candidates := deviceCandidates(Config{Driver: "rtlsdr"}, nil)
	if len(candidates) == 0 || candidates[0].Driver != "rtlsdr" {
		t.Fatalf("unexpected RTL-SDR candidates: %+v", candidates)
	}
}

func TestProfileForDrivers(t *testing.T) {
	if ProfileFor("rtlsdr") != ProfileRTLSDR {
		t.Fatal("rtlsdr profile")
	}
	if ProfileFor("sdrplay3") != ProfileSDRplay {
		t.Fatal("sdrplay3 profile")
	}
	if ProfileFor("hackrf") != ProfileGeneric {
		t.Fatal("hackrf profile")
	}
}

func TestSampleRateAttemptsKeepRequestedFirst(t *testing.T) {
	got := sampleRateAttempts("airspy", 2_048_000)
	if got[0] != 2_048_000 {
		t.Fatalf("first rate %v, want requested 2048000", got[0])
	}
	if !containsRate(got, 2_500_000) || !containsRate(got, 10_000_000) {
		t.Fatalf("airspy fallbacks missing: %v", got)
	}
}

func TestCollectSoapyModules(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"rtlsdrSupport.dll", "hackrfSupport.so", "notes.txt", "libAirspySupport.dylib"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := collectSoapyModules(dir)
	if len(got) != 3 {
		t.Fatalf("got %v, want 3 modules", got)
	}
}

func containsDriver(candidates []Config, driver string) bool {
	for _, candidate := range candidates {
		if candidate.Driver == driver {
			return true
		}
	}
	return false
}

func containsDriverSerial(candidates []Config, driver, serial string) bool {
	for _, candidate := range candidates {
		if candidate.Driver == driver && candidate.Serial == serial {
			return true
		}
	}
	return false
}

func driversOf(candidates []Config) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.Driver)
	}
	return out
}

func containsRate(rates []float64, want float64) bool {
	for _, rate := range rates {
		if rate == want {
			return true
		}
	}
	return false
}
