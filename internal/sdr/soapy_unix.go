//go:build !windows

package sdr

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ebitengine/purego"
)

func loadShared(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

func closeShared(handle uintptr) {
	if handle != 0 {
		_ = purego.Dlclose(handle)
	}
}

func loadSoapy(config Config) (*soapyAPI, error) {
	root := ""
	if config.RuntimeRoot != "" {
		if absolute, err := filepath.Abs(config.RuntimeRoot); err == nil {
			root = absolute
		} else {
			root = config.RuntimeRoot
		}
	}

	bundled := bundledSoapyCore(root)
	if bundled != "" {
		api, err := loadBundledSoapy(config, root, bundled)
		if err == nil {
			return api, nil
		}
		config.trace("SoapySDR: bundled runtime rejected: %v", err)
	}

	var failures []error
	for _, corePath := range unixSoapyCoreCandidates(config) {
		if bundled != "" && corePath == bundled {
			continue
		}
		deps := preloadLibraries(config, unixRadioDependencies(root))
		deps = append(deps, preloadLibraries(config, unixSDRplayAPICandidates())...)
		config.trace("SoapySDR: dlopen %s", corePath)
		core, err := loadShared(corePath)
		if err != nil {
			for index := len(deps) - 1; index >= 0; index-- {
				closeShared(deps[index])
			}
			failures = append(failures, fmt.Errorf("load %s: %w", corePath, err))
			continue
		}
		api := registerSoapy(core)
		api.dependencies = deps
		api.loadModules(config, collectSoapyModules(unixSoapyModuleDirs(root)...))
		config.trace("SoapySDR: using %s", corePath)
		return api, nil
	}
	if len(failures) == 0 {
		failures = append(failures, fmt.Errorf("libSoapySDR not found (bundled runtime missing under %s)", root))
	}
	return nil, fmt.Errorf("SoapySDR backend: %w", errors.Join(failures...))
}

func loadBundledSoapy(config Config, root, corePath string) (*soapyAPI, error) {
	pluginDir := filepath.Join(root, "lib", "SoapySDR", "modules0.8")
	_ = os.Setenv("SOAPY_SDR_ROOT", root)
	_ = os.Setenv("SOAPY_SDR_PLUGIN_PATH", pluginDir)
	libDir := filepath.Join(root, "lib")
	if runtime.GOOS == "darwin" {
		_ = os.Setenv("DYLD_LIBRARY_PATH", libDir+string(os.PathListSeparator)+os.Getenv("DYLD_LIBRARY_PATH"))
	} else {
		_ = os.Setenv("LD_LIBRARY_PATH", libDir+string(os.PathListSeparator)+os.Getenv("LD_LIBRARY_PATH"))
	}
	deps := preloadLibraries(config, unixRadioDependencies(root))
	deps = append(deps, preloadLibraries(config, unixSDRplayAPICandidates())...)
	config.trace("SoapySDR: loading bundled %s", corePath)
	core, err := loadShared(corePath)
	if err != nil {
		for index := len(deps) - 1; index >= 0; index-- {
			closeShared(deps[index])
		}
		return nil, fmt.Errorf("load bundled %s: %w", corePath, err)
	}
	api := registerSoapy(core)
	api.dependencies = deps
	api.loadModules(config, collectSoapyModules(pluginDir, filepath.Join(root, "lib64", "SoapySDR", "modules0.8")))
	return api, nil
}

func bundledSoapyCore(root string) string {
	if root == "" {
		return ""
	}
	for _, name := range unixSoapyCoreNames() {
		for _, dir := range []string{"lib", "bin"} {
			path := filepath.Join(root, dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func unixSoapyCoreNames() []string {
	if runtime.GOOS == "darwin" {
		return []string{"libSoapySDR.dylib", "libSoapySDR.0.8.dylib", "libSoapySDR.0.dylib"}
	}
	return []string{"libSoapySDR.so.0.8", "libSoapySDR.so.0", "libSoapySDR.so"}
}

func unixSoapyCoreCandidates(config Config) []string {
	root := config.RuntimeRoot
	names := unixSoapyCoreNames()
	var out []string
	if root != "" {
		for _, name := range names {
			out = append(out, filepath.Join(root, "lib", name), filepath.Join(root, "bin", name))
		}
	}
	if homebrew := unixHomebrewPrefix(); homebrew != "" {
		out = append(out, filepath.Join(homebrew, "lib", "libSoapySDR.dylib"))
	}
	out = append(out, names...)
	return uniqueStrings(out)
}

func unixSoapyModuleDirs(root string) []string {
	var dirs []string
	if root != "" {
		dirs = append(dirs,
			filepath.Join(root, "lib", "SoapySDR", "modules0.8"),
			filepath.Join(root, "lib64", "SoapySDR", "modules0.8"),
		)
	}
	if homebrew := unixHomebrewPrefix(); homebrew != "" {
		dirs = append(dirs, filepath.Join(homebrew, "lib", "SoapySDR", "modules0.8"))
	}
	dirs = append(dirs,
		"/usr/lib/x86_64-linux-gnu/SoapySDR/modules0.8",
		"/usr/lib/aarch64-linux-gnu/SoapySDR/modules0.8",
		"/usr/lib/arm-linux-gnueabihf/SoapySDR/modules0.8",
		"/usr/local/lib/SoapySDR/modules0.8",
		"/usr/lib/SoapySDR/modules0.8",
	)
	return uniqueStrings(dirs)
}

func unixRadioDependencies(root string) []string {
	names := []string{
		"libusb-1.0.so.0", "libusb-1.0.so", "libusb-1.0.0.dylib", "libusb-1.0.dylib",
		"librtlsdr.so.0", "librtlsdr.so", "librtlsdr.0.dylib", "librtlsdr.dylib",
		"libhackrf.so.0", "libhackrf.so", "libhackrf.0.dylib", "libhackrf.dylib",
		"libairspy.so.0", "libairspy.so", "libairspy.0.dylib", "libairspy.dylib",
		"libairspyhf.so.0", "libairspyhf.so", "libairspyhf.0.dylib", "libairspyhf.dylib",
	}
	var out []string
	if root != "" {
		for _, name := range names {
			out = append(out, filepath.Join(root, "lib", name), filepath.Join(root, "bin", name))
		}
	}
	if homebrew := unixHomebrewPrefix(); homebrew != "" {
		for _, name := range []string{"libusb-1.0.dylib", "librtlsdr.dylib", "libhackrf.dylib", "libairspy.dylib", "libairspyhf.dylib"} {
			out = append(out, filepath.Join(homebrew, "lib", name))
		}
	}
	out = append(out, names...)
	return uniqueStrings(out)
}

func unixSDRplayAPICandidates() []string {
	var out []string
	if homebrew := unixHomebrewPrefix(); homebrew != "" {
		out = append(out, filepath.Join(homebrew, "lib", "libsdrplay_api.dylib"))
	}
	out = append(out,
		"/usr/local/lib/libsdrplay_api.so",
		"/usr/lib/libsdrplay_api.so",
		"/usr/lib/x86_64-linux-gnu/libsdrplay_api.so",
		"/usr/lib/aarch64-linux-gnu/libsdrplay_api.so",
		"/opt/sdrplay/lib/libsdrplay_api.so",
		"libsdrplay_api.so",
		"libsdrplay_api.dylib",
	)
	return uniqueStrings(out)
}

func unixHomebrewPrefix() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
		if _, err := os.Stat(filepath.Join(prefix, "lib")); err == nil {
			return prefix
		}
	}
	return ""
}
