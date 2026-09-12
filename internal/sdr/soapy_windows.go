//go:build windows

package sdr

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

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
	config.trace("SoapySDR/%s: LoadLibrary %s", config.Driver, corePath)
	core, err := loadShared(corePath)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", corePath, err)
	}
	config.trace("SoapySDR/%s: SoapySDR.dll loaded", config.Driver)
	api := registerSoapy(core)
	moduleName := "rtlsdrSupport.dll"
	if config.Driver == "sdrplay" {
		vendorPath := filepath.Join(os.Getenv("ProgramFiles"), "SDRplay", "API", "x64", "sdrplay_api.dll")
		config.trace("SoapySDR/sdrplay: LoadLibrary %s", vendorPath)
		api.vendor, err = loadShared(vendorPath)
		if err != nil {
			api.close()
			return nil, fmt.Errorf("load SDRplay API %s: %w", vendorPath, err)
		}
		config.trace("SoapySDR/sdrplay: vendor API loaded")
		moduleName = "sdrPlaySupport.dll"
	} else if config.Driver == "rtlsdr" {
		for _, name := range []string{"libusb-1.0.dll", "rtlsdr.dll"} {
			dependencyPath := filepath.Join(root, "bin", name)
			config.trace("SoapySDR/rtlsdr: LoadLibrary %s", dependencyPath)
			handle, loadErr := loadShared(dependencyPath)
			if loadErr != nil {
				api.close()
				return nil, fmt.Errorf("load RTL-SDR dependency %s: %w", name, loadErr)
			}
			config.trace("SoapySDR/rtlsdr: %s loaded", name)
			api.dependencies = append(api.dependencies, handle)
		}
	}
	modulePath := filepath.Join(root, "lib", "SoapySDR", "modules0.8", moduleName)
	config.trace("SoapySDR/%s: loading module %s", config.Driver, modulePath)
	message := api.consume(api.loadModule(modulePath))
	if message != "" {
		api.close()
		return nil, fmt.Errorf("load Soapy module %s: %s", moduleName, message)
	}
	config.trace("SoapySDR/%s: module loaded", config.Driver)
	return api, nil
}
