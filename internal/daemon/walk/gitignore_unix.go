//go:build darwin || linux

package walk

// gitignore_unix.go — non-blocking open(2) implementation of openWithDeadline.
//
// #1729 introduced O_NONBLOCK + fcntl(F_GETFL/F_SETFL) to defend against
// macOS fsevents kernel stalls. These syscalls are Unix-only; the Windows
// counterpart lives in gitignore_windows.go and uses plain os.Open (Windows
// has no fsevents stall problem).
//
// Ported to a platform split in response to #1781 (CI cross-platform gate):
// the original single-file implementation broke `GOOS=windows go vet` because
// syscall.SYS_FCNTL, syscall.F_GETFL, syscall.F_SETFL, and the Handle→int
// conversion are undefined on Windows.

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// openWithDeadline opens path and returns the file or an error.
//
// See the full design commentary in gitignore.go (ParseIgnoreFile).
// This implementation uses O_NONBLOCK + fcntl to defend against macOS
// fsevents kernel stalls (#1721, #1723, #1729).
func openWithDeadline(path string, timeout time.Duration) (*os.File, error) {
	// Step 1: lstat. If the path doesn't exist or isn't a regular file
	// we don't need to open it at all.
	fi, err := os.Lstat(path)
	if err != nil {
		// Preserve os.IsNotExist semantics for callers.
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		// Special files (FIFO, socket, device, symlink-to-nothing) cannot
		// be ignore files. Treat as inaccessible — same as a kernel stall.
		return nil, ErrIgnoreFileTimeout
	}

	// Step 2: try to acquire a worker slot without blocking. If we can't,
	// the daemon is already saturated with leaked workers — bail rather
	// than make it worse.
	select {
	case openSlotSem <- struct{}{}:
	default:
		return nil, ErrIgnoreFileTimeout
	}

	type result struct {
		f   *os.File
		err error
	}
	ch := make(chan result, 1)
	go func() {
		// Step 3: non-blocking open. POSIX guarantees open(2) with
		// O_NONBLOCK returns without blocking — this is the actual leak
		// fix for the macOS fsevents kernel-stall case.
		fd, oerr := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
		if oerr != nil {
			// EWOULDBLOCK / EAGAIN means "would block" — treat as stall.
			if errors.Is(oerr, syscall.EWOULDBLOCK) || errors.Is(oerr, syscall.EAGAIN) {
				ch <- result{err: ErrIgnoreFileTimeout}
				<-openSlotSem
				return
			}
			ch <- result{err: &os.PathError{Op: "open", Path: path, Err: oerr}}
			<-openSlotSem
			return
		}

		// Clear O_NONBLOCK so subsequent Read(2) calls behave normally on
		// the returned file. Failure here is non-fatal; regular files
		// don't actually need this cleared but we do it for hygiene.
		if flags, ferr := fcntlGetFl(int(fd)); ferr == nil && (flags&syscall.O_NONBLOCK) != 0 {
			_ = fcntlSetFl(int(fd), flags&^syscall.O_NONBLOCK)
		}

		f := os.NewFile(uintptr(fd), path)
		ch <- result{f: f}
		<-openSlotSem
	}()

	select {
	case r := <-ch:
		return r.f, r.err
	case <-time.After(timeout):
		// Defensive: open with O_NONBLOCK should never block, but if the
		// platform/kernel doesn't honour it we still cap total leaks via
		// the semaphore. The slot will be released by the worker when it
		// eventually unblocks.
		return nil, ErrIgnoreFileTimeout
	}
}

// fcntlGetFl wraps fcntl(fd, F_GETFL).
func fcntlGetFl(fd int) (int, error) {
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(syscall.F_GETFL), 0)
	if errno != 0 {
		return 0, errno
	}
	return int(flags), nil
}

// fcntlSetFl wraps fcntl(fd, F_SETFL, flags).
func fcntlSetFl(fd int, flags int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(syscall.F_SETFL), uintptr(flags))
	if errno != 0 {
		return errno
	}
	return nil
}
