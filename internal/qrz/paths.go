package qrz

import (
	"os"
	"path/filepath"
)

// UserDataPath keeps QRZ browser state outside the portable application DATA
// directory. In particular, this prevents authenticated browser cookies from
// being copied when an IC-SDR release folder is shared or backed up.
func UserDataPath(parts ...string) string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(append([]string{base, "IC-SDR", "QRZ"}, parts...)...)
}
