//go:build !windows

package locale

import "os"

func systemTag() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(key); value != "" && value != "C" {
			return value
		}
	}
	return ""
}
