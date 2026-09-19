//go:build windows

package sdr

import (
	"go-zero/internal/i18n"

	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
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
	enumerate              func(string, *uintptr) uintptr
	enumerateClear         func(uintptr, uintptr)
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
	getGain                func(uintptr, int32, uintptr) float64
	setGain                func(uintptr, int32, uintptr, float64) int32
	listAntennas           func(uintptr, int32, uintptr, *uintptr) uintptr
	getAntenna             func(uintptr, int32, uintptr) uintptr
	setAntenna             func(uintptr, int32, uintptr, string) int32
	stringsClear           func(*uintptr, uintptr)
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
	serial     string
	antenna    string
	antennas   []string
	sampleRate float64
	buffer     []float32
	buffers    [1]uintptr
}

func openSoapy(config Config) (*soapyDevice, error) {
	candidates := deviceCandidates(config)
	var failures []error
	for _, candidate := range candidates {
		config.trace(i18n.Source("text.8ac7b4eb296e"), candidate.Driver, candidate.Serial)
		device, err := openSoapyCandidate(candidate)
		if err == nil {
			config.trace(i18n.Source("text.27ccf6128f91"), candidate.Driver)
			return device, nil
		}
		config.trace(i18n.Source("text.14909b682516"), candidate.Driver, err)
		failures = append(failures, fmt.Errorf("%s: %w", candidate.Driver, err))
	}
	return nil, fmt.Errorf(i18n.Source("text.2221fda17c81"), errors.Join(failures...))
}

func openSoapyExact(config Config) (*soapyDevice, error) { return openSoapyCandidate(config) }

type soapyKwargs struct{ size, keys, vals uintptr }

var scanner struct {
	sync.Mutex
	api    *soapyAPI
	loaded map[string]bool
}

func listSoapyDevices(config Config) ([]DeviceOption, error) {
	scanner.Lock()
	defer scanner.Unlock()
	if scanner.loaded == nil {
		scanner.loaded = make(map[string]bool)
	}
	var firstErr error
	for _, driver := range []string{"sdrplay", "rtlsdr", "hackrf"} {
		if driver == "hackrf" && !hasHackRFModule(config.RuntimeRoot) {
			continue
		}
		if scanner.loaded[driver] {
			continue
		}
		candidate := config
		candidate.Driver = driver
		api, err := loadSoapy(candidate)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		scanner.loaded[driver] = true
		scanner.api = api // Keep DLLs loaded while the process uses SoapySDR.
	}
	if scanner.api == nil {
		return nil, firstErr
	}
	var length uintptr
	list := scanner.api.enumerate("", &length)
	if list == 0 || length == 0 {
		return nil, nil
	}
	defer scanner.api.enumerateClear(list, length)
	result := make([]DeviceOption, 0, int(length))
	stride, pointerSize := unsafe.Sizeof(soapyKwargs{}), unsafe.Sizeof(uintptr(0))
	for index := uintptr(0); index < length; index++ {
		kwargs := (*soapyKwargs)(unsafe.Pointer(list + index*stride))
		option := DeviceOption{}
		for pair := uintptr(0); pair < kwargs.size; pair++ {
			key := *(*uintptr)(unsafe.Pointer(kwargs.keys + pair*pointerSize))
			value := *(*uintptr)(unsafe.Pointer(kwargs.vals + pair*pointerSize))
			switch cString(key) {
			case "driver":
				option.Driver = cString(value)
			case "serial":
				option.Serial = cString(value)
			case "label":
				option.Label = cString(value)
			}
		}
		if option.Driver == "sdrplay" || option.Driver == "rtlsdr" || option.Driver == "hackrf" {
			result = append(result, option)
		}
	}
	return mergeDeviceOptions(result, nil), nil
}

func deviceCandidates(config Config) []Config {
	candidates := []Config{config}
	// A saved serial may have disappeared. Try another unit of the same kind
	// before moving on to the other installed receiver families.
	if config.Serial != "" {
		any := config
		any.Serial = ""
		candidates = append(candidates, any)
	}
	for _, driver := range []string{"sdrplay", "rtlsdr", "hackrf"} {
		if driver == config.Driver {
			continue
		}
		fallback := config
		fallback.Driver, fallback.Serial = driver, ""
		candidates = append(candidates, fallback)
	}
	return candidates
}

func hasHackRFModule(root string) bool {
	_, err := os.Stat(filepath.Join(root, "lib", "SoapySDR", "modules0.8", "HackRFSupport.dll"))
	return err == nil
}

