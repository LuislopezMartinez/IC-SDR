package tetrapolruntime

import (
	"fmt"
	"sync"
	"time"
)

const (
	nativeIQQueueCapacity    = 8
	nativeAudioQueueCapacity = 32
)

// Native runs the authorized liliPOL engine in-process. Every DLL call is
// serialized on one worker; the SDR callback hands it a copied IQ block while
// preserving stream continuity.
type Native struct {
	mu                 sync.RWMutex
	api                nativeAPI
	handle             uintptr
	iq                 chan []float32
	audio              chan []float32
	stop               chan struct{}
	done               chan struct{}
	audioDone          chan struct{}
	running            bool
	emit               func([]float32)
	telemetry          Telemetry
	lastError          string
	lastDecodedAudio   time.Time
	lastConcealedAudio uint64
	droppedIQ          uint64
	droppedAudio       uint64
	lastTCHSession     int
	tchSessionEvent    uint64
}

func (decoder *Native) Start(library, channel string, sampleRate float64, emit func([]float32)) error {
	return decoder.StartConfigured(library, channel, "UHF", "DOWN", "", -1, sampleRate, emit)
}

func (decoder *Native) StartConfigured(library, channel, band, direction, cellID string, channelID int, sampleRate float64, emit func([]float32)) error {
	decoder.mu.Lock()
	defer decoder.mu.Unlock()
	if decoder.running {
		return nil
	}
	api, err := loadNativeAPI(library)
	if err != nil {
		return err
	}
	mode := 1
	if channel == "CCH" {
		mode = 0
	}
	bandMode := 2
	if band == "VHF" {
		bandMode = 1
	}
	directionMode := 1
	if direction == "UP" {
		directionMode = 2
	}
	handle := api.createConfigured(mode, bandMode, directionMode)
	if handle == 0 {
		api.close()
		return fmt.Errorf("liliPOL x64 could not create a %s decoder", channel)
	}
	decoder.api, decoder.handle = api, handle
	decoder.iq = make(chan []float32, nativeIQQueueCapacity)
	decoder.audio = make(chan []float32, nativeAudioQueueCapacity)
	decoder.stop, decoder.done, decoder.audioDone = make(chan struct{}), make(chan struct{}), make(chan struct{})
	decoder.emit, decoder.telemetry, decoder.lastError = emit, Telemetry{}, ""
	decoder.lastDecodedAudio, decoder.lastConcealedAudio = time.Time{}, 0
	decoder.droppedIQ, decoder.droppedAudio = 0, 0
	decoder.lastTCHSession, decoder.tchSessionEvent = -1, 0
	api.setSampleRate(handle, sampleRate)
	api.setContext(handle, cellID, channelID)
	api.setAudioCallback(handle, func(samples []float32) {
		block := append([]float32(nil), samples...)
		select {
		case decoder.audio <- block:
		default:
			decoder.mu.Lock()
			decoder.droppedAudio++
			decoder.mu.Unlock()
		}
	})
	decoder.running = true
	go decoder.audioLoop()
	go decoder.worker()
	return nil
}

func (decoder *Native) SubmitIQ(samples []float32) error {
	if len(samples) < 2 {
		return nil
	}
	decoder.mu.RLock()
	if !decoder.running || decoder.iq == nil {
		decoder.mu.RUnlock()
		return fmt.Errorf("liliPOL x64 is not running")
	}
	queue := decoder.iq
	stop := decoder.stop
	decoder.mu.RUnlock()
	block := append([]float32(nil), samples...)
	// Frame synchronization cannot survive arbitrary holes in the IQ stream.
	// Apply backpressure instead of silently throwing away a complete block.
	// The direct 32 kS/s native path processes far faster than real time; the
	// stop case still guarantees that shutdown cannot leave this call blocked.
	select {
	case queue <- block:
		return nil
	case <-stop:
		return fmt.Errorf("liliPOL x64 stopped while accepting IQ")
	}
}

