package sdr

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go-zero/internal/aircraft"
	"go-zero/internal/ais"
	"go-zero/internal/aprs"
	"go-zero/internal/dmr"
	"go-zero/internal/dsp"
	"go-zero/internal/radiosonde"
	"go-zero/internal/rtl433"
	"go-zero/internal/sstv"
	"go-zero/internal/tetra"
)

const demodulatedAudioSampleRate = 48_000

type Config struct {
	RadiosondeDirectory       string
	AISExecutable             string
	Aircraft1090Executable    string
	Aircraft978Executable     string
	AircraftUATTextExecutable string
	RuntimeRoot               string
	Driver                    string
	Serial                    string
	FrequencyHz               int64
	SampleRate                float64
	FFTSize                   int
	CalibrationDB             float32
	// InitialHardware is applied before the receiver starts publishing samples.
	// It lets the application restore the original IC-SDR hardware profile
	// instead of inheriting an arbitrary AGC state from the driver.
	InitialHardware                                  *HardwareSettings
	DMRExecutable                                    string
	RTL433Executable                                 string
	APRSExecutable, APRSConfig, APRSWorkingDirectory string
	SSTVExecutable, SSTVOutputDirectory              string
	TETRACodec                                       string
	StartupLog                                       func(string, ...any)
}

func (config Config) trace(format string, args ...any) {
	if config.StartupLog != nil {
		config.StartupLog(format, args...)
	}
}

type Stats struct {
	Device, Status                                string
	SampleRate, RMS, Peak, FFTPerSecond           float64
	ReceivedSamples, FFTBlocks, Timeouts          uint64
	Overflows, InvalidSamples                     uint64
	SpectrumCenterHz                              int64
	AudioBuffered, AudioOverruns, AudioUnderflows uint64
	AudioProduced, AudioConsumed                  uint64
	AudioPeak, SignalDBm                          float32
	SquelchOpen                                   bool
}

type HardwareSettings struct {
	Available, AGC, BiasT, RFNotch, DABNotch, IQCorrection bool
	DigitalAGC, OffsetTuning, IQSwap                       bool
	Device, Driver                                         string
	RFGain, IFGain, PPM                                    float32
	AGCSetpoint, DirectSampling                            int
}

type Receiver struct {
	radiosonde *radiosonde.Decoder
	ais        *ais.Decoder
	aircraft   *aircraft.Decoder
	config     Config
	device     *soapyDevice
	fft        *dsp.FFT
	am         *dsp.AMDemodulator
	nfm        *dsp.NFMDemodulator
	wfm        *dsp.NFMDemodulator
	ssb        *dsp.SSBDemodulator
	dmr        *dmr.Decoder
	rtl433     *rtl433.Decoder
	aprs       *aprs.Decoder
	sstv       *sstv.Decoder
	tetra      *tetra.Decoder
	subtone    *dsp.SubtoneDetector

	stop         chan struct{}
	done         chan struct{}
	tuneDone     chan struct{}
	running      atomic.Bool
	averagingMs  atomic.Int64
	deemphasisUs atomic.Int64
	closeOnce    sync.Once

	mu                                 sync.RWMutex
	spectrum                           []float32
	stats                              Stats
	tune                               chan int64
	settings                           chan HardwareSettings
	centerHz                           atomic.Int64
	spectrumGeneration                 atomic.Uint64
	hardware                           HardwareSettings
	demodMode                          string
	tunedHz                            int64
	demodBandwidthHz                   int
	pbtLowHz, pbtHighHz                int
	pbtBypassed                        bool
	squelchEnabled                     bool
	squelchThresholdDBm                float32
	squelchHoldMs, squelchCloseMs      int
	squelchLevelOpen, squelchClosing   bool
	squelchGain, squelchCloseStartGain float32
	squelchHoldRemaining               int
	squelchCloseRemaining              int
	audio                              []float32
	audioRead, audioWrite, audioCount  int
}

