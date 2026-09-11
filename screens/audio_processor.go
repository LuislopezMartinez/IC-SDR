package screens

import (
	"math"
	"sync"
)

var audioEQFrequencies = [5]float64{120, 300, 800, 2000, 5000}

type audioBiquad struct{ b0, b1, b2, a1, a2, z1, z2 float32 }

func (f *audioBiquad) configure(frequency, gainDB float64) {
	a := math.Pow(10, gainDB/40)
	w := 2 * math.Pi * frequency / audioSampleRate
	alpha, cosine := math.Sin(w)/2, math.Cos(w)
	a0 := 1 + alpha/a
	f.b0, f.b1, f.b2 = float32((1+alpha*a)/a0), float32((-2*cosine)/a0), float32((1-alpha*a)/a0)
	f.a1, f.a2 = float32((-2*cosine)/a0), float32((1-alpha/a)/a0)
}

func (f *audioBiquad) process(input float32) float32 {
	output := input*f.b0 + f.z1
	f.z1 = input*f.b1 + f.z2 - f.a1*output
	f.z2 = input*f.b2 - f.a2*output
	return output
}

type AudioProcessor struct {
	mu                                          sync.Mutex
	lowCut, highCut                             int
	eqEnabled                                   bool
	eqGains                                     [5]float32
	eq                                          [5]audioBiquad
	profile                                     string
	previousInput, highpass, lowpass1, lowpass2 float32
	ring                                        [512]float32
	ringWrite, ringCount                        int
}

func NewAudioProcessor() *AudioProcessor {
	p := &AudioProcessor{lowCut: 100, highCut: 4000, eqEnabled: true, profile: "NORMAL"}
	p.configureEQ()
	return p
}

func (p *AudioProcessor) Configure(lowCut, highCut int, eqEnabled bool, gains [5]float32, profile string) {
	p.mu.Lock()
	p.lowCut, p.highCut = min(max(lowCut, 20), 4000), min(max(highCut, 250), 16000)
	p.eqEnabled, p.eqGains, p.profile = eqEnabled, gains, profile
	p.configureEQ()
	p.mu.Unlock()
}

func (p *AudioProcessor) configureEQ() {
	for i := range p.eq {
		p.eq[i].configure(audioEQFrequencies[i], float64(p.eqGains[i]))
	}
}

func (p *AudioProcessor) Process(samples []float32) {
	p.mu.Lock()
	highpassR := float32(math.Exp(-2 * math.Pi * float64(p.lowCut) / audioSampleRate))
	lowpassAlpha := float32(1 - math.Exp(-2*math.Pi*float64(p.highCut)/audioSampleRate))
	for i, input := range samples {
		p.highpass = input - p.previousInput + highpassR*p.highpass
		p.previousInput = input
		p.lowpass1 += lowpassAlpha * (p.highpass - p.lowpass1)
		p.lowpass2 += lowpassAlpha * (p.lowpass1 - p.lowpass2)
		value := p.lowpass2
		if p.eqEnabled {
			for band := range p.eq {
				value = p.eq[band].process(value)
			}
		}
		switch p.profile {
		case "SOFT":
			value *= .82
		case "STRONG":
			value = float32(math.Tanh(float64(value*1.8))) * .92
		default:
			value = float32(math.Tanh(float64(value*1.2))) * .94
		}
		value = min(max(value, -.98), .98)
		samples[i] = value
		p.ring[p.ringWrite] = value
		p.ringWrite = (p.ringWrite + 1) % len(p.ring)
		p.ringCount = min(p.ringCount+1, len(p.ring))
	}
	p.mu.Unlock()
}

func (p *AudioProcessor) Spectrum(destination []float32) {
	p.mu.Lock()
	count, snapshot, write := p.ringCount, p.ring, p.ringWrite
	p.mu.Unlock()
	if count < 128 {
		for i := range destination {
			destination[i] = -80
		}
		return
	}
	for bin := range destination {
		frequency := 16000 * float64(bin) / float64(max(len(destination)-1, 1))
		var real, imaginary float64
		for n := 0; n < count; n++ {
			sample := float64(snapshot[(write-count+n+len(snapshot))%len(snapshot)])
			window := .5 - .5*math.Cos(2*math.Pi*float64(n)/float64(count-1))
			phase := -2 * math.Pi * frequency * float64(n) / audioSampleRate
			real += sample * window * math.Cos(phase)
			imaginary += sample * window * math.Sin(phase)
		}
		magnitude := 2 * math.Hypot(real, imaginary) / float64(count)
		destination[bin] = float32(max(20*math.Log10(magnitude+1e-7), -80))
	}
}
