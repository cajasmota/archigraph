//go:build windows

package walk

// gitignore_windows.go — plain os.Open fallback for openWithDeadline on Windows.
//
// Windows does not have the fsevents / kqueue kernel-stall behaviour that
// motivated the O_NONBLOCK + fcntl dance in gitignore_unix.go (#1729).
// Plain os.Open is sufficient; the 5s context deadline in ParseIgnoreFile
// remains the only safety net, which is fine on Windows.
//
// The 64-slot openSlotSem semaphore is still honoured here to bound the
// maximum number of concurrently-outstanding open goroutines (callers of
// ParseIgnoreFile expect the same concurrency-control contract regardless of
// platform).

import (
	"os"
	"time"
)

// openWithDeadline opens path and returns the file or an error.
//
// Windows implementation: plain os.Open inside a semaphore-bounded goroutine.
// There is no O_NONBLOCK dance because Windows does not suffer the macOS
// fsevents kernel-stall problem that motivated the Unix version.
func openWithDeadline(path string, timeout time.Duration) (*os.File, error) {
	// Step 1: lstat. Quick bail for non-existent or non-regular paths.
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, ErrIgnoreFileTimeout
	}

	// Step 2: semaphore — bail early if already saturated.
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
		f, oerr := os.Open(path)
		if oerr != nil {
			ch <- result{err: oerr}
		} else {
			ch <- result{f: f}
		}
		<-openSlotSem
	}()

	select {
	case r := <-ch:
		return r.f, r.err
	case <-time.After(timeout):
		return nil, ErrIgnoreFileTimeout
	}
}
