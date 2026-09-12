//go:build windows

package tetra

import "syscall"

func openCodecLibrary(path string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(path)
	if err != nil {
		return 0, err
	}
	return uintptr(handle), nil
}

func codecSymbol(lib uintptr, name string) error {
	_, err := syscall.GetProcAddress(syscall.Handle(lib), name)
	return err
}
