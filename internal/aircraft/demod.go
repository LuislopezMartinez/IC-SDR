package aircraft

import "math"

const (
	modeSPreambleSamples = 16
	modeSLongBits        = 112
	modeSLongSamples     = modeSLongBits * 2
	modeSOverlapSamples  = modeSPreambleSamples + modeSLongSamples
)

func resampleIQ(iq []float32, inputRate, outputRate float64) []float32 {
	n := len(iq) / 2
	if n == 0 || inputRate <= 0 || outputRate <= 0 {
		return nil
	}
	if math.Abs(inputRate-outputRate)/outputRate < 0.001 {
		out := make([]float32, n*2)
		copy(out, iq[:n*2])
		return out
	}
	outN := int(float64(n) * outputRate / inputRate)
	if outN <= 0 {
		return nil
	}
	out := make([]float32, outN*2)
	for j := 0; j < outN; j++ {
		p := float64(j) * inputRate / outputRate
		i0 := int(p)
		frac := float32(p - float64(i0))
		i1 := i0 + 1
		if i0 >= n {
			i0 = n - 1
		}
		if i1 >= n {
			i1 = n - 1
		}
		for q := 0; q < 2; q++ {
			a := iq[i0*2+q]
			b := iq[i1*2+q]
			out[j*2+q] = a + (b-a)*frac
		}
	}
	return out
}

func magnitudes(iq []float32) []uint16 {
	n := len(iq) / 2
	mag := make([]uint16, n)
	for i := 0; i < n; i++ {
		v := math.Hypot(float64(iq[i*2]), float64(iq[i*2+1]))
		if v > 1 {
			v = 1
		}
		mag[i] = uint16(v * 65535)
	}
	return mag
}

func preambleOK(m []uint16, j int) bool {
	if j+15 >= len(m) {
		return false
	}
	if !(m[j] > m[j+1] &&
		m[j+1] < m[j+2] &&
		m[j+2] > m[j+3] &&
		m[j+3] < m[j] &&
		m[j+4] < m[j] &&
		m[j+5] < m[j] &&
		m[j+6] < m[j] &&
		m[j+7] > m[j+8] &&
		m[j+8] < m[j+9] &&
		m[j+9] > m[j+6]) {
		return false
	}
	high := (int(m[j]) + int(m[j+2]) + int(m[j+7]) + int(m[j+9])) / 6
	if int(m[j+4]) >= high || int(m[j+5]) >= high {
		return false
	}
	if j+14 >= len(m) {
		return false
	}
	if int(m[j+11]) >= high || int(m[j+12]) >= high || int(m[j+13]) >= high || int(m[j+14]) >= high {
		return false
	}
	return (int(m[j])+int(m[j+2])+int(m[j+7])+int(m[j+9]))/4 >= 32
}

func detectModeS(m []uint16) [][]byte {
	minLen := modeSPreambleSamples + modeSLongSamples
	if len(m) < minLen {
		return nil
	}
	var frames [][]byte
	for j := 0; j <= len(m)-minLen; j++ {
		if !preambleOK(m, j) {
			continue
		}
		data := m[j+modeSPreambleSamples : j+minLen]
		var bits [modeSLongBits]byte
		errors := 0
		var peak int
		for i := 0; i < modeSLongSamples; i += 2 {
			a, b := int(data[i]), int(data[i+1])
			delta := a - b
			if delta < 0 {
				delta = -delta
			}
			peak += delta
			if a == b {
				bits[i/2] = 2
				if i < 56*2 {
					errors++
				}
			} else if a > b {
				bits[i/2] = 1
			} else {
				bits[i/2] = 0
			}
		}
		if errors > 0 {
			continue
		}
		preamblePeak := (int(m[j]) + int(m[j+2]) + int(m[j+7]) + int(m[j+9])) / 4
		if peak/modeSLongBits < preamblePeak/8 {
			continue
		}
		msg := packModeSBits(bits[:])
		if !modeSCRCValid(msg) {
			continue
		}
		bitsLen := modeSMessageBits(msg)
		frame := make([]byte, bitsLen/8)
		copy(frame, msg[:len(frame)])
		frames = append(frames, frame)
		j += modeSPreambleSamples + bitsLen*2 - 1
	}
	return frames
}

func packModeSBits(bits []byte) []byte {
	msg := make([]byte, modeSLongBits/8)
	for i := 0; i < modeSLongBits; i += 8 {
		msg[i/8] = bits[i]<<7 | bits[i+1]<<6 | bits[i+2]<<5 | bits[i+3]<<4 | bits[i+4]<<3 | bits[i+5]<<2 | bits[i+6]<<1 | bits[i+7]
	}
	return msg
}

func magToIQ(mag []uint16) []float32 {
	iq := make([]float32, len(mag)*2)
	for i, v := range mag {
		x := float32(v) / 65535
		iq[i*2] = x
	}
	return iq
}
