//go:build !windows

package main

// isWindowsFileInUseErrno is a no-op on non-Windows platforms: the
// sharing-violation errno semantics it checks are Windows-specific. On Unix an
// open file can be unlinked, so this path is never the cause of a remove
// failure.
func isWindowsFileInUseErrno(err error) bool { return false }
