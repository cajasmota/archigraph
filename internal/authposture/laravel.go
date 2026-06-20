// laravel.go — the Laravel auth-posture resolver (#4541; replaces the laravel
// stub).
//
// Laravel expresses endpoint authorization through route MIDDLEWARE (declared on
// the route, a route group, or in the controller constructor), plus Gates and
// Policies invoked per-action. The engine stamps the reconciled middleware chain
// onto the route/endpoint (with a raw route/controller-source fallback):
//
//   - ->middleware('auth' | 'auth:api' | 'auth:sanctum')        → authenticated.
//   - $this->middleware('auth') in the controller __construct    → authenticated
//     (class scope, honoring ->only([...]) / ->except([...])).
//   - ->middleware('role:admin') (spatie/permission)             → role.
//   - ->middleware('can:update,post') / can:permission           → action (ability).
//   - $this->authorize('update', $post) / Gate::allows           → action (Gate/Policy).
//   - ->middleware('role:superuser'|'admin') with superuser name → superuser.
//   - ->withoutMiddleware('auth') / no auth middleware           → public.
//
// SCOPE PRECEDENCE (most-specific wins — #4541):
//
//  1. PER-ROUTE / PER-ACTION — a route-level `->middleware('auth')` /
//     `->withoutMiddleware('auth')` or a controller-method `$this->authorize(...)`
//     Gate/Policy check. Applies to THIS route/action. A `->withoutMiddleware('auth')`
//     opens the route even when a group applied auth.
//  2. GROUP / CONTROLLER-CONSTRUCTOR — a `Route::group(['middleware'=>['auth']])`
//     or `$this->middleware('auth')->only([...])`/->except([...]) applies to the
//     routes/actions it scopes without their own auth middleware.
//
// The engine reconciles route+group+constructor middleware (resolving only:/except:
// scoping) into the per-route posture (auth_required/auth_middleware/auth_roles/
// auth_permissions/auth_method); the resolver reads that first and falls back to
// scanning the route/controller source. Output normalises into the shared
// {Kind, Literal} vocabulary so the diff core compares a Laravel posture against
// the Django oracle or a NestJS posture directly.
package authposture

import (
	"regexp"
	"strings"
)

type laravelResolver struct{}

func (laravelResolver) Name() string { return "laravel" }

var (
	// ->middleware('auth') / ->middleware('auth:api') / $this->middleware('auth:sanctum')
	laravelAuthMwRe = regexp.MustCompile(`middleware\s*\(\s*\[?\s*['"]auth(?::[\w.-]+)?['"]`)
	// ->middleware('role:admin') / 'role:admin,editor'  (spatie/permission)
	laravelRoleMwRe = regexp.MustCompile(`['"](?:role|hasrole)\s*:\s*([\w.-]+)`)
	// ->middleware('permission:edit articles') / 'can:update,post'  → ability/action
	laravelCanMwRe = regexp.MustCompile(`['"](?:can|permission)\s*:\s*([\w. -]+?)(?:,[^'"]*)?['"]`)
	// ->withoutMiddleware('auth')  → public override.
	laravelWithoutAuthRe = regexp.MustCompile(`withoutMiddleware\s*\(\s*\[?\s*['"]auth(?::[\w.-]+)?['"]`)
	// $this->authorize('update', $post) / Gate::allows('update', ...) / @can('x')
	laravelAuthorizeRe = regexp.MustCompile(`(?:\$this->authorize|Gate::(?:allows|denies|authorize)|@can)\s*\(\s*['"]([\w. -]+)['"]`)
)

