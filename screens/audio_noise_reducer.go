package screens

import (
	"math"
	"math/cmplx"
)

const (
	noiseFFTSize = 512
	noiseHopSize = noiseFFTSize / 2
)

// audioNoiseReducer is a streaming spectral Wiener filter tuned for received
// communications audio. It deliberately has no speech detector: weak SSB and
// AM signals remain valid input instead of being discarded as non-speech.
type audioNoiseReducer struct {
	input       [noiseFFTSize]float32
	inputWrite  int
	overlap     [noiseHopSize]float64
	output      [noiseFFTSize * 4]float32
	outputRead  int
	outputWrite int
	outputCount int
	window      [noiseFFTSize]float64
	noisePower  [noiseFFTSize/2 + 1]float64
	smoothPower [noiseFFTSize/2 + 1]float64
	minimum     [noiseFFTSize/2 + 1]float64
	previousMin [noiseFFTSize/2 + 1]float64
	gain        [noiseFFTSize/2 + 1]float64
	frames      int
	minimumAge  int
	activity    float32
}

func newAudioNoiseReducer() audioNoiseReducer {
	var reducer audioNoiseReducer
	for index := range reducer.window {
		// A square-root Hann window reconstructs at unity with 50% overlap.
		reducer.window[index] = math.Sqrt(.5 - .5*math.Cos(2*math.Pi*float64(index)/noiseFFTSize))
	}
	for index := range reducer.gain {
		reducer.gain[index] = 1
	}
	// Begin in the second half; the zeroed first half supplies causal history.
	reducer.inputWrite = noiseHopSize
	return reducer
}

func (reducer *audioNoiseReducer) process(samples []float32, enabled bool, strength float32) {
	amount := float64(min(max(strength, 0), 100)) / 100
	if !enabled {
		amount = 0
	}
	for index, sample := range samples {
		reducer.input[reducer.inputWrite] = sample
		reducer.inputWrite++
		if reducer.inputWrite == noiseFFTSize {
			reducer.processFrame(amount)
			copy(reducer.input[:noiseHopSize], reducer.input[noiseHopSize:])
			reducer.inputWrite = noiseHopSize
		}
		if reducer.outputCount == 0 {
			samples[index] = 0
			continue
		}
		samples[index] = reducer.output[reducer.outputRead]
		reducer.outputRead = (reducer.outputRead + 1) % len(reducer.output)
		reducer.outputCount--
	}
}

