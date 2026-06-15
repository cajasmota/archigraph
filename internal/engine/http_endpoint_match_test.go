package engine

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

func TestNormalizeEndpointPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/users/{pk}", "/users/{*}"},
		{"/users/:id", "/users/{*}"},
		{"/users/<int:id>", "/users/{*}"},
		{"/Users/{userId}/", "/users/{*}"},
		{"/users/{userId}/posts/{postId}", "/users/{*}/posts/{*}"},
		{"/api/v1/inspections/{pk}/create-deficiencies", "/api/v1/inspections/{*}/create-deficiencies"},
		{"/inspections/{id}/create-deficiencies/", "/inspections/{*}/create-deficiencies"},
		{"/", "/"},
	}
	for _, c := range cases {
		if got := normalizeEndpointPath(c.in); got != c.want {
			t.Errorf("normalizeEndpointPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripEndpointAPIPrefix(t *testing.T) {
	cases := []struct {
		in       string
		want     string
		stripped bool
	}{
		{"/api/v1/inspections/{*}/create-deficiencies", "/inspections/{*}/create-deficiencies", true},
		{"/api/users", "/users", true},
		{"/v2/things", "/things", true},
		{"/api", "/", true},
		{"/inspections/{*}/create-deficiencies", "", false}, // no prefix
		{"/apiary/bees", "", false},                         // not a real api prefix segment
	}
	for _, c := range cases {
		got, ok := stripEndpointAPIPrefix(c.in)
		if ok != c.stripped || (ok && got != c.want) {
			t.Errorf("stripEndpointAPIPrefix(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.stripped)
		}
	}
}

func TestVerbsMatchCompat(t *testing.T) {
	yes := [][2]string{{"POST", "ANY"}, {"ANY", "GET"}, {"GET", "GET"}, {"", "POST"}, {"delete", "DELETE"}}
	no := [][2]string{{"DELETE", "PATCH"}, {"GET", "POST"}, {"PUT", "GET"}}
	for _, c := range yes {
		if !verbsMatchCompat(c[0], c[1]) {
			t.Errorf("verbsMatchCompat(%q,%q) = false, want true", c[0], c[1])
		}
	}
	for _, c := range no {
		if verbsMatchCompat(c[0], c[1]) {
			t.Errorf("verbsMatchCompat(%q,%q) = true, want false", c[0], c[1])
		}
	}
}

// TestResolveStructuralFallbackAndRetarget exercises the #1615 path: a call
// synthetic that differs from its definition on API prefix + param-token name +
// trailing slash + verb (ANY) must (a) resolve via the structural fallback and
// (b) have its inbound caller→call-synthetic FETCHES edge retargeted at the
// definition so it stops counting as an orphan.
func TestResolveStructuralFallbackAndRetarget(t *testing.T) {
	merged := []types.EntityRecord{
		// Producer handler + its definition synthetic (mounted under /api/v1, {pk}, ANY verb).
		{
			Kind: "SCOPE.Operation", Name: "InspectionViewSet.create_deficiency", SourceFile: "views.py",
		},
		{
			Kind: httpEndpointDefinitionKind,
			Name: "http:ANY:/api/v1/inspections/{pk}/create-deficiencies",
			Properties: map[string]string{
				"path": "/api/v1/inspections/{pk}/create-deficiencies", "verb": "ANY",
				"source_handler": "SCOPE.Operation:InspectionViewSet.create_deficiency",
			},
			SourceFile: "views.py",
		},
		// Consumer caller + its call synthetic (no prefix, {id}, trailing slash, POST verb).
		{
			Kind: "SCOPE.Operation", Name: "createDeficiencies", SourceFile: "api.ts",
		},
		{
			Kind: httpEndpointCallKind,
			Name: "http:POST:/inspections/{id}/create-deficiencies/",
			Properties: map[string]string{
				"path": "/inspections/{id}/create-deficiencies/", "verb": "POST",
				"source_caller": "SCOPE.Operation:createDeficiencies",
			},
			SourceFile: "api.ts",
		},
	}

	out, stats := ResolveHTTPEndpointHandlers(merged)

	if stats.CallsLinked < 1 {
		t.Fatalf("expected the call synthetic to resolve via structural fallback, calls_linked=%d", stats.CallsLinked)
	}
	if stats.CallerEdgesRetargeted < 1 {
		t.Fatalf("expected the inbound caller FETCHES edge to be retargeted, caller_edges_retargeted=%d", stats.CallerEdgesRetargeted)
	}

	// The caller entity's FETCHES edge must now point at the DEFINITION stub,
	// not the call-synthetic stub.
	defStub := httpEndpointDefinitionKind + ":http:ANY:/api/v1/inspections/{pk}/create-deficiencies"
	found := false
	for _, e := range out {
		if e.Name != "createDeficiencies" {
			continue
		}
		for _, rel := range e.Relationships {
			if string(rel.Kind) == fetchesEdgeKind && rel.ToID == defStub {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("caller createDeficiencies has no FETCHES edge retargeted to %q", defStub)
	}
}

// TestHTTPLinker_PrefixNormalization_ApiV1 exercises the #2547 prefix-injection
// tier: a frontend call synthetic that emits the raw path `/searchBuildings`
// (no API prefix) must resolve to a backend definition mounted at
// `/api/v1/searchBuildings`, and the resolved FETCHES edge must carry
// prefix_normalized="api/v1" so the match strategy is traceable.
//
// The test drives resolveCallByPath directly with a definitionByPath that has
// been pre-populated with ONLY the full-prefixed key (not the stripped alias),
// forcing tier 1 to miss and tier 2 to fire. This mirrors the real-world
// scenario where the definition index is built from a different repo's entities
// than the call synthetic — the intra-repo definitionByPath only contains the
// backend repo's entries; a frontend-only merged set has no backend definitions
// at all, making the per-call-path probe the only resolution avenue.
func TestHTTPLinker_PrefixNormalization_ApiV1(t *testing.T) {
	// Build the merged slice with only the definition (no call synthetic here;
	// we drive resolveCallByPath directly to isolate tier 2 behavior).
	definition := types.EntityRecord{
		Kind: httpEndpointDefinitionKind,
		Name: "http:GET:/api/v1/searchBuildings",
		Properties: map[string]string{
			"path": "/api/v1/searchBuildings",
			"verb": "GET",
		},
		SourceFile: "buildings/views.py",
	}
	merged := []types.EntityRecord{definition}

	// Populate definitionByPath with ONLY the prefixed key — no stripped alias.
	// This simulates the scenario where the stripped alias was not registered
	// (e.g., the definition's path property was populated after indexing, or
	// another extractor variant that doesn't run endpointMatchKeys).
	definitionByPath := map[string][]int{
		"/api/v1/searchbuildings": {0},
	}

	call := &types.EntityRecord{
		Kind: httpEndpointCallKind,
		Name: "http:GET:/searchBuildings",
		Properties: map[string]string{
			"path": "/searchBuildings",
			"verb": "GET",
		},
		SourceFile: "src/api/buildings.ts",
	}

	idx, prefixUsed, found := resolveCallByPath(call, merged, definitionByPath)
	if !found {
		t.Fatalf("expected prefix-injection to find the definition, got not-found")
	}
	if idx != 0 {
		t.Errorf("resolved index: want 0, got %d", idx)
	}
	if prefixUsed != "api/v1" {
		t.Errorf("prefixUsed: want %q, got %q", "api/v1", prefixUsed)
	}

	// Also verify end-to-end through ResolveHTTPEndpointHandlers: with the
	// definition path properly indexed (both full and stripped keys, as the
	// real indexer does), the call resolves and the edge is emitted. We do NOT
	// assert prefix_normalized here because tier 1 (structural) fires first
	// when both keys are present — that is the correct preference order (tier 1
	// is more precise, tier 2 is the fallback for missing stripped aliases).
	merged2 := []types.EntityRecord{
		{
			Kind: httpEndpointDefinitionKind,
			Name: "http:GET:/api/v1/searchBuildings",
			Properties: map[string]string{
				"path": "/api/v1/searchBuildings",
				"verb": "GET",
			},
			SourceFile: "buildings/views.py",
		},
		{
			Kind: httpEndpointCallKind,
			Name: "http:GET:/searchBuildings",
			Properties: map[string]string{
				"path": "/searchBuildings",
				"verb": "GET",
			},
			SourceFile: "src/api/buildings.ts",
		},
	}
	_, stats := ResolveHTTPEndpointHandlers(merged2)
	if stats.CallsLinked != 1 {
		t.Errorf("end-to-end: expected call to resolve, calls_linked=%d calls_unresolved=%d",
			stats.CallsLinked, stats.CallsUnresolved)
	}
}

// TestHTTPLinker_NoPrefixMatch_UnresolvedKept verifies the negative case (#2547):
// a frontend call to `/healthz` that has no matching backend definition — even
// after prefix-injection with all candidate prefixes — stays as an
// UNRESOLVED_FETCH orphan and is not incorrectly matched against an unrelated route.
func TestHTTPLinker_NoPrefixMatch_UnresolvedKept(t *testing.T) {
	merged := []types.EntityRecord{
		// Backend definition for an unrelated route — /healthz is not served here.
		{
			Kind: httpEndpointDefinitionKind,
			Name: "http:GET:/api/v1/buildings",
			Properties: map[string]string{
				"path": "/api/v1/buildings",
				"verb": "GET",
			},
			SourceFile: "buildings/views.py",
		},
		// Frontend call to /healthz — no backend serves this path even with prefixes.
		{
			Kind: httpEndpointCallKind,
			Name: "http:GET:/healthz",
			Properties: map[string]string{
				"path": "/healthz",
				"verb": "GET",
			},
			SourceFile: "src/health.ts",
		},
	}

	_, stats := ResolveHTTPEndpointHandlers(merged)

	if stats.CallsLinked != 0 {
		t.Errorf("expected no match for /healthz, calls_linked=%d", stats.CallsLinked)
	}
	if stats.CallsUnresolved != 1 {
		t.Errorf("expected /healthz to remain an orphan, calls_unresolved=%d", stats.CallsUnresolved)
	}
}

// TestResolveStructuralFallbackVerbGuard ensures a call with an incompatible
// specific verb does NOT resolve to a definition of a different specific verb
// (false-match guard).
func TestResolveStructuralFallbackVerbGuard(t *testing.T) {
	merged := []types.EntityRecord{
		{
			Kind:       httpEndpointDefinitionKind,
			Name:       "http:GET:/me-email-templates",
			Properties: map[string]string{"path": "/me-email-templates", "verb": "GET"},
			SourceFile: "views.py",
		},
		{
			Kind:       httpEndpointCallKind,
			Name:       "http:DELETE:/me-email-templates",
			Properties: map[string]string{"path": "/me-email-templates", "verb": "DELETE"},
			SourceFile: "api.ts",
		},
	}
	_, stats := ResolveHTTPEndpointHandlers(merged)
	if stats.CallsLinked != 0 {
		t.Errorf("DELETE call must not resolve to a GET-only definition, calls_linked=%d", stats.CallsLinked)
	}
	if stats.CallsUnresolved != 1 {
		t.Errorf("expected the DELETE call to stay unresolved, calls_unresolved=%d", stats.CallsUnresolved)
	}
}
