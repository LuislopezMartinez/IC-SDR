package sdr

import (
	"math"
	"testing"
)

func TestComplexResamplerConvertsAirspyRate(t *testing.T) {
	const inputRate = 2_500_000
	const outputRate = 2_048_000
	input := make([]float32, inputRate/100*2)
	for sample := 0; sample < len(input)/2; sample++ {
		angle := 2 * math.Pi * 100_000 * float64(sample) / inputRate
		input[sample*2] = float32(math.Cos(angle))
		input[sample*2+1] = float32(math.Sin(angle))
	}
	r := newComplexResampler(inputRate, outputRate)
	r.push(input)
	output := make([]float32, outputRate/100*2)
	produced := r.pull(output)
	if produced < len(output)/2-64 || produced > len(output)/2 {
		t.Fatalf("produced %d samples, want approximately %d", produced, len(output)/2)
	}
	var power float64
	for _, value := range output[128 : produced*2] {
		power += float64(value * value)
	}
	power /= float64(produced*2 - 128)
	if math.Abs(power-.5) > .02 {
		t.Fatalf("unexpected component power %.4f", power)
	}
}

func TestComplexResamplerRejectsOutOfBandEnergy(t *testing.T) {
	const inputRate = 2_500_000
	const outputRate = 2_048_000
	r := newComplexResampler(inputRate, outputRate)
	input := make([]float32, 20_000)
	for sample := 0; sample < len(input)/2; sample++ {
		angle := 2 * math.Pi * 1_150_000 * float64(sample) / inputRate
		input[sample*2] = float32(math.Cos(angle))
		input[sample*2+1] = float32(math.Sin(angle))
	}
	r.push(input)
	output := make([]float32, 20_000)
	produced := r.pull(output)
	var power float64
	for _, value := range output[128 : produced*2] {
		power += float64(value * value)
	}
	power /= float64(produced*2 - 128)
	if power > .01 {
		t.Fatalf("out-of-band component power %.4f is too high", power)
	}
}
