// Package resources locates files shipped with IC-SDR independently of the
// process working directory. Release builds place them in DATA beside the exe;
// development builds fall back to ORIGEN/IC_SDR, then the checkout DATA folder.
package resources

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// WritablePath returns a location inside DATA beside the executable. Keeping
// mutable state here makes a portable installation self-contained regardless
// of the process working directory.
func WritablePath(parts ...string) string {
	return filepath.Join(append([]string{dataRoot()}, parts...)...)
}

// dataRoot uses the portable DATA directory in release builds and the
// checkout's DATA directory during development (including `go run` and tests).
func dataRoot() string {
	exe := executableDir()
	release := filepath.Join(exe, "DATA")
	if exists(release) && !isEphemeralExecutableDir(exe) {
		return release
	}
	for _, start := range searchStarts() {
		if root := checkoutRoot(start); root != "" {
			dev := filepath.Join(root, "DATA")
			if exists(dev) || exists(filepath.Join(root, "ORIGEN", "IC_SDR")) {
				return dev
			}
		}
	}
	return release
}

// Path returns an absolute path for a resource relative to IC_SDR's data root.
// The returned release path is useful in error messages even when it is absent.
func Path(parts ...string) string {
	exe := executableDir()
	release := filepath.Join(append([]string{exe, "DATA"}, parts...)...)
	if exists(release) && !isEphemeralExecutableDir(exe) {
		return release
	}

	for _, start := range searchStarts() {
		root := checkoutRoot(start)
		if root == "" {
			continue
		}
		origen := filepath.Join(append([]string{root, "ORIGEN", "IC_SDR"}, parts...)...)
		if exists(origen) {
			return absOrSelf(origen)
		}
		dev := filepath.Join(append([]string{root, "DATA"}, parts...)...)
		if exists(dev) {
			return absOrSelf(dev)
		}
	}
	return release
}

func searchStarts() []string {
	starts := []string{workingDir(), executableDir()}
	if _, file, _, ok := runtime.Caller(1); ok {
		starts = append(starts, filepath.Dir(file))
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		starts = append(starts, filepath.Dir(file))
	}
	return uniqueStrings(starts)
}

func isEphemeralExecutableDir(dir string) bool {
	cleaned := strings.ToLower(filepath.ToSlash(dir))
	cleaned = strings.ReplaceAll(cleaned, `\`, "/")
	return strings.Contains(cleaned, "/go-build")
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, value := range in {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func checkoutRoot(start string) string {
	for dir := start; dir != ""; dir = filepath.Dir(dir) {
		if exists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}

func absOrSelf(path string) string {
	absolute, err := filepath.Abs(path)
	if err == nil {
		return absolute
	}
	return path
}

func executableDir() string {
	executable, err := os.Executable()
	if err == nil {
		if absolute, absErr := filepath.Abs(executable); absErr == nil {
			return filepath.Dir(absolute)
		}
	}
	return workingDir()
}

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// SDRRuntimeRoot is the portable SoapySDR tree for this OS/architecture.
// Windows prefers DATA/runtime/windows-<arch> and falls back to windows-x64.
func SDRRuntimeRoot() string {
	if runtime.GOOS == "windows" {
		preferred := Path("runtime", "windows-"+runtime.GOARCH)
		fallback := Path("runtime", "windows-x64")
		if exists(preferred) {
			return preferred
		}
		if preferred != fallback && exists(fallback) {
			return fallback
		}
		return preferred
	}
	return Path("runtime", runtime.GOOS+"-"+runtime.GOARCH)
}
