package resources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevelopmentResourceCanBeLocatedOutsideWorkingDirectory(t *testing.T) {
	path := Path("config", "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("optional DATA file not in this checkout: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("resource path must be absolute: %q", path)
	}
}

func TestWritablePathStaysInsideProjectData(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "DATA")
	if _, err := os.Stat(want); err != nil {
		t.Skip("DATA tree is not in this checkout")
	}
	got := WritablePath("recordings", "capture.wav")
	prefix := want + string(filepath.Separator)
	if got != want && !strings.HasPrefix(got, prefix) {
		t.Fatalf("writable path escaped DATA: %q; want prefix %q", got, prefix)
	}
	if isEphemeralExecutableDir(filepath.Dir(got)) {
		t.Fatalf("writable path used go test temp dir: %q", got)
	}
}

func TestEphemeralGoBuildDirIsDetected(t *testing.T) {
	if !isEphemeralExecutableDir(`C:\Users\PHolm\AppData\Local\Temp\go-build1242902542\b264`) {
		t.Fatal("windows go-build temp dir was not detected")
	}
	if !isEphemeralExecutableDir("/tmp/go-build123/b001") {
		t.Fatal("unix go-build temp dir was not detected")
	}
	if isEphemeralExecutableDir(`D:\IC-SDR-Go`) {
		t.Fatal("portable install dir was treated as ephemeral")
	}
}
