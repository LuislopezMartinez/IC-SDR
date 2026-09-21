package tetrapol

import "math"

const channelSampleRate = 32_000.0

// channelizer translates the selected VFO to DC, applies a 12.5 kHz channel
// filter and decimates the wide SDR capture before GMSK demodulation.
type channelizer struct {
	inputRate, outputRate float64
	decimation            int
	taps                  []float64
	history               []complex128
	position, counter     int
	phase, phaseStep      float64
}

func newChannelizer(inputRate float64) *channelizer {
	decimation := max(int(math.Round(inputRate/channelSampleRate)), 1)
	outputRate := inputRate / float64(decimation)
	const tapCount = 1025
	taps := make([]float64, tapCount)
	center := float64(tapCount-1) / 2
	cutoff := 5_500.0 / inputRate
	var sum float64
	for index := range taps {
		x := float64(index) - center
		sinc := 2 * cutoff
		if x != 0 {
			sinc = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
		}
		window := .42 - .5*math.Cos(2*math.Pi*float64(index)/float64(tapCount-1)) + .08*math.Cos(4*math.Pi*float64(index)/float64(tapCount-1))
		taps[index] = sinc * window
		sum += taps[index]
	}
	for index := range taps {
		taps[index] /= sum
	}
	return &channelizer{inputRate: inputRate, outputRate: outputRate, decimation: decimation, taps: taps, history: make([]complex128, tapCount)}
}

func (c *channelizer) setOffset(offsetHz float64) {
	c.phaseStep = 2 * math.Pi * offsetHz / c.inputRate
}

func (c *channelizer) reset() {
	clear(c.history)
	c.position, c.counter, c.phase = 0, 0, 0
}

func (c *channelizer) process(iq []float32) []complex128 {
	output := make([]complex128, 0, len(iq)/2/c.decimation+1)
	for index := 0; index+1 < len(iq); index += 2 {
		input := complex(float64(iq[index]), float64(iq[index+1]))
		rotation := complex(math.Cos(c.phase), -math.Sin(c.phase))
		c.phase = math.Mod(c.phase+c.phaseStep, 2*math.Pi)
		c.history[c.position] = input * rotation
		c.position = (c.position + 1) % len(c.history)
		c.counter++
		if c.counter < c.decimation {
			continue
		}
		c.counter = 0
		var filtered complex128
		position := c.position - 1
		if position < 0 {
			position += len(c.history)
		}
		for tap, coefficient := range c.taps {
			historyIndex := position - tap
			if historyIndex < 0 {
				historyIndex += len(c.history)
			}
			filtered += complex(coefficient, 0) * c.history[historyIndex]
		}
		output = append(output, filtered)
	}
	return output
}
