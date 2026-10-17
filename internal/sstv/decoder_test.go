package sstv

import (
	"encoding/binary"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestFrameCarriesReceivedLineQuality(t *testing.T) {
	d := New("missing", t.TempDir())
	payload := make([]byte, 16+2*3*3)
	binary.LittleEndian.PutUint32(payload[4:8], 2)
	binary.LittleEndian.PutUint32(payload[8:12], 3)
	binary.LittleEndian.PutUint32(payload[12:16], 2)
	d.applyFullLocked(0, 1, payload)
	if got := d.frames[0].LineQuality; len(got) != 3 || got[0] != 1 || got[1] != 1 || got[2] != 0 {
		t.Fatalf("line quality = %v", got)
	}
}

func TestHorizontalCorrectionPreservesOriginalAndAlignsShiftedRow(t *testing.T) {
	const width = 48
	frame := Frame{Width: width, Height: 2, Lines: 2, RGB: make([]byte, width*2*3), LineQuality: []uint8{1, 1}}
	for x := 0; x < width; x++ {
		value := byte(0)
		if (x/4)%2 == 0 {
			value = 255
		}
		for channel := 0; channel < 3; channel++ {
			frame.RGB[x*3+channel] = value
			sourceX := max(x-3, 0) // Second row is displaced three pixels right.
			frame.RGB[(width+x)*3+channel] = frame.RGB[sourceX*3+channel]
		}
	}
	original := append([]byte(nil), frame.RGB...)
	enhanced, corrected := alignSSTVRows(frame)
	if corrected != 1 || enhanced.LineQuality[1] != 2 {
		t.Fatalf("corrected lines = %d quality=%v", corrected, enhanced.LineQuality)
	}
	if string(frame.RGB) != string(original) {
		t.Fatal("original frame was modified")
	}
	for x := 8; x < width-8; x++ {
		if enhanced.RGB[(width+x)*3] != enhanced.RGB[x*3] {
			t.Fatalf("row remains displaced at x=%d", x)
		}
	}
}

func TestLineReconstructionInterpolatesOnlyShortBoundedGaps(t *testing.T) {
	frame := Frame{Width: 1, Height: 5, Lines: 5, RGB: make([]byte, 15), LineQuality: []uint8{1, 0, 0, 1, 0}}
	frame.RGB[0], frame.RGB[1], frame.RGB[2] = 0, 30, 60
	frame.RGB[9], frame.RGB[10], frame.RGB[11] = 90, 120, 150
	enhanced, count := reconstructSSTVLines(frame, 2)
	if count != 2 || enhanced.LineQuality[1] != 3 || enhanced.LineQuality[2] != 3 || enhanced.LineQuality[4] != 0 {
		t.Fatalf("reconstruction quality=%v count=%d", enhanced.LineQuality, count)
	}
	if enhanced.RGB[3] != 30 || enhanced.RGB[6] != 60 {
		t.Fatalf("interpolated red values = %d, %d", enhanced.RGB[3], enhanced.RGB[6])
	}
	if frame.LineQuality[1] != 0 || frame.RGB[3] != 0 {
		t.Fatal("original frame was modified")
	}
}

func TestLineReconstructionSwitchControlsEnhancedOutput(t *testing.T) {
	d := New("missing", t.TempDir())
	frame := Frame{Width: 8, Height: 3, Lines: 3, RGB: make([]byte, 72), LineQuality: []uint8{1, 0, 1}}
	for i := 48; i < 72; i++ {
		frame.RGB[i] = 180
	}
	original, err := d.saveFramePNG(frame, "switch-test")
	if err != nil {
		t.Fatal(err)
	}
	d.SetReconstructLines(false)
	if path, _, reconstructed, _, err := d.saveEnhancedFramePNG(frame, original); err != nil || path != "" || reconstructed != 0 {
		t.Fatalf("disabled reconstruction produced %q, %d, %v", path, reconstructed, err)
	}
	d.SetReconstructLines(true)
	if path, _, reconstructed, _, err := d.saveEnhancedFramePNG(frame, original); err != nil || path == "" || reconstructed != 1 {
		t.Fatalf("enabled reconstruction produced %q, %d, %v", path, reconstructed, err)
	}
}

func TestImpulseNoiseReductionIsSelectiveAndNonDestructive(t *testing.T) {
	frame := Frame{Width: 5, Height: 5, Lines: 5, RGB: make([]byte, 75), LineQuality: []uint8{1, 1, 1, 1, 1}}
	for index := range frame.RGB {
		frame.RGB[index] = 100
	}
	centre := (2*frame.Width + 2) * 3
	frame.RGB[centre], frame.RGB[centre+1], frame.RGB[centre+2] = 255, 255, 255
	original := append([]byte(nil), frame.RGB...)
	enhanced, changed := reduceSSTVImpulseNoise(frame)
	if changed != 1 || enhanced.RGB[centre] != 100 || enhanced.RGB[centre+1] != 100 || enhanced.RGB[centre+2] != 100 {
		t.Fatalf("impulse reduction changed=%d pixel=%v", changed, enhanced.RGB[centre:centre+3])
	}
	if string(frame.RGB) != string(original) {
		t.Fatal("noise reduction modified original frame")
	}
}

func TestNoiseReductionSwitchControlsEnhancedOutput(t *testing.T) {
	d := New("missing", t.TempDir())
	d.SetReconstructLines(false)
	frame := Frame{Width: 5, Height: 5, Lines: 5, RGB: make([]byte, 75), LineQuality: []uint8{1, 1, 1, 1, 1}}
	for index := range frame.RGB {
		frame.RGB[index] = 80
	}
	centre := (2*frame.Width + 2) * 3
	frame.RGB[centre], frame.RGB[centre+1], frame.RGB[centre+2] = 255, 255, 255
	original, err := d.saveFramePNG(frame, "noise-switch")
	if err != nil {
		t.Fatal(err)
	}
	d.SetReduceNoise(false)
	if path, _, _, denoised, err := d.saveEnhancedFramePNG(frame, original); err != nil || path != "" || denoised != 0 {
		t.Fatalf("disabled noise reduction produced %q, %d, %v", path, denoised, err)
	}
	d.SetReduceNoise(true)
	if path, _, _, denoised, err := d.saveEnhancedFramePNG(frame, original); err != nil || path == "" || denoised != 1 {
		t.Fatalf("enabled noise reduction produced %q, %d, %v", path, denoised, err)
	}
}

func TestSSTVQualityLabelsAreUnderstandable(t *testing.T) {
	for _, test := range []struct {
		lines, height, corrected, reconstructed int
		want                                    string
	}{
		{240, 240, 2, 0, "BUENA"},
		{200, 240, 20, 4, "ACEPTABLE"},
		{120, 240, 0, 0, "DEGRADADA"},
		{40, 240, 0, 0, "MUY INCOMPLETA"},
	} {
		if got := sstvQualityLabel(Frame{Lines: test.lines, Height: test.height}, test.corrected, test.reconstructed); got != test.want {
			t.Errorf("quality(%d/%d) = %q, want %q", test.lines, test.height, got, test.want)
		}
	}
}

func TestNewVISArchivesPartialAndStartsWithCleanFrames(t *testing.T) {
	d := New("missing", t.TempDir())
	d.generation = 7
	d.frames[0] = Frame{
		Sequence: 12,
		Width:    2,
		Height:   2,
		Lines:    1,
		RGB: []byte{
			255, 0, 0, 0, 255, 0,
			0, 0, 0, 0, 0, 0,
		},
	}
	d.status.Width, d.status.Height, d.status.Lines, d.status.Progress = 2, 2, 1, 50

	d.parseStatus(7, "SSTV STATE Leader lock")
	d.parseStatus(7, "SSTV STATE VIS")
	d.parseStatus(7, "SSTV PRIMARY RESET VIS_LOCKED")
	if d.frames[0].Width == 0 || !d.visResetPending || d.status.State != "VALIDANDO NUEVO VIS" {
		t.Fatalf("VIS was committed before MODE confirmation: frame=%+v status=%+v", d.frames[0], d.status)
	}

	// Simulate a late row from the old stdout pipe. MODE must clear it again
	// before the first rows belonging to the newly detected image arrive.
	d.frames[0] = Frame{Sequence: 13, Width: 2, Height: 2, Lines: 1, RGB: make([]byte, 12)}
	d.status.Width, d.status.Height, d.status.Lines = 2, 2, 1
	d.parseStatus(7, "SSTV MODE Robot 36 Color 320 240")
	if d.frames[0].Width != 0 || d.visResetPending {
		t.Fatalf("MODE retained stale pixels: frame=%+v pending=%v", d.frames[0], d.visResetPending)
	}
	if d.status.Mode != "ROBOT 36 COLOR" || d.status.Progress != 0 {
		t.Fatalf("new VIS status = %+v", d.status)
	}
	if !strings.HasSuffix(d.status.LastSaved, ".png") || len(d.status.History) != 1 || d.status.History[0].Reason != "Interrumpida por nuevo VIS" {
		t.Fatalf("partial image was not archived as PNG: %+v", d.status)
	}
	file, err := os.Open(d.status.LastSaved)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = png.Decode(file); err != nil {
		_ = file.Close()
		t.Fatalf("archived partial is not a valid PNG: %v", err)
	}
	_ = file.Close()
}

func TestForceReceiveStartsSelectedModeWithoutVIS(t *testing.T) {
	d := New("missing", t.TempDir())
	d.selectedMode = "PD120"

	automatic := strings.Join(d.runtimeArgs(false), " ")
	if strings.Contains(automatic, "--force-start") || strings.Contains(automatic, "--forced-mode") {
		t.Fatalf("automatic arguments unexpectedly force a mode: %s", automatic)
	}

	forced := strings.Join(d.runtimeArgs(true), " ")
	if !strings.Contains(forced, "--force-start") || !strings.Contains(forced, "--forced-mode PD120") {
		t.Fatalf("forced reception arguments do not start the selected mode: %s", forced)
	}
}

func TestUnconfirmedVISDoesNotReplaceCurrentImage(t *testing.T) {
	d := New("missing", t.TempDir())
	d.generation = 4
	d.frames[0] = Frame{Sequence: 2, Width: 1, Height: 2, Lines: 1, RGB: make([]byte, 6)}
	d.parseStatus(4, "SSTV PRIMARY RESET VIS_LOCKED")
	if d.frames[0].Width == 0 || d.visResetPending || len(d.status.History) != 0 {
		t.Fatalf("unconfirmed VIS replaced active image: frame=%+v status=%+v", d.frames[0], d.status)
	}
}

func TestContinuousResetDoesNotArchiveACompletedFrameAgain(t *testing.T) {
	d := New("missing", t.TempDir())
	d.generation = 3
	d.frames[0] = Frame{Sequence: 1, Width: 1, Height: 1, Lines: 1, RGB: []byte{1, 2, 3}}
	d.status.LastSaved = "already-saved.bmp"
	d.parseStatus(3, "SSTV PRIMARY RESET CONTINUOUS")
	if d.status.LastSaved != "already-saved.bmp" || d.visResetPending {
		t.Fatalf("continuous reset unexpectedly archived frame: %+v", d.status)
	}
}

func TestWatchdogAndCompleteSavePartialPNGOnlyOnce(t *testing.T) {
	d := New("missing", t.TempDir())
	d.generation = 9
	d.frames[0] = Frame{Sequence: 21, Width: 2, Height: 3, Lines: 2, RGB: make([]byte, 18)}

	d.parseStatus(9, "SSTV WATCHDOG SAVE_PARTIAL")
	first := d.status.LastSaved
	if !strings.HasSuffix(first, ".png") || !strings.Contains(d.status.Detail, "2/3") || len(d.status.History) != 1 || !d.status.History[0].Partial || d.status.History[0].Reason != "Señal perdida / timeout" {
		t.Fatalf("watchdog did not save partial PNG: %+v", d.status)
	}
	d.parseStatus(9, "SSTV COMPLETE ignored-runtime-image.bmp")
	if d.status.LastSaved != first || len(d.status.History) != 1 {
		t.Fatalf("same frame was saved twice: %q then %q", first, d.status.LastSaved)
	}

	d.frames[0].Sequence++
	d.frames[0].Lines = 3
	d.parseStatus(9, "SSTV COMPLETE ignored-runtime-image.bmp")
	if d.status.LastSaved == first || !strings.Contains(d.status.Detail, "completa") || len(d.status.History) != 2 || d.status.History[0].Partial || d.status.History[0].Reason != "Completada" {
		t.Fatalf("completed frame was not archived: %+v", d.status)
	}
}

func TestNewVISDoesNotSaveWatchdogFrameTwice(t *testing.T) {
	d := New("missing", t.TempDir())
	d.generation = 11
	d.status.Mode = "ROBOT 36 COLOR"
	d.frames[0] = Frame{Sequence: 31, Width: 2, Height: 3, Lines: 2, RGB: make([]byte, 18)}

	d.parseStatus(11, "SSTV WATCHDOG SAVE_PARTIAL")
	first := d.status.LastSaved
	if len(d.status.History) != 1 {
		t.Fatalf("watchdog history = %d, want 1", len(d.status.History))
	}

	d.parseStatus(11, "SSTV STATE Leader lock")
	d.parseStatus(11, "SSTV STATE VIS")
	d.parseStatus(11, "SSTV PRIMARY RESET VIS_LOCKED")
	d.parseStatus(11, "SSTV MODE Martin 1 320 256")

	if d.status.LastSaved != first || len(d.status.History) != 1 {
		t.Fatalf("new VIS duplicated the watchdog PNG: last=%q history=%+v", d.status.LastSaved, d.status.History)
	}
	if d.frames[0].Width != 0 || d.status.Mode != "MARTIN 1" {
		t.Fatalf("new reception was not reset correctly: frame=%+v mode=%q", d.frames[0], d.status.Mode)
	}
}
