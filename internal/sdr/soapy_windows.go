//go:build windows

package sdr

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	soapyRX       = 1
	soapyTimeout  = -1
	soapyOverflow = -4
)

type soapyAPI struct {
	core, vendor, module   syscall.Handle
	dependencies           []syscall.Handle
	loadModule             func(string) uintptr
	free                   func(uintptr)
	makeDevice             func(string) uintptr
	unmakeDevice           func(uintptr) int32
	hardwareKey            func(uintptr) uintptr
	lastError              func() uintptr
	setSampleRate          func(uintptr, int32, uintptr, float64) int32
	getSampleRate          func(uintptr, int32, uintptr) float64
	setFrequency           func(uintptr, int32, uintptr, float64, uintptr) int32
	getFrequency           func(uintptr, int32, uintptr) float64
	getFrequencyCorrection func(uintptr, int32, uintptr) float64
	setFrequencyCorrection func(uintptr, int32, uintptr, float64) int32
	getGainMode            func(uintptr, int32, uintptr) bool
	setGainMode            func(uintptr, int32, uintptr, bool) int32
	getGainElement         func(uintptr, int32, uintptr, string) float64
	setGainElement         func(uintptr, int32, uintptr, string, float64) int32
	readSetting            func(uintptr, string) uintptr
	writeSetting           func(uintptr, string, string) int32
	setupStream            func(uintptr, int32, string, uintptr, uintptr, uintptr) uintptr
	closeStream            func(uintptr, uintptr) int32
	activateStream         func(uintptr, uintptr, int32, int64, uintptr) int32
	deactivateStream       func(uintptr, uintptr, int32, int64) int32
	readStream             func(uintptr, uintptr, *uintptr, uintptr, *int32, *int64, int64) int32
	errToString            func(int32) uintptr
}

type soapyDevice struct {
	api        *soapyAPI
	device     uintptr
	stream     uintptr
	hardware   string
	driver     string
	sampleRate float64
	buffer     []float32
	buffers    [1]uintptr
}

func openSoapy(config Config) (*soapyDevice, error) {
	candidates := deviceCandidates(config)
	var failures []error
	for _, candidate := range candidates {
		config.trace("SoapySDR: probando driver=%s serial=%q", candidate.Driver, candidate.Serial)
		device, err := openSoapyCandidate(candidate)
		if err == nil {
			config.trace("SoapySDR: driver %s abierto", candidate.Driver)
			return device, nil
		}
		config.trace("SoapySDR: driver %s rechazado: %v", candidate.Driver, err)
		failures = append(failures, fmt.Errorf("%s: %w", candidate.Driver, err))
	}
	return nil, fmt.Errorf("no RSP or RTL-SDR found: %w", errors.Join(failures...))
}

func deviceCandidates(config Config) []Config {
	candidates := []Config{config}
	if config.Serial != "" {
		anyRSP := config
		anyRSP.Serial = ""
		candidates = append(candidates, anyRSP)
	}
	if config.Driver != "rtlsdr" {
		rtl := config
		rtl.Driver, rtl.Serial = "rtlsdr", ""
		candidates = append(candidates, rtl)
	}
	return candidates
}

