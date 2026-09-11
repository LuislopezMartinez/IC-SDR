package digitalvoice

import (
	"math"
	"sync"
)

const (
	frontendOutputRate = 48_000
	frontendFIRTaps    = 511
	frontendFIRPhases  = 256
	frontendFIRHalf    = frontendFIRTaps / 2
	frontendHistory    = 4096
)

// frontend is a dedicated flat-discriminator path for DSD-neo. It performs
// channel translation and a Blackman-windowed polyphase FIR before FM phase
// discrimination. No de-emphasis, audio AGC, squelch or listening filter is
// present, so four-level FSK symbol spacing reaches the decoder intact.
type frontend struct {
	mu                   sync.Mutex
	inputRate, offsetHz  float64
	bandwidth            int
	ncoCos, ncoSin       float64
	ncoSamples           uint64
	iHistory, qHistory   [frontendHistory]float64
	inputSampleIndex     int64
	nextOutputPosition   float64
	previousI, previousQ float64
	kernel               [][]float64
}

func newFrontend(rate, offset float64, bandwidth int) *frontend {
	f := &frontend{inputRate: rate}
	f.reset(offset, bandwidth)
	return f
}

func (f *frontend) reset(offset float64, bandwidth int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	bandwidth = min(max(bandwidth, 8_000), 20_000)
	if f.kernel == nil || bandwidth != f.bandwidth {
		// 7 kHz retains DMR/P25/YSF symbol transitions while rejecting the
		// adjacent 12.5 kHz channel visible in a wide SDR capture.
		f.kernel = buildFrontendKernel(f.inputRate, math.Min(7000, float64(bandwidth)*.52))
	}
	f.offsetHz, f.bandwidth = offset, bandwidth
	f.ncoCos, f.ncoSin, f.ncoSamples = 1, 0, 0
	f.inputSampleIndex, f.nextOutputPosition = 0, 0
	f.previousI, f.previousQ = 1, 0
	f.iHistory, f.qHistory = [frontendHistory]float64{}, [frontendHistory]float64{}
}

func buildFrontendKernel(inputRate, cutoffHz float64) [][]float64 {
	result := make([][]float64, frontendFIRPhases)
	if inputRate <= 0 {
		return result
	}
	normalizedCutoff := cutoffHz / inputRate
	for phase := range result {
		fraction := float64(phase) / float64(frontendFIRPhases-1)
		result[phase] = make([]float64, frontendFIRTaps)
		sum := 0.0
		for tap := range result[phase] {
			distance := float64(tap-frontendFIRHalf) - fraction
			sinc := 2 * normalizedCutoff
			if math.Abs(distance) >= 1e-12 {
				sinc = math.Sin(2*math.Pi*normalizedCutoff*distance) / (math.Pi * distance)
			}
			window := .42 - .5*math.Cos(2*math.Pi*float64(tap)/float64(frontendFIRTaps-1)) + .08*math.Cos(4*math.Pi*float64(tap)/float64(frontendFIRTaps-1))
			result[phase][tap] = sinc * window
			sum += result[phase][tap]
		}
		if math.Abs(sum) > 1e-12 {
			for tap := range result[phase] {
				result[phase][tap] /= sum
			}
		}
	}
	return result
}

func (f *frontend) process(iq []float32) ([]int16, float32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inputRate <= 0 || len(f.kernel) != frontendFIRPhases {
		return nil, -60
	}
	phaseStep := 2 * math.Pi * f.offsetHz / f.inputRate
	stepCos, stepSin := math.Cos(phaseStep), math.Sin(phaseStep)
	inputPerOutput := f.inputRate / frontendOutputRate
	result := make([]int16, 0, int(float64(len(iq)/2)/inputPerOutput)+2)
	power := float64(0)
	for n := 0; n+1 < len(iq); n += 2 {
		i := float64(iq[n])*f.ncoCos + float64(iq[n+1])*f.ncoSin
		q := float64(iq[n+1])*f.ncoCos - float64(iq[n])*f.ncoSin
		oldCos := f.ncoCos
		f.ncoCos = oldCos*stepCos - f.ncoSin*stepSin
		f.ncoSin = f.ncoSin*stepCos + oldCos*stepSin
		f.ncoSamples++
		if f.ncoSamples%4096 == 0 {
			magnitude := math.Hypot(f.ncoCos, f.ncoSin)
			if magnitude > 0 {
				f.ncoCos, f.ncoSin = f.ncoCos/magnitude, f.ncoSin/magnitude
			}
		}
		historyIndex := int(f.inputSampleIndex % frontendHistory)
		f.iHistory[historyIndex], f.qHistory[historyIndex] = i, q
		f.inputSampleIndex++
		for float64(f.inputSampleIndex-1) >= f.nextOutputPosition+frontendFIRHalf {
			center := int64(math.Floor(f.nextOutputPosition))
			fraction := f.nextOutputPosition - float64(center)
			phase := min(max(int(math.Round(fraction*float64(frontendFIRPhases-1))), 0), frontendFIRPhases-1)
			cleanI, cleanQ := 0.0, 0.0
			for tap, coefficient := range f.kernel[phase] {
				sourceIndex := center + int64(tap-frontendFIRHalf)
				if sourceIndex < 0 || f.inputSampleIndex-sourceIndex > frontendHistory {
					continue
				}
				source := int(sourceIndex % frontendHistory)
				cleanI += f.iHistory[source] * coefficient
				cleanQ += f.qHistory[source] * coefficient
			}
			cross := cleanQ*f.previousI - cleanI*f.previousQ
			dot := cleanI*f.previousI + cleanQ*f.previousQ
			instantHz := math.Atan2(cross, dot) * frontendOutputRate / (2 * math.Pi)
			f.previousI, f.previousQ = cleanI, cleanQ
			// ±2.5 kHz deviation maps near ±0.55 full scale. This retains
			// headroom for frequency error without compressing the four levels.
			value := min(max(instantHz/2500*.55, -.95), .95)
			power += value * value
			result = append(result, int16(math.Round(value*32767)))
			f.nextOutputPosition += inputPerOutput
		}
	}
	if len(result) == 0 {
		return result, -60
	}
	return result, float32(20 * math.Log10(max(math.Sqrt(power/float64(len(result))), 1e-6)))
}
