//go:build windows

package pocsag

import (
	"os/exec"
	"testing"
	"time"
)

func TestRuntimeJobTerminatesChildOnRelease(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping 127.0.0.1 -n 30 >nul")
	cmd.SysProcAttr = hiddenProcessAttributes()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	release, err := attachProcessLifetime(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatal(err)
	}
	release()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("multimon child survived closing its kill-on-close job")
	}
}
