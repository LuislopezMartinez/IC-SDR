//go:build !windows

package feedback

import (
	"errors"
	"runtime"
)

type Window struct{}

func NewWindow(string) *Window { return &Window{} }
func (*Window) Open(TechnicalInfo) error {
	return errors.New("la ventana de reportes requiere Windows")
}
func (*Window) Close()                      {}
func PlatformDescription() (string, string) { return runtime.GOOS, runtime.GOARCH }
