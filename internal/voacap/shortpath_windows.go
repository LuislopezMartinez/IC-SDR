//go:build windows

package voacap

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func shortEnginePath(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	longPath, err := windows.UTF16PtrFromString(absolute)
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, windows.MAX_PATH)
	length, err := windows.GetShortPathName(longPath, &buffer[0], uint32(len(buffer)))
	if err != nil || length == 0 || int(length) >= len(buffer) {
		return "", errors.New("VOACAP NO PUEDE RESOLVER LA RUTA PORTABLE")
	}
	return windows.UTF16ToString(buffer[:length]), nil
}