func NewReceiver(config Config) *Receiver {
	if config.Driver == "" {
		config.Driver = "sdrplay"
	}
	if config.SampleRate <= 0 {
		config.SampleRate = 2_048_000
	}
	if config.FFTSize <= 0 {
		config.FFTSize = 4096
	}
	if config.CalibrationDB == 0 {
		config.CalibrationDB = 23
	}
	receiver := &Receiver{
		config: config,
		fft:    dsp.NewFFT(config.FFTSize),
		am:     dsp.NewAMDemodulator(config.SampleRate, 48_000),
		nfm:    dsp.NewNFMDemodulator(config.SampleRate, 48_000),
		wfm:    dsp.NewNFMDemodulator(config.SampleRate, 48_000),
		ssb:    dsp.NewSSBDemodulator(config.SampleRate, 48_000),
		stop:   make(chan struct{}), done: make(chan struct{}), tuneDone: make(chan struct{}),
		tune:                make(chan int64, 1),
		settings:            make(chan HardwareSettings, 1),
		spectrum:            make([]float32, config.FFTSize),
		stats:               Stats{Status: "SDR desconectado"},
		tunedHz:             config.FrequencyHz,
		demodBandwidthHz:    9_000,
		pbtLowHz:            100,
		pbtHighHz:           3_250,
		squelchThresholdDBm: -100,
		squelchHoldMs:       80,
		squelchCloseMs:      125,
		squelchLevelOpen:    true,
		squelchGain:         1,
		audio:               make([]float32, 48_000),
	}
	receiver.centerHz.Store(config.FrequencyHz)
	receiver.dmr = dmr.New(config.SampleRate, config.DMRExecutable, receiver.enqueueDigitalAudio)
	receiver.rtl433 = rtl433.New(config.SampleRate, config.RTL433Executable)
	receiver.radiosonde = radiosonde.New(config.SampleRate, config.RadiosondeDirectory)
	receiver.ais = ais.New(config.SampleRate, config.AISExecutable)
	receiver.aircraft = aircraft.New(config.SampleRate, config.Aircraft1090Executable, config.Aircraft978Executable, config.AircraftUATTextExecutable)
	receiver.aprs = aprs.New(config.SampleRate, config.APRSExecutable, config.APRSConfig, config.APRSWorkingDirectory)
	receiver.sstv = sstv.New(config.SSTVExecutable, config.SSTVOutputDirectory)
	receiver.tetra = tetra.New(config.SampleRate, config.TETRACodec, receiver.enqueueDigitalAudio)
	receiver.subtone = dsp.NewSubtoneDetector()
	receiver.averagingMs.Store(120)
	receiver.deemphasisUs.Store(50)
	for index := range receiver.spectrum {
		receiver.spectrum[index] = -120
	}
	return receiver
}

func (receiver *Receiver) Start() error {
	receiver.trace("SDR: searching for device candidates")
	device, err := openSoapy(receiver.config)
	if err != nil {
		receiver.setError(err)
		return err
	}
	receiver.device = device
	receiver.trace("SDR: device opened · hardware=%s · driver=%s", device.hardware, device.driver)
	receiver.trace("SDR: reading physical controls")
	hardware := device.hardwareSettings()
	if receiver.config.InitialHardware != nil {
		receiver.trace("SDR: aplicando ajustes iniciales")
		initial := *receiver.config.InitialHardware
		initial.Available = true
		initial.Device = hardware.Device
		initial.Driver = hardware.Driver
		if err := device.applyHardwareSettings(initial); err != nil {
			device.close()
			receiver.device = nil
			receiver.setError(err)
			return err
		}
		hardware = device.hardwareSettings()
	}
	receiver.trace("SDR: ajustes confirmados · sampleRate=%.0f", device.sampleRate)
	receiver.mu.Lock()
	receiver.stats.Device = device.hardware
	receiver.stats.SampleRate = device.sampleRate
	receiver.stats.SpectrumCenterHz = receiver.config.FrequencyHz
	receiver.stats.Status = "IQ esperando primeras muestras"
	receiver.hardware = hardware
	receiver.mu.Unlock()
	receiver.running.Store(true)
	go receiver.run()
	go receiver.runTuner()
	receiver.trace("SDR: capture and tune threads started")
	return nil
}

func (receiver *Receiver) trace(format string, args ...any) {
	if receiver.config.StartupLog != nil {
		receiver.config.StartupLog(format, args...)
	}
}

