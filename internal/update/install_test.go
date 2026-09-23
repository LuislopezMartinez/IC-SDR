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