func openSoapyCandidate(config Config) (result *soapyDevice, err error) {
	config.trace("SoapySDR/%s: loading DLL and module", config.Driver)
	api, err := loadSoapy(config)
	if err != nil {
		return nil, err
	}
	config.trace("SoapySDR/%s: creating device", config.Driver)
	device := api.makeDevice(config.deviceArguments())
	if device == 0 {
		message := api.deviceError()
		api.close()
		return nil, fmt.Errorf("SoapySDR make device: %s", message)
	}
	result = &soapyDevice{api: api, device: device, driver: config.Driver, buffer: make([]float32, config.FFTSize*2)}
	defer func() {
		if err != nil {
			result.close()
		}
	}()
	result.hardware = api.consume(api.hardwareKey(device))
	config.trace("SoapySDR/%s: configuring sample rate %.0f", config.Driver, config.SampleRate)
	if err = api.check(api.setSampleRate(device, soapyRX, 0, config.SampleRate), "set sample rate"); err != nil {
		return nil, err
	}
	result.sampleRate = api.getSampleRate(device, soapyRX, 0)
	config.trace("SoapySDR/%s: tuning %d Hz", config.Driver, config.FrequencyHz)
	if err = api.check(api.setFrequency(device, soapyRX, 0, float64(config.FrequencyHz), 0), "set frequency"); err != nil {
		return nil, err
	}
	config.trace("SoapySDR/%s: creando stream CF32", config.Driver)
	result.stream = api.setupStream(device, soapyRX, "CF32", 0, 0, 0)
	if result.stream == 0 {
		return nil, fmt.Errorf("setup CF32 stream: %s", api.deviceError())
	}
	config.trace("SoapySDR/%s: activando stream", config.Driver)
	if err = api.check(api.activateStream(device, result.stream, 0, 0, 0), "activate stream"); err != nil {
		return nil, err
	}
	config.trace("SoapySDR/%s: stream activo", config.Driver)
	return result, nil
}

