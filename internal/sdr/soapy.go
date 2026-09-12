package sdr

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	soapyRX       = 1
	soapyTimeout  = -1
	soapyOverflow = -4
)

// ControlProfile selects which hardware sliders and settings a driver understands.
type ControlProfile int

const (
	ProfileGeneric ControlProfile = iota
	ProfileRTLSDR
	ProfileSDRplay
)

// ProfileFor maps a Soapy driver name onto the UI/backend control layout.
func ProfileFor(driver string) ControlProfile {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "rtlsdr":
		return ProfileRTLSDR
	case "sdrplay", "sdrplay3":
		return ProfileSDRplay
	default:
		return ProfileGeneric
	}
}

var soapyProbeDrivers = []string{
	"sdrplay", "rtlsdr", "sdrplay3", "airspy", "airspyhf", "hackrf",
	"lime", "plutosdr", "miri", "uhd", "bladerf", "rfspace", "redpitaya", "remote",
}

var soapyBiasSettings = []string{"biastee", "bias_tee", "biasT_ctrl", "bias_tx"}

type soapyIdentity struct {
	Driver string
	Serial string
	Label  string
}

// DeviceOption is one enumerated radio the user can pick in the header.
type DeviceOption struct {
	Driver, Serial, Label string
}

func FormatDeviceLabel(opt DeviceOption) string {
	name := strings.TrimSpace(opt.Label)
	if name == "" {
		name = strings.TrimSpace(opt.Driver)
	}
	if name == "" {
		name = "SDR"
	}
	if opt.Serial != "" {
		serial := opt.Serial
		if len(serial) > 10 {
			serial = serial[len(serial)-8:]
		}
		return name + " · " + serial
	}
	if opt.Driver != "" && !strings.EqualFold(name, opt.Driver) {
		return name + " · " + opt.Driver
	}
	return name
}

var (
	soapyLoadMu sync.Mutex
	soapyOpMu   sync.Mutex
	soapyLoaded *soapyAPI
)

func sharedSoapy(config Config) (*soapyAPI, error) {
	soapyLoadMu.Lock()
	defer soapyLoadMu.Unlock()
	if soapyLoaded != nil {
		return soapyLoaded, nil
	}
	api, err := loadSoapy(config)
	if err != nil {
		return nil, err
	}
	soapyLoaded = api
	return api, nil
}

func identitiesToOptions(list []soapyIdentity) []DeviceOption {
	out := make([]DeviceOption, 0, len(list))
	seen := map[string]struct{}{}
	for _, ident := range list {
		key := strings.ToLower(ident.Driver) + "\x00" + ident.Serial
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, DeviceOption{Driver: ident.Driver, Serial: ident.Serial, Label: ident.Label})
	}
	return out
}

func mergeDeviceOptions(base, extra []DeviceOption) []DeviceOption {
	out := append([]DeviceOption(nil), base...)
	seen := map[string]struct{}{}
	for _, opt := range out {
		seen[strings.ToLower(opt.Driver)+"\x00"+opt.Serial] = struct{}{}
	}
	for _, opt := range extra {
		key := strings.ToLower(opt.Driver) + "\x00" + opt.Serial
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, opt)
	}
	return out
}

func listSoapyDevices(config Config) ([]DeviceOption, error) {
	api, err := sharedSoapy(config)
	if err != nil {
		return nil, err
	}
	soapyOpMu.Lock()
	defer soapyOpMu.Unlock()
	return identitiesToOptions(api.discoverDevices(config)), nil
}

type soapyKwargs struct {
	size uintptr
	keys uintptr
	vals uintptr
}

