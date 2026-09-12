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

func TestUnixRadioDependenciesPreferRuntimeRoot(t *testing.T) {
	root := filepath.FromSlash("/opt/ic-sdr-runtime")
	got := unixRadioDependencies(root)
	if len(got) == 0 || !strings.HasPrefix(got[0], root) {
		t.Fatalf("first radio dependency %q, want under %q", got, root)
	}
	joined := strings.Join(got, "\n")
	for _, name := range []string{"librtlsdr", "libhackrf", "libairspy"} {
		if !strings.Contains(joined, name) {
			t.Fatalf("missing %s in %v", name, got)
		}
	}
}

func TestUnixSoapyModuleDirsIncludeRuntimeRoot(t *testing.T) {
	root := filepath.FromSlash("/opt/ic-sdr-runtime")
	got := unixSoapyModuleDirs(root)
	want := filepath.Join(root, "lib", "SoapySDR", "modules0.8")
	if len(got) == 0 || got[0] != want {
		t.Fatalf("first module dir %q, want %q", got, want)
	}
}
