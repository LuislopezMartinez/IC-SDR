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
	band := flag.String("band", "UHF", "perfil VHF o UHF")
	channel := flag.String("channel", "TCH", "tipo CCH o TCH")
	direction := flag.String("direction", "DOWN", "dirección DOWN o UP")
	runtimeDir := flag.String("runtime", filepath.Join("DATA", "tools", "tetrapol", "runtime", "bin"), "directorio del runtime")
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

	pipeline := &tetrapolruntime.Pipeline{}
	err = pipeline.Start(tetrapolruntime.Config{
		KitExecutable:    filepath.Join(*runtimeDir, "tetrapol_dump.exe"),
		KitArgs:          []string{"-b", *band, "-t", *channel, "-d", *direction},
		RPCELPExecutable: filepath.Join(*runtimeDir, "rpcelp.exe"),
	}, func([]float32) {})
	if err != nil {
		fatal(err)
	}
	decoder := tetrapol.New(float64(reader.SampleRate()))
	decoder.SetBitSink(func(bits []byte) { _ = pipeline.SubmitBits(bits) })
	decoder.Configure(true)
	for {
		iq, readErr := reader.ReadFrames(8192)
		decoder.ProcessIQ(iq)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = pipeline.Stop()
			fatal(readErr)
		}
	}
	time.Sleep(300 * time.Millisecond)
	telemetry := pipeline.Telemetry()
	physical := decoder.Snapshot()
	_ = pipeline.Stop()
	fmt.Printf("IQ: %.0f sps · %.1f dBFS · %.0f Hz desviación · %d símbolos\n", physical.InputRate, physical.LevelDBFS, physical.FrequencyDeviationHz, physical.Symbols)
	fmt.Printf("TETRAPOL: %d válidas · %d CRC · calidad %.1f%% · voz=%v · SCR=%v/%d\n", telemetry.ValidFrames, telemetry.CRCFailures, telemetry.QualityPercent, telemetry.VoiceActive, telemetry.SCRKnown, telemetry.SCR)
	if telemetry.ValidFrames == 0 {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "TETRAPOL CHECK:", err)
	os.Exit(2)
}