func (reducer *audioNoiseReducer) processFrame(amount float64) {
	var bins [noiseFFTSize]complex128
	var power [noiseFFTSize/2 + 1]float64
	var targetGain [noiseFFTSize/2 + 1]float64
	var protected [noiseFFTSize/2 + 1]bool
	for index, sample := range reducer.input {
		bins[index] = complex(float64(sample)*reducer.window[index], 0)
	}
	fftComplex(bins[:], false)
	for bin := 0; bin <= noiseFFTSize/2; bin++ {
		realPart, imaginaryPart := real(bins[bin]), imag(bins[bin])
		power[bin] = realPart*realPart + imaginaryPart*imaginaryPart
	}
	overSubtract := 1 + 1.35*amount
	minimumGain := .80 - .75*amount
	for bin := 0; bin <= noiseFFTSize/2; bin++ {
		currentPower := power[bin]
		neighbourPower, neighbours := 0.0, 0
		// Compare against bins outside the main-lobe width of the Hann
		// window. Immediate neighbours contain the same wanted carrier.
		for offset := -6; offset <= 6; offset++ {
			candidate := bin + offset
			if (offset <= -3 || offset >= 3) && candidate >= 0 && candidate < len(power) {
				neighbourPower += power[candidate]
				neighbours++
			}
		}
		if neighbours > 0 {
			neighbourPower /= float64(neighbours)
		}
		spectralPeak := currentPower > 6*neighbourPower
		signalPresent := spectralPeak
		if reducer.frames > 0 {
			signalPresent = signalPresent || currentPower > 6*reducer.noisePower[bin]
		}
		protected[bin] = spectralPeak
		if reducer.frames == 0 {
			reducer.noisePower[bin] = currentPower
			reducer.smoothPower[bin] = currentPower
			reducer.minimum[bin] = currentPower
			reducer.previousMin[bin] = currentPower
		} else {
			reducer.smoothPower[bin] += .20 * (currentPower - reducer.smoothPower[bin])
			reducer.minimum[bin] = min(reducer.minimum[bin], reducer.smoothPower[bin])
			// A half-second minimum is biased below the average periodogram.
			// Compensation keeps ordinary receiver hiss close to its true floor.
			candidate := 2.0 * min(reducer.minimum[bin], reducer.previousMin[bin])
			rate := .10
			if candidate > reducer.noisePower[bin] {
				rate = .006
			}
			if signalPresent && candidate > reducer.noisePower[bin] {
				rate = 0
			}
			reducer.noisePower[bin] += rate * (candidate - reducer.noisePower[bin])
		}
		ratio := 1.0
		if currentPower > 1e-16 {
			ratio = math.Sqrt(max(0, 1-overSubtract*reducer.noisePower[bin]/currentPower))
		}
		targetGain[bin] = max(minimumGain, ratio)
		if signalPresent {
			targetGain[bin] = 1
		}
	}
	for bin, preserve := range protected {
		if !preserve {
			continue
		}
		for offset := -3; offset <= 3; offset++ {
			candidate := bin + offset
			if candidate >= 0 && candidate < len(targetGain) {
				targetGain[candidate] = 1
			}
		}
	}
	attenuation := 0.0
	wet := min(1.0, amount*2.0)
	for bin := 0; bin <= noiseFFTSize/2; bin++ {
		target := .5 * targetGain[bin]
		weight := .5
		if bin > 0 {
			target += .25 * targetGain[bin-1]
			weight += .25
		}
		if bin < noiseFFTSize/2 {
			target += .25 * targetGain[bin+1]
			weight += .25
		}
		target /= weight
		rate := .18
		if target > reducer.gain[bin] {
			rate = .35
		}
		reducer.gain[bin] += rate * (target - reducer.gain[bin])
		applied := 1 - wet*(1-reducer.gain[bin])
		bins[bin] *= complex(applied, 0)
		if bin > 0 && bin < noiseFFTSize/2 {
			bins[noiseFFTSize-bin] *= complex(applied, 0)
		}
		attenuation += 1 - applied
	}
	reducer.frames++
	reducer.minimumAge++
	if reducer.minimumAge >= 94 {
		reducer.previousMin = reducer.minimum
		reducer.minimum = reducer.smoothPower
		reducer.minimumAge = 0
	}
	fftComplex(bins[:], true)
	for index := 0; index < noiseHopSize; index++ {
		first := real(bins[index])*reducer.window[index] + reducer.overlap[index]
		reducer.enqueue(float32(first))
		reducer.overlap[index] = real(bins[index+noiseHopSize]) * reducer.window[index+noiseHopSize]
	}
	instant := float32(attenuation / float64(noiseFFTSize/2+1))
	reducer.activity += .18 * (instant - reducer.activity)
}

func (reducer *audioNoiseReducer) enqueue(sample float32) {
	if reducer.outputCount == len(reducer.output) {
		reducer.outputRead = (reducer.outputRead + 1) % len(reducer.output)
		reducer.outputCount--
	}
	reducer.output[reducer.outputWrite] = sample
	reducer.outputWrite = (reducer.outputWrite + 1) % len(reducer.output)
	reducer.outputCount++
}

func fftComplex(values []complex128, inverse bool) {
	for i, j := 1, 0; i < len(values); i++ {
		bit := len(values) >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			values[i], values[j] = values[j], values[i]
		}
	}
	for length := 2; length <= len(values); length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		if inverse {
			angle = -angle
		}
		step := cmplx.Rect(1, angle)
		for start := 0; start < len(values); start += length {
			factor := complex(1, 0)
			for offset := 0; offset < length/2; offset++ {
				even := values[start+offset]
				odd := values[start+offset+length/2] * factor
				values[start+offset], values[start+offset+length/2] = even+odd, even-odd
				factor *= step
			}
		}
	}
	if inverse {
		scale := complex(float64(len(values)), 0)
		for index := range values {
			values[index] /= scale
		}
	}
}
