package python_test

// django_drf_actions_test.go — fixture-style tests for issue #1967.
//
// Goal: the @action(...) decorator kwargs land on the per-method Operation
// entity properties, AND a per-method decorator summary lands on the
// parent class so the ClassManifest builder can surface it.

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/types"
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
		// #3628 area #6 — endpoint protection normalised onto the action.
		"auth_required":   "true",
		"auth_guard":      "IsAdmin",
		"auth_method":     "permission_classes",
		"auth_confidence": "high",
	}
	for k, want := range checks {
		if got := op.Properties[k]; got != want {
			t.Errorf("Properties[%q] = %q, want %q", k, got, want)
		}
	}
}

// TestDRFAction_AllowAnyIsPublic verifies a per-action permission_classes of
// only [AllowAny] marks the endpoint explicitly public (auth_required=false),
// not protected. #3628 area #6.
func TestDRFAction_AllowAnyIsPublic(t *testing.T) {
	src := `from rest_framework.decorators import action

class PublicViewSet:
    @action(detail=False, methods=["get"], permission_classes=[AllowAny])
    def ping(self, request):
        return None
`
	out := extractPy(t, src, "api/views.py")
	op := findDRFOp(out, "api/views.py", "PublicViewSet.ping")
	if op == nil {
		t.Fatalf("expected PublicViewSet.ping operation entity")
	}
	if got := op.Properties["auth_required"]; got != "false" {
		t.Errorf("auth_required = %q, want \"false\"", got)
	}
	if got := op.Properties["auth_guard"]; got != "" {
		t.Errorf("auth_guard = %q, want empty (public)", got)
	}
}

// TestDRFAction_NoPermissionInherits verifies an action WITHOUT a per-action
// permission_classes does not get an auth_required stamp here — it inherits the
// class-level posture (django_drf_permissions.go). #3628 area #6.
func TestDRFAction_NoPermissionInherits(t *testing.T) {
	src := `from rest_framework.decorators import action

class InheritViewSet:
    @action(detail=True, methods=["post"])
    def archive(self, request):
        return None
`
	out := extractPy(t, src, "api/views.py")
	op := findDRFOp(out, "api/views.py", "InheritViewSet.archive")
	if op == nil {
		t.Fatalf("expected InheritViewSet.archive operation entity")
	}
	if _, ok := op.Properties["auth_required"]; ok {
		t.Errorf("auth_required should be unset (inherits class posture), got %q", op.Properties["auth_required"])
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
