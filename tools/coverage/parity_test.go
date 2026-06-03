package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// rec is a tiny helper to build a flat-capability record for parity
// fixtures. Each entry in caps is "key:status".
func rec(id, lang, cat, sub string, caps ...string) Record {
	m := map[string]Capability{}
	for _, c := range caps {
		parts := strings.SplitN(c, ":", 2)
		m[parts[0]] = Capability{Status: parts[1]}
	}
	return Record{ID: id, Language: lang, Category: cat, Subcategory: sub, Label: id, Capabilities: m}
}

// findFinding returns the finding for a capability, or nil.
func findFinding(r ParityReport, cap string) *ParityFinding {
	for i := range r.Findings {
		if r.Findings[i].Capability == cap {
			return &r.Findings[i]
		}
	}
	return nil
}

func ids(fs []parityFrameworkStatus) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.RecordID
	}
	return out
}

// TestParityAsymmetryFlagsTrailingSiblings is the core value assertion:
// a group with 1 full + 2 missing for a capability yields exactly one
// finding that names BOTH trailing frameworks, the flagship, and the
// honest group size.
func TestParityAsymmetryFlagsTrailingSiblings(t *testing.T) {
	reg := &Registry{Records: []Record{
		rec("lang.x.framework.a", "x", "http_framework", "http_backend", "auth_coverage:full"),
		rec("lang.x.framework.b", "x", "http_framework", "http_backend", "auth_coverage:missing"),
		rec("lang.x.framework.c", "x", "http_framework", "http_backend", "auth_coverage:missing"),
	}}
	r := computeParity(reg, "", 2, true)
	if len(r.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(r.Findings), r.Findings)
	}
	f := r.Findings[0]
	if f.Capability != "auth_coverage" {
		t.Fatalf("wrong capability: %q", f.Capability)
	}
	if f.GroupSize != 3 {
		t.Fatalf("expected group size 3, got %d", f.GroupSize)
	}
	if got := ids(f.FullIn); len(got) != 1 || got[0] != "lang.x.framework.a" {
		t.Fatalf("flagship mismatch: %v", got)
	}
	if got := ids(f.TrailingIn); len(got) != 2 ||
		got[0] != "lang.x.framework.b" || got[1] != "lang.x.framework.c" {
		t.Fatalf("expected both trailing siblings named in order, got %v", got)
	}
}

// TestParityUniformScaffoldSuppressed is the anti-trap assertion: a group
// where EVERY framework is missing the capability (the http_framework+orm
// matrix scaffold default) produces ZERO findings, and is counted as a
// suppressed scaffold so the report proves it saw and ignored it.
func TestParityUniformScaffoldSuppressed(t *testing.T) {
	reg := &Registry{Records: []Record{
		rec("lang.y.framework.a", "y", "orm", "", "orm_mapping:missing"),
		rec("lang.y.framework.b", "y", "orm", "", "orm_mapping:missing"),
		rec("lang.y.framework.c", "y", "orm", "", "orm_mapping:missing"),
	}}
	r := computeParity(reg, "", 2, true)
	if len(r.Findings) != 0 {
		t.Fatalf("uniform-all-missing scaffold must yield 0 findings, got %d: %+v", len(r.Findings), r.Findings)
	}
	if r.SuppressedScaff != 1 {
		t.Fatalf("expected 1 suppressed-scaffold cell, got %d", r.SuppressedScaff)
	}
}

// TestParityAllCreditedNoFinding: a group where every framework is full
// (no trailing sibling) is not an asymmetry.
func TestParityAllCreditedNoFinding(t *testing.T) {
	reg := &Registry{Records: []Record{
		rec("lang.z.framework.a", "z", "http_framework", "http_backend", "endpoint_synthesis:full"),
		rec("lang.z.framework.b", "z", "http_framework", "http_backend", "endpoint_synthesis:full"),
	}}
	r := computeParity(reg, "", 2, true)
	if len(r.Findings) != 0 {
		t.Fatalf("all-credited group must yield 0 findings, got %+v", r.Findings)
	}
	if r.SuppressedScaff != 0 {
		t.Fatalf("all-credited is not a scaffold suppression, got %d", r.SuppressedScaff)
	}
}

