package screens

import (
	"go-zero/internal/i18n"

	"math"
	"sync/atomic"

	"go-zero/internal/sdr"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const audioSampleRate = 48_000

const (
	// WASAPI requests two periods of roughly 30 ms immediately after Play.
	// Keep 100 ms queued so both initial callbacks and ordinary DSP jitter are
	// absorbed without ever having to synthesize silence.
	analogPrebufferSamples = audioSampleRate * 100 / 1000
	// DSDcc emits decoded DMR voice in bursts of roughly 360 ms. Keeping almost
	// one burst queued prevents ordinary scheduler jitter from becoming audible.
	digitalPrebufferSamples = audioSampleRate * 340 / 1000
	digitalGraceSamples     = audioSampleRate * 100 / 1000
	// raylib uses two sub-buffers. Two 25 ms halves reproduce the original
	// JavaSound device buffer of 50 ms.
	audioSubBufferSamples = audioSampleRate * 25 / 1000
)

type AudioPlayer struct {
	receiver       *sdr.Receiver
	stream         rl.AudioStream
	ready          bool
	started        bool
	paused         bool
	volume         float32
	muted          bool
	webMuted       atomic.Bool
	starved        atomic.Bool
	callback       rl.AudioCallback
	source         []float32
	sourceCount    int
	sourcePosition float64
	processor      *AudioProcessor
	squelchEnabled atomic.Bool
	squelchOpen    atomic.Bool
	meterPeakBits  atomic.Uint32
	digitalMode    atomic.Bool
	wideFMMode     atomic.Bool
	digitalActive  atomic.Bool
	digitalStarved int
	playbackMode   string
	webAudioSink   atomic.Pointer[webAudioSink]
}

type webAudioSink struct{ publish func([]float32) }

func NewAudioPlayer(receiver *sdr.Receiver, _ *AudioRecorder) *AudioPlayer {
	return &AudioPlayer{receiver: receiver, volume: .62, source: make([]float32, 8192), processor: NewAudioProcessor()}
}

func (player *AudioPlayer) Pump() {
	if player.receiver != nil {
		mode, active := player.receiver.AudioPlaybackState()
		player.transitionPlaybackMode(mode)
		player.digitalMode.Store(mode == i18n.Source("text.2604864ce4d3") || mode == i18n.Source("text.f69d86a86926") || mode == i18n.Source("text.3ae4feb8250d"))
		player.wideFMMode.Store(mode == i18n.Source("text.6b742bac3eb4"))
		player.digitalActive.Store(active)
	}
	if !player.ready {
		rl.InitAudioDevice()
		if !rl.IsAudioDeviceReady() {
			return
		}
		rl.SetAudioStreamBufferSizeDefault(audioSubBufferSamples)
		player.stream = rl.LoadAudioStream(audioSampleRate, 32, 1)
		player.callback = func(data []float32, frames int) {
			player.fillAudio(data[:min(frames, len(data))])
		}
		rl.SetAudioStreamCallback(player.stream, player.callback)
		player.ready = true
		player.applyVolume()
	}
	if player.started && player.starved.Swap(false) {
		rl.PauseAudioStream(player.stream)
		player.started = false
		player.paused = true
		player.sourceCount = 0
		player.sourcePosition = 0
	}
	target := playbackPrebufferSamples(player.digitalMode.Load())
	if !player.started && player.bufferedSamples() >= target && (!player.digitalMode.Load() || player.digitalActive.Load()) {
		if player.paused {
			rl.ResumeAudioStream(player.stream)
		} else {
			rl.PlayAudioStream(player.stream)
		}
		player.started = true
	}
}

// transitionPlaybackMode discards timing and starvation state belonging to
// the previous demodulator. An exhausted burst-oriented DMR session must not
// leave the continuous NFM stream paused.
func (player *AudioPlayer) transitionPlaybackMode(mode string) {
	if mode == player.playbackMode {
		return
	}
	player.resetPlaybackState()
	player.playbackMode = mode
}

// ResetPlayback forces a fresh prebuffer even when two bands happen to use
// the same demodulator (for example NFM -> NFM).
func (player *AudioPlayer) ResetPlayback() { player.resetPlaybackState() }

func (player *AudioPlayer) resetPlaybackState() {
	if player.ready && player.started {
		rl.PauseAudioStream(player.stream)
		player.paused = true
	}
	player.started = false
	player.sourceCount = 0
	player.sourcePosition = 0
	player.digitalStarved = 0
	player.starved.Store(false)
}

// fillAudio continuously reconciles the independent SDR and WASAPI clocks.
// A small, inaudible resampling correction keeps the queue away from both
// underflow and overrun without dropping or inserting discontinuous samples.
func (player *AudioPlayer) fillAudio(destination []float32) {
	if len(destination) == 0 || player.receiver == nil {
		clear(destination)
		return
	}
	digital := player.digitalMode.Load()
	target := playbackPrebufferSamples(digital)
	buffered := player.receiver.AudioBufferedSamples() + player.sourceCount
	errorRatio := float64(buffered-target) / float64(target)
	correctionLimit := .04
	if digital {
		// Vocoder output already has an exact 48 kHz timebase. Only compensate
		// long-term device clock drift; a 4% correction audibly bends DMR speech.
		correctionLimit = .002
	}
	correction := min(max(errorRatio*.10, -correctionLimit), correctionLimit)
	ratio := 1 + correction
	needed := int(math.Ceil(player.sourcePosition+ratio*float64(len(destination)-1))) + 2
	needed = min(needed, len(player.source))
	for player.sourceCount < needed {
		count := player.receiver.ReadAudio(player.source[player.sourceCount:needed])
		player.sourceCount += count
		if count == 0 {
			break
		}
	}
	availableOutput := len(destination)
	if player.sourceCount < 2 {
		availableOutput = 0
	} else {
		maximumPosition := float64(player.sourceCount - 1)
		availableOutput = min(availableOutput, int(math.Floor((maximumPosition-player.sourcePosition)/ratio))+1)
	}
	for index := 0; index < availableOutput; index++ {
		position := player.sourcePosition + float64(index)*ratio
		base := int(position)
		fraction := float32(position - float64(base))
		first := player.source[base]
		destination[index] = first + fraction*(player.source[base+1]-first)
	}
	player.sourcePosition += float64(availableOutput) * ratio
	consumed := min(int(player.sourcePosition), player.sourceCount)
	if consumed > 0 {
		copy(player.source, player.source[consumed:player.sourceCount])
		player.sourceCount -= consumed
		player.sourcePosition -= float64(consumed)
	}
	if availableOutput < len(destination) {
		clear(destination[availableOutput:])
		if digital {
			player.digitalStarved += len(destination) - availableOutput
			if player.digitalStarved >= digitalGraceSamples {
				player.starved.Store(true)
				player.digitalStarved = 0
			}
		} else {
			player.starved.Store(true)
		}
	} else {
		player.digitalStarved = 0
	}
	if player.wideFMMode.Load() {
		player.processor.ProcessWideFM(destination[:availableOutput])
	} else {
		player.processor.Process(destination[:availableOutput])
	}
	player.publishAudioPeak(destination[:availableOutput])
	if sink := player.webAudioSink.Load(); sink != nil {
		if player.webMuted.Load() {
			sink.publish(make([]float32, len(destination)))
		} else {
			sink.publish(destination)
		}
	}
}

// SetWebAudioSink publishes the post-processed, pre-local-volume audio.
// Remote listeners therefore use their device volume independently of the PC.
func (player *AudioPlayer) SetWebAudioSink(sink func([]float32)) {
	if sink == nil {
		player.webAudioSink.Store(nil)
		return
	}
	player.webAudioSink.Store(&webAudioSink{publish: sink})
}

func playbackPrebufferSamples(digital bool) int {
	if digital {
		return digitalPrebufferSamples
	}
	return analogPrebufferSamples
}

// publishAudioPeak retains the largest post-processed audio sample produced
// between two screen frames. It deliberately runs before raylib's mute/volume
// control so the level remains visible while monitoring with the speaker muted.
func (player *AudioPlayer) publishAudioPeak(samples []float32) {
	peak := float32(0)
	for _, sample := range samples {
		if sample < 0 {
			sample = -sample
		}
		if sample > peak {
			peak = sample
		}
	}
	for peak > 0 {
		previousBits := player.meterPeakBits.Load()
		if peak <= math.Float32frombits(previousBits) || player.meterPeakBits.CompareAndSwap(previousBits, math.Float32bits(peak)) {
			return
		}
	}
}

// ConsumeAudioPeak returns and clears the accumulated audio peak. The UI adds
// attack/release ballistics, while the callback remains lock-free.
func (player *AudioPlayer) ConsumeAudioPeak() float32 {
	return math.Float32frombits(player.meterPeakBits.Swap(0))
}

func (player *AudioPlayer) bufferedSamples() int {
	if player.receiver == nil {
		return 0
	}
	return player.receiver.AudioBufferedSamples()
}

func (player *AudioPlayer) SetVolume(volume float32) {
	player.volume = min(max(volume, 0), 1)
	player.applyVolume()
}
func (player *AudioPlayer) SetMuted(muted bool) {
	player.muted = muted
	player.webMuted.Store(muted)
	player.applyVolume()
}
func (player *AudioPlayer) ConfigureProcessing(lowCut, highCut int, enabled bool, gains [5]float32, profile string) {
	player.processor.Configure(lowCut, highCut, enabled, gains, profile)
}
func (player *AudioPlayer) Spectrum(destination []float32) { player.processor.Spectrum(destination) }
func (player *AudioPlayer) SetRecorder(recorder *AudioRecorder) {
	if player.receiver == nil {
		return
	}
	if recorder == nil {
		player.receiver.SetRecorderSink(nil)
		return
	}
	player.receiver.SetRecorderSink(recorder.Submit)
}
func (player *AudioPlayer) SetRecorderSquelch(enabled, open bool) {
	player.squelchEnabled.Store(enabled)
	player.squelchOpen.Store(open)
}
func (player *AudioPlayer) applyVolume() {
	if !player.ready {
		return
	}
	volume := player.volume
	if player.muted {
		volume = 0
	}
	rl.SetAudioStreamVolume(player.stream, volume)
}

func (player *AudioPlayer) Close() {
	if !player.ready {
		return
	}
	rl.StopAudioStream(player.stream)
	rl.UnloadAudioStream(player.stream)
	rl.CloseAudioDevice()
	player.ready = false
	player.started = false
	player.paused = false
}
