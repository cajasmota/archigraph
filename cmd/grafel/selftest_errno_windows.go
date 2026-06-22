//go:build windows

package main

import (
	"errors"
	"syscall"
)

// Windows error numbers (winerror.h). These numeric codes are locale-invariant,
// unlike the human-readable message strings.
const (
	errorAccessDenied     syscall.Errno = 5  // ERROR_ACCESS_DENIED
	errorSharingViolation syscall.Errno = 32 // ERROR_SHARING_VIOLATION
	errorLockViolation    syscall.Errno = 33 // ERROR_LOCK_VIOLATION
)

// isWindowsFileInUseErrno reports whether err unwraps to one of the Windows
// "file is in use" / access-denied error numbers. It compares numeric
// syscall.Errno values rather than localized message text.
func isWindowsFileInUseErrno(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case errorAccessDenied, errorSharingViolation, errorLockViolation:
			return true
		}
	}
	return false
}
