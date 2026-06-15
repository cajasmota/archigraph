package python_test

// django_drf_permissions_test.go — issue #2816.
//
// Verifies the python extractor stamps the DRF class-level authorisation
// surface (permission_classes attribute + get_permissions override) onto the
// ViewSet class entity, and harvests the global REST_FRAMEWORK
// DEFAULT_PERMISSION_CLASSES from a settings module.

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// drfPermClass looks up a class entity by name (reusing the shared
// findClassEntity helper which returns a value) and returns a pointer to its
// properties via a fresh local copy is insufficient, so we re-resolve the
// pointer here for in-place property assertions.
func drfPermClass(t *testing.T, entities []types.EntityRecord, name string) *types.EntityRecord {
	t.Helper()
	for i := range entities {
		e := &entities[i]
		if e.Name == name && e.Kind == "SCOPE.Component" && e.Subtype == "class" {
			return e
		}
	}
	return nil
}

func TestDRFPermissions_ClassAttribute(t *testing.T) {
	src := `from rest_framework.permissions import IsAuthenticated

class BuildingViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAuthenticated]
    serializer_class = BuildingSerializer
`
	out := extractPy(t, src, "core/views/building.py")
	cls := drfPermClass(t, out, "BuildingViewSet")
	if cls == nil {
		t.Fatalf("BuildingViewSet class entity not found")
	}
	if cls.Properties["has_permission_classes"] != "true" {
		t.Errorf("has_permission_classes = %q, want true", cls.Properties["has_permission_classes"])
	}
	if cls.Properties["permission_classes"] != "IsAuthenticated" {
		t.Errorf("permission_classes = %q, want IsAuthenticated", cls.Properties["permission_classes"])
	}
}

func TestDRFPermissions_AllowAnyTuple(t *testing.T) {
	src := `from rest_framework.permissions import AllowAny

class LoginViewSet(viewsets.ModelViewSet):
    permission_classes = (AllowAny,)
`
	out := extractPy(t, src, "core/views/auth.py")
	cls := drfPermClass(t, out, "LoginViewSet")
	if cls == nil {
		t.Fatalf("LoginViewSet class entity not found")
	}
	if cls.Properties["permission_classes"] != "AllowAny" {
		t.Errorf("permission_classes = %q, want AllowAny", cls.Properties["permission_classes"])
	}
}

func TestDRFPermissions_DottedPermission(t *testing.T) {
	src := `class AdminViewSet(viewsets.ModelViewSet):
    permission_classes = [permissions.IsAdminUser]
`
	out := extractPy(t, src, "core/views/admin.py")
	cls := drfPermClass(t, out, "AdminViewSet")
	if cls == nil {
		t.Fatalf("AdminViewSet class entity not found")
	}
	if cls.Properties["permission_classes"] != "IsAdminUser" {
		t.Errorf("permission_classes = %q, want IsAdminUser (leaf of permissions.IsAdminUser)", cls.Properties["permission_classes"])
	}
}

func TestDRFPermissions_GetPermissionsOverride(t *testing.T) {
	src := `from rest_framework.permissions import IsAuthenticated

class UserViewSet(viewsets.ModelViewSet):
    serializer_class = UserSerializer

    def get_permissions(self):
        if self.action in ["list", "retrieve"]:
            permission_classes = [IsAuthenticated]
        else:
            permission_classes = [IsAuthenticated, CustomActionPermissionCheck]
        return [permission() for permission in permission_classes]
`
	out := extractPy(t, src, "core/views/user.py")
	cls := drfPermClass(t, out, "UserViewSet")
	if cls == nil {
		t.Fatalf("UserViewSet class entity not found")
	}
	if cls.Properties["has_get_permissions"] != "true" {
		t.Errorf("has_get_permissions = %q, want true", cls.Properties["has_get_permissions"])
	}
	gp := cls.Properties["get_permissions_classes"]
	if gp == "" || !containsAll(gp, "IsAuthenticated", "CustomActionPermissionCheck") {
		t.Errorf("get_permissions_classes = %q, want it to include IsAuthenticated + CustomActionPermissionCheck", gp)
	}
}

func TestDRFPermissions_NonViewClassUntouched(t *testing.T) {
	src := `class PlainModel(models.Model):
    name = models.CharField(max_length=10)
`
	out := extractPy(t, src, "core/models/plain.py")
	cls := drfPermClass(t, out, "PlainModel")
	if cls == nil {
		t.Fatalf("PlainModel class entity not found")
	}
	if _, ok := cls.Properties["has_permission_classes"]; ok {
		t.Errorf("non-DRF class should not carry has_permission_classes")
	}
	if _, ok := cls.Properties["has_get_permissions"]; ok {
		t.Errorf("non-DRF class should not carry has_get_permissions")
	}
}

func TestDRFPermissions_SettingsDefaultPermissionClasses(t *testing.T) {
	src := `REST_FRAMEWORK = {
    "DEFAULT_AUTHENTICATION_CLASSES": ("rest_framework.authentication.TokenAuthentication",),
    "DEFAULT_PERMISSION_CLASSES": ("rest_framework.permissions.IsAuthenticated",),
}
`
	out := extractPy(t, src, "proj/settings.py")
	cm := findConfigModule(out)
	if cm == nil {
		t.Fatalf("settings config_module entity not found")
	}
	if cm.Properties["drf_default_permission_present"] != "true" {
		t.Errorf("drf_default_permission_present = %q, want true", cm.Properties["drf_default_permission_present"])
	}
	if cm.Properties["drf_default_permission_classes"] != "IsAuthenticated" {
		t.Errorf("drf_default_permission_classes = %q, want IsAuthenticated", cm.Properties["drf_default_permission_classes"])
	}
}

func TestDRFPermissions_SettingsNoDefaultPermission(t *testing.T) {
	// upvate-core's real shape: DEFAULT_AUTHENTICATION_CLASSES set, but no
	// DEFAULT_PERMISSION_CLASSES → DRF's built-in default (AllowAny) applies,
	// so the harvested property must be absent.
	src := `REST_FRAMEWORK = {
    "DEFAULT_AUTHENTICATION_CLASSES": ("rest_framework.authentication.TokenAuthentication",),
    "DEFAULT_RENDERER_CLASSES": ("rest_framework.renderers.JSONRenderer",),
}
`
	out := extractPy(t, src, "proj/settings.py")
	cm := findConfigModule(out)
	if cm == nil {
		t.Fatalf("settings config_module entity not found")
	}
	if _, ok := cm.Properties["drf_default_permission_present"]; ok {
		t.Errorf("drf_default_permission_present must be absent when DEFAULT_PERMISSION_CLASSES is not set")
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, n := range needles {
		found := false
		for _, part := range splitComma(haystack) {
			if part == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}
