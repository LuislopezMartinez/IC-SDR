//go:build !windows

package pocsag

import (
	"os/exec"
	"syscall"
)

func hiddenProcessAttributes() *syscall.SysProcAttr   { return nil }
func attachProcessLifetime(*exec.Cmd) (func(), error) { return func() {}, nil }
