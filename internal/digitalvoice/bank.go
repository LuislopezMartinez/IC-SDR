package digitalvoice

import (
	"go-zero/internal/i18n"

	"fmt"
	"os"
	"strings"
	"sync"
)

// Bank runs one auto decoder for the complete matrix, or one narrowly scoped
// decoder per selected mode for an arbitrary subset. DSD-neo's CLI presets are
// mutually replacing, so parallel scoped instances are the only way to honor
// combinations such as DMR+NXDN without silently enabling every protocol.
type Bank struct {
	mu         sync.RWMutex
	inputRate  float64
	executable string
	onAudio    func([]float32)
	offsetHz   float64
	bandwidth  int
	decoders   []*Decoder
	available  bool
}

func NewBank(inputRate float64, executable string, onAudio func([]float32)) *Bank {
	_, err := os.Stat(executable)
	return &Bank{inputRate: inputRate, executable: executable, onAudio: onAudio, bandwidth: 15_000, available: err == nil}
}

func (b *Bank) Configure(offsetHz float64, bandwidth int) {
	b.mu.Lock()
	b.offsetHz, b.bandwidth = offsetHz, bandwidth
	decoders := append([]*Decoder(nil), b.decoders...)
	b.mu.Unlock()
	for _, decoder := range decoders {
		decoder.Configure(offsetHz, bandwidth)
	}
}

func (b *Bank) Start(selection string) error {
	b.Stop()
	modes := []string{selection}
	if strings.Contains(selection, "|") {
		modes = strings.Split(selection, "|")
	}
	if len(modes) == 0 {
		modes = []string{i18n.Source("text.5f08cf2bcce3")}
	}
	b.mu.Lock()
	for range modes {
		decoder := New(b.inputRate, b.executable, b.onAudio)
		decoder.Configure(b.offsetHz, b.bandwidth)
		b.decoders = append(b.decoders, decoder)
	}
	decoders := append([]*Decoder(nil), b.decoders...)
	b.mu.Unlock()
	started := 0
	var firstErr error
	for i, decoder := range decoders {
		if err := decoder.Start(modes[i]); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			started++
		}
	}
	if started == 0 {
		return firstErr
	}
	if firstErr != nil {
		return fmt.Errorf(i18n.Source("text.2c6ce48d1160"), firstErr)
	}
	return nil
}

func (b *Bank) Stop() {
	b.mu.Lock()
	decoders := b.decoders
	b.decoders = nil
	b.mu.Unlock()
	for _, decoder := range decoders {
		decoder.Stop()
	}
}

func (b *Bank) Close() { b.Stop() }

func (b *Bank) ProcessIQ(iq []float32) {
	b.mu.RLock()
	decoders := append([]*Decoder(nil), b.decoders...)
	b.mu.RUnlock()
	for _, decoder := range decoders {
		decoder.ProcessIQ(iq)
	}
}

func (b *Bank) Snapshot() Status {
	b.mu.RLock()
	decoders := append([]*Decoder(nil), b.decoders...)
	b.mu.RUnlock()
	if len(decoders) == 0 {
		detail := i18n.Source("text.e2cab4411acf")
		if !b.available {
			detail = i18n.Source("text.c25c64e91cd9")
		}
		return Status{State: i18n.Source("text.7dc7253c376a"), Detail: detail, Available: b.available, InputDBFS: -60}
	}
	result := decoders[0].Snapshot()
	result.Events = nil
	for _, decoder := range decoders {
		status := decoder.Snapshot()
		result.Running = result.Running || status.Running
		result.Available = result.Available || status.Available
		result.Events = append(result.Events, status.Events...)
		if status.VoiceActive || (result.Protocol == "" && status.Protocol != "") {
			result = mergePrimary(result, status)
		}
	}
	return result
}

func mergePrimary(base, primary Status) Status {
	events, running, available := base.Events, base.Running, base.Available
	base = primary
	base.Events, base.Running, base.Available = events, running, available
	return base
}
