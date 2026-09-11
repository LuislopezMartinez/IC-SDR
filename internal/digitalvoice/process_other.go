//go:build !windows

package digitalvoice

import "os/exec"

func configureHiddenProcess(*exec.Cmd) {}