func loadSoapy(config Config) (*soapyAPI, error) {
	root, err := filepath.Abs(config.RuntimeRoot)
	if err != nil {
		return nil, err
	}
	corePath := filepath.Join(root, "bin", "SoapySDR.dll")
	config.trace("SoapySDR/%s: LoadLibrary %s", config.Driver, corePath)
	core, err := syscall.LoadLibrary(corePath)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", corePath, err)
	}
	config.trace("SoapySDR/%s: SoapySDR.dll cargada", config.Driver)
	api := &soapyAPI{core: core}
	moduleName := "rtlsdrSupport.dll"
	if config.Driver == "sdrplay" {
		vendorPath := filepath.Join(os.Getenv("ProgramFiles"), "SDRplay", "API", "x64", "sdrplay_api.dll")
		config.trace("SoapySDR/sdrplay: LoadLibrary %s", vendorPath)
		api.vendor, err = syscall.LoadLibrary(vendorPath)
		if err != nil {
			api.close()
			return nil, fmt.Errorf("load SDRplay API %s: %w", vendorPath, err)
		}
		config.trace("SoapySDR/sdrplay: vendor API loaded")
		moduleName = "sdrPlaySupport.dll"
	} else if config.Driver == "rtlsdr" {
		// Load transitive DLLs by absolute path. Relying on PATH works in the
		// source tree but fails in the portable DATA layout beside the exe.
		for _, name := range []string{"libusb-1.0.dll", "rtlsdr.dll"} {
			dependencyPath := filepath.Join(root, "bin", name)
			config.trace("SoapySDR/rtlsdr: LoadLibrary %s", dependencyPath)
			handle, loadErr := syscall.LoadLibrary(dependencyPath)
			if loadErr != nil {
				api.close()
				return nil, fmt.Errorf("load RTL-SDR dependency %s: %w", name, loadErr)
			}
			config.trace("SoapySDR/rtlsdr: %s cargada", name)
			api.dependencies = append(api.dependencies, handle)
		}
	}
	modulePath := filepath.Join(root, "lib", "SoapySDR", "modules0.8", moduleName)
	config.trace("SoapySDR/%s: registering API symbols", config.Driver)
	purego.RegisterLibFunc(&api.loadModule, uintptr(core), "SoapySDR_loadModule")
	purego.RegisterLibFunc(&api.free, uintptr(core), "SoapySDR_free")
	purego.RegisterLibFunc(&api.makeDevice, uintptr(core), "SoapySDRDevice_makeStrArgs")
	purego.RegisterLibFunc(&api.unmakeDevice, uintptr(core), "SoapySDRDevice_unmake")
	purego.RegisterLibFunc(&api.hardwareKey, uintptr(core), "SoapySDRDevice_getHardwareKey")
	purego.RegisterLibFunc(&api.lastError, uintptr(core), "SoapySDRDevice_lastError")
	purego.RegisterLibFunc(&api.setSampleRate, uintptr(core), "SoapySDRDevice_setSampleRate")
	purego.RegisterLibFunc(&api.getSampleRate, uintptr(core), "SoapySDRDevice_getSampleRate")
	purego.RegisterLibFunc(&api.setFrequency, uintptr(core), "SoapySDRDevice_setFrequency")
	purego.RegisterLibFunc(&api.getFrequency, uintptr(core), "SoapySDRDevice_getFrequency")
	purego.RegisterLibFunc(&api.getFrequencyCorrection, uintptr(core), "SoapySDRDevice_getFrequencyCorrection")
	purego.RegisterLibFunc(&api.setFrequencyCorrection, uintptr(core), "SoapySDRDevice_setFrequencyCorrection")
	purego.RegisterLibFunc(&api.getGainMode, uintptr(core), "SoapySDRDevice_getGainMode")
	purego.RegisterLibFunc(&api.setGainMode, uintptr(core), "SoapySDRDevice_setGainMode")
	purego.RegisterLibFunc(&api.getGainElement, uintptr(core), "SoapySDRDevice_getGainElement")
	purego.RegisterLibFunc(&api.setGainElement, uintptr(core), "SoapySDRDevice_setGainElement")
	purego.RegisterLibFunc(&api.readSetting, uintptr(core), "SoapySDRDevice_readSetting")
	purego.RegisterLibFunc(&api.writeSetting, uintptr(core), "SoapySDRDevice_writeSetting")
	purego.RegisterLibFunc(&api.setupStream, uintptr(core), "SoapySDRDevice_setupStream")
	purego.RegisterLibFunc(&api.closeStream, uintptr(core), "SoapySDRDevice_closeStream")
	purego.RegisterLibFunc(&api.activateStream, uintptr(core), "SoapySDRDevice_activateStream")
	purego.RegisterLibFunc(&api.deactivateStream, uintptr(core), "SoapySDRDevice_deactivateStream")
	purego.RegisterLibFunc(&api.readStream, uintptr(core), "SoapySDRDevice_readStream")
	purego.RegisterLibFunc(&api.errToString, uintptr(core), "SoapySDR_errToStr")

	config.trace("SoapySDR/%s: loading module %s", config.Driver, modulePath)
	message := api.consume(api.loadModule(modulePath))
	if message != "" {
		api.close()
		return nil, fmt.Errorf("load Soapy module %s: %s", moduleName, message)
	}
	config.trace("SoapySDR/%s: module loaded", config.Driver)
	return api, nil
}

func (device *soapyDevice) read(destination []float32) (int, int32, error) {
	requested := min(len(destination)/2, len(device.buffer)/2)
	device.buffers[0] = uintptr(unsafe.Pointer(&device.buffer[0]))
	var flags int32
	var timestamp int64
	read := device.api.readStream(device.device, device.stream, &device.buffers[0], uintptr(requested), &flags, &timestamp, 100_000)
	if read == soapyTimeout || read == soapyOverflow {
		return 0, read, nil
	}
	if read < 0 {
		return 0, read, fmt.Errorf("read stream: %s", device.api.errorText(read))
	}
	copy(destination, device.buffer[:int(read)*2])
	return int(read), read, nil
}

func (device *soapyDevice) setCenterFrequency(frequencyHz int64) error {
	return device.api.check(
		device.api.setFrequency(device.device, soapyRX, 0, float64(frequencyHz), 0),
		"set center frequency",
	)
}

func (device *soapyDevice) centerFrequency() int64 {
	return int64(math.Round(device.api.getFrequency(device.device, soapyRX, 0)))
}

