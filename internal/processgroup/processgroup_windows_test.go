//go:build windows

package processgroup

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestKillOnCloseJobTerminatesHelper(t *testing.T) {
	command := exec.Command("cmd.exe", "/c", "ping 127.0.0.1 -n 30 >nul")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	job, err := newKillOnCloseJob()
	if err != nil {
		_ = command.Process.Kill()
		t.Fatal(err)
	}
	if err = assignPID(job, uint32(command.Process.Pid)); err != nil {
		_ = windows.CloseHandle(job)
		_ = command.Process.Kill()
		t.Fatal(err)
	}
	// The primary thread remains suspended, avoiding a race in which the test
	// helper could finish before assignment. Closing the only Job handle must
	// still terminate it.
	if err = windows.CloseHandle(job); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("the helper survived closing its kill-on-close Job")
	}
}

func TestConfigureIndependentPreservesProcessAttributes(t *testing.T) {
	command := exec.Command("cmd.exe")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	ConfigureIndependent(command)
	if !command.SysProcAttr.HideWindow {
		t.Fatal("ConfigureIndependent discarded existing process attributes")
	}
	if command.SysProcAttr.CreationFlags&createBreakawayFromJob == 0 {
		t.Fatal("the updater was not configured to break away from IC-SDR's Job")
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("ConfigureIndependent discarded existing creation flags")
	}
}
