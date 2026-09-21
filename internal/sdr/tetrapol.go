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

type TETRAPOLRuntimeStatus struct {
	Running   bool
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
	r.tetrapolRuntimeMu.Lock()
	defer r.tetrapolRuntimeMu.Unlock()
	if r.tetrapolRuntime == nil {
		r.tetrapolRuntime = &tetrapolruntime.Pipeline{}
	}
	if r.tetrapolRuntime.Running() {
		return nil
	}
	_, _, _, kitArgs := tetrapolRuntimeArguments(channelType, band, direction)
	r.tetrapolPrevious, r.tetrapolHavePrevious = 0, false
	err := r.tetrapolRuntime.Start(tetrapolruntime.Config{
		KitExecutable:    r.config.TETRAPOLKitExecutable,
		KitArgs:          kitArgs,
		RPCELPExecutable: r.config.TETRAPOLRPCELPExecutable,
		ClearOnly:        true,
	}, r.enqueueTETRAPOLAudio)
	if err != nil {
		r.tetrapolRuntimeError = err.Error()
		return err
	}
	r.tetrapolRuntimeError = ""
	r.tetrapol.SetBitSink(func(bits []byte) {
		if err := r.tetrapolRuntime.SubmitBits(bits); err != nil {
			r.tetrapolRuntimeMu.Lock()
			r.tetrapolRuntimeError = err.Error()
			r.tetrapolRuntimeMu.Unlock()
		}
	})
	r.tetrapol.Configure(true)
	return nil
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
		r.tetrapol.Configure(false)
	}
	if r.tetrapolRuntime == nil || !r.tetrapolRuntime.Running() {
		return nil
	}
	if err := r.tetrapolRuntime.Stop(); err != nil {
		r.tetrapolRuntimeError = err.Error()
		return fmt.Errorf("stop TETRAPOL runtime: %w", err)
	}
	return nil
}

func (r *Receiver) TETRAPOLRuntimeStatus() TETRAPOLRuntimeStatus {
	r.tetrapolRuntimeMu.Lock()
	defer r.tetrapolRuntimeMu.Unlock()
	status := TETRAPOLRuntimeStatus{Running: r.tetrapolRuntime != nil && r.tetrapolRuntime.Running(), Error: r.tetrapolRuntimeError}
	if r.tetrapolRuntime != nil {
		status.Telemetry = r.tetrapolRuntime.Telemetry()
	}
	return status
}

func (r *Receiver) enqueueTETRAPOLAudio(input []float32) {
	if len(input) == 0 || !r.tetrapolAudioEnabled.Load() {
		return
	}
	// RP-CELP produces 8 kHz mono; the shared receiver ring is 48 kHz.
	output := make([]float32, 0, len(input)*6)
	for _, current := range input {
		if !r.tetrapolHavePrevious {
			r.tetrapolPrevious, r.tetrapolHavePrevious = current, true
			continue
		}
		for phase := 0; phase < 6; phase++ {
			fraction := float32(phase) / 6
			output = append(output, r.tetrapolPrevious+(current-r.tetrapolPrevious)*fraction)
		}
		r.tetrapolPrevious = current
	}
	r.enqueueDigitalAudio(output)
}

func (r *Receiver) SetTETRAPOLAudioEnabled(enabled bool) {
	r.tetrapolAudioEnabled.Store(enabled)
}