func (device *soapyDevice) hardwareSettings() HardwareSettings {
	if device.driver == "rtlsdr" {
		return HardwareSettings{
			Available: true, Device: device.hardware, Driver: device.driver,
			AGC:            device.api.getGainMode(device.device, soapyRX, 0),
			RFGain:         float32(device.api.getGainElement(device.device, soapyRX, 0, "TUNER")),
			PPM:            float32(device.api.getFrequencyCorrection(device.device, soapyRX, 0)),
			BiasT:          device.readBoolSetting("biastee"),
			DigitalAGC:     device.readBoolSetting("digital_agc"),
			OffsetTuning:   device.readBoolSetting("offset_tune"),
			IQSwap:         device.readBoolSetting("iq_swap"),
			DirectSampling: device.readIntSetting("direct_samp", 0),
		}
	}
	return HardwareSettings{
		Available: true, Device: device.hardware, Driver: device.driver,
		AGC:          device.api.getGainMode(device.device, soapyRX, 0),
		RFGain:       float32(device.api.getGainElement(device.device, soapyRX, 0, "RFGR")),
		IFGain:       float32(device.api.getGainElement(device.device, soapyRX, 0, "IFGR")),
		PPM:          float32(device.api.getFrequencyCorrection(device.device, soapyRX, 0)),
		BiasT:        device.readBoolSetting("biasT_ctrl"),
		RFNotch:      device.readBoolSetting("rfnotch_ctrl"),
		DABNotch:     device.readBoolSetting("dabnotch_ctrl"),
		IQCorrection: device.readBoolSetting("iqcorr_ctrl"),
		AGCSetpoint:  device.readIntSetting("agc_setpoint", -30),
	}
}

func (device *soapyDevice) applyHardwareSettings(settings HardwareSettings) error {
	current := device.hardwareSettings()
	var failures []error
	apply := func(code int32, name string) {
		if err := device.api.check(code, name); err != nil {
			failures = append(failures, err)
		}
	}
	if device.driver == "rtlsdr" {
		if current.AGC != settings.AGC {
			apply(device.api.setGainMode(device.device, soapyRX, 0, settings.AGC), "RTL-SDR AGC")
		}
		if !settings.AGC && current.RFGain != settings.RFGain {
			apply(device.api.setGainElement(device.device, soapyRX, 0, "TUNER", float64(settings.RFGain)), "RTL-SDR tuner gain")
		}
		if current.PPM != settings.PPM {
			apply(device.api.setFrequencyCorrection(device.device, soapyRX, 0, float64(settings.PPM)), "RTL-SDR frequency correction")
		}
		if current.BiasT != settings.BiasT {
			apply(device.api.writeSetting(device.device, "biastee", boolString(settings.BiasT)), "RTL-SDR Bias-T")
		}
		if current.DigitalAGC != settings.DigitalAGC {
			apply(device.api.writeSetting(device.device, "digital_agc", boolString(settings.DigitalAGC)), "RTL-SDR digital AGC")
		}
		if current.OffsetTuning != settings.OffsetTuning {
			apply(device.api.writeSetting(device.device, "offset_tune", boolString(settings.OffsetTuning)), "RTL-SDR offset tuning")
		}
		if current.IQSwap != settings.IQSwap {
			apply(device.api.writeSetting(device.device, "iq_swap", boolString(settings.IQSwap)), "RTL-SDR IQ swap")
		}
		if current.DirectSampling != settings.DirectSampling {
			apply(device.api.writeSetting(device.device, "direct_samp", fmt.Sprintf("%d", settings.DirectSampling)), "RTL-SDR direct sampling")
		}
		return errors.Join(failures...)
	}

	gainChanged := current.RFGain != settings.RFGain || current.IFGain != settings.IFGain
	if (current.AGC && gainChanged) || (current.AGC && !settings.AGC) {
		apply(device.api.setGainMode(device.device, soapyRX, 0, false), "disable AGC")
	}
	if current.RFGain != settings.RFGain {
		apply(device.api.setGainElement(device.device, soapyRX, 0, "RFGR", float64(settings.RFGain)), "RFGR")
	}
	if current.IFGain != settings.IFGain {
		apply(device.api.setGainElement(device.device, soapyRX, 0, "IFGR", float64(settings.IFGain)), "IFGR")
	}
	if current.PPM != settings.PPM {
		apply(device.api.setFrequencyCorrection(device.device, soapyRX, 0, float64(settings.PPM)), "frequency correction")
	}
	if current.BiasT != settings.BiasT {
		apply(device.api.writeSetting(device.device, "biasT_ctrl", boolString(settings.BiasT)), "Bias-T")
	}
	if current.RFNotch != settings.RFNotch {
		apply(device.api.writeSetting(device.device, "rfnotch_ctrl", boolString(settings.RFNotch)), "RF notch")
	}
	if current.DABNotch != settings.DABNotch {
		apply(device.api.writeSetting(device.device, "dabnotch_ctrl", boolString(settings.DABNotch)), "DAB notch")
	}
	if current.IQCorrection != settings.IQCorrection {
		apply(device.api.writeSetting(device.device, "iqcorr_ctrl", boolString(settings.IQCorrection)), "IQ correction")
	}
	if current.AGCSetpoint != settings.AGCSetpoint {
		apply(device.api.writeSetting(device.device, "agc_setpoint", fmt.Sprintf("%d", settings.AGCSetpoint)), "AGC setpoint")
	}
	if current.AGC != settings.AGC || (settings.AGC && gainChanged) {
		apply(device.api.setGainMode(device.device, soapyRX, 0, settings.AGC), "AGC")
	}
	return errors.Join(failures...)
}

