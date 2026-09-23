//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"go-zero/internal/update"

	"golang.org/x/sys/windows"
)

func main() {
	target := flag.String("target", "", "portable installation directory")
	source := flag.String("source", "", "validated update directory")
	work := flag.String("work", "", "update working directory")
	restart := flag.String("restart", "IC-SDR-Go.exe", "application executable")
	pid := flag.Int("pid", 0, "application process ID")
	flag.Parse()

	if err := waitForProcess(*pid, 90*time.Second); err != nil {
		fail(*target, err)
	}
	err := update.Apply(update.InstallOptions{
		Target: *target, Source: *source, Work: *work, RestartExe: *restart,
	})
	if err != nil {
		fail(*target, err)
	}
}

func waitForProcess(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("PID de IC-SDR no válido")
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// The process may already have completed before the updater opens it.
		if err == windows.ERROR_INVALID_PARAMETER {
			return nil
		}
		return fmt.Errorf("no se pudo esperar al cierre de IC-SDR: %w", err)
	}
	defer windows.CloseHandle(handle)
	result, err := windows.WaitForSingleObject(handle, uint32(timeout/time.Millisecond))
	if err != nil {
		return err
	}
	if result == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("IC-SDR no se cerró a tiempo")
	}
	return nil
}

func fail(target string, err error) {
	message := "La actualización no se pudo completar.\n\n" + err.Error()
	if target != "" {
		logPath := filepath.Join(filepath.Dir(target), "IC-SDR-update-error.txt")
		_ = os.WriteFile(logPath, []byte(message+"\n"), 0o644)
	}
	showError(message)
	os.Exit(1)
}

func showError(message string) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	text, _ := windows.UTF16PtrFromString(message)
	title, _ := windows.UTF16PtrFromString("IC-SDR Updater")
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
