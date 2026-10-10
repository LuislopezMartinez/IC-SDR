package sdr

import "go-zero/internal/dab"

func (r *Receiver) ConfigureDAB(enabled bool, serviceID uint32) error {
	if r.dab == nil {
		return nil
	}
	r.mu.Lock()
	r.dabPlaybackEnabled = enabled
	r.audioRead, r.audioWrite, r.audioCount = 0, 0, 0
	r.digitalAudioRead, r.digitalAudioWrite, r.digitalAudioCount = 0, 0, 0
	r.stats.AudioBuffered = 0
	r.mu.Unlock()
	if err := r.dab.Configure(enabled, serviceID); err != nil {
		r.mu.Lock()
		r.dabPlaybackEnabled = false
		r.mu.Unlock()
		return err
	}
	return nil
}

func (r *Receiver) SelectDABService(serviceID uint32) error {
	if r.dab == nil {
		return nil
	}
	return r.dab.SelectService(serviceID)
}

func (r *Receiver) DABStatus() dab.Status {
	if r.dab == nil {
		return dab.Status{State: "unavailable", Detail: "DAB decoder unavailable"}
	}
	return r.dab.Snapshot()
}
