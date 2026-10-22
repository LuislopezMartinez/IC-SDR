package sdr

import (
	"math"
	"testing"
)

func TestTETRAPOLAudioUpsamplerProducesExactContinuousRate(t *testing.T) {
	input := make([]float32, 320)
	for index := range input {
		input[index] = float32(math.Sin(2 * math.Pi * 1000 * float64(index) / tetrapolAudioInputRate))
	}
	var whole, split tetrapolAudioUpsampler
	want := whole.process(input)
	got := append(split.process(input[:137]), split.process(input[137:])...)
	if len(got) != len(input)*tetrapolAudioInterpolation || len(got) != len(want) {
		t.Fatalf("output length = %d, want %d", len(got), len(input)*tetrapolAudioInterpolation)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("block boundary changed sample %d: got %g, want %g", index, got[index], want[index])
		}
	}
}

func TestTETRAPOLAudioUpsamplerRejectsFirstImage(t *testing.T) {
	const toneHz = 1000.0
	input := make([]float32, tetrapolAudioInputRate)
	for index := range input {
		input[index] = float32(math.Sin(2 * math.Pi * toneHz * float64(index) / tetrapolAudioInputRate))
	}
	var upsampler tetrapolAudioUpsampler
	output := upsampler.process(input)
	// Skip the short FIR startup transient before measuring the desired 1 kHz
	// component and its nearest interpolation image at 7 kHz.
	output = output[tetrapolAudioPhaseTaps*tetrapolAudioInterpolation:]
	desired := toneMagnitude(output, toneHz, tetrapolAudioOutputRate)
	image := toneMagnitude(output, tetrapolAudioInputRate-toneHz, tetrapolAudioOutputRate)
	if desired < .9 {
		t.Fatalf("1 kHz passband magnitude = %.3f, want >= 0.9", desired)
	}
	if rejectionDB := 20 * math.Log10(image/desired); rejectionDB > -50 {
		t.Fatalf("7 kHz image rejection = %.1f dB, want <= -50 dB", rejectionDB)
	}
}

func TestTETRAPOLPlaybackStateIsDigitalWhilePCMIsBuffered(t *testing.T) {
	receiver := NewReceiver(Config{SampleRate: 2_048_000, FFTSize: 4096})
	receiver.mu.Lock()
	receiver.demodMode = "TETRAPOL"
	receiver.audioCount = 960
	receiver.mu.Unlock()
	mode, active := receiver.AudioPlaybackState()
	if mode != "TETRAPOL" || !active {
		t.Fatalf("playback state = %q/%v, want TETRAPOL/active", mode, active)
	}
}

func TestTETRAPOLExplicitlyEncryptedSessionBlocksAndClearsAudio(t *testing.T) {
	receiver := NewReceiver(Config{SampleRate: 2_048_000, FFTSize: 4096})
	voice := make([]float32, 320)
	for index := range voice {
		voice[index] = float32(math.Sin(2 * math.Pi * 1000 * float64(index) / tetrapolAudioInputRate))
	}
	receiver.enqueueTETRAPOLAudio(voice)
	if receiver.audioCount == 0 {
		t.Fatal("clear TETRAPOL audio was not queued")
	}

	receiver.setTETRAPOLCipherState("encrypted")
	if receiver.audioCount != 0 || receiver.digitalAudioCount != 0 {
		t.Fatalf("queued clear audio was retained for encrypted session: analog=%d digital=%d", receiver.audioCount, receiver.digitalAudioCount)
	}
	receiver.enqueueTETRAPOLAudio(voice)
	if receiver.audioCount != 0 || receiver.digitalAudioCount != 0 {
		t.Fatalf("encrypted TETRAPOL audio reached playback queue: analog=%d digital=%d", receiver.audioCount, receiver.digitalAudioCount)
	}

	receiver.setTETRAPOLCipherState("unknown")
	receiver.enqueueTETRAPOLAudio(voice)
	if receiver.audioCount == 0 {
		t.Fatal("unknown direct-TCH audio was incorrectly blocked")
	}
}

func toneMagnitude(samples []float32, frequency, sampleRate float64) float64 {
	var real, imaginary float64
	for index, sample := range samples {
		angle := 2 * math.Pi * frequency * float64(index) / sampleRate
		real += float64(sample) * math.Cos(angle)
		imaginary -= float64(sample) * math.Sin(angle)
	}
	return 2 * math.Hypot(real, imaginary) / float64(len(samples))
}
