//go:build windows

package digitalvoice

import (
	"os/exec"
	"syscall"
)

// configureHiddenProcess prevents the decoder backend from creating a visible
// console window. DSD-neo communicates exclusively through redirected pipes,
// so it does not need an interactive terminal.
func configureHiddenProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
