//go:build windows

package sdr

import (
	"math"
	"os"
	"testing"

	"go-zero/internal/resources"
)

// This test is opt-in because it needs exclusive access to a connected dongle.
func TestRTLSDRHardwareControls(t *testing.T) {
	if os.Getenv("RTLSDR_HARDWARE_TEST") != "1" {
		t.Skip("set RTLSDR_HARDWARE_TEST=1 with an RTL-SDR connected")
	}
	root := resources.Path("runtime", "windows-x64")
	device, err := openSoapy(Config{RuntimeRoot: root, Driver: "rtlsdr", FrequencyHz: 100_000_000, SampleRate: 2_048_000, FFTSize: 4096})
	if err != nil {
		t.Fatal(err)
	}
	original := device.hardwareSettings()
	defer func() {
		_ = device.applyHardwareSettings(original)
		device.close()
	}()

	want := original
	want.AGC = false
	want.RFGain = 28
	want.PPM = original.PPM + 1
	want.DigitalAGC = !original.DigitalAGC
	want.OffsetTuning = !original.OffsetTuning
	want.IQSwap = !original.IQSwap
	if err = device.applyHardwareSettings(want); err != nil {
		t.Fatal(err)
	}
	got := device.hardwareSettings()
	if got.AGC || math.Abs(float64(got.RFGain-want.RFGain)) > .2 || math.Abs(float64(got.PPM-want.PPM)) > .1 {
		t.Fatalf("analog controls not confirmed: got %+v want %+v", got, want)
	}
	if got.DigitalAGC != want.DigitalAGC || got.OffsetTuning != want.OffsetTuning || got.IQSwap != want.IQSwap {
		t.Fatalf("RTL settings not confirmed: got %+v want %+v", got, want)
	}
}
