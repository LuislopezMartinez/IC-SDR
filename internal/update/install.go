package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var mutableDataDirectories = []string{"cache", "captures", "config", "exports", "logs", "recordings"}

type InstallOptions struct {
	Target     string
	Source     string
	Work       string
	RestartExe string
}

// Apply replaces a portable installation only after every path has been
// validated. The old installation remains available for rollback until the
// replacement has been installed and its executable has started.
func Apply(options InstallOptions) error {
	target, source, work, err := validateInstallOptions(options)
	if err != nil {
		return err
	}
	backup := filepath.Join(filepath.Dir(target), fmt.Sprintf(".ic-sdr-backup-%d", time.Now().UnixNano()))
	if err := renameWithRetry(target, backup, 60*time.Second); err != nil {
		return fmt.Errorf("IC-SDR sigue utilizando archivos de la instalación: %w", err)
	}
	installed := false
	defer func() {
		if !installed {
			_ = restoreMutableData(target, backup)
			_ = os.RemoveAll(target)
			_ = os.Rename(backup, target)
		}
	}()
	if err := os.Rename(source, target); err != nil {
		return fmt.Errorf("no se pudo colocar la nueva versión: %w", err)
	}
	if err := restoreMutableData(backup, target); err != nil {
		return fmt.Errorf("no se pudieron conservar los datos del usuario: %w", err)
	}
	restartPath := filepath.Join(target, options.RestartExe)
	if !regularFile(restartPath) {
		return fmt.Errorf("la nueva versión no contiene %s", options.RestartExe)
	}
	command := exec.Command(restartPath)
	command.Dir = target
	if err := command.Start(); err != nil {
		return fmt.Errorf("no se pudo reiniciar IC-SDR: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	select {
	case err := <-exited:
		if err == nil {
			err = fmt.Errorf("la nueva versión se cerró durante el arranque")
		}
		return fmt.Errorf("la nueva versión no ha superado la comprobación de arranque: %w", err)
	case <-time.After(8 * time.Second):
	}
	installed = true
	_ = os.RemoveAll(backup)
	_ = os.RemoveAll(work)
	return nil
}

func validateInstallOptions(options InstallOptions) (target, source, work string, err error) {
	if options.RestartExe != "IC-SDR-Go.exe" {
		return "", "", "", fmt.Errorf("ejecutable de reinicio no válido")
	}
	target, err = filepath.Abs(options.Target)
	if err != nil {
		return "", "", "", err
	}
	source, err = filepath.Abs(options.Source)
	if err != nil {
		return "", "", "", err
	}
	work, err = filepath.Abs(options.Work)
	if err != nil {
		return "", "", "", err
	}
	parent := filepath.Dir(target)
	if filepath.Dir(work) != parent || !strings.HasPrefix(filepath.Base(work), ".ic-sdr-update-") {
		return "", "", "", fmt.Errorf("carpeta temporal no válida")
	}
	if !within(work, source) || source == work {
		return "", "", "", fmt.Errorf("origen de actualización no válido")
	}
	if target == filepath.VolumeName(target)+string(filepath.Separator) || target == parent {
		return "", "", "", fmt.Errorf("destino de actualización no seguro")
	}
	if !regularFile(filepath.Join(target, "IC-SDR-Go.exe")) || !regularFile(filepath.Join(source, "IC-SDR-Go.exe")) {
		return "", "", "", fmt.Errorf("instalación portable incompleta")
	}
	return target, source, work, nil
}

func restoreMutableData(backup, target string) error {
	for _, name := range mutableDataDirectories {
		oldPath := filepath.Join(backup, "DATA", name)
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		newPath := filepath.Join(target, "DATA", name)
		if err := os.RemoveAll(newPath); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
			return err
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	}
	return nil
}

func renameWithRetry(source, destination string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if err := os.Rename(source, destination); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(250 * time.Millisecond)
	}
}
