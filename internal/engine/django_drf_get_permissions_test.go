package engine

import (
	"reflect"
	"testing"
)

// TestApplyDjangoDRFRoutes_GetPermissionsPerActionBranches is the #3933
// value-asserting test: a ViewSet whose `get_permissions(self)` branches on
// `self.action` must attach the per-action permission to the matching CRUD
// route — POST /x (create→IsAdminUser), GET /x (list→AllowAny), and the default
// branch (IsAuthenticated) to every other CRUD route — NOT a flat union on all.
func TestApplyDjangoDRFRoutes_GetPermissionsPerActionBranches(t *testing.T) {
	files := fileMap{
		"urls.py": `
from rest_framework import routers
from views import OrderViewSet

router = routers.DefaultRouter()
router.register(r"orders", OrderViewSet)
`,
		"views.py": `
from rest_framework.viewsets import ModelViewSet
from rest_framework.permissions import IsAuthenticated, IsAdminUser, AllowAny

class OrderViewSet(ModelViewSet):
    def get_permissions(self):
        if self.action == 'create':
            return [IsAdminUser()]
        elif self.action in ['list', 'retrieve']:
            return [AllowAny()]
        return [IsAuthenticated()]
`,
	}
	got := ApplyDjangoDRFRoutes([]string{"urls.py", "views.py"}, files.reader)

	// create → POST /orders → IsAdminUser (admin-only, distinct from union).
	assertEndpointProp(t, got, "http:POST:/orders", "auth_required", "true")
	assertEndpointProp(t, got, "http:POST:/orders", "middleware_names", "IsAdminUser")

	// list → GET /orders → AllowAny → public (auth NOT required).
	assertEndpointProp(t, got, "http:GET:/orders", "middleware_names", "AllowAny")
	assertNoProp(t, got, "http:GET:/orders", "auth_required")

	// retrieve → GET /orders/{pk} → AllowAny (shares the list branch).
	assertEndpointProp(t, got, "http:GET:/orders/{pk}", "middleware_names", "AllowAny")
	assertNoProp(t, got, "http:GET:/orders/{pk}", "auth_required")

	// update / partial_update / destroy fall through to the default → IsAuthenticated.
	for _, id := range []string{
		"http:PUT:/orders/{pk}",
		"http:PATCH:/orders/{pk}",
		"http:DELETE:/orders/{pk}",
	} {
		assertEndpointProp(t, got, id, "middleware_names", "IsAuthenticated")
		assertEndpointProp(t, got, id, "auth_required", "true")
	}
}

// TestApplyDjangoDRFRoutes_GetPermissionsActionRoute verifies an @action with no
// permission_classes kwarg picks up its per-action get_permissions branch on
// THAT action's route (the matching @action route), distinct from CRUD routes.
func TestApplyDjangoDRFRoutes_GetPermissionsActionRoute(t *testing.T) {
	files := fileMap{
		"urls.py": `
from rest_framework import routers
from views import ReportViewSet

router = routers.DefaultRouter()
router.register(r"reports", ReportViewSet)
`,
		"views.py": `
from rest_framework.viewsets import ModelViewSet
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated, IsAdminUser, AllowAny

class ReportViewSet(ModelViewSet):
    @action(detail=False, methods=["get"], url_path="summary")
    def summary(self, request):
        pass

    def get_permissions(self):
        if self.action == 'summary':
            return [AllowAny()]
        return [IsAuthenticated()]
`,
	}
	got := ApplyDjangoDRFRoutes([]string{"urls.py", "views.py"}, files.reader)

	// The @action route 'summary' picks up its own get_permissions branch.
	assertEndpointProp(t, got, "http:GET:/reports/summary", "middleware_names", "AllowAny")
	assertNoProp(t, got, "http:GET:/reports/summary", "auth_required")

	// A regular CRUD route gets the default branch.
	assertEndpointProp(t, got, "http:GET:/reports", "middleware_names", "IsAuthenticated")
	assertEndpointProp(t, got, "http:GET:/reports", "auth_required", "true")
}

