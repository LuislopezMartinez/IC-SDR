//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	linked := captureLinkedProcesses(*pid, *target)
	defer closeLinkedProcesses(linked)
	if err := waitForProcess(*pid, 90*time.Second); err != nil {
		fail(*target, err)
	}
	terminateLinkedProcesses(linked, 5*time.Second)
	err := update.Apply(update.InstallOptions{
		Target: *target, Source: *source, Work: *work, RestartExe: *restart,
	})
	if err != nil {
		fail(*target, err)
	}
}

type linkedProcess struct {
	handle windows.Handle
}

// captureLinkedProcesses is the compatibility bridge for versions released
// before the application-wide Job Object existed. It records descendants of
// IC-SDR that execute from the portable installation while the parent PID is
// still alive. The updater itself runs from %TEMP% and is therefore excluded.
func captureLinkedProcesses(rootPID int, target string) []linkedProcess {
	if rootPID <= 0 || target == "" {
		return nil
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return nil
	}
	target = strings.TrimSuffix(target, string(filepath.Separator)) + string(filepath.Separator)
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if windows.Process32First(snapshot, &entry) != nil {
		return nil
	}
	parents := make(map[uint32]uint32)
	for {
		parents[entry.ProcessID] = entry.ParentProcessID
		if windows.Process32Next(snapshot, &entry) != nil {
			break
		}
	}
	var linked []linkedProcess
	for processID := range parents {
		if processID == uint32(rootPID) || !isDescendant(processID, uint32(rootPID), parents) {
			continue
		}
		handle, openErr := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, processID)
		if openErr != nil {
			continue
		}
		buffer := make([]uint16, 32768)
		size := uint32(len(buffer))
		if windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size) != nil {
			_ = windows.CloseHandle(handle)
			continue
		}
		path := windows.UTF16ToString(buffer[:size])
		if !strings.HasPrefix(strings.ToLower(path), strings.ToLower(target)) {
			_ = windows.CloseHandle(handle)
			continue
		}
		linked = append(linked, linkedProcess{handle: handle})
	}
	return linked
}

func isDescendant(processID, rootPID uint32, parents map[uint32]uint32) bool {
	visited := make(map[uint32]bool)
	for processID != 0 && !visited[processID] {
		if processID == rootPID {
			return true
		}
		visited[processID] = true
		processID = parents[processID]
	}
	return false
}

func terminateLinkedProcesses(processes []linkedProcess, timeout time.Duration) {
	for _, process := range processes {
		if result, _ := windows.WaitForSingleObject(process.handle, 0); result == uint32(windows.WAIT_TIMEOUT) {
			_ = windows.TerminateProcess(process.handle, 1)
		}
	}
	deadline := time.Now().Add(timeout)
	for _, process := range processes {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		_, _ = windows.WaitForSingleObject(process.handle, uint32(remaining/time.Millisecond))
	}
}

func closeLinkedProcesses(processes []linkedProcess) {
	for _, process := range processes {
		_ = windows.CloseHandle(process.handle)
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
