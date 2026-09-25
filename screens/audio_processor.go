package screens

import (
	"go-zero/internal/i18n"

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

type audioNotchFilter struct {
	b0, b1, b2, a1, a2           float32
	targetB0, targetB1, targetB2 float32
	targetA1, targetA2           float32
	z1, z2, mix, targetMix       float32
}

func (filter *audioNotchFilter) configure(enabled bool, frequencyHz, widthHz int, depthDB float32) {
	frequency := min(max(float64(frequencyHz), 80), audioSampleRate*.45)
	width := min(max(float64(widthHz), 20), 2000)
	q := max(frequency/width, .25)
	w := 2 * math.Pi * frequency / audioSampleRate
	alpha, cosine := math.Sin(w)/(2*q), math.Cos(w)
	a0 := 1 + alpha
	filter.targetB0 = float32(1 / a0)
	filter.targetB1 = float32(-2 * cosine / a0)
	filter.targetB2 = filter.targetB0
	filter.targetA1 = filter.targetB1
	filter.targetA2 = float32((1 - alpha) / a0)
	if filter.b0 == 0 {
		filter.b0, filter.b1, filter.b2 = filter.targetB0, filter.targetB1, filter.targetB2
		filter.a1, filter.a2 = filter.targetA1, filter.targetA2
	}
	filter.targetMix = 0
	if enabled {
		depthDB = min(max(depthDB, -60), -6)
		filter.targetMix = 1 - float32(math.Pow(10, float64(depthDB)/20))
	}
}

func (filter *audioNotchFilter) process(input float32) float32 {
	// Roughly 10 ms coefficient/mix transition prevents zipper noise while the
	// notch is dragged over the live spectrum.
	const smoothing = float32(.002)
	filter.b0 += smoothing * (filter.targetB0 - filter.b0)
	filter.b1 += smoothing * (filter.targetB1 - filter.b1)
	filter.b2 += smoothing * (filter.targetB2 - filter.b2)
	filter.a1 += smoothing * (filter.targetA1 - filter.a1)
	filter.a2 += smoothing * (filter.targetA2 - filter.a2)
	filter.mix += smoothing * (filter.targetMix - filter.mix)
	filtered := input*filter.b0 + filter.z1
	filter.z1 = input*filter.b1 + filter.z2 - filter.a1*filtered
	filter.z2 = input*filter.b2 - filter.a2*filtered
	return input + filter.mix*(filtered-input)
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
	notch                                       audioNotchFilter
	profile                                     string
	previousInput, highpass, lowpass1, lowpass2 float32
	limiterGain                                 float32
	ring                                        [512]float32
	preNotchRing                                [512]float32
	spectrumWindow                              [512]float64
	ringWrite, ringCount                        int
}

func NewAudioProcessor() *AudioProcessor {
	p := &AudioProcessor{lowCut: 100, highCut: 4000, eqEnabled: true, profile: i18n.Source("text.db2cb3fe28e2"), limiterGain: 1}
	for index := range p.spectrumWindow {
		p.spectrumWindow[index] = .5 - .5*math.Cos(2*math.Pi*float64(index)/float64(len(p.spectrumWindow)-1))
	}
	p.configureEQ()
	p.notch.configure(false, 1000, 120, -35)
	return p
}

func (p *AudioProcessor) Configure(lowCut, highCut int, eqEnabled bool, gains [5]float32, profile string) {
	p.mu.Lock()
	p.lowCut, p.highCut = min(max(lowCut, 20), 4000), min(max(highCut, 250), 16000)
	p.eqEnabled, p.eqGains, p.profile = eqEnabled, gains, profile
	p.configureEQ()
	p.mu.Unlock()
}

func (p *AudioProcessor) ConfigureNotch(enabled bool, frequencyHz, widthHz int, depthDB float32) {
	p.mu.Lock()
	p.notch.configure(enabled, frequencyHz, widthHz, depthDB)
	p.mu.Unlock()
}

func (p *AudioProcessor) configureEQ() {
	for i := range p.eq {
		p.eq[i].configure(audioEQFrequencies[i], float64(p.eqGains[i]))
	}
}

func (p *AudioProcessor) Process(samples []float32) {
	p.process(samples, false)
}

func (p *AudioProcessor) ProcessWideFM(samples []float32) {
	p.process(samples, true)
}

func (p *AudioProcessor) process(samples []float32, wideFM bool) {
	p.mu.Lock()
	lowCut, highCut := p.lowCut, p.highCut
	if wideFM {
		// The default 100 Hz..4 kHz speech passband is appropriate for radio
		// communications but makes broadcast FM sound like a telephone.
		lowCut, highCut = 30, max(highCut, 15_000)
	}
	highpassR := float32(math.Exp(-2 * math.Pi * float64(lowCut) / audioSampleRate))
	lowpassAlpha := float32(1 - math.Exp(-2*math.Pi*float64(highCut)/audioSampleRate))
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
		p.preNotchRing[p.ringWrite] = value
		value = p.notch.process(value)
		switch p.profile {
		case i18n.Source("text.692233b9c713"):
			value *= .82
		case i18n.Source("text.e5ccf011d642"):
			value = float32(math.Tanh(float64(value*1.8))) * .92
		}
		// Demodulators already apply their own speech compressor. Applying tanh
		// again in the default listening profile flattened voice peaks and made
		// strong stations sound saturated. This final stage now stays linear and
		// only catches filter/EQ overshoot close to full scale.
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			value = 0
		}
		const ceiling = float32(.98)
		desiredGain := float32(1)
		if magnitude := absFloat32(value); magnitude > ceiling {
			desiredGain = ceiling / magnitude
		}
		if desiredGain < p.limiterGain {
			p.limiterGain = desiredGain
		} else {
			// About 100 ms release at 48 kHz avoids pumping after a single peak.
			p.limiterGain += float32(1-math.Exp(-1/(.1*audioSampleRate))) * (desiredGain - p.limiterGain)
		}
		value = min(max(value*p.limiterGain, -ceiling), ceiling)
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
	calculateAudioSpectrum(destination, snapshot, count, write, p.spectrumWindow)
}

