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

func TestAudioSpectrumRefreshTracksDisplayFrameRate(t *testing.T) {
	if audioSpectrumRefreshSeconds > 1.0/55.0 {
		t.Fatalf("audio spectrum refresh interval %.4f s is not real-time", audioSpectrumRefreshSeconds)
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

func TestAudioNotchAttenuatesSelectedToneAndPreservesAdjacentAudio(t *testing.T) {
	measure := func(frequency float64, enabled bool) float64 {
		processor := NewAudioProcessor()
		processor.Configure(20, 16000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
		processor.ConfigureNotch(enabled, 1000, 120, -40)
		samples := make([]float32, audioSampleRate)
		for index := range samples {
			samples[index] = float32(.4 * math.Sin(2*math.Pi*frequency*float64(index)/audioSampleRate))
		}
		processor.Process(samples)
		var power float64
		for _, sample := range samples[audioSampleRate/2:] {
			power += float64(sample * sample)
		}
		return math.Sqrt(power / (audioSampleRate / 2))
	}
	selectedRatio := measure(1000, true) / measure(1000, false)
	if selectedRatio > .04 {
		t.Fatalf("notch retained too much selected tone: ratio %.4f", selectedRatio)
	}
	adjacentRatio := measure(2000, true) / measure(2000, false)
	if adjacentRatio < .85 {
		t.Fatalf("notch removed adjacent audio: ratio %.4f", adjacentRatio)
	}
}

func TestAudioNoiseReductionAttenuatesStationaryNoise(t *testing.T) {
	measure := func(enabled bool) float64 {
		processor := NewAudioProcessor()
		processor.Configure(20, 4000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
		processor.ConfigureNoiseReduction(enabled, 70)
		samples := make([]float32, audioSampleRate)
		seed := uint32(1)
		for index := range samples {
			seed = seed*1664525 + 1013904223
			samples[index] = (float32(seed>>8)/float32(1<<24)*2 - 1) * .18
		}
		processor.Process(samples)
		var power float64
		for _, sample := range samples[audioSampleRate/2:] {
			power += float64(sample * sample)
		}
		return math.Sqrt(power / (audioSampleRate / 2))
	}
	ratio := measure(true) / measure(false)
	if ratio >= .82 {
		t.Fatalf("NR retained too much stationary noise: ratio %.3f", ratio)
	}
}

func TestRecommendedNoiseReductionIsAudible(t *testing.T) {
	measure := func(enabled bool) float64 {
		processor := NewAudioProcessor()
		processor.Configure(100, 4000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
		processor.ConfigureNoiseReduction(enabled, 50)
		samples := make([]float32, audioSampleRate*2)
		seed := uint32(17)
		for index := range samples {
			seed = seed*1664525 + 1013904223
			white := (float32(seed>>8)/float32(1<<24)*2 - 1) * .16
			// Add slow amplitude variation to approximate receiver hiss rather
			// than validating only an ideal, perfectly stationary source.
			envelope := float32(.78 + .22*math.Sin(2*math.Pi*1.7*float64(index)/audioSampleRate))
			samples[index] = white * envelope
		}
		processor.Process(samples)
		var power float64
		for _, sample := range samples[len(samples)-audioSampleRate/2:] {
			power += float64(sample * sample)
		}
		return math.Sqrt(power / (audioSampleRate / 2))
	}
	ratio := measure(true) / measure(false)
	if ratio >= .82 {
		t.Fatalf("recommended NR setting is barely audible: ratio %.3f", ratio)
	}
}

func TestAudioNoiseReductionProducesFiniteOutput(t *testing.T) {
	processor := NewAudioProcessor()
	processor.ConfigureNoiseReduction(true, 100)
	samples := make([]float32, 8192)
	for index := range samples {
		samples[index] = float32(.35 * math.Sin(2*math.Pi*900*float64(index)/audioSampleRate))
	}
	processor.Process(samples)
	for index, sample := range samples {
		if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) || sample < -.98 || sample > .98 {
			t.Fatalf("invalid NR output sample %d: %v", index, sample)
		}
	}
}

func TestAudioNoiseReductionPreservesSustainedCarrier(t *testing.T) {
	measure := func(enabled bool) float64 {
		processor := NewAudioProcessor()
		processor.Configure(20, 4000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
		processor.ConfigureNoiseReduction(enabled, 85)
		samples := make([]float32, audioSampleRate*2)
		for index := range samples {
			samples[index] = float32(.25 * math.Sin(2*math.Pi*1000*float64(index)/audioSampleRate))
		}
		processor.Process(samples)
		var power float64
		for _, sample := range samples[len(samples)-audioSampleRate/2:] {
			power += float64(sample * sample)
		}
		return math.Sqrt(power / (audioSampleRate / 2))
	}
	ratio := measure(true) / measure(false)
	if ratio < .75 {
		t.Fatalf("NR treated a sustained carrier as noise: ratio %.3f", ratio)
	}
}

func TestNoiseReductionModePolicy(t *testing.T) {
	for _, mode := range []string{"AM", i18n.Source("text.0896d612d497"), i18n.Source("text.61f0acff1735"), i18n.Source("text.6323db4948ad")} {
		if !noiseReductionMode(mode) {
			t.Fatalf("NR unexpectedly disabled for %s", mode)
		}
	}
	for _, mode := range []string{"CW", i18n.Source("text.6b742bac3eb4"), i18n.Source("text.2604864ce4d3")} {
		if noiseReductionMode(mode) {
			t.Fatalf("NR unexpectedly enabled for %s", mode)
		}
	}
}
