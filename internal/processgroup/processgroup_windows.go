//go:build windows

// Package processgroup keeps every helper launched by IC-SDR tied to the
// lifetime of the main application process.
package processgroup

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createBreakawayFromJob = 0x01000000

var (
	job     windows.Handle
	jobOnce sync.Once
	jobErr  error
)

// Initialize places IC-SDR in a kill-on-close Windows Job. Processes started
// by IC-SDR inherit membership in this job, so Windows terminates all helpers
// when the main process exits, even after a crash or forced termination.
func Initialize() error {
	jobOnce.Do(func() {
		job, jobErr = newKillOnCloseJob()
		if jobErr != nil {
			return
		}
		err := assignPID(job, uint32(os.Getpid()))
		if err != nil {
			_ = windows.CloseHandle(job)
			job = 0
			jobErr = fmt.Errorf("asociar IC-SDR al grupo de procesos: %w", err)
		}
	})
	return jobErr
}

func newKillOnCloseJob() (windows.Handle, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("crear grupo de procesos: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
	if _, err = windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(handle)
		return 0, fmt.Errorf("configurar grupo de procesos: %w", err)
	}
	return handle, nil
}

func assignPID(handle windows.Handle, pid uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	return windows.AssignProcessToJobObject(handle, process)
}

// ConfigureIndependent marks a command as independent from IC-SDR's Job.
// This is reserved for the updater, which must remain alive after IC-SDR exits.
func ConfigureIndependent(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= createBreakawayFromJob
}
