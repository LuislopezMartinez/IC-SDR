package sdr

import (
	"go-zero/internal/i18n"

	"math"
	"testing"
	"time"
)

func TestIQStats(t *testing.T) {
	rms, peak, invalid := iqStats([]float32{1, 0, 0, 1, float32(math.NaN()), 0})
	if math.Abs(rms-math.Sqrt(2.0/3.0)) > 1e-6 || peak != 1 || invalid != 1 {
		t.Fatalf("rms=%v peak=%v invalid=%d", rms, peak, invalid)
	}
}

func TestDigitalVoiceBypassSelectsDigitalThenReturnsToNFM(t *testing.T) {
	receiver := NewReceiver(Config{FrequencyHz: 100_000_000, SampleRate: 2_048_000, FFTSize: 4096})
	receiver.mu.Lock()
	receiver.demodMode = i18n.Source("text.3ae4feb8250d")
	receiver.digitalVoiceBypass = true
	receiver.audio[0], receiver.audioWrite, receiver.audioCount = .25, 1, 1
	receiver.mu.Unlock()

	receiver.enqueueDigitalAudio([]float32{.75})
	output := make([]float32, 1)
	if count := receiver.ReadAudio(output); count != 1 || output[0] != .75 {
		t.Fatalf("active bypass output = %v (%d samples), want digital .75", output, count)
	}

	receiver.mu.Lock()
	receiver.digitalVoiceLastAudio = time.Now().Add(-time.Second)
	receiver.audio[0], receiver.audioRead, receiver.audioWrite, receiver.audioCount = .25, 0, 1, 1
	receiver.mu.Unlock()
	if count := receiver.ReadAudio(output); count != 1 || output[0] != .25 {
		t.Fatalf("idle bypass output = %v (%d samples), want NFM .25", output, count)
	}
}

func TestChangingDemodulatorClearsDigitalVoiceBypassQueue(t *testing.T) {
	receiver := NewReceiver(Config{FrequencyHz: 100_000_000, SampleRate: 2_048_000, FFTSize: 4096})
	receiver.mu.Lock()
	receiver.demodMode = i18n.Source("text.3ae4feb8250d")
	receiver.digitalVoiceBypass = true
	receiver.mu.Unlock()
	receiver.enqueueDigitalAudio([]float32{.75})

	receiver.SetDemodulator("NFM", 100_000_000, 12_500)
	receiver.mu.RLock()
	count := receiver.digitalAudioCount
	receiver.mu.RUnlock()
	if count != 0 {
		t.Fatalf("digital bypass queue survived demodulator change: %d", count)
	}
}

func TestRecorderSinkReceivesContinuousDemodulatorAudio(t *testing.T) {
	receiver := &Receiver{}
	var got []float32
	var enabled, open bool
	receiver.SetRecorderSink(func(samples []float32, squelchEnabled, squelchOpen bool) {
		got = append(got, samples...)
		enabled, open = squelchEnabled, squelchOpen
	})
	receiver.publishRecorderAudio([]float32{.1, -.2, .3}, true, true)
	if len(got) != 3 || got[1] != -.2 || !enabled || !open {
		t.Fatalf("recorder sink received samples=%v enabled=%v open=%v", got, enabled, open)
	}
	receiver.SetRecorderSink(nil)
	receiver.publishRecorderAudio([]float32{.9}, false, false)
	if len(got) != 3 {
		t.Fatal("disabled recorder sink still received audio")
	}
}

func TestSquelchClosesAndReopensAudio(t *testing.T) {
	receiver := NewReceiver(Config{SampleRate: 2_048_000, FFTSize: 4096})
	receiver.SetSquelch(true, -100, 0, 20)

	closed := make([]float32, 1_200)
	for index := range closed {
		closed[index] = 1
	}
	receiver.mu.Lock()
	receiver.applySquelchLocked(closed, -110)
	receiver.mu.Unlock()
	if closed[len(closed)-1] != 0 || receiver.stats.SquelchOpen {
		t.Fatalf("squelch did not close: last=%v open=%v", closed[len(closed)-1], receiver.stats.SquelchOpen)
	}

	opened := make([]float32, 1_200)
	for index := range opened {
		opened[index] = 1
	}
	receiver.mu.Lock()
	receiver.applySquelchLocked(opened, -90)
	receiver.mu.Unlock()
	if opened[len(opened)-1] < .9 || !receiver.stats.SquelchOpen {
		t.Fatalf("squelch did not reopen: last=%v open=%v", opened[len(opened)-1], receiver.stats.SquelchOpen)
	}
}

func TestSignalLevelUsesTunedPassband(t *testing.T) {
	receiver := NewReceiver(Config{FrequencyHz: 100_000_000, SampleRate: 2_048_000, FFTSize: 4096})
	receiver.stats.FFTBlocks = 1
	receiver.spectrum[2048] = -82
	if got := receiver.signalLevelLocked("NFM", 100_000_000, 100_000_000, 12_500); got != -82 {
		t.Fatalf("signal level = %v, want -82", got)
	}
}

func TestDMRPublishesRFLevelForSMeter(t *testing.T) {
	receiver := NewReceiver(Config{FrequencyHz: 100_000_000, SampleRate: 2_048_000, FFTSize: 4096})
	receiver.mu.Lock()
	receiver.demodMode = "DMR BETA"
	receiver.tunedHz = 100_000_000
	receiver.demodBandwidthHz = 12_500
	receiver.stats.FFTBlocks = 1
	for index := range receiver.spectrum {
		receiver.spectrum[index] = -83
	}
	receiver.mu.Unlock()

	receiver.processAudio(make([]float32, 128))
	receiver.mu.RLock()
	level := receiver.stats.SignalDBm
	receiver.mu.RUnlock()
	if level != -83 {
		t.Fatalf("DMR SignalDBm=%v, want -83", level)
	}
}

func TestDigitalToAnalogModeChangeClearsSharedAudio(t *testing.T) {
	receiver := NewReceiver(Config{FrequencyHz: 100_000_000, SampleRate: 2_048_000, FFTSize: 4096})
	receiver.mu.Lock()
	receiver.demodMode = "DMR BETA"
	receiver.audioRead, receiver.audioWrite, receiver.audioCount = 0, 3, 3
	receiver.stats.AudioBuffered = 3
	receiver.mu.Unlock()

	receiver.SetDemodulator("NFM", 100_000_000, 12_500)

	if got := receiver.AudioBufferedSamples(); got != 0 {
		t.Fatalf("stale DMR audio remained after selecting NFM: %d samples", got)
	}
	receiver.mu.RLock()
	buffered := receiver.stats.AudioBuffered
	receiver.mu.RUnlock()
	if buffered != 0 {
		t.Fatalf("published buffered count was not reset: %d", buffered)
	}
}

func TestStopAllDecodersStopsTETRAAndClearsAudio(t *testing.T) {
	receiver := NewReceiver(Config{FrequencyHz: 100_000_000, SampleRate: 2_048_000, FFTSize: 4096})
	receiver.tetra.Configure(true)
	if !receiver.tetra.Snapshot().Running {
		t.Fatal("TETRA did not start for the lifecycle test")
	}
	receiver.enqueueDigitalAudio([]float32{.1, .2, .3})

	receiver.StopAllDecoders()

	if receiver.tetra.Snapshot().Running {
		t.Fatal("TETRA remained active after StopAllDecoders")
	}
	if got := receiver.AudioBufferedSamples(); got != 0 {
		t.Fatalf("decoder audio remained after StopAllDecoders: %d", got)
	}
}
