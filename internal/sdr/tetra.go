package sdr

import "go-zero/internal/i18n"

import "go-zero/internal/tetra"

func (r *Receiver) ConfigureTETRA(enabled bool) {
	if r.tetra != nil {
		r.tetra.Configure(enabled)
	}
}
func (r *Receiver) TETRAStatus() tetra.Status {
	if r.tetra == nil {
		return tetra.Status{State: i18n.Source("text.67b9e10a1cbd")}
	}
	return r.tetra.Snapshot()
}
func (r *Receiver) ClearTETRA() {
	if r.tetra != nil {
		r.tetra.Clear()
	}
	r.mu.Lock()
	r.audioRead, r.audioWrite, r.audioCount = 0, 0, 0
	r.stats.AudioBuffered = 0
	r.stats.SquelchOpen = false
	r.mu.Unlock()
}
func (r *Receiver) SetTETRAAudioPolicy(slot int, clearOnly bool) {
	if r.tetra != nil {
		r.tetra.SetAudioPolicy(slot, clearOnly)
	}
}
func (r *Receiver) TETRALiveSnapshot(frequencyHz int64) tetra.LiveSnapshot {
	if r.tetra == nil {
		return tetra.LiveSnapshot{FrequencyHz: frequencyHz}
	}
	return r.tetra.Live(frequencyHz)
}
