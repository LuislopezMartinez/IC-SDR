package main

import (
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
	var err error
	startupLogFile, err = configureStartupLog()
	if err != nil {
		showStartupError("IC-SDR cannot create DATA\\logs\\startup.log:\n\n" + err.Error() + "\n\nCheck that the portable folder is writable.")
		return
	}
	if startupLogFile != nil {
		defer startupLogFile.Close()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			message := "IC-SDR could not start. See DATA\\logs\\startup.log."
			startupStep("UNRECOVERABLE FAILURE: %v\n%s", recovered, debug.Stack())
			showStartupError(message)
		}
	}()
	startupStep("Process started · PID=%d · %s/%s · Go=%s", os.Getpid(), runtime.GOOS, runtime.GOARCH, runtime.Version())
	executable, executableErr := os.Executable()
	workingDirectory, workingErr := os.Getwd()
	startupStep("Executable=%q (error=%v)", executable, executableErr)
	startupStep("Working directory=%q (error=%v)", workingDirectory, workingErr)
	startupStep("Arguments=%q", os.Args)
	startupStep("DATA=%q", resources.WritablePath())
	stopWatchdog := startStartupWatchdog()
	defer stopWatchdog()

	if len(os.Args) == 3 && os.Args[1] == "--rtl433-viewer" {
		startupStep("Opening RTL_433 viewer")
		screens.RunRTL433Viewer(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--aprs-viewer" {
		startupStep("Opening APRS viewer")
		screens.RunAPRSViewer(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--ais-map" {
		startupStep("Opening AIS map")
		screens.RunAISMap(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--aircraft-map" {
		startupStep("Opening ADS-B map")
		screens.RunAircraftMap(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--satellite-map" {
		startupStep("Opening satellite map")
		screens.RunSatelliteMap(os.Args[2])
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "--tetra-viewer" {
		startupStep("Opening TETRA console")
		screens.RunTETRAViewer(os.Args[2], os.Args[3])
		return
	}
	startupStep("Configuring SimpleUI")
	simpleui.SetLifecycleLogger(startupStep)
	simpleui.SetMode(1600, 900, simpleui.Stretch)
	simpleui.SetTextScale(1.25)
	simpleui.SetTitle("IC-SDR · Go port")
	simpleui.SetMinimumSize(960, 540)

	// Same baseline as the original RSP1B profile in the reference capture.
	// In particular, restoring -14 dBFS avoids the driver's common -30 dB AGC
	// default, which otherwise lowers the whole Go spectrum by about 16 dB.
	initialHardware := &sdr.HardwareSettings{
		AGC: true, RFGain: 0, IFGain: 20, AGCSetpoint: -14, IQCorrection: true,
	}
	startupStep("Checking portable resources")
	logPortableResources()
	startupStep("Building receiver and decoders")
	receiver := sdr.NewReceiver(sdr.Config{
		RuntimeRoot: sdrRuntimeRoot(),
		Driver:      "sdrplay",
		// Empty serial accepts any connected RSP. If none is available the
		// receiver automatically falls back to an RTL-SDR device.
		Serial:                    "",
		FrequencyHz:               14_261_000,
		SampleRate:                2_048_000,
		FFTSize:                   4096,
		CalibrationDB:             23,
		InitialHardware:           initialHardware,
		DMRExecutable:             toolExecutable("dmr", "dmr_sample_runner"),
		RTL433Executable:          toolExecutable("rtl_433", "rtl_433"),
		RadiosondeDirectory:       resources.Path("tools", "radiosonde", "runtime", "bin"),
		AISExecutable:             toolExecutable("ais", "AIS-catcher"),
		Aircraft1090Executable:    toolExecutable("aircraft", "dump1090"),
		Aircraft978Executable:     toolExecutable("aircraft", "dump978"),
		AircraftUATTextExecutable: toolExecutable("aircraft", "uat2text"),
		APRSExecutable:            toolExecutable("aprs", "direwolf"),
		APRSConfig:                resources.Path("tools", "aprs", "config", "direwolf-rx.conf"),
		APRSWorkingDirectory:      resources.Path("tools", "aprs", "runtime"),
		SSTVExecutable:            toolExecutable("sstv", "sstv_decoder_runner"),
		TETRACodec:                tetraCodecPath(),
		SSTVOutputDirectory:       resources.WritablePath("captures", "sstv"),
		StartupLog:                startupStep,
	})
	startupStep("Receiver built")
	startupStep("Opening SDR device")
	if err := receiver.Start(); err != nil {
		startupStep("Receiver could not start; the interface will remain available: %v", err)
	} else {
		startupStep("SDR device started successfully")
	}
	defer receiver.Close()

	startupStep("Creating MainScreen")
	mainScreen := screens.NewMainScreen(receiver)
	startupStep("MainScreen created")
	defer mainScreen.Close()
	startupStep("Creating controls")
	mainScreen.CreateControls()
	startupStep("Controls created")
	var firstFrame sync.Once
	simpleui.Run(func() {
		firstFrame.Do(func() {
			startupStep("First interface frame started; startup completed")
			stopWatchdog()
		})
		mainScreen.Draw()
	})
	startupStep("Normal shutdown")
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
	log.Print("STEP · " + message)
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
				log.Printf("WAIT · process still inside: %s", stage)
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
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			startupStep("Resource MISSING: %q · %v", path, err)
			continue
		}
		startupStep("Resource OK: %q · %d bytes", path, info.Size())
	}
}
