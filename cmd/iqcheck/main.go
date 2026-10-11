package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go-zero/internal/resources"
	"go-zero/internal/sdr"
)

func main() {
	duration := flag.Duration("duration", 5*time.Second, "duración de la prueba")
	retuneEvery := flag.Duration("retune-every", 0, "intervalo entre resintonizaciones; 0 las desactiva")
	frequency := flag.Int64("frequency", 14_200_000, "frecuencia central inicial en Hz")
	flag.Parse()

	receiver := sdr.NewReceiver(sdr.Config{
		RuntimeRoot:   resources.Path("runtime", "windows-x64"),
		Driver:        "sdrplay",
		Serial:        "2401019760",
		FrequencyHz:   *frequency,
		SampleRate:    2_048_000,
		FFTSize:       4096,
		CalibrationDB: 23,
	})
	if err := receiver.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "IQ CHECK ERROR:", err)
		os.Exit(1)
	}
	defer receiver.Close()
	receiver.SetDemodulator("USB", *frequency, 2_400)

	spectrum := make([]float32, 4096)
	totalSeconds := int(duration.Round(time.Second) / time.Second)
	if totalSeconds < 1 {
		totalSeconds = 1
	}
	retuneSeconds := int(retuneEvery.Round(time.Second) / time.Second)
	currentFrequency := *frequency
	previous := receiver.Snapshot(spectrum)
	for second := 1; second <= totalSeconds; second++ {
		time.Sleep(time.Second)
		if retuneSeconds > 0 && second%retuneSeconds == 0 {
			if currentFrequency == *frequency {
				currentFrequency = *frequency + 100_000
			} else {
				currentFrequency = *frequency
			}
			receiver.SetCenterFrequency(currentFrequency)
			receiver.SetDemodulator("USB", currentFrequency, 2_400)
			fmt.Printf("      retune %.6f MHz\n", float64(currentFrequency)/1e6)
		}
		stats := receiver.Snapshot(spectrum)
		fmt.Printf("[%d/%d] samples/s=%d fft/s=%d timeout/s=%d ovf/s=%d restarts=%d · %s\n",
			second, totalSeconds,
			stats.ReceivedSamples-previous.ReceivedSamples,
			stats.FFTBlocks-previous.FFTBlocks,
			stats.Timeouts-previous.Timeouts,
			stats.Overflows-previous.Overflows,
			stats.CaptureRestarts, stats.String())
		previous = stats
	}

	stats := receiver.Snapshot(spectrum)
	if stats.ReceivedSamples == 0 || stats.FFTBlocks == 0 || stats.RMS == 0 || stats.InvalidSamples != 0 {
		fmt.Fprintln(os.Stderr, "IQ CHECK FAILED: el stream no cumple la validación mínima")
		os.Exit(2)
	}
	fmt.Println("IQ CHECK OK: muestras IQ válidas y FFT alimentada correctamente")
}
