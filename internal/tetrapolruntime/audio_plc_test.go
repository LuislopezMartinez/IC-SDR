package tetrapolruntime

import (
	"testing"
	"time"
)

func TestAudioPLCConcealsShortGapAndCrossfadesReturn(t *testing.T) {
	var plc audioPLC
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	first := make([]float32, 160)
	second := make([]float32, 160)
	for index := range first {
		first[index], second[index] = .8, -.8
	}
	if got := plc.process(first, now); len(got) != 160 {
		t.Fatalf("first output samples = %d", len(got))
	}
	got := plc.process(second, now.Add(60*time.Millisecond))
	if len(got) != 480 {
		t.Fatalf("concealed output samples = %d, want 480", len(got))
	}
	decoded, concealed, longGaps := plc.stats()
	if decoded != 2 || concealed != 2 || longGaps != 0 {
		t.Fatalf("PLC stats = %d/%d/%d", decoded, concealed, longGaps)
	}
	if got[320] <= -.8 || got[320] >= .8 {
		t.Fatalf("return was not crossfaded: first recovered sample %.3f", got[320])
	}
}

func TestAudioPLCLimitsLongGap(t *testing.T) {
	var plc audioPLC
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	frame := make([]float32, 160)
	plc.process(frame, now)
	got := plc.process(frame, now.Add(time.Second))
	if len(got) != 160*(tetrapolMaxConcealments+1) {
		t.Fatalf("long-gap samples = %d", len(got))
	}
	_, concealed, longGaps := plc.stats()
	if concealed != tetrapolMaxConcealments || longGaps != 1 {
		t.Fatalf("long-gap stats concealed=%d gaps=%d", concealed, longGaps)
	}
}