type soapyAPI struct {
	core, vendor, module   uintptr
	dependencies           []uintptr
	loadModule             func(string) uintptr
	free                   func(uintptr)
	enumerate              func(string, *uintptr) uintptr
	enumerateClear         func(uintptr, uintptr)
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
	getGain                func(uintptr, int32, uintptr) float64
	setGain                func(uintptr, int32, uintptr, float64) int32
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
	listAntennas           func(uintptr, int32, uintptr, *uintptr) uintptr
	getAntenna             func(uintptr, int32, uintptr) uintptr
	setAntenna             func(uintptr, int32, uintptr, string) int32
	// SoapySDRStrings_clear(char ***elems, size_t length) — pointer to the list pointer.
	stringsClear func(*uintptr, uintptr)
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

func soapyDriverNeedsSerial(driver string) bool {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "hackrf":
		// SoapyHackRF refuses make() without a serial, including HackRF Pro.
		return true
	default:
		return false
	}
}

func deviceCandidates(config Config, discovered []soapyIdentity) []Config {
	var out []Config
	seen := make(map[string]struct{})
	add := func(driver, serial string) {
		if driver == "" {
			return
		}
		if soapyDriverNeedsSerial(driver) && serial == "" {
			return
		}
		key := driver + "\x00" + serial
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		candidate := config
		candidate.Driver, candidate.Serial = driver, serial
		out = append(out, candidate)
	}

	enumerated := make(map[string]struct{}, len(discovered))
	for _, ident := range discovered {
		enumerated[strings.ToLower(ident.Driver)] = struct{}{}
	}

	if len(discovered) > 0 {
		requested := strings.ToLower(strings.TrimSpace(config.Driver))
		if _, ok := enumerated[requested]; ok {
			add(config.Driver, config.Serial)
			if config.Serial != "" {
				add(config.Driver, "")
			}
		}
		for _, ident := range discovered {
			add(ident.Driver, ident.Serial)
			if ident.Serial != "" {
				add(ident.Driver, "")
			}
		}
		return out
	}

	add(config.Driver, config.Serial)
	if config.Serial != "" {
		add(config.Driver, "")
	}
	for _, driver := range soapyProbeDrivers {
		add(driver, "")
	}
	return out
}

func sampleRateAttempts(driver string, requested float64) []float64 {
	extras := []float64{2_048_000, 2_000_000, 2_500_000, 3_000_000, 4_000_000, 8_000_000, 10_000_000}
	switch strings.ToLower(driver) {
	case "airspy":
		extras = []float64{2_500_000, 10_000_000, 6_000_000, 3_000_000, 2_048_000}
	case "airspyhf":
		extras = []float64{768_000, 912_000, 456_000, 192_000, 2_048_000}
	case "hackrf":
		// HackRF One and Pro accept 2–20 MS/s in the legacy radio mode.
		extras = []float64{2_000_000, 2_048_000, 4_000_000, 8_000_000, 10_000_000, 16_000_000, 20_000_000}
	case "lime", "plutosdr", "uhd", "bladerf":
		extras = []float64{2_048_000, 2_000_000, 4_000_000, 5_000_000, 8_000_000, 10_000_000}
	}
	out := []float64{requested}
	seen := map[float64]struct{}{requested: {}}
	for _, rate := range extras {
		if _, ok := seen[rate]; ok || rate <= 0 {
			continue
		}
		seen[rate] = struct{}{}
		out = append(out, rate)
	}
	return out
}

func openSoapy(config Config) (*soapyDevice, error) {
	return openSoapyWith(config, false)
}

func openSoapyExact(config Config) (*soapyDevice, error) {
	return openSoapyWith(config, true)
}

