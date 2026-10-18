package datv

import (
	"math"
	"sync"
)

// channelizer translates the selected DATV channel to baseband and, when the
// input rate permits it, decimates it behind a windowed-sinc anti-alias filter.
// State is preserved between SDR blocks so neither the oscillator nor the FIR
// develops discontinuities at block boundaries.
type channelizer struct {
	inputRate, outputRate int
	decimation            int
	phase, phaseStep      float64
	baseOffsetHz          float64
	mu                    sync.RWMutex
	taps                  []float32
	history               []complex64
	historyAt, decimAt    int
}

func newChannelizer(inputRate, symbolRate int, offsetHz int64) *channelizer {
	if inputRate <= 0 {
		inputRate = 2_048_000
	}
	// Leave at least two complex samples per symbol. This also leaves room for
	// the DVB-S/S2 RRC roll-off while giving useful reductions at narrow rates.
	decimation := 1
	if symbolRate > 0 {
		decimation = max(inputRate/(2*symbolRate), 1)
	}
	c := &channelizer{
		inputRate:    inputRate,
		outputRate:   inputRate / decimation,
		decimation:   decimation,
		baseOffsetHz: float64(offsetHz),
		phaseStep:    -2 * math.Pi * float64(offsetHz) / float64(inputRate),
	}
	if decimation > 1 {
		c.taps = lowpassTaps(16*decimation+1, .46/float64(decimation))
		c.history = make([]complex64, len(c.taps))
	}
	return c
}

func (c *channelizer) process(iq []float32) []float32 {
	if len(iq) < 2 {
		return nil
	}
	out := make([]float32, 0, len(iq)/c.decimation+2)
	c.mu.RLock()
	phaseStep := c.phaseStep
	c.mu.RUnlock()
	for i := 0; i+1 < len(iq); i += 2 {
		cs, sn := float32(math.Cos(c.phase)), float32(math.Sin(c.phase))
		iSample, qSample := iq[i], iq[i+1]
		mixed := complex(iSample*cs-qSample*sn, iSample*sn+qSample*cs)
		c.phase += phaseStep
		if c.phase > math.Pi {
			c.phase -= 2 * math.Pi
		} else if c.phase < -math.Pi {
			c.phase += 2 * math.Pi
		}
		if c.decimation == 1 {
			out = append(out, real(mixed), imag(mixed))
			continue
		}
		c.history[c.historyAt] = mixed
		c.historyAt = (c.historyAt + 1) % len(c.history)
		c.decimAt++
		if c.decimAt < c.decimation {
			continue
		}
		c.decimAt = 0
		var filtered complex64
		at := c.historyAt
		for tap, coefficient := range c.taps {
			index := at - 1 - tap
			if index < 0 {
				index += len(c.history)
			}
			filtered += complex(coefficient, 0) * c.history[index]
		}
		out = append(out, real(filtered), imag(filtered))
	}
	return out
}

func (c *channelizer) setCorrection(hz float64) {
	c.mu.Lock()
	c.phaseStep = -2 * math.Pi * (c.baseOffsetHz + hz) / float64(c.inputRate)
	c.mu.Unlock()
}

func lowpassTaps(count int, cutoff float64) []float32 {
	if count < 3 {
		count = 3
	}
	if count%2 == 0 {
		count++
	}
	taps := make([]float32, count)
	mid := float64(count-1) / 2
	var sum float64
	for i := range taps {
		x := float64(i) - mid
		value := 2 * cutoff
		if x != 0 {
			value = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
		}
		// Blackman gives strong adjacent-channel rejection without an excessive
		// tap count for the narrow QO-100 symbol rates.
		window := .42 - .5*math.Cos(2*math.Pi*float64(i)/float64(count-1)) + .08*math.Cos(4*math.Pi*float64(i)/float64(count-1))
		taps[i] = float32(value * window)
		sum += value * window
	}
	for i := range taps {
		taps[i] /= float32(sum)
	}
	return taps
}
