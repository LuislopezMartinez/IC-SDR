package resources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDevelopmentResourceCanBeLocatedOutsideWorkingDirectory(t *testing.T) {
	path := Path("config", "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("resource path %q is not usable: %v", path, err)
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
	want := filepath.Join(root, "DATA") + string(filepath.Separator)
	got := WritablePath("recordings", "capture.wav")
	if len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("writable path escaped DATA: %q; want prefix %q", got, want)
	}
}
