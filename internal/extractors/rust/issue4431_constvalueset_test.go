package rust_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// Issue #4431 — Rust const/static constant COLLECTIONS (const slice maps,
// phf_map! static maps, lazy_static! HashMap builds, module constant groups) and
// data-enums must be emitted as first-class, name-searchable SCOPE.Enum
// value-set entities carrying their {key,value} members as STRUCTURED,
// enumerable Properties (members_json), reusing the shared cross-language
// builder (#4420/#4429). A downstream cross-graph parity-audit can then diff the
// Rust permission map against the Django PERMISSION_PAGES / v3 PermissionPage
// oracles without re-parsing source.
//
// These tests run the REAL Rust extract pipeline on a byte-copy of a
// representative fixture, asserting (a) the value-set is emitted, (b) it is
// searchable by name, and (c) its members enumerate the real key→value pairs.

type rustMember struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Line  int    `json:"line"`
}

func rustParseMembers(t *testing.T, e *types.EntityRecord) []rustMember {
	t.Helper()
	raw := e.Properties["members_json"]
	if raw == "" {
		t.Fatalf("entity %q has no members_json; props=%v", e.Name, e.Properties)
	}
	var got []rustMember
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("members_json not valid JSON: %v (raw=%s)", err, raw)
	}
	return got
}

func rustMemberValue(members []rustMember, key string) (string, bool) {
	for _, m := range members {
		if m.Key == key {
			return m.Value, true
		}
	}
	return "", false
}

