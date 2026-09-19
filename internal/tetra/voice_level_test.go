package tetra

import (
	"math"
	"testing"
)

func TestVoiceLevelerRaisesQuietTETRASpeechWithoutClipping(t *testing.T) {
	pcm := make([]int16, 480)
	for i := range pcm {
		pcm[i] = int16(2200 * math.Sin(2*math.Pi*float64(i)/40))
	}
	var leveler voiceLeveler
	output := leveler.convert(pcm)
	if len(output) != len(pcm)*6 {
		t.Fatalf("output length = %d, want %d", len(output), len(pcm)*6)
	}
	peak := float32(0)
	for _, sample := range output {
		peak = max(peak, float32(math.Abs(float64(sample))))
		if sample < -.92 || sample > .92 {
			t.Fatalf("unbounded output sample: %v", sample)
		}
	}
	inputPeak := float32(2200.0 / 32768.0)
	if peak < inputPeak*2 {
		t.Fatalf("quiet TETRA audio was not raised enough: input %.3f output %.3f", inputPeak, peak)
	}
}

func TestVoiceLevelerDoesNotAmplifySilence(t *testing.T) {
	var leveler voiceLeveler
	output := leveler.convert(make([]int16, 480))
	for _, sample := range output {
		if sample != 0 {
			t.Fatalf("silence became non-zero: %v", sample)
		}
	}
}
