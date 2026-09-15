package sdr

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func LoadPreferredDevice(path string) DeviceOption {
	data, err := os.ReadFile(path)
	if err != nil {
		return DeviceOption{}
	}
	var option DeviceOption
	if json.Unmarshal(data, &option) != nil {
		return DeviceOption{}
	}
	if option.Driver != "sdrplay" && option.Driver != "rtlsdr" && option.Driver != "hackrf" {
		return DeviceOption{}
	}
	return option
}

func SavePreferredDevice(path string, option DeviceOption) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(option, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