func (decoder *Native) worker() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer close(decoder.done)
	for {
		select {
		case samples := <-decoder.iq:
			decoder.api.processIQ(decoder.handle, samples)
		case now := <-ticker.C:
			decoder.poll(now)
		case <-decoder.stop:
			decoder.api.clearAudioCallback(decoder.handle)
			decoder.api.destroy(decoder.handle)
			decoder.api.close()
			return
		}
	}
}

func (decoder *Native) audioLoop() {
	defer close(decoder.audioDone)
	for {
		select {
		case samples := <-decoder.audio:
			if decoder.emit != nil && len(samples) > 0 {
				decoder.emit(samples)
			}
		case <-decoder.stop:
			return
		}
	}
}

func (decoder *Native) poll(now time.Time) {
	value, err := decoder.api.snapshot(decoder.handle)
	decoder.mu.Lock()
	defer decoder.mu.Unlock()
	if err != nil {
		decoder.lastError = err.Error()
		return
	}
	decoder.reconcileTelemetry(&value, now)
	decoder.telemetry = value
}

// reconcileTelemetry derives host-side timing and one-shot events from the
// monotonically increasing counters exported by the native decoder. The caller
// holds decoder.mu.
func (decoder *Native) reconcileTelemetry(value *Telemetry, now time.Time) {
	if value.ValidFrames != decoder.telemetry.ValidFrames || value.CRCFailures != decoder.telemetry.CRCFailures {
		value.LastRecord = now
	} else {
		value.LastRecord = decoder.telemetry.LastRecord
	}
	decodedProgress := value.AudioDecoded > decoder.telemetry.AudioDecoded
	concealedProgress := value.AudioDecoded > 0 && value.AudioConcealed > decoder.lastConcealedAudio
	if decodedProgress || concealedProgress {
		decoder.lastDecodedAudio = now
	}
	decoder.lastConcealedAudio = value.AudioConcealed
	value.VoiceActive = !decoder.lastDecodedAudio.IsZero() && now.Sub(decoder.lastDecodedAudio) < 300*time.Millisecond
	if value.VoiceActive {
		value.FrameType = "VOICE"
	} else {
		value.FrameType = "DATA"
	}
	value.AudioLongGaps = decoder.droppedAudio
	value.IQDropped = decoder.droppedIQ
	value.IQQueued = len(decoder.iq)
	if value.LogicalChannel == "TCH" {
		if value.TCHSessionState != decoder.lastTCHSession {
			decoder.lastTCHSession = value.TCHSessionState
			decoder.tchSessionEvent++
		}
		if value.TCHSessionState == 2 {
			value.ChannelAction = "return"
			value.ChannelEvent = decoder.tchSessionEvent
		}
	}
}

func (decoder *Native) Stop() error {
	decoder.mu.Lock()
	if !decoder.running {
		decoder.mu.Unlock()
		return nil
	}
	decoder.running = false
	stop, done, audioDone := decoder.stop, decoder.done, decoder.audioDone
	close(stop)
	decoder.mu.Unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		return fmt.Errorf("liliPOL x64 did not stop within 2s")
	}
	select {
	case <-audioDone:
	case <-time.After(500 * time.Millisecond):
		return fmt.Errorf("liliPOL x64 audio worker did not stop")
	}
	decoder.mu.Lock()
	decoder.handle, decoder.iq, decoder.audio = 0, nil, nil
	decoder.mu.Unlock()
	return nil
}

func (decoder *Native) Running() bool {
	decoder.mu.RLock()
	defer decoder.mu.RUnlock()
	return decoder.running
}

func (decoder *Native) Error() string {
	decoder.mu.RLock()
	defer decoder.mu.RUnlock()
	return decoder.lastError
}

func (decoder *Native) Telemetry() Telemetry {
	decoder.mu.RLock()
	defer decoder.mu.RUnlock()
	return decoder.telemetry
}
