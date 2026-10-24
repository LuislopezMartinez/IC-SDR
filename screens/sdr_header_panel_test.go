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

func TestHackRFHeaderExposesHardwareStages(t *testing.T) {
	p := NewSDRHeaderPanel(nil, nil)
	p.current = sdr.HardwareSettings{
		Available: true, Driver: "hackrf", Device: "HackRF One",
		RFGain: 24, IFGain: 40, ExternalAmp: true, BiasT: true,
	}
	p.refresh()
	if p.rfGain.Value() != 24 || p.ifGain.Value() != 40 || !p.rfGain.Enabled() || !p.ifGain.Enabled() {
		t.Fatalf("HackRF gains unavailable: LNA=%.0f VGA=%.0f enabled=%v/%v", p.rfGain.Value(), p.ifGain.Value(), p.rfGain.Enabled(), p.ifGain.Enabled())
	}
	if !p.agc.Active() || !p.biasT.Active() || !p.agc.Enabled() || !p.biasT.Enabled() {
		t.Fatalf("HackRF AMP/Bias-T unavailable: active=%v/%v enabled=%v/%v", p.agc.Active(), p.biasT.Active(), p.agc.Enabled(), p.biasT.Enabled())
	}
	if !strings.Contains(p.rfLabel.Text(), "LNA") || !strings.Contains(p.ifLabel.Text(), "VGA") {
		t.Fatalf("HackRF labels incorrect: %q / %q", p.rfLabel.Text(), p.ifLabel.Text())
	}
	p.current = sdr.HardwareSettings{Available: true, Driver: "rtlsdr"}
	p.refresh()
	if p.agc.Label() == "AMP" {
		t.Fatal("HackRF AMP label leaked into RTL-SDR controls")
	}
}

func TestAirspyHeaderExposesThreeGainStages(t *testing.T) {
	p := NewSDRHeaderPanel(nil, nil)
	p.current = sdr.HardwareSettings{
		Available: true, Driver: "airspy", Device: "Airspy",
		RFGain: 8, IFGain: 7, BasebandGain: 6, BiasT: true,
	}
	p.refresh()
	if p.rfGain.Value() != 8 || p.ifGain.Value() != 7 || p.ppm.Value() != 6 {
		t.Fatalf("Airspy gains unavailable: LNA=%.0f MIX=%.0f VGA=%.0f", p.rfGain.Value(), p.ifGain.Value(), p.ppm.Value())
	}
	if !strings.Contains(p.rfLabel.Text(), "LNA") || !strings.Contains(p.ifLabel.Text(), "MIX") || !strings.Contains(p.ppmLabel.Text(), "VGA") {
		t.Fatalf("Airspy labels incorrect: %q / %q / %q", p.rfLabel.Text(), p.ifLabel.Text(), p.ppmLabel.Text())
	}
	if !p.rfGain.Enabled() || !p.ifGain.Enabled() || !p.ppm.Enabled() || !p.biasT.Active() {
		t.Fatalf("Airspy manual controls unavailable")
	}
	p.current.AGC = true
	p.refresh()
	if p.rfGain.Enabled() || p.ifGain.Enabled() || p.ppm.Enabled() {
		t.Fatal("Airspy manual gains remained enabled under AGC")
	}
}

func TestRSPDuoHeaderExposesThreeInputSelectors(t *testing.T) {
	p := NewSDRHeaderPanel(nil, nil)
	p.current = sdr.HardwareSettings{
		Available: true, Driver: "sdrplay", Device: "RSPduo", Antenna: "Tuner 1 Hi-Z",
		Antennas: []string{"Tuner 1 50 ohm", "Tuner 1 Hi-Z", "Tuner 2 50 ohm"},
	}
	p.refresh()
	want := []string{"T1 50Ω", "T1 Hi-Z", "T2 50Ω"}
	for index, button := range p.antenna {
		if !button.Visible() || !strings.Contains(button.Label(), want[index]) {
			t.Fatalf("RSPduo selector %d incorrect: visible=%v label=%q", index, button.Visible(), button.Label())
		}
	}
}
