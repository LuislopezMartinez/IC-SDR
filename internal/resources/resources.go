// Package resources locates files shipped with IC-SDR independently of the
// process working directory. Release builds place them in DATA beside the exe;
// development builds use the checkout's DATA directory.
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
		for dir := start; dir != ""; dir = filepath.Dir(dir) {
			if exists(filepath.Join(dir, "go.mod")) && exists(filepath.Join(dir, "DATA")) {
				return filepath.Join(dir, "DATA")
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return release
}

// Path returns an absolute path for a resource relative to DATA.
// The returned release path is useful in error messages even when it is absent.
func Path(parts ...string) string {
	base := executableDir()
	release := filepath.Join(append([]string{base, "DATA"}, parts...)...)
	if exists(release) {
		return release
	}

	for _, start := range []string{workingDir(), base} {
		for dir := start; dir != ""; dir = filepath.Dir(dir) {
			if exists(filepath.Join(dir, "go.mod")) {
				candidate := filepath.Join(append([]string{dir, "DATA"}, parts...)...)
				if exists(candidate) {
					return candidate
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return release
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
