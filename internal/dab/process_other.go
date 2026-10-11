//go:build !windows

package dab

import "os/exec"

func configureHiddenProcess(*exec.Cmd) {}

func attachProcessLifetime(*exec.Cmd) (func(), error) { return func() {}, nil }
