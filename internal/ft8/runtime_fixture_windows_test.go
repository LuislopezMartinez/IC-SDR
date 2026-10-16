//go:build windows

package ft8

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Set FT8_FIXTURE_WAV to a 12 kHz mono PCM16 slot for an end-to-end decoder
// check. The repository test suite does not need to carry a large radio file.
func TestBundledRuntimeWithOptionalFixture(t *testing.T) {
	fixture := os.Getenv("FT8_FIXTURE_WAV")
	if fixture == "" {
		t.Skip("FT8_FIXTURE_WAV is not set")
	}
	wav, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(wav) < 44+SlotSamples*2 || string(wav[:4]) != "RIFF" || binary.LittleEndian.Uint32(wav[24:28]) != DecoderRate {
		t.Fatal("fixture must be a complete 12 kHz PCM16 WAV slot")
	}
	executable := filepath.Join("..", "..", "DATA", "tools", "ft8", "runtime", "bin", "ic-sdr-ft8-runtime.exe")
	var input bytes.Buffer
	input.WriteString("FT8S")
	_ = binary.Write(&input, binary.LittleEndian, int64(1_700_000_000))
	_ = binary.Write(&input, binary.LittleEndian, uint32(SlotSamples))
	input.Write(wav[44 : 44+SlotSamples*2])
	cmd := exec.Command(executable, "--budget", "5")
	cmd.Stdin = &input
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runtime failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), `"type":"decode"`) {
		t.Fatalf("fixture produced no FT8 decodes:\n%s", output)
	}
}
