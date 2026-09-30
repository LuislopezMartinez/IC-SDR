package dsp

import (
	"math"

	"go-zero/internal/i18n"
)

const (
	// SSB needs considerably more gain reserve than AM or FM because weak
	// sideband signals can remain intelligible far below their output level.
	ssbTargetRMS      = float32(.34)
	ssbMaximumGain    = float32(96)
	ssbInitialGain    = float32(16)
	ssbLimiterCeiling = float32(.94)
)

// SSBDemodulator isolates and translates either sideband into mono audio.
type SSBDemodulator struct {
	inputRate, outputRate             float64
	channelPhase, audioPhase          float64
	resamplePhase                     float64
	iFilter, qFilter                  [3]float32
	previousAudio, highpass, tuneGain float32
	rmsPower, gain, limiterGain       float32
	configuredMode                    string
	configuredOffset                  float64
	configured                        bool
	output                            []float32
}

func NewSSBDemodulator(inputRate, outputRate float64) *SSBDemodulator {
	return &SSBDemodulator{inputRate: inputRate, outputRate: outputRate, rmsPower: 1e-6, gain: ssbInitialGain, limiterGain: 1}
}

func (demod *SSBDemodulator) Process(iq []float32, mode string, frequencyOffsetHz float64, bandwidthHz int) []float32 {
	return demod.ProcessPBT(iq, mode, frequencyOffsetHz, 100, bandwidthHz)
}

func (demod *SSBDemodulator) ProcessPBT(iq []float32, mode string, frequencyOffsetHz float64, lowCutHz, highCutHz int) []float32 {
	demod.output = demod.output[:0]
	if mode != i18n.Source("text.61f0acff1735") && mode != i18n.Source("text.6323db4948ad") {
		return demod.output
	}
	if !demod.configured || mode != demod.configuredMode || math.Abs(frequencyOffsetHz-demod.configuredOffset) >= 1 {
		demod.resetSignalPath()
	}
	demod.configured, demod.configuredMode, demod.configuredOffset = true, mode, frequencyOffsetHz
	lowCutHz = min(max(lowCutHz, 50), 4800)
	highCutHz = min(max(highCutHz, lowCutHz+200), 5000)
	lowHz := float32(lowCutHz)
	highHz := float32(highCutHz)
	sidebandSign := float32(1)
	if mode == i18n.Source("text.6323db4948ad") {
		sidebandSign = -1
	}
	audioCenter := (lowHz + highHz) * .5
	halfBandwidth := (highHz - lowHz) * .5
	channelStep := 2 * math.Pi * (frequencyOffsetHz + float64(sidebandSign*audioCenter)) / demod.inputRate
	filterCutoff := max(float32(180), halfBandwidth*1.9)
	filterAlpha := float32(1 - math.Exp(-2*math.Pi*float64(filterCutoff)/demod.inputRate))
	audioStep := 2 * math.Pi * float64(sidebandSign*audioCenter) / demod.outputRate
	highpassR := float32(math.Exp(-2 * math.Pi * 120 / demod.outputRate))
	tuneAttack := coefficient(.015, demod.outputRate)
	for index := 0; index+1 < len(iq); index += 2 {
		cosine, sine := float32(math.Cos(demod.channelPhase)), float32(math.Sin(demod.channelPhase))
		shiftedI := iq[index]*cosine + iq[index+1]*sine
		shiftedQ := iq[index+1]*cosine - iq[index]*sine
		demod.channelPhase += channelStep
		if demod.channelPhase > math.Pi {
			demod.channelPhase -= 2 * math.Pi
		}
		if demod.channelPhase < -math.Pi {
			demod.channelPhase += 2 * math.Pi
		}
		demod.iFilter[0] += filterAlpha * (shiftedI - demod.iFilter[0])
		demod.qFilter[0] += filterAlpha * (shiftedQ - demod.qFilter[0])
		for stage := 1; stage < 3; stage++ {
			demod.iFilter[stage] += filterAlpha * (demod.iFilter[stage-1] - demod.iFilter[stage])
			demod.qFilter[stage] += filterAlpha * (demod.qFilter[stage-1] - demod.qFilter[stage])
		}
		demod.resamplePhase += demod.outputRate
		if demod.resamplePhase < demod.inputRate {
			continue
		}
		demod.resamplePhase -= demod.inputRate
		audioCosine, audioSine := float32(math.Cos(demod.audioPhase)), float32(math.Sin(demod.audioPhase))
		audio := demod.iFilter[2]*audioCosine - demod.qFilter[2]*audioSine
		demod.audioPhase += audioStep
		if demod.audioPhase > math.Pi {
			demod.audioPhase -= 2 * math.Pi
		}
		if demod.audioPhase < -math.Pi {
			demod.audioPhase += 2 * math.Pi
		}
		demod.highpass = audio - demod.previousAudio + highpassR*demod.highpass
		demod.previousAudio = audio
		demod.tuneGain += tuneAttack * (1 - demod.tuneGain)
		demod.output = append(demod.output, demod.processDynamics(demod.highpass*demod.tuneGain))
	}
	return demod.output
}

func (demod *SSBDemodulator) processDynamics(sample float32) float32 {
	power := sample * sample
	rmsCoefficient := coefficient(.650, demod.outputRate)
	if power > demod.rmsPower {
		rmsCoefficient = coefficient(.020, demod.outputRate)
	}
	demod.rmsPower += rmsCoefficient * (power - demod.rmsPower)
	rms := float32(math.Sqrt(float64(max(demod.rmsPower, 1e-8))))
	desired := min(max(ssbTargetRMS/max(rms, .001), .5), ssbMaximumGain)
	gainCoefficient := coefficient(.300, demod.outputRate)
	if desired < demod.gain {
		gainCoefficient = coefficient(.020, demod.outputRate)
	}
	demod.gain += gainCoefficient * (desired - demod.gain)
	value := sample * demod.gain
	magnitude := absFloat(value)
	if magnitude > 1e-6 {
		inputDB := float32(20 * math.Log10(float64(magnitude)))
		outputDB := softKnee(inputDB, -10, 3.5, 6)
		value *= float32(math.Pow(10, float64(outputDB-inputDB+4)/20))
	}
	desiredLimiter := float32(1)
	if absFloat(value) > ssbLimiterCeiling {
		desiredLimiter = ssbLimiterCeiling / absFloat(value)
	}
	if desiredLimiter < demod.limiterGain {
		demod.limiterGain = desiredLimiter
	} else {
		demod.limiterGain += coefficient(.100, demod.outputRate) * (desiredLimiter - demod.limiterGain)
	}
	return min(max(value*demod.limiterGain, -ssbLimiterCeiling), ssbLimiterCeiling)
}

func (demod *SSBDemodulator) resetSignalPath() {
	demod.channelPhase, demod.audioPhase, demod.resamplePhase = 0, 0, 0
	demod.iFilter, demod.qFilter = [3]float32{}, [3]float32{}
	demod.previousAudio, demod.highpass, demod.tuneGain = 0, 0, 0
	demod.rmsPower, demod.gain, demod.limiterGain = 1e-6, ssbInitialGain, 1
}
