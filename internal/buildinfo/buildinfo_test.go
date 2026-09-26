package buildinfo

import "testing"

func TestDisplayVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })
	for _, test := range []struct{ input, want string }{
		{"dev", "desarrollo"}, {"", "desarrollo"}, {"0.9.3", "v0.9.3"}, {"v1.2.0", "v1.2.0"},
	} {
		Version = test.input
		if got := DisplayVersion(); got != test.want {
			t.Errorf("DisplayVersion(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestWindowTitleIncludesVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })
	Version = "1.2.3"
	if got := WindowTitle(); got != "IC-SDR · v1.2.3" {
		t.Fatalf("WindowTitle() = %q", got)
	}
}