func (p *AudioProcessor) SpectrumBeforeNotch(destination []float32) {
	p.mu.Lock()
	count, snapshot, write := p.ringCount, p.preNotchRing, p.ringWrite
	p.mu.Unlock()
	calculateAudioSpectrum(destination, snapshot, count, write, p.spectrumWindow)
}

func calculateAudioSpectrum(destination []float32, snapshot [512]float32, count, write int, windowTable [512]float64) {
	if count < 128 {
		for i := range destination {
			destination[i] = -80
		}
		return
	}
	for bin := range destination {
		frequency := 16000 * float64(bin) / float64(max(len(destination)-1, 1))
		var real, imaginary float64
		phaseStep := -2 * math.Pi * frequency / audioSampleRate
		stepReal, stepImaginary := math.Cos(phaseStep), math.Sin(phaseStep)
		oscillatorReal, oscillatorImaginary := 1.0, 0.0
		for n := 0; n < count; n++ {
			sample := float64(snapshot[(write-count+n+len(snapshot))%len(snapshot)])
			window := windowTable[n]
			if count != len(snapshot) {
				window = .5 - .5*math.Cos(2*math.Pi*float64(n)/float64(count-1))
			}
			windowed := sample * window
			real += windowed * oscillatorReal
			imaginary += windowed * oscillatorImaginary
			nextReal := oscillatorReal*stepReal - oscillatorImaginary*stepImaginary
			oscillatorImaginary = oscillatorReal*stepImaginary + oscillatorImaginary*stepReal
			oscillatorReal = nextReal
		}
		magnitude := 2 * math.Hypot(real, imaginary) / float64(count)
		destination[bin] = float32(max(20*math.Log10(magnitude+1e-7), -80))
	}
}
