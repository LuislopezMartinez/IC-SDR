package dsp

import (
	"math"
	"testing"
)

func TestSSBDemodulatorRecoversUSBAndLSB(t *testing.T) {
	for _, mode := range []string{"USB", "LSB"} {
		t.Run(mode, func(t *testing.T) {
			const inputRate, outputRate, offset, tone = 192000.0, 48000.0, 12000.0, 1000.0
			sign := 1.0
			if mode == "LSB" {
				sign = -1
			}
			iq := make([]float32, int(inputRate/2)*2)
			for sample := 0; sample < len(iq)/2; sample++ {
				phase := 2 * math.Pi * (offset + sign*tone) * float64(sample) / inputRate
				iq[sample*2], iq[sample*2+1] = float32(math.Cos(phase)), float32(math.Sin(phase))
			}
			demod := NewSSBDemodulator(inputRate, outputRate)
			audio := demod.Process(iq, mode, offset, 2400)
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
			if amplitude := 2 * math.Hypot(sine, cosine) / float64(len(audio)-start); amplitude < .40 {
				t.Fatalf("SSB listening level is too low: %.4f", amplitude)
			}
			for _, sample := range audio {
				if absFloat(sample) > ssbLimiterCeiling+.0001 {
					t.Fatalf("SSB output exceeded limiter ceiling: %.4f", sample)
				}
			}
		})
	}
}

func TestSSBDemodulatorAmplifiesWeakSidebandAudio(t *testing.T) {
	for _, mode := range []string{"USB", "LSB"} {
		t.Run(mode, func(t *testing.T) {
			const inputRate, outputRate, offset, tone = 192000.0, 48000.0, 12000.0, 1000.0
			const inputAmplitude = .003
			sign := 1.0
			if mode == "LSB" {
				sign = -1
			}
			iq := make([]float32, int(inputRate)*2)
			for sample := 0; sample < len(iq)/2; sample++ {
				phase := 2 * math.Pi * (offset + sign*tone) * float64(sample) / inputRate
				iq[sample*2] = float32(inputAmplitude * math.Cos(phase))
				iq[sample*2+1] = float32(inputAmplitude * math.Sin(phase))
			}

			audio := NewSSBDemodulator(inputRate, outputRate).Process(iq, mode, offset, 2400)
			start := len(audio) * 3 / 4
			var power float64
			for _, sample := range audio[start:] {
				power += float64(sample * sample)
			}
			rms := math.Sqrt(power / float64(len(audio)-start))
			if rms < .12 {
				t.Fatalf("weak %s audio was not amplified enough: RMS %.4f", mode, rms)
			}
		})
	}
}
