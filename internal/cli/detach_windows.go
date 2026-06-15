//go:build windows

package cli

import "syscall"

// detachSysProcAttr returns a SysProcAttr that detaches the daemon on
// Windows. Phase A doesn't ship Windows service install, but the
// `grafel start` command still needs to spawn a background daemon
// the CLI can disown.
func detachSysProcAttr() *syscall.SysProcAttr {
	const detachedProcess = 0x00000008
	const createNewProcessGroup = 0x00000200
	return &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
}