func (receiver *Receiver) Close() {
	receiver.closeOnce.Do(func() {
		if receiver.dmr != nil {
			receiver.dmr.Stop()
		}
		if receiver.sstv != nil {
			receiver.sstv.Close()
		}
		if receiver.rtl433 != nil {
			receiver.rtl433.Stop()
		}
		if receiver.radiosonde != nil {
			receiver.radiosonde.Close()
		}
		if receiver.ais != nil {
			receiver.ais.Close()
		}
		if receiver.aircraft != nil {
			receiver.aircraft.Close()
		}
		if receiver.aprs != nil {
			receiver.aprs.Stop()
		}
		if receiver.tetra != nil {
			receiver.tetra.Close()
		}
		if receiver.running.Swap(false) {
			close(receiver.stop)
			<-receiver.done
			<-receiver.tuneDone
			receiver.device.close()
		} else if receiver.device != nil {
			receiver.device.close()
		}
	})
}

func (receiver *Receiver) Snapshot(destination []float32) Stats {
	receiver.mu.RLock()
	copy(destination, receiver.spectrum)
	stats := receiver.stats
	receiver.mu.RUnlock()
	return stats
}

// SetCenterFrequency queues a retune without blocking the UI thread.
func (receiver *Receiver) SetCenterFrequency(frequencyHz int64) {
	frequencyHz = max(frequencyHz, 1_000)
	if receiver.centerHz.Swap(frequencyHz) == frequencyHz {
		return
	}
	select {
	case receiver.tune <- frequencyHz:
	default:
		select {
		case <-receiver.tune:
		default:
		}
		select {
		case receiver.tune <- frequencyHz:
		default:
		}
	}
}

func (receiver *Receiver) CenterFrequency() int64 { return receiver.centerHz.Load() }

func (receiver *Receiver) SetFFTWindow(windowType string) { receiver.fft.SetWindowType(windowType) }
func (receiver *Receiver) SetSpectrumAveraging(milliseconds int) {
	receiver.averagingMs.Store(int64(min(max(milliseconds, 10), 300)))
}
func (receiver *Receiver) SetFMDeemphasis(microseconds int) {
	if microseconds != 75 {
		microseconds = 50
	}
	receiver.deemphasisUs.Store(int64(microseconds))
}

func (receiver *Receiver) SetSubtoneMode(mode string) {
	if receiver.subtone != nil {
		receiver.subtone.SetMode(mode)
	}
}
func (receiver *Receiver) SubtoneStatus() dsp.SubtoneStatus {
	if receiver.subtone == nil {
		return dsp.SubtoneStatus{Mode: "OFF"}
	}
	return receiver.subtone.Snapshot()
}
func (receiver *Receiver) SetDemodulator(mode string, tunedHz int64, bandwidthHz int) {
	receiver.mu.Lock()
	receiver.demodMode = mode
	receiver.tunedHz = tunedHz
	receiver.demodBandwidthHz = max(bandwidthHz, 1_000)
	if mode != "AM" && mode != "NFM" && mode != "WFM" && mode != "USB" && mode != "LSB" {
		receiver.audioRead, receiver.audioWrite, receiver.audioCount = 0, 0, 0
	}
	receiver.mu.Unlock()
	if receiver.dmr != nil {
		receiver.dmr.Configure(mode == "DMR BETA", float64(tunedHz-receiver.centerHz.Load()), bandwidthHz)
	}
	if receiver.tetra != nil {
		receiver.tetra.SetTuningOffset(float64(tunedHz - receiver.centerHz.Load()))
	}
}

func (receiver *Receiver) DMRStatus() dmr.Status {
	if receiver.dmr == nil {
		return dmr.Status{State: "OFF", ColorCode: -1, Slot1: "--", Slot2: "--"}
	}
	return receiver.dmr.Snapshot()
}
func (receiver *Receiver) SetDMRAutoCenter(enabled bool) {
	if receiver.dmr != nil {
		receiver.dmr.SetAutoCenter(enabled)
	}
}
func (receiver *Receiver) SetDMRAudioSlot(slot string) {
	if receiver.dmr != nil {
		receiver.dmr.SetAudioSlot(slot)
	}
}
func (receiver *Receiver) ResyncDMR() bool { return receiver.dmr != nil && receiver.dmr.Resync() }

