//go:build windows

package vlcembed

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestEmbeddedPlayerStartsWhenVLCIsInstalled(t *testing.T) {
	if ResolveLibrary() == "" {
		t.Skip("libVLC is not installed")
	}
	class, _ := windows.UTF16PtrFromString("STATIC")
	parent, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0x80000000, 0, 0, 320, 240, 0, 0, 0, 0)
	if parent == 0 {
		t.Skipf("cannot create test window: %v", err)
	}
	defer procDestroyWindow.Call(parent)
	p := New()
	if err := p.Start(unsafe.Pointer(parent), 0, 0, 320, 240, "udp://@127.0.0.1:19549"); err != nil {
		t.Fatal(err)
	}
	if !p.Running() {
		t.Fatal("libVLC did not enter running state")
	}
	p.Close()
}