// TestParityPartialFlagshipExposesGap: a partial flagship still exposes a
// missing sibling when include-partial is on, and lands in PartialIn.
// With include-partial off, partial is neither credit nor gap so the cell
// becomes all-missing-but-one-partial → no credited flagship → no finding.
func TestParityPartialFlagship(t *testing.T) {
	reg := &Registry{Records: []Record{
		rec("lang.p.framework.a", "p", "http_framework", "http_backend", "request_validation:partial"),
		rec("lang.p.framework.b", "p", "http_framework", "http_backend", "request_validation:missing"),
	}}
	on := computeParity(reg, "", 2, true)
	f := findFinding(on, "request_validation")
	if f == nil {
		t.Fatal("partial flagship should expose the missing sibling when include-partial=true")
	}
	if got := ids(f.PartialIn); len(got) != 1 || got[0] != "lang.p.framework.a" {
		t.Fatalf("expected partial flagship in PartialIn, got %v", got)
	}
	if len(f.FullIn) != 0 {
		t.Fatalf("no full flagship expected, got %v", ids(f.FullIn))
	}

	off := computeParity(reg, "", 2, false)
	if findFinding(off, "request_validation") != nil {
		t.Fatal("with include-partial=false, a partial-only flagship must not produce a finding")
	}
}

// TestParityMinGroupThreshold: a singleton group (only one framework
// declares the capability / only one sibling exists) is never flagged.
func TestParityMinGroupThreshold(t *testing.T) {
	reg := &Registry{Records: []Record{
		rec("lang.s.framework.solo", "s", "http_framework", "http_backend", "auth_coverage:full"),
	}}
	r := computeParity(reg, "", 2, true)
	if len(r.Findings) != 0 {
		t.Fatalf("singleton group must never be flagged, got %+v", r.Findings)
	}

	// A 2-framework asymmetric group is flagged at min-group=2 but
	// suppressed when the threshold is raised to 3.
	reg2 := &Registry{Records: []Record{
		rec("lang.s.framework.a", "s", "http_framework", "http_backend", "auth_coverage:full"),
		rec("lang.s.framework.b", "s", "http_framework", "http_backend", "auth_coverage:missing"),
	}}
	if got := len(computeParity(reg2, "", 2, true).Findings); got != 1 {
		t.Fatalf("min-group=2: expected 1 finding, got %d", got)
	}
	if got := len(computeParity(reg2, "", 3, true).Findings); got != 0 {
		t.Fatalf("min-group=3: expected 0 findings, got %d", got)
	}
}

// TestParityNotApplicableIsNeutral: a not_applicable cell is neither a
// flagship nor a trailing gap. A group of {full, not_applicable} has no
// missing sibling → no finding. {not_applicable, missing} has no credited
// flagship → no finding (and is not a scaffold either, since n/a≠missing).
func TestParityNotApplicableIsNeutral(t *testing.T) {
	reg := &Registry{Records: []Record{
		rec("lang.n.framework.a", "n", "http_framework", "http_backend", "auth_coverage:full"),
		rec("lang.n.framework.b", "n", "http_framework", "http_backend", "auth_coverage:not_applicable"),
	}}
	if got := computeParity(reg, "", 2, true).Findings; len(got) != 0 {
		t.Fatalf("{full, n/a} has no trailing gap, got %+v", got)
	}
	reg2 := &Registry{Records: []Record{
		rec("lang.n.framework.a", "n", "http_framework", "http_backend", "auth_coverage:not_applicable"),
		rec("lang.n.framework.b", "n", "http_framework", "http_backend", "auth_coverage:missing"),
	}}
	r := computeParity(reg2, "", 2, true)
	if len(r.Findings) != 0 {
		t.Fatalf("{n/a, missing} has no credited flagship, got %+v", r.Findings)
	}
	if r.SuppressedScaff != 0 {
		t.Fatalf("{n/a, missing} is not a uniform scaffold (n/a != missing), got %d", r.SuppressedScaff)
	}
}

// TestParitySubcategoryIsolation: siblings are scoped to the SAME
// (category, subcategory) lane. A full in one subcategory does not flag a
// missing in a different subcategory of the same category/language.
func TestParitySubcategoryIsolation(t *testing.T) {
	reg := &Registry{Records: []Record{
		rec("lang.q.framework.backend", "q", "http_framework", "http_backend", "auth_coverage:full"),
		rec("lang.q.framework.ui", "q", "http_framework", "ui_frontend", "auth_coverage:missing"),
	}}
	if got := computeParity(reg, "", 2, true).Findings; len(got) != 0 {
		t.Fatalf("cross-subcategory comparison must not flag, got %+v", got)
	}
}

