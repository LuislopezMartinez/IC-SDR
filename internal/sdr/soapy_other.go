//go:build !windows

package sdr

import "fmt"

const (
	soapyTimeout  = -1
	soapyOverflow = -4
)

type soapyDevice struct {
	hardware   string
	driver     string
	sampleRate float64
}

func openSoapy(Config) (*soapyDevice, error) {
	return nil, fmt.Errorf("the first SoapySDR backend is currently available only on Windows")
}
func (*soapyDevice) read([]float32) (int, int32, error)           { return 0, 0, nil }
func (*soapyDevice) close()                                       {}
func (*soapyDevice) setCenterFrequency(int64) error               { return nil }
func (*soapyDevice) centerFrequency() int64                       { return 0 }
func (*soapyDevice) hardwareSettings() HardwareSettings           { return HardwareSettings{} }
func (*soapyDevice) applyHardwareSettings(HardwareSettings) error { return nil }
