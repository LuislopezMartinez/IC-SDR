package sdr

import "go-zero/internal/datv"

func (r *Receiver) StartDATV(settings datv.Settings) error {
	if r.datv == nil {
		return nil
	}
	return r.datv.Start(settings)
}
func (r *Receiver) StopDATV() {
	if r.datv != nil {
		r.datv.Stop()
	}
}
func (r *Receiver) DATVStatus() datv.Status {
	if r.datv == nil {
		return datv.Status{State: "unavailable"}
	}
	return r.datv.Snapshot()
}
func (r *Receiver) SetDATVRecording(path string) error {
	if r.datv == nil {
		return nil
	}
	return r.datv.SetRecording(path)
}
