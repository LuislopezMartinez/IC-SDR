package digitalvoice

// linearResampler keeps interpolation state across stdout reads. DSD-neo
// writes headerless PCM in arbitrary block sizes, so treating each Read as an
// independent clip would introduce a discontinuity at every boundary.
type linearResampler struct {
	ratio        int
	previous     float32
	havePrevious bool
}

func (r *linearResampler) reset(inputRate, outputRate int) {
	r.ratio = max(outputRate/max(inputRate, 1), 1)
	r.previous, r.havePrevious = 0, false
}

func (r *linearResampler) process(input []float32) []float32 {
	if len(input) == 0 {
		return nil
	}
	if r.ratio <= 1 {
		return append([]float32(nil), input...)
	}
	output := make([]float32, 0, len(input)*r.ratio)
	for _, current := range input {
		if !r.havePrevious {
			r.previous, r.havePrevious = current, true
			continue
		}
		for phase := 0; phase < r.ratio; phase++ {
			fraction := float32(phase) / float32(r.ratio)
			output = append(output, r.previous+(current-r.previous)*fraction)
		}
		r.previous = current
	}
	return output
}
