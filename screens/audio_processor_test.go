package screens

import (
	"math"
	"testing"
)

func TestAudioProcessorProducesFiniteBoundedOutput(t *testing.T) {
	processor := NewAudioProcessor()
	processor.Configure(100, 4000, true, [5]float32{3, -2, 1.5, 0, -1}, "STRONG")
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