// TestParityGroupedAndFrameworkSpecificCells exercises the non-flat
// carriers: a grouped flagship and a framework_specific trailing cell are
// both flattened by AllCapabilitiesIncludingFrameworkSpecific and compared.
func TestParityGroupedCells(t *testing.T) {
	flagship := Record{
		ID: "lang.g.framework.a", Language: "g", Category: "http_framework", Subcategory: "http_backend", Label: "A",
		Groups: map[string]map[string]Capability{
			"Auth": {"auth_coverage": {Status: StatusFull}},
		},
	}
	sibling := Record{
		ID: "lang.g.framework.b", Language: "g", Category: "http_framework", Subcategory: "http_backend", Label: "B",
		Groups: map[string]map[string]Capability{
			"Auth": {"auth_coverage": {Status: StatusMissing}},
		},
	}
	r := computeParity(&Registry{Records: []Record{flagship, sibling}}, "", 2, true)
	f := findFinding(r, "auth_coverage")
	if f == nil {
		t.Fatal("grouped flagship vs grouped sibling should produce a finding")
	}
	if got := ids(f.TrailingIn); len(got) != 1 || got[0] != "lang.g.framework.b" {
		t.Fatalf("trailing sibling mismatch: %v", got)
	}
}

// TestParityLanguageFilter restricts the scan to one language.
func TestParityLanguageFilter(t *testing.T) {
	reg := &Registry{Records: []Record{
		rec("lang.a.framework.x", "a", "http_framework", "http_backend", "auth_coverage:full"),
		rec("lang.a.framework.y", "a", "http_framework", "http_backend", "auth_coverage:missing"),
		rec("lang.b.framework.x", "b", "http_framework", "http_backend", "auth_coverage:full"),
		rec("lang.b.framework.y", "b", "http_framework", "http_backend", "auth_coverage:missing"),
	}}
	r := computeParity(reg, "a", 2, true)
	if len(r.Findings) != 1 || r.Findings[0].Language != "a" {
		t.Fatalf("language filter should yield only language a, got %+v", r.Findings)
	}
}

// TestParityCmdStrictExit asserts the CLI contract: without --strict the
// command succeeds (informational) even with findings; with --strict it
// exits non-zero when findings exist; JSON is well-formed.
func TestParityCmdStrictExit(t *testing.T) {
	out, _, err := runCmd(t, "parity", "--file", fixturePath(t))
	if err != nil {
		t.Fatalf("parity (non-strict) must not error: %v", err)
	}
	if !strings.Contains(out, "coverage parity probe") {
		t.Fatalf("unexpected text output:\n%s", out)
	}

	// JSON output decodes into a ParityReport.
	jout, _, err := runCmd(t, "parity", "--file", fixturePath(t), "--json")
	if err != nil {
		t.Fatalf("parity --json: %v", err)
	}
	var report ParityReport
	if err := json.Unmarshal([]byte(jout), &report); err != nil {
		t.Fatalf("parity --json did not produce valid ParityReport JSON: %v\n%s", err, jout)
	}

	// --strict exits non-zero iff there is at least one finding.
	_, _, serr := runCmd(t, "parity", "--file", fixturePath(t), "--strict")
	if len(report.Findings) > 0 && serr == nil {
		t.Fatal("parity --strict must exit non-zero when findings exist")
	}
	if len(report.Findings) == 0 && serr != nil {
		t.Fatalf("parity --strict must succeed when no findings: %v", serr)
	}
}

// TestParityDeterministic: the report ordering is stable across runs
// regardless of record input order.
func TestParityDeterministic(t *testing.T) {
	a := []Record{
		rec("lang.d.framework.a", "d", "http_framework", "http_backend", "auth_coverage:full"),
		rec("lang.d.framework.b", "d", "http_framework", "http_backend", "auth_coverage:missing", "request_validation:missing"),
		rec("lang.d.framework.c", "d", "http_framework", "http_backend", "request_validation:full"),
	}
	b := []Record{a[2], a[0], a[1]}
	r1 := computeParity(&Registry{Records: a}, "", 2, true)
	r2 := computeParity(&Registry{Records: b}, "", 2, true)
	if len(r1.Findings) != len(r2.Findings) {
		t.Fatalf("finding count differs by input order: %d vs %d", len(r1.Findings), len(r2.Findings))
	}
	for i := range r1.Findings {
		if r1.Findings[i].Capability != r2.Findings[i].Capability {
			t.Fatalf("finding order not deterministic at %d: %q vs %q", i, r1.Findings[i].Capability, r2.Findings[i].Capability)
		}
	}
}