func (receiver *Receiver) ConfigureSSTV(enabled bool) {
	if receiver.sstv != nil {
		receiver.sstv.Configure(enabled)
	}
}
func (receiver *Receiver) SSTVStatus() sstv.Status {
	if receiver.sstv == nil {
		return sstv.Status{State: "UNAVAILABLE"}
	}
	return receiver.sstv.Snapshot()
}
func (receiver *Receiver) SSTVFrame(channel, sequence int) (sstv.Frame, bool) {
	if receiver.sstv == nil {
		return sstv.Frame{}, false
	}
	return receiver.sstv.Frame(channel, sequence)
}
func (receiver *Receiver) SetSSTVAutomatic(value bool) {
	if receiver.sstv != nil {
		receiver.sstv.SetAutomatic(value)
	}
}
func (receiver *Receiver) SetSSTVMode(value string) {
	if receiver.sstv != nil {
		receiver.sstv.SetSelectedMode(value)
	}
}
func (receiver *Receiver) SetSSTVCandidateMode(index int, value string) {
	if receiver.sstv != nil {
		receiver.sstv.SetCandidateMode(index, value)
	}
}
func (receiver *Receiver) ForceSSTV() bool {
	return receiver.sstv != nil && receiver.sstv.Restart(true)
}
func (receiver *Receiver) StopSSTVReceive() {
	if receiver.sstv != nil {
		receiver.sstv.StopReceive()
	}
}
func (receiver *Receiver) RestartSSTV() bool {
	return receiver.sstv != nil && receiver.sstv.Restart(false)
}
func (receiver *Receiver) SaveSSTVPartial() (string, error) {
	if receiver.sstv == nil {
		return "", fmt.Errorf("SSTV no disponible")
	}
	return receiver.sstv.SavePartial()
}
func (receiver *Receiver) SSTVOutputFolder() string {
	if receiver.sstv == nil {
		return ""
	}
	return receiver.sstv.OutputFolder()
}

func (receiver *Receiver) ConfigureRTL433(enabled bool, frequencyHz int64, bandwidthHz ...int) {
	if receiver.rtl433 != nil {
		width := 250_000
		if len(bandwidthHz) > 0 {
			width = bandwidthHz[0]
		}
		receiver.rtl433.Configure(enabled, frequencyHz, receiver.centerHz.Load(), width)
	}
}
func (receiver *Receiver) RTL433Status() rtl433.Status {
	if receiver.rtl433 == nil {
		return rtl433.Status{State: "UNAVAILABLE"}
	}
	return receiver.rtl433.Snapshot()
}
func (receiver *Receiver) RTL433Events() []rtl433.Event {
	if receiver.rtl433 == nil {
		return nil
	}
	return receiver.rtl433.Events()
}
func (receiver *Receiver) ClearRTL433Events() {
	if receiver.rtl433 != nil {
		receiver.rtl433.Clear()
	}
}

func (receiver *Receiver) ConfigureAPRS(enabled bool, frequencyHz int64, bandwidthHz int) {
	if receiver.aprs != nil {
		receiver.aprs.Configure(enabled, frequencyHz, receiver.centerHz.Load(), bandwidthHz)
	}
}
func (receiver *Receiver) APRSStatus() aprs.Status {
	if receiver.aprs == nil {
		return aprs.Status{State: "UNAVAILABLE", AudioLevel: -1}
	}
	return receiver.aprs.Snapshot()
}
func (receiver *Receiver) APRSPackets() []aprs.Packet {
	if receiver.aprs == nil {
		return nil
	}
	return receiver.aprs.Packets()
}
func (receiver *Receiver) ClearAPRSPackets() {
	if receiver.aprs != nil {
		receiver.aprs.Clear()
	}
}

func (receiver *Receiver) SetTwinPBT(lowHz, highHz int, bypassed bool) {
	receiver.mu.Lock()
	receiver.pbtLowHz = min(max(lowHz, 50), 4800)
	receiver.pbtHighHz = min(max(highHz, receiver.pbtLowHz+200), 5000)
	receiver.pbtBypassed = bypassed
	receiver.mu.Unlock()
}

