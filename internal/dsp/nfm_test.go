package dsp

import (
	"math"
	"testing"
)

func TestNFMDemodulatorRecoversTone(t *testing.T) {
	const inputRate, outputRate = 192000.0, 48000.0
	const carrier, tone, deviation = 12000.0, 1000.0, 2500.0
	iq := make([]float32, int(inputRate/2)*2)
	for sample := 0; sample < len(iq)/2; sample++ {
		time := float64(sample) / inputRate
		phase := 2*math.Pi*carrier*time - deviation/tone*math.Cos(2*math.Pi*tone*time)
		iq[sample*2], iq[sample*2+1] = float32(math.Cos(phase)), float32(math.Sin(phase))
	}
	demod := NewNFMDemodulator(inputRate, outputRate)
	audio := demod.Process(iq, carrier, 12500, 50)
	if len(audio) < 23000 || len(audio) > 25000 {
		t.Fatalf("unexpected output length: %d", len(audio))
	}
	start := len(audio) / 2
	var sine, cosine float64
	for index, sample := range audio[start:] {
		phase := 2 * math.Pi * tone * float64(index) / outputRate
		sine += float64(sample) * math.Sin(phase)
		cosine += float64(sample) * math.Cos(phase)
	}
	amplitude := 2 * math.Hypot(sine, cosine) / float64(len(audio)-start)
	if amplitude < .02 {
		t.Fatalf("NFM tone was not recovered: amplitude %.4f", amplitude)
	}
}

func TestWFMDemodulatorRecoversBroadcastTone(t *testing.T) {
	const inputRate = 2_048_000
	const outputRate = 48_000
	const toneHz = 1_000
	const deviationHz = 75_000
	seconds := .12
	iq := make([]float32, int(inputRate*seconds)*2)
	phase := 0.
	for sample := 0; sample < len(iq)/2; sample++ {
		instantaneous := deviationHz * math.Sin(2*math.Pi*toneHz*float64(sample)/inputRate)
		phase += 2 * math.Pi * instantaneous / inputRate
		iq[sample*2] = float32(math.Cos(phase))
		iq[sample*2+1] = float32(math.Sin(phase))
	}
	demod := NewNFMDemodulator(inputRate, outputRate)
	audio := demod.ProcessWide(iq, 0, 200_000, 50)
	if len(audio) < int(outputRate*seconds*.9) {
		t.Fatalf("WFM produced too little audio: %d samples", len(audio))
	}
	var power float64
	for _, sample := range audio[len(audio)/3:] {
		power += float64(sample * sample)
	}
	rms := math.Sqrt(power / float64(len(audio)-len(audio)/3))
	if rms < .03 {
		t.Fatalf("WFM tone was not recovered: RMS %.4f", rms)
	}
}

func TestWFMAntiAliasRejectsRDSSubcarrier(t *testing.T) {
	const inputRate, outputRate = 2_048_000., 48_000.
	const seconds = .15
	makeSignal := func(toneHz float64) []float32 {
		iq := make([]float32, int(inputRate*seconds)*2)
		phase := 0.
		for sample := 0; sample < len(iq)/2; sample++ {
			instantaneous := 8_000. * math.Sin(2*math.Pi*toneHz*float64(sample)/inputRate)
			phase += 2 * math.Pi * instantaneous / inputRate
			iq[sample*2], iq[sample*2+1] = float32(math.Cos(phase)), float32(math.Sin(phase))
		}
		return iq
	}
	measure := func(toneHz float64) float64 {
		demod := NewNFMDemodulator(inputRate, outputRate)
		audio := demod.ProcessWide(makeSignal(toneHz), 0, 200_000, 50)
		start := len(audio) / 2
		var power float64
		for _, sample := range audio[start:] {
			power += float64(sample * sample)
		}
		return math.Sqrt(power / float64(len(audio)-start))
	}
	wanted, rds := measure(1_000), measure(57_000)
	if wanted < .03 || rds > wanted*.08 {
		t.Fatalf("WFM anti-alias insufficient: 1 kHz RMS %.5f, 57 kHz RMS %.5f", wanted, rds)
	}
}
