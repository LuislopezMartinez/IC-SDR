//go:build windows

package dab

import (
	"os/exec"
	"testing"
	"time"
)

func TestAttachedProcessIsTerminatedWhenLifetimeCloses(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping 127.0.0.1 -n 30 >nul")
	configureHiddenProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	release, err := attachProcessLifetime(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	release()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child process survived closing its kill-on-close job")
	}
}
