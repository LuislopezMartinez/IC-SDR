//go:build windows

package omnirig

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	omniRigProgID = "OmniRig.OmniRigX"
	omniRigCLSID  = "{0839E8C6-ED30-4950-8087-966F970F0CAE}"
)

// registerPortableServer publishes the bundled out-of-process COM server only
// for the current user. HKCU\Software\Classes takes precedence over HKLM and
// requires no elevation. It is only called after normal COM activation fails,
// so a working system installation is never replaced.
func registerPortableServer(executable string) error {
	if executable == "" {
		return fmt.Errorf("la copia portable no está configurada")
	}
	abs, err := filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("ruta portable no válida: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return fmt.Errorf("no se encuentra OmniRig.exe portable en %s", abs)
	}

	values := []struct {
		path, name, value string
	}{
		{`Software\Classes\` + omniRigProgID, "", "OmniRigX Object"},
		{`Software\Classes\` + omniRigProgID + `\CLSID`, "", omniRigCLSID},
		{`Software\Classes\CLSID\` + omniRigCLSID, "", "OmniRigX Object"},
		{`Software\Classes\CLSID\` + omniRigCLSID + `\ProgID`, "", omniRigProgID},
		{`Software\Classes\CLSID\` + omniRigCLSID + `\LocalServer32`, "", `"` + abs + `"`},
		{`Software\Classes\CLSID\` + omniRigCLSID + `\LocalServer32`, "ServerExecutable", abs},
		{`Software\IC-SDR\OmniRig`, "Executable", abs},
	}
	for _, item := range values {
		key, _, createErr := registry.CreateKey(registry.CURRENT_USER, item.path, registry.SET_VALUE)
		if createErr != nil {
			return fmt.Errorf("no se pudo registrar Omni-Rig para este usuario: %w", createErr)
		}
		setErr := key.SetStringValue(item.name, item.value)
		_ = key.Close()
		if setErr != nil {
			return fmt.Errorf("no se pudo completar el registro de Omni-Rig: %w", setErr)
		}
	}
	return nil
}
