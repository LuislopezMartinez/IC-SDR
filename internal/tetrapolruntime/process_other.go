//go:build !windows

package tetrapolruntime

import "os/exec"

func applyProcessWindowPolicy(_ *exec.Cmd) {}
