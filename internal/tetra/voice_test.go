package tetra

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testCodecPath(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("LIBTETRADEC"); path != "" {
		return path
	}
	name := "libtetradec.so"
	switch runtime.GOOS {
	case "windows":
		name = "libtetradec.dll"
	case "darwin":
		name = "libtetradec.dylib"
	}
	candidates := []string{
		filepath.Join("..", "..", "DATA", "tools", "tetra", "runtime", "bin", name),
		filepath.Join("..", "..", "build", "libtetradec", name),
		filepath.Join("..", "..", "build", "libtetradec", "Release", name),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			absolute, absErr := filepath.Abs(path)
			if absErr != nil {
				return path
			}
			return absolute
		}
	}
	return ""
}

func TestVoiceDecoderLoadsCodec(t *testing.T) {
	path := testCodecPath(t)
	if path == "" {
		t.Skip("libtetradec is not built; run scripts/build-libtetradec.ps1")
	}
	voice := newVoiceDecoder(path)
	if !voice.ready {
		t.Fatalf("codec %s failed to load: %s", path, voice.errText)
	}
	if _, ok := voice.decode(make([]byte, 431), false); ok {
		t.Fatal("short frame must be rejected")
	}
	pcm, ok := voice.decode(make([]byte, 432), false)
	if ok && len(pcm) != 480*6 {
		t.Fatalf("unexpected PCM length %d", len(pcm))
	}
}
