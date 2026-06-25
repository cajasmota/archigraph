package watch

// quarantine_recover_test.go — Q3 auto-recover-on-query/reference (#5618).
//
// These tests exercise QuarantineTracker.Recover deterministically (fake clock,
// injected tracker, temp repo for persistence). They verify the four contract
// points from #5618:
//   - a query/reference to a path UNDER a quarantined dir un-quarantines it;
//   - a query to a NON-quarantined path is a cheap no-op;
//   - a PINNED dir is never auto-recovered (operator override is respected);
//   - after recovery, continued churn RE-quarantines (anti-flap is "surface
//     then re-quarantine", not "stay recovered while thrashing").

import (
	"path/filepath"
	"testing"
)

// quarantineDir drives a dir into churn-quarantine and asserts it landed.
func quarantineDir(t *testing.T, q *QuarantineTracker, repo, rel string) {
	t.Helper()
	p := filepath.Join(repo, filepath.FromSlash(rel), "trash.o")
	pump(q, repo, p, 10) // threshold is 10 in newTestTracker
	for _, r := range q.List(repo) {
		if r.Rel == rel {
			return
		}
	}
	t.Fatalf("setup: expected %q to be quarantined, got %+v", rel, q.List(repo))
}

func TestRecover_QueryUnderQuarantinedDirRecovers(t *testing.T) {
	repo := t.TempDir()
	q, _ := newTestTracker(t)
	quarantineDir(t, q, repo, "app/build")

	// A query resolves an entity whose source file lives under the quarantined
	// dir — that dir is genuinely needed, so it must be recovered immediately.
	queried := filepath.Join(repo, "app", "build", "generated", "real.go")
	rel, recovered := q.Recover(repo, queried)
	if !recovered {
		t.Fatalf("Recover should un-quarantine the ancestor of a queried path")
	}
	if rel != "app/build" {
		t.Fatalf("recovered rel = %q, want app/build", rel)
	}
	if len(q.List(repo)) != 0 {
		t.Fatalf("dir should be gone from the quarantine set after recover: %+v", q.List(repo))
	}

	// Idempotent: a second query for the same (now-clean) path is a no-op.
	if _, again := q.Recover(repo, queried); again {
		t.Fatalf("second Recover on an already-clean path must be a no-op")
	}
}

func TestRecover_ExactDirPathRecovers(t *testing.T) {
	repo := t.TempDir()
	q, _ := newTestTracker(t)
	quarantineDir(t, q, repo, "gen")

	// A file directly in the quarantined dir (not a descendant) also recovers.
	rel, recovered := q.Recover(repo, filepath.Join(repo, "gen", "schema.ts"))
	if !recovered || rel != "gen" {
		t.Fatalf("Recover(file in quarantined dir) = (%q,%v), want (gen,true)", rel, recovered)
	}
}

func TestRecover_NonQuarantinedPathIsNoOp(t *testing.T) {
	repo := t.TempDir()
	q, _ := newTestTracker(t)
	quarantineDir(t, q, repo, "app/build")

	// A query for a path NOT under any quarantined dir changes nothing.
	rel, recovered := q.Recover(repo, filepath.Join(repo, "src", "service.go"))
	if recovered || rel != "" {
		t.Fatalf("Recover of a non-quarantined path must be a no-op, got (%q,%v)", rel, recovered)
	}
	// The unrelated quarantine is untouched.
	if got := q.List(repo); len(got) != 1 || got[0].Rel != "app/build" {
		t.Fatalf("unrelated quarantine must survive, got %+v", got)
	}
}

func TestRecover_PinnedDirIsNotAutoRecovered(t *testing.T) {
	repo := t.TempDir()
	q, _ := newTestTracker(t)
	quarantineDir(t, q, repo, "vendor/gen")

	// Operator pins the dir (Q2 override). Pin it in place.
	q.mu.Lock()
	reason := q.quarantined[repo]["vendor/gen"]
	reason.Pinned = true
	q.quarantined[repo]["vendor/gen"] = reason
	q.mu.Unlock()

	// A query under the pinned dir must NOT auto-recover it.
	rel, recovered := q.Recover(repo, filepath.Join(repo, "vendor", "gen", "x.go"))
	if recovered || rel != "" {
		t.Fatalf("pinned dir must not be auto-recovered, got (%q,%v)", rel, recovered)
	}
	if got := q.List(repo); len(got) != 1 || !got[0].Pinned {
		t.Fatalf("pinned dir must stay quarantined and pinned, got %+v", got)
	}
}

func TestRecover_ReQuarantinesIfChurnContinues(t *testing.T) {
	repo := t.TempDir()
	q, _ := newTestTracker(t)
	quarantineDir(t, q, repo, "build")

	// A query recovers it.
	if _, recovered := q.Recover(repo, filepath.Join(repo, "build", "real.go")); !recovered {
		t.Fatalf("expected recover")
	}
	if len(q.List(repo)) != 0 {
		t.Fatalf("expected recovered (empty) set")
	}

	// It is still a churning build dir: continued churn re-quarantines it. The
	// counter was reset on recover, so a fresh threshold's worth of events trips
	// it again — real-but-churning dirs surface, then re-quarantine.
	churnPath := filepath.Join(repo, "build", "out.o")
	if drops := pump(q, repo, churnPath, 9); drops != 0 {
		t.Fatalf("9 post-recovery events should not re-quarantine yet, got %d", drops)
	}
	if !q.Observe(repo, churnPath) {
		t.Fatalf("10th post-recovery event should re-quarantine the still-churning dir")
	}
	if got := q.List(repo); len(got) != 1 || got[0].Rel != "build" {
		t.Fatalf("expected build re-quarantined, got %+v", got)
	}
}

func TestRecover_PersistsAcrossReload(t *testing.T) {
	repo := t.TempDir()
	q, _ := newTestTracker(t)
	quarantineDir(t, q, repo, "dist")
	if _, recovered := q.Recover(repo, filepath.Join(repo, "dist", "app.js")); !recovered {
		t.Fatalf("expected recover")
	}

	// A fresh tracker reads the persisted set: the recovered dir must be gone.
	q2, _ := newTestTracker(t)
	if got := q2.List(repo); len(got) != 0 {
		t.Fatalf("recover must persist (empty set after reload), got %+v", got)
	}
}

func TestRecover_NilAndDisabledSafe(t *testing.T) {
	var nilq *QuarantineTracker
	if rel, ok := nilq.Recover("/repo", "/repo/x/y.go"); ok || rel != "" {
		t.Fatalf("nil receiver Recover must be a safe no-op, got (%q,%v)", rel, ok)
	}

	repo := t.TempDir()
	q, _ := newTestTracker(t)
	q.cfg.disabled = true
	if rel, ok := q.Recover(repo, filepath.Join(repo, "build", "x.o")); ok || rel != "" {
		t.Fatalf("disabled tracker Recover must be a no-op, got (%q,%v)", rel, ok)
	}
}

func TestRecover_PathOutsideRepoIsNoOp(t *testing.T) {
	repo := t.TempDir()
	q, _ := newTestTracker(t)
	quarantineDir(t, q, repo, "build")
	if rel, ok := q.Recover(repo, "/some/other/place/file.go"); ok || rel != "" {
		t.Fatalf("path outside repo must be a no-op, got (%q,%v)", rel, ok)
	}
}
