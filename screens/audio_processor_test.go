package screens

import (
	"go-zero/internal/i18n"

	"math"
	"testing"
)

func TestAudioProcessorProducesFiniteBoundedOutput(t *testing.T) {
	processor := NewAudioProcessor()
	processor.Configure(100, 4000, true, [5]float32{3, -2, 1.5, 0, -1}, "FUERTE")
	samples := make([]float32, 4096)
	for index := range samples {
		samples[index] = float32(.8 * math.Sin(2*math.Pi*1000*float64(index)/audioSampleRate))
	}
	processor.Process(samples)
	for index, sample := range samples {
		if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) || sample < -.98 || sample > .98 {
			t.Fatalf("invalid processed sample %d: %v", index, sample)
		}
	}
}

func TestAudioSpectrumFindsTone(t *testing.T) {
	processor := NewAudioProcessor()
	samples := make([]float32, 2048)
	for index := range samples {
		samples[index] = float32(.5 * math.Sin(2*math.Pi*1000*float64(index)/audioSampleRate))
	}
	processor.Process(samples)
	spectrum := make([]float32, 96)
	processor.Spectrum(spectrum)
	maximum := 0
	for index := range spectrum {
		if spectrum[index] > spectrum[maximum] {
			maximum = index
		}
	}
	frequency := 16000 * float64(maximum) / float64(len(spectrum)-1)
	if math.Abs(frequency-1000) > 200 {
		t.Fatalf("spectrum peak = %.0f Hz, want approximately 1000 Hz", frequency)
	}
}

func TestNormalAudioProfileRemainsLinearBelowLimiter(t *testing.T) {
	processor := NewAudioProcessor()
	processor.Configure(20, 16000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
	samples := make([]float32, 48000)
	for index := range samples {
		samples[index] = float32(.4 * math.Sin(2*math.Pi*1000*float64(index)/audioSampleRate))
	}
	processor.Process(samples)
	// Ignore filter startup and compare the positive peak after settling. A
	// second saturator would reduce this to roughly tanh(.48)*.94 = .418 and
	// introduce harmonics; the normal profile should preserve the filter level.
	peak := float32(0)
	for _, sample := range samples[24000:] {
		peak = max(peak, sample)
	}
	if math.Abs(float64(peak-.4)) > .015 {
		t.Fatalf("normal profile peak = %.4f, want linear level near .4", peak)
	}
}

func TestWideFMRetainsBroadcastAudioAboveSpeechBand(t *testing.T) {
	processor := NewAudioProcessor()
	processor.Configure(100, 4000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
	samples := make([]float32, 48000)
	for index := range samples {
		samples[index] = float32(.3 * math.Sin(2*math.Pi*10000*float64(index)/audioSampleRate))
	}
	processor.ProcessWideFM(samples)
	var power float64
	for _, sample := range samples[24000:] {
		power += float64(sample * sample)
	}
	rms := math.Sqrt(power / 24000)
	if rms < .08 {
		t.Fatalf("WFM 10 kHz programme audio was removed: RMS %.4f", rms)
	}
}
