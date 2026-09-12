//go:build windows

package sdr

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var windowsRadioLibraries = []string{
	"libusb-1.0.dll",
	"rtlsdr.dll",
	"hackrf.dll",
	"libhackrf-0.dll",
	"libhackrf.dll",
	"airspy.dll",
	"airspyhf.dll",
}

func loadShared(path string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(path)
	if err != nil {
		return 0, err
	}
	return uintptr(handle), nil
}

func closeShared(handle uintptr) {
	if handle != 0 {
		_ = syscall.FreeLibrary(syscall.Handle(handle))
	}
}

func loadSoapy(config Config) (*soapyAPI, error) {
	root, err := filepath.Abs(config.RuntimeRoot)
	if err != nil {
		return nil, err
	}
	corePath := filepath.Join(root, "bin", "SoapySDR.dll")
	pluginDir := filepath.Join(root, "lib", "SoapySDR", "modules0.8")
	_ = os.Setenv("SOAPY_SDR_ROOT", root)
	_ = os.Setenv("SOAPY_SDR_PLUGIN_PATH", pluginDir)
	bin := filepath.Join(root, "bin")
	_ = os.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var radioPaths []string
	for _, name := range windowsRadioLibraries {
		radioPaths = append(radioPaths, filepath.Join(root, "bin", name))
	}
	deps := preloadLibraries(config, radioPaths)
	if vendor := firstExisting(windowsSDRplayAPICandidates()); vendor != "" {
		config.trace("SoapySDR: LoadLibrary %s", vendor)
		if handle, vendorErr := loadShared(vendor); vendorErr == nil {
			deps = append(deps, handle)
			config.trace("SoapySDR: SDRplay vendor API loaded")
		} else {
			config.trace("SoapySDR: SDRplay vendor API skipped: %v", vendorErr)
		}
	}
	config.trace("SoapySDR: LoadLibrary %s", corePath)
	core, err := loadShared(corePath)
	if err != nil {
		for index := len(deps) - 1; index >= 0; index-- {
			closeShared(deps[index])
		}
		return nil, fmt.Errorf("load %s: %w", corePath, err)
	}
	config.trace("SoapySDR: SoapySDR.dll loaded")
	api := registerSoapy(core)
	api.dependencies = deps
	api.loadModules(config, collectSoapyModules(pluginDir))
	return api, nil
}

func windowsSDRplayAPICandidates() []string {
	var out []string
	for _, root := range []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramW6432"),
		os.Getenv("ProgramFiles(x86)"),
	} {
		if root == "" {
			continue
		}
		out = append(out,
			filepath.Join(root, "SDRplay", "API", "x64", "sdrplay_api.dll"),
			filepath.Join(root, "SDRplay", "API", "arm64", "sdrplay_api.dll"),
			filepath.Join(root, "SDRplay", "API", "x86", "sdrplay_api.dll"),
		)
	}
	return uniqueStrings(out)
}

func firstExisting(paths []string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
