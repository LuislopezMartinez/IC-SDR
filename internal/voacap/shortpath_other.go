//go:build !windows

package voacap

func shortEnginePath(root string) (string, error) { return root, nil }
