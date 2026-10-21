package sdr

import "math"

const (
	tetrapolAudioInterpolation = 6
	tetrapolAudioPhaseTaps     = 64
	tetrapolAudioInputRate     = 8_000
	tetrapolAudioOutputRate    = tetrapolAudioInputRate * tetrapolAudioInterpolation
)

// tetrapolAudioUpsampler is a streaming polyphase low-pass interpolator. The
// native RP-CELP decoder emits exact 20 ms blocks at 8 kHz; each input sample
// therefore produces exactly six output samples without depending on block
// boundaries.
type tetrapolAudioUpsampler struct {
	history  [tetrapolAudioPhaseTaps]float32
	position int
}

var tetrapolAudioCoefficients = buildTETRAPOLAudioCoefficients()

func buildTETRAPOLAudioCoefficients() [tetrapolAudioInterpolation][tetrapolAudioPhaseTaps]float64 {
	var coefficients [tetrapolAudioInterpolation][tetrapolAudioPhaseTaps]float64
	const totalTaps = tetrapolAudioInterpolation * tetrapolAudioPhaseTaps
	center := float64(totalTaps-1) / 2
	cutoff := float64(tetrapolAudioInputRate) / 2 / tetrapolAudioOutputRate
	for phase := 0; phase < tetrapolAudioInterpolation; phase++ {
		sum := 0.0
		for tap := 0; tap < tetrapolAudioPhaseTaps; tap++ {
			index := phase + tap*tetrapolAudioInterpolation
			x := float64(index) - center
			ideal := 2 * cutoff
			if x != 0 {
				ideal = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
			}
			// Blackman suppresses the first 8 kHz image without introducing
			// ringing around the sharp consonants produced by RP-CELP.
			window := .42 - .5*math.Cos(2*math.Pi*float64(index)/float64(totalTaps-1)) + .08*math.Cos(4*math.Pi*float64(index)/float64(totalTaps-1))
			coefficients[phase][tap] = ideal * window
			sum += coefficients[phase][tap]
		}
		// Every phase must have unity DC gain. This also avoids a small
		// six-sample-period level ripple caused by finite precision.
		for tap := range coefficients[phase] {
			coefficients[phase][tap] /= sum
		}
	}
	return coefficients
}

func (upsampler *tetrapolAudioUpsampler) reset() {
	clear(upsampler.history[:])
	upsampler.position = 0
}

func (upsampler *tetrapolAudioUpsampler) process(input []float32) []float32 {
	if len(input) == 0 {
		return nil
	}
	output := make([]float32, len(input)*tetrapolAudioInterpolation)
	outputIndex := 0
	for _, sample := range input {
		upsampler.position--
		if upsampler.position < 0 {
			upsampler.position = len(upsampler.history) - 1
		}
		upsampler.history[upsampler.position] = sample
		for phase := 0; phase < tetrapolAudioInterpolation; phase++ {
			sum := 0.0
			historyIndex := upsampler.position
			for _, coefficient := range tetrapolAudioCoefficients[phase] {
				sum += coefficient * float64(upsampler.history[historyIndex])
				historyIndex++
				if historyIndex == len(upsampler.history) {
					historyIndex = 0
				}
			}
			output[outputIndex] = float32(sum)
			outputIndex++
		}
	}
	return output
}
