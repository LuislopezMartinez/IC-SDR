//go:build windows

package omnirig

import "testing"

func TestModeMappings(t *testing.T) {
	for mode, raw := range map[string]int32{
		ModeCW: pmCWU, ModeUSB: pmSSBU, ModeLSB: pmSSBL, ModeDigital: pmDIGU, ModeAM: pmAM, ModeFM: pmFM,
	} {
		if got := modeToParam(mode); got != raw {
			t.Fatalf("modeToParam(%q) = %d, want %d", mode, got, raw)
		}
		if got := paramToMode(raw); got != mode {
			t.Fatalf("paramToMode(%d) = %q, want %q", raw, got, mode)
		}
	}
	if modeToParam("unsupported") != 0 || paramToMode(1) != ModeUnknown {
		t.Fatal("unsupported Omni-Rig mode was not ignored")
	}
}

func TestWritableFrequencyPropertyUsesProfileCapabilities(t *testing.T) {
	tests := []struct {
		mask int32
		want string
	}{
		{mask: pmFreq, want: "Freq"},
		{mask: pmFreqA, want: "FreqA"},
		{mask: pmFreqB, want: "FreqB"},
		{mask: pmFreq | pmFreqA, want: "Freq"},
		{mask: 0, want: ""},
	}
	for _, test := range tests {
		if got := writableFrequencyProperty(test.mask); got != test.want {
			t.Errorf("writableFrequencyProperty(%d) = %q, want %q", test.mask, got, test.want)
		}
	}
}