func (device *soapyDevice) readBoolSetting(key string) bool {
	return device.api.consume(device.api.readSetting(device.device, key)) == "true"
}

func (device *soapyDevice) readIntSetting(key string, fallback int) int {
	value := device.api.consume(device.api.readSetting(device.device, key))
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return fallback
	}
	return result
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func (device *soapyDevice) close() {
	if device == nil || device.api == nil {
		return
	}
	if device.device != 0 && device.stream != 0 {
		device.api.deactivateStream(device.device, device.stream, 0, 0)
		device.api.closeStream(device.device, device.stream)
		device.stream = 0
	}
	if device.device != 0 {
		device.api.unmakeDevice(device.device)
		device.device = 0
	}
	// SDRplay API can still finish internal callbacks after unmake. Keep the
	// process-wide DLL handles loaded; Windows releases them safely on exit.
	device.api = nil
}

func (api *soapyAPI) check(code int32, operation string) error {
	if code == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s (%s)", operation, api.errorText(code), api.deviceError())
}

func (api *soapyAPI) errorText(code int32) string { return cString(api.errToString(code)) }
func (api *soapyAPI) deviceError() string         { return cString(api.lastError()) }

func (api *soapyAPI) consume(pointer uintptr) string {
	if pointer == 0 {
		return ""
	}
	value := cString(pointer)
	api.free(pointer)
	return value
}

func (api *soapyAPI) close() {
	if api.module != 0 {
		syscall.FreeLibrary(api.module)
		api.module = 0
	}
	if api.core != 0 {
		syscall.FreeLibrary(api.core)
		api.core = 0
	}
	if api.vendor != 0 {
		syscall.FreeLibrary(api.vendor)
		api.vendor = 0
	}
	for index := len(api.dependencies) - 1; index >= 0; index-- {
		syscall.FreeLibrary(api.dependencies[index])
	}
	api.dependencies = nil
}

func cString(pointer uintptr) string {
	if pointer == 0 {
		return ""
	}
	bytes := make([]byte, 0, 128)
	for offset := uintptr(0); ; offset++ {
		value := *(*byte)(unsafe.Pointer(pointer + offset))
		if value == 0 {
			return string(bytes)
		}
		bytes = append(bytes, value)
	}
}
