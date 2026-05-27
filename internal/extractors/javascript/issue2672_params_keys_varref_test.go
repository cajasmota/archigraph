// Package javascript_test — issue #2672: params_keys variable-ref resolution.
// Extends #2665 by resolving variable references in params arguments to their
// object literal bindings within the same file, extracting keys for var-ref params.
package javascript_test

import (
	"testing"
)

// TestNavParamsKeys_VarRef_SimpleBinding verifies that a simple variable
// reference to an object literal binding is resolved and keys extracted.
//
// const p = {a, b}; router.push({pathname: '/x', params: p}) → params_keys=[a,b]
func TestNavParamsKeys_VarRef_SimpleBinding(t *testing.T) {
	src := `
import { useRouter } from "expo-router";
const Comp = () => {
  const router = useRouter();
  const paramsObj = {id, name};
  const go = () => {
    router.push({pathname: '/detail', params: paramsObj});
  };
  return null;
};
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)
	raw := findNavParamsKeys(ents, "go")
	if raw == "" {
		t.Fatalf("expected params_keys for variable-ref params, got empty")
	}
	got := decodeParamsKeys(t, raw)
	want := []string{"id", "name"}
	if len(got) != len(want) {
		t.Fatalf("params_keys: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("params_keys[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestNavParamsKeys_VarRef_WithSpread verifies that spread elements in a
// variable-ref binding are recorded as the ...spread sentinel.
//
// const p = {id, ...rest}; router.push({pathname: '/x', params: p})
// → params_keys=[id, ...spread]
func TestNavParamsKeys_VarRef_WithSpread(t *testing.T) {
	src := `
import { useRouter } from "expo-router";
const Comp = () => {
  const router = useRouter();
  const paramsObj = {inspectionId, ...otherParams};
  const go = () => {
    router.push({pathname: '/inspection', params: paramsObj});
  };
  return null;
};
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)
	raw := findNavParamsKeys(ents, "go")
	if raw == "" {
		t.Fatalf("expected params_keys for variable-ref params, got empty")
	}
	got := decodeParamsKeys(t, raw)
	foundID := false
	foundSpread := false
	for _, k := range got {
		if k == "inspectionId" {
			foundID = true
		}
		if k == "...spread" {
			foundSpread = true
		}
	}
	if !foundID {
		t.Errorf("expected 'inspectionId' key in params_keys; got %v", got)
	}
	if !foundSpread {
		t.Errorf("expected '...spread' sentinel in params_keys; got %v", got)
	}
}

// TestNavParamsKeys_VarRef_NonLiteral verifies that variable references to
// non-literal bindings (function calls, spreads, etc.) leave params_keys empty.
//
// const p = getParams(); router.push({pathname: '/x', params: p}) → params_keys=[]
func TestNavParamsKeys_VarRef_NonLiteral(t *testing.T) {
	src := `
import { useRouter } from "expo-router";
const getParams = () => ({a: 1});
const Comp = () => {
  const router = useRouter();
  const paramsObj = getParams();
  const go = () => {
    router.push({pathname: '/detail', params: paramsObj});
  };
  return null;
};
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)
	raw := findNavParamsKeys(ents, "go")
	// paramsObj = getParams() is not an object literal, so it can't be resolved.
	// Expected: params_keys=[] (empty array, indicating dynamic/unresolvable)
	if raw != "[]" {
		t.Fatalf("expected params_keys=[] for non-literal binding, got %q", raw)
	}
}

// TestNavParamsKeys_VarRef_Undefined verifies that references to undefined
// variables are handled gracefully (leave empty).
//
// router.push({pathname: '/x', params: undefinedVar}) → params_keys=[]
func TestNavParamsKeys_VarRef_Undefined(t *testing.T) {
	src := `
import { useRouter } from "expo-router";
const Comp = () => {
  const router = useRouter();
  const go = () => {
    router.push({pathname: '/detail', params: undefinedVar});
  };
  return null;
};
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)
	raw := findNavParamsKeys(ents, "go")
	// undefinedVar has no binding, so resolution fails gracefully.
	// Expected: params_keys=[] (or no resolution, stays empty)
	if raw != "[]" && raw != "" {
		t.Fatalf("expected params_keys=[] or empty for undefined var, got %q", raw)
	}
}