// findValueSet returns the SCOPE.Enum value-set entity with the given name (the
// name-searchable view), or nil. It deliberately matches Kind=SCOPE.Enum so it
// does not collide with the SCOPE.Component:enum the walk also emits.
func findValueSet(ents []types.EntityRecord, name string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Kind == "SCOPE.Enum" && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

func loadRustFixture(t *testing.T) []types.EntityRecord {
	t.Helper()
	src, err := os.ReadFile("testdata/issue4431/permissions.rs")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return extractRust(t, "core/permissions.rs", string(src))
}

// TestIssue4431_ConstSliceMap asserts the `const X: &[(&str,&str)] = &[...]`
// permission table is a name-searchable value-set enumerating its key→value
// pairs (the `core-admin` hyphen literal captured as structured data).
func TestIssue4431_ConstSliceMap(t *testing.T) {
	ents := loadRustFixture(t)

	vs := findValueSet(ents, "PERMISSION_PAGES")
	if vs == nil {
		t.Fatal("SCOPE.Enum:PERMISSION_PAGES value-set not found (RED before fix)")
	}
	if got := vs.Properties["kind_hint"]; got != "rust_const_slice_map" {
		t.Fatalf("kind_hint = %q, want rust_const_slice_map", got)
	}
	members := rustParseMembers(t, vs)
	want := map[string]string{
		"core-admin":        "Core Admin",
		"contract-proposal": "Contract Proposals",
		"users":             "Users",
		"sync":              "Sync",
	}
	for k, v := range want {
		got, ok := rustMemberValue(members, k)
		if !ok {
			t.Fatalf("member %q missing from members_json", k)
		}
		if got != v {
			t.Fatalf("member %q = %q, want %q", k, got, v)
		}
	}
	for _, m := range members {
		if m.Key == "core-admin" && m.Line <= 0 {
			t.Fatalf("core-admin line = %d, want > 0", m.Line)
		}
	}
}

// TestIssue4431_PhfMap asserts a `static X: phf::Map = phf_map! { "k" => "v" }`
// compile-time map is indexed as a value-set.
func TestIssue4431_PhfMap(t *testing.T) {
	ents := loadRustFixture(t)

	vs := findValueSet(ents, "PAGE_LABELS")
	if vs == nil {
		t.Fatal("SCOPE.Enum:PAGE_LABELS phf_map value-set not found")
	}
	if got := vs.Properties["kind_hint"]; got != "rust_phf_map" {
		t.Fatalf("kind_hint = %q, want rust_phf_map", got)
	}
	members := rustParseMembers(t, vs)
	want := map[string]string{
		"core-admin": "Core Admin",
		"billing":    "Billing",
		"users":      "Users",
	}
	for k, v := range want {
		got, ok := rustMemberValue(members, k)
		if !ok {
			t.Fatalf("phf member %q missing", k)
		}
		if got != v {
			t.Fatalf("phf member %q = %q, want %q", k, got, v)
		}
	}
}

// TestIssue4431_LazyStaticMap asserts a `lazy_static! { static ref X = { ..
// inserts .. } }` HashMap build is indexed as a value-set from its insert calls.
func TestIssue4431_LazyStaticMap(t *testing.T) {
	ents := loadRustFixture(t)

	vs := findValueSet(ents, "ROUTE_TABLE")
	if vs == nil {
		t.Fatal("SCOPE.Enum:ROUTE_TABLE lazy_static value-set not found")
	}
	if got := vs.Properties["kind_hint"]; got != "rust_lazy_map" {
		t.Fatalf("kind_hint = %q, want rust_lazy_map", got)
	}
	members := rustParseMembers(t, vs)
	want := map[string]string{
		"home":    "/",
		"admin":   "/admin",
		"billing": "/billing",
	}
	for k, v := range want {
		got, ok := rustMemberValue(members, k)
		if !ok {
			t.Fatalf("lazy member %q missing", k)
		}
		if got != v {
			t.Fatalf("lazy member %q = %q, want %q", k, got, v)
		}
	}
}

// TestIssue4431_DataEnum asserts a Rust data-enum is indexed as a value-set:
// the explicit discriminant carries its value, the data-carrying variant
// records the name only (honest-partial).
func TestIssue4431_DataEnum(t *testing.T) {
	ents := loadRustFixture(t)

	vs := findValueSet(ents, "AccessLevel")
	if vs == nil {
		t.Fatal("SCOPE.Enum:AccessLevel data-enum value-set not found")
	}
	if got := vs.Properties["kind_hint"]; got != "rust_enum" {
		t.Fatalf("kind_hint = %q, want rust_enum", got)
	}
	members := rustParseMembers(t, vs)
	if v, ok := rustMemberValue(members, "Read"); !ok || v != "1" {
		t.Fatalf("Read discriminant = %q ok=%v, want 1", v, ok)
	}
	if v, ok := rustMemberValue(members, "None"); !ok || v != "0" {
		t.Fatalf("None discriminant = %q ok=%v, want 0", v, ok)
	}
	// Data-carrying variant: name recorded, value empty (honest-partial).
	if v, ok := rustMemberValue(members, "Custom"); !ok {
		t.Fatal("Custom variant missing from value-set")
	} else if v != "" {
		t.Fatalf("Custom (data-carrying) value = %q, want empty", v)
	}
}

// TestIssue4431_ModuleConstGroup asserts the loose module-level scalar
// const/static literals are aggregated into ONE synthetic value-set.
func TestIssue4431_ModuleConstGroup(t *testing.T) {
	ents := loadRustFixture(t)

	vs := findValueSet(ents, "PermissionsConstants")
	if vs == nil {
		t.Fatal("SCOPE.Enum:PermissionsConstants module-constant group not found")
	}
	if got := vs.Properties["kind_hint"]; got != "rust_module_constants" {
		t.Fatalf("kind_hint = %q, want rust_module_constants", got)
	}
	members := rustParseMembers(t, vs)
	want := map[string]string{
		"MAX_PERMISSIONS": "256",
		"DEFAULT_SCOPE":   "core",
		"SERVICE_NAME":    "acme",
	}
	for k, v := range want {
		got, ok := rustMemberValue(members, k)
		if !ok {
			t.Fatalf("module const %q missing", k)
		}
		if got != v {
			t.Fatalf("module const %q = %q, want %q", k, got, v)
		}
	}
}

// TestIssue4431_ValueSetDoesNotReplaceComponent asserts the supplemental
// value-set pass is append-only: the struct/enum SCOPE.Component entities the
// walk emits are still present alongside the new SCOPE.Enum value-sets.
func TestIssue4431_ValueSetDoesNotReplaceComponent(t *testing.T) {
	ents := loadRustFixture(t)

	if !hasComponent(ents, "enum", "AccessLevel") {
		t.Fatal("SCOPE.Component:enum AccessLevel missing — value-set pass must be additive")
	}
	if findValueSet(ents, "AccessLevel") == nil {
		t.Fatal("SCOPE.Enum:AccessLevel value-set missing alongside the Component")
	}
}
