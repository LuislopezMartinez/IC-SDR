//go:build !windows

package sdr

import "go-zero/internal/i18n"

import "fmt"

const (
	soapyTimeout  = -1
	soapyOverflow = -4
)

type soapyDevice struct{}

func openSoapy(Config) (*soapyDevice, error) {
	return nil, fmt.Errorf("%s", i18n.Source("text.ccfc87bf0564"))
}
func openSoapyExact(config Config) (*soapyDevice, error)          { return openSoapy(config) }
func listSoapyDevices(Config) ([]DeviceOption, error)             { return nil, nil }
func (*soapyDevice) read([]float32) (int, int32, error)           { return 0, 0, nil }
func (*soapyDevice) close()                                       {}
func (*soapyDevice) setCenterFrequency(int64) error               { return nil }
func (*soapyDevice) centerFrequency() int64                       { return 0 }
func (*soapyDevice) hardwareSettings() HardwareSettings           { return HardwareSettings{} }
func (*soapyDevice) applyHardwareSettings(HardwareSettings) error { return nil }
