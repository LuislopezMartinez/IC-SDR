package main

import (
	"runtime"

	"go-zero/internal/resources"
)

func toolExecutable(tool, name string) string {
	file := name
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	return resources.Path("tools", tool, "runtime", "bin", file)
}

func sdrRuntimeRoot() string {
	if runtime.GOOS == "windows" {
		return resources.Path("runtime", "windows-x64")
	}
	return resources.Path("runtime", runtime.GOOS+"-"+runtime.GOARCH)
}

func tetraCodecPath() string {
	if runtime.GOOS == "windows" {
		return resources.Path("tools", "tetra", "runtime", "bin", "libtetradec.dll")
	}
	return resources.Path("tools", "tetra", "runtime", "bin", "libtetradec.so")
}
