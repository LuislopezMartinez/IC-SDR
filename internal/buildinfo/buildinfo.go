// Package buildinfo exposes metadata injected by build-release.ps1.
package buildinfo

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	Version   = "dev"
	Revision  = "unknown"
	BuildDate = "unknown"
)

func DisplayVersion() string {
	version := strings.TrimSpace(Version)
	if version == "" || strings.EqualFold(version, "dev") {
		return "desarrollo"
	}
	if strings.HasPrefix(strings.ToLower(version), "v") {
		return version
	}
	return "v" + version
}

func WindowTitle() string { return "IC-SDR · " + DisplayVersion() }

func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

func Details() string {
	return fmt.Sprintf("IC-SDR\nVersión: %s\nCompilación: %s\nRevisión: %s\nSistema: %s/%s",
		DisplayVersion(), BuildDate, Revision, runtime.GOOS, runtime.GOARCH)
}
