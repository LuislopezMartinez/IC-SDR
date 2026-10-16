//go:build !windows

package ft8

import "syscall"

func hiddenProcessAttributes() *syscall.SysProcAttr { return nil }
