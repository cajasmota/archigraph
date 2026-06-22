//go:build darwin

package watchers

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// err5 returns an *exec.ExitError-like error whose ExitCode() is 5, matching
// the flaky launchctl "Bootstrap failed: 5: Input/output error" condition.
// We obtain a real ExitError by running a process that exits 5.
func err5(t *testing.T) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit 5").Run()
	if err == nil {
		t.Fatal("expected non-nil error from `exit 5`")
	}
	if !isLaunchctlErr5(err) {
		t.Fatalf("test scaffold produced wrong exit code: %v", err)
	}
	return err
}

// withFakeLaunchctl swaps launchctlRunner for the duration of the test.
func withFakeLaunchctl(t *testing.T, fn func(args ...string) ([]byte, error)) {
	t.Helper()
	orig := launchctlRunner
	origBackoff := bootstrapBackoff
	launchctlRunner = fn
	bootstrapBackoff = time.Millisecond // keep tests fast
	t.Cleanup(func() {
		launchctlRunner = orig
		bootstrapBackoff = origBackoff
	})
}

// writePlist writes a dummy plist for a unit into a temp HOME so Load's
// os.Stat(path) precondition passes.
func writePlist(t *testing.T, u Unit) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	path, err := UnitPath(u)
	if err != nil {
		t.Fatalf("UnitPath: %v", err)
	}
	if _, err := Write(u); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.HasSuffix(path, ".plist") {
		t.Fatalf("expected .plist path, got %s", path)
	}
}

// TestLoad_RetriesOnErr5 verifies that a bootstrap that fails with err 5 once
// (or twice) and then succeeds results in Load returning nil — i.e. the
// bootout→bootstrap pair is retried specifically on the flaky err 5.
func TestLoad_RetriesOnErr5(t *testing.T) {
	u := Unit{Group: "demo", Repo: filepath.Join(t.TempDir(), "core"), BinPath: "/bin/grafel"}
	writePlist(t, u)

	bootstraps := 0
	withFakeLaunchctl(t, func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "bootstrap" {
			bootstraps++
			if bootstraps < 2 {
				return []byte("Bootstrap failed: 5: Input/output error"), err5(t)
			}
			return nil, nil // succeed on the 2nd attempt
		}
		return nil, nil // bootout always "succeeds"
	})

	if err := (darwinLoader{}).Load(u); err != nil {
		t.Fatalf("Load should succeed after err-5 retry, got: %v", err)
	}
	if bootstraps != 2 {
		t.Fatalf("expected 2 bootstrap attempts, got %d", bootstraps)
	}
}

// TestLoad_GivesUpAfterRetriesOnErr5 verifies that a persistent err 5 is
// retried up to bootstrapRetries times and then surfaced as an error.
func TestLoad_GivesUpAfterRetriesOnErr5(t *testing.T) {
	u := Unit{Group: "demo", Repo: filepath.Join(t.TempDir(), "core"), BinPath: "/bin/grafel"}
	writePlist(t, u)

	bootstraps := 0
	withFakeLaunchctl(t, func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "bootstrap" {
			bootstraps++
			return []byte("Bootstrap failed: 5: Input/output error"), err5(t)
		}
		return nil, nil
	})

	if err := (darwinLoader{}).Load(u); err == nil {
		t.Fatal("Load should fail when err 5 never clears")
	}
	if bootstraps != bootstrapRetries {
		t.Fatalf("expected %d bootstrap attempts, got %d", bootstrapRetries, bootstraps)
	}
}

// TestLoad_DoesNotRetryNonErr5 verifies that a non-err-5 bootstrap failure is
// NOT retried — it is a real error surfaced immediately.
func TestLoad_DoesNotRetryNonErr5(t *testing.T) {
	u := Unit{Group: "demo", Repo: filepath.Join(t.TempDir(), "core"), BinPath: "/bin/grafel"}
	writePlist(t, u)

	bootstraps := 0
	withFakeLaunchctl(t, func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "bootstrap" {
			bootstraps++
			// exit 1, not 5 — a real error.
			return []byte("some other failure"), exec.Command("sh", "-c", "exit 1").Run()
		}
		return nil, nil
	})

	if err := (darwinLoader{}).Load(u); err == nil {
		t.Fatal("Load should fail on a non-err-5 failure")
	}
	if bootstraps != 1 {
		t.Fatalf("expected exactly 1 bootstrap attempt for non-err-5, got %d", bootstraps)
	}
}
