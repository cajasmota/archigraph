package javascript_test

import (
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func jsExcEdge(recs []types.EntityRecord, fromName, kind, typeName string) bool {
	want := extreg.ExceptionTypeTargetID(typeName)
	for i := range recs {
		if recs[i].Name != fromName {
			continue
		}
		for _, r := range recs[i].Relationships {
			if r.Kind == kind && r.ToID == want {
				return true
			}
		}
	}
	return false
}

func jsExcNode(recs []types.EntityRecord, typeName string) (string, int) {
	want := extreg.ExceptionTypeName(typeName)
	id, count := "", 0
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == want {
			id = recs[i].ID
			count++
		}
	}
	return id, count
}

// TestJSExceptionFlow_ThrowInstanceofConverge: `throw new AuthError()` in one
// function and `e instanceof AuthError` catch in another converge on ONE node.
func TestJSExceptionFlow_ThrowInstanceofConverge(t *testing.T) {
	src := []byte(`function authenticate(token) {
  if (!token) {
    throw new AuthError("missing token");
  }
}

function middleware(req) {
  try {
    authenticate(req.token);
  } catch (e) {
    if (e instanceof AuthError) {
      return 401;
    }
    throw e;
  }
}
`)
	recs := extract(t, src, "javascript", parseJS(t, src))

	if !jsExcEdge(recs, "authenticate", "THROWS", "AuthError") {
		t.Errorf("missing THROWS(authenticate -> AuthError)")
	}
	if !jsExcEdge(recs, "middleware", "CATCHES", "AuthError") {
		t.Errorf("missing CATCHES(middleware -> AuthError)")
	}
	id, count := jsExcNode(recs, "AuthError")
	if id == "" {
		t.Fatal("no AuthError exception-type node")
	}
	if count != 1 {
		t.Fatalf("throw + instanceof-catch of AuthError must converge on ONE node, got %d", count)
	}
}

// TestJSExceptionFlow_BareError: `throw new Error()` records THROWS Error.
func TestJSExceptionFlow_BareError(t *testing.T) {
	src := []byte(`function boom() {
  throw new Error("kaboom");
}
`)
	recs := extract(t, src, "javascript", parseJS(t, src))
	if !jsExcEdge(recs, "boom", "THROWS", "Error") {
		t.Errorf("missing THROWS(boom -> Error)")
	}
}

// TestTSExceptionFlow_QualifiedThrow: `throw new errors.NotFound()` records the
// bare class NotFound (TypeScript source).
func TestTSExceptionFlow_QualifiedThrow(t *testing.T) {
	src := []byte(`function get(id: string): void {
  throw new errors.NotFound("x");
}
`)
	recs := extract(t, src, "typescript", parseTS(t, src))
	if !jsExcEdge(recs, "get", "THROWS", "NotFound") {
		t.Errorf("missing THROWS(get -> NotFound) for qualified new errors.NotFound")
	}
}

// TestJSExceptionFlow_UntypedThrowDropped: NEGATIVE — `throw err` (re-throw of a
// variable) and `throw {code}` (object literal) emit NO THROWS edge / node.
func TestJSExceptionFlow_UntypedThrowDropped(t *testing.T) {
	src := []byte(`function f(err) {
  throw err;
}
function g() {
  throw { code: 500 };
}
`)
	recs := extract(t, src, "javascript", parseJS(t, src))
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) {
			t.Errorf("untyped throw must not create a node: %q", recs[i].Name)
		}
		for _, r := range recs[i].Relationships {
			if r.Kind == "THROWS" {
				t.Errorf("untyped throw must emit no THROWS, got ToID=%q", r.ToID)
			}
		}
	}
}

// TestJSExceptionFlow_UntypedCatchDropped: NEGATIVE — a catch with no instanceof
// test emits NO CATCHES edge (TS catch bindings are untyped → uninformative).
func TestJSExceptionFlow_UntypedCatchDropped(t *testing.T) {
	src := []byte(`function f() {
  try {
    g();
  } catch (e) {
    console.log(e);
  }
}
`)
	recs := extract(t, src, "javascript", parseJS(t, src))
	for i := range recs {
		for _, r := range recs[i].Relationships {
			if r.Kind == "CATCHES" {
				t.Errorf("untyped catch must emit no CATCHES, got ToID=%q", r.ToID)
			}
		}
	}
}