func openSoapyWith(config Config, exact bool) (*soapyDevice, error) {
	config.trace("SoapySDR: loading runtime and modules")
	api, err := sharedSoapy(config)
	if err != nil {
		return nil, err
	}
	soapyOpMu.Lock()
	defer soapyOpMu.Unlock()
	discovered := api.discoverDevices(config)
	var candidates []Config
	if exact {
		if soapyDriverNeedsSerial(config.Driver) && strings.TrimSpace(config.Serial) == "" {
			return nil, fmt.Errorf("driver %s needs a serial", config.Driver)
		}
		if len(discovered) > 0 {
			matched := false
			for _, ident := range discovered {
				if !strings.EqualFold(ident.Driver, config.Driver) {
					continue
				}
				if config.Serial != "" && ident.Serial != config.Serial {
					continue
				}
				config.Serial = ident.Serial
				matched = true
				break
			}
			if !matched {
				return nil, fmt.Errorf("%s %s is not connected", config.Driver, strings.TrimSpace(config.Serial))
			}
		}
		candidates = []Config{config}
	} else {
		candidates = deviceCandidates(config, discovered)
	}
	var failures []error
	for _, candidate := range candidates {
		config.trace("SoapySDR: trying driver=%s serial=%q", candidate.Driver, candidate.Serial)
		device, err := openSoapyCandidate(api, candidate)
		if err == nil {
			config.trace("SoapySDR: driver %s opened at %.0f Hz IQ", candidate.Driver, device.sampleRate)
			return device, nil
		}
		config.trace("SoapySDR: driver %s rejected: %v", candidate.Driver, err)
		failures = append(failures, fmt.Errorf("%s: %w", candidate.Driver, err))
	}
	if len(failures) == 0 {
		return nil, fmt.Errorf("no SDR device found")
	}
	return nil, fmt.Errorf("no SDR device found: %w", errors.Join(failures...))
}

func openSoapyCandidate(api *soapyAPI, config Config) (result *soapyDevice, err error) {
	config.trace("SoapySDR/%s: creating device", config.Driver)
	device := api.makeDevice(config.deviceArguments())
	if device == 0 {
		return nil, fmt.Errorf("SoapySDR make device: %s", api.deviceError())
	}
	result = &soapyDevice{api: api, device: device, driver: config.Driver, serial: config.Serial, buffer: make([]float32, config.FFTSize*2)}
	defer func() {
		if err != nil {
			result.close()
		}
	}()
	result.hardware = api.consume(api.hardwareKey(device))
	var rateErr error
	for _, rate := range sampleRateAttempts(config.Driver, config.SampleRate) {
		config.trace("SoapySDR/%s: configuring sample rate %.0f", config.Driver, rate)
		rateErr = api.check(api.setSampleRate(device, soapyRX, 0, rate), "set sample rate")
		if rateErr == nil {
			break
		}
		config.trace("SoapySDR/%s: sample rate %.0f rejected: %v", config.Driver, rate, rateErr)
	}
	result.sampleRate = api.getSampleRate(device, soapyRX, 0)
	if rateErr != nil && result.sampleRate <= 0 {
		return nil, rateErr
	}
	config.trace("SoapySDR/%s: tuning %d Hz", config.Driver, config.FrequencyHz)
	if err = api.check(api.setFrequency(device, soapyRX, 0, float64(config.FrequencyHz), 0), "set frequency"); err != nil {
		return nil, err
	}
	if soapyAntennaSwitchSupported(config.Driver) {
		config.trace("SoapySDR/%s: reading antennas", config.Driver)
		result.refreshAntennas()
		config.trace("SoapySDR/%s: antennas=%d current=%s", config.Driver, len(result.antennas), result.antenna)
	}
	config.trace("SoapySDR/%s: creating CF32 stream", config.Driver)
	result.stream = api.setupStream(device, soapyRX, "CF32", 0, 0, 0)
	if result.stream == 0 {
		return nil, fmt.Errorf("setup CF32 stream: %s", api.deviceError())
	}
	config.trace("SoapySDR/%s: activating stream", config.Driver)
	if err = api.check(api.activateStream(device, result.stream, 0, 0, 0), "activate stream"); err != nil {
		return nil, err
	}
	config.trace("SoapySDR/%s: stream activate returned", config.Driver)
	if wanted := strings.TrimSpace(config.requestedAntenna()); wanted != "" && soapyAntennaSwitchSupported(config.Driver) {
		if setErr := result.setAntenna(wanted); setErr != nil {
			config.trace("SoapySDR/%s: antenna %q after stream: %v", config.Driver, wanted, setErr)
		}
	}
	config.trace("SoapySDR/%s: stream active · antenna=%s", config.Driver, result.antenna)
	return result, nil
}

