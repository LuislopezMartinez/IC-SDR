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
		showStartupError("IC-SDR no puede crear DATA\\logs\\startup.log:\n\n" + err.Error() + "\n\nCompruebe que la carpeta portable permite escritura.")
		return
	}
	if startupLogFile != nil {
		defer startupLogFile.Close()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			message := "IC-SDR no pudo iniciarse. Consulte DATA\\logs\\startup.log."
			startupStep("FALLO IRRECUPERABLE: %v\n%s", recovered, debug.Stack())
			showStartupError(message)
		}
	}()
	startupStep("Proceso iniciado · PID=%d · %s/%s · Go=%s", os.Getpid(), runtime.GOOS, runtime.GOARCH, runtime.Version())
	executable, executableErr := os.Executable()
	workingDirectory, workingErr := os.Getwd()
	startupStep("Ejecutable=%q (error=%v)", executable, executableErr)
	startupStep("Directorio de trabajo=%q (error=%v)", workingDirectory, workingErr)
	startupStep("Argumentos=%q", os.Args)
	startupStep("DATA=%q", resources.WritablePath())
	stopWatchdog := startStartupWatchdog()
	defer stopWatchdog()

	if len(os.Args) == 3 && os.Args[1] == "--rtl433-viewer" {
		startupStep("Abriendo visor RTL_433")
		screens.RunRTL433Viewer(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--aprs-viewer" {
		startupStep("Abriendo visor APRS")
		screens.RunAPRSViewer(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--ais-map" {
		startupStep("Abriendo mapa AIS")
		screens.RunAISMap(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--aircraft-map" {
		startupStep("Abriendo mapa ADS-B")
		screens.RunAircraftMap(os.Args[2])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--satellite-map" {
		startupStep("Abriendo mapa de satélites")
		screens.RunSatelliteMap(os.Args[2])
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "--tetra-viewer" {
		startupStep("Abriendo consola TETRA")
		screens.RunTETRAViewer(os.Args[2], os.Args[3])
		return
	}
	startupStep("Configurando SimpleUI")
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
	startupStep("Comprobando recursos portables")
	logPortableResources()
	startupStep("Construyendo receptor y decodificadores")
	receiver := sdr.NewReceiver(sdr.Config{
		RuntimeRoot: resources.Path("runtime", "windows-x64"),
		Driver:      "sdrplay",
		// Empty serial accepts any connected RSP. If none is available the
		// receiver automatically falls back to an RTL-SDR device.
		Serial:                    "",
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
		SSTVOutputDirectory:       resources.WritablePath("captures", "sstv"),
		StartupLog:                startupStep,
	})
	startupStep("Receptor construido")
	startupStep("Abriendo dispositivo SDR")
	if err := receiver.Start(); err != nil {
		startupStep("El receptor no se pudo iniciar; la interfaz continuará disponible: %v", err)
	} else {
		startupStep("Dispositivo SDR iniciado correctamente")
	}
	defer receiver.Close()

	startupStep("Creando MainScreen")
	mainScreen := screens.NewMainScreen(receiver)
	startupStep("MainScreen creado")
	defer mainScreen.Close()
	startupStep("Creando controles")
	mainScreen.CreateControls()
	startupStep("Controles creados")
	if err := mainScreen.StartConfiguredWebServer(); err != nil {
		startupStep("Servidor web no disponible: %v", err)
	}
	var firstFrame sync.Once
	simpleui.Run(func() {
		firstFrame.Do(func() {
			startupStep("Primera trama de interfaz iniciada; arranque completado")
			stopWatchdog()
		})
		mainScreen.Draw()
	})
	startupStep("Cierre normal")
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
	log.Print("PASO · " + message)
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
				log.Printf("ESPERA · el proceso sigue dentro de: %s", stage)
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
			startupStep("Recurso AUSENTE: %q · %v", path, err)
			continue
		}
		startupStep("Recurso OK: %q · %d bytes", path, info.Size())
	}
}
