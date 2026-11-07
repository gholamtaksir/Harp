// Package privilege detects and acquires the administrator rights that TUN
// mode needs.
package privilege

import (
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"golang.org/x/sys/windows"
)

// Hint tells the user how to obtain the required privileges.
const Hint = "run Harp from an elevated (Administrator) terminal"

// CanRelaunch reports whether RelaunchElevated is supported.
const CanRelaunch = true

// IsElevated reports whether the process runs with administrator rights.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// RelaunchElevated starts the current executable again with administrator
// rights, showing the UAC prompt. The caller should exit afterwards.
func RelaunchElevated(args []string) error {
	executable, err := os.Executable()
	if err != nil {
		return errors.WithStack(err)
	}
	workingDirectory, _ := os.Getwd()
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(executable)
	parameters, _ := syscall.UTF16PtrFromString(joinArgs(args))
	directory, _ := syscall.UTF16PtrFromString(workingDirectory)
	if err := windows.ShellExecute(0, verb, file, parameters, directory, windows.SW_NORMAL); err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return errors.New("the administrator prompt was cancelled")
		}
		return errors.Wrap(err, "start elevated process")
	}
	return nil
}

// WaitForExit waits until the process with the given id exits or the
// timeout elapses.
func WaitForExit(pid int, timeout time.Duration) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)
	_, _ = windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
}

// joinArgs quotes arguments for the Windows command line.
func joinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = syscall.EscapeArg(arg)
	}
	return strings.Join(quoted, " ")
}