func registerSoapy(core uintptr) *soapyAPI {
	api := &soapyAPI{core: core}
	purego.RegisterLibFunc(&api.loadModule, core, "SoapySDR_loadModule")
	purego.RegisterLibFunc(&api.free, core, "SoapySDR_free")
	purego.RegisterLibFunc(&api.enumerate, core, "SoapySDRDevice_enumerateStrArgs")
	purego.RegisterLibFunc(&api.enumerateClear, core, "SoapySDRKwargsList_clear")
	purego.RegisterLibFunc(&api.makeDevice, core, "SoapySDRDevice_makeStrArgs")
	purego.RegisterLibFunc(&api.unmakeDevice, core, "SoapySDRDevice_unmake")
	purego.RegisterLibFunc(&api.hardwareKey, core, "SoapySDRDevice_getHardwareKey")
	purego.RegisterLibFunc(&api.lastError, core, "SoapySDRDevice_lastError")
	purego.RegisterLibFunc(&api.setSampleRate, core, "SoapySDRDevice_setSampleRate")
	purego.RegisterLibFunc(&api.getSampleRate, core, "SoapySDRDevice_getSampleRate")
	purego.RegisterLibFunc(&api.setFrequency, core, "SoapySDRDevice_setFrequency")
	purego.RegisterLibFunc(&api.getFrequency, core, "SoapySDRDevice_getFrequency")
	purego.RegisterLibFunc(&api.getFrequencyCorrection, core, "SoapySDRDevice_getFrequencyCorrection")
	purego.RegisterLibFunc(&api.setFrequencyCorrection, core, "SoapySDRDevice_setFrequencyCorrection")
	purego.RegisterLibFunc(&api.getGainMode, core, "SoapySDRDevice_getGainMode")
	purego.RegisterLibFunc(&api.setGainMode, core, "SoapySDRDevice_setGainMode")
	purego.RegisterLibFunc(&api.getGain, core, "SoapySDRDevice_getGain")
	purego.RegisterLibFunc(&api.setGain, core, "SoapySDRDevice_setGain")
	purego.RegisterLibFunc(&api.getGainElement, core, "SoapySDRDevice_getGainElement")
	purego.RegisterLibFunc(&api.setGainElement, core, "SoapySDRDevice_setGainElement")
	purego.RegisterLibFunc(&api.readSetting, core, "SoapySDRDevice_readSetting")
	purego.RegisterLibFunc(&api.writeSetting, core, "SoapySDRDevice_writeSetting")
	purego.RegisterLibFunc(&api.setupStream, core, "SoapySDRDevice_setupStream")
	purego.RegisterLibFunc(&api.closeStream, core, "SoapySDRDevice_closeStream")
	purego.RegisterLibFunc(&api.activateStream, core, "SoapySDRDevice_activateStream")
	purego.RegisterLibFunc(&api.deactivateStream, core, "SoapySDRDevice_deactivateStream")
	purego.RegisterLibFunc(&api.readStream, core, "SoapySDRDevice_readStream")
	purego.RegisterLibFunc(&api.errToString, core, "SoapySDR_errToStr")
	purego.RegisterLibFunc(&api.listAntennas, core, "SoapySDRDevice_listAntennas")
	purego.RegisterLibFunc(&api.getAntenna, core, "SoapySDRDevice_getAntenna")
	purego.RegisterLibFunc(&api.setAntenna, core, "SoapySDRDevice_setAntenna")
	purego.RegisterLibFunc(&api.stringsClear, core, "SoapySDRStrings_clear")
	return api
}

