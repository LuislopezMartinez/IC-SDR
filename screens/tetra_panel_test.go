package screens

import (
	"path/filepath"
	"testing"
	"time"

	"go-zero/internal/sdr"
)

func TestTETRAWavecomCaptureStaysOutsideUtilitiesSidebar(t *testing.T) {
	panel := NewTETRAPanel(&MainScreen{})
	bounds := panel.iqCapture.Bounds()
	if bounds.X < toolContentX {
		t.Fatalf("Wavecom IQ capture overlaps utilities sidebar: x=%v, content starts at %v", bounds.X, toolContentX)
	}
	if bounds.X+bounds.Width > toolContentRight {
		t.Fatalf("Wavecom IQ capture exceeds tool content: right=%v, limit=%v", bounds.X+bounds.Width, toolContentRight)
	}
	if bounds.Y < toolY || bounds.Y+bounds.Height > toolY+toolH {
		t.Fatalf("Wavecom IQ capture is outside TETRA panel: bounds=%+v", bounds)
	}
}

func TestTETRAManualTuneRestartsFreshDecoderAfterDebounce(t *testing.T) {
	receiver := sdr.NewReceiver(sdr.Config{FrequencyHz: 390_000_000, SampleRate: 2_048_000, FFTSize: 4096})
	defer receiver.Close()
	receiver.ConfigureTETRA(true)
	screen := &MainScreen{receiver: receiver, frequencyHz: 390_012_500, demodBandwidthHz: tetraChannelBandwidthHz}
	panel := &TETRAPanel{
		screen:       screen,
		enabled:      true,
		clearOnly:    true,
		snapshotPath: filepath.Join(t.TempDir(), "tetra-live.json"),
	}

	panel.resetAfterManualTune()
	if !panel.pendingManualTune || receiver.TETRAStatus().Running {
		t.Fatalf("manual tune did not pause decoder: pending=%v running=%v", panel.pendingManualTune, receiver.TETRAStatus().Running)
	}
	firstDeadline := panel.restartAfter
	time.Sleep(time.Millisecond)
	panel.resetAfterManualTune()
	if !panel.restartAfter.After(firstDeadline) || receiver.TETRAStatus().Running {
		t.Fatalf("repeated tune did not extend debounce: first=%v next=%v running=%v", firstDeadline, panel.restartAfter, receiver.TETRAStatus().Running)
	}

	panel.restartAfter = time.Now().Add(-time.Millisecond)
	panel.resumeAfterManualTune()
	if panel.pendingManualTune || !receiver.TETRAStatus().Running {
		t.Fatalf("decoder did not restart after tune settled: pending=%v running=%v", panel.pendingManualTune, receiver.TETRAStatus().Running)
	}
}
