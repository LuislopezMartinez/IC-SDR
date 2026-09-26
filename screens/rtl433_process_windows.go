//go:build windows

package screens

import (
	"syscall"
	"unsafe"
)

func rtl433ViewerProcessAttributes() *syscall.SysProcAttr {
	return hiddenChildProcessAttributes()
}

func hiddenChildProcessAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

func focusRTL433Viewer(pid int) {
	user32 := syscall.NewLazyDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	getPID := user32.NewProc("GetWindowThreadProcessId")
	show := user32.NewProc("ShowWindowAsync")
	foreground := user32.NewProc("SetForegroundWindow")
	callback := syscall.NewCallback(func(window uintptr, _ uintptr) uintptr {
		var windowPID uint32
		_, _, _ = getPID.Call(window, uintptr(unsafe.Pointer(&windowPID)))
		if int(windowPID) == pid {
			_, _, _ = show.Call(window, 9)
			_, _, _ = foreground.Call(window)
			return 0
		}
		return 1
	})
	_, _, _ = enumWindows.Call(callback, 0)
}
