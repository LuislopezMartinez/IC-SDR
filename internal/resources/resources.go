// Package resources locates files shipped with IC-SDR independently of the
// process working directory. Release builds place them in DATA beside the exe;
// development builds fall back to ORIGEN/IC_SDR, then the checkout DATA folder.
package resources

import (
	"os"
	"path/filepath"
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
	base := executableDir()
	release := filepath.Join(base, "DATA")
	if exists(release) {
		return release
	}
	for _, start := range []string{workingDir(), base} {
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
	base := executableDir()
	release := filepath.Join(append([]string{base, "DATA"}, parts...)...)
	if exists(release) {
		return release
	}

	for _, start := range []string{workingDir(), base} {
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
