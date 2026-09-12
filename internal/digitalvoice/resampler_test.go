package digitalvoice

import "testing"

func TestVoiceResamplerPreservesContinuityAcrossBlocks(t *testing.T) {
	var r linearResampler
	r.reset(8_000, 48_000)
	first := r.process([]float32{0, 1})
	second := r.process([]float32{2})
	if len(first) != 6 || len(second) != 6 {
		t.Fatalf("unexpected lengths %d, %d", len(first), len(second))
	}
	if first[0] != 0 || second[0] != 1 {
		t.Fatalf("boundary is discontinuous: first=%v second=%v", first, second)
	}
	for i := 1; i < len(second); i++ {
		if second[i] <= second[i-1] {
			t.Fatalf("non-monotonic interpolation: %v", second)
		}
	}
}
