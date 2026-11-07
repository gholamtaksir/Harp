//go:build !windows

// Package privilege detects and acquires the administrator rights that TUN
// mode needs.
package privilege

import (
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
)

// Hint tells the user how to obtain the required privileges.
var Hint = func() string {
	if runtime.GOOS == "linux" {
		return "run Harp with sudo, or grant it CAP_NET_ADMIN with: sudo setcap cap_net_admin,cap_net_bind_service+ep <path to harp>"
	}
	return "run Harp with sudo"
}()

// CanRelaunch reports whether RelaunchElevated is supported.
const CanRelaunch = false

// IsElevated reports whether the process runs as root. On Linux,
// CAP_NET_ADMIN is also sufficient for TUN mode, so a false result is only a
// hint there.
func IsElevated() bool {
	return os.Geteuid() == 0
}

// RelaunchElevated is not supported outside Windows.
func RelaunchElevated(args []string) error {
	return errors.New("restarting with elevated privileges is only supported on Windows; " + Hint)
}

// WaitForExit waits until the process with the given id exits or the
// timeout elapses.
func WaitForExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
