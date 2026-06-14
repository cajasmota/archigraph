package watchreg

import (
	"errors"
	"os"
	"time"
)

// lockFile acquires an advisory lock on path via an atomic O_CREATE|O_EXCL
// create, returning an unlock closure that removes the lock file. This is the
// portable (no cgo, no x/sys) cross-process mutex used to serialize the
// registry read-modify-write between the standalone watchers and the daemon
// sweep.
//
// A stale lock (older than staleLockAge — left by a process that crashed
// mid-mutation) is forcibly reclaimed so the registry can never wedge
// permanently. The mutation it guards is sub-millisecond, so the timeout is
// generous relative to the work.
func lockFile(path string) (func(), error) {
	const (
		staleLockAge = 30 * time.Second
		maxWait      = 5 * time.Second
		pollEvery    = 5 * time.Millisecond
	)
	deadline := time.Now().Add(maxWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		// Lock held — reclaim it if it is stale, else wait.
		if fi, statErr := os.Stat(path); statErr == nil {
			if time.Since(fi.ModTime()) > staleLockAge {
				_ = os.Remove(path) // best-effort steal; loop retries the create.
				continue
			}
		}
		if time.Now().After(deadline) {
			// Last-resort steal so a hung holder cannot block the daemon forever.
			_ = os.Remove(path)
			continue
		}
		time.Sleep(pollEvery)
	}
}
