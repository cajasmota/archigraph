package coverage

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// effFixture is a set of production entities already stamped with #5037
// reachability + #5036 line-coverage properties (as the #5061 enrichment pass
// would), exercising every cross-product quadrant:
//
//	reachNoLines  : reachable, coverage_pct=0   -> EffReachableNoLines (headline)
//	reachLow      : reachable, coverage_pct=20  -> EffReachableLowCoverage
//	reachCovered  : reachable, coverage_pct=90  -> EffReachableCovered
//	reachNoCov    : reachable, NO coverage prop -> EffReachableNoCoverage (degrade)
//	orphan        : reachable=false             -> EffUntested
//	unstamped     : no reachability prop        -> skipped entirely
//
// reachNoLines + reachLow live in module "api"; the rest in "svc"/"core".
func effFixture() []types.EntityRecord {
	mk := func(id, mod string, props map[string]string) types.EntityRecord {
		p := map[string]string{}
		for k, v := range props {
			p[k] = v
		}
		if mod != "" {
			p["module"] = mod
		}
		return types.EntityRecord{
			ID: id, Kind: string(types.EntityKindFunction), Name: id,
			SourceFile: "src/" + id + ".go", Properties: p,
		}
	}
	return []types.EntityRecord{
		mk("reachNoLines", "api", map[string]string{
			PropTestReachable: "true", PropReachDepth: "1", PropReachingTestCount: "2",
			PropCoveragePct: "0.0", PropCoveredLines: "0", PropTotalLines: "12",
		}),
		mk("reachLow", "api", map[string]string{
			PropTestReachable: "true", PropReachDepth: "2", PropReachingTestCount: "1",
			PropCoveragePct: "20.0", PropCoveredLines: "2", PropTotalLines: "10",
		}),
		mk("reachCovered", "svc", map[string]string{
			PropTestReachable: "true", PropReachDepth: "1", PropReachingTestCount: "3",
			PropCoveragePct: "90.0", PropCoveredLines: "9", PropTotalLines: "10",
		}),
		mk("reachNoCov", "svc", map[string]string{
			PropTestReachable: "true", PropReachDepth: "1", PropReachingTestCount: "1",
		}),
		mk("orphan", "core", map[string]string{
			PropTestReachable: "false",
		}),
		// Unstamped: no reachability prop -> skipped (e.g. indexed pre-#5061).
		{ID: "unstamped", Kind: string(types.EntityKindFunction), Name: "unstamped",
			SourceFile: "src/unstamped.go", Properties: map[string]string{"module": "core"}},
	}
}

