//go:build windows

package feedback

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Window struct{}

var (
	feedbackShell32      = windows.NewLazySystemDLL("shell32.dll")
	feedbackShellExecute = feedbackShell32.NewProc("ShellExecuteW")
)

func NewWindow() *Window { return &Window{} }

func PlatformDescription() (string, string) {
	version := windows.RtlGetVersion()
	name := "Windows"
	if version.MajorVersion >= 10 && version.BuildNumber >= 22000 {
		name = "Windows 11"
	} else if version.MajorVersion >= 10 {
		name = "Windows 10"
	}
	return name + " · compilación " + fmt.Sprint(version.BuildNumber), runtime.GOARCH
}

// Open delegates the HTTPS address to Windows so the user's default browser is
// used and GitHub can reuse their existing authenticated session.
func (*Window) Open(info TechnicalInfo) error {
	operation, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	address, err := windows.UTF16PtrFromString(IssueURL(info))
	if err != nil {
		return err
	}
	result, _, callErr := feedbackShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(address)),
		0,
		0,
		1, // SW_SHOWNORMAL
	)
	if result <= 32 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("no se pudo abrir GitHub: %w", callErr)
		}
		return errors.New("Windows no pudo abrir la página de reportes en GitHub")
	}
	return nil
}

func (*Window) Close() {}