func openSoapyCandidate(config Config) (result *soapyDevice, err error) {
	config.trace(i18n.Source("text.4b60d3b38649"), config.Driver)
	api, err := loadSoapy(config)
	if err != nil {
		return nil, err
	}
	scanner.Lock()
	if scanner.loaded == nil {
		scanner.loaded = make(map[string]bool)
	}
	scanner.loaded[config.Driver] = true
	scanner.api = api
	scanner.Unlock()
	if config.Serial == "" {
		var count uintptr
		list := api.enumerate("driver="+config.Driver, &count)
		if list != 0 {
			if count > 0 {
				info := (*soapyKwargs)(unsafe.Pointer(list))
				step := unsafe.Sizeof(uintptr(0))
				for i := uintptr(0); i < info.size; i++ {
					if cString(*(*uintptr)(unsafe.Pointer(info.keys + i*step))) == "serial" {
						config.Serial = cString(*(*uintptr)(unsafe.Pointer(info.vals + i*step)))
						break
					}
				}
			}
			api.enumerateClear(list, count)
		}
	}
	config.trace(i18n.Source("text.ea33a82d950f"), config.Driver)
	device := api.makeDevice(config.deviceArguments())
	if device == 0 {
		message := api.deviceError()
		return nil, fmt.Errorf(i18n.Source("text.243b35027c16"), message)
	}
	result = &soapyDevice{api: api, device: device, driver: config.Driver, serial: config.Serial, buffer: make([]float32, config.FFTSize*2)}
	defer func() {
		if err != nil {
			result.close()
		}
	}()
	result.hardware = api.consume(api.hardwareKey(device))
	result.refreshAntennas()
	config.trace(i18n.Source("text.62c09cdaa0ef"), config.Driver, config.SampleRate)
	if err = api.check(api.setSampleRate(device, soapyRX, 0, config.SampleRate), i18n.Source("text.27405f9cff8f")); err != nil {
		return nil, err
	}
	result.sampleRate = api.getSampleRate(device, soapyRX, 0)
	config.trace(i18n.Source("text.d028519ac0c5"), config.Driver, config.FrequencyHz)
	if err = api.check(api.setFrequency(device, soapyRX, 0, float64(config.FrequencyHz), 0), i18n.Source("text.cf4982d2de5b")); err != nil {
		return nil, err
	}
	config.trace(i18n.Source("text.3bf73f2ea947"), config.Driver)
	result.stream = api.setupStream(device, soapyRX, "CF32", 0, 0, 0)
	if result.stream == 0 {
		return nil, fmt.Errorf(i18n.Source("text.7bad50125987"), api.deviceError())
	}
	config.trace(i18n.Source("text.567829dc0738"), config.Driver)
	if err = api.check(api.activateStream(device, result.stream, 0, 0, 0), i18n.Source("text.66abcbe21a7d")); err != nil {
		return nil, err
	}
	if result.antenna != "" && len(result.antennas) > 1 {
		if antennaErr := result.setAntenna(result.antenna); antennaErr != nil {
			config.trace(i18n.Source("text.0f11289a3f2a"), config.Driver, antennaErr)
		}
	}
	config.trace(i18n.Source("text.b3924c740b11"), config.Driver)
	return result, nil
}

