package sdr

import "go-zero/internal/ais"

func (r *Receiver) ConfigureAIS(enabled bool) {
	if r.ais != nil {
		r.ais.Configure(enabled)
	}
}
func (r *Receiver) AISStatus() ais.Status {
	if r.ais == nil {
		return ais.Status{State: "UNAVAILABLE"}
	}
	return r.ais.Snapshot()
}
func (r *Receiver) AISVessels() []ais.Vessel {
	if r.ais == nil {
		return nil
	}
	return r.ais.Vessels()
}
func (r *Receiver) ClearAIS() {
	if r.ais != nil {
		r.ais.Clear()
	}
}
