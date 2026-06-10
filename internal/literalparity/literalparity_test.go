package literalparity

import (
	"reflect"
	"testing"
)

func TestNormalizeKey(t *testing.T) {
	cases := map[string]string{
		"PAGE_SLUG":       "page_slug",
		"page-slug":       "page_slug",
		"page.slug":       "page_slug",
		"  Page Slug  ":   "page_slug",
		"CORE_ADMIN":      "core_admin",
		"core-admin":      "core_admin",
		"a__b--c":         "a_b_c",
		"trailing_":       "trailing",
		"witnessing/comp": "witnessing_comp",
	}
	for in, want := range cases {
		if got := NormalizeKey(in); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCanonicalKey(t *testing.T) {
	cases := map[string]string{
		"CORE_ADMIN":   "core_admin",
		"CoreAdmin":    "core_admin", // PascalCase splits to align with SCREAMING_SNAKE
		"coreAdmin":    "core_admin",
		"core-admin":   "core_admin",
		"EmailTemplates": "email_templates",
		"EMAIL_TEMPLATES": "email_templates",
		"HTTPServer":   "http_server", // acronym/word boundary
		"page2Slug":    "page_2_slug", // letter↔digit boundary
		"PERMISSION_PAGES": "permission_pages",
		"PermissionPage":   "permission_page",
	}
	for in, want := range cases {
		if got := CanonicalKey(in); got != want {
			t.Errorf("CanonicalKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// BUG A (#4532): cross-framework key alignment. Oracle SCREAMING_SNAKE keys vs
// v3 PascalCase keys must align by canonical form so a real value drift reaches
// value_mismatches instead of being dumped into only_in_* by key-spelling.
func TestDiff_CrossFrameworkKeyAlignment(t *testing.T) {
	oracle := []Member{
		{Key: "CORE_ADMIN", Value: "core-admin"},
		{Key: "EMAIL_TEMPLATES", Value: "email-templates"},
		{Key: "AOC_HARVEST", Value: "aoc-harvest"},
	}
	v3 := []Member{
		{Key: "CoreAdmin", Value: "core_admin"},      // _ vs - drift, same canonical key
		{Key: "EmailTemplates", Value: "email-templates"}, // reconciled
		{Key: "AocHarvest", Value: "aoc-harvest"},          // reconciled
	}
	res := Diff("page_slugs", oracle, v3)
	if res.Verdict != VerdictDrift {
		t.Fatalf("verdict = %q, want drift; %+v", res.Verdict, res)
	}
	want := []ValueMismatch{{Key: "CORE_ADMIN", Oracle: "core-admin", V3: "core_admin"}}
	if !reflect.DeepEqual(res.ValueMismatches, want) {
		t.Fatalf("value_mismatches = %+v, want %+v", res.ValueMismatches, want)
	}
	// And the differing literals surface in the value multiset too.
	if !reflect.DeepEqual(res.OnlyInOracle, []string{"core-admin"}) {
		t.Errorf("only_in_oracle = %v, want [core-admin]", res.OnlyInOracle)
	}
	if !reflect.DeepEqual(res.OnlyInV3, []string{"core_admin"}) {
		t.Errorf("only_in_v3 = %v, want [core_admin]", res.OnlyInV3)
	}
}

// BUG A (#4532): a fully reconciled cross-framework set (SCREAMING_SNAKE vs
// PascalCase keys, IDENTICAL values both sides) must return equivalent.
func TestDiff_CrossFrameworkReconciledEquivalent(t *testing.T) {
	oracle := []Member{
		{Key: "CORE_ADMIN", Value: "core_admin"},
		{Key: "EMAIL_TEMPLATES", Value: "email_templates"},
	}
	v3 := []Member{
		{Key: "CoreAdmin", Value: "core_admin"},
		{Key: "EmailTemplates", Value: "email_templates"},
	}
	res := Diff("page_slugs", oracle, v3)
	if res.Verdict != VerdictEquivalent {
		t.Fatalf("verdict = %q, want equivalent; %+v", res.Verdict, res)
	}
	if len(res.OnlyInOracle) != 0 || len(res.OnlyInV3) != 0 ||
		len(res.ValueMismatches) != 0 || len(res.IntraV3Inconsistencies) != 0 {
		t.Fatalf("expected clean equivalent, got %+v", res)
	}
}

// equivalent: identical key+value sets (modulo key separator/case) → no diff.
func TestDiff_Equivalent(t *testing.T) {
	oracle := []Member{
		{Key: "DASHBOARD", Value: "dashboard"},
		{Key: "SETTINGS", Value: "settings"},
	}
	v3 := []Member{
		{Key: "dashboard", Value: "dashboard"},
		{Key: "settings", Value: "settings"},
	}
	res := Diff("page_slugs", oracle, v3)
	if res.Verdict != VerdictEquivalent {
		t.Fatalf("verdict = %q, want equivalent; result=%+v", res.Verdict, res)
	}
	if len(res.OnlyInOracle) != 0 || len(res.OnlyInV3) != 0 ||
		len(res.ValueMismatches) != 0 || len(res.IntraV3Inconsistencies) != 0 {
		t.Fatalf("expected clean equivalent, got %+v", res)
	}
}

// value_mismatch: same aligned key, different literal value (the _ vs - class).
func TestDiff_ValueMismatch(t *testing.T) {
	oracle := []Member{{Key: "ADMIN", Value: "core_admin"}}
	v3 := []Member{{Key: "ADMIN", Value: "core-admin"}}
	res := Diff("action_codenames", oracle, v3)
	if res.Verdict != VerdictDrift {
		t.Fatalf("verdict = %q, want drift", res.Verdict)
	}
	want := []ValueMismatch{{Key: "ADMIN", Oracle: "core_admin", V3: "core-admin"}}
	if !reflect.DeepEqual(res.ValueMismatches, want) {
		t.Fatalf("value_mismatches = %+v, want %+v", res.ValueMismatches, want)
	}
	// The differing literals ALSO surface in the value-multiset membership (the
	// value is the parity contract): oracle-only "core_admin", v3-only "core-admin".
	if !reflect.DeepEqual(res.OnlyInOracle, []string{"core_admin"}) {
		t.Errorf("only_in_oracle = %v, want [core_admin]", res.OnlyInOracle)
	}
	if !reflect.DeepEqual(res.OnlyInV3, []string{"core-admin"}) {
		t.Errorf("only_in_v3 = %v, want [core-admin]", res.OnlyInV3)
	}
}

// only_in: keys present on only one side.
func TestDiff_OnlyIn(t *testing.T) {
	oracle := []Member{
		{Key: "KEEP", Value: "keep"},
		{Key: "DROPPED", Value: "dropped"},
	}
	v3 := []Member{
		{Key: "KEEP", Value: "keep"},
		{Key: "ADDED", Value: "added"},
	}
	res := Diff("status_strings", oracle, v3)
	if res.Verdict != VerdictDrift {
		t.Fatalf("verdict = %q, want drift", res.Verdict)
	}
	// Membership is reported on the VALUE multiset: oracle-only value "dropped",
	// v3-only value "added" (the shared "keep" value is on both sides).
	if !reflect.DeepEqual(res.OnlyInOracle, []string{"dropped"}) {
		t.Errorf("only_in_oracle = %v, want [dropped]", res.OnlyInOracle)
	}
	if !reflect.DeepEqual(res.OnlyInV3, []string{"added"}) {
		t.Errorf("only_in_v3 = %v, want [added]", res.OnlyInV3)
	}
}

// intra-v3 separator inconsistency: v3 mixes underscore + hyphen value
// conventions within one set.
func TestDiff_IntraV3Inconsistency(t *testing.T) {
	oracle := []Member{
		{Key: "EMAIL", Value: "email_templates"},
		{Key: "WITNESS", Value: "witnessing_companies"},
	}
	v3 := []Member{
		{Key: "EMAIL", Value: "email_templates"},      // snake
		{Key: "WITNESS", Value: "witnessing-companies"}, // kebab — the outlier
	}
	res := Diff("page_slugs", oracle, v3)
	if res.Verdict != VerdictDrift {
		t.Fatalf("verdict = %q, want drift", res.Verdict)
	}
	if len(res.IntraV3Inconsistencies) != 1 {
		t.Fatalf("expected 1 intra-v3 inconsistency, got %+v", res.IntraV3Inconsistencies)
	}
	ic := res.IntraV3Inconsistencies[0]
	if ic.Convention != "snake" {
		t.Errorf("dominant convention = %q, want snake", ic.Convention)
	}
	if !reflect.DeepEqual(ic.Outliers, []string{"WITNESS"}) {
		t.Errorf("outliers = %v, want [WITNESS]", ic.Outliers)
	}
	// This case ALSO trips a value_mismatch on the aligned WITNESS key.
	if len(res.ValueMismatches) != 1 || res.ValueMismatches[0].Key != "WITNESS" {
		t.Errorf("expected value_mismatch on WITNESS, got %+v", res.ValueMismatches)
	}
}

// A clean v3 set with one consistent convention does NOT trip intra-v3.
func TestDiff_IntraV3_ConsistentSnake(t *testing.T) {
	v3 := []Member{
		{Key: "A", Value: "alpha_one"},
		{Key: "B", Value: "beta_two"},
		{Key: "C", Value: "single"}, // no separator — convention-neutral
	}
	ic := detectIntraInconsistency(v3)
	if len(ic) != 0 {
		t.Fatalf("expected no intra inconsistency for consistent snake set, got %+v", ic)
	}
}

// A single value carrying BOTH separators is flagged "mixed".
func TestDiff_IntraV3_MixedSeparator(t *testing.T) {
	v3 := []Member{
		{Key: "A", Value: "weird_mixed-token"},
		{Key: "B", Value: "plain_snake"},
	}
	ic := detectIntraInconsistency(v3)
	if len(ic) != 1 {
		t.Fatalf("expected 1 inconsistency for mixed-separator value, got %+v", ic)
	}
	found := false
	for _, o := range ic[0].Outliers {
		if o == "A" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected A flagged as outlier, got %+v", ic[0].Outliers)
	}
}

// Value-less members (enum constants without literals) fall back to KEY
// convention and still participate in alignment / membership.
func TestDiff_ValuelessMembers(t *testing.T) {
	oracle := []Member{{Key: "ACTIVE"}, {Key: "ARCHIVED"}}
	v3 := []Member{{Key: "active"}, {Key: "archived"}}
	res := Diff("enum:Status", oracle, v3)
	if res.Verdict != VerdictEquivalent {
		t.Fatalf("verdict = %q, want equivalent; %+v", res.Verdict, res)
	}
}
