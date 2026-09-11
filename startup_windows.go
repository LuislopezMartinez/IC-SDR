//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func showStartupError(message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	text, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("IC-SDR - Startup error")
	messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