// TestNavParamsKeys_VarRef_Reassigned verifies that variable reassignments
// use the most recent binding (last-in-scope semantics).
//
// const p = {a}; ... p = {b, c}; router.push({params: p})
// → should use the second binding {b, c}, not the first {a}
func TestNavParamsKeys_VarRef_Reassigned(t *testing.T) {
	src := `
import { useRouter } from "expo-router";
const Comp = () => {
  const router = useRouter();
  let paramsObj = {oldKey};
  paramsObj = {newKey1, newKey2};
  const go = () => {
    router.push({pathname: '/detail', params: paramsObj});
  };
  return null;
};
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)
	raw := findNavParamsKeys(ents, "go")
	if raw == "" {
		t.Fatalf("expected params_keys for reassigned variable, got empty")
	}
	got := decodeParamsKeys(t, raw)
	// Should pick the SECOND binding (last-in-scope), so {newKey1, newKey2}.
	// We expect newKey1 and newKey2, not oldKey.
	hasNewKey1 := false
	hasNewKey2 := false
	hasOldKey := false
	for _, k := range got {
		if k == "newKey1" {
			hasNewKey1 = true
		}
		if k == "newKey2" {
			hasNewKey2 = true
		}
		if k == "oldKey" {
			hasOldKey = true
		}
	}
	if !hasNewKey1 || !hasNewKey2 {
		t.Errorf("expected {newKey1, newKey2} from second binding; got %v", got)
	}
	if hasOldKey {
		t.Errorf("should use last binding, not first; got oldKey in %v", got)
	}
}

// TestNavParamsKeys_VarRef_BlockScoped verifies that variable bindings within
// nested blocks are resolved correctly (closest visible binding wins).
func TestNavParamsKeys_VarRef_BlockScoped(t *testing.T) {
	src := `
import { useRouter } from "expo-router";
const Comp = () => {
  const router = useRouter();
  const parentParams = {parent: 1};
  const go = () => {
    const localParams = {local: 2};
    router.push({pathname: '/detail', params: localParams});
  };
  return null;
};
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)
	raw := findNavParamsKeys(ents, "go")
	if raw == "" {
		t.Fatalf("expected params_keys for block-scoped variable, got empty")
	}
	got := decodeParamsKeys(t, raw)
	// Should resolve to localParams (the most recent binding before the push).
	hasLocal := false
	for _, k := range got {
		if k == "local" {
			hasLocal = true
		}
	}
	if !hasLocal {
		t.Errorf("expected 'local' key from block-scoped binding; got %v", got)
	}
}

// TestNavParamsKeys_VarRef_RegexssionDirectLiteral verifies that direct object
// literals (non-variable-ref) still work after the variable-ref changes.
func TestNavParamsKeys_VarRef_RegressionDirectLiteral(t *testing.T) {
	src := `
import { useRouter } from "expo-router";
const Comp = () => {
  const router = useRouter();
  const go = () => {
    router.push({pathname: '/detail', params: {direct, literal, keys}});
  };
  return null;
};
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)
	raw := findNavParamsKeys(ents, "go")
	if raw == "" {
		t.Fatalf("expected params_keys for direct literal, got empty")
	}
	got := decodeParamsKeys(t, raw)
	want := []string{"direct", "keys", "literal"} // sorted
	if len(got) != len(want) {
		t.Fatalf("params_keys: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("params_keys[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestNavParamsKeys_VarRef_ExplicitKeyValue verifies that key-value pairs in
// variable-ref bindings are handled correctly (keys extracted, values ignored).
func TestNavParamsKeys_VarRef_ExplicitKeyValue(t *testing.T) {
	src := `
import { useRouter } from "expo-router";
const Comp = () => {
  const router = useRouter();
  const paramsObj = {id: 123, type: 'view', mode: 'edit'};
  const go = () => {
    router.push({pathname: '/detail', params: paramsObj});
  };
  return null;
};
`
	tree := parseTSRel(t, []byte(src))
	ents := runJS(t, src, "typescript", tree)
	raw := findNavParamsKeys(ents, "go")
	if raw == "" {
		t.Fatalf("expected params_keys for key-value binding, got empty")
	}
	got := decodeParamsKeys(t, raw)
	want := []string{"id", "mode", "type"} // sorted
	if len(got) != len(want) {
		t.Fatalf("params_keys: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("params_keys[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
