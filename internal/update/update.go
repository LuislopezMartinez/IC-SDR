// Package update checks the official IC-SDR release feed.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
type githubRelease struct {
	TagName, Body string `json:"tag_name"`
	Assets        []struct {
		Name, URL string `json:"browser_download_url"`
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

func Download(ctx context.Context, release Release, destination string) error {
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
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, 2<<30)); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), want[0]) {
		return fmt.Errorf("la comprobación SHA-256 ha fallado")
	}
	return file.Sync()
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
