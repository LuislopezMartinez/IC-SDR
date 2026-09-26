//go:build windows

package omnirig

import (
	"strings"
	"syscall"
	"unsafe"
)

const (
	swRestore       = 9
	wmNull          = 0
	wmUser          = 0x0400
	smtoAbortIfHung = 0x0002
)

type omniRigWindow struct {
	handle    uintptr
	className string
	title     string
	hung      bool
}

var user32 = syscall.NewLazyDLL("user32.dll")
var (
	enumWindowsProc         = user32.NewProc("EnumWindows")
	getWindowTextLengthProc = user32.NewProc("GetWindowTextLengthW")
	getWindowTextProc       = user32.NewProc("GetWindowTextW")
	getClassNameProc        = user32.NewProc("GetClassNameW")
	isHungAppWindowProc     = user32.NewProc("IsHungAppWindow")
	showWindowProc          = user32.NewProc("ShowWindowAsync")
	setForegroundProc       = user32.NewProc("SetForegroundWindow")
	bringWindowToTopProc    = user32.NewProc("BringWindowToTop")
	postMessageProc         = user32.NewProc("PostMessageW")
	sendMessageTimeoutProc  = user32.NewProc("SendMessageTimeoutW")
)

func requestOmniRigWindow() bool {
	if restoreOmniRigWindow() {
		return true
	}
	for _, window := range enumerateOmniRigWindows() {
		if isOmniRigApplicationClass(window.className) && isOmniRigWindowTitle(window.title) && !window.hung {
			result, _, _ := postMessageProc.Call(window.handle, wmUser, 73, 88)
			return result != 0
		}
	}
	return false
}

func restoreOmniRigWindow() bool {
	for _, window := range enumerateOmniRigWindows() {
		if !isOmniRigDialogClass(window.className) || !isOmniRigWindowTitle(window.title) || window.hung {
			continue
		}
		_, _, _ = showWindowProc.Call(window.handle, swRestore)
		_, _, _ = bringWindowToTopProc.Call(window.handle)
		_, _, _ = setForegroundProc.Call(window.handle)
		return windowResponds(window.handle)
	}
	return false
}

func enumerateOmniRigWindows() []omniRigWindow {
	windows := make([]omniRigWindow, 0, 2)
	callback := syscall.NewCallback(func(window uintptr, _ uintptr) uintptr {
		classBuffer := make([]uint16, 128)
		classLength, _, _ := getClassNameProc.Call(window, uintptr(unsafe.Pointer(&classBuffer[0])), uintptr(len(classBuffer)))
		if classLength == 0 {
			return 1
		}
		className := syscall.UTF16ToString(classBuffer)
		if !isOmniRigDialogClass(className) && !isOmniRigApplicationClass(className) {
			return 1
		}
		length, _, _ := getWindowTextLengthProc.Call(window)
		buffer := make([]uint16, int(length)+1)
		_, _, _ = getWindowTextProc.Call(window, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		hung, _, _ := isHungAppWindowProc.Call(window)
		windows = append(windows, omniRigWindow{handle: window, className: className, title: syscall.UTF16ToString(buffer), hung: hung != 0})
		return 1
	})
	_, _, _ = enumWindowsProc.Call(callback, 0)
	return windows
}

func windowResponds(window uintptr) bool {
	var result uintptr
	ok, _, _ := sendMessageTimeoutProc.Call(window, wmNull, 0, 0, smtoAbortIfHung, 300, uintptr(unsafe.Pointer(&result)))
	return ok != 0
}

func isOmniRigWindowTitle(title string) bool {
	title = strings.ToLower(strings.TrimSpace(title))
	return strings.Contains(title, "omni-rig") || strings.Contains(title, "omnirig")
}

func isOmniRigDialogClass(className string) bool {
	return strings.EqualFold(strings.TrimSpace(className), "TMainForm")
}

func isOmniRigApplicationClass(className string) bool {
	return strings.EqualFold(strings.TrimSpace(className), "TApplication")
}