// SetSquelch configures the common RF-level audio gate.
func (receiver *Receiver) SetSquelch(enabled bool, thresholdDBm float32, holdMs, closeMs int) {
	receiver.mu.Lock()
	receiver.squelchEnabled = enabled
	receiver.squelchThresholdDBm = min(max(thresholdDBm, -140), 0)
	receiver.squelchHoldMs = min(max(holdMs, 0), 300)
	receiver.squelchCloseMs = min(max(closeMs, 20), 500)
	if !enabled {
		receiver.squelchLevelOpen = true
		receiver.squelchClosing = false
		receiver.squelchGain = 1
	}
	receiver.mu.Unlock()
}

func (receiver *Receiver) ReadAudio(destination []float32) int {
	receiver.mu.Lock()
	count := min(len(destination), receiver.audioCount)
	for index := 0; index < count; index++ {
		destination[index] = receiver.audio[receiver.audioRead]
		receiver.audioRead = (receiver.audioRead + 1) % len(receiver.audio)
	}
	receiver.audioCount -= count
	receiver.stats.AudioConsumed += uint64(count)
	if count < len(destination) && receiver.running.Load() && receiver.demodMode != "" {
		receiver.stats.AudioUnderflows++
	}
	receiver.stats.AudioBuffered = uint64(receiver.audioCount)
	receiver.mu.Unlock()
	return count
}

func (receiver *Receiver) AudioBufferedSamples() int {
	receiver.mu.RLock()
	count := receiver.audioCount
	receiver.mu.RUnlock()
	return count
}

// AudioPlaybackState lets the output stage apply the burst-oriented buffering
// required by DMR without coupling analog demodulators to the digital decoder.
func (receiver *Receiver) AudioPlaybackState() (mode string, digitalSignalActive bool) {
	receiver.mu.RLock()
	mode = receiver.demodMode
	receiver.mu.RUnlock()
	if mode == "DMR BETA" && receiver.dmr != nil {
		digitalSignalActive = receiver.dmr.Snapshot().SignalActive
	} else if mode == "TETRA" && receiver.tetra != nil {
		status := receiver.tetra.Snapshot()
		digitalSignalActive = !status.LastAudio.IsZero() && time.Since(status.LastAudio) < time.Second
	}
	return
}

func (receiver *Receiver) HardwareSettings() HardwareSettings {
	receiver.mu.RLock()
	settings := receiver.hardware
	receiver.mu.RUnlock()
	return settings
}

func (receiver *Receiver) ApplyHardwareSettings(settings HardwareSettings) {
	receiver.mu.Lock()
	receiver.hardware = settings
	receiver.mu.Unlock()
	select {
	case receiver.settings <- settings:
	default:
		select {
		case <-receiver.settings:
		default:
		}
		select {
		case receiver.settings <- settings:
		default:
		}
	}
}