func loadSoapy(config Config) (*soapyAPI, error) {
	root, err := filepath.Abs(config.RuntimeRoot)
	if err != nil {
		return nil, err
	}
	corePath := filepath.Join(root, "bin", "SoapySDR.dll")
	config.trace(i18n.Source("text.bc4e5a652327"), config.Driver, corePath)
	core, err := syscall.LoadLibrary(corePath)
	if err != nil {
		return nil, fmt.Errorf(i18n.Source("text.415376b5d3b0"), corePath, err)
	}
	config.trace(i18n.Source("text.f0bba3ff5b10"), config.Driver)
	api := &soapyAPI{core: core}
	moduleName := "rtlsdrSupport.dll"
	if config.Driver == "sdrplay" {
		vendorPath := filepath.Join(os.Getenv("ProgramFiles"), "SDRplay", i18n.Source("text.c8e5998f6a39"), "x64", "sdrplay_api.dll")
		config.trace(i18n.Source("text.70f3f1e0d5a1"), vendorPath)
		api.vendor, err = syscall.LoadLibrary(vendorPath)
		if err != nil {
			api.close()
			return nil, fmt.Errorf(i18n.Source("text.58136254d6d7"), vendorPath, err)
		}
		config.trace(i18n.Source("text.bc9b492c772d"))
		moduleName = "sdrPlaySupport.dll"
	} else if config.Driver == "rtlsdr" {
		// Load transitive DLLs by absolute path. Relying on PATH works in the
		// source tree but fails in the portable DATA layout beside the exe.
		for _, name := range []string{"libusb-1.0.dll", "rtlsdr.dll"} {
			dependencyPath := filepath.Join(root, "bin", name)
			config.trace(i18n.Source("text.4a6e591a09c9"), dependencyPath)
			handle, loadErr := syscall.LoadLibrary(dependencyPath)
			if loadErr != nil {
				api.close()
				return nil, fmt.Errorf(i18n.Source("text.5422deaa347c"), name, loadErr)
			}
			config.trace(i18n.Source("text.6867dd214f65"), name)
			api.dependencies = append(api.dependencies, handle)
		}
	} else if config.Driver == "hackrf" {
		moduleName = "HackRFSupport.dll"
		for _, name := range []string{"libusb-1.0.dll", "pthreadVC3.dll", "hackrf.dll"} {
			dependencyPath := filepath.Join(root, "bin", name)
			handle, loadErr := syscall.LoadLibrary(dependencyPath)
			if loadErr != nil {
				api.close()
				return nil, fmt.Errorf(i18n.Source("text.ddeb9d145739"), name, loadErr)
			}
			api.dependencies = append(api.dependencies, handle)
		}
	}
	modulePath := filepath.Join(root, "lib", "SoapySDR", "modules0.8", moduleName)
	config.trace(i18n.Source("text.e8fe4854f48e"), config.Driver)
	purego.RegisterLibFunc(&api.loadModule, uintptr(core), "SoapySDR_loadModule")
	purego.RegisterLibFunc(&api.enumerate, uintptr(core), "SoapySDRDevice_enumerateStrArgs")
	purego.RegisterLibFunc(&api.enumerateClear, uintptr(core), "SoapySDRKwargsList_clear")
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
	purego.RegisterLibFunc(&api.getGain, uintptr(core), "SoapySDRDevice_getGain")
	purego.RegisterLibFunc(&api.setGain, uintptr(core), "SoapySDRDevice_setGain")
	purego.RegisterLibFunc(&api.listAntennas, uintptr(core), "SoapySDRDevice_listAntennas")
	purego.RegisterLibFunc(&api.getAntenna, uintptr(core), "SoapySDRDevice_getAntenna")
	purego.RegisterLibFunc(&api.setAntenna, uintptr(core), "SoapySDRDevice_setAntenna")
	purego.RegisterLibFunc(&api.stringsClear, uintptr(core), "SoapySDRStrings_clear")
	purego.RegisterLibFunc(&api.readSetting, uintptr(core), "SoapySDRDevice_readSetting")
	purego.RegisterLibFunc(&api.writeSetting, uintptr(core), "SoapySDRDevice_writeSetting")
	purego.RegisterLibFunc(&api.setupStream, uintptr(core), "SoapySDRDevice_setupStream")
	purego.RegisterLibFunc(&api.closeStream, uintptr(core), "SoapySDRDevice_closeStream")
	purego.RegisterLibFunc(&api.activateStream, uintptr(core), "SoapySDRDevice_activateStream")
	purego.RegisterLibFunc(&api.deactivateStream, uintptr(core), "SoapySDRDevice_deactivateStream")
	purego.RegisterLibFunc(&api.readStream, uintptr(core), "SoapySDRDevice_readStream")
	purego.RegisterLibFunc(&api.errToString, uintptr(core), "SoapySDR_errToStr")

	config.trace(i18n.Source("text.e8b27774cadc"), config.Driver, modulePath)
	message := api.consume(api.loadModule(modulePath))
	if !moduleLoadSucceeded(message, modulePath) {
		api.close()
		return nil, fmt.Errorf(i18n.Source("text.492af6d66f82"), moduleName, message)
	}
	config.trace(i18n.Source("text.29ef816f4a0f"), config.Driver)
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
		return 0, read, fmt.Errorf(i18n.Source("text.d6b1d17ba435"), device.api.errorText(read))
	}
	copy(destination, device.buffer[:int(read)*2])
	return int(read), read, nil
}

func (device *soapyDevice) setCenterFrequency(frequencyHz int64) error {
	if err := device.api.check(
		device.api.setFrequency(device.device, soapyRX, 0, float64(frequencyHz), 0),
		i18n.Source("text.9144d33a1202"),
	); err != nil {
		return err
	}
	if device.antenna != "" && len(device.antennas) > 1 {
		return device.setAntenna(device.antenna)
	}
	return nil
}

