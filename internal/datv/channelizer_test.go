package datv

import (
	"math"
	"testing"
)

func TestChannelizerMovesSelectionAndDecimates(t *testing.T) {
	const inputRate = 2_048_000
	c := newChannelizer(inputRate, 125_000, 300_000)
	if c.outputRate != 256_000 || c.decimation != 8 {
		t.Fatalf("rate=%d decimation=%d", c.outputRate, c.decimation)
	}
	iq := complexTone(inputRate, 300_000, 32_768)
	out := c.process(iq)
	if len(out) != len(iq)/8 {
		t.Fatalf("output floats=%d want=%d", len(out), len(iq)/8)
	}
	// Once translated to DC, consecutive samples should have almost no phase
	// rotation (ignore the FIR warm-up).
	start := len(out) / 2
	var phase float64
	for i := start + 2; i+1 < len(out); i += 2 {
		a := complex(float64(out[i-2]), float64(out[i-1]))
		b := complex(float64(out[i]), float64(out[i+1]))
		phase += math.Abs(math.Atan2(imag(b*complex(real(a), -imag(a))), real(b*complex(real(a), -imag(a)))))
	}
	if phase/float64((len(out)-start)/2-1) > 1e-3 {
		t.Fatalf("tone was not centered: mean phase step=%g", phase/float64((len(out)-start)/2-1))
	}
}

func TestChannelizerPreservesStateAcrossBlocks(t *testing.T) {
	const inputRate = 2_048_000
	iq := complexTone(inputRate, -175_000, 20_003)
	one := newChannelizer(inputRate, 66_000, -175_000).process(iq)
	splitter := newChannelizer(inputRate, 66_000, -175_000)
	two := append(splitter.process(iq[:12346]), splitter.process(iq[12346:])...)
	if len(one) != len(two) {
		t.Fatalf("length differs: %d != %d", len(one), len(two))
	}
	for i := range one {
		if one[i] != two[i] {
			t.Fatalf("stream discontinuity at float %d: %g != %g", i, one[i], two[i])
		}
	}
}

func complexTone(rate, frequency, samples int) []float32 {
	iq := make([]float32, samples*2)
	for i := 0; i < samples; i++ {
		phase := 2 * math.Pi * float64(frequency*i) / float64(rate)
		iq[2*i], iq[2*i+1] = float32(math.Cos(phase)), float32(math.Sin(phase))
	}
	return iq
}
