package sdr

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBundledTETRAPOLRuntimeStartsAndStops(t *testing.T) {
	dll := filepath.Join("..", "..", "DATA", "tools", "tetrapol", "native", "bin", "lilipol_native_x64.dll")
	receiver := NewReceiver(Config{
		SampleRate:        2_048_000,
		FFTSize:           4096,
		TETRAPOLNativeDLL: dll,
	})
	if err := receiver.StartTETRAPOLRuntime(); err != nil {
		t.Fatal(err)
	}
	if !receiver.TETRAPOLRuntimeStatus().Running {
		t.Fatal("TETRAPOL runtime did not report running")
	}
	receiver.tetrapol.ProcessIQ(make([]float32, 8_192*2))
	time.Sleep(150 * time.Millisecond)
	telemetry := receiver.TETRAPOLRuntimeStatus().Telemetry
	if got := telemetry.LogicalChannel; got != "TCH" {
		t.Fatalf("logical channel = %q, want TCH", got)
	}
	if telemetry.NativeRate != 32_000 || telemetry.NativeSPS != 4 {
		t.Fatalf("native frontend = %.0f S/s and %.2f samples/symbol, want 32000 and 4; telemetry=%+v", telemetry.NativeRate, telemetry.NativeSPS, telemetry)
	}
	if telemetry.IQSamples != 128 {
		t.Fatalf("native IQ samples = %d, want 128 direct channel samples", telemetry.IQSamples)
	}
	if err := receiver.StopTETRAPOLRuntime(); err != nil {
		t.Fatal(err)
	}
	if receiver.TETRAPOLRuntimeStatus().Running {
		t.Fatal("TETRAPOL runtime remained active after stop")
	}
}

func TestBundledTETRAPOLRuntimeAcceptsEveryRadioProfile(t *testing.T) {
	dll := filepath.Join("..", "..", "DATA", "tools", "tetrapol", "native", "bin", "lilipol_native_x64.dll")
	for _, test := range []struct{ channel, band, direction string }{
		{"CCH", "UHF", "DOWN"}, {"TCH", "UHF", "DOWN"},
		{"CCH", "VHF", "UP"}, {"TCH", "VHF", "UP"},
	} {
		t.Run(test.channel+"_"+test.band+"_"+test.direction, func(t *testing.T) {
			receiver := NewReceiver(Config{SampleRate: 2_048_000, FFTSize: 4096, TETRAPOLNativeDLL: dll})
			receiver.SetTETRAPOLFollowContext("001-002", 17)
			if err := receiver.StartTETRAPOLRuntimeConfigured(test.channel, test.band, test.direction); err != nil {
				t.Fatal(err)
			}
			time.Sleep(150 * time.Millisecond)
			if got := receiver.TETRAPOLRuntimeStatus().Telemetry.LogicalChannel; got != test.channel {
				t.Fatalf("logical channel = %q, want %s", got, test.channel)
			}
			if err := receiver.StopTETRAPOLRuntime(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