func (receiver *Receiver) run() {
	defer close(receiver.done)

	readBuffer := make([]float32, receiver.config.FFTSize*2)
	fftBuffer := make([]float32, receiver.config.FFTSize*2)
	workingSpectrum := make([]float32, receiver.config.FFTSize)
	smoothedSpectrum := make([]float32, receiver.config.FFTSize)
	firstFFT := true
	observedSpectrumGeneration := receiver.spectrumGeneration.Load()
	blockSeconds := float64(receiver.config.FFTSize) / receiver.config.SampleRate
	fill := 0
	windowStarted := time.Now()
	windowFFTs := uint64(0)

	for {
		select {
		case <-receiver.stop:
			return
		default:
		}
		read, code, err := receiver.device.read(readBuffer)
		if err != nil {
			receiver.setError(err)
			return
		}
		if code == soapyTimeout {
			receiver.addEvent(true, false)
			continue
		}
		if code == soapyOverflow {
			receiver.addEvent(false, true)
			continue
		}
		if read <= 0 {
			continue
		}
		if generation := receiver.spectrumGeneration.Load(); generation != observedSpectrumGeneration {
			// Discard a read that may straddle the hardware retune. Reusing it (or
			// the previous exponential average) moves old peaks to false frequencies.
			observedSpectrumGeneration = generation
			fill = 0
			firstFFT = true
			continue
		}
		receiver.processAudio(readBuffer[:read*2])
		if receiver.rtl433 != nil {
			receiver.rtl433.ProcessIQ(readBuffer[:read*2])
		}
		if receiver.aprs != nil {
			receiver.aprs.ProcessIQ(readBuffer[:read*2])
		}
		if receiver.radiosonde != nil {
			receiver.radiosonde.ProcessIQ(readBuffer[:read*2])
		}
		if receiver.ais != nil {
			receiver.ais.ProcessIQ(readBuffer[:read*2])
		}
		if receiver.aircraft != nil {
			receiver.aircraft.ProcessIQ(readBuffer[:read*2])
		}
		if receiver.tetra != nil {
			receiver.tetra.ProcessIQ(readBuffer[:read*2])
		}
		rms, peak, invalid := iqStats(readBuffer[:read*2])
		receiver.mu.Lock()
		receiver.stats.ReceivedSamples += uint64(read)
		receiver.stats.RMS = rms
		receiver.stats.Peak = peak
		receiver.stats.InvalidSamples += invalid
		receiver.stats.Status = "IQ valid"
		receiver.mu.Unlock()

		source := 0
		for source < read {
			count := min(read-source, receiver.config.FFTSize-fill)
			copy(fftBuffer[fill*2:(fill+count)*2], readBuffer[source*2:(source+count)*2])
			fill += count
			source += count
			if fill == receiver.config.FFTSize {
				receiver.fft.Process(fftBuffer, workingSpectrum)
				averagingSeconds := float64(receiver.averagingMs.Load()) / 1000
				smoothingAlpha := float32(1 - math.Exp(-blockSeconds/averagingSeconds))
				for index := range workingSpectrum {
					workingSpectrum[index] += receiver.config.CalibrationDB
				}
				for index := range workingSpectrum {
					if firstFFT {
						smoothedSpectrum[index] = workingSpectrum[index]
					} else {
						smoothedSpectrum[index] += smoothingAlpha * (workingSpectrum[index] - smoothedSpectrum[index])
					}
				}
				firstFFT = false
				receiver.mu.Lock()
				copy(receiver.spectrum, smoothedSpectrum)
				receiver.stats.FFTBlocks++
				receiver.mu.Unlock()
				windowFFTs++
				fill = 0
			}
		}

		elapsed := time.Since(windowStarted)
		if elapsed >= time.Second {
			receiver.mu.Lock()
			receiver.stats.FFTPerSecond = float64(windowFFTs) / elapsed.Seconds()
			receiver.mu.Unlock()
			windowStarted, windowFFTs = time.Now(), 0
		}
	}
}

func (receiver *Receiver) processAudio(iq []float32) {
	receiver.mu.RLock()
	mode, tunedHz, bandwidthHz := receiver.demodMode, receiver.tunedHz, receiver.demodBandwidthHz
	pbtLowHz, pbtHighHz, pbtBypassed := receiver.pbtLowHz, receiver.pbtHighHz, receiver.pbtBypassed
	centerHz := receiver.centerHz.Load()
	signalDBm := receiver.signalLevelLocked(mode, tunedHz, centerHz, bandwidthHz)
	receiver.mu.RUnlock()
	// This is an RF measurement shared by analog and digital demodulators.
	// Publish it before DMR branches away from the analog squelch/audio path.
	receiver.mu.Lock()
	receiver.stats.SignalDBm = signalDBm
	receiver.mu.Unlock()
	if mode == "DMR BETA" {
		if receiver.dmr != nil {
			receiver.dmr.ProcessIQ(iq)
			status := receiver.dmr.Snapshot()
			receiver.mu.Lock()
			receiver.stats.SquelchOpen = status.SignalActive
			receiver.mu.Unlock()
		}
		return
	}
	var samples []float32
	switch mode {
	case "AM":
		samples = receiver.am.Process(iq, float64(tunedHz-centerHz), bandwidthHz)
	case "NFM":
		samples = receiver.nfm.Process(iq, float64(tunedHz-centerHz), bandwidthHz, int(receiver.deemphasisUs.Load()))
		if receiver.subtone != nil {
			receiver.subtone.Process(receiver.nfm.Subaudible())
		}
	case "WFM":
		samples = receiver.wfm.ProcessWide(iq, float64(tunedHz-centerHz), bandwidthHz, int(receiver.deemphasisUs.Load()))
	case "USB", "LSB":
		if pbtBypassed {
			pbtLowHz, pbtHighHz = 100, bandwidthHz
		}
		samples = receiver.ssb.ProcessPBT(iq, mode, float64(tunedHz-centerHz), pbtLowHz, pbtHighHz)
	default:
		return
	}
	// SSTV needs the untouched demodulated stream. Speaker mute, volume,
	// listening filters and RF squelch must never remove its timing tones.
	if receiver.sstv != nil {
		receiver.sstv.ProcessAudio(samples)
	}
	receiver.mu.Lock()
	receiver.applySquelchLocked(samples, signalDBm)
	receiver.stats.AudioProduced += uint64(len(samples))
	for _, sample := range samples {
		if receiver.audioCount == len(receiver.audio) {
			receiver.audioRead = (receiver.audioRead + 1) % len(receiver.audio)
			receiver.audioCount--
			receiver.stats.AudioOverruns++
		}
		receiver.audio[receiver.audioWrite] = sample
		receiver.audioWrite = (receiver.audioWrite + 1) % len(receiver.audio)
		receiver.audioCount++
		if value := float32(math.Abs(float64(sample))); value > receiver.stats.AudioPeak {
			receiver.stats.AudioPeak = value
		}
	}
	receiver.stats.AudioBuffered = uint64(receiver.audioCount)
	receiver.mu.Unlock()
}

