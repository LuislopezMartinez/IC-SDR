package main

import (
	"fmt"
	"os"
	"time"

	"go-zero/internal/resources"
	"go-zero/internal/sdr"
)

func main() {
	receiver := sdr.NewReceiver(sdr.Config{
		RuntimeRoot:   resources.Path("runtime", "windows-x64"),
		Driver:        "sdrplay",
		Serial:        "2401019760",
		FrequencyHz:   14_261_000,
		SampleRate:    2_048_000,
		FFTSize:       4096,
		CalibrationDB: 23,
	})
	if err := receiver.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "IQ CHECK ERROR:", err)
		os.Exit(1)
	}
	defer receiver.Close()
	receiver.SetDemodulator("NFM", 14_261_000, 12_500)

	spectrum := make([]float32, 4096)
	for second := 1; second <= 5; second++ {
		time.Sleep(time.Second)
		if second == 2 {
			receiver.SetCenterFrequency(14_361_000)
			fmt.Println("      retune +100 kHz")
		}
		if second == 4 {
			receiver.SetCenterFrequency(14_261_000)
			fmt.Println("      retune initial frequency")
		}
		stats := receiver.Snapshot(spectrum)
		fmt.Printf("[%d/5] %s · samples=%d · fft=%d · timeout=%d · bad=%d · audio=%d buffered=%d\n",
			second, stats.String(), stats.ReceivedSamples, stats.FFTBlocks,
			stats.Timeouts, stats.InvalidSamples, stats.AudioProduced, stats.AudioBuffered)
	}

	stats := receiver.Snapshot(spectrum)
	if stats.ReceivedSamples == 0 || stats.FFTBlocks == 0 || stats.RMS == 0 || stats.InvalidSamples != 0 {
		fmt.Fprintln(os.Stderr, "IQ CHECK FAILED: stream does not meet the minimum validation")
		os.Exit(2)
	}
	fmt.Println("IQ CHECK OK: valid IQ samples and FFT fed correctly")
}
