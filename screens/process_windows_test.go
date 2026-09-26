//go:build windows

package screens

import "testing"

func TestHiddenChildProcessDoesNotOpenConsoleWindow(t *testing.T) {
	attributes := hiddenChildProcessAttributes()
	if attributes == nil || !attributes.HideWindow {
		t.Fatal("child process window is not hidden")
	}
	if attributes.CreationFlags&0x08000000 == 0 {
		t.Fatal("CREATE_NO_WINDOW is not enabled")
	}
}