func TestClassifyEffectiveness_Quadrants(t *testing.T) {
	cases := []struct {
		name  string
		props map[string]string
		want  EffectivenessVerdict
	}{
		{"reachable 0% -> no lines", map[string]string{PropTestReachable: "true", PropCoveragePct: "0.0"}, EffReachableNoLines},
		{"reachable just below threshold -> low", map[string]string{PropTestReachable: "true", PropCoveragePct: "49.9"}, EffReachableLowCoverage},
		{"reachable at threshold -> covered", map[string]string{PropTestReachable: "true", PropCoveragePct: "50.0"}, EffReachableCovered},
		{"reachable high -> covered", map[string]string{PropTestReachable: "true", PropCoveragePct: "95.0"}, EffReachableCovered},
		{"reachable no coverage prop -> degrade", map[string]string{PropTestReachable: "true"}, EffReachableNoCoverage},
		{"reachable bad coverage prop -> degrade", map[string]string{PropTestReachable: "true", PropCoveragePct: "n/a"}, EffReachableNoCoverage},
		{"unreachable -> untested", map[string]string{PropTestReachable: "false"}, EffUntested},
		{"no reachability prop -> not measured", map[string]string{PropCoveragePct: "80.0"}, EffNotMeasured},
	}
	for _, c := range cases {
		if got := ClassifyEffectiveness(c.props); got != c.want {
			t.Errorf("%s: ClassifyEffectiveness = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestComputeEffectivenessReport_QuadrantCountsAndIneffectiveList(t *testing.T) {
	rep := ComputeEffectivenessReport(effFixture())

	// Group quadrant counts. The unstamped entity must be skipped entirely.
	g := rep.Group
	if g.Total != 5 {
		t.Fatalf("Group.Total = %d, want 5 (unstamped entity must be skipped)", g.Total)
	}
	if g.ReachableNoLines != 1 {
		t.Errorf("ReachableNoLines = %d, want 1", g.ReachableNoLines)
	}
	if g.ReachableLowCoverage != 1 {
		t.Errorf("ReachableLowCoverage = %d, want 1", g.ReachableLowCoverage)
	}
	if g.ReachableCovered != 1 {
		t.Errorf("ReachableCovered = %d, want 1", g.ReachableCovered)
	}
	if g.ReachableNoCoverage != 1 {
		t.Errorf("ReachableNoCoverage = %d, want 1", g.ReachableNoCoverage)
	}
	if g.Untested != 1 {
		t.Errorf("Untested = %d, want 1", g.Untested)
	}
	if g.CoverageMeasured != 3 {
		t.Errorf("CoverageMeasured = %d, want 3 (reachNoLines+reachLow+reachCovered)", g.CoverageMeasured)
	}
	if !g.LineCrossAvailable() {
		t.Error("LineCrossAvailable = false, want true (3 measured entities)")
	}

	// Headline ineffective-test list: exactly reachNoLines.
	if len(rep.Ineffective) != 1 {
		t.Fatalf("len(Ineffective) = %d, want 1", len(rep.Ineffective))
	}
	in := rep.Ineffective[0]
	if in.EntityID != "reachNoLines" {
		t.Errorf("Ineffective[0].EntityID = %q, want reachNoLines", in.EntityID)
	}
	if in.Verdict != EffReachableNoLines {
		t.Errorf("Ineffective[0].Verdict = %q, want %q", in.Verdict, EffReachableNoLines)
	}
	if !in.Reachable || in.ReachCount != 2 || !in.HasCoverage || in.CoveragePct != 0 {
		t.Errorf("Ineffective[0] raw signals wrong: %+v", in)
	}

	// Per-module roll-up: "api" has the two reachable-with-coverage, "svc" has
	// covered+nocov, "core" has the single orphan.
	api := rep.Modules["api"]
	if api.Total != 2 || api.ReachableNoLines != 1 || api.ReachableLowCoverage != 1 {
		t.Errorf("api module roll-up wrong: %+v", api)
	}
	svc := rep.Modules["svc"]
	if svc.Total != 2 || svc.ReachableCovered != 1 || svc.ReachableNoCoverage != 1 {
		t.Errorf("svc module roll-up wrong: %+v", svc)
	}
	if svc.CoverageMeasured != 1 {
		t.Errorf("svc CoverageMeasured = %d, want 1", svc.CoverageMeasured)
	}
	core := rep.Modules["core"]
	if core.Total != 1 || core.Untested != 1 {
		t.Errorf("core module roll-up wrong: %+v", core)
	}
}

// TestComputeEffectivenessReport_HonestDegradation: a group with reachability
// but NO ingested line coverage reports reachability quadrants and signals that
// the line-coverage cross is unavailable — it does NOT fabricate ineffective
// tests.
func TestComputeEffectivenessReport_HonestDegradation(t *testing.T) {
	ents := []types.EntityRecord{
		{ID: "a", Kind: string(types.EntityKindFunction), Name: "a", SourceFile: "src/a.go",
			Properties: map[string]string{PropTestReachable: "true", PropReachDepth: "1", PropReachingTestCount: "1"}},
		{ID: "b", Kind: string(types.EntityKindFunction), Name: "b", SourceFile: "src/b.go",
			Properties: map[string]string{PropTestReachable: "false"}},
	}
	rep := ComputeEffectivenessReport(ents)
	if rep.Group.CoverageMeasured != 0 {
		t.Fatalf("CoverageMeasured = %d, want 0", rep.Group.CoverageMeasured)
	}
	if rep.Group.LineCrossAvailable() {
		t.Error("LineCrossAvailable = true, want false (no ingested line coverage)")
	}
	if len(rep.Ineffective) != 0 {
		t.Errorf("Ineffective = %v, want empty (must not fabricate without coverage)", rep.Ineffective)
	}
	if rep.Group.ReachableNoCoverage != 1 || rep.Group.Untested != 1 {
		t.Errorf("degraded quadrants wrong: %+v", rep.Group)
	}
}

// TestComputeEffectivenessReport_Unstamped: an entirely unstamped batch yields
// an empty report so the caller can say "reachability not computed — reindex".
func TestComputeEffectivenessReport_Unstamped(t *testing.T) {
	ents := []types.EntityRecord{
		{ID: "x", Kind: string(types.EntityKindFunction), Name: "x", SourceFile: "src/x.go"},
	}
	rep := ComputeEffectivenessReport(ents)
	if rep.Group.Total != 0 || len(rep.Rows) != 0 {
		t.Errorf("unstamped batch: want empty report, got Total=%d rows=%d", rep.Group.Total, len(rep.Rows))
	}
}
