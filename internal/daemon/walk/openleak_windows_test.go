//go:build windows

package walk

// openleak_windows_test.go — goroutine-leak and concurrency tests for the
// Windows os.Open fallback path in gitignore_windows.go.
//
// The Unix tests (openleak_test.go) use POSIX FIFOs to simulate a blocked
// open(2). On Windows we can't do that, so we exercise the happy path and
// the semaphore-under-concurrency path instead.
//
// The key invariant we test: concurrent calls to ParseIgnoreFile do NOT
// accumulate goroutines. The 64-slot openSlotSem semaphore must ensure
// that goroutine count stays bounded even under parallel load.

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestOpenWithDeadline_Windows_RegularFileFastPath confirms that ParseIgnoreFile
// correctly reads a regular .gitignore on Windows using the os.Open fallback.
func TestOpenWithDeadline_Windows_RegularFileFastPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".gitignore")
	body := "node_modules/\n*.log\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ig, err := ParseIgnoreFile("", p, ".gitignore")
	if err != nil {
		t.Fatalf("ParseIgnoreFile: %v", err)
	}
	if ig == nil || len(ig.patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %#v", ig)
	}
}

// TestOpenWithDeadline_Windows_NoGoroutineLeak confirms that repeated
// ParseIgnoreFile calls (on a real regular file) do not accumulate goroutines.
// This mirrors TestOpenWithDeadline_NoGoroutineLeak from openleak_test.go but
// uses a real file instead of a FIFO.
func TestOpenWithDeadline_Windows_NoGoroutineLeak(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(p, []byte("dist/\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const iterations = 200
	for i := 0; i < iterations; i++ {
		ig, err := ParseIgnoreFile("", p, ".gitignore")
		if err != nil {
			t.Fatalf("iter %d: unexpected error: %v", i, err)
		}
		if ig == nil {
			t.Fatalf("iter %d: expected non-nil IgnoreFile", i)
		}
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	const slack = 5
	if after > baseline+slack {
		t.Fatalf("goroutine leak: baseline=%d after=%d iterations=%d slack=%d",
			baseline, after, iterations, slack)
	}
	t.Logf("baseline=%d after=%d iterations=%d (no leak)", baseline, after, iterations)
}

// TestOpenWithDeadline_Windows_NoLeakUnderConcurrency hammers ParseIgnoreFile
// from multiple goroutines simultaneously and verifies the goroutine count
// does not grow without bound.
func TestOpenWithDeadline_Windows_NoLeakUnderConcurrency(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(p, []byte("vendor/\n__pycache__/\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const workers = 16
	const perWorker = 50
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				_, _ = ParseIgnoreFile("", p, ".gitignore")
			}
		}()
	}
	wg.Wait()

	runtime.GC()
	time.Sleep(150 * time.Millisecond)
	after := runtime.NumGoroutine()

	const slack = 10
	if after > baseline+slack {
		t.Fatalf("goroutine leak under concurrency: baseline=%d after=%d total_calls=%d",
			baseline, after, workers*perWorker)
	}
	t.Logf("baseline=%d after=%d total_calls=%d (no leak)", baseline, after, workers*perWorker)
}

// TestOpenWithDeadline_Windows_SemaphoreSaturation confirms that when all
// semaphore slots are held, ParseIgnoreFile returns ErrIgnoreFileTimeout
// immediately rather than blocking or leaking an additional goroutine.
func TestOpenWithDeadline_Windows_SemaphoreSaturation(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(p, []byte("dist/\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Saturate the semaphore manually.
	for i := 0; i < cap(openSlotSem); i++ {
		openSlotSem <- struct{}{}
	}
	defer func() {
		// Drain the semaphore after the test.
		for i := 0; i < cap(openSlotSem); i++ {
			<-openSlotSem
		}
	}()

	ig, err := ParseIgnoreFile("", p, ".gitignore")
	if err != ErrIgnoreFileTimeout {
		t.Fatalf("expected ErrIgnoreFileTimeout when semaphore saturated, got err=%v ig=%v", err, ig)
	}
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile on semaphore saturation")
	}
	t.Log("semaphore saturation correctly returns ErrIgnoreFileTimeout")
}
