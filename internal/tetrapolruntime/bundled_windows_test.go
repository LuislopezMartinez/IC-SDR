package tetrapolruntime

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func bundledRuntimePath(name string) string {
	return filepath.Join("..", "..", "DATA", "tools", "tetrapol", "runtime", "bin", name)
}

func TestBundledTetrapolDumpStarts(t *testing.T) {
	output, err := exec.Command(bundledRuntimePath("tetrapol_dump.exe"), "-h").CombinedOutput()
	if err != nil {
		t.Fatalf("tetrapol_dump -h: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "Decode data from demodulated TETRAPOL channel") {
		t.Fatalf("unexpected tetrapol_dump help: %s", output)
	}
}

func TestBundledRPCELPProducesOneAudioFrame(t *testing.T) {
	command := exec.Command(bundledRuntimePath("rpcelp.exe"))
	command.Stdin = strings.NewReader(strings.Repeat("0", 120) + "\n")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(output), 160*2; got != want {
		t.Fatalf("RP-CELP output = %d bytes, want %d", got, want)
	}
}
