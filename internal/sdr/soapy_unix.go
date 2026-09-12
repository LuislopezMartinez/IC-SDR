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
		config.trace("SoapySDR/%s: bundled runtime rejected: %v", config.Driver, err)
	}

	var failures []error
	for _, corePath := range unixSoapyCoreCandidates(config) {
		if bundled != "" && corePath == bundled {
			continue
		}
		config.trace("SoapySDR/%s: dlopen %s", config.Driver, corePath)
		core, err := loadShared(corePath)
		if err != nil {
			failures = append(failures, fmt.Errorf("load %s: %w", corePath, err))
			continue
		}
		api := registerSoapy(core)
		for _, dep := range unixRTLDependencies(root) {
			if handle, depErr := loadShared(dep); depErr == nil {
				api.dependencies = append(api.dependencies, handle)
			}
		}
		for _, modulePath := range unixSoapyModuleCandidates(config) {
			if _, statErr := os.Stat(modulePath); statErr != nil {
				continue
			}
			config.trace("SoapySDR/%s: loading module %s", config.Driver, modulePath)
			message := api.consume(api.loadModule(modulePath))
			if message == "" {
				config.trace("SoapySDR/%s: module loaded (%s)", config.Driver, modulePath)
				return api, nil
			}
			failures = append(failures, fmt.Errorf("module %s: %s", modulePath, message))
		}
		config.trace("SoapySDR/%s: using library without an explicit module path", config.Driver)
		return api, nil
	}
	if len(failures) == 0 {
		failures = append(failures, fmt.Errorf("libSoapySDR not found (bundled runtime missing under %s)", root))
	}
	return nil, fmt.Errorf("SoapySDR backend: %w", errors.Join(failures...))
}

func loadBundledSoapy(config Config, root, corePath string) (*soapyAPI, error) {
	_ = os.Setenv("SOAPY_SDR_ROOT", root)
	_ = os.Setenv("SOAPY_SDR_PLUGIN_PATH", filepath.Join(root, "lib", "SoapySDR", "modules0.8"))
	config.trace("SoapySDR/%s: loading bundled %s", config.Driver, corePath)
	for _, dep := range unixRTLDependencies(root) {
		if _, err := os.Stat(dep); err != nil {
			continue
		}
		if _, err := loadShared(dep); err != nil {
			return nil, fmt.Errorf("load bundled dependency %s: %w", dep, err)
		}
	}
	core, err := loadShared(corePath)
	if err != nil {
		return nil, fmt.Errorf("load bundled %s: %w", corePath, err)
	}
	api := registerSoapy(core)
	var moduleErrs []error
	for _, modulePath := range unixSoapyModuleCandidates(config) {
		if _, err := os.Stat(modulePath); err != nil {
			continue
		}
		message := api.consume(api.loadModule(modulePath))
		if message == "" {
			config.trace("SoapySDR/%s: bundled module loaded (%s)", config.Driver, modulePath)
			return api, nil
		}
		moduleErrs = append(moduleErrs, fmt.Errorf("%s: %s", modulePath, message))
	}
	if len(moduleErrs) > 0 {
		api.close()
		return nil, errors.Join(moduleErrs...)
	}
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

func unixSoapyModuleCandidates(config Config) []string {
	root := config.RuntimeRoot
	moduleNames := unixSoapyModuleNames(config.Driver)
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
		"/usr/local/lib/SoapySDR/modules0.8",
	)
	var out []string
	for _, dir := range dirs {
		for _, name := range moduleNames {
			out = append(out, filepath.Join(dir, name))
		}
	}
	out = append(out, moduleNames...)
	return uniqueStrings(out)
}

func unixSoapyModuleNames(driver string) []string {
	base := "rtlsdrSupport"
	if driver == "sdrplay" {
		base = "sdrPlaySupport"
	}
	return []string{
		"lib" + base + ".so",
		base + ".so",
		"lib" + base + ".dylib",
		base + ".dylib",
	}
}

func unixRTLDependencies(root string) []string {
	names := []string{
		"libusb-1.0.so.0", "libusb-1.0.so", "libusb-1.0.0.dylib", "libusb-1.0.dylib",
		"librtlsdr.so.0", "librtlsdr.so", "librtlsdr.0.dylib", "librtlsdr.dylib",
	}
	var out []string
	if root != "" {
		for _, name := range names {
			out = append(out, filepath.Join(root, "lib", name), filepath.Join(root, "bin", name))
		}
	}
	if homebrew := unixHomebrewPrefix(); homebrew != "" {
		out = append(out,
			filepath.Join(homebrew, "lib", "libusb-1.0.dylib"),
			filepath.Join(homebrew, "lib", "librtlsdr.dylib"),
		)
	}
	out = append(out, names...)
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
