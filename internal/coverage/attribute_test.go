package coverage

import (
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

func sampleEntities() []types.EntityRecord {
	return []types.EntityRecord{
		// Whole-file (module) entity — no span → file-scope = 80%.
		{ID: "mod1", Kind: string(types.EntityKindModule), Name: "calc", SourceFile: "src/calc.ts"},
		// Fully-covered function (lines 1-5 all hit) → 100%.
		{ID: "fn-add", Kind: string(types.EntityKindFunction), Name: "addNumbers", SourceFile: "src/calc.ts", StartLine: 1, EndLine: 5},
		// Uncovered function (lines 7-9 all 0) → 0%.
		{ID: "fn-unused", Kind: string(types.EntityKindFunction), Name: "unusedHelper", SourceFile: "src/calc.ts", StartLine: 7, EndLine: 9},
		// Entity in a file not present in the report → skipped.
		{ID: "fn-orphan", Kind: string(types.EntityKindFunction), Name: "ghost", SourceFile: "src/missing.ts", StartLine: 1, EndLine: 3},
	}
}

func TestAttribute(t *testing.T) {
	rep := loadSample(t)
	// rootPrefix "project" strips /home/ci/project/ from calc.ts so it
	// normalizes to src/calc.ts and joins the entity source files.
	attrs := Attribute(sampleEntities(), rep, "project")

	got := map[string]Attribution{}
	for _, a := range attrs {
		got[a.EntityID] = a
	}
	if len(got) != 3 {
		t.Fatalf("want 3 attributions (orphan skipped), got %d: %+v", len(got), attrs)
	}

	if a := got["mod1"]; a.CoveragePct != 80.0 || a.CoveredLines != 8 || a.TotalLines != 10 {
		t.Errorf("file-scope: want 80%% 8/10, got %.1f%% %d/%d", a.CoveragePct, a.CoveredLines, a.TotalLines)
	}
	if a := got["fn-add"]; a.CoveragePct != 100.0 || a.CoveredLines != 5 || a.TotalLines != 5 {
		t.Errorf("addNumbers: want 100%% 5/5, got %.1f%% %d/%d", a.CoveragePct, a.CoveredLines, a.TotalLines)
	}
	if a := got["fn-unused"]; a.CoveragePct != 0.0 || a.CoveredLines != 0 || a.TotalLines != 3 {
		t.Errorf("unusedHelper: want 0%% 0/3, got %.1f%% %d/%d", a.CoveragePct, a.CoveredLines, a.TotalLines)
	}
	if _, ok := got["fn-orphan"]; ok {
		t.Error("orphan entity (file not in report) must be skipped")
	}
	for _, a := range attrs {
		if a.Source != SourceLCOV {
			t.Errorf("source: want %q, got %q", SourceLCOV, a.Source)
		}
	}
}

func TestApplyAttributions(t *testing.T) {
	rep := loadSample(t)
	ents := sampleEntities()
	attrs := Attribute(ents, rep, "project")

	out := ApplyAttributions(ents, attrs, "2026-06-12T00:00:00Z")

	// Inputs not mutated.
	if ents[1].Properties != nil {
		t.Error("input entity was mutated")
	}
	var add types.EntityRecord
	for _, e := range out {
		if e.ID == "fn-add" {
			add = e
		}
	}
	if add.Properties[PropCoveragePct] != "100.0" {
		t.Errorf("coverage_pct prop: want 100.0, got %q", add.Properties[PropCoveragePct])
	}
	if add.Properties[PropCoveredLines] != "5" || add.Properties[PropTotalLines] != "5" {
		t.Errorf("line props wrong: %+v", add.Properties)
	}
	if add.Properties[PropCoverageSource] != SourceLCOV {
		t.Errorf("source prop: %q", add.Properties[PropCoverageSource])
	}
	if add.Properties[PropCoverageMeasAt] != "2026-06-12T00:00:00Z" {
		t.Errorf("measured_at prop: %q", add.Properties[PropCoverageMeasAt])
	}
}

func TestConfigEnabled(t *testing.T) {
	if (Config{}).Enabled() {
		t.Error("empty config must be disabled")
	}
	if (Config{Format: FormatLCOV}).Enabled() {
		t.Error("format without paths must be disabled")
	}
	if !(Config{Format: FormatLCOV, ReportPaths: []string{"coverage/lcov.info"}}).Enabled() {
		t.Error("format+paths must be enabled")
	}
}
