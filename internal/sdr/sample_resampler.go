package sdr

import "math"

// complexResampler converts interleaved complex float samples between fixed
// rates. A small windowed-sinc table keeps the Airspy adapter isolated from
// the receiver DSP, which continues to run at its established 2.048 MS/s.
type complexResampler struct {
	step, position float64
	half, phases   int
	table          [][]float32
	input          []float32
}

func newComplexResampler(inputRate, outputRate float64) *complexResampler {
	const taps = 32
	const phases = 1024
	r := &complexResampler{
		step:     inputRate / outputRate,
		position: taps / 2,
		half:     taps / 2,
		phases:   phases,
		table:    make([][]float32, phases),
		input:    make([]float32, taps*2),
	}
	cutoff := .47 * math.Min(1, outputRate/inputRate)
	for phase := range r.table {
		fraction := float64(phase) / phases
		coefficients := make([]float32, taps)
		var sum float64
		for tap := range coefficients {
			x := float64(tap-r.half+1) - fraction
			window := .42 - .5*math.Cos(2*math.Pi*float64(tap)/float64(taps-1)) + .08*math.Cos(4*math.Pi*float64(tap)/float64(taps-1))
			value := 2 * cutoff * sinc(2*cutoff*x) * window
			coefficients[tap] = float32(value)
			sum += value
		}
		for tap := range coefficients {
			coefficients[tap] /= float32(sum)
		}
		r.table[phase] = coefficients
	}
	return r
}

func sinc(x float64) float64 {
	if math.Abs(x) < 1e-12 {
		return 1
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}

func (r *complexResampler) push(samples []float32) { r.input = append(r.input, samples...) }

func (r *complexResampler) pull(destination []float32) int {
	capacitySamples := len(destination) / 2
	produced := 0
	inputSamples := len(r.input) / 2
	for produced < capacitySamples && int(r.position)+r.half < inputSamples {
		base := int(math.Floor(r.position)) - r.half + 1
		fraction := r.position - math.Floor(r.position)
		phase := int(math.Round(fraction * float64(r.phases)))
		if phase == r.phases {
			phase = 0
			base++
		}
		coefficients := r.table[phase]
		var iValue, qValue float32
		for tap, coefficient := range coefficients {
			index := (base + tap) * 2
			iValue += r.input[index] * coefficient
			qValue += r.input[index+1] * coefficient
		}
		destination[produced*2] = iValue
		destination[produced*2+1] = qValue
		produced++
		r.position += r.step
	}

	drop := int(math.Floor(r.position)) - r.half
	if drop > 0 {
		copy(r.input, r.input[drop*2:])
		r.input = r.input[:len(r.input)-drop*2]
		r.position -= float64(drop)
	}
	return produced
}
