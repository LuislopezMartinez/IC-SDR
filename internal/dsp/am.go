package dsp

import "math"

const (
	// AM carries the audio as only a fraction of the RF envelope. Weak signals
	// and modest modulation depths therefore need more reserve than NFM to
	// reach the same perceived listening level.
	amTargetRMS   = float32(.30)
	amMaximumGain = float32(32)
	amInitialGain = float32(6)
)

// AMDemodulator translates, filters and envelope-demodulates complex IQ.
type AMDemodulator struct {
	inputRate, outputRate float64
	oscillatorPhase       float64
	resamplePhase         float64
	iFilter, qFilter      [3]float32
	dcLevel, audioLowpass float32
	rmsPower, audioGain   float32
	limiterGain           float32
	output                []float32
}

func NewAMDemodulator(inputRate, outputRate float64) *AMDemodulator {
	return &AMDemodulator{inputRate: inputRate, outputRate: outputRate, rmsPower: 1e-6, audioGain: amInitialGain, limiterGain: 1}
}

func (demod *AMDemodulator) Process(iq []float32, frequencyOffsetHz float64, bandwidthHz int) []float32 {
	demod.output = demod.output[:0]
	phaseStep := 2 * math.Pi * frequencyOffsetHz / demod.inputRate
	channelCutoff := math.Min(float64(max(bandwidthHz, 1000))*.5, 12000)
	rfAlpha := float32(1 - math.Exp(-2*math.Pi*channelCutoff/demod.inputRate))
	audioAlpha := float32(1 - math.Exp(-2*math.Pi*math.Min(4500, channelCutoff)/demod.outputRate))
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
		demod.resamplePhase += demod.outputRate
		if demod.resamplePhase < demod.inputRate {
			continue
		}
		demod.resamplePhase -= demod.inputRate
		envelope := float32(math.Hypot(float64(demod.iFilter[2]), float64(demod.qFilter[2])))
		demod.dcLevel += .0025 * (envelope - demod.dcLevel)
		audio := envelope - demod.dcLevel
		demod.audioLowpass += audioAlpha * (audio - demod.audioLowpass)
		demod.output = append(demod.output, demod.processDynamics(demod.audioLowpass))
	}
	return demod.output
}

func (demod *AMDemodulator) processDynamics(sample float32) float32 {
	power := sample * sample
	rmsCoefficient := coefficient(.650, demod.outputRate)
	if power > demod.rmsPower {
		rmsCoefficient = coefficient(.020, demod.outputRate)
	}
	demod.rmsPower += rmsCoefficient * (power - demod.rmsPower)
	rms := float32(math.Sqrt(float64(max(demod.rmsPower, 1e-8))))
	desired := min(max(amTargetRMS/max(rms, .002), .45), amMaximumGain)
	// Rise promptly when a weak AM transmission appears, while retaining the
	// fast downward attack that prevents a strong carrier from clipping.
	gainCoefficient := coefficient(.300, demod.outputRate)
	if desired < demod.audioGain {
		gainCoefficient = coefficient(.030, demod.outputRate)
	}
	demod.audioGain += gainCoefficient * (desired - demod.audioGain)
	compressed := sample * demod.audioGain
	magnitude := absFloat(compressed)
	if magnitude > 1e-6 {
		inputDB := float32(20 * math.Log10(float64(magnitude)))
		outputDB := softKnee(inputDB, -14, 3, 6)
		compressed *= float32(math.Pow(10, float64(outputDB-inputDB+2)/20))
	}
	const ceiling = float32(.89125)
	desiredLimiter := float32(1)
	if absFloat(compressed) > ceiling {
		desiredLimiter = ceiling / absFloat(compressed)
	}
	if desiredLimiter < demod.limiterGain {
		demod.limiterGain = desiredLimiter
	} else {
		demod.limiterGain += coefficient(.100, demod.outputRate) * (desiredLimiter - demod.limiterGain)
	}
	return min(max(compressed*demod.limiterGain, -ceiling), ceiling)
}

func coefficient(seconds, rate float64) float32 { return float32(1 - math.Exp(-1/(seconds*rate))) }
func absFloat(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}
func softKnee(input, threshold, ratio, knee float32) float32 {
	lower, upper := threshold-knee*.5, threshold+knee*.5
	if input <= lower {
		return input
	}
	if input >= upper {
		return threshold + (input-threshold)/ratio
	}
	distance := input - lower
	return input + (1/ratio-1)*distance*distance/(2*knee)
}
