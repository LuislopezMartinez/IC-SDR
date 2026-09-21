package tetrapolruntime

import (
	"os/exec"
	"testing"
)

func TestExternalRuntimeUsesHiddenWindow(t *testing.T) {
	command := exec.Command("unused.exe")
	applyProcessWindowPolicy(command)
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow {
		t.Fatal("external runtime window is not hidden")
	}
	if command.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatal("CREATE_NO_WINDOW is not enabled")
	}
}
