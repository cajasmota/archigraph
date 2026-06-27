package extractors_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cajasmota/grafel/internal/extractors"
	"github.com/cajasmota/grafel/internal/indexer/diff"
)

// TestIncremental_FallbackRefreshesStampsAndReconciles is the #5668 regression
// test for the residual reindex-loop gap.
//
// When the incremental change-detector hits the too-many-changed fallback, it
// must persist a FULLY RECONCILED manifest — UpdateManifest refreshes the
// content stamps of the surviving files AND prunes entries absent from the
// gitignore-aware walk. 0.1.7.2 saved only the GC'd map, which left STALE
// STAMPS in place: on the next pass those files are re-reported as "changed",
// re-trip the fallback, and the daemon loops a full reindex forever (the bug
// behind 0.1.7.1/0.1.7.2 that needed a manual clean-manifest rebuild to clear).
//
// The test seeds a manifest whose on-disk files carry WRONG stamps so they all
// read as changed and trip the fallback (limit=2). After one pass the persisted
// manifest must have correct stamps, and a second pass must NOT fall back.
// Against the pre-fix code the second pass still falls back (stamps never
// refreshed) and this test fails.
func TestIncremental_FallbackRefreshesStampsAndReconciles(t *testing.T) {
	t.Setenv("GRAFEL_INCREMENTAL_MAX_FILES", "2") // fall back when >2 read as changed

	repo := t.TempDir()
	stateDir := t.TempDir()

	files := []string{"a.go", "b.go", "c.go", "d.go", "e.go"}
	for i, rel := range files {
		writeFile(t, repo, rel, fmt.Sprintf("package p\n\nfunc F%d() {}\n", i))
	}

	// Seed a manifest with deliberately WRONG stamps for every on-disk file, so
	// the change-detector reports all 5 as changed (5 > limit 2 → fallback).
	m := diff.LoadManifest(stateDir)
	for _, rel := range files {
		m.Files[rel] = diff.FileEntry{SHA256: "0000000000000000", Size: 1, Mtime: 1}
	}
	if err := diff.SaveManifest(stateDir, repo, m); err != nil {
		t.Fatalf("save seeded manifest: %v", err)
	}

	// Pass 1: must fall back (too-many-changed).
	res := extractors.TryIncremental(context.Background(), repo, stateDir, nil, nil)
	if res.Done {
		t.Fatalf("pass 1: expected too-many-changed fallback, got Done=true")
	}

	// The fix: the persisted manifest must now carry REFRESHED stamps (not the
	// "0000…" sentinel) for the surviving files.
	reloaded := diff.LoadManifest(stateDir)
	e, ok := reloaded.Files["a.go"]
	if !ok {
		t.Fatalf("a.go missing from reconciled manifest")
	}
	if e.SHA256 == "0000000000000000" || e.SHA256 == "" {
		t.Fatalf("pass 1 did not reconcile: a.go still has the stale stamp %q — stamps were not refreshed on the fallback (the loop bug)", e.SHA256)
	}

	// Pass 2: stamps are correct now → nothing reads as changed → must NOT fall
	// back. Against the pre-fix code this still loops (Done=false).
	res2 := extractors.TryIncremental(context.Background(), repo, stateDir, nil, nil)
	if !res2.Done {
		t.Fatalf("pass 2: loop NOT broken — second pass still fell back (reason=%q); a reconciled manifest must not re-trip too-many-changed", res2.FallbackReason)
	}
}
