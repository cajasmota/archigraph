package python_test

// django_drf_actions_test.go — fixture-style tests for issue #1967.
//
// Goal: the @action(...) decorator kwargs land on the per-method Operation
// entity properties, AND a per-method decorator summary lands on the
// parent class so the ClassManifest builder can surface it.

import (
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

func findDRFOp(entities []types.EntityRecord, file, name string) *types.EntityRecord {
	for i := range entities {
		e := &entities[i]
		if e.SourceFile == file && e.Name == name && e.Kind == "SCOPE.Operation" {
			return e
		}
	}
	return nil
}

func findDRFClass(entities []types.EntityRecord, file, name string) *types.EntityRecord {
	for i := range entities {
		e := &entities[i]
		if e.SourceFile == file && e.Name == name && e.Kind == "SCOPE.Component" && e.Subtype == "class" {
			return e
		}
	}
	return nil
}

// TestDRFAction_BasicKwargs verifies @action(detail=False, methods=["get"],
// serializer_class=FooSerializer) lands on the Operation entity.
//
// Issue #1967.
func TestDRFAction_BasicKwargs(t *testing.T) {
	src := `from rest_framework.decorators import action

class ContractViewSet:
    @action(detail=False, methods=["get"], serializer_class=AssignContactsSerializer, url_path="assign", permission_classes=[IsAdmin])
    def assign_contacts(self, request):
        return None
`
	out := extractPy(t, src, "api/views.py")
	op := findDRFOp(out, "api/views.py", "ContractViewSet.assign_contacts")
	if op == nil {
		t.Fatalf("expected ContractViewSet.assign_contacts operation entity, not found. entities=%d", len(out))
	}
	checks := map[string]string{
		"drf_action":         "true",
		"is_detail":          "false",
		"http_method":        "get",
		"http_methods":       "get",
		"serializer_class":   "AssignContactsSerializer",
		"url_path":           "assign",
		"permission_classes": "IsAdmin",
	}
	for k, want := range checks {
		if got := op.Properties[k]; got != want {
			t.Errorf("Properties[%q] = %q, want %q", k, got, want)
		}
	}
}

// TestDRFAction_MultiMethod verifies `methods=["get","post"]` produces
// `http_method="get"` (first verb) and `http_methods="get,post"`.
func TestDRFAction_MultiMethod(t *testing.T) {
	src := `from rest_framework.decorators import action

class PermitViewSet:
    @action(detail=True, methods=["get", "post"])
    def approve(self, request):
        return None
`
	out := extractPy(t, src, "api/views.py")
	op := findDRFOp(out, "api/views.py", "PermitViewSet.approve")
	if op == nil {
		t.Fatalf("expected PermitViewSet.approve operation entity")
	}
	if op.Properties["http_method"] != "get" {
		t.Errorf("http_method = %q, want \"get\"", op.Properties["http_method"])
	}
	if op.Properties["http_methods"] != "get,post" {
		t.Errorf("http_methods = %q, want \"get,post\"", op.Properties["http_methods"])
	}
	if op.Properties["is_detail"] != "true" {
		t.Errorf("is_detail = %q, want \"true\"", op.Properties["is_detail"])
	}
}

// TestDRFAction_PerMethodDecoratorOnClass verifies the parent class also
// gets a per-method decorator_<method> property so the ClassManifest
// builder can render per-action decorations.
func TestDRFAction_PerMethodDecoratorOnClass(t *testing.T) {
	src := `from rest_framework.decorators import action

class PermitViewSet:
    @action(detail=False, methods=["post"])
    def bulk_create(self, request):
        return None
`
	out := extractPy(t, src, "api/views.py")
	cls := findDRFClass(out, "api/views.py", "PermitViewSet")
	if cls == nil {
		t.Fatalf("PermitViewSet class not found")
	}
	v := cls.Properties["decorator_bulk_create"]
	if v == "" {
		t.Fatalf("expected decorator_bulk_create property on class, got empty")
	}
	if !strings.Contains(v, "action") {
		t.Errorf("decorator snippet should mention 'action', got %q", v)
	}
}

// TestDRFAction_NonActionDecoratorIgnored verifies methods decorated with
// something OTHER than @action don't get drf_action=true stamped.
func TestDRFAction_NonActionDecoratorIgnored(t *testing.T) {
	src := `class FooView:
    @staticmethod
    def helper(): pass
    @property
    def x(self): return 1
`
	out := extractPy(t, src, "api/views.py")
	for _, n := range []string{"FooView.helper", "FooView.x"} {
		op := findDRFOp(out, "api/views.py", n)
		if op == nil {
			continue
		}
		if op.Properties["drf_action"] == "true" {
			t.Errorf("%s should NOT be tagged drf_action=true", n)
		}
	}
}
