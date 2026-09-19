package dsp

import "math"

// NFMDemodulator implements a quadrature discriminator followed by DC removal,
// FM de-emphasis and the same listening dynamics used by the AM path.
type NFMDemodulator struct {
	inputRate, outputRate               float64
	oscillatorPhase, resamplePhase      float64
	iFilter, qFilter                    [3]float32
	previousI, previousQ                float32
	previousAudio, highpass, deemphasis float32
	rmsPower, gain, limiterGain         float32
	wideFIR                             [511]float32
	wideHistory                         [511]float32
	wideWrite                           int
	output                              []float32
	subaudible                          []float32
}

func NewNFMDemodulator(inputRate, outputRate float64) *NFMDemodulator {
	demod := &NFMDemodulator{inputRate: inputRate, outputRate: outputRate, previousI: 1, rmsPower: .0001, gain: 1, limiterGain: 1}
	demod.configureWideFIR()
	return demod
}

func (demod *NFMDemodulator) configureWideFIR() {
	const cutoffHz = 16_000
	middle := float64(len(demod.wideFIR)-1) / 2
	sum := float64(0)
	for tap := range demod.wideFIR {
		distance := float64(tap) - middle
		coefficient := 2 * cutoffHz / demod.inputRate
		if distance != 0 {
			coefficient = math.Sin(2*math.Pi*cutoffHz/demod.inputRate*distance) / (math.Pi * distance)
		}
		window := .42 - .5*math.Cos(2*math.Pi*float64(tap)/float64(len(demod.wideFIR)-1)) + .08*math.Cos(4*math.Pi*float64(tap)/float64(len(demod.wideFIR)-1))
		demod.wideFIR[tap] = float32(coefficient * window)
		sum += coefficient * window
	}
	for tap := range demod.wideFIR {
		demod.wideFIR[tap] /= float32(sum)
	}
}

func (demod *NFMDemodulator) pushWide(sample float32) {
	demod.wideHistory[demod.wideWrite] = sample
	demod.wideWrite = (demod.wideWrite + 1) % len(demod.wideHistory)
}

func (demod *NFMDemodulator) filteredWide() float32 {
	value := float32(0)
	position := demod.wideWrite
	for tap := range demod.wideFIR {
		position--
		if position < 0 {
			position = len(demod.wideHistory) - 1
		}
		value += demod.wideHistory[position] * demod.wideFIR[tap]
	}
	return value
}

func (demod *NFMDemodulator) Process(iq []float32, frequencyOffsetHz float64, bandwidthHz, deemphasisUs int) []float32 {
	return demod.process(iq, frequencyOffsetHz, bandwidthHz, deemphasisUs, false)
}

// ProcessWide demodulates mono broadcast FM. It uses the wider RF channel and
// approximately 75 kHz peak deviation used by WFM broadcasts while retaining
// the common 48 kHz audio output and configurable regional de-emphasis.
func (demod *NFMDemodulator) ProcessWide(iq []float32, frequencyOffsetHz float64, bandwidthHz, deemphasisUs int) []float32 {
	return demod.process(iq, frequencyOffsetHz, bandwidthHz, deemphasisUs, true)
}

