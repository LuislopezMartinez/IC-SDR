package screens

import (
	"strings"
	"testing"

	"go-zero/internal/sdr"
)

func TestRTLSDRHeaderUsesDriverSpecificControls(t *testing.T) {
	p := NewSDRHeaderPanel(nil, nil)
	p.current = sdr.HardwareSettings{
		Available: true, Driver: "rtlsdr", Device: "R820T",
		RFGain: 28, DirectSampling: 0,
	}
	p.refresh()
	if p.rfGain.Value() != 28 || !p.rfGain.Enabled() {
		t.Fatalf("RTL tuner gain unavailable: value %.1f enabled=%v", p.rfGain.Value(), p.rfGain.Enabled())
	}
	if !p.ifGain.Enabled() || p.setpoint.Enabled() {
		t.Fatalf("driver-specific controls incorrect: direct=%v setpoint=%v", p.ifGain.Enabled(), p.setpoint.Enabled())
	}
	if !strings.Contains(p.rfLabel.Text(), "TUNER") || !strings.Contains(p.ifLabel.Text(), "DIRECT") {
		t.Fatalf("RTL labels incorrect: %q / %q", p.rfLabel.Text(), p.ifLabel.Text())
	}
	p.current.AGC = true
	p.refresh()
	if p.rfGain.Enabled() {
		t.Fatal("manual tuner gain remained enabled under AGC")
	}
}

func TestGenericHeaderUsesOverallGain(t *testing.T) {
	p := NewSDRHeaderPanel(nil, nil)
	p.current = sdr.HardwareSettings{
		Available: true, Driver: "hackrf", Device: "HackRF One",
		RFGain: 14, AGC: false,
	}
	p.refresh()
	if !p.rfGain.Enabled() || p.ifGain.Enabled() || p.setpoint.Enabled() {
		t.Fatalf("generic controls incorrect: gain=%v if=%v setpoint=%v", p.rfGain.Enabled(), p.ifGain.Enabled(), p.setpoint.Enabled())
	}
	if !strings.Contains(p.rfLabel.Text(), "GAIN") {
		t.Fatalf("generic gain label incorrect: %q", p.rfLabel.Text())
	}
}
