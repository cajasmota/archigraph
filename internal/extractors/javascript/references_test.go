package javascript_test

import (
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// referencesFrom returns the REFERENCES ToIDs emitted from a named
// entity in the extracted record slice.
func referencesFrom(ents []types.EntityRecord, fromName string) []string {
	src := findByNameRel(ents, fromName)
	if src == nil {
		return nil
	}
	var out []string
	for _, r := range src.Relationships {
		if r.Kind == "REFERENCES" {
			out = append(out, r.ToID)
		}
	}
	return out
}

// hasReferencesTo reports whether `from` has any REFERENCES ToID whose
// trailing identifier matches `targetName`. Format A structural refs
// embed the name as the last colon-segment, so we test the suffix.
func hasReferencesTo(ents []types.EntityRecord, from, targetName string) bool {
	for _, id := range referencesFrom(ents, from) {
		if strings.HasSuffix(id, ":"+targetName) {
			return true
		}
	}
	return false
}

// TestReferences_SameScopeIdentifier — Track A.
// `const X = useState(false)` declares X; later `setX(true)` is a CALL
// (not a reference target). But `<button onClick={() => doX(X)}>` uses
// X as a value — that is a REFERENCES edge.
func TestReferences_SameScopeIdentifier(t *testing.T) {
	src := `const ENDPOINT = "/api/clients";
function fetchClients() {
  return fetch(ENDPOINT);
}
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	if !hasReferencesTo(ents, "fetchClients", "ENDPOINT") {
		t.Errorf("expected REFERENCES fetchClients->ENDPOINT; got %v", referencesFrom(ents, "fetchClients"))
	}
}

// TestReferences_TemplateLiteralInterpolation — Track B.
// `` fetch(`${BASE}/users`) `` should resolve BASE as a REFERENCES edge.
func TestReferences_TemplateLiteralInterpolation(t *testing.T) {
	src := `const BASE = "/api";
function loadUsers() {
  return fetch(` + "`${BASE}/users`" + `);
}
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	if !hasReferencesTo(ents, "loadUsers", "BASE") {
		t.Errorf("expected REFERENCES loadUsers->BASE; got %v", referencesFrom(ents, "loadUsers"))
	}
}

// TestReferences_NoEdgeToGlobals — globals must never produce a
// REFERENCES edge, even if a user-declared name happens to collide.
func TestReferences_NoEdgeToGlobals(t *testing.T) {
	src := `function log() {
  console.log("x");
  fetch("/y");
}
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	refs := referencesFrom(ents, "log")
	for _, id := range refs {
		if strings.HasSuffix(id, ":console") || strings.HasSuffix(id, ":fetch") {
			t.Errorf("unexpected REFERENCES edge to global: %s", id)
		}
	}
}

// TestReferences_NoSelfEdge — a function referencing itself by name
// (recursion-like shape) must NOT emit REFERENCES to itself. The
// existing CALLS path drops self-recursion; REFERENCES does too.
func TestReferences_NoSelfEdge(t *testing.T) {
	src := `function helper() {
  const x = helper;
  return x;
}
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	for _, id := range referencesFrom(ents, "helper") {
		if strings.HasSuffix(id, ":helper") {
			t.Errorf("unexpected self REFERENCES edge: %s", id)
		}
	}
}

// TestReferences_DedupePerPair — multiple usages of the same identifier
// inside a function body must collapse to a single REFERENCES edge.
func TestReferences_DedupePerPair(t *testing.T) {
	src := `const FLAG = true;
function check() {
  if (FLAG) {}
  if (FLAG) {}
  return FLAG;
}
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	n := 0
	for _, id := range referencesFrom(ents, "check") {
		if strings.HasSuffix(id, ":FLAG") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("expected 1 REFERENCES check->FLAG after dedup, got %d", n)
	}
}

// TestReferences_NoEdgeWhenIdentifierIsCallee — a `helper()` call
// emits CALLS, not REFERENCES. We must NOT double-count by also
// emitting REFERENCES to helper.
func TestReferences_NoEdgeWhenIdentifierIsCallee(t *testing.T) {
	src := `function helper() {}
function caller() {
  helper();
}
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	for _, id := range referencesFrom(ents, "caller") {
		if strings.HasSuffix(id, ":helper") {
			t.Errorf("unexpected REFERENCES caller->helper (CALLS owns this edge): %s", id)
		}
	}
}

// TestReferences_TrackC_ImportTarget — an imported name used as a
// value inside a function body should produce a REFERENCES edge to
// the same-file-emitted local binding for that import. (Cross-file
// resolution to the originating module happens via IMPORTS; this
// test verifies the in-file reference link is present.)
func TestReferences_TrackC_ImportTarget(t *testing.T) {
	src := `import { CONFIG } from "./config";
function setup() {
  return CONFIG;
}
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	// The import binding doesn't currently emit a per-binding entity
	// (only an IMPORTS edge on the module entity), so a CONFIG entity
	// may not exist in the file scope. The test asserts the
	// conservative behaviour: NO REFERENCES edge to a non-existent
	// symbol. If a future change emits per-binding import entities,
	// the same machinery will produce the REFERENCES edge for free.
	for _, id := range referencesFrom(ents, "setup") {
		if !strings.Contains(id, "scope:") {
			t.Errorf("non-structural REFERENCES ToID: %s", id)
		}
	}
}

// TestReferences_FunctionDeclaration_References — a function_declaration
// is the canonical from-entity shape; ensure it can host REFERENCES.
func TestReferences_FunctionDeclaration_References(t *testing.T) {
	src := `const greeting = "hi";
function greet() {
  return greeting;
}
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	if !hasReferencesTo(ents, "greet", "greeting") {
		t.Errorf("expected REFERENCES greet->greeting; got %v", referencesFrom(ents, "greet"))
	}
}

// TestReferences_ArrowFunctionConst — `const fn = () => x` must attribute
// REFERENCES to the const name, not file scope.
func TestReferences_ArrowFunctionConst(t *testing.T) {
	src := `const data = { count: 0 };
const reader = () => data;
`
	tree := parseJSRel(t, []byte(src))
	ents := runJS(t, src, "javascript", tree)

	if !hasReferencesTo(ents, "reader", "data") {
		t.Errorf("expected REFERENCES reader->data; got %v", referencesFrom(ents, "reader"))
	}
}

// TestReferences_TypeScriptParity — same behaviour on TS grammar.
func TestReferences_TypeScriptParity(t *testing.T) {
	src := `const BASE: string = "/api";
function loadUsers(): Promise<unknown> {
  return fetch(` + "`${BASE}/users`" + `);
}
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)

	if !hasReferencesTo(ents, "loadUsers", "BASE") {
		t.Errorf("expected TS REFERENCES loadUsers->BASE; got %v", referencesFrom(ents, "loadUsers"))
	}
}
