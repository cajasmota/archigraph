package javascript_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// Issue #4420 — TS/JS module-level constant COLLECTIONS (`const X = {...} as
// const`, `enum`, union-literal `type`) must be emitted as first-class,
// name-searchable SCOPE.Enum value-set entities carrying their {key,value}
// members as STRUCTURED, enumerable Properties (members_json).
//
// The headline test runs the REAL extract pipeline on a byte-copy of the live
// v3 mirror (src/common/auth/page/permission-page.ts) so the fixture cannot
// drift from production.

type memberEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Line  int    `json:"line"`
}

func findEnumJS(ents []types.EntityRecord, name string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Kind == "SCOPE.Enum" && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

func parseMembersJSON(t *testing.T, e *types.EntityRecord) []memberEntry {
	t.Helper()
	raw := e.Properties["members_json"]
	if raw == "" {
		t.Fatalf("entity %q has no members_json property; props=%v", e.Name, e.Properties)
	}
	var got []memberEntry
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("members_json not valid JSON: %v (raw=%s)", err, raw)
	}
	return got
}

func memberValue(members []memberEntry, key string) (string, bool) {
	for _, m := range members {
		if m.Key == key {
			return m.Value, true
		}
	}
	return "", false
}

// TestIssue4420_RealPermissionPageConstObject runs the live v3 file through the
// extractor and asserts the PermissionPage const-object is a searchable
// value-set whose members enumerate the real key→value pairs — proving the
// `core-admin` literal is captured as structured data.
func TestIssue4420_RealPermissionPageConstObject(t *testing.T) {
	src, err := os.ReadFile("testdata/issue4420/permission-page.ts")
	if err != nil {
		t.Fatalf("read v3 testdata: %v", err)
	}
	tree := parseTS(t, src)
	ents := runJSPath(t, string(src), "typescript", tree, "src/common/auth/page/permission-page.ts")

	en := findEnumJS(ents, "PermissionPage")
	if en == nil {
		t.Fatal("SCOPE.Enum:PermissionPage value-set node not found (RED before fix)")
	}
	if got := en.Properties["kind_hint"]; got != "ts_const_object" {
		t.Fatalf("kind_hint = %q, want ts_const_object", got)
	}
	members := parseMembersJSON(t, en)
	want := map[string]string{
		"CoreAdmin":         "core-admin",
		"ContractProposals": "contract-proposal",
		"Buildings":         "buildings",
		"DobSync":           "sync",
	}
	for k, v := range want {
		got, ok := memberValue(members, k)
		if !ok {
			t.Fatalf("member %q missing from members_json", k)
		}
		if got != v {
			t.Fatalf("member %q value = %q, want %q", k, got, v)
		}
	}
	for _, m := range members {
		if m.Key == "CoreAdmin" && m.Line <= 0 {
			t.Fatalf("CoreAdmin line = %d, want > 0", m.Line)
		}
	}
	if len(members) < 30 {
		t.Fatalf("expected the full PermissionPage map (>=30 members), got %d", len(members))
	}
}

// TestIssue4420_TSConstObjectStructured covers a small `as const` object.
func TestIssue4420_TSConstObjectStructured(t *testing.T) {
	src := `export const Color = { Red: 'red', Green: 'green' } as const;`
	tree := parseTS(t, []byte(src))
	ents := runJSPath(t, src, "typescript", tree, "color.ts")
	en := findEnumJS(ents, "Color")
	if en == nil {
		t.Fatal("SCOPE.Enum:Color const-object not found")
	}
	members := parseMembersJSON(t, en)
	if v, _ := memberValue(members, "Red"); v != "red" {
		t.Fatalf("Red value = %q, want red", v)
	}
}

// TestIssue4420_TSEnumStructuredMembers asserts the existing TS enum shape now
// also carries structured members_json (general enrichment).
func TestIssue4420_TSEnumStructuredMembers(t *testing.T) {
	src := `enum E { A = 'a', B = 'b' }`
	tree := parseTS(t, []byte(src))
	ents := runJSPath(t, src, "typescript", tree, "e.ts")
	en := findEnumJS(ents, "E")
	if en == nil {
		t.Fatal("SCOPE.Enum:E not found")
	}
	members := parseMembersJSON(t, en)
	if v, _ := memberValue(members, "A"); v != "a" {
		t.Fatalf("A value = %q, want a", v)
	}
}

// TestIssue4420_BareObjectNoNode is the honest-partial negative: a plain object
// literal NOT marked `as const` / `satisfies` is resolver state (issue #562),
// not a declared source-of-truth value-set, so it emits no SCOPE.Enum node.
func TestIssue4420_BareObjectNoNode(t *testing.T) {
	src := `export const ROUTES = { home: '/', login: '/login' };`
	tree := parseTS(t, []byte(src))
	ents := runJSPath(t, src, "typescript", tree, "routes.ts")
	if en := findEnumJS(ents, "ROUTES"); en != nil {
		t.Fatalf("bare object literal (no `as const`) must not emit a value-set, got %+v", en.Properties)
	}
}

// TestIssue4420_NonLiteralObjectNoNode is the honest-partial negative: an
// `as const` object whose values are non-literal expressions is not a closed
// enumerated value-set and emits no node.
func TestIssue4420_NonLiteralObjectNoNode(t *testing.T) {
	src := `const handlers = { a: doThing, b: other() } as const;`
	tree := parseTS(t, []byte(src))
	ents := runJSPath(t, src, "typescript", tree, "handlers.ts")
	if en := findEnumJS(ents, "handlers"); en != nil {
		t.Fatalf("non-literal const object should not emit a value-set, got %+v", en.Properties)
	}
}
