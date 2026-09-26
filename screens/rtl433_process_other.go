//go:build !windows

package screens

import "syscall"

func rtl433ViewerProcessAttributes() *syscall.SysProcAttr { return nil }
func hiddenChildProcessAttributes() *syscall.SysProcAttr  { return nil }
func focusRTL433Viewer(int)                               {}
