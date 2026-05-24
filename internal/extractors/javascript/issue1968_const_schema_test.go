// Package javascript — unit tests for issue #1968: top-level const declarations
// with primitive RHS must be emitted as SCOPE.Schema/constant entities with the
// literal value in Properties["value"], not swallowed or misclassified.
package javascript_test

import (
	"testing"
)

// --------------------------------------------------------------------------
// Basic regression — the W1R5 witness case from the issue
// --------------------------------------------------------------------------

// TestConst_PrimitiveString_ModuleScope — the exact witness from issue #1968:
//
//	export const PROPOSAL_COUNTS_QUERY_KEY = "proposalCounts"
//
// Must produce a SCOPE.Schema entity named PROPOSAL_COUNTS_QUERY_KEY with
// subtype="constant" and Properties["value"]="proposalCounts".
func TestConst_PrimitiveString_ModuleScope(t *testing.T) {
	src := []byte(`export const PROPOSAL_COUNTS_QUERY_KEY = "proposalCounts";`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	e := findByName(entities, "PROPOSAL_COUNTS_QUERY_KEY")
	if e == nil {
		t.Fatalf("PROPOSAL_COUNTS_QUERY_KEY entity not found; names: %v", entityNames(entities))
	}
	if e.Kind != "SCOPE.Schema" {
		t.Errorf("Kind=%q, want SCOPE.Schema", e.Kind)
	}
	if e.Subtype != "constant" {
		t.Errorf("Subtype=%q, want constant", e.Subtype)
	}
	if got := e.Properties["value"]; got != "proposalCounts" {
		t.Errorf("Properties[value]=%q, want %q", got, "proposalCounts")
	}
}

// TestConst_SingleQuoteString — single-quoted string literal.
func TestConst_SingleQuoteString(t *testing.T) {
	src := []byte(`const API_BASE = 'https://api.example.com';`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	e := findByName(entities, "API_BASE")
	if e == nil {
		t.Fatalf("API_BASE not found; names: %v", entityNames(entities))
	}
	if e.Kind != "SCOPE.Schema" || e.Subtype != "constant" {
		t.Errorf("Kind=%q Subtype=%q, want SCOPE.Schema/constant", e.Kind, e.Subtype)
	}
	if got := e.Properties["value"]; got != "https://api.example.com" {
		t.Errorf("Properties[value]=%q, want %q", got, "https://api.example.com")
	}
}

// TestConst_NumberLiteral — numeric literal.
func TestConst_NumberLiteral(t *testing.T) {
	src := []byte(`const MAX_RETRIES = 3;`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	e := findByName(entities, "MAX_RETRIES")
	if e == nil {
		t.Fatalf("MAX_RETRIES not found; names: %v", entityNames(entities))
	}
	if e.Kind != "SCOPE.Schema" || e.Subtype != "constant" {
		t.Errorf("Kind=%q Subtype=%q, want SCOPE.Schema/constant", e.Kind, e.Subtype)
	}
	if got := e.Properties["value"]; got != "3" {
		t.Errorf("Properties[value]=%q, want %q", got, "3")
	}
}

// TestConst_BooleanLiteral — boolean true/false.
func TestConst_BooleanLiteral(t *testing.T) {
	src := []byte(`const IS_DEV = true;`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	e := findByName(entities, "IS_DEV")
	if e == nil {
		t.Fatalf("IS_DEV not found; names: %v", entityNames(entities))
	}
	if e.Kind != "SCOPE.Schema" || e.Subtype != "constant" {
		t.Errorf("Kind=%q Subtype=%q, want SCOPE.Schema/constant", e.Kind, e.Subtype)
	}
	if got := e.Properties["value"]; got != "true" {
		t.Errorf("Properties[value]=%q, want %q", got, "true")
	}
}

// TestConst_NullLiteral — null value.
func TestConst_NullLiteral(t *testing.T) {
	src := []byte(`const SENTINEL = null;`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	e := findByName(entities, "SENTINEL")
	if e == nil {
		t.Fatalf("SENTINEL not found; names: %v", entityNames(entities))
	}
	if e.Kind != "SCOPE.Schema" || e.Subtype != "constant" {
		t.Errorf("Kind=%q Subtype=%q, want SCOPE.Schema/constant", e.Kind, e.Subtype)
	}
}

// TestConst_PrimitiveString_TypeScript — same as the JS case but parsed via
// the TypeScript grammar to confirm both paths work.
func TestConst_PrimitiveString_TypeScript(t *testing.T) {
	src := []byte(`export const QUERY_KEY = "queryKey";`)
	tree := parseTS(t, src)
	entities := extract(t, src, "typescript", tree)

	e := findByName(entities, "QUERY_KEY")
	if e == nil {
		t.Fatalf("QUERY_KEY not found; names: %v", entityNames(entities))
	}
	if e.Kind != "SCOPE.Schema" || e.Subtype != "constant" {
		t.Errorf("Kind=%q Subtype=%q, want SCOPE.Schema/constant", e.Kind, e.Subtype)
	}
	if got := e.Properties["value"]; got != "queryKey" {
		t.Errorf("Properties[value]=%q, want %q", got, "queryKey")
	}
}

// --------------------------------------------------------------------------
// Non-regression — object/array/call-RHS consts MUST NOT be emitted as
// SCOPE.Schema/constant (they are handled elsewhere or deliberately excluded).
// --------------------------------------------------------------------------

// TestConst_ObjectLiteral_NotEmittedAsConstant — `const config = {...}` must
// NOT be emitted as SCOPE.Schema/constant.
func TestConst_ObjectLiteral_NotEmittedAsConstant(t *testing.T) {
	src := []byte(`const config = { host: "localhost", port: 8080 };`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	e := findByName(entities, "config")
	if e != nil && e.Kind == "SCOPE.Schema" && e.Subtype == "constant" {
		t.Errorf("config should NOT be SCOPE.Schema/constant; got Kind=%q Subtype=%q", e.Kind, e.Subtype)
	}
}

// TestConst_ArrowFunction_NotConstant — arrow functions must still be SCOPE.Operation.
func TestConst_ArrowFunction_NotConstant(t *testing.T) {
	src := []byte(`const greet = (name) => "hello " + name;`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	e := findByName(entities, "greet")
	if e == nil {
		t.Fatalf("greet not found; names: %v", entityNames(entities))
	}
	if e.Kind != "SCOPE.Operation" {
		t.Errorf("greet Kind=%q, want SCOPE.Operation", e.Kind)
	}
}

// --------------------------------------------------------------------------
// Scope guard — primitive consts inside function bodies must NOT be emitted
// as SCOPE.Schema/constant (they stay as local_scope entities or absent).
// --------------------------------------------------------------------------

// TestConst_PrimitiveInsideFunction_NotSchema — primitive const inside a
// function body must NOT become SCOPE.Schema/constant. It should either not
// be emitted, or if emitted, must have local_scope=true.
func TestConst_PrimitiveInsideFunction_NotSchema(t *testing.T) {
	src := []byte(`
function setup() {
  const TIMEOUT = 5000;
  return TIMEOUT;
}
`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	e := findByName(entities, "TIMEOUT")
	if e != nil {
		// If emitted at all, it must NOT be Schema/constant (it's a local).
		if e.Kind == "SCOPE.Schema" && e.Subtype == "constant" {
			t.Errorf("TIMEOUT inside function must NOT be SCOPE.Schema/constant; got Kind=%q Subtype=%q", e.Kind, e.Subtype)
		}
	}
}

// --------------------------------------------------------------------------
// Multiple constants in one file — all should be extracted independently.
// --------------------------------------------------------------------------

func TestConst_MultipleInFile(t *testing.T) {
	src := []byte(`
export const FOO = "foo";
export const BAR = 42;
export const BAZ = false;
`)
	tree := parseJS(t, src)
	entities := extract(t, src, "javascript", tree)

	for _, tc := range []struct {
		name  string
		value string
	}{
		{"FOO", "foo"},
		{"BAR", "42"},
		{"BAZ", "false"},
	} {
		e := findByName(entities, tc.name)
		if e == nil {
			t.Errorf("%s entity not found; names: %v", tc.name, entityNames(entities))
			continue
		}
		if e.Kind != "SCOPE.Schema" || e.Subtype != "constant" {
			t.Errorf("%s: Kind=%q Subtype=%q, want SCOPE.Schema/constant", tc.name, e.Kind, e.Subtype)
		}
		if got := e.Properties["value"]; got != tc.value {
			t.Errorf("%s: Properties[value]=%q, want %q", tc.name, got, tc.value)
		}
	}
}
