//go:build windows

package process

import (
	"golang.org/x/sys/windows"
)

// stillActive is the exit code reported by GetExitCodeProcess for a
// process that has not yet terminated (Win32 STILL_ACTIVE == 259).
const stillActive = 259

// IsAlive reports whether a process with the given pid is currently
// running on Windows.
//
// os.FindProcess always succeeds on Windows and Process.Signal is
// unsupported there, so the unix signal-0 probe always reports the wrong
// answer. Instead we OpenProcess with PROCESS_QUERY_LIMITED_INFORMATION
// (the least-privileged handle that still answers liveness, available
// even for processes in other sessions) and inspect the exit code:
// STILL_ACTIVE (259) means running. A failed open with
// ERROR_INVALID_PARAMETER means the pid does not map to any process —
// dead.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// ERROR_INVALID_PARAMETER: no process with this pid -> dead.
		// ERROR_ACCESS_DENIED: the process exists but we may not open
		// it -> alive (the kernel resolved the pid to deny us).
		if err == windows.ERROR_ACCESS_DENIED {
			return true
		}
		return false
	}
	defer windows.CloseHandle(h)

	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
