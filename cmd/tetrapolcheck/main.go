package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go-zero/internal/iqcapture"
	"go-zero/internal/tetrapol"
	"go-zero/internal/tetrapolruntime"
)

func main() {
	channel := flag.String("channel", "TCH", "tipo CCH o TCH")
	nativeDLL := flag.String("native", filepath.Join("DATA", "tools", "tetrapol", "native", "bin", "lilipol_native_x64.dll"), "DLL nativa x64")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "uso: tetrapolcheck [opciones] captura.wav")
		os.Exit(2)
	}
	reader, err := iqcapture.Open(flag.Arg(0))
	if err != nil {
		fatal(err)
	}
	defer reader.Close()

	native := &tetrapolruntime.Native{}
	err = native.Start(*nativeDLL, *channel, float64(reader.SampleRate()), func([]float32) {})
	if err != nil {
		fatal(err)
	}
	decoder := tetrapol.New(float64(reader.SampleRate()))
	decoder.Configure(true)
	for {
		iq, readErr := reader.ReadFrames(8192)
		decoder.ProcessIQ(iq)
		if submitErr := native.SubmitIQ(iq); submitErr != nil {
			_ = native.Stop()
			fatal(submitErr)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = native.Stop()
			fatal(readErr)
		}
	}
	time.Sleep(300 * time.Millisecond)
	telemetry := native.Telemetry()
	physical := decoder.Snapshot()
	_ = native.Stop()
	fmt.Printf("IQ: %.0f sps · %.1f dBFS · %.0f Hz desviación · AFC %s %+.0f Hz (error %+.0f) · %d símbolos\n", physical.InputRate, physical.LevelDBFS, physical.FrequencyDeviationHz, physical.AFCState, physical.CarrierOffsetHz, physical.AFCErrorHz, physical.Symbols)
	fmt.Printf("TETRAPOL: %d válidas · %d CRC · calidad %.1f%% · voz=%v · SCR=%v/%d\n", telemetry.ValidFrames, telemetry.CRCFailures, telemetry.QualityPercent, telemetry.VoiceActive, telemetry.SCRKnown, telemetry.SCR)
	if telemetry.ValidFrames == 0 {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "TETRAPOL CHECK:", err)
	os.Exit(2)
}
