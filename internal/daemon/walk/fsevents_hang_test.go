//go:build darwin || linux

// fsevents_hang_test.go — #1721 regression coverage.
//
// We simulate the macOS fsevents kernel stall by pointing ParseIgnoreFile and
// isLinguistGeneratedDir at a POSIX FIFO (named pipe). A FIFO's read-side
// open(2) blocks until a writer connects — the writer never connects here, so
// without the deadline fix both callers would block indefinitely. After the
// fix they must return within the test's timeout budget.
//
// Build-constrained to darwin||linux: syscall.Mkfifo is a POSIX-only syscall
// and does not exist on Windows.
package walk

import (
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// TestParseIgnoreFile_TimesOutOnStuckOpen confirms that ParseIgnoreFile does
// not hang when the underlying open(2) blocks indefinitely (FIFO, no writer).
// The deadline fix must surface ErrIgnoreFileTimeout and return an empty (no-op)
// IgnoreFile within the test budget.
func TestParseIgnoreFile_TimesOutOnStuckOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO test requires POSIX mkfifo; skipping on Windows")
	}
	dir := t.TempDir()
	fifoPath := filepath.Join(dir, ".gitignore")
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	type result struct {
		ig  *IgnoreFile
		err error
	}
	ch := make(chan result, 1)
	start := time.Now()
	go func() {
		ig, err := ParseIgnoreFile("", fifoPath, ".gitignore")
		ch <- result{ig: ig, err: err}
	}()

	const budget = 7 * time.Second // 5s deadline + 2s headroom
	select {
	case got := <-ch:
		elapsed := time.Since(start)
		if elapsed > budget {
			t.Fatalf("ParseIgnoreFile returned but took too long: %v (expected < %v)", elapsed, budget)
		}
		if got.err != ErrIgnoreFileTimeout {
			t.Errorf("expected ErrIgnoreFileTimeout, got: %v", got.err)
		}
		if got.ig == nil {
			t.Fatal("expected non-nil IgnoreFile on timeout")
		}
		if len(got.ig.patterns) != 0 {
			t.Errorf("expected empty patterns on timeout, got %d", len(got.ig.patterns))
		}
		t.Logf("ParseIgnoreFile returned ErrIgnoreFileTimeout after %v", elapsed)
	case <-time.After(budget + 2*time.Second):
		t.Fatalf("ParseIgnoreFile did not return within %v — fix regression", budget+2*time.Second)
	}
}

// TestIsLinguistGeneratedDir_TimesOutOnStuckOpen confirms that
// isLinguistGeneratedDir does not hang when .gitattributes open(2) blocks.
// On timeout it must return false (conservative safe default) within budget.
func TestIsLinguistGeneratedDir_TimesOutOnStuckOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFO test requires POSIX mkfifo; skipping on Windows")
	}
	dir := t.TempDir()
	fifoPath := filepath.Join(dir, ".gitattributes")
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	type result struct{ got bool }
	ch := make(chan result, 1)
	start := time.Now()
	go func() {
		ch <- result{got: isLinguistGeneratedDir(dir)}
	}()

	const budget = 7 * time.Second
	select {
	case got := <-ch:
		elapsed := time.Since(start)
		if elapsed > budget {
			t.Fatalf("isLinguistGeneratedDir returned but took too long: %v", elapsed)
		}
		if got.got {
			t.Error("expected false (safe default) on timeout, got true")
		}
		t.Logf("isLinguistGeneratedDir returned false after %v", elapsed)
	case <-time.After(budget + 2*time.Second):
		t.Fatalf("isLinguistGeneratedDir did not return within %v — fix regression", budget+2*time.Second)
	}
}
