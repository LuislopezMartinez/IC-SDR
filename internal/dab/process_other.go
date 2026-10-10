//go:build !windows

package dab

import "os/exec"

func configureHiddenProcess(*exec.Cmd) {}