func (demod *NFMDemodulator) process(iq []float32, frequencyOffsetHz float64, bandwidthHz, deemphasisUs int, wide bool) []float32 {
	demod.output = demod.output[:0]
	demod.subaudible = demod.subaudible[:0]
	bandwidthHz = max(bandwidthHz, 5000)
	if deemphasisUs != 75 {
		deemphasisUs = 50
	}
	phaseStep := 2 * math.Pi * frequencyOffsetHz / demod.inputRate
	channelCutoff := math.Min(float64(bandwidthHz)*.48, 14000)
	deviationHz := min(max(float32(bandwidthHz)/5, 1500), 5000)
	highpassHz := 280.
	if wide {
		channelCutoff = math.Min(float64(bandwidthHz)*.48, 100000)
		deviationHz = min(max(float32(bandwidthHz)*.36, 45000), 85000)
		highpassHz = 30
	}
	rfAlpha := float32(1 - math.Exp(-2*math.Pi*channelCutoff/demod.inputRate))
	highpassR := float32(math.Exp(-2 * math.Pi * highpassHz / demod.outputRate))
	deemphasisAlpha := float32(1 - math.Exp(-1/(float64(deemphasisUs)*1e-6*demod.outputRate)))
	deviationRadians := float32(2 * math.Pi * float64(deviationHz) / demod.inputRate)
	for index := 0; index+1 < len(iq); index += 2 {
		cosine, sine := float32(math.Cos(demod.oscillatorPhase)), float32(math.Sin(demod.oscillatorPhase))
		mixedI := iq[index]*cosine + iq[index+1]*sine
		mixedQ := iq[index+1]*cosine - iq[index]*sine
		demod.oscillatorPhase += phaseStep
		if demod.oscillatorPhase > math.Pi {
			demod.oscillatorPhase -= 2 * math.Pi
		}
		if demod.oscillatorPhase < -math.Pi {
			demod.oscillatorPhase += 2 * math.Pi
		}
		demod.iFilter[0] += rfAlpha * (mixedI - demod.iFilter[0])
		demod.qFilter[0] += rfAlpha * (mixedQ - demod.qFilter[0])
		for stage := 1; stage < 3; stage++ {
			demod.iFilter[stage] += rfAlpha * (demod.iFilter[stage-1] - demod.iFilter[stage])
			demod.qFilter[stage] += rfAlpha * (demod.qFilter[stage-1] - demod.qFilter[stage])
		}
		cross := demod.qFilter[2]*demod.previousI - demod.iFilter[2]*demod.previousQ
		dot := demod.iFilter[2]*demod.previousI + demod.qFilter[2]*demod.previousQ
		discriminator := float32(math.Atan2(float64(cross), float64(dot))) / max(deviationRadians, 1e-6)
		demod.previousI, demod.previousQ = demod.iFilter[2], demod.qFilter[2]
		if wide {
			// Remove the 19 kHz pilot, 38 kHz stereo difference signal, 57 kHz
			// RDS and ultrasonic FM noise before decimation to 48 kHz. Sampling
			// the raw discriminator folded those components into audible audio.
			demod.pushWide(discriminator)
		}
		demod.resamplePhase += demod.outputRate
		if demod.resamplePhase < demod.inputRate {
			continue
		}
		demod.resamplePhase -= demod.inputRate
		if wide {
			discriminator = demod.filteredWide()
		}
		demod.highpass = discriminator - demod.previousAudio + highpassR*demod.highpass
		demod.previousAudio = discriminator
		demod.subaudible = append(demod.subaudible, discriminator)
		demod.deemphasis += deemphasisAlpha * (demod.highpass - demod.deemphasis)
		if wide {
			demod.output = append(demod.output, demod.processWideDynamics(demod.deemphasis))
		} else {
			demod.output = append(demod.output, demod.processDynamics(demod.deemphasis))
		}
	}
	return demod.output
}

func (demod *NFMDemodulator) processWideDynamics(sample float32) float32 {
	// Broadcast stations already apply carefully controlled programme dynamics.
	// Preserve them and use only a transparent safety limiter.
	const ceiling = float32(.95)
	desired := float32(1)
	if magnitude := absFloat(sample); magnitude > ceiling {
		desired = ceiling / magnitude
	}
	if desired < demod.limiterGain {
		demod.limiterGain = desired
	} else {
		demod.limiterGain += coefficient(.150, demod.outputRate) * (desired - demod.limiterGain)
	}
	return min(max(sample*demod.limiterGain, -ceiling), ceiling)
}

func (demod *NFMDemodulator) Subaudible() []float32 { return demod.subaudible }

func (demod *NFMDemodulator) processDynamics(sample float32) float32 {
	power := sample * sample
	rmsCoefficient := coefficient(.650, demod.outputRate)
	if power > demod.rmsPower {
		rmsCoefficient = coefficient(.020, demod.outputRate)
	}
	demod.rmsPower += rmsCoefficient * (power - demod.rmsPower)
	rms := float32(math.Sqrt(float64(max(demod.rmsPower, 1e-8))))
	desired := min(max(.22/max(rms, .008), .35), 12)
	gainCoefficient := coefficient(.750, demod.outputRate)
	if desired < demod.gain {
		gainCoefficient = coefficient(.030, demod.outputRate)
	}
	demod.gain += gainCoefficient * (desired - demod.gain)
	value := sample * demod.gain
	magnitude := absFloat(value)
	if magnitude > 1e-6 {
		inputDB := float32(20 * math.Log10(float64(magnitude)))
		outputDB := softKnee(inputDB, -14, 3, 6)
		value *= float32(math.Pow(10, float64(outputDB-inputDB+2)/20))
	}
	const ceiling = float32(.89125)
	desiredLimiter := float32(1)
	if absFloat(value) > ceiling {
		desiredLimiter = ceiling / absFloat(value)
	}
	if desiredLimiter < demod.limiterGain {
		demod.limiterGain = desiredLimiter
	} else {
		demod.limiterGain += coefficient(.100, demod.outputRate) * (desiredLimiter - demod.limiterGain)
	}
	return min(max(value*demod.limiterGain, -ceiling), ceiling)
}
