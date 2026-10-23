package sdr

import (
	"fmt"
	"go-zero/internal/tetrapol"
	"go-zero/internal/tetrapolruntime"
)

func (r *Receiver) ConfigureTETRAPOL(enabled bool) {
	if r.tetrapol != nil {
		r.tetrapol.Configure(enabled)
	}
}
func (r *Receiver) TETRAPOLStatus() tetrapol.Status {
	if r.tetrapol == nil {
		return tetrapol.Status{}
	}
	return r.tetrapol.Snapshot()
}

func (r *Receiver) SetTETRAPOLAFCEnabled(enabled bool) {
	if r.tetrapol != nil {
		r.tetrapol.SetAFCEnabled(enabled)
	}
}

type TETRAPOLRuntimeStatus struct {
	Running   bool
	Backend   string
	Error     string
	Telemetry tetrapolruntime.Telemetry
}

func (r *Receiver) StartTETRAPOLRuntime() error {
	return r.StartTETRAPOLRuntimeFor("TCH")
}

func (r *Receiver) StartTETRAPOLRuntimeFor(channelType string) error {
	return r.StartTETRAPOLRuntimeConfigured(channelType, "UHF", "DOWN")
}

func (r *Receiver) StartTETRAPOLRuntimeConfigured(channelType, band, direction string) error {
	return r.StartTETRAPOLRuntimeConfiguredWithCipher(channelType, band, direction, "")
}

func (r *Receiver) StartTETRAPOLRuntimeConfiguredWithCipher(channelType, band, direction, cipherState string) error {
	r.tetrapolRuntimeMu.Lock()
	defer r.tetrapolRuntimeMu.Unlock()
	r.setTETRAPOLCipherState(cipherState)
	if r.tetrapolNative != nil && r.tetrapolNative.Running() {
		return nil
	}
	channelType, _, _, _ = tetrapolRuntimeArguments(channelType, band, direction)
	r.tetrapolAudioRate.reset()
	if r.tetrapolNative == nil {
		r.tetrapolNative = &tetrapolruntime.Native{}
	}
	cellID, channelID := "", -1
	if channelType == "TCH" {
		cellID, channelID = r.tetrapolCellID, r.tetrapolChannelID
	}
	err := r.tetrapolNative.StartConfigured(r.config.TETRAPOLNativeDLL, channelType, band, direction, cellID, channelID, r.tetrapol.ChannelSampleRate(), r.enqueueTETRAPOLAudio)
	if err != nil {
		r.tetrapolRuntimeError = err.Error()
		return err
	}
	r.tetrapolRuntimeError = ""
	r.tetrapol.SetBitSink(nil)
	r.tetrapol.SetIQSink(func(iq []float32) {
		if err := r.tetrapolNative.SubmitIQ(iq); err != nil {
			r.tetrapolRuntimeMu.Lock()
			r.tetrapolRuntimeError = err.Error()
			r.tetrapolRuntimeMu.Unlock()
		}
	})
	r.tetrapol.Configure(true)
	return nil
}

func (r *Receiver) SetTETRAPOLFollowContext(cellID string, channelID int) {
	r.tetrapolRuntimeMu.Lock()
	r.tetrapolCellID, r.tetrapolChannelID = cellID, channelID
	r.tetrapolRuntimeMu.Unlock()
}

// ClearTETRAPOLNetworkState invalidates decoded network history when the user
// manually changes frequency. It is deliberately not part of Stop: automatic
// CCH/TCH transitions need the shared CCH registry to follow and return.
func (r *Receiver) ClearTETRAPOLNetworkState() {
	r.tetrapolRuntimeMu.Lock()
	defer r.tetrapolRuntimeMu.Unlock()
	r.tetrapolCellID, r.tetrapolChannelID = "", -1
	if r.tetrapolNative != nil && r.tetrapolNative.Running() {
		r.tetrapolNative.ClearNetworkState()
	}
}

func tetrapolRuntimeArguments(channelType, band, direction string) (string, string, string, []string) {
	if channelType != "CCH" {
		channelType = "TCH"
	}
	if band != "VHF" {
		band = "UHF"
	}
	if direction != "UP" {
		direction = "DOWN"
	}
	return channelType, band, direction, []string{"-b", band, "-t", channelType, "-d", direction}
}

func (r *Receiver) StopTETRAPOLRuntime() error {
	r.tetrapolRuntimeMu.Lock()
	defer r.tetrapolRuntimeMu.Unlock()
	if r.tetrapol != nil {
		r.tetrapol.SetBitSink(nil)
		r.tetrapol.SetIQSink(nil)
		r.tetrapol.Configure(false)
	}
	if r.tetrapolNative != nil && r.tetrapolNative.Running() {
		if err := r.tetrapolNative.Stop(); err != nil {
			r.tetrapolRuntimeError = err.Error()
			return fmt.Errorf("stop liliPOL x64 runtime: %w", err)
		}
		return nil
	}
	return nil
}

func (r *Receiver) TETRAPOLRuntimeStatus() TETRAPOLRuntimeStatus {
	r.tetrapolRuntimeMu.Lock()
	defer r.tetrapolRuntimeMu.Unlock()
	status := TETRAPOLRuntimeStatus{Backend: "LILIPOL NATIVO X64", Error: r.tetrapolRuntimeError}
	if r.tetrapolNative != nil && r.tetrapolNative.Running() {
		status.Running, status.Telemetry = true, r.tetrapolNative.Telemetry()
		return status
	}
	if r.tetrapolNative != nil && status.Error == "" {
		status.Error = r.tetrapolNative.Error()
	}
	return status
}

func (r *Receiver) enqueueTETRAPOLAudio(input []float32) {
	if len(input) == 0 || !r.tetrapolAudioEnabled.Load() || r.tetrapolAudioBlocked.Load() {
		return
	}
	// RP-CELP produces 8 kHz mono; reconstruct its speech band before placing
	// it in the shared 48 kHz ring. Linear interpolation noticeably attenuates
	// the upper voice band and leaves audible spectral images.
	r.enqueueDigitalAudio(r.tetrapolAudioRate.process(input))
}

func (r *Receiver) SetTETRAPOLAudioEnabled(enabled bool) {
	r.tetrapolAudioEnabled.Store(enabled)
}

func (r *Receiver) setTETRAPOLCipherState(state string) {
	blocked := state == "encrypted"
	r.tetrapolAudioBlocked.Store(blocked)
	if !blocked {
		return
	}
	// Do not leave PCM from a previous clear call waiting in the shared
	// playback rings when an explicitly encrypted session is selected.
	r.mu.Lock()
	r.audioRead, r.audioWrite, r.audioCount = 0, 0, 0
	r.digitalAudioRead, r.digitalAudioWrite, r.digitalAudioCount = 0, 0, 0
	r.stats.AudioBuffered = 0
	r.mu.Unlock()
}