func (receiver *Receiver) enqueueDigitalAudio(samples []float32) {
	receiver.mu.Lock()
	receiver.stats.AudioProduced += uint64(len(samples))
	for _, sample := range samples {
		if receiver.audioCount == len(receiver.audio) {
			receiver.audioRead = (receiver.audioRead + 1) % len(receiver.audio)
			receiver.audioCount--
			receiver.stats.AudioOverruns++
		}
		receiver.audio[receiver.audioWrite] = sample
		receiver.audioWrite = (receiver.audioWrite + 1) % len(receiver.audio)
		receiver.audioCount++
		if value := float32(math.Abs(float64(sample))); value > receiver.stats.AudioPeak {
			receiver.stats.AudioPeak = value
		}
	}
	receiver.stats.AudioBuffered = uint64(receiver.audioCount)
	receiver.stats.SquelchOpen = len(samples) > 0
	receiver.mu.Unlock()
}

func (receiver *Receiver) signalLevelLocked(mode string, tunedHz, centerHz int64, bandwidthHz int) float32 {
	if receiver.stats.FFTBlocks == 0 || len(receiver.spectrum) == 0 || receiver.config.SampleRate <= 0 {
		return -140
	}
	centerBin := (.5 + float64(tunedHz-centerHz)/receiver.config.SampleRate) * float64(len(receiver.spectrum))
	bandBins := max(float64(bandwidthHz)/receiver.config.SampleRate*float64(len(receiver.spectrum)), 1)
	low, high := centerBin-bandBins*.5, centerBin+bandBins*.5
	if mode == "USB" {
		low, high = centerBin, centerBin+bandBins
	} else if mode == "LSB" {
		low, high = centerBin-bandBins, centerBin
	}
	first := max(int(math.Floor(low)), 0)
	last := min(int(math.Ceil(high)), len(receiver.spectrum)-1)
	if first > last {
		return -140
	}
	level := float32(-140)
	for index := first; index <= last; index++ {
		level = max(level, receiver.spectrum[index])
	}
	return level
}

