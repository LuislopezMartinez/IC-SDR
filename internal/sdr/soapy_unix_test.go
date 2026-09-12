//go:build !windows

package sdr

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUnixSoapyPrefersRuntimeRoot(t *testing.T) {
	root := filepath.FromSlash("/opt/ic-sdr-runtime")
	got := unixSoapyCoreCandidates(Config{RuntimeRoot: root, Driver: "rtlsdr"})
	if len(got) == 0 {
		t.Fatal("no SoapySDR candidates")
	}
	want := filepath.Join(root, "lib", unixSoapyCoreNames()[0])
	if got[0] != want {
		t.Fatalf("first candidate %q, want bundled %q", got[0], want)
	}
}

func TestUnixRTLDependenciesPreferRuntimeRoot(t *testing.T) {
	root := filepath.FromSlash("/opt/ic-sdr-runtime")
	got := unixRTLDependencies(root)
	if len(got) == 0 || !strings.HasPrefix(got[0], root) {
		t.Fatalf("first RTL dependency %q, want under %q", got, root)
	}
}
