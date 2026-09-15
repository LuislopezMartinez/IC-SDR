package sdr

import "strings"

// DeviceOption identifies one connected receiver by its Soapy driver and serial.
type DeviceOption struct{ Driver, Serial, Label string }

func FormatDeviceLabel(option DeviceOption) string {
	label := strings.TrimSpace(option.Label)
	if label == "" {
		label = option.Driver
	}
	if option.Serial != "" {
		serial := option.Serial
		if len(serial) > 10 {
			serial = serial[len(serial)-8:]
		}
		return label + " · " + serial
	}
	return label
}

func mergeDeviceOptions(base, extra []DeviceOption) []DeviceOption {
	result := make([]DeviceOption, 0, len(base)+len(extra))
	seen := make(map[string]bool)
	for _, option := range append(append([]DeviceOption{}, base...), extra...) {
		key := strings.ToLower(option.Driver) + "\x00" + option.Serial
		if option.Driver == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, option)
	}
	return result
}
