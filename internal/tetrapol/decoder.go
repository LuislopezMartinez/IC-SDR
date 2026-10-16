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
	CarrierOffsetHz, AFCErrorHz, SamplesPerSymbol, TimingError      float64
	AFCState                                                        string
	Samples, Symbols                                                uint64
}
type bitSink struct{ write func([]byte) }

type Decoder struct {
	mu                                               sync.RWMutex
	rate                                             float64
	enabled                                          bool
	previousRaw, previousCorrected                   complex128
	havePreviousRaw, havePreviousCorrected           bool
	level, deviation                                 float64
	samples, symbols                                 uint64
	symbolClock, symbolFrequency, rawSymbolFrequency float64
	symbolSamples                                    int
	afcEnabled                                       bool
	afcPhase, afcOffset, afcError                    float64
	carrierAverage                                   float64
	carrierSamples                                   uint64
	frequencyHigh, frequencyLow                      float64
	frequencyLevelsReady                             bool
	timingError, lastDiscriminator                   float64
	haveDiscriminator                                bool
	afcStableSymbols                                 int
	bits                                             atomic.Pointer[bitSink]
	channel                                          *channelizer
}

func New(rate float64) *Decoder {
	return &Decoder{rate: rate, afcEnabled: true, channel: newChannelizer(rate)}
}

func (d *Decoder) Configure(on bool) {
	d.mu.Lock()
	d.enabled = on
	d.havePreviousRaw, d.havePreviousCorrected, d.haveDiscriminator = false, false, false
	d.channel.reset()
	d.symbolClock, d.symbolFrequency, d.rawSymbolFrequency, d.symbolSamples = 0, 0, 0, 0
	d.afcPhase, d.afcOffset, d.afcError = 0, 0, 0
	d.carrierAverage, d.carrierSamples = 0, 0
	d.frequencyHigh, d.frequencyLow, d.frequencyLevelsReady = 0, 0, false
	d.timingError, d.lastDiscriminator, d.afcStableSymbols = 0, 0, 0
	d.mu.Unlock()
}

func (d *Decoder) SetAFCEnabled(enabled bool) {
	d.mu.Lock()
	d.afcEnabled = enabled
	if !enabled {
		d.afcPhase, d.afcOffset, d.afcError, d.afcStableSymbols = 0, 0, 0, 0
	}
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
		rawFrequency := 0.0
		if d.havePreviousRaw {
			p := v * complex(real(d.previousRaw), -imag(d.previousRaw))
			rawFrequency = math.Atan2(imag(p), real(p)) * workRate / (2 * math.Pi)
			dev += rawFrequency
			// TETRAPOL scrambling makes the two GMSK polarities balanced over
			// time. A robust, slow mean therefore estimates carrier offset
			// without confusing either symbol deviation with frequency error.
			if magnitude := real(v)*real(v) + imag(v)*imag(v); magnitude > 1e-10 && math.Abs(rawFrequency) < 6_000 {
				if d.carrierSamples == 0 {
					d.carrierAverage = rawFrequency
				} else {
					d.carrierAverage += .0002 * (rawFrequency - d.carrierAverage)
				}
				d.carrierSamples++
			}
		}
		d.previousRaw, d.havePreviousRaw = v, true

		correction := 0.0
		if d.afcEnabled {
			correction = d.afcOffset
		}
		rotation := complex(math.Cos(d.afcPhase), -math.Sin(d.afcPhase))
		d.afcPhase = math.Mod(d.afcPhase+2*math.Pi*correction/workRate, 2*math.Pi)
		corrected := v * rotation
		if d.havePreviousCorrected {
			p := corrected * complex(real(d.previousCorrected), -imag(d.previousCorrected))
			frequency := math.Atan2(imag(p), real(p)) * workRate / (2 * math.Pi)
			d.symbolFrequency += frequency
			d.rawSymbolFrequency += rawFrequency
			d.symbolSamples++

			// A GMSK transition is close to a symbol boundary. Nudging the
			// integrate-and-dump clock at strong zero crossings compensates
			// sample-rate error without making noise move the clock.
			if d.haveDiscriminator && frequency*d.lastDiscriminator < 0 && math.Abs(frequency-d.lastDiscriminator) > 300 {
				phaseError := d.symbolClock
				if phaseError > .5 {
					phaseError--
				}
				d.timingError = .9*d.timingError + .1*phaseError
				d.symbolClock -= .08 * phaseError
				if d.symbolClock < 0 {
					d.symbolClock++
				}
			}
			d.lastDiscriminator, d.haveDiscriminator = frequency, true
			d.symbolClock += SymbolRate / workRate
			if d.symbolClock >= 1 {
				average := d.symbolFrequency / float64(d.symbolSamples)
				rawAverage := d.rawSymbolFrequency / float64(d.symbolSamples)
				d.updateAFC(rawAverage)
				if average >= 0 {
					output = append(output, 1)
				} else {
					output = append(output, 0)
				}
				d.symbolClock -= 1
				d.symbolFrequency, d.rawSymbolFrequency, d.symbolSamples = 0, 0, 0
			}
		}
		d.previousCorrected, d.havePreviousCorrected = corrected, true
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

func (d *Decoder) updateAFC(frequency float64) {
	if !d.afcEnabled {
		return
	}
	if !d.frequencyLevelsReady {
		if d.symbols == 0 && d.frequencyHigh == 0 && d.frequencyLow == 0 {
			d.frequencyHigh, d.frequencyLow = frequency, frequency
		} else {
			d.frequencyHigh = max(d.frequencyHigh, frequency)
			d.frequencyLow = min(d.frequencyLow, frequency)
		}
		if d.frequencyHigh-d.frequencyLow >= 500 {
			d.frequencyLevelsReady = true
		}
	} else {
		midpoint := (d.frequencyHigh + d.frequencyLow) * .5
		const levelAlpha = .04
		if frequency >= midpoint {
			d.frequencyHigh += levelAlpha * (frequency - d.frequencyHigh)
		} else {
			d.frequencyLow += levelAlpha * (frequency - d.frequencyLow)
		}
	}
	if !d.frequencyLevelsReady {
		return
	}
	if d.carrierSamples < uint64(d.channel.outputRate/20) {
		return
	}
	target := d.carrierAverage
	d.afcError = target - d.afcOffset
	gain := .08
	if d.afcStableSymbols >= 80 {
		gain = .025
	}
	d.afcOffset += gain * d.afcError
	d.afcOffset = max(-4_000.0, min(4_000.0, d.afcOffset))
	if math.Abs(d.afcError) < 75 {
		d.afcStableSymbols++
	} else {
		d.afcStableSymbols = max(0, d.afcStableSymbols-2)
	}
}

func (d *Decoder) Snapshot() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()
	separation := d.frequencyHigh - d.frequencyLow
	quality := math.Min(100, math.Max(0, separation/20))
	state := "OFF"
	if d.afcEnabled {
		state = "ACQUIRE"
		if d.frequencyLevelsReady {
			state = "TRACK"
		}
		if d.afcStableSymbols >= 80 {
			state = "LOCKED"
		}
	}
	return Status{
		Running: d.enabled, InputRate: d.rate, SymbolRate: SymbolRate,
		LevelDBFS: d.level, FrequencyDeviationHz: d.deviation, Quality: quality,
		CarrierOffsetHz: d.afcOffset, AFCErrorHz: d.afcError,
		SamplesPerSymbol: d.channel.outputRate / SymbolRate, TimingError: d.timingError,
		AFCState: state, Samples: d.samples, Symbols: d.symbols,
	}
}
