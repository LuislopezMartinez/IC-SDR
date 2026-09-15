package sdr

import "go-zero/internal/i18n"

import "go-zero/internal/radiosonde"

func (r *Receiver) ConfigureRadiosonde(enabled bool, family string, frequency int64) {
	if r.radiosonde != nil {
		r.radiosonde.Configure(enabled, family, frequency, r.centerHz.Load())
	}
}
func (r *Receiver) RadiosondeStatus() radiosonde.Status {
	if r.radiosonde == nil {
		return radiosonde.Status{State: i18n.Source("text.67b9e10a1cbd")}
	}
	return r.radiosonde.Snapshot()
}
func (r *Receiver) RadiosondeEvents() []radiosonde.Event {
	if r.radiosonde == nil {
		return nil
	}
	return r.radiosonde.Events()
}
func (r *Receiver) ClearRadiosondeEvents() {
	if r.radiosonde != nil {
		r.radiosonde.Clear()
	}
}
