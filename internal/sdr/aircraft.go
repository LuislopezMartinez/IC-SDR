package sdr

import "go-zero/internal/aircraft"

func (r *Receiver) ConfigureAircraft(enabled bool, mode string) {
	if r.aircraft != nil {
		r.aircraft.Configure(enabled, mode)
	}
}
func (r *Receiver) SetAircraftReference(lat, lon float64, ok bool) {
	if r.aircraft != nil {
		r.aircraft.SetReference(lat, lon, ok)
	}
}
func (r *Receiver) AircraftStatus() aircraft.Status {
	if r.aircraft == nil {
		return aircraft.Status{State: "UNAVAILABLE"}
	}
	return r.aircraft.Snapshot()
}
func (r *Receiver) Aircraft() []aircraft.Aircraft {
	if r.aircraft == nil {
		return nil
	}
	return r.aircraft.Aircraft()
}
func (r *Receiver) ClearAircraft() {
	if r.aircraft != nil {
		r.aircraft.Clear()
	}
}
