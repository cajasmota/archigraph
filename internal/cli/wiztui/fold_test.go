package wiztui

import (
	"testing"

	"github.com/cajasmota/grafel/internal/progress"
)

func ev(slug, phase string, ts int64) progress.Event {
	return progress.Event{RepoSlug: slug, Phase: phase, TS: ts}
}

// TestFold_MultipleReposManyRows is the dropped-repo regression: events for two
// repos must yield two rows, not one. This is the core bug #5340 fixes.
func TestFold_MultipleReposManyRows(t *testing.T) {
	rows := map[string]Row{}
	rows = Fold(rows, ev("backend", progress.PhaseScan, 1))
	rows = Fold(rows, ev("frontend", progress.PhaseScan, 2))
	rows = Fold(rows, ev("backend", progress.PhaseExtractAST, 3))
	rows = Fold(rows, ev("frontend", progress.PhaseExtractAST, 4))

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (backend, frontend) — dropped-repo bug", len(rows))
	}
	if _, ok := rows["backend"]; !ok {
		t.Error("backend row missing")
	}
	if _, ok := rows["frontend"]; !ok {
		t.Error("frontend row missing")
	}
}

// TestFold_MonotonicPhase: a late lower-ordered phase never regresses a row.
func TestFold_MonotonicPhase(t *testing.T) {
	rows := map[string]Row{}
	rows = Fold(rows, ev("r", progress.PhaseWriteGraph, 5))
	// A late scanning event (higher ts but lower phase) must not pull it back.
	rows = Fold(rows, ev("r", progress.PhaseScan, 6))
	if rows["r"].Phase != progress.PhaseWriteGraph {
		t.Errorf("phase regressed to %q, want %q", rows["r"].Phase, progress.PhaseWriteGraph)
	}
}

// TestFold_StaleTimestampIgnored: an event older than what we have is dropped.
func TestFold_StaleTimestampIgnored(t *testing.T) {
	rows := map[string]Row{}
	rows = Fold(rows, ev("r", progress.PhaseExtractAST, 10))
	before := rows["r"]
	rows = Fold(rows, ev("r", progress.PhaseScan, 5)) // older ts
	if rows["r"] != before {
		t.Errorf("stale event mutated row: %+v -> %+v", before, rows["r"])
	}
}

// TestFold_TerminalNotRegressed: a done row stays done even if a module-scoped
// in-flight event arrives later.
func TestFold_TerminalNotRegressed(t *testing.T) {
	rows := map[string]Row{}
	rows = Fold(rows, ev("r", progress.PhaseDone, 10))
	rows = Fold(rows, ev("r", progress.PhaseExtractAST, 11))
	if !rows["r"].Terminal() {
		t.Errorf("terminal row regressed to %q", rows["r"].Phase)
	}
}

// TestFold_FilesDoneNeverRegress: files_done only moves forward.
func TestFold_FilesDoneNeverRegress(t *testing.T) {
	rows := map[string]Row{}
	e := ev("r", progress.PhaseExtractAST, 1)
	e.FilesDone, e.FilesTotal = 50, 100
	rows = Fold(rows, e)
	e2 := ev("r", progress.PhaseExtractAST, 2)
	e2.FilesDone, e2.FilesTotal = 30, 100 // out-of-order lower count
	rows = Fold(rows, e2)
	if rows["r"].FilesDone != 50 {
		t.Errorf("files_done regressed to %d, want 50", rows["r"].FilesDone)
	}
}

// TestAggregateProgress_ExpectedReposDenominator: a not-yet-reported repo counts
// as 0 so the bar doesn't jump.
func TestAggregateProgress_ExpectedReposDenominator(t *testing.T) {
	rows := map[string]Row{}
	rows = Fold(rows, ev("a", progress.PhaseDone, 1)) // 100%
	// Only 1 of 2 expected repos reported → ~50%.
	got := AggregateProgress(rows, 2)
	if got < 0.45 || got > 0.55 {
		t.Errorf("aggregate = %.2f, want ~0.50 (1 done of 2 expected)", got)
	}
	// Without expectedRepos, denominator is rows → 100%.
	if g := AggregateProgress(rows, 0); g != 1 {
		t.Errorf("aggregate w/o expected = %.2f, want 1.0", g)
	}
}

// TestOverallPhaseLabel_LeastAdvanced: the gating label is the least-advanced
// active repo.
func TestOverallPhaseLabel_LeastAdvanced(t *testing.T) {
	rows := map[string]Row{}
	rows = Fold(rows, ev("a", progress.PhaseWriteGraph, 1))
	rows = Fold(rows, ev("b", progress.PhaseScan, 2))
	if l := OverallPhaseLabel(rows, false); l != "Scanning…" {
		t.Errorf("label = %q, want Scanning… (least-advanced gates)", l)
	}
	if l := OverallPhaseLabel(rows, true); l != "Done" {
		t.Errorf("terminal label = %q, want Done", l)
	}
}

// TestRowsTerminal_GatesOnExpected: not terminal until all expected repos exist
// and each is terminal.
func TestRowsTerminal_GatesOnExpected(t *testing.T) {
	rows := map[string]Row{}
	rows = Fold(rows, ev("a", progress.PhaseDone, 1))
	if RowsTerminal(rows, 2) {
		t.Error("reported terminal with 1/2 repos — early-fire bug")
	}
	rows = Fold(rows, ev("b", progress.PhaseDone, 2))
	if !RowsTerminal(rows, 2) {
		t.Error("not terminal with 2/2 done repos")
	}
	if RowsTerminal(rows, 0) {
		t.Error("terminal with unknown expected count — should defer")
	}
}

func TestSortRows_StableBySlug(t *testing.T) {
	rows := map[string]Row{}
	rows = Fold(rows, ev("zeta", progress.PhaseScan, 1))
	rows = Fold(rows, ev("alpha", progress.PhaseScan, 2))
	got := SortRows(rows)
	if got[0].RepoSlug != "alpha" || got[1].RepoSlug != "zeta" {
		t.Errorf("sort order = %v, want [alpha zeta]", []string{got[0].RepoSlug, got[1].RepoSlug})
	}
}
