//go:build windows

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessesRunningFromFindsPortableHelper(t *testing.T) {
	root := t.TempDir()
	helper := filepath.Join(root, "ic-sdr-test-helper.exe")
	data, err := os.ReadFile(filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(helper, data, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(helper, "/c", "ping 127.0.0.1 -n 30 >nul")
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		detail := processesRunningFrom(root)
		if strings.Contains(detail, "ic-sdr-test-helper.exe") && strings.Contains(detail, "PID") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("running helper was not reported; got %q", detail)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestMoveChildrenWorksWhileRootDirectoryIsInUse(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "IC-SDR-Go")
	lockedDirectory := filepath.Join(target, "DATA")
	backup := filepath.Join(root, "backup")
	if err := os.MkdirAll(lockedDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockedDirectory, "settings.json"), []byte("settings"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("ping.exe", "127.0.0.1", "-n", "30")
	command.Dir = lockedDirectory
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()
	time.Sleep(100 * time.Millisecond)

	if err := os.Rename(target, backup); err == nil {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			t.Fatal(restoreErr)
		}
		t.Skip("Windows allowed renaming a process working directory")
	}
	if err := moveChildren(target, backup); err != nil {
		t.Fatalf("content fallback failed while root was in use: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backup, "DATA", "settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := moveChildren(backup, target); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceDirectoryContentsWhileDestinationIsInUse(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "backup", "DATA", "exports")
	destination := filepath.Join(root, "target", "DATA", "exports")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "user.csv"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "packaged.csv"), []byte("default"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("ping.exe", "127.0.0.1", "-n", "30")
	command.Dir = destination
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()
	time.Sleep(100 * time.Millisecond)

	if err := replaceDirectoryContents(source, destination); err != nil {
		t.Fatalf("mutable data restore failed while destination was in use: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "user.csv"))
	if err != nil || string(data) != "user" {
		t.Fatalf("restored export = %q, %v", data, err)
	}
}
