package sdr

import "testing"

func TestAntennaShortLabel(t *testing.T) {
	cases := map[string]string{
		"Antenna A":      "A",
		"Antenna B":      "B",
		"Antenna C":      "C",
		"A":              "A",
		"c":              "C",
		"Hi-Z":           "Hi-Z",
		"Tuner 1 50 ohm": "T1",
		"Tuner 2 50 ohm": "T2",
	}
	for name, want := range cases {
		if got := AntennaShortLabel(name); got != want {
			t.Fatalf("%q: got %q want %q", name, got, want)
		}
	}
}

func TestMatchAntennaAcceptsShortAndSoapyNames(t *testing.T) {
	available := []string{"Antenna A", "Antenna B", "Antenna C"}
	if got := MatchAntenna("C", available); got != "Antenna C" {
		t.Fatalf("short C mapped to %q", got)
	}
	if got := MatchAntenna("antenna b", available); got != "Antenna B" {
		t.Fatalf("case-insensitive B mapped to %q", got)
	}
	if got := MatchAntenna("Antenna A", available); got != "Antenna A" {
		t.Fatalf("exact A mapped to %q", got)
	}
}

func TestDefaultSDRplayAntennasAreABC(t *testing.T) {
	got := defaultAntennasFor("sdrplay")
	if len(got) != 3 || got[0] != "Antenna A" || got[2] != "Antenna C" {
		t.Fatalf("RSP-Dx fallback antennas: %v", got)
	}
	if defaultAntennasFor("rtlsdr") != nil {
		t.Fatal("RTL-SDR should not invent extra antenna ports")
	}
}

func TestSoapyAntennaSwitchIsSDRplayOnly(t *testing.T) {
	if !soapyAntennaSwitchSupported("sdrplay") || !soapyAntennaSwitchSupported("sdrplay3") {
		t.Fatal("SDRplay should use Soapy listAntennas / getAntenna")
	}
	for _, driver := range []string{"rtlsdr", "hackrf", "airspy"} {
		if soapyAntennaSwitchSupported(driver) {
			t.Fatalf("%s must not call Soapy antenna APIs after the IQ stream starts", driver)
		}
	}
}
