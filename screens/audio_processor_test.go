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

func TestTETRAPOLProcessingIsIndependentFromGenericAudioEQ(t *testing.T) {
	plain := NewAudioProcessor()
	configured := NewAudioProcessor()
	configured.Configure(350, 1800, true, [5]float32{-12, 10, -8, 12, -10}, "FUERTE")

	a := make([]float32, audioSampleRate/5)
	b := make([]float32, len(a))
	for index := range a {
		a[index] = float32(.25 * math.Sin(2*math.Pi*1750*float64(index)/audioSampleRate))
		b[index] = a[index]
	}
	plain.ProcessTETRAPOL(a)
	configured.ProcessTETRAPOL(b)
	for index := range a {
		if math.Abs(float64(a[index]-b[index])) > 1e-6 {
			t.Fatalf("generic EQ leaked into TETRAPOL sample %d: %v != %v", index, a[index], b[index])
		}
		if math.IsNaN(float64(a[index])) || math.IsInf(float64(a[index]), 0) || a[index] < -.98 || a[index] > .98 {
			t.Fatalf("invalid TETRAPOL sample %d: %v", index, a[index])
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
	if ratio >= .68 {
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
	if ratio >= .72 {
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

func TestAudioNoiseReductionPreservesWeakVoiceHarmonics(t *testing.T) {
	measure := func(enabled bool) float64 {
		processor := NewAudioProcessor()
		processor.Configure(100, 4000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
		processor.ConfigureNoiseReduction(enabled, 70)
		samples := make([]float32, audioSampleRate*2)
		seed := uint32(91)
		for index := range samples {
			seed = seed*1664525 + 1013904223
			noise := (float32(seed>>8)/float32(1<<24)*2 - 1) * .09
			timeSeconds := float64(index) / audioSampleRate
			envelope := .55 + .45*math.Sin(2*math.Pi*3.2*timeSeconds)
			voice := envelope * (.032*math.Sin(2*math.Pi*180*timeSeconds) + .024*math.Sin(2*math.Pi*540*timeSeconds) + .017*math.Sin(2*math.Pi*1260*timeSeconds))
			samples[index] = noise + float32(voice)
		}
		processor.Process(samples)
		// Correlation with the known harmonic pattern is a compact proxy for
		// retaining weak, voiced speech beneath receiver hiss.
		correlation := 0.0
		start := len(samples) - audioSampleRate/2
		for index, sample := range samples[start:] {
			timeSeconds := float64(start+index-noiseHopSize) / audioSampleRate
			reference := math.Sin(2*math.Pi*180*timeSeconds) + .75*math.Sin(2*math.Pi*540*timeSeconds) + .53*math.Sin(2*math.Pi*1260*timeSeconds)
			correlation += float64(sample) * reference
		}
		return math.Abs(correlation)
	}
	ratio := measure(true) / measure(false)
	if ratio < .68 {
		t.Fatalf("NR removed too much weak voice structure: ratio %.3f", ratio)
	}
}

func TestAudioNoiseReductionSwitchesWithoutDiscontinuity(t *testing.T) {
	processor := NewAudioProcessor()
	processor.Configure(100, 4000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
	processor.ConfigureNoiseReduction(false, 70)
	phase := 0
	previous := float32(0)
	for block := 0; block < 80; block++ {
		if block == 20 {
			processor.ConfigureNoiseReduction(true, 70)
		}
		if block == 55 {
			processor.ConfigureNoiseReduction(false, 70)
		}
		samples := make([]float32, 256)
		for index := range samples {
			samples[index] = float32(.25 * math.Sin(2*math.Pi*900*float64(phase)/audioSampleRate))
			phase++
		}
		processor.Process(samples)
		for _, sample := range samples {
			if jump := absFloat32(sample - previous); jump > .14 {
				t.Fatalf("NR transition produced a discontinuity of %.3f", jump)
			}
			previous = sample
		}
	}
}

func TestAudioNoiseReductionClosesAfterVoiceEnds(t *testing.T) {
	measureTail := func(enabled bool) float64 {
		processor := NewAudioProcessor()
		processor.Configure(100, 4000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
		processor.ConfigureNoiseReduction(enabled, 70)
		samples := make([]float32, audioSampleRate*2)
		seed := uint32(211)
		for index := range samples {
			seed = seed*1664525 + 1013904223
			noise := (float32(seed>>8)/float32(1<<24)*2 - 1) * .12
			samples[index] = noise
			if index < audioSampleRate {
				timeSeconds := float64(index) / audioSampleRate
				samples[index] += float32(.12*math.Sin(2*math.Pi*220*timeSeconds) + .07*math.Sin(2*math.Pi*660*timeSeconds))
			}
		}
		processor.Process(samples)
		power := 0.0
		start := len(samples) - audioSampleRate/4
		for _, sample := range samples[start:] {
			power += float64(sample * sample)
		}
		return math.Sqrt(power / float64(len(samples)-start))
	}
	ratio := measureTail(true) / measureTail(false)
	if ratio >= .68 {
		t.Fatalf("NR stayed open after voice ended: residual noise ratio %.3f", ratio)
	}
}

func TestNoiseReductionFastIntegrationClosesEarlier(t *testing.T) {
	measure := func(speed string) float32 {
		processor := NewAudioProcessor()
		processor.Configure(100, 4000, false, [5]float32{}, i18n.Source("text.db2cb3fe28e2"))
		processor.ConfigureNoiseReduction(true, 70)
		processor.ConfigureNoiseReductionSpeed(speed)
		samples := make([]float32, audioSampleRate+audioSampleRate/8)
		seed := uint32(313)
		for index := range samples {
			seed = seed*1664525 + 1013904223
			samples[index] = (float32(seed>>8)/float32(1<<24)*2 - 1) * .10
			if index < audioSampleRate {
				timeSeconds := float64(index) / audioSampleRate
				samples[index] += float32(.16*math.Sin(2*math.Pi*240*timeSeconds) + .09*math.Sin(2*math.Pi*720*timeSeconds))
			}
		}
		processor.Process(samples)
		return processor.NoiseReductionStatus().ReductionDB
	}
	slow, fast := measure("slow"), measure("fast")
	if fast <= slow+.5 {
		t.Fatalf("fast NR integration did not close earlier: slow %.2f dB fast %.2f dB", slow, fast)
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
