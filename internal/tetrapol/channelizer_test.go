package tetrapol

import (
	"math"
	"testing"
)

func channelizerToneLevel(offsetHz, toneHz float64) float64 {
	const rate = 2_048_000.0
	channel := newChannelizer(rate)
	channel.setOffset(offsetHz)
	iq := make([]float32, 131_072*2)
	for sample := 0; sample < len(iq)/2; sample++ {
		phase := 2 * math.Pi * toneHz * float64(sample) / rate
		iq[2*sample], iq[2*sample+1] = float32(math.Cos(phase)), float32(math.Sin(phase))
	}
	output := channel.process(iq)
	output = output[min(40, len(output)):]
	var power float64
	for _, value := range output {
		power += real(value)*real(value) + imag(value)*imag(value)
	}
	return math.Sqrt(power / float64(len(output)))
}

func TestChannelizerSelectsTETRAPOLChannelAndRejectsAdjacentEnergy(t *testing.T) {
	const selected = 100_000.0
	desired := channelizerToneLevel(selected, selected+1_000)
	adjacent := channelizerToneLevel(selected, selected+30_000)
	if desired < .9 {
		t.Fatalf("selected channel level = %.3f", desired)
	}
	if attenuationDB := 20 * math.Log10(adjacent/desired); attenuationDB > -35 {
		t.Fatalf("adjacent rejection = %.1f dB, want <= -35 dB", attenuationDB)
	}
}
