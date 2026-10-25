//go:build !windows

package processgroup

import "os/exec"

func Initialize() error              { return nil }
func ConfigureIndependent(*exec.Cmd) {}
