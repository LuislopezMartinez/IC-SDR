//go:build windows

package update

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processesRunningFrom reports processes whose executable is inside the
// portable installation. It deliberately does not terminate them: the Job
// Object owns IC-SDR helpers, while unrelated lockers must remain untouched.
func processesRunningFrom(root string) string {
	root, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	root = strings.TrimSuffix(root, string(filepath.Separator)) + string(filepath.Separator)
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err = windows.Process32First(snapshot, &entry); err != nil {
		return ""
	}
	var found []string
	for {
		process, openErr := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if openErr == nil {
			buffer := make([]uint16, 32768)
			size := uint32(len(buffer))
			if windows.QueryFullProcessImageName(process, 0, &buffer[0], &size) == nil {
				path := windows.UTF16ToString(buffer[:size])
				if strings.HasPrefix(strings.ToLower(path), strings.ToLower(root)) {
					found = append(found, fmt.Sprintf("%s, PID %d", filepath.Base(path), entry.ProcessID))
				}
			}
			_ = windows.CloseHandle(process)
		}
		if err = windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return strings.Join(found, "; ")
}
