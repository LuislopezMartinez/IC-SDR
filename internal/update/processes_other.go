//go:build !windows

package update

func processesRunningFrom(string) string { return "" }
