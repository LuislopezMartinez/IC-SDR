//go:build !windows

package datv

import "os/exec"

func configureHiddenProcess(*exec.Cmd)                {}
func attachProcessLifetime(*exec.Cmd) (func(), error) { return func() {}, nil }
