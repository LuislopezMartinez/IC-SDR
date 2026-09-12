package sdr

import (
	"strings"
	"unsafe"
)

var rspDxAntennas = []string{"Antenna A", "Antenna B", "Antenna C"}

var soapyAntennaSettings = []string{"antenna", "antenna_sel", "rspdx_antenna"}

// AntennaShortLabel is the compact A/B/C (or Hi-Z) text shown on the switch.
func AntennaShortLabel(name string) string {
	trimmed := strings.TrimSpace(name)
	lower := strings.ToLower(trimmed)
	switch {
	case lower == "a" || strings.Contains(lower, "antenna a") || strings.HasSuffix(lower, " ant a"):
		return "A"
	case lower == "b" || strings.Contains(lower, "antenna b") || strings.HasSuffix(lower, " ant b"):
		return "B"
	case lower == "c" || strings.Contains(lower, "antenna c") || strings.HasSuffix(lower, " ant c"):
		return "C"
	case strings.Contains(lower, "hi-z") || strings.Contains(lower, "hiz") || strings.Contains(lower, "hi z"):
		return "Hi-Z"
	case strings.Contains(lower, "tuner 1") && strings.Contains(lower, "50"):
		return "T1"
	case strings.Contains(lower, "tuner 2"):
		return "T2"
	case trimmed == "":
		return ""
	default:
		return trimmed
	}
}

// MatchAntenna maps a saved or typed name onto a Soapy antenna string.
func MatchAntenna(name string, available []string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, candidate := range available {
		if strings.EqualFold(strings.TrimSpace(candidate), name) {
			return candidate
		}
	}
	want := AntennaShortLabel(name)
	for _, candidate := range available {
		if AntennaShortLabel(candidate) == want {
			return candidate
		}
	}
	return name
}

func defaultAntennasFor(driver string) []string {
	if ProfileFor(driver) == ProfileSDRplay {
		return append([]string(nil), rspDxAntennas...)
	}
	return nil
}

func soapyAntennaSwitchSupported(driver string) bool {
	return ProfileFor(driver) == ProfileSDRplay
}

const maxSoapyAntennas = 16

func (api *soapyAPI) listAntennaNames(device uintptr) []string {
	if api == nil || device == 0 {
		return nil
	}
	var length uintptr
	list := api.listAntennas(device, soapyRX, 0, &length)
	if list == 0 || length == 0 {
		return nil
	}
	if length > maxSoapyAntennas {
		// A garbage length would walk off the list and crash. Skip the free
		// rather than pass a truncated count into SoapySDRStrings_clear.
		return nil
	}
	defer api.stringsClear(&list, length)
	pointerSize := unsafe.Sizeof(uintptr(0))
	out := make([]string, 0, int(length))
	for index := uintptr(0); index < length; index++ {
		value := cString(*(*uintptr)(unsafe.Pointer(list + index*pointerSize)))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (device *soapyDevice) refreshAntennas() {
	if device == nil || device.api == nil {
		return
	}
	if !soapyAntennaSwitchSupported(device.driver) {
		device.antennas = nil
		device.antenna = ""
		return
	}
	antennas := device.api.listAntennaNames(device.device)
	if len(antennas) == 0 {
		antennas = defaultAntennasFor(device.driver)
	}
	device.antennas = antennas
	current := strings.TrimSpace(device.api.consume(device.api.getAntenna(device.device, soapyRX, 0)))
	if current == "" && len(antennas) > 0 {
		current = antennas[0]
	}
	device.antenna = MatchAntenna(current, antennas)
}

func (device *soapyDevice) antennaState() (string, []string) {
	if !soapyAntennaSwitchSupported(device.driver) {
		return "", nil
	}
	if len(device.antennas) == 0 {
		device.refreshAntennas()
	}
	current := ""
	if device.api != nil {
		current = strings.TrimSpace(device.api.consume(device.api.getAntenna(device.device, soapyRX, 0)))
	}
	if current == "" {
		current = device.antenna
	}
	current = MatchAntenna(current, device.antennas)
	if current == "" && len(device.antennas) > 0 {
		current = device.antennas[0]
	}
	device.antenna = current
	return current, append([]string(nil), device.antennas...)
}

func (device *soapyDevice) setAntenna(name string) error {
	if device == nil || device.api == nil {
		return nil
	}
	if !soapyAntennaSwitchSupported(device.driver) {
		return nil
	}
	if len(device.antennas) == 0 {
		device.refreshAntennas()
	}
	name = MatchAntenna(name, device.antennas)
	if name == "" {
		return nil
	}
	if err := device.api.check(device.api.setAntenna(device.device, soapyRX, 0, name), "antenna"); err != nil {
		if !device.writeSettingAny(soapyAntennaSettings, name) {
			return err
		}
	}
	device.antenna = name
	got := MatchAntenna(strings.TrimSpace(device.api.consume(device.api.getAntenna(device.device, soapyRX, 0))), device.antennas)
	if got != "" && AntennaShortLabel(got) != AntennaShortLabel(name) {
		device.writeSettingAny(soapyAntennaSettings, name)
		_ = device.api.setAntenna(device.device, soapyRX, 0, name)
		device.antenna = name
	}
	return nil
}

func (device *soapyDevice) applyAntenna(name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	return device.setAntenna(name)
}