// TestApplyDjangoDRFRoutes_PermissionClassesByActionDict verifies the
// `permission_classes_by_action = {...}` DRF idiom attaches per-action perms.
func TestApplyDjangoDRFRoutes_PermissionClassesByActionDict(t *testing.T) {
	files := fileMap{
		"urls.py": `
from rest_framework import routers
from views import ItemViewSet

router = routers.DefaultRouter()
router.register(r"items", ItemViewSet)
`,
		"views.py": `
from rest_framework.viewsets import ModelViewSet
from rest_framework.permissions import IsAuthenticated, IsAdminUser, AllowAny

class ItemViewSet(ModelViewSet):
    permission_classes_by_action = {
        'create': [IsAdminUser],
        'list': [AllowAny],
        'default': [IsAuthenticated],
    }

    def get_permissions(self):
        try:
            return [p() for p in self.permission_classes_by_action[self.action]]
        except KeyError:
            return [p() for p in self.permission_classes_by_action['default']]
`,
	}
	got := ApplyDjangoDRFRoutes([]string{"urls.py", "views.py"}, files.reader)

	assertEndpointProp(t, got, "http:POST:/items", "middleware_names", "IsAdminUser")
	assertEndpointProp(t, got, "http:POST:/items", "auth_required", "true")

	assertEndpointProp(t, got, "http:GET:/items", "middleware_names", "AllowAny")
	assertNoProp(t, got, "http:GET:/items", "auth_required")

	// retrieve/update/etc. → default IsAuthenticated.
	assertEndpointProp(t, got, "http:GET:/items/{pk}", "middleware_names", "IsAuthenticated")
	assertEndpointProp(t, got, "http:DELETE:/items/{pk}", "middleware_names", "IsAuthenticated")
}

// TestApplyDjangoDRFRoutes_ActionKwargBeatsGetPermissions verifies precedence:
// an explicit `permission_classes=[...]` on the @action decorator wins over a
// per-action get_permissions branch for the same action.
func TestApplyDjangoDRFRoutes_ActionKwargBeatsGetPermissions(t *testing.T) {
	files := fileMap{
		"urls.py": `
from rest_framework import routers
from views import DocViewSet

router = routers.DefaultRouter()
router.register(r"docs", DocViewSet)
`,
		"views.py": `
from rest_framework.viewsets import ModelViewSet
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated, IsAdminUser, AllowAny

class DocViewSet(ModelViewSet):
    @action(detail=False, methods=["get"], url_path="export", permission_classes=[IsAdminUser])
    def export(self, request):
        pass

    def get_permissions(self):
        if self.action == 'export':
            return [AllowAny()]
        return [IsAuthenticated()]
`,
	}
	got := ApplyDjangoDRFRoutes([]string{"urls.py", "views.py"}, files.reader)

	// The decorator kwarg IsAdminUser wins, not the get_permissions AllowAny.
	assertEndpointProp(t, got, "http:GET:/docs/export", "middleware_names", "IsAdminUser")
	assertEndpointProp(t, got, "http:GET:/docs/export", "auth_required", "true")
}

// TestApplyDjangoDRFRoutes_DynamicActionConditionFallsBackToUnion is the
// negative / honest-partial test: a get_permissions whose conditions are NOT
// statically resolvable self.action branches falls back to the flat-union
// class-level permission_classes for every route.
func TestApplyDjangoDRFRoutes_DynamicActionConditionFallsBackToUnion(t *testing.T) {
	files := fileMap{
		"urls.py": `
from rest_framework import routers
from views import DynViewSet

router = routers.DefaultRouter()
router.register(r"dyn", DynViewSet)
`,
		"views.py": `
from rest_framework.viewsets import ModelViewSet
from rest_framework.permissions import IsAuthenticated, IsAdminUser

class DynViewSet(ModelViewSet):
    permission_classes = [IsAuthenticated]

    def get_permissions(self):
        if self.request.user.is_staff:
            return [IsAdminUser()]
        return super().get_permissions()
`,
	}
	got := ApplyDjangoDRFRoutes([]string{"urls.py", "views.py"}, files.reader)

	// No resolvable self.action branch → every route keeps the flat union
	// (class-level permission_classes = IsAuthenticated).
	for _, id := range []string{
		"http:GET:/dyn",
		"http:POST:/dyn",
		"http:GET:/dyn/{pk}",
		"http:DELETE:/dyn/{pk}",
	} {
		assertEndpointProp(t, got, id, "middleware_names", "IsAuthenticated")
		assertEndpointProp(t, got, id, "auth_required", "true")
	}
}

