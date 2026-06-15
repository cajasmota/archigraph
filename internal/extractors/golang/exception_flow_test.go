package golang_test

import (
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func goExcEdge(recs []types.EntityRecord, fromName, kind, typeName string) bool {
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

func goExcNodeCount(recs []types.EntityRecord, typeName string) int {
	want := extractor.ExceptionTypeName(typeName)
	c := 0
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == want {
			c++
		}
	}
	return c
}

// TestGoExceptionFlow_ReturnAndIsConverge: `return ErrNotFound` in one function
// and `errors.Is(err, ErrNotFound)` in another converge on the SAME node.
func TestGoExceptionFlow_ReturnAndIsConverge(t *testing.T) {
	src := `package repo

import "errors"

var ErrNotFound = errors.New("not found")

func Get(id string) (string, error) {
	if id == "" {
		return "", ErrNotFound
	}
	return id, nil
}

func Handle(id string) int {
	_, err := Get(id)
	if errors.Is(err, ErrNotFound) {
		return 404
	}
	return 200
}
`
	recs := extractGoRaw(t, src)

	if !goExcEdge(recs, "Get", "THROWS", "ErrNotFound") {
		t.Errorf("missing THROWS(Get -> ErrNotFound)")
	}
	if !goExcEdge(recs, "Handle", "CATCHES", "ErrNotFound") {
		t.Errorf("missing CATCHES(Handle -> ErrNotFound)")
	}
	if n := goExcNodeCount(recs, "ErrNotFound"); n != 1 {
		t.Fatalf("return + errors.Is of ErrNotFound must converge on ONE node, got %d", n)
	}
}

// TestGoExceptionFlow_WrappedSentinel: `fmt.Errorf("x: %w", ErrNotFound)`
// records THROWS of the wrapped named sentinel.
func TestGoExceptionFlow_WrappedSentinel(t *testing.T) {
	src := `package repo

import "fmt"

var ErrNotFound = fmt.Errorf("nf")

func Load(id string) error {
	return fmt.Errorf("loading %s: %w", id, ErrNotFound)
}
`
	recs := extractGoRaw(t, src)
	if !goExcEdge(recs, "Load", "THROWS", "ErrNotFound") {
		t.Errorf("missing THROWS(Load -> ErrNotFound) for wrapped sentinel")
	}
}

// TestGoExceptionFlow_QualifiedSentinel: `return io.EOF`-style qualified
// sentinel records the bare trailing name (ErrXxx convention).
func TestGoExceptionFlow_QualifiedSentinel(t *testing.T) {
	src := `package repo

import "mypkg"

func Read() error {
	return mypkg.ErrClosed
}
`
	recs := extractGoRaw(t, src)
	if !goExcEdge(recs, "Read", "THROWS", "ErrClosed") {
		t.Errorf("missing THROWS(Read -> ErrClosed) for qualified sentinel")
	}
}

// TestGoExceptionFlow_AnonymousInlineDropped: NEGATIVE — `return errors.New(...)`
// and bare `fmt.Errorf("...")` carry no NAMED sentinel and emit no edge/node.
func TestGoExceptionFlow_AnonymousInlineDropped(t *testing.T) {
	src := `package repo

import (
	"errors"
	"fmt"
)

func A() error {
	return errors.New("boom")
}

func B(n int) error {
	return fmt.Errorf("bad value: %d", n)
}
`
	recs := extractGoRaw(t, src)
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) {
			t.Errorf("anonymous inline error must not create a node: %q", recs[i].Name)
		}
	}
}

// TestGoExceptionFlow_OpaquePassThroughDropped: NEGATIVE — `return err` passing
// through a local error variable is not a named sentinel and emits no edge.
func TestGoExceptionFlow_OpaquePassThroughDropped(t *testing.T) {
	src := `package repo

func C() error {
	err := doWork()
	if err != nil {
		return err
	}
	return nil
}
`
	recs := extractGoRaw(t, src)
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) {
			t.Errorf("opaque return err must not create a node: %q", recs[i].Name)
		}
		for _, r := range recs[i].Relationships {
			if r.Kind == "THROWS" {
				t.Errorf("opaque return err must emit no THROWS, got %q", r.ToID)
			}
		}
	}
}
