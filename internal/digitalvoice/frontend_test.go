package digitalvoice

import (
	"math"
	"testing"
)

func TestFrontendPreservesFourFSKOrderingAndRate(t *testing.T) {
	const inputRate = 2_048_000
	levels := []float64{-1800, -600, 600, 1800}
	iq := make([]float32, 0, inputRate/50*2)
	phase := 0.0
	for n := 0; n < inputRate/50; n++ {
		frequency := levels[(n/(inputRate/4800))%len(levels)]
		phase += 2 * math.Pi * frequency / inputRate
		iq = append(iq, float32(math.Cos(phase)), float32(math.Sin(phase)))
	}
	f := newFrontend(inputRate, 0, 15_000)
	pcm, level := f.process(iq)
	if len(pcm) < 850 || len(pcm) > 1000 {
		t.Fatalf("unexpected 48 kHz output count: %d", len(pcm))
	}
	if level < -30 || level > -1 {
		t.Fatalf("unexpected discriminator level: %.1f dBFS", level)
	}
	minimum, maximum := int16(32767), int16(-32768)
	for _, sample := range pcm {
		if sample < minimum {
			minimum = sample
		}
		if sample > maximum {
			maximum = sample
		}
	}
	if minimum >= -3000 || maximum <= 3000 {
		t.Fatalf("FSK deviation collapsed: min=%d max=%d", minimum, maximum)
	}
}
