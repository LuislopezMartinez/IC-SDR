package update

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGitHubReleaseFieldsDecodeIndependently(t *testing.T) {
	data := []byte(`{"tag_name":"v1.2.3","body":"notes","assets":[{"name":"IC-SDR-Go-windows-x64.zip","browser_download_url":"https://example.test/update.zip"}]}`)
	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v1.2.3" || release.Body != "notes" || len(release.Assets) != 1 || release.Assets[0].Name != packageName || release.Assets[0].URL == "" {
		t.Fatalf("release metadata decoded incorrectly: %+v", release)
	}
}

func TestExtractPackageRejectsPathTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("unsafe")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(t.TempDir(), "payload")
	if _, err := extractPackage(archivePath, destination); err == nil {
		t.Fatal("extractPackage accepted a path traversal entry")
	}
}

func TestExtractPackageReturnsSinglePortableRoot(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range map[string]string{
		"IC-SDR-Go/IC-SDR-Go.exe":      "application",
		"IC-SDR-Go/IC-SDR-Updater.exe": "updater",
	} {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write([]byte(content)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(t.TempDir(), "payload")
	root, err := extractPackage(archivePath, destination)
	if err != nil {
		t.Fatal(err)
	}
	if !regularFile(filepath.Join(root, "IC-SDR-Go.exe")) || !regularFile(filepath.Join(root, "IC-SDR-Updater.exe")) {
		t.Fatal("portable executables were not extracted")
	}
}

func TestExtractPackageReturnsPortableArchiveRoot(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range map[string]string{
		"IC-SDR-Go.exe":      "application",
		"IC-SDR-Updater.exe": "updater",
	} {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write([]byte(content)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(t.TempDir(), "payload")
	root, err := extractPackage(archivePath, destination)
	if err != nil {
		t.Fatal(err)
	}
	if root != destination {
		t.Fatalf("root = %q, want archive root %q", root, destination)
	}
	if !portablePackageAt(root) {
		t.Fatal("portable executables were not extracted at the archive root")
	}
}

func TestRestoreMutableDataKeepsUserConfiguration(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "backup")
	target := filepath.Join(root, "target")
	oldConfig := filepath.Join(backup, "DATA", "config")
	newConfig := filepath.Join(target, "DATA", "config")
	if err := os.MkdirAll(oldConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newConfig, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldConfig, "settings.json"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newConfig, "settings.json"), []byte("default"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := restoreMutableData(backup, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(newConfig, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "user" {
		t.Fatalf("restored settings = %q, want user", got)
	}
}

func TestRestoreMutableDataKeepsContestLogAndBackups(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(root, "backup")
	target := filepath.Join(root, "target")
	files := map[string]string{
		"oceania-dx-ssb-2026.json":                 `{"qsos":[{"worked_call":"EA3IWW"}]}`,
		filepath.Join("backups", "contest-1.json"): `{"qsos":[{"worked_call":"K3WU"}]}`,
		filepath.Join("archive", "finished.json"):  `{"qsos":[{"worked_call":"A44A"}]}`,
		filepath.Join("templates", "cq-ww.json"):   `{"contest":"CQ-WW-SSB"}`,
	}

	for name, content := range files {
		path := filepath.Join(backup, "DATA", "logbook", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stale := filepath.Join(target, "DATA", "logbook", "packaged-default.json")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := restoreMutableData(backup, target); err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(target, "DATA", "logbook", name))
		if err != nil {
			t.Fatalf("read restored %s: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("restored %s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("packaged logbook was not replaced: %v", err)
	}
}

func TestMoveChildrenRoundTrip(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	files := map[string]string{
		"IC-SDR-Go.exe": "application",
		filepath.Join("DATA", "config", "x.json"): "settings",
	}
	for name, content := range files {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := moveChildren(source, destination); err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		data, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil || string(data) != want {
			t.Fatalf("moved %s = %q, %v; want %q", name, data, err, want)
		}
	}
	if entries, err := os.ReadDir(source); err != nil || len(entries) != 0 {
		t.Fatalf("source was not emptied: %v, %v", entries, err)
	}
	if err := moveChildren(destination, source); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceDirectoryContents(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "backup", "DATA", "exports")
	destination := filepath.Join(root, "target", "DATA", "exports")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "user.csv"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "packaged.csv"), []byte("default"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceDirectoryContents(source, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "user.csv"))
	if err != nil || string(data) != "user" {
		t.Fatalf("restored export = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "packaged.csv")); !os.IsNotExist(err) {
		t.Fatalf("packaged data was not replaced: %v", err)
	}
}
