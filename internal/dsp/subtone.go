package dsp

import (
	"go-zero/internal/i18n"

	"fmt"
	"math"
	"sort"
	"sync"
)

var ctcssTones = []float64{67.0, 69.3, 71.9, 74.4, 77.0, 79.7, 82.5, 85.4, 88.5, 91.5, 94.8, 97.4, 100.0, 103.5, 107.2, 110.9, 114.8, 118.8, 123.0, 127.3, 131.8, 136.5, 141.3, 146.2, 151.4, 156.7, 159.8, 162.2, 165.5, 167.9, 171.3, 173.8, 177.3, 179.9, 183.5, 186.2, 189.9, 192.8, 196.6, 199.5, 203.5, 206.5, 210.7, 218.1, 225.7, 229.1, 233.6, 241.8, 250.3, 254.1}

type SubtoneStatus struct {
	Mode, Kind, Value string
	Confidence        float32
	Detected          bool
}

// SubtoneDetector consumes the discriminator output before voice filtering.
type SubtoneDetector struct {
	mu            sync.RWMutex
	mode          string
	status        SubtoneStatus
	lowpass       float32
	decimation    int
	ctcss         []float32
	dcs           dcsDetector
	candidate     string
	confirmations int
}

func NewSubtoneDetector() *SubtoneDetector {
	d := &SubtoneDetector{mode: i18n.Source("text.6ea56fae9eac"), ctcss: make([]float32, 0, 1800)}
	d.status.Mode = i18n.Source("text.6ea56fae9eac")
	d.dcs.init()
	return d
}

func (d *SubtoneDetector) SetMode(mode string) {
	if mode != i18n.Source("text.6ea56fae9eac") && mode != i18n.Source("text.74108b47eb26") && mode != i18n.Source("text.fb09c8f399c7") && mode != i18n.Source("text.38cca6bea010") {
		mode = i18n.Source("text.6ea56fae9eac")
	}
	d.mu.Lock()
	d.mode = mode
	d.status = SubtoneStatus{Mode: mode}
	d.candidate = ""
	d.confirmations = 0
	d.mu.Unlock()
}
func (d *SubtoneDetector) Snapshot() SubtoneStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.status
}

func (d *SubtoneDetector) Process(samples []float32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.mode == i18n.Source("text.38cca6bea010") {
		return
	}
	alpha := float32(1 - math.Exp(-2*math.Pi*250/48000))
	for _, sample := range samples {
		d.lowpass += alpha * (sample - d.lowpass)
		d.decimation++
		if d.decimation < 8 {
			continue
		}
		d.decimation = 0
		v := d.lowpass
		if d.mode == i18n.Source("text.6ea56fae9eac") || d.mode == i18n.Source("text.fb09c8f399c7") {
			if code, inverted, confidence, ok := d.dcs.process(v); ok {
				suffix := "N"
				if inverted {
					suffix = "I"
				}
				d.confirm(i18n.Source("text.fb09c8f399c7"), fmt.Sprintf("%03o%s", code, suffix), confidence)
			}
		}
		if d.mode == i18n.Source("text.6ea56fae9eac") || d.mode == i18n.Source("text.74108b47eb26") {
			d.ctcss = append(d.ctcss, v)
		}
		if len(d.ctcss) >= 1800 {
			if tone, confidence, ok := detectCTCSS(d.ctcss); ok {
				d.confirm(i18n.Source("text.74108b47eb26"), fmt.Sprintf(i18n.Source("text.8157dbd8ea72"), tone), confidence)
			} else if d.status.Kind == i18n.Source("text.74108b47eb26") {
				d.fade()
			}
			d.ctcss = d.ctcss[:0]
		}
	}
}

func (d *SubtoneDetector) confirm(kind, value string, confidence float32) {
	key := kind + value
	if key == d.candidate {
		d.confirmations++
	} else {
		d.candidate, d.confirmations = key, 1
	}
	if d.confirmations >= 2 || kind == i18n.Source("text.fb09c8f399c7") {
		d.status = SubtoneStatus{Mode: d.mode, Kind: kind, Value: value, Confidence: confidence, Detected: true}
	}
}
func (d *SubtoneDetector) fade() {
	d.status.Confidence *= .65
	if d.status.Confidence < .2 {
		d.status = SubtoneStatus{Mode: d.mode}
	}
}

func detectCTCSS(samples []float32) (float64, float32, bool) {
	powers := make([]float64, len(ctcssTones))
	total := float64(0)
	for _, s := range samples {
		total += float64(s * s)
	}
	if total < 1e-7 {
		return 0, 0, false
	}
	for i, frequency := range ctcssTones {
		coefficient := 2 * math.Cos(2*math.Pi*frequency/6000)
		q1, q2 := 0.0, 0.0
		for _, sample := range samples {
			q0 := coefficient*q1 - q2 + float64(sample)
			q2, q1 = q1, q0
		}
		powers[i] = q1*q1 + q2*q2 - coefficient*q1*q2
	}
	ordered := append([]float64(nil), powers...)
	sort.Float64s(ordered)
	noise := ordered[len(ordered)/2] + 1e-12
	best, second := 0, 0
	for i := range powers {
		if powers[i] > powers[best] {
			second, best = best, i
		} else if i != best && powers[i] > powers[second] {
			second = i
		}
	}
	ratio := powers[best] / max(noise, powers[second]*.35)
	confidence := float32(min(max((ratio-2)/10, 0), 1))
	return ctcssTones[best], confidence, ratio > 5 && powers[best] > total*12
}

