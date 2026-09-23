// Package update checks the official IC-SDR release feed.
package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CurrentVersion is set by build-release.ps1. Development builds deliberately
// do not offer installation over a checkout.
var CurrentVersion = "dev"

const latestURL = "https://api.github.com/repos/LuislopezMartinez/IC-SDR/releases/latest"
const packageName = "IC-SDR-Go-windows-x64.zip"

type Release struct{ Version, Notes, PackageURL, SHA256URL string }
type ProgressFunc func(downloaded, total int64)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func Check(ctx context.Context) (Release, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return Release{}, false, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "IC-SDR/"+CurrentVersion)
	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return Release{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, false, fmt.Errorf("servidor de actualizaciones: %s", response.Status)
	}
	var remote githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&remote); err != nil {
		return Release{}, false, err
	}
	release := Release{Version: remote.TagName, Notes: strings.TrimSpace(remote.Body)}
	for _, asset := range remote.Assets {
		if asset.Name == packageName {
			release.PackageURL = asset.URL
		}
		if asset.Name == packageName+".sha256" {
			release.SHA256URL = asset.URL
		}
	}
	if release.Version == "" || release.PackageURL == "" || release.SHA256URL == "" {
		return release, false, nil
	}
	return release, newer(release.Version, CurrentVersion), nil
}

func Download(ctx context.Context, release Release, destination string, progress ProgressFunc) error {
	checksum, err := getText(ctx, release.SHA256URL)
	if err != nil {
		return err
	}
	want := strings.Fields(checksum)
	if len(want) == 0 || len(want[0]) != 64 {
		return fmt.Errorf("checksum de actualización no válido")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, release.PackageURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "IC-SDR/"+CurrentVersion)
	response, err := (&http.Client{Timeout: 10 * time.Minute}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("descarga: %s", response.Status)
	}
	partial := destination + ".part"
	_ = os.Remove(partial)
	file, err := os.Create(partial)
	if err != nil {
		return err
	}
	hash := sha256.New()
	reader := io.Reader(io.LimitReader(response.Body, 2<<30))
	if progress != nil {
		reader = &progressReader{reader: reader, total: response.ContentLength, report: progress}
	}
	if _, err = io.Copy(io.MultiWriter(file, hash), reader); err != nil {
		file.Close()
		_ = os.Remove(partial)
		return err
	}
	if err = file.Sync(); err == nil {
		err = file.Close()
	} else {
		file.Close()
	}
	if err != nil {
		_ = os.Remove(partial)
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), want[0]) {
		_ = os.Remove(partial)
		return fmt.Errorf("la comprobación SHA-256 ha fallado")
	}
	_ = os.Remove(destination)
	return os.Rename(partial, destination)
}

type progressReader struct {
	reader      io.Reader
	total, done int64
	report      ProgressFunc
}

func (reader *progressReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.done += int64(count)
	reader.report(reader.done, reader.total)
	return count, err
}

// Prepare downloads and validates the release, extracts it beside the current
// portable installation and starts a temporary copy of the external updater.
// The returned process has already been detached; the caller should then close
// IC-SDR normally so files and SDR helper processes are released cleanly.
func Prepare(ctx context.Context, release Release, progress ProgressFunc) error {
	if _, ok := parse(CurrentVersion); !ok {
		return fmt.Errorf("las compilaciones de desarrollo no se pueden actualizar automáticamente")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	installDir := filepath.Dir(executable)
	if !strings.EqualFold(filepath.Base(executable), "IC-SDR-Go.exe") {
		return fmt.Errorf("la aplicación no se está ejecutando desde una distribución portable")
	}
	workDir, err := os.MkdirTemp(filepath.Dir(installDir), ".ic-sdr-update-")
	if err != nil {
		return fmt.Errorf("no se puede preparar la actualización junto a la instalación: %w", err)
	}
	keepWorkDir := false
	defer func() {
		if !keepWorkDir {
			_ = os.RemoveAll(workDir)
		}
	}()
	packagePath := filepath.Join(workDir, packageName)
	if err := Download(ctx, release, packagePath, progress); err != nil {
		return err
	}
	payloadDir := filepath.Join(workDir, "payload")
	root, err := extractPackage(packagePath, payloadDir)
	if err != nil {
		return fmt.Errorf("paquete de actualización no válido: %w", err)
	}
	newApplication := filepath.Join(root, "IC-SDR-Go.exe")
	newUpdater := filepath.Join(root, "IC-SDR-Updater.exe")
	if !regularFile(newApplication) || !regularFile(newUpdater) {
		return fmt.Errorf("el paquete no contiene IC-SDR-Go.exe e IC-SDR-Updater.exe")
	}
	temporaryUpdater := filepath.Join(os.TempDir(), fmt.Sprintf("IC-SDR-Updater-%d.exe", time.Now().UnixNano()))
	if err := copyFile(newUpdater, temporaryUpdater); err != nil {
		return fmt.Errorf("no se pudo preparar el asistente de actualización: %w", err)
	}
	command := exec.Command(temporaryUpdater,
		"--target", installDir,
		"--source", root,
		"--work", workDir,
		"--restart", "IC-SDR-Go.exe",
		"--pid", strconv.Itoa(os.Getpid()),
	)
	command.Dir = filepath.Dir(installDir)
	if err := command.Start(); err != nil {
		_ = os.Remove(temporaryUpdater)
		return fmt.Errorf("no se pudo iniciar el asistente de actualización: %w", err)
	}
	keepWorkDir = true
	return nil
}

func extractPackage(packagePath, destination string) (string, error) {
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	cleanDestination, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cleanDestination, 0o755); err != nil {
		return "", err
	}
	for _, entry := range archive.File {
		name := filepath.Clean(filepath.FromSlash(entry.Name))
		if name == "." || filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("ruta no segura en el ZIP: %q", entry.Name)
		}
		target := filepath.Join(cleanDestination, name)
		if !within(cleanDestination, target) {
			return "", fmt.Errorf("ruta fuera del paquete: %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		source, err := entry.Open()
		if err != nil {
			return "", err
		}
		destinationFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err == nil {
			_, err = io.Copy(destinationFile, io.LimitReader(source, 2<<30))
		}
		closeErr := destinationFileClose(destinationFile)
		source.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	entries, err := os.ReadDir(cleanDestination)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return "", fmt.Errorf("el ZIP debe contener una única carpeta raíz")
	}
	return filepath.Join(cleanDestination, entries[0].Name()), nil
}

func destinationFileClose(file *os.File) error {
	if file == nil {
		return nil
	}
	return file.Close()
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func within(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func getText(ctx context.Context, url string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	response, err := (&http.Client{Timeout: 12 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum: %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024))
	return string(data), err
}
func newer(remote, local string) bool {
	a, oka := parse(remote)
	b, okb := parse(local)
	return oka && okb && (a[0] > b[0] || a[0] == b[0] && (a[1] > b[1] || a[1] == b[1] && a[2] > b[2]))
}
func parse(value string) ([3]int, bool) {
	var out [3]int
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i := range out {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