func (receiver *Receiver) applySquelchLocked(samples []float32, signalDBm float32) {
	receiver.stats.SignalDBm = signalDBm
	if !receiver.squelchEnabled {
		receiver.squelchLevelOpen = true
		receiver.squelchGain = 1
		receiver.stats.SquelchOpen = true
		return
	}
	if receiver.squelchLevelOpen {
		if signalDBm < receiver.squelchThresholdDBm-3 {
			receiver.squelchLevelOpen = false
		}
	} else if signalDBm >= receiver.squelchThresholdDBm {
		receiver.squelchLevelOpen = true
	}
	if receiver.squelchLevelOpen {
		receiver.squelchHoldRemaining = demodulatedAudioSampleRate * receiver.squelchHoldMs / 1000
		receiver.squelchClosing = false
		receiver.squelchCloseRemaining = 0
	}
	attack := float32(1 - math.Exp(-1/(.008*demodulatedAudioSampleRate)))
	for index := range samples {
		if receiver.squelchLevelOpen {
			receiver.squelchGain += attack * (1 - receiver.squelchGain)
		} else if receiver.squelchHoldRemaining > 0 {
			receiver.squelchHoldRemaining--
		} else {
			if !receiver.squelchClosing && receiver.squelchGain > 0 {
				receiver.squelchClosing = true
				receiver.squelchCloseRemaining = max(1, demodulatedAudioSampleRate*receiver.squelchCloseMs/1000)
				receiver.squelchCloseStartGain = receiver.squelchGain
			}
			if receiver.squelchClosing {
				total := max(1, demodulatedAudioSampleRate*receiver.squelchCloseMs/1000)
				remaining := float32(receiver.squelchCloseRemaining) / float32(total)
				receiver.squelchGain = receiver.squelchCloseStartGain * remaining * remaining * remaining
				receiver.squelchCloseRemaining--
				if receiver.squelchCloseRemaining <= 0 {
					receiver.squelchGain = 0
					receiver.squelchClosing = false
				}
			}
		}
		samples[index] *= receiver.squelchGain
	}
	receiver.stats.SquelchOpen = receiver.squelchLevelOpen || receiver.squelchHoldRemaining > 0 || receiver.squelchGain > .001
}

// runTuner mirrors IC-SDR: hardware tuning and IQ reads proceed concurrently.
func (receiver *Receiver) runTuner() {
	defer close(receiver.tuneDone)
	for {
		select {
		case <-receiver.stop:
			return
		case frequencyHz := <-receiver.tune:
			if err := receiver.device.setCenterFrequency(frequencyHz); err != nil {
				receiver.setError(err)
				continue
			}
			actualFrequencyHz := receiver.device.centerFrequency()
			if actualFrequencyHz <= 0 {
				actualFrequencyHz = frequencyHz
			}
			receiver.mu.Lock()
			receiver.stats.SpectrumCenterHz = actualFrequencyHz
			for index := range receiver.spectrum {
				receiver.spectrum[index] = -120
			}
			receiver.mu.Unlock()
			receiver.spectrumGeneration.Add(1)
		case settings := <-receiver.settings:
			err := receiver.device.applyHardwareSettings(settings)
			confirmed := receiver.device.hardwareSettings()
			receiver.mu.Lock()
			receiver.hardware = confirmed
			receiver.mu.Unlock()
			if err != nil {
				receiver.setError(err)
			}
		}
	}
}

func (receiver *Receiver) addEvent(timeout, overflow bool) {
	receiver.mu.Lock()
	if timeout {
		receiver.stats.Timeouts++
	}
	if overflow {
		receiver.stats.Overflows++
	}
	receiver.mu.Unlock()
}

func (receiver *Receiver) setError(err error) {
	receiver.mu.Lock()
	receiver.stats.Status = "ERROR: " + err.Error()
	receiver.mu.Unlock()
}

func iqStats(interleaved []float32) (rms, peak float64, invalid uint64) {
	var power float64
	samples := len(interleaved) / 2
	for index := 0; index+1 < len(interleaved); index += 2 {
		iValue, qValue := float64(interleaved[index]), float64(interleaved[index+1])
		if math.IsNaN(iValue) || math.IsInf(iValue, 0) || math.IsNaN(qValue) || math.IsInf(qValue, 0) {
			invalid++
			continue
		}
		power += iValue*iValue + qValue*qValue
		peak = max(peak, math.Abs(iValue), math.Abs(qValue))
	}
	if samples > 0 {
		rms = math.Sqrt(power / float64(samples))
	}
	return rms, peak, invalid
}

func (config Config) deviceArguments() string {
	arguments := "driver=" + config.Driver
	if config.Serial != "" {
		arguments += ",serial=" + config.Serial
	}
	return arguments
}

func (stats Stats) String() string {
	return fmt.Sprintf("%s · %.3f MS/s · IQ RMS %.4f PEAK %.4f · FFT %.1f/s · OVF %d",
		stats.Status, stats.SampleRate/1e6, stats.RMS, stats.Peak, stats.FFTPerSecond, stats.Overflows)
}
