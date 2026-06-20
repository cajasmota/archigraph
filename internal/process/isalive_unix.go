//go:build !windows

package process

import (
	"os"
	"syscall"
)

// IsAlive reports whether a process with the given pid is currently
// running. On unix (darwin/linux) it uses signal 0 — the POSIX existence
// probe — which validates the pid without delivering a signal.
//
// A pid that exists but is owned by another user returns EPERM from the
// probe; we treat that as alive because the process is demonstrably
// present (the kernel had to look it up to deny us). Only ESRCH (no such
// process) means dead.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists but we may not signal it: still alive.
	return err == syscall.EPERM
}
