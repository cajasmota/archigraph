package python_test

import (
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// excEdge reports whether the entity Named fromName has a relationship of the
// given kind whose ToID targets the exception type.
func excEdge(recs []types.EntityRecord, fromName, kind, typeName string) bool {
	want := extractor.ExceptionTypeTargetID(typeName)
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

func excNodeID(recs []types.EntityRecord, typeName string) string {
	want := extractor.ExceptionTypeName(typeName)
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == want {
			return recs[i].ID
		}
	}
	return ""
}

// TestPyExceptionFlow_RaiseCatchConverge: `raise NotFound()` in get_user and
// `except NotFound:` in handler must produce THROWS + CATCHES edges that
// converge on the SAME exception-type node — the value of the capability.
func TestPyExceptionFlow_RaiseCatchConverge(t *testing.T) {
	src := `class NotFound(Exception):
    pass

def get_user(uid):
    raise NotFound("missing")

def handler(uid):
    try:
        return get_user(uid)
    except NotFound:
        return None
`
	recs := extractPy(t, src, "svc.py")

	if !excEdge(recs, "get_user", "THROWS", "NotFound") {
		t.Errorf("missing THROWS(get_user -> NotFound)")
	}
	if !excEdge(recs, "handler", "CATCHES", "NotFound") {
		t.Errorf("missing CATCHES(handler -> NotFound)")
	}

	// Convergence: exactly one NotFound node, and both edges point at its ID.
	id := excNodeID(recs, "NotFound")
	if id == "" {
		t.Fatal("no NotFound exception-type node emitted")
	}
	count := 0
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == "exception:NotFound" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("THROWS + CATCHES of NotFound must converge on ONE node, got %d", count)
	}
}

// TestPyExceptionFlow_MultiExcept: `except (ValueError, KeyError):` yields a
// CATCHES edge for each named type.
func TestPyExceptionFlow_MultiExcept(t *testing.T) {
	src := `def parse(s):
    try:
        return int(s)
    except (ValueError, KeyError):
        return 0
`
	recs := extractPy(t, src, "p.py")
	if !excEdge(recs, "parse", "CATCHES", "ValueError") {
		t.Errorf("missing CATCHES(parse -> ValueError)")
	}
	if !excEdge(recs, "parse", "CATCHES", "KeyError") {
		t.Errorf("missing CATCHES(parse -> KeyError)")
	}
}

// TestPyExceptionFlow_QualifiedRaise: `raise errors.NotFound()` records the
// bare class name (last dotted segment), matching `except NotFound:`.
func TestPyExceptionFlow_QualifiedRaise(t *testing.T) {
	src := `import errors

def lookup(k):
    raise errors.NotFound("x")
`
	recs := extractPy(t, src, "q.py")
	if !excEdge(recs, "lookup", "THROWS", "NotFound") {
		t.Errorf("qualified raise errors.NotFound should THROWS bare NotFound")
	}
}

// TestPyExceptionFlow_BareReraiseDropped: NEGATIVE — a bare `raise` (re-raise)
// has no type token and must emit no edge.
func TestPyExceptionFlow_BareReraiseDropped(t *testing.T) {
	src := `def f():
    try:
        g()
    except Exception:
        raise
`
	recs := extractPy(t, src, "r.py")
	// The `except Exception:` SHOULD still record a CATCHES(Exception); the
	// bare `raise` must NOT add a THROWS node beyond that.
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == "exception:raise" {
			t.Errorf("bare raise fabricated a node: %q", recs[i].Name)
		}
	}
	// Sanity: there is no THROWS edge on f (only the CATCHES of Exception).
	for i := range recs {
		if recs[i].Name != "f" {
			continue
		}
		for _, r := range recs[i].Relationships {
			if r.Kind == "THROWS" {
				t.Errorf("bare raise must not emit THROWS, got ToID=%q", r.ToID)
			}
		}
	}
}

// TestPyExceptionFlow_BareExceptDropped: NEGATIVE — a bare `except:` with no
// type must emit no CATCHES edge.
func TestPyExceptionFlow_BareExceptDropped(t *testing.T) {
	src := `def f():
    try:
        g()
    except:
        return None
`
	recs := extractPy(t, src, "b.py")
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) {
			t.Errorf("bare except must not create an exception node: %q", recs[i].Name)
		}
		for _, r := range recs[i].Relationships {
			if r.Kind == "CATCHES" {
				t.Errorf("bare except must not emit CATCHES, got ToID=%q", r.ToID)
			}
		}
	}
}

// TestPyExceptionFlow_DynamicRaiseDropped: NEGATIVE — `raise exc_class()` where
// the type is a variable/computed expression must emit no edge.
func TestPyExceptionFlow_DynamicRaiseDropped(t *testing.T) {
	src := `def f(exc_class):
    raise exc_class()
`
	recs := extractPy(t, src, "d.py")
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) {
			t.Errorf("dynamic raise must not create node: %q", recs[i].Name)
		}
	}
}
