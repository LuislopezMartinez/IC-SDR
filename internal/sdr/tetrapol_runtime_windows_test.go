package sdr

import (
	"path/filepath"
	"testing"
)

func TestBundledTETRAPOLRuntimeStartsAndStops(t *testing.T) {
	bin := filepath.Join("..", "..", "DATA", "tools", "tetrapol", "runtime", "bin")
	receiver := NewReceiver(Config{
		SampleRate:               2_048_000,
		FFTSize:                  4096,
		TETRAPOLKitExecutable:    filepath.Join(bin, "tetrapol_dump.exe"),
		TETRAPOLRPCELPExecutable: filepath.Join(bin, "rpcelp.exe"),
	})
	if err := receiver.StartTETRAPOLRuntime(); err != nil {
		t.Fatal(err)
	}
	if !receiver.TETRAPOLRuntimeStatus().Running {
		t.Fatal("TETRAPOL runtime did not report running")
	}
	if err := receiver.StopTETRAPOLRuntime(); err != nil {
		t.Fatal(err)
	}
	if receiver.TETRAPOLRuntimeStatus().Running {
		t.Fatal("TETRAPOL runtime remained active after stop")
	}
}
