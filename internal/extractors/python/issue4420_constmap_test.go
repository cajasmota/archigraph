package python_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// Issue #4420 — module/class-level constant COLLECTIONS must be emitted as
// first-class, name-searchable SCOPE.Enum value-set entities carrying their
// {key,value} members as STRUCTURED, enumerable Properties (members_json), so a
// downstream cross-graph parity-audit can diff the literal set without
// re-parsing source.
//
// These tests run the REAL Python extract pipeline on a byte-copy of the live
// oracle source (core/permissions_config.py from upvate_core) — not a
// hand-written fixture — so a fixture that drifts from production cannot lie at
// merge time.

// memberEntry mirrors one structured member of the members_json property.
type memberEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Line  int    `json:"line"`
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

// TestIssue4420_RealPermissionPagesDict runs the live oracle file through the
// extractor and asserts (a) a name-searchable PERMISSION_PAGES value-set entity
// exists, and (b) its members enumerate the real key→value pairs — proving the
// `core-admin` hyphen literal is captured as structured data.
func TestIssue4420_RealPermissionPagesDict(t *testing.T) {
	src, err := os.ReadFile("testdata/issue4420/permissions_config.py")
	if err != nil {
		t.Fatalf("read oracle testdata: %v", err)
	}
	ents := extractPy(t, string(src), "core/permissions_config.py")

	en := findEnum(ents, "PERMISSION_PAGES")
	if en == nil {
		t.Fatal("SCOPE.Enum:PERMISSION_PAGES value-set node not found (RED before fix)")
	}
	if en.Kind != "SCOPE.Enum" {
		t.Fatalf("Kind = %q, want SCOPE.Enum", en.Kind)
	}
	if got := en.Properties["kind_hint"]; got != "python_const_map" {
		t.Fatalf("kind_hint = %q, want python_const_map", got)
	}

	members := parseMembersJSON(t, en)
	want := map[string]string{
		"CORE_ADMIN":         "core-admin",
		"CONTRACT_PROPOSALS": "contract-proposal",
		"USERS":              "users",
		"SYNC":               "sync",
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
	// CORE_ADMIN must carry a source line so a diff tool can locate it.
	for _, m := range members {
		if m.Key == "CORE_ADMIN" && m.Line <= 0 {
			t.Fatalf("CORE_ADMIN line = %d, want > 0", m.Line)
		}
	}
	if len(members) < 30 {
		t.Fatalf("expected the full PERMISSION_PAGES map (>=30 members), got %d", len(members))
	}
}

// TestIssue4420_PythonEnumGetsStructuredMembers asserts the structured
// members_json is also emitted for the pre-existing Enum / TextChoices / Literal
// shapes — the enrichment is general, not dict-specific.
func TestIssue4420_PythonEnumStructuredMembers(t *testing.T) {
	src := `
from enum import Enum

class Color(Enum):
    RED = "red"
    GREEN = "green"
`
	ents := extractPy(t, src, "app/color.py")
	en := findEnum(ents, "Color")
	if en == nil {
		t.Fatal("SCOPE.Enum:Color not found")
	}
	members := parseMembersJSON(t, en)
	if v, _ := memberValue(members, "RED"); v != "red" {
		t.Fatalf("RED value = %q, want red", v)
	}
}

// TestIssue4420_PythonTextChoicesMap covers Django TextChoices via the same
// structured-member contract.
func TestIssue4420_PythonTextChoices(t *testing.T) {
	src := `
from django.db import models

class Status(models.TextChoices):
    ACTIVE = "active", "Active"
    DONE = "done", "Done"
`
	ents := extractPy(t, src, "app/status.py")
	en := findEnum(ents, "Status")
	if en == nil {
		t.Fatal("SCOPE.Enum:Status (TextChoices) not found")
	}
	members := parseMembersJSON(t, en)
	if v, _ := memberValue(members, "ACTIVE"); v != "active" {
		t.Fatalf("ACTIVE value = %q, want active", v)
	}
}

// TestIssue4420_PythonLiteralAlias covers `Foo = Literal['a','b']`.
func TestIssue4420_PythonLiteralAlias(t *testing.T) {
	src := `
from typing import Literal

Mode = Literal["fast", "slow"]
`
	ents := extractPy(t, src, "app/mode.py")
	en := findEnum(ents, "Mode")
	if en == nil {
		t.Fatal("SCOPE.Enum:Mode (Literal) value-set not found")
	}
	if got := en.Properties["kind_hint"]; got != "python_literal" {
		t.Fatalf("kind_hint = %q, want python_literal", got)
	}
	members := parseMembersJSON(t, en)
	if v, _ := memberValue(members, "fast"); v != "fast" {
		t.Fatalf("literal arm fast value = %q, want fast", v)
	}
}

// TestIssue4420_NonConstMapNoNode is the honest-partial negative: a module
// dict whose values are non-literal expressions does NOT become a value-set.
func TestIssue4420_NonConstMapNoNode(t *testing.T) {
	src := `
HANDLERS = {
    "a": some_func,
    "b": other_func(),
}
`
	ents := extractPy(t, src, "app/handlers.py")
	if en := findEnum(ents, "HANDLERS"); en != nil {
		t.Fatalf("non-literal dict should not emit a value-set node, got %+v", en.Properties)
	}
}
