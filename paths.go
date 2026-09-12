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
	return resources.SDRRuntimeRoot()
}

func tetraCodecName() string {
	switch runtime.GOOS {
	case "windows":
		return "libtetradec.dll"
	case "darwin":
		return "libtetradec.dylib"
	default:
		return "libtetradec.so"
	}
}

func tetraCodecPath() string {
	return resources.Path("tools", "tetra", "runtime", "bin", tetraCodecName())
}