func (api *soapyAPI) discoverDevices(config Config) []soapyIdentity {
	var length uintptr
	list := api.enumerate("", &length)
	if list == 0 || length == 0 {
		config.trace("SoapySDR: enumerate found no devices")
		return nil
	}
	defer api.enumerateClear(list, length)
	out := make([]soapyIdentity, 0, int(length))
	stride := unsafe.Sizeof(soapyKwargs{})
	pointerSize := unsafe.Sizeof(uintptr(0))
	for index := uintptr(0); index < length; index++ {
		kwargs := (*soapyKwargs)(unsafe.Pointer(list + index*stride))
		ident := soapyIdentity{}
		for pair := uintptr(0); pair < kwargs.size; pair++ {
			keyPtr := *(*uintptr)(unsafe.Pointer(kwargs.keys + pair*pointerSize))
			valPtr := *(*uintptr)(unsafe.Pointer(kwargs.vals + pair*pointerSize))
			switch cString(keyPtr) {
			case "driver":
				ident.Driver = cString(valPtr)
			case "serial":
				ident.Serial = cString(valPtr)
			case "label":
				ident.Label = cString(valPtr)
			}
		}
		if ident.Driver == "" {
			continue
		}
		config.trace("SoapySDR: found driver=%s serial=%q label=%q", ident.Driver, ident.Serial, ident.Label)
		out = append(out, ident)
	}
	return out
}

func (api *soapyAPI) loadModules(config Config, paths []string) {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		config.trace("SoapySDR: loading module %s", path)
		message := api.consume(api.loadModule(path))
		if message != "" {
			config.trace("SoapySDR: module %s skipped: %s", path, message)
			continue
		}
		config.trace("SoapySDR: module loaded (%s)", path)
	}
}

func collectSoapyModules(dirs ...string) []string {
	var out []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext != ".dll" && ext != ".so" && ext != ".dylib" {
				continue
			}
			out = append(out, filepath.Join(dir, name))
		}
	}
	return uniqueStrings(out)
}

