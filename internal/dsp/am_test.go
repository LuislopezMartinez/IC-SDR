package dsp

import (
	"math"
	"testing"
)

func TestAMDemodulatorRecoversTone(t *testing.T) {
	const inputRate, outputRate, carrier, tone = 192000.0, 48000.0, 12000.0, 1000.0
	iq := make([]float32, int(inputRate/2)*2)
	for sample := 0; sample < len(iq)/2; sample++ {
		time := float64(sample) / inputRate
		envelope := 1 + .5*math.Sin(2*math.Pi*tone*time)
		iq[sample*2] = float32(envelope * math.Cos(2*math.Pi*carrier*time))
		iq[sample*2+1] = float32(envelope * math.Sin(2*math.Pi*carrier*time))
	}
	demod := NewAMDemodulator(inputRate, outputRate)
	audio := demod.Process(iq, carrier, 9000)
	if len(audio) < 23000 || len(audio) > 25000 {
		t.Fatalf("unexpected output length: %d", len(audio))
	}
	var energy float64
	for _, sample := range audio[len(audio)/2:] {
		energy += float64(sample * sample)
	}
	if energy/float64(len(audio)/2) < 1e-4 {
		t.Fatal("AM tone was not recovered")
	}
}

func TestAMDemodulatorRaisesWeakModulationToListeningLevel(t *testing.T) {
	const inputRate, outputRate, carrier, tone = 192000.0, 48000.0, 7000.0, 900.0
	// A weak carrier with only 20% modulation used to remain conspicuously
	// quieter than the other analogue demodulators during the first seconds.
	iq := make([]float32, int(inputRate)*2)
	for sample := 0; sample < len(iq)/2; sample++ {
		timeSeconds := float64(sample) / inputRate
		envelope := .025 * (1 + .20*math.Sin(2*math.Pi*tone*timeSeconds))
		iq[sample*2] = float32(envelope * math.Cos(2*math.Pi*carrier*timeSeconds))
		iq[sample*2+1] = float32(envelope * math.Sin(2*math.Pi*carrier*timeSeconds))
	}

	audio := NewAMDemodulator(inputRate, outputRate).Process(iq, carrier, 9000)
	start := len(audio) * 3 / 4
	var power float64
	for _, sample := range audio[start:] {
		power += float64(sample * sample)
	}
	rms := math.Sqrt(power / float64(len(audio)-start))
	if rms < .07 {
		t.Fatalf("weak AM remains too quiet: RMS %.4f", rms)
	}
	if rms > .65 {
		t.Fatalf("weak AM was over-amplified: RMS %.4f", rms)
	}
}
