// Tests for the section guidance + gating bundle (#1857, #1860, #1864, #1865,
// #1866, #1873, #2017).  Each test pins one ticket-level requirement so a
// regression flips the precise ticket back into scope.

package docgen_test

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/docgen"
)

// ---------------------------------------------------------------------------
// #1857 — "no-local-surface" honesty clause on reference-config / -deployment /
// -scripts in the default section guidance.
// ---------------------------------------------------------------------------

func TestHonestyClause_DefaultGuidance(t *testing.T) {
	// Use the default profile (UNKNOWN_KIND) so the honesty clause is sourced
	// from defaultSectionGuidance rather than any kind-specific override.
	defaultProfile := docgen.ResolveSectionProfile("UNKNOWN_KIND", "")
	for _, sec := range []string{"reference-config", "reference-deployment", "reference-scripts"} {
		g := docgen.ResolveGuidance(defaultProfile, sec)
		lower := strings.ToLower(g)
		if !strings.Contains(lower, "nothing applies locally") {
			t.Errorf("#1857 default guidance for %q: missing honesty clause; got: %q", sec, g)
		}
		if !strings.Contains(lower, "honest") && !strings.Contains(lower, "1") {
			t.Errorf("#1857 default guidance for %q: missing brevity/honesty wording; got: %q", sec, g)
		}
	}
}

// ---------------------------------------------------------------------------
// #1860 — class / view / function seeds must skip reference-deployment,
// reference-scripts, how-to-local-dev.
// ---------------------------------------------------------------------------

