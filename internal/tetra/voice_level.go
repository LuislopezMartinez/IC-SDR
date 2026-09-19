package tetra

import "math"

// voiceLeveler compensates the deliberately conservative output level of the
// TETRA ACELP codec. It works after voice decoding, so it cannot affect burst
// synchronization or channel decoding.
type voiceLeveler struct {
	gain float32
}

func (l *voiceLeveler) reset() { l.gain = 2.4 }

func (l *voiceLeveler) convert(pcm8 []int16) []float32 {
	if l.gain == 0 {
		l.reset()
	}
	var power float64
	for _, sample := range pcm8 {
		value := float64(sample) / 32768
		power += value * value
	}
	rms := 0.0
	if len(pcm8) > 0 {
		rms = math.Sqrt(power / float64(len(pcm8)))
	}
	// Do not chase codec background noise during pauses. For speech, aim at a
	// healthy -15 dBFS RMS and keep enough headroom for consonant peaks.
	desired := l.gain
	if rms >= .003 {
		desired = float32(min(max(.18/rms, 1), 4))
	}
	if desired < l.gain {
		l.gain += .65 * (desired - l.gain)
	} else {
		l.gain += .12 * (desired - l.gain)
	}

	output := make([]float32, len(pcm8)*6)
	for i, sample := range pcm8 {
		value := float32(sample) / 32768 * l.gain
		// Smooth limiting protects malformed or unusually loud codec frames.
		value = float32(math.Tanh(float64(value/.92))) * .92
		for n := 0; n < 6; n++ {
			output[i*6+n] = value
		}
	}
	return output
}