func (device *soapyDevice) centerFrequency() int64 {
	return int64(math.Round(device.api.getFrequency(device.device, soapyRX, 0)))
}

func (device *soapyDevice) refreshAntennas() {
	if !IsRSPDx(device.hardware) {
		return
	}
	var count uintptr
	list := device.api.listAntennas(device.device, soapyRX, 0, &count)
	if list == 0 || count == 0 || count > 16 {
		return
	}
	defer device.api.stringsClear(&list, count)
	step := unsafe.Sizeof(uintptr(0))
	for i := uintptr(0); i < count; i++ {
		name := cString(*(*uintptr)(unsafe.Pointer(list + i*step)))
		if name != "" {
			device.antennas = append(device.antennas, name)
		}
	}
	if len(device.antennas) > 0 {
		device.antenna = MatchAntenna(device.api.consume(device.api.getAntenna(device.device, soapyRX, 0)), device.antennas)
	}
}

func (device *soapyDevice) antennaState() (string, []string) {
	if len(device.antennas) <= 1 {
		return "", nil
	}
	current := MatchAntenna(device.api.consume(device.api.getAntenna(device.device, soapyRX, 0)), device.antennas)
	if current != "" {
		device.antenna = current
	}
	return device.antenna, append([]string(nil), device.antennas...)
}

func (device *soapyDevice) setAntenna(name string) error {
	name = MatchAntenna(name, device.antennas)
	if name == "" {
		return fmt.Errorf("%s", i18n.Source("text.0fcfcf3eb4a5"))
	}
	if err := device.api.check(device.api.setAntenna(device.device, soapyRX, 0, name), i18n.Source("text.b6e4f64a15cc")); err != nil {
		return err
	}
	device.antenna = name
	return nil
}