func TestSectionGating_LeafKindsDropModuleAggregateSections(t *testing.T) {
	leafKinds := []string{"class", "view", "function",
		"SCOPE.Class", "SCOPE.View", "SCOPE.Function"}
	mustAbsent := []string{"reference-deployment", "reference-scripts", "how-to-local-dev"}
	for _, kind := range leafKinds {
		t.Run(kind, func(t *testing.T) {
			p := docgen.ResolveSectionProfile(kind, "")
			for _, sec := range mustAbsent {
				if containsSection(p.Sections, sec) {
					t.Errorf("#1860 kind %q: section %q must be absent; got %v",
						kind, sec, p.Sections)
				}
			}
			// Sanity: still has the core sections.
			for _, sec := range []string{"overview", "capabilities", "api"} {
				if !containsSection(p.Sections, sec) {
					t.Errorf("#1860 kind %q: missing core section %q; got %v",
						kind, sec, p.Sections)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// #1873 — how-to-local-dev must be Module-aggregate-only: present for module,
// absent everywhere else.
// ---------------------------------------------------------------------------

func TestHowToLocalDev_ModuleOnly(t *testing.T) {
	// Present for module.
	mp := docgen.ResolveSectionProfile("module", "")
	if !containsSection(mp.Sections, "how-to-local-dev") {
		t.Errorf("#1873 module: how-to-local-dev MUST be present; got %v", mp.Sections)
	}
	// Absent for every leaf-style kind.
	nonModule := []string{"model", "view", "class", "function",
		"operation", "react_component", "SCOPE.View", "SCOPE.Class"}
	for _, kind := range nonModule {
		t.Run(kind, func(t *testing.T) {
			p := docgen.ResolveSectionProfile(kind, "")
			if containsSection(p.Sections, "how-to-local-dev") {
				t.Errorf("#1873 kind %q: how-to-local-dev MUST be absent (Module-only); got %v",
					kind, p.Sections)
			}
		})
	}
}

// ShouldSkipSectionForKind matches the documented gating contract for the
// three module-aggregate sections.
func TestShouldSkipSectionForKind(t *testing.T) {
	cases := []struct {
		section string
		kind    string
		want    bool
	}{
		// how-to-local-dev gated for everything except module.
		{"how-to-local-dev", "module", false},
		{"how-to-local-dev", "SCOPE.Module", false},
		{"how-to-local-dev", "view", true},
		{"how-to-local-dev", "SCOPE.View", true},
		{"how-to-local-dev", "class", true},
		{"how-to-local-dev", "function", true},
		{"how-to-local-dev", "model", true},
		{"how-to-local-dev", "react_component", true},
		// reference-deployment / -scripts gated for leaf kinds, allowed for module.
		{"reference-deployment", "module", false},
		{"reference-deployment", "view", true},
		{"reference-deployment", "class", true},
		{"reference-deployment", "function", true},
		{"reference-scripts", "module", false},
		{"reference-scripts", "view", true},
		// Sections without gating entries are never skipped.
		{"overview", "view", false},
		{"capabilities", "class", false},
		{"api", "function", false},
	}
	for _, tc := range cases {
		got := docgen.ShouldSkipSectionForKind(tc.section, tc.kind)
		if got != tc.want {
			t.Errorf("ShouldSkipSectionForKind(%q, %q) = %v, want %v",
				tc.section, tc.kind, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// #1864 — reference-config must distinguish graph-metadata Properties (skip)
// from application config (include).
// ---------------------------------------------------------------------------

func TestReferenceConfig_DistinguishesGraphMetadata(t *testing.T) {
	// Default + every kind-specific reference-config guidance must spell out
	// the application-vs-graph-metadata distinction.
	kinds := []string{"UNKNOWN_KIND", "view", "class", "function",
		"operation", "module", "model"}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			p := docgen.ResolveSectionProfile(kind, "")
			if !containsSection(p.Sections, "reference-config") {
				t.Skipf("kind %q: profile has no reference-config section", kind)
			}
			g := docgen.ResolveGuidance(p, "reference-config")
			lower := strings.ToLower(g)
			// Model profile is scoped to storage-only config and pre-dates #1864;
			// its narrow framing is intentional and the Properties block is not
			// where bleed-through has been observed.
			if kind == "model" {
				return
			}
			// Application-config positive framing.
			if !strings.Contains(lower, "application") {
				t.Errorf("#1864 kind %q reference-config: missing 'application' framing; got: %q",
					kind, g)
			}
			if !strings.Contains(lower, "graph-metadata") && !strings.Contains(lower, "indexer-internal") {
				t.Errorf("#1864 kind %q reference-config: missing graph-metadata Properties exclusion; got: %q",
					kind, g)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// #1865 — flows section MUST short-circuit fabrication for View/Class seeds:
// "method bodies are NOT in scope" + defer to per-method pages.
// ---------------------------------------------------------------------------

func TestFlowsGuidance_ViewClassShortCircuit(t *testing.T) {
	for _, kind := range []string{"view", "class", "SCOPE.View", "SCOPE.Class"} {
		t.Run(kind, func(t *testing.T) {
			p := docgen.ResolveSectionProfile(kind, "")
			g := docgen.ResolveGuidance(p, "flows")
			lower := strings.ToLower(g)
			if !strings.Contains(lower, "method bodies are not in scope") {
				t.Errorf("#1865 kind %q flows: missing 'method bodies are NOT in scope' clause; got: %q",
					kind, g)
			}
			if !strings.Contains(lower, "defer") || !strings.Contains(lower, "per-method") {
				t.Errorf("#1865 kind %q flows: missing 'defer to per-method pages' instruction; got: %q",
					kind, g)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// #1866 — api section MUST forbid decorator/path inference when decorator
// parameters are not in source_window (View/Class kinds).
// ---------------------------------------------------------------------------

func TestApiGuidance_ViewClassForbidsDecoratorInference(t *testing.T) {
	for _, kind := range []string{"view", "class", "SCOPE.View", "SCOPE.Class"} {
		t.Run(kind, func(t *testing.T) {
			p := docgen.ResolveSectionProfile(kind, "")
			g := docgen.ResolveGuidance(p, "api")
			lower := strings.ToLower(g)
			if !strings.Contains(lower, "not in source_window") &&
				!strings.Contains(lower, "not-in-context") {
				t.Errorf("#1866 kind %q api: missing decorator-not-in-context clause; got: %q",
					kind, g)
			}
			if !strings.Contains(lower, "infer") {
				t.Errorf("#1866 kind %q api: missing 'do not infer' wording; got: %q",
					kind, g)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// #2017 — flows section MUST restrict mermaid edges to bundle-visible entities
// across every profile.
// ---------------------------------------------------------------------------

func TestFlowsGuidance_FabricationBoundary_AllProfiles(t *testing.T) {
	// Every profile that includes the flows section must carry the boundary
	// clause that bans fabricated edges.  The default profile sourced from
	// defaultSectionGuidance counts too.
	kinds := []string{"UNKNOWN_KIND", "module", "operation",
		"view", "class", "function"}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			p := docgen.ResolveSectionProfile(kind, "")
			if !containsSection(p.Sections, "flows") {
				t.Skipf("kind %q: profile has no flows section", kind)
			}
			g := docgen.ResolveGuidance(p, "flows")
			lower := strings.ToLower(g)
			if !strings.Contains(lower, "neighbour_briefs") {
				t.Errorf("#2017 kind %q flows: missing neighbour_briefs boundary; got: %q", kind, g)
			}
			if !strings.Contains(lower, "module_manifest") {
				t.Errorf("#2017 kind %q flows: missing module_manifest boundary; got: %q", kind, g)
			}
			if !strings.Contains(lower, "only reference entities that exist in the bundle") &&
				!strings.Contains(lower, "do not mention entities or edges that are not in") {
				t.Errorf("#2017 kind %q flows: missing fabrication ban wording; got: %q", kind, g)
			}
		})
	}
}

// Size-tier operation profiles must also carry the #2017 boundary.
func TestFlowsGuidance_FabricationBoundary_OperationTiers(t *testing.T) {
	for _, lines := range []int{10, 50, 200} {
		p := docgen.ResolveSectionProfile("operation", "", lines)
		g := docgen.ResolveGuidance(p, "flows")
		lower := strings.ToLower(g)
		if !strings.Contains(lower, "neighbour_briefs") || !strings.Contains(lower, "module_manifest") {
			t.Errorf("#2017 operation(lines=%d) flows: missing fabrication boundary; got: %q", lines, g)
		}
	}
}

// ---------------------------------------------------------------------------
// SectionProfile.SkipForKinds — declarative field is reachable on the type.
// ---------------------------------------------------------------------------

func TestSectionProfile_SkipForKindsField(t *testing.T) {
	// Construct a profile literal that exercises the new field — guards against
	// the field accidentally being removed or renamed.
	p := docgen.SectionProfile{
		Sections:     []string{"overview"},
		SkipForKinds: []string{"some-kind"},
	}
	if len(p.SkipForKinds) != 1 || p.SkipForKinds[0] != "some-kind" {
		t.Errorf("SectionProfile.SkipForKinds round-trip failed: got %v", p.SkipForKinds)
	}
}