type dcsDetector struct {
	bitPhase, bitsPerSample float64
	samples                 []float32
	eqIndex                 int
	mid, previous           float32
	normal, inverted        uint32
	last                    uint
	repeats                 int
}

func (d *dcsDetector) init() {
	d.bitsPerSample = 134.4 / 6000
	d.samples = make([]float32, int(math.Round(6000/134.4*23)))
}
func (d *dcsDetector) process(sample float32) (uint, bool, float32, bool) {
	d.samples[d.eqIndex] = sample
	d.eqIndex++
	if d.eqIndex == len(d.samples) {
		low, high := d.samples[0], d.samples[0]
		for _, v := range d.samples[1:] {
			if v < low {
				low = v
			}
			if v > high {
				high = v
			}
		}
		d.mid = (low + high) / 2
		d.eqIndex = 0
	}
	if (d.previous < d.mid && sample >= d.mid) || (d.previous > d.mid && sample <= d.mid) {
		d.bitPhase = 0
	}
	d.previous = sample
	previous := d.bitPhase
	d.bitPhase += d.bitsPerSample
	if previous < .5 && d.bitPhase >= .5 {
		bit := uint32(0)
		if sample > d.mid {
			bit = 1
		}
		d.normal = (bit << 23) | (d.normal >> 1)
		d.inverted = ((1 - bit) << 23) | (d.inverted >> 1)
		for polarity, word := range []uint32{d.normal, d.inverted} {
			candidate := word & 0x7fffff
			if ((candidate >> 9) & 7) != 4 {
				continue
			}
			corrected, errors, ok := golayCorrect(candidate)
			if !ok {
				continue
			}
			code := uint(corrected & 0x1ff)
			if !standardDCS(code) {
				continue
			}
			if code == d.last {
				d.repeats++
			} else {
				d.last = code
				d.repeats = 1
			}
			if d.repeats >= 2 {
				return code, polarity == 1, 1 - float32(errors)*.2, true
			}
		}
	}
	if d.bitPhase > 1 {
		d.bitPhase -= 1
	}
	return 0, false, 0, false
}

var golayH = [11]uint32{0b10000000000101001001111, 0b01000000000111101101000, 0b00100000000011110110100, 0b00010000000001111011010, 0b00001000000000111101101, 0b00000100000101010111001, 0b00000010000111100010011, 0b00000001000110111000110, 0b00000000100011011100011, 0b00000000010100100111110, 0b00000000001010010011111}

func golaySyndrome(word uint32) uint32 {
	var s uint32
	for i, h := range golayH {
		s |= uint32(bitsParity(word&h)) << uint(10-i)
	}
	return s
}
func bitsParity(v uint32) int {
	v ^= v >> 16
	v ^= v >> 8
	v ^= v >> 4
	v &= 15
	return int((0x6996 >> v) & 1)
}
func golayCorrect(word uint32) (uint32, int, bool) {
	if golaySyndrome(word) == 0 {
		return word, 0, true
	}
	for i := 0; i < 23; i++ {
		w := word ^ (1 << i)
		if golaySyndrome(w) == 0 {
			return w, 1, true
		}
	}
	for i := 0; i < 23; i++ {
		for j := i + 1; j < 23; j++ {
			w := word ^ (1 << i) ^ (1 << j)
			if golaySyndrome(w) == 0 {
				return w, 2, true
			}
		}
	}
	for i := 0; i < 23; i++ {
		for j := i + 1; j < 23; j++ {
			for k := j + 1; k < 23; k++ {
				w := word ^ (1 << i) ^ (1 << j) ^ (1 << k)
				if golaySyndrome(w) == 0 {
					return w, 3, true
				}
			}
		}
	}
	return word, 0, false
}
func standardDCS(code uint) bool {
	for _, v := range []uint{023, 025, 026, 031, 032, 036, 043, 047, 051, 053, 054, 065, 071, 072, 073, 074, 114, 115, 116, 122, 125, 131, 132, 134, 143, 145, 152, 155, 156, 162, 165, 172, 174, 205, 212, 223, 225, 226, 243, 244, 245, 246, 251, 252, 255, 261, 263, 265, 266, 271, 274, 306, 311, 315, 325, 331, 332, 343, 346, 351, 356, 364, 365, 371, 411, 412, 413, 423, 431, 432, 445, 446, 452, 454, 455, 462, 464, 465, 466, 503, 506, 516, 523, 526, 532, 546, 565, 606, 612, 624, 627, 631, 632, 654, 662, 664, 703, 712, 723, 731, 732, 734, 743, 754} {
		if code == v {
			return true
		}
	}
	return false
}