func (device *soapyDevice) hardwareSettings() HardwareSettings {
	antenna, antennas := device.antennaState()
	if device.driver == "rtlsdr" {
		return HardwareSettings{
			Available: true, Device: device.hardware, Driver: device.driver, Serial: device.serial,
			AGC:            device.api.getGainMode(device.device, soapyRX, 0),
			RFGain:         float32(device.api.getGainElement(device.device, soapyRX, 0, i18n.Source("text.91c1fd825c5a"))),
			PPM:            float32(device.api.getFrequencyCorrection(device.device, soapyRX, 0)),
			BiasT:          device.readBoolSetting("biastee"),
			DigitalAGC:     device.readBoolSetting("digital_agc"),
			OffsetTuning:   device.readBoolSetting("offset_tune"),
			IQSwap:         device.readBoolSetting("iq_swap"),
			DirectSampling: device.readIntSetting("direct_samp", 0),
		}
	}
	if device.driver == "hackrf" {
		return HardwareSettings{
			Available: true, Device: device.hardware, Driver: device.driver, Serial: device.serial,
			RFGain:      float32(device.api.getGainElement(device.device, soapyRX, 0, "LNA")),
			IFGain:      float32(device.api.getGainElement(device.device, soapyRX, 0, "VGA")),
			ExternalAmp: device.api.getGainElement(device.device, soapyRX, 0, "AMP") > 0,
			BiasT:       device.readBoolSetting("bias_tx"),
		}
	}
	return HardwareSettings{
		Available: true, Device: device.hardware, Driver: device.driver, Serial: device.serial,
		Antenna: antenna, Antennas: antennas,
		AGC:          device.api.getGainMode(device.device, soapyRX, 0),
		RFGain:       float32(device.api.getGainElement(device.device, soapyRX, 0, i18n.Source("text.a5a6f8a6d9f7"))),
		IFGain:       float32(device.api.getGainElement(device.device, soapyRX, 0, i18n.Source("text.beb717ff2ec6"))),
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
			apply(device.api.setGainMode(device.device, soapyRX, 0, settings.AGC), i18n.Source("text.55a69b8d806b"))
		}
		if !settings.AGC && current.RFGain != settings.RFGain {
			apply(device.api.setGainElement(device.device, soapyRX, 0, i18n.Source("text.91c1fd825c5a"), float64(settings.RFGain)), i18n.Source("text.1a4cbbb95852"))
		}
		if current.PPM != settings.PPM {
			apply(device.api.setFrequencyCorrection(device.device, soapyRX, 0, float64(settings.PPM)), i18n.Source("text.4e40828c2907"))
		}
		if current.BiasT != settings.BiasT {
			apply(device.api.writeSetting(device.device, "biastee", boolString(settings.BiasT)), i18n.Source("text.a3faa3c8bc3f"))
		}
		if current.DigitalAGC != settings.DigitalAGC {
			apply(device.api.writeSetting(device.device, "digital_agc", boolString(settings.DigitalAGC)), i18n.Source("text.b1d91b77232c"))
		}
		if current.OffsetTuning != settings.OffsetTuning {
			apply(device.api.writeSetting(device.device, "offset_tune", boolString(settings.OffsetTuning)), i18n.Source("text.aa586adc01d7"))
		}
		if current.IQSwap != settings.IQSwap {
			apply(device.api.writeSetting(device.device, "iq_swap", boolString(settings.IQSwap)), i18n.Source("text.c700b2ff4e9e"))
		}
		if current.DirectSampling != settings.DirectSampling {
			apply(device.api.writeSetting(device.device, "direct_samp", fmt.Sprintf("%d", settings.DirectSampling)), i18n.Source("text.8b4630cd6d1e"))
		}
		return errors.Join(failures...)
	}
	if device.driver == "hackrf" {
		if current.RFGain != settings.RFGain {
			apply(device.api.setGainElement(device.device, soapyRX, 0, "LNA", float64(settings.RFGain)), "HackRF LNA")
		}
		if current.IFGain != settings.IFGain {
			apply(device.api.setGainElement(device.device, soapyRX, 0, "VGA", float64(settings.IFGain)), "HackRF VGA")
		}
		if current.ExternalAmp != settings.ExternalAmp {
			gain := float64(0)
			if settings.ExternalAmp {
				gain = 14
			}
			apply(device.api.setGainElement(device.device, soapyRX, 0, "AMP", gain), "HackRF AMP")
		}
		if current.BiasT != settings.BiasT {
			apply(device.api.writeSetting(device.device, "bias_tx", boolString(settings.BiasT)), "HackRF Bias-T")
		}
		return errors.Join(failures...)
	}

	gainChanged := current.RFGain != settings.RFGain || current.IFGain != settings.IFGain
	if (current.AGC && gainChanged) || (current.AGC && !settings.AGC) {
		apply(device.api.setGainMode(device.device, soapyRX, 0, false), i18n.Source("text.62e9fda52b66"))
	}
	if current.RFGain != settings.RFGain {
		apply(device.api.setGainElement(device.device, soapyRX, 0, i18n.Source("text.a5a6f8a6d9f7"), float64(settings.RFGain)), i18n.Source("text.a5a6f8a6d9f7"))
	}
	if current.IFGain != settings.IFGain {
		apply(device.api.setGainElement(device.device, soapyRX, 0, i18n.Source("text.beb717ff2ec6"), float64(settings.IFGain)), i18n.Source("text.beb717ff2ec6"))
	}
	if current.PPM != settings.PPM {
		apply(device.api.setFrequencyCorrection(device.device, soapyRX, 0, float64(settings.PPM)), i18n.Source("text.93fda1bda2d8"))
	}
	if current.BiasT != settings.BiasT {
		apply(device.api.writeSetting(device.device, "biasT_ctrl", boolString(settings.BiasT)), "Bias-T")
	}
	if current.RFNotch != settings.RFNotch {
		apply(device.api.writeSetting(device.device, "rfnotch_ctrl", boolString(settings.RFNotch)), i18n.Source("text.8096b7d67b23"))
	}
	if current.DABNotch != settings.DABNotch {
		apply(device.api.writeSetting(device.device, "dabnotch_ctrl", boolString(settings.DABNotch)), i18n.Source("text.5e48685f29c5"))
	}
	if current.IQCorrection != settings.IQCorrection {
		apply(device.api.writeSetting(device.device, "iqcorr_ctrl", boolString(settings.IQCorrection)), i18n.Source("text.f515ed0cfa08"))
	}
	if current.AGCSetpoint != settings.AGCSetpoint {
		apply(device.api.writeSetting(device.device, "agc_setpoint", fmt.Sprintf("%d", settings.AGCSetpoint)), i18n.Source("text.8814c5e8f044"))
	}
	if current.AGC != settings.AGC || (settings.AGC && gainChanged) {
		apply(device.api.setGainMode(device.device, soapyRX, 0, settings.AGC), i18n.Source("text.20e0541e8b46"))
	}
	if settings.Antenna != "" && settings.Antenna != current.Antenna {
		if err := device.setAntenna(settings.Antenna); err != nil {
			failures = append(failures, err)
		}
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
	return fmt.Errorf(i18n.Source("text.b6e27aa7ff9c"), operation, api.errorText(code), api.deviceError())
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