// Resolve decodes a Laravel auth signal. Recognises the framework when the entity
// carries a Laravel framework hint OR a recognisable auth middleware / Gate /
// Policy construct in its props or source. Reconciled props win; source scanning is
// the fallback.
func (l laravelResolver) Resolve(sig Signal) (Posture, bool) {
	fw := strings.ToLower(firstNonEmpty(sig.Framework, sig.prop("framework")))
	mw := firstNonEmpty(sig.prop("auth_middleware"), sig.prop("middleware"))
	roles := sig.prop("auth_roles")
	perms := firstNonEmpty(sig.prop("auth_permissions"), sig.prop("auth_page"))
	authReq := sig.prop("auth_required")
	method := strings.ToLower(sig.prop("auth_method"))
	src := sig.Source

	isLaravel := strings.Contains(fw, "laravel") || strings.Contains(fw, "php") || strings.Contains(fw, "lumen") ||
		strings.Contains(method, "middleware") || strings.Contains(method, "gate") || strings.Contains(method, "policy") ||
		mw != "" || roles != "" || perms != "" || authReq != "" ||
		hasLaravelAuth(src)
	if !isLaravel {
		return Posture{}, false
	}

	// (1) PER-ROUTE OVERRIDE — withoutMiddleware('auth') opens the route.
	if authReq == "false" && strings.Contains(method, "without") {
		return Posture{Kind: KindPublic, Detail: "->withoutMiddleware('auth')"}, true
	}
	if src != "" && laravelWithoutAuthRe.MatchString(src) {
		return Posture{Kind: KindPublic, Detail: "->withoutMiddleware('auth')"}, true
	}

	// (2) Reconciled role / permission props, tightest first.
	if roles != "" {
		if r := firstCSV(roles); strings.EqualFold(r, "superuser") || strings.EqualFold(r, "super-admin") || strings.EqualFold(r, "root") {
			return Posture{Kind: KindSuperuser, Detail: "auth_roles=" + roles}, true
		}
		return Posture{Kind: KindRole, Literal: firstCSV(roles), Detail: "auth_roles=" + roles}, true
	}
	if perms != "" {
		return Posture{Kind: KindAction, Literal: firstCSV(perms), Detail: "can/permission=" + perms}, true
	}

	// (3) Middleware-chain prop — classify the named middleware.
	if mw != "" {
		if p, ok := decodeLaravelMiddleware(mw, "auth_middleware="+mw); ok {
			return p, true
		}
	}

	// (4) Source fallback — scan the route registration / controller body.
	if src != "" {
		if p, ok := decodeLaravelSource(src); ok {
			return p, true
		}
	}

	// (5) Reconciled auth_required without a decodable middleware.
	switch authReq {
	case "true":
		return Posture{Kind: KindAuthenticated, Detail: "auth_required=true (auth middleware, no decodable role/ability)"}, true
	case "false":
		return Posture{Kind: KindPublic, Detail: "auth_required=false (no auth middleware)"}, true
	}

	// Recognised as Laravel but no decodable gate → unknown (never false-public).
	return Posture{Kind: KindUnknown, Detail: "Laravel route with no decodable auth middleware / Gate / Policy"}, true
}

// decodeLaravelMiddleware classifies a middleware chain string (e.g.
// `auth:sanctum`, `role:admin`, `can:update,post`).
func decodeLaravelMiddleware(mw, detail string) (Posture, bool) {
	if m := laravelRoleMwRe.FindStringSubmatch(mw); m != nil {
		role := strings.TrimSpace(m[1])
		if strings.EqualFold(role, "superuser") || strings.EqualFold(role, "super-admin") || strings.EqualFold(role, "root") {
			return Posture{Kind: KindSuperuser, Detail: detail}, true
		}
		return Posture{Kind: KindRole, Literal: role, Detail: detail}, true
	}
	if m := laravelCanMwRe.FindStringSubmatch(mw); m != nil {
		return Posture{Kind: KindAction, Literal: strings.TrimSpace(m[1]), Detail: detail}, true
	}
	if laravelAuthMwRe.MatchString(mw) {
		return Posture{Kind: KindAuthenticated, Detail: detail}, true
	}
	return Posture{}, false
}

// decodeLaravelSource scans a route/controller source body in role ▸ can/ability
// ▸ Gate/Policy authorize ▸ auth-middleware priority order.
func decodeLaravelSource(src string) (Posture, bool) {
	if m := laravelRoleMwRe.FindStringSubmatch(src); m != nil {
		role := strings.TrimSpace(m[1])
		if strings.EqualFold(role, "superuser") || strings.EqualFold(role, "super-admin") || strings.EqualFold(role, "root") {
			return Posture{Kind: KindSuperuser, Detail: "->middleware('role:" + role + "')"}, true
		}
		return Posture{Kind: KindRole, Literal: role, Detail: "->middleware('role:" + role + "')"}, true
	}
	if m := laravelCanMwRe.FindStringSubmatch(src); m != nil {
		return Posture{Kind: KindAction, Literal: strings.TrimSpace(m[1]), Detail: "->middleware('can:" + m[1] + "')"}, true
	}
	if m := laravelAuthorizeRe.FindStringSubmatch(src); m != nil {
		return Posture{Kind: KindAction, Literal: strings.TrimSpace(m[1]), Detail: "$this->authorize('" + m[1] + "') (Gate/Policy)"}, true
	}
	if laravelAuthMwRe.MatchString(src) {
		return Posture{Kind: KindAuthenticated, Detail: "->middleware('auth')"}, true
	}
	return Posture{}, false
}

// hasLaravelAuth reports whether src carries a recognisable Laravel auth construct.
func hasLaravelAuth(src string) bool {
	if src == "" {
		return false
	}
	return laravelAuthMwRe.MatchString(src) ||
		laravelRoleMwRe.MatchString(src) ||
		laravelCanMwRe.MatchString(src) ||
		laravelWithoutAuthRe.MatchString(src) ||
		laravelAuthorizeRe.MatchString(src)
}
