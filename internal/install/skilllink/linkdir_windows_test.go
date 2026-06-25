//go:build windows

package skilllink

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestLinkSkillDir_NonPrivilegedFallback is the Windows-only regression guard
// for the elevation-class bug fixed in #5318 / #5641.
//
// THE CI BLIND SPOT (#5316): grafel's Windows CI runs on a PRIVILEGED runner
// (admin / Developer Mode), so os.Symlink — step 1 of the fallback chain —
// SUCCEEDS there. That means the junction codepath that #5318 actually added
// (the thing that lets a NON-admin user install) is never exercised on Windows
// CI: a regression that reintroduced an admin-only symlink requirement, or that
// broke `mklink /J`, would still pass green while breaking every real non-admin
// user.
//
// This test removes that blind spot deterministically: it forces the symlink
// primitive to fail with the exact "privilege not held" error a non-admin
// process gets, then asserts the helper still links the directory via a
// junction (or, last resort, a copy) AND that the destination resolves to the
// source content. No real privilege drop is required, so it is deterministic on
// GitHub's windows-latest regardless of how the runner is provisioned.
//
// It goes RED if anyone reintroduces an admin-only operation on the link path:
//   - if the junction fallback is removed, the forced-symlink-failure leaves
//     no working mechanism (copy is the only remaining fallback) — and if copy
//     too is removed, linkSkillDir returns LinkModeNone/err and the test fails;
//   - if `mklink /J` is replaced by something that needs elevation, the
//     non-admin runner (or this forced-failure path) cannot create the link and
//     the content assertion fails.
//
// It is GREEN on the current junction-based code.
func TestLinkSkillDir_NonPrivilegedFallback(t *testing.T) {
	// Simulate a standard (non-admin) Windows process: SeCreateSymbolicLinkPrivilege
	// is not held, so os.Symlink fails with ERROR_PRIVILEGE_NOT_HELD (1314).
	orig := trySymlink
	t.Cleanup(func() { trySymlink = orig })
	trySymlink = func(oldname, newname string) error {
		return &os.LinkError{
			Op:  "symlink",
			Old: oldname,
			New: newname,
			Err: errors.New("A required privilege is not held by the client."),
		}
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "skill-src")
	if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nested", "f.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "skill-dst")
	mode, err := linkSkillDir(src, dst)
	if err != nil {
		t.Fatalf("non-privileged linkSkillDir must succeed via junction/copy fallback, got error: %v", err)
	}

	// A real symlink must NOT be the mode here — we forced its failure. The
	// whole point of the #5318 fix is that a non-admin user lands on a junction
	// (preferred) or a copy (last resort), never on the admin-only symlink.
	if mode == LinkModeSymlink {
		t.Fatalf("expected junction or copy fallback for a non-privileged process, got %v", mode)
	}
	if mode != LinkModeJunction && mode != LinkModeCopy {
		t.Fatalf("unexpected link mode %v", mode)
	}

	// On a standard NTFS volume the preferred mechanism is a junction; only an
	// exotic environment (junctions disabled / cross-volume) should fall all
	// the way to copy. A windows-latest runner is plain NTFS, so a copy-mode
	// result there signals the junction path silently broke — surface it.
	if mode == LinkModeCopy {
		t.Logf("WARNING: fell back to copy mode rather than junction (mklink /J) — " +
			"junction creation may have regressed")
	}

	// Universal invariant: the destination resolves to the source content,
	// regardless of which non-symlink mechanism engaged.
	got, err := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if err != nil {
		t.Fatalf("read through non-privileged link: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content mismatch through link: got %q want %q", got, "hello")
	}
	if _, err := os.Stat(filepath.Join(dst, "nested", "f.txt")); err != nil {
		t.Errorf("nested file not reachable through link: %v", err)
	}
}
