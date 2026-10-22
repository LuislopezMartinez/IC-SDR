//go:build !windows

package feedback

import (
	"errors"
	"runtime"
)

type Window struct{}

func NewWindow() *Window { return &Window{} }
func (*Window) Open(TechnicalInfo) error {
	return errors.New("no se pudo abrir la página de reportes en GitHub")
}
func (*Window) Close()                      {}
func PlatformDescription() (string, string) { return runtime.GOOS, runtime.GOARCH }
