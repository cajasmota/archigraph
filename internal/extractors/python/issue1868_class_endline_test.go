package python

import (
	"context"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
)

// Issue #1868 — Class/View entities still showed `end_line=0` sentinels in
// production reindex output (dogfood R1 2026-05-23 on ContractViewSet:
// "Source: core/views/contract_viewset.py (lines 1575-0)") even after the
// #1964 fix landed. The fix targeted Operations + Modules; this regression
// test pins down the Class/View cases the #1964 test fixture did not cover.
//
// Pinned shapes:
//
//  1. Decorated class (`@decorator class Foo:`) — the inner class_definition
//     node is reached via decorated_definition.definition. Verifies that
//     buildClass uses the inner node's EndPoint regardless of the wrapper.
//
//  2. Large class with multi-statement methods — verifies the end_line is
//     the line of the LAST statement of the LAST method (the dedent line),
//     not an early-truncated value.
//
//  3. Nested class — both outer and inner classes carry non-zero end_line
//     that bracket their respective bodies.
//
//  4. Class with NO body (just `class Empty: pass` or `class Empty: ...`)
//     — must still emit end_line ≥ start_line (no 0 sentinel).
func TestPython_Issue1868_ClassEndLine_DecoratedAndLargeAndNested(t *testing.T) {
	src := `# Issue #1868 fixture
import logging


@register("v1")
class DecoratedView:
    """Decorated class declared in the same file as a large class.

    The decorated_definition wrapper must not cause the inner
    class_definition's EndPoint to be lost.
    """

    def list(self, request):
        return {"ok": True}

    def create(self, request):
        log = logging.getLogger(__name__)
        log.info("creating")
        return {"id": 1}


class LargeViewSet:
    """Multi-method class — end_line must reach the LAST statement of the
    LAST method, not an early-truncated value."""

    queryset = None
    serializer_class = None

    def list(self, request):
        return self.queryset

    def retrieve(self, request, pk=None):
        obj = self.queryset.filter(pk=pk).first()
        return obj

    def create(self, request):
        return {"created": True}

    def update(self, request, pk=None):
        obj = self.queryset.filter(pk=pk).first()
        return obj

    def destroy(self, request, pk=None):
        return {"deleted": True}


class Outer:
    class Inner:
        def inner_method(self):
            return 42

    def outer_method(self):
        return Outer.Inner()


class Empty:
    pass


class EllipsisOnly:
    ...
`

	e := &Extractor{}
	out, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     "fixture/issue1868.py",
		Content:  []byte(src),
		Language: "python",
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Total file line count (used as the upper-bound sanity check — no
	// class's end_line should exceed the file length).
	fileLineCount := strings.Count(src, "\n") + 1

	type want struct {
		minStart int
		maxEnd   int
		minSpan  int // end - start
	}
	wants := map[string]want{
		// Decorated class spans roughly the docstring + 2 methods.
		"DecoratedView": {minStart: 1, maxEnd: fileLineCount, minSpan: 8},
		// Large class spans many methods.
		"LargeViewSet": {minStart: 1, maxEnd: fileLineCount, minSpan: 20},
		// Outer class wraps Inner + outer_method.
		"Outer": {minStart: 1, maxEnd: fileLineCount, minSpan: 6},
		// Nested classes are emitted with dotted names.
		"Outer.Inner": {minStart: 1, maxEnd: fileLineCount, minSpan: 2},
		// Empty classes must still have end_line ≥ start_line.
		"Empty":        {minStart: 1, maxEnd: fileLineCount, minSpan: 0},
		"EllipsisOnly": {minStart: 1, maxEnd: fileLineCount, minSpan: 0},
	}

	seen := map[string]bool{}
	for _, ent := range out {
		if ent.Kind != "SCOPE.Component" || ent.Subtype != "class" {
			continue
		}
		w, ok := wants[ent.Name]
		if !ok {
			continue
		}
		seen[ent.Name] = true
		if ent.StartLine < w.minStart {
			t.Errorf("class %q: start_line=%d < minStart=%d", ent.Name, ent.StartLine, w.minStart)
		}
		if ent.EndLine <= 0 {
			t.Errorf("class %q: end_line=%d — issue #1868 sentinel re-introduced", ent.Name, ent.EndLine)
		}
		if ent.EndLine > w.maxEnd {
			t.Errorf("class %q: end_line=%d > maxEnd=%d (file length)", ent.Name, ent.EndLine, w.maxEnd)
		}
		if ent.EndLine < ent.StartLine {
			t.Errorf("class %q: end_line(%d) < start_line(%d)", ent.Name, ent.EndLine, ent.StartLine)
		}
		span := ent.EndLine - ent.StartLine
		if span < w.minSpan {
			t.Errorf("class %q: span=%d (start=%d end=%d) < minSpan=%d — body likely truncated",
				ent.Name, span, ent.StartLine, ent.EndLine, w.minSpan)
		}
	}
	for name := range wants {
		if !seen[name] {
			t.Errorf("class %q not emitted", name)
		}
	}
}
