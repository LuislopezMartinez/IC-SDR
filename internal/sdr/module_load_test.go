package sdr

import "testing"

func TestModuleLoadSucceeded(t *testing.T) {
	path := "C:/runtime/rtlsdrSupport.dll"
	for _, message := range []string{"", path + " already loaded"} {
		if !moduleLoadSucceeded(message, path) {
			t.Fatalf("rejected %q", message)
		}
	}
	for _, message := range []string{"missing dependency", "another.dll already loaded"} {
		if moduleLoadSucceeded(message, path) {
			t.Fatalf("accepted %q", message)
		}
	}
}
