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
	if containsDriver(candidates, "hackrf") {
		t.Fatalf("HackRF must not be probed without a serial: %+v", driversOf(candidates))
	}
	if !containsDriver(candidates, "airspy") {
		t.Fatalf("probe list missing Airspy: %+v", driversOf(candidates))
	}
}

func TestDeviceCandidatesPreferEnumeratedHardware(t *testing.T) {
	candidates := deviceCandidates(
		Config{Driver: "sdrplay"},
		[]soapyIdentity{
			{Driver: "sdrplay", Serial: "RSP1", Label: "RSPdx"},
			{Driver: "hackrf", Serial: "ABC", Label: "HackRF One"},
		},
	)
	if candidates[0].Driver != "sdrplay" {
		t.Fatalf("requested driver must stay first when it was enumerated: %+v", candidates)
	}
	if !containsDriverSerial(candidates, "hackrf", "ABC") {
		t.Fatalf("enumerated HackRF missing: %+v", candidates)
	}
	if containsDriver(candidates, "lime") {
		t.Fatalf("unused probe drivers should not run when hardware was enumerated: %+v", driversOf(candidates))
	}
}

func TestDeviceCandidatesOpenEnumeratedHackRFProWithoutProbingRSP(t *testing.T) {
	candidates := deviceCandidates(
		Config{Driver: "sdrplay"},
		[]soapyIdentity{{Driver: "hackrf", Serial: "0000000000000000abcdef", Label: "HackRF Pro #0 abcdef"}},
	)
	if len(candidates) == 0 || candidates[0].Driver != "hackrf" || candidates[0].Serial == "" {
		t.Fatalf("HackRF Pro must be opened by serial first: %+v", candidates)
	}
	if containsDriver(candidates, "sdrplay") {
		t.Fatalf("absent RSP must not be probed ahead of HackRF Pro: %+v", driversOf(candidates))
	}
	if containsDriverSerial(candidates, "hackrf", "") {
		t.Fatalf("HackRF must not be opened without a serial: %+v", candidates)
	}
}

func TestDeviceCandidatesDoNotRequireRSPForRTLSDR(t *testing.T) {
	candidates := deviceCandidates(Config{Driver: "rtlsdr"}, nil)
	if len(candidates) == 0 || candidates[0].Driver != "rtlsdr" {
		t.Fatalf("unexpected RTL-SDR candidates: %+v", candidates)
	}
}

func TestDeviceCandidatesPreferSavedRTLWhenEnumerated(t *testing.T) {
	candidates := deviceCandidates(
		Config{Driver: "rtlsdr", Serial: "00000001"},
		[]soapyIdentity{
			{Driver: "sdrplay", Serial: "RSP1", Label: "RSP1B"},
			{Driver: "rtlsdr", Serial: "00000001", Label: "Generic RTL2832U"},
		},
	)
	if len(candidates) == 0 || candidates[0].Driver != "rtlsdr" || candidates[0].Serial != "00000001" {
		t.Fatalf("saved RTL-SDR must open first: %+v", candidates)
	}
}

func TestFormatDeviceLabelUsesSerialSuffix(t *testing.T) {
	got := FormatDeviceLabel(DeviceOption{Driver: "rtlsdr", Serial: "00000001", Label: "Generic RTL2832U"})
	if got != "Generic RTL2832U · 00000001" {
		t.Fatal(got)
	}
}

func TestSelectDeviceRejectedAfterClose(t *testing.T) {
	receiver := NewReceiver(Config{SampleRate: 2_048_000, FFTSize: 4096, Driver: "rtlsdr"})
	receiver.Close()
	if err := receiver.SelectDevice("rtlsdr", "00000001"); err == nil {
		t.Fatal("SelectDevice after Close must fail")
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

func TestHackRFSampleRateAttemptsIncludeLegacyTwoMega(t *testing.T) {
	got := sampleRateAttempts("hackrf", 2_048_000)
	if got[0] != 2_048_000 {
		t.Fatalf("first rate %v, want requested 2048000", got[0])
	}
	if !containsRate(got, 2_000_000) || !containsRate(got, 20_000_000) {
		t.Fatalf("HackRF One/Pro rates missing: %v", got)
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
