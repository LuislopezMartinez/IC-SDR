//go:build !windows

package tetra

import "github.com/ebitengine/purego"

func openCodecLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
}

func codecSymbol(lib uintptr, name string) error {
	_, err := purego.Dlsym(lib, name)
	return err
}
