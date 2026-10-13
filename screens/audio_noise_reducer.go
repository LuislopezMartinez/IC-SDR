package screens

import (
	"math"
	"math/cmplx"
)

const (
	noiseFFTSize = 512
	noiseHopSize = noiseFFTSize / 2
)

var noiseFrequencyWeights = [5]float64{.10, .20, .40, .20, .10}

// audioNoiseReducer is a streaming spectral Wiener filter tuned for received
// communications audio. It uses a soft signal-presence probability rather
// than a voice gate, so weak SSB and AM remain valid input.
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
	priorSNR    [noiseFFTSize/2 + 1]float64
	tonalMemory [noiseFFTSize/2 + 1]float64
	gain        [noiseFFTSize/2 + 1]float64
	frames      int
	minimumAge  int
	activity    float32
	reductionDB float32
	signal      float32
	wet         float64
	broadGain   float64
	voice       float64
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
	reducer.broadGain = 1
	// Begin in the second half; the zeroed first half supplies causal history.
	reducer.inputWrite = noiseHopSize
	return reducer
}

func (reducer *audioNoiseReducer) process(samples []float32, enabled bool, strength float32, speed string) {
	amount := float64(min(max(strength, 0), 100)) / 100
	if !enabled {
		amount = 0
	}
	for index, sample := range samples {
		reducer.input[reducer.inputWrite] = sample
		reducer.inputWrite++
		if reducer.inputWrite == noiseFFTSize {
			reducer.processFrame(amount, speed)
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

func (reducer *audioNoiseReducer) processFrame(amount float64, speed string) {
	var bins [noiseFFTSize]complex128
	var power [noiseFFTSize/2 + 1]float64
	var targetGain [noiseFFTSize/2 + 1]float64
	var speechProbability [noiseFFTSize/2 + 1]float64
	var tonalPeak [noiseFFTSize/2 + 1]bool
	for index, sample := range reducer.input {
		bins[index] = complex(float64(sample)*reducer.window[index], 0)
	}
	fftComplex(bins[:], false)
	for bin := 0; bin <= noiseFFTSize/2; bin++ {
		realPart, imaginaryPart := real(bins[bin]), imag(bins[bin])
		power[bin] = realPart*realPart + imaginaryPart*imaginaryPart
	}
	// The strength control changes both the maximum attenuation and the
	// aggressiveness of the Wiener estimate. Limiting the useful range to
	// roughly 6..18 dB avoids the metallic audio produced by near-zero bins.
	overSubtract := 1.0 + 1.15*amount
	maximumReductionDB := 3.0 + 15.0*amount
	minimumGain := math.Pow(10, -maximumReductionDB/20)
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
		if reducer.frames == 0 {
			reducer.noisePower[bin] = max(currentPower, 1e-16)
			reducer.smoothPower[bin] = currentPower
			reducer.minimum[bin] = currentPower
			reducer.previousMin[bin] = currentPower
		} else {
			reducer.smoothPower[bin] += .16 * (currentPower - reducer.smoothPower[bin])
			reducer.minimum[bin] = min(reducer.minimum[bin], reducer.smoothPower[bin])
		}

		posteriorSNR := currentPower / max(reducer.noisePower[bin], 1e-16)
		instantPrior := max(posteriorSNR-1, 0)
		// Decision-directed prior SNR changes slowly enough that speech gaps do
		// not open and close individual FFT bins on every frame.
		reducer.priorSNR[bin] = .92*reducer.priorSNR[bin] + .08*instantPrior
		probability := min(max((reducer.priorSNR[bin]-.15)/3.5, 0), 1)
		peakNow := neighbourPower > 0 && currentPower > 5*neighbourPower
		if peakNow {
			// Preserve stable carriers and voiced harmonics without turning the
			// whole decision into the old binary peak detector.
			reducer.tonalMemory[bin] += .34 * (1 - reducer.tonalMemory[bin])
		} else {
			reducer.tonalMemory[bin] *= .25
		}
		// Random peaks in receiver hiss normally last one frame. Requiring
		// persistence prevents those peaks from becoming metallic whistles.
		if reducer.tonalMemory[bin] > .46 {
			probability = max(probability, .92)
			tonalPeak[bin] = true
		}
		speechProbability[bin] = probability

		if reducer.frames > 0 {
			// Minima-controlled recursive averaging: the floor follows falling
			// noise quickly, rising noise slowly, and nearly freezes under speech.
			candidate := max(1e-16, 2.0*min(reducer.minimum[bin], reducer.previousMin[bin]))
			rate := .08 * (1 - .92*probability)
			if candidate > reducer.noisePower[bin] {
				rate *= .10
			}
			reducer.noisePower[bin] += rate * (candidate - reducer.noisePower[bin])
		}

		wiener := reducer.priorSNR[bin] / (reducer.priorSNR[bin] + overSubtract)
		wiener = max(minimumGain, math.Sqrt(max(wiener, 0)))
		// Soft protection retains consonants and weak speech instead of making
		// an all-or-nothing decision for each spectral bin.
		targetGain[bin] = wiener + probability*.62*(1-wiener)
	}
	for bin, peak := range tonalPeak {
		if !peak {
			continue
		}
		// A Hann-window tone occupies several adjacent bins. Protect its whole
		// main lobe so frequency smoothing cannot erode it from the sides.
		for offset := -3; offset <= 3; offset++ {
			candidate := bin + offset
			if candidate >= 0 && candidate < len(targetGain) {
				targetGain[candidate] = max(targetGain[candidate], .92)
			}
		}
	}
	// Estimate voice presence only across the communications passband. Bins
	// above it mostly contain filtered residue and used to dilute this value.
	bandPower, bandNoise, tonalPresence := 0.0, 0.0, 0.0
	for bin := 1; bin <= min(43, noiseFFTSize/2); bin++ {
		bandPower += power[bin]
		bandNoise += reducer.noisePower[bin]
		if reducer.tonalMemory[bin] > .80 {
			tonalPresence = max(tonalPresence, reducer.tonalMemory[bin])
		}
	}
	// A genuine voice frame raises the total passband energy above the learned
	// floor. Coloured hiss may contain persistent peaks, but once learnt it does
	// not satisfy this SNR test and can no longer hold the broadband gate open.
	bandSNR := bandPower / max(bandNoise, 1e-16)
	presenceThreshold, presenceRange := 2.4, 3.5
	voiceRelease, noiseClose := .16, .10
	if speed == "slow" {
		presenceThreshold, presenceRange = 1.9, 3.8
		voiceRelease, noiseClose = .07, .045
	} else if speed == "fast" {
		presenceThreshold, presenceRange = 3.0, 3.0
		voiceRelease, noiseClose = .30, .22
	}
	energyPresence := min(max((bandSNR-presenceThreshold)/presenceRange, 0), 1)
	rawSignal := min(max(energyPresence, tonalPresence), 1)
	voiceRate := voiceRelease
	if rawSignal > reducer.voice {
		voiceRate = .34
	}
	reducer.voice += voiceRate * (rawSignal - reducer.voice)
	frameSignal := reducer.voice
	// Spectral suppression stays moderate while a smooth broadband stage lowers
	// the remaining hiss in speech gaps. Uniform attenuation cannot create the
	// isolated spectral holes perceived as "musical noise".
	extraReductionDB := 2.0 + 6.0*amount
	noiseOnlyGain := math.Pow(10, -extraReductionDB/20)
	speechMix := min(max((frameSignal-.10)/.42, 0), 1)
	targetBroadGain := noiseOnlyGain + speechMix*(1-noiseOnlyGain)
	broadRate := noiseClose
	if targetBroadGain > reducer.broadGain {
		// Open quickly for consonants; close slowly after speech to avoid pumping.
		broadRate = .42
	}
	reducer.broadGain += broadRate * (targetBroadGain - reducer.broadGain)
	attenuation := 0.0
	targetWet := min(1.0, amount*2.0)
	// About 35 ms at a 256-sample hop prevents clicks and abrupt ambience
	// changes when NR is enabled, disabled, or moved across the slider.
	reducer.wet += .14 * (targetWet - reducer.wet)
	for bin := 0; bin <= noiseFFTSize/2; bin++ {
		// Five-bin frequency smoothing suppresses isolated spectral holes, the
		// main source of the characteristic "musical noise" artifact.
		target, weight := 0.0, 0.0
		for offset := -2; offset <= 2; offset++ {
			candidate := bin + offset
			if candidate >= 0 && candidate < len(targetGain) {
				coefficient := noiseFrequencyWeights[offset+2]
				target += coefficient * targetGain[candidate]
				weight += coefficient
			}
		}
		target /= weight
		rate := .12
		if target > reducer.gain[bin] {
			rate = .32
		}
		reducer.gain[bin] += rate * (target - reducer.gain[bin])
		spectralApplied := 1 - reducer.wet*(1-reducer.gain[bin])
		broadApplied := 1 - reducer.wet*(1-reducer.broadGain)
		applied := spectralApplied * broadApplied
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
	meanGain := max(1e-4, 1-float64(reducer.activity))
	reduction := float32(-20 * math.Log10(meanGain))
	reducer.reductionDB += .12 * (reduction - reducer.reductionDB)
	signal := float32(frameSignal)
	reducer.signal += .12 * (signal - reducer.signal)
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
