package main

import (
	"go-zero/internal/i18n"

	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-zero/internal/resources"
	"go-zero/internal/sdr"
	"go-zero/screens"
	"go-zero/simpleui"
)

var startupLogFile *os.File
var startupStage atomic.Value

func main() {
	i18n.Init()
	var err error
	startupLogFile, err = configureStartupLog()
	if err != nil {
		showStartupError("IC-SDR no puede crear DATA\\logs\\startup.log:\n\n" + err.Error() + i18n.Source("text.db1ccb72bda6"))
		return
	}
	if startupLogFile != nil {
		defer startupLogFile.Close()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			message := "IC-SDR no pudo iniciarse. Consulte DATA\\logs\\startup.log."
			startupStep(i18n.Source("text.a494185715d3"), recovered, debug.Stack())
			showStartupError(message)
		}
	}()
	startupStep(i18n.Source("text.1eb6e8bf9c68"), os.Getpid(), runtime.GOOS, runtime.GOARCH, runtime.Version())
	executable, executableErr := os.Executable()
	workingDirectory, workingErr := os.Getwd()
	startupStep(i18n.Source("text.ae2092f2053f"), executable, executableErr)
	startupStep(i18n.Source("text.1fda812726e4"), workingDirectory, workingErr)
	startupStep("Argumentos=%q", os.Args)
	startupStep("DATA=%q", resources.WritablePath())
	stopWatchdog := startStartupWatchdog()
	defer stopWatchdog()

	if len(os.Args) == 3 && os.Args[1] == "--rtl433-viewer" {
		startupStep("%s", i18n.Source("text.d4572e98e78b"))
		screens.RunRTL433Viewer(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--aprs-viewer" {
		startupStep("%s", i18n.Source("text.62c027f9cb9a"))
		screens.RunAPRSViewer(os.Args[2])
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--distance-map" {
		screens.RunDistanceMap()
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--aprs-map" {
		screens.RunAPRSMap(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--ais-map" {
		startupStep("%s", i18n.Source("text.2f21ce1b069d"))
		screens.RunAISMap(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--aircraft-map" {
		startupStep("%s", i18n.Source("text.6d35800b9a29"))
		screens.RunAircraftMap(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--satellite-map" {
		startupStep("%s", i18n.Source("text.dafb6513636c"))
		screens.RunSatelliteMap(os.Args[2])
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "--tetra-viewer" {
		startupStep("%s", i18n.Source("text.682bc14da825"))
		screens.RunTETRAViewer(os.Args[2], os.Args[3])
		return
	}
	startupStep("%s", i18n.Source("text.4228c897f8e8"))
	simpleui.SetLifecycleLogger(startupStep)
	simpleui.SetMode(1600, 900, simpleui.Stretch)
	simpleui.SetTextScale(1.25)
	simpleui.SetTitle(i18n.Source("text.29b8efd09a7c"))
	simpleui.SetMinimumSize(960, 540)
	// F1-F12 are available for user-assigned memory shortcuts. In particular,
	// do not reserve F11 for window maximization in the main receiver window.
	simpleui.SetMaximizeKey(0)

	// Same baseline as the original RSP1B profile in the reference capture.
	// In particular, restoring -14 dBFS avoids the driver's common -30 dB AGC
	// default, which otherwise lowers the whole Go spectrum by about 16 dB.
	initialHardware := &sdr.HardwareSettings{
		AGC: true, RFGain: 0, IFGain: 20, AGCSetpoint: -14, IQCorrection: true,
	}
	startupStep("%s", i18n.Source("text.1e19e1c6764f"))
	logPortableResources()
	startupStep("%s", i18n.Source("text.d41900a2bab0"))
	preferredPath := resources.WritablePath("config", "sdr-device.json")
	preferred := sdr.LoadPreferredDevice(preferredPath)
	if preferred.Driver == "" {
		preferred.Driver = "sdrplay"
	}
	receiver := sdr.NewReceiver(sdr.Config{
		RuntimeRoot:         resources.Path("runtime", "windows-x64"),
		PreferredDevicePath: preferredPath,
		Driver:              preferred.Driver,
		// Empty serial accepts any connected RSP. If none is available the
		// receiver automatically falls back to an RTL-SDR device.
		Serial:                    preferred.Serial,
		FrequencyHz:               14_261_000,
		SampleRate:                2_048_000,
		FFTSize:                   4096,
		CalibrationDB:             23,
		InitialHardware:           initialHardware,
		DMRExecutable:             resources.Path("tools", "dmr", "runtime", "bin", "dmr_sample_runner.exe"),
		DigitalVoiceExecutable:    resources.Path("tools", "digital_voice", "runtime", "bin", "dsd-neo.exe"),
		RTL433Executable:          resources.Path("tools", "rtl_433", "runtime", "bin", "rtl_433.exe"),
		RadiosondeDirectory:       resources.Path("tools", "radiosonde", "runtime", "bin"),
		AISExecutable:             resources.Path("tools", "ais", "runtime", "bin", "AIS-catcher.exe"),
		Aircraft1090Executable:    resources.Path("tools", "aircraft", "runtime", "bin", "dump1090.exe"),
		Aircraft978Executable:     resources.Path("tools", "aircraft", "runtime", "bin", "dump978.exe"),
		AircraftUATTextExecutable: resources.Path("tools", "aircraft", "runtime", "bin", "uat2text.exe"),
		APRSExecutable:            resources.Path("tools", "aprs", "runtime", "bin", "direwolf.exe"),
		APRSConfig:                resources.Path("tools", "aprs", "config", "direwolf-rx.conf"),
		APRSWorkingDirectory:      resources.Path("tools", "aprs", "runtime"),
		SSTVExecutable:            resources.Path("tools", "sstv", "runtime", "bin", "sstv_decoder_runner.exe"),
		TETRACodec:                resources.Path("tools", "tetra", "runtime", "bin", "libtetradec.dll"),
		TETRAPOLKitExecutable:     resources.Path("tools", "tetrapol", "runtime", "bin", "tetrapol_dump.exe"),
		TETRAPOLRPCELPExecutable:  resources.Path("tools", "tetrapol", "runtime", "bin", "rpcelp.exe"),
		SSTVOutputDirectory:       resources.WritablePath("captures", "sstv"),
		StartupLog:                startupStep,
	})
	startupStep("%s", i18n.Source("text.23bbe630f3e8"))
	startupStep("%s", i18n.Source("text.052a8cf7778f"))
	if err := receiver.Start(); err != nil {
		startupStep(i18n.Source("text.61cc30e1d60a"), err)
	} else {
		startupStep("%s", i18n.Source("text.fc31cbf5a67f"))
	}
	defer receiver.Close()

	startupStep("%s", i18n.Source("text.f0d082de2f44"))
	mainScreen := screens.NewMainScreen(receiver)
	startupStep("%s", i18n.Source("text.37c3e3da83c9"))
	defer mainScreen.Close()
	startupStep("%s", i18n.Source("text.5f0bca1a6790"))
	mainScreen.CreateControls()
	startupStep("%s", i18n.Source("text.e0e73d388406"))
	if err := mainScreen.StartConfiguredWebServer(); err != nil {
		startupStep(i18n.Source("text.bfb85c80ae58"), err)
	}
	var firstFrame sync.Once
	simpleui.Run(func() {
		firstFrame.Do(func() {
			startupStep("%s", i18n.Source("text.3f70ba22ee80"))
			stopWatchdog()
		})
		mainScreen.Draw()
	})
	startupStep("%s", i18n.Source("text.b7ad813982af"))
}

func configureStartupLog() (*os.File, error) {
	name := "startup.log"
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--") {
		role := strings.Trim(strings.ReplaceAll(os.Args[1], "_", "-"), "-")
		name = "startup-" + role + ".log"
	}
	logPath := resources.WritablePath("logs", name)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	// A windowsgui executable has no usable stderr handle. Writing directly to
	// the file ensures the first diagnostic is not discarded by MultiWriter.
	log.SetOutput(file)
	return file, nil
}

func startupStep(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	startupStage.Store(message)
	log.Print(i18n.Source("text.de8317d391e4") + message)
	if startupLogFile != nil {
		_ = startupLogFile.Sync()
	}
}

func startStartupWatchdog() func() {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stage, _ := startupStage.Load().(string)
				log.Printf(i18n.Source("text.ade951723c79"), stage)
				if startupLogFile != nil {
					_ = startupLogFile.Sync()
				}
			case <-done:
				return
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

func logPortableResources() {
	paths := []string{
		resources.Path("runtime", "windows-x64", "bin", "SoapySDR.dll"),
		resources.Path("runtime", "windows-x64", "bin", "MSVCP140.dll"),
		resources.Path("runtime", "windows-x64", "bin", "VCRUNTIME140.dll"),
		resources.Path("runtime", "windows-x64", "bin", "VCRUNTIME140_1.dll"),
		resources.Path("runtime", "windows-x64", "lib", "SoapySDR", "modules0.8", "sdrPlaySupport.dll"),
		resources.Path("runtime", "windows-x64", "lib", "SoapySDR", "modules0.8", "rtlsdrSupport.dll"),
		resources.Path("tools", "digital_voice", "runtime", "bin", "dsd-neo.exe"),
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			startupStep(i18n.Source("text.c5161cab873e"), path, err)
			continue
		}
		startupStep(i18n.Source("text.9cdeef9cc7f0"), path, info.Size())
	}
}
