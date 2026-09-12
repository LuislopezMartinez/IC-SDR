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
		Available: true, Driver: "hackrf", Device: "HackRF Pro",
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

func TestRSPDxHeaderShowsThreeAntennaPorts(t *testing.T) {
	p := NewSDRHeaderPanel(nil, nil)
	p.current = sdr.HardwareSettings{
		Available: true, Driver: "sdrplay", Device: "RSPdx",
		Antenna:  "Antenna A",
		Antennas: []string{"Antenna A", "Antenna B", "Antenna C"},
	}
	p.refresh()
	for index, want := range []string{"A", "B", "C"} {
		if !p.antenna[index].Visible() || !p.antenna[index].Enabled() || p.antenna[index].Label() != want {
			t.Fatalf("antenna %d: visible=%v enabled=%v label=%q want %q",
				index, p.antenna[index].Visible(), p.antenna[index].Enabled(), p.antenna[index].Label(), want)
		}
	}
	p.current.Antenna = "Antenna C"
	p.refresh()
	if p.antenna[2].Label() != "C" {
		t.Fatal("port C was not selected")
	}
}

func TestGenericHeaderKeepsDCSpikeSwitch(t *testing.T) {
	p := NewSDRHeaderPanel(nil, nil)
	p.current = sdr.HardwareSettings{
		Available: true, Driver: "hackrf", Device: "HackRF Pro",
		RFGain: 14, AGC: false,
	}
	p.refresh()
	if !p.dcSpike.Enabled() || !p.dcSpike.Active() {
		t.Fatalf("DC spike switch unavailable on generic SDR: enabled=%v active=%v", p.dcSpike.Enabled(), p.dcSpike.Active())
	}
	if p.iqCorrection.Enabled() || p.rfNotch.Enabled() {
		t.Fatal("hardware IQ/notch controls must stay disabled on generic profiles")
	}
}

func TestDCSpikeSwitchUpdatesReceiver(t *testing.T) {
	receiver := sdr.NewReceiver(sdr.Config{SampleRate: 2_048_000, FFTSize: 4096})
	p := NewSDRHeaderPanel(receiver, nil)
	if !receiver.RemoveDCSpike() {
		t.Fatal("DC spike removal should start enabled")
	}
	receiver.SetRemoveDCSpike(false)
	p.refresh()
	if p.dcSpike.Active() {
		t.Fatal("header did not follow the receiver DC spike flag")
	}
	receiver.SetRemoveDCSpike(true)
	p.refresh()
	if !p.dcSpike.Active() {
		t.Fatal("header did not restore the DC spike flag")
	}
}

func TestRTLHeaderHidesAntennaSwitch(t *testing.T) {
	p := NewSDRHeaderPanel(nil, nil)
	p.current = sdr.HardwareSettings{Available: true, Driver: "rtlsdr", Device: "R820T"}
	p.refresh()
	for index, button := range p.antenna {
		if button.Visible() {
			t.Fatalf("RTL-SDR showed antenna button %d", index)
		}
	}
}