func preloadLibraries(config Config, paths []string) []uintptr {
	var handles []uintptr
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		handle, err := loadShared(path)
		if err != nil {
			config.trace("SoapySDR: skip %s: %v", path, err)
			continue
		}
		config.trace("SoapySDR: preloaded %s", path)
		handles = append(handles, handle)
	}
	return handles
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, value := range in {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (device *soapyDevice) read(destination []float32) (int, int32, error) {
	if device == nil || device.api == nil || device.device == 0 || device.stream == 0 {
		return 0, 0, fmt.Errorf("device closed")
	}
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
	if device == nil || device.api == nil || device.device == 0 {
		return fmt.Errorf("device closed")
	}
	err := device.api.check(
		device.api.setFrequency(device.device, soapyRX, 0, float64(frequencyHz), 0),
		"set center frequency",
	)
	if device.antenna != "" {
		_ = device.setAntenna(device.antenna)
	}
	return err
}

func (device *soapyDevice) centerFrequency() int64 {
	return int64(math.Round(device.api.getFrequency(device.device, soapyRX, 0)))
}

func (device *soapyDevice) hardwareSettings() HardwareSettings {
	antenna, antennas := device.antennaState()
	switch ProfileFor(device.driver) {
	case ProfileRTLSDR:
		return HardwareSettings{
			Available: true, Device: device.hardware, Driver: device.driver, Serial: device.serial,
			Antenna: antenna, Antennas: antennas,
			AGC:            device.api.getGainMode(device.device, soapyRX, 0),
			RFGain:         float32(device.api.getGainElement(device.device, soapyRX, 0, "TUNER")),
			PPM:            float32(device.api.getFrequencyCorrection(device.device, soapyRX, 0)),
			BiasT:          device.readBoolSetting("biastee"),
			DigitalAGC:     device.readBoolSetting("digital_agc"),
			OffsetTuning:   device.readBoolSetting("offset_tune"),
			IQSwap:         device.readBoolSetting("iq_swap"),
			DirectSampling: device.readIntSetting("direct_samp", 0),
		}
	case ProfileSDRplay:
		return HardwareSettings{
			Available: true, Device: device.hardware, Driver: device.driver, Serial: device.serial,
			Antenna: antenna, Antennas: antennas,
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
	default:
		return HardwareSettings{
			Available: true, Device: device.hardware, Driver: device.driver, Serial: device.serial,
			Antenna: antenna, Antennas: antennas,
			AGC:    device.api.getGainMode(device.device, soapyRX, 0),
			RFGain: float32(device.api.getGain(device.device, soapyRX, 0)),
			PPM:    float32(device.api.getFrequencyCorrection(device.device, soapyRX, 0)),
			BiasT:  device.readBoolSettingAny(soapyBiasSettings...),
		}
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
	switch ProfileFor(device.driver) {
	case ProfileRTLSDR:
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
	case ProfileSDRplay:
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
	default:
		if current.AGC != settings.AGC {
			apply(device.api.setGainMode(device.device, soapyRX, 0, settings.AGC), "AGC")
		}
		if !settings.AGC && current.RFGain != settings.RFGain {
			apply(device.api.setGain(device.device, soapyRX, 0, float64(settings.RFGain)), "gain")
		}
		if current.PPM != settings.PPM {
			apply(device.api.setFrequencyCorrection(device.device, soapyRX, 0, float64(settings.PPM)), "frequency correction")
		}
		if current.BiasT != settings.BiasT {
			device.writeSettingAny(soapyBiasSettings, boolString(settings.BiasT))
		}
	}
	if err := device.applyAntenna(settings.Antenna); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

func (device *soapyDevice) readBoolSetting(key string) bool {
	return device.readBoolSettingAny(key)
}

func (device *soapyDevice) readBoolSettingAny(keys ...string) bool {
	for _, key := range keys {
		value := strings.ToLower(strings.TrimSpace(device.api.consume(device.api.readSetting(device.device, key))))
		switch value {
		case "true", "1", "on":
			return true
		case "false", "0", "off":
			return false
		}
	}
	return false
}

func (device *soapyDevice) writeSettingAny(keys []string, value string) bool {
	for _, key := range keys {
		if device.api.writeSetting(device.device, key, value) == 0 {
			return true
		}
	}
	return false
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

func (device *soapyDevice) stopStream() {
	if device == nil || device.api == nil {
		return
	}
	if device.device != 0 && device.stream != 0 {
		device.api.deactivateStream(device.device, device.stream, 0, 0)
		device.api.closeStream(device.device, device.stream)
		device.stream = 0
	}
}

func (device *soapyDevice) close() {
	if device == nil || device.api == nil {
		return
	}
	device.stopStream()
	if device.device != 0 {
		device.api.unmakeDevice(device.device)
		device.device = 0
	}
	// Vendor APIs can still finish internal callbacks after unmake. Keep the
	// process-wide library handles loaded; the OS releases them on exit.
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
		closeShared(api.module)
		api.module = 0
	}
	if api.core != 0 {
		closeShared(api.core)
		api.core = 0
	}
	if api.vendor != 0 {
		closeShared(api.vendor)
		api.vendor = 0
	}
	for index := len(api.dependencies) - 1; index >= 0; index-- {
		closeShared(api.dependencies[index])
	}
	api.dependencies = nil
}

func cString(pointer uintptr) string {
	if pointer == 0 {
		return ""
	}
	const maxCString = 4096
	bytes := make([]byte, 0, 128)
	for offset := uintptr(0); offset < maxCString; offset++ {
		value := *(*byte)(unsafe.Pointer(pointer + offset))
		if value == 0 {
			return string(bytes)
		}
		bytes = append(bytes, value)
	}
	return string(bytes)
}
