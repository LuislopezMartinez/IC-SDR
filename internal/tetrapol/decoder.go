// Package tetrapol contains the passive physical-layer monitor used by IC-SDR.
package tetrapol

import (
	"math"
	"sync"
	"sync/atomic"
)

const SymbolRate = 8000.0

type Status struct {
	Running                                                         bool
	InputRate, SymbolRate, LevelDBFS, FrequencyDeviationHz, Quality float64
	Samples, Symbols                                                uint64
}
type bitSink struct{ write func([]byte) }

type Decoder struct {
	mu                                            sync.RWMutex
	rate                                          float64
	enabled                                       bool
	previous                                      complex128
	havePrevious                                  bool
	level, deviation                              float64
	samples, symbols                              uint64
	symbolClock, symbolFrequency, carrierEstimate float64
	symbolSamples                                 int
	bits                                          atomic.Pointer[bitSink]
	channel                                       *channelizer
}

func New(rate float64) *Decoder { return &Decoder{rate: rate, channel: newChannelizer(rate)} }

func (d *Decoder) Configure(on bool) {
	d.mu.Lock()
	d.enabled = on
	d.havePrevious = false
	d.channel.reset()
	d.symbolClock, d.symbolFrequency, d.carrierEstimate, d.symbolSamples = 0, 0, 0, 0
	d.mu.Unlock()
}

func (d *Decoder) SetTuningOffset(offsetHz float64) {
	d.mu.Lock()
	d.channel.setOffset(offsetHz)
	d.mu.Unlock()
}

// SetBitSink receives hard GMSK symbols at 8 kBd. Its callback must return
// quickly because it runs in the SDR receive path.
func (d *Decoder) SetBitSink(write func([]byte)) {
	if write == nil {
		d.bits.Store(nil)
		return
	}
	d.bits.Store(&bitSink{write: write})
}

func (d *Decoder) ProcessIQ(iq []float32) {
	d.mu.Lock()
	if !d.enabled || len(iq) < 2 {
		d.mu.Unlock()
		return
	}
	baseband := d.channel.process(iq)
	if len(baseband) == 0 {
		d.mu.Unlock()
		return
	}
	var power, dev float64
	count := 0
	workRate := d.channel.outputRate
	output := make([]byte, 0, int(float64(len(baseband))*SymbolRate/workRate)+1)
	for _, v := range baseband {
		power += real(v)*real(v) + imag(v)*imag(v)
		if d.havePrevious {
			p := v * complex(real(d.previous), -imag(d.previous))
			frequency := math.Atan2(imag(p), real(p)) * workRate / (2 * math.Pi)
			dev += frequency
			d.symbolFrequency += frequency
			d.symbolSamples++
			d.symbolClock += SymbolRate / workRate
			if d.symbolClock >= 1 {
				average := d.symbolFrequency / float64(d.symbolSamples)
				d.carrierEstimate = .995*d.carrierEstimate + .005*average
				if average >= d.carrierEstimate {
					output = append(output, 1)
				} else {
					output = append(output, 0)
				}
				d.symbolClock -= 1
				d.symbolFrequency, d.symbolSamples = 0, 0
			}
		}
		d.previous, d.havePrevious = v, true
		count++
	}
	d.samples += uint64(len(iq) / 2)
	d.symbols += uint64(len(output))
	d.level = 20 * math.Log10(math.Sqrt(power/float64(count))+1e-12)
	d.deviation = dev / math.Max(1, float64(count-1))
	sink := d.bits.Load()
	d.mu.Unlock()
	if sink != nil && len(output) > 0 {
		sink.write(output)
	}
}
func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()
	quality := 100 - math.Min(100, math.Abs(d.deviation)/120)
	return Status{d.enabled, d.rate, SymbolRate, d.level, d.deviation, quality, d.samples, d.symbols}
}
