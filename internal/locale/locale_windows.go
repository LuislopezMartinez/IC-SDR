//go:build windows

package locale

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	getUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
)

func systemTag() string {
	buf := make([]uint16, 85)
	r, _, _ := getUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