// TestParseDRFActionPermissions covers the parser directly for both idioms and
// the dynamic-condition skip.
func TestParseDRFActionPermissions(t *testing.T) {
	cases := []struct {
		name string
		body string
		want map[string][]string
	}{
		{
			name: "get_permissions branches",
			body: `
    def get_permissions(self):
        if self.action == 'create':
            return [IsAdminUser()]
        elif self.action in ['list', 'retrieve']:
            return [AllowAny()]
        return [IsAuthenticated()]
`,
			want: map[string][]string{
				"create":   {"IsAdminUser"},
				"list":     {"AllowAny"},
				"retrieve": {"AllowAny"},
				"":         {"IsAuthenticated"},
			},
		},
		{
			name: "dict idiom",
			body: `
    permission_classes_by_action = {
        'create': [IsAdminUser],
        'list': [AllowAny],
        'default': [IsAuthenticated],
    }
`,
			want: map[string][]string{
				"create": {"IsAdminUser"},
				"list":   {"AllowAny"},
				"":       {"IsAuthenticated"},
			},
		},
		{
			name: "dotted permission refs",
			body: `
    def get_permissions(self):
        if self.action == 'destroy':
            return [permissions.IsAdminUser()]
        return [permissions.IsAuthenticated()]
`,
			want: map[string][]string{
				"destroy": {"IsAdminUser"},
				"":        {"IsAuthenticated"},
			},
		},
		{
			name: "dynamic condition skipped",
			body: `
    def get_permissions(self):
        if self.request.user.is_staff:
            return [IsAdminUser()]
        return super().get_permissions()
`,
			// Only the unguarded default return resolves; the dynamic if-branch
			// is skipped, and the trailing super() call is not a list literal.
			want: nil,
		},
		{
			name: "no override",
			body: `
    permission_classes = [IsAuthenticated]
    queryset = None
`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := parseDRFActionPermissions(tc.body)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseDRFActionPermissions() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestPostureForAction verifies the per-action override layering over the
// ViewSet posture, including the default-branch fallthrough and the
// no-override passthrough.
func TestPostureForAction(t *testing.T) {
	vc := drfViewSetClass{
		posture: drfPosture{permissionClasses: []string{"IsAuthenticated"}},
		actionPermissions: map[string][]string{
			"create": {"IsAdminUser"},
			"":       {"IsAuthenticated"},
		},
	}
	if got := postureForAction(vc, "create").permissionClasses; !reflect.DeepEqual(got, []string{"IsAdminUser"}) {
		t.Errorf("create perms = %v, want [IsAdminUser]", got)
	}
	if got := postureForAction(vc, "list").permissionClasses; !reflect.DeepEqual(got, []string{"IsAuthenticated"}) {
		t.Errorf("list perms (default) = %v, want [IsAuthenticated]", got)
	}

	// No actionPermissions at all → posture returned unchanged.
	bare := drfViewSetClass{posture: drfPosture{permissionClasses: []string{"IsAuthenticated"}}}
	if got := postureForAction(bare, "create").permissionClasses; !reflect.DeepEqual(got, []string{"IsAuthenticated"}) {
		t.Errorf("bare passthrough = %v, want [IsAuthenticated]", got)
	}
}

// TestPostureForAction_PermissionPages verifies the #3972 per-action page-key
// identity layers onto the posture with the same explicit→default("")
// precedence as the permission-class list.
func TestPostureForAction_PermissionPages(t *testing.T) {
	vc := drfViewSetClass{
		posture: drfPosture{permissionClasses: []string{"IsAuthenticated"}},
		actionPermissions: map[string][]string{
			"get_jurisdictions_by_group": {"IsAuthenticated", "CustomPagePermissionCheck"},
			"":                           {"IsAuthenticated", "CustomActionPermissionCheck"},
		},
		actionPermissionPages: map[string][]string{
			"get_jurisdictions_by_group": {"JURISDICTIONS"},
		},
	}
	if got := postureForAction(vc, "get_jurisdictions_by_group").permissionPages; !reflect.DeepEqual(got, []string{"JURISDICTIONS"}) {
		t.Errorf("jurisdictions page-keys = %v, want [JURISDICTIONS]", got)
	}
	// An action with no explicit page-key entry and no default ("") page-key →
	// no page-key stamped (honest-partial).
	if got := postureForAction(vc, "some_other_action").permissionPages; len(got) != 0 {
		t.Errorf("default action page-keys = %v, want empty", got)
	}
}

// TestParseDRFActionPermissions_AssignThenReturnComprehension is the #3972
// value-asserting direct-parser test on the REAL acme GroupViewSet shape:
// each branch ASSIGNS `permission_classes = [...]` (not `return [...]`) and the
// method ends with `return [p() for p in permission_classes]` — the assigned
// list must bind per branch, the PERMISSION_PAGES["<KEY>"] argument must surface
// as the per-action page-key, and a branch without a page-key (CustomActionPermissionCheck
// default) must NOT fabricate one.
func TestParseDRFActionPermissions_AssignThenReturnComprehension(t *testing.T) {
	body := `
    def get_permissions(self):
        if self.action in ["groups_list", "get_group_type"]:
            permission_classes = [IsAuthenticated]
        elif self.action in ["update_group_jurisdiction_config", "get_jurisdictions_by_group"]:
            permission_classes = [IsAuthenticated, CustomPagePermissionCheck(PERMISSION_PAGES["JURISDICTIONS"])]
        elif self.action in ["filter"]:
            permission_classes = [IsAuthenticated, CustomPagePermissionCheck(PERMISSION_PAGES["CONTRACT_PROPOSALS"])]
        else:
            permission_classes = [IsAuthenticated, CustomActionPermissionCheck]
        return [permission() for permission in permission_classes]
`
	perms, pages := parseDRFActionPermissions(body)

	// Permission-class lists bind per branch (assignment, not return literal).
	wantPerms := map[string][]string{
		"groups_list":                      {"IsAuthenticated"},
		"get_group_type":                   {"IsAuthenticated"},
		"update_group_jurisdiction_config": {"IsAuthenticated", "CustomPagePermissionCheck"},
		"get_jurisdictions_by_group":       {"IsAuthenticated", "CustomPagePermissionCheck"},
		"filter":                           {"IsAuthenticated", "CustomPagePermissionCheck"},
		"":                                 {"IsAuthenticated", "CustomActionPermissionCheck"},
	}
	if !reflect.DeepEqual(perms, wantPerms) {
		t.Errorf("perms = %#v\nwant %#v", perms, wantPerms)
	}

	// Page-keys: the JURISDICTIONS branch surfaces JURISDICTIONS on BOTH its
	// actions; the filter branch surfaces CONTRACT_PROPOSALS — DISTINCT per
	// action. The plain-IsAuthenticated and CustomActionPermissionCheck branches
	// have no page-key (honest-partial — absent, not fabricated).
	wantPages := map[string][]string{
		"update_group_jurisdiction_config": {"JURISDICTIONS"},
		"get_jurisdictions_by_group":       {"JURISDICTIONS"},
		"filter":                           {"CONTRACT_PROPOSALS"},
	}
	if !reflect.DeepEqual(pages, wantPages) {
		t.Errorf("pages = %#v\nwant %#v", pages, wantPages)
	}
}

// TestParseDRFActionPermissions_DynamicPageKeyHonestPartial is the #3972
// negative test: a PERMISSION_PAGES subscript whose key is a VARIABLE (not a
// string literal) is not statically resolvable, so no page-key is captured
// (honest-partial) — but the permission-class list still binds.
func TestParseDRFActionPermissions_DynamicPageKeyHonestPartial(t *testing.T) {
	body := `
    def get_permissions(self):
        if self.action in ["dynamic"]:
            permission_classes = [IsAuthenticated, CustomPagePermissionCheck(PERMISSION_PAGES[self.page_key])]
        else:
            permission_classes = [IsAuthenticated]
        return [permission() for permission in permission_classes]
`
	perms, pages := parseDRFActionPermissions(body)
	if got := perms["dynamic"]; !reflect.DeepEqual(got, []string{"IsAuthenticated", "CustomPagePermissionCheck"}) {
		t.Errorf("dynamic perms = %v, want [IsAuthenticated CustomPagePermissionCheck]", got)
	}
	// Dynamic (variable) subscript → no page-key resolved anywhere.
	if len(pages) != 0 {
		t.Errorf("pages = %#v, want empty (dynamic key honest-partial)", pages)
	}
}

// TestApplyDjangoDRFRoutes_PageKeyPerActionRoute is the #3972 end-to-end
// value-asserting test on the real GroupViewSet idiom: @action routes whose
// get_permissions branch assigns CustomPagePermissionCheck(PERMISSION_PAGES["<KEY>"])
// surface the page-key as `auth_permissions` on THAT action's route — distinct
// per action — while a branch with no page-key does not.
func TestApplyDjangoDRFRoutes_PageKeyPerActionRoute(t *testing.T) {
	files := fileMap{
		"urls.py": `
from rest_framework import routers
from views import GroupViewSet

router = routers.DefaultRouter()
router.register(r"groups", GroupViewSet)
`,
		"views.py": `
from rest_framework.viewsets import ModelViewSet
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from core.helper.permissions import CustomPagePermissionCheck, CustomActionPermissionCheck
from acme_core.settings import PERMISSION_PAGES

class GroupViewSet(ModelViewSet):
    @action(detail=False, methods=["get"], url_path="jurisdictions")
    def get_jurisdictions_by_group(self, request):
        pass

    @action(detail=False, methods=["get"], url_path="proposals")
    def filter(self, request):
        pass

    @action(detail=False, methods=["get"], url_path="types")
    def get_group_type(self, request):
        pass

    def get_permissions(self):
        if self.action in ["get_group_type"]:
            permission_classes = [IsAuthenticated]
        elif self.action in ["get_jurisdictions_by_group"]:
            permission_classes = [IsAuthenticated, CustomPagePermissionCheck(PERMISSION_PAGES["JURISDICTIONS"])]
        elif self.action in ["filter"]:
            permission_classes = [IsAuthenticated, CustomPagePermissionCheck(PERMISSION_PAGES["CONTRACT_PROPOSALS"])]
        else:
            permission_classes = [IsAuthenticated, CustomActionPermissionCheck]
        return [permission() for permission in permission_classes]
`,
	}
	got := ApplyDjangoDRFRoutes([]string{"urls.py", "views.py"}, files.reader)

	// get_jurisdictions_by_group → JURISDICTIONS page-key on its route.
	assertEndpointProp(t, got, "http:GET:/groups/jurisdictions", "auth_permissions", "JURISDICTIONS")
	assertEndpointProp(t, got, "http:GET:/groups/jurisdictions", "auth_required", "true")

	// filter → CONTRACT_PROPOSALS — DISTINCT from the jurisdictions route.
	assertEndpointProp(t, got, "http:GET:/groups/proposals", "auth_permissions", "CONTRACT_PROPOSALS")
	assertEndpointProp(t, got, "http:GET:/groups/proposals", "auth_required", "true")

	// get_group_type → plain IsAuthenticated branch → NO page-key surfaced.
	assertNoProp(t, got, "http:GET:/groups/types", "auth_permissions")
}
