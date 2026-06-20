// flask.go — the Flask auth-posture resolver (#4540; replaces the flask stub).
//
// Flask expresses endpoint authorization through view DECORATORS (Flask-Login,
// Flask-Security, or custom) and blueprint/app `before_request` hooks. The engine
// stamps the reconciled posture onto the view/endpoint (with a raw view-source
// fallback):
//
//   - @login_required (Flask-Login)                  → authenticated.
//   - @roles_required('admin') / @roles_accepted     → role (Flask-Security).
//   - @permission_required('export') (Flask-Principal) → action (permission grant).
//   - @requires_auth / @auth.login_required (custom)  → authenticated.
//   - @app.before_request / blueprint before_request auth → applies to the
//     app/blueprint scope (authenticated, unless the hook names a role).
//   - @admin_required / @superuser_required           → role / superuser.
//   - no auth decorator + no before_request           → public/unknown (honest).
//
// SCOPE PRECEDENCE (most-specific wins — #4540):
//
//  1. VIEW DECORATOR — a @login_required/@roles_required/... on the view function.
//     Applies to THAT view only. Read from the engine-reconciled per-view posture
//     (auth_required/auth_roles/auth_permissions/auth_method) ▸ a view-source
//     decorator scan.
//  2. BLUEPRINT / APP before_request — a `@bp.before_request`/`@app.before_request`
//     hook that enforces auth applies to EVERY view on that blueprint/app without
//     its own decorator. The engine stamps the resolved blueprint scope into
//     auth_required/auth_method=before_request; the resolver honours it when no
//     view decorator is present.
//
// Output normalises into the shared {Kind, Literal} vocabulary so the diff core
// compares a Flask posture against the Django oracle or a NestJS posture directly.
package authposture

import (
	"regexp"
	"strings"
)

type flaskResolver struct{}

func (flaskResolver) Name() string { return "flask" }

var (
	flaskLoginRequiredRe = regexp.MustCompile(`@(?:\w+\.)?login_required\b|@requires_auth\b|@require_login\b|@auth_required\b`)
	flaskRolesRequiredRe = regexp.MustCompile(`@(?:\w+\.)?roles?_(?:required|accepted)\s*\(\s*["']([^"']+)["']`)
	flaskPermRequiredRe  = regexp.MustCompile(`@(?:\w+\.)?permission_required\s*\(\s*(?:Permission\s*\(\s*)?["']([^"']+)["']`)
	flaskAdminRe         = regexp.MustCompile(`@(?:admin_required|superuser_required|staff_required)\b`)
	flaskSuperuserRe     = regexp.MustCompile(`@(?:superuser_required|staff_required)\b`)
	flaskBeforeRequestRe = regexp.MustCompile(`@(?:\w+)\.before_request\b`)
)

// Resolve decodes a Flask auth signal. Recognises the framework when the entity
// carries a Flask framework hint OR a recognisable auth decorator/before_request
// in its props or source. Reconciled props win; source decorators are the fallback.
func (f flaskResolver) Resolve(sig Signal) (Posture, bool) {
	fw := strings.ToLower(firstNonEmpty(sig.Framework, sig.prop("framework")))
	roles := sig.prop("auth_roles")
	perms := firstNonEmpty(sig.prop("auth_permissions"), sig.prop("auth_page"))
	authReq := sig.prop("auth_required")
	method := strings.ToLower(sig.prop("auth_method"))
	decorator := strings.ToLower(sig.prop("auth_decorator"))
	src := sig.Source

	isFlask := strings.Contains(fw, "flask") || strings.Contains(fw, "quart") ||
		strings.Contains(method, "login_required") || strings.Contains(method, "before_request") ||
		strings.Contains(decorator, "login_required") || strings.Contains(decorator, "roles_required") ||
		roles != "" || perms != "" || authReq != "" ||
		hasFlaskAuth(src)
	if !isFlask {
		return Posture{}, false
	}

	// (1) VIEW DECORATOR — reconciled role / permission props, tightest first.
	if decorator != "" {
		if strings.Contains(decorator, "superuser") || strings.Contains(decorator, "staff") {
			return Posture{Kind: KindSuperuser, Detail: "auth_decorator=" + decorator}, true
		}
		if strings.Contains(decorator, "admin") {
			return Posture{Kind: KindRole, Literal: "admin", Detail: "auth_decorator=" + decorator}, true
		}
	}
	if roles != "" {
		if r := firstCSV(roles); strings.EqualFold(r, "superuser") || strings.EqualFold(r, "staff") {
			return Posture{Kind: KindSuperuser, Detail: "auth_roles=" + roles}, true
		}
		return Posture{Kind: KindRole, Literal: firstCSV(roles), Detail: "auth_roles=" + roles}, true
	}
	if perms != "" {
		return Posture{Kind: KindAction, Literal: firstCSV(perms), Detail: "permission_required=" + perms}, true
	}

	// (2) Source-decorator fallback — view-level decorator scan.
	if src != "" {
		if p, ok := decodeFlaskSource(src); ok {
			return p, true
		}
	}

	// (3) Reconciled auth_required / before_request scope.
	switch authReq {
	case "true":
		detail := "auth_required=true"
		if strings.Contains(method, "before_request") {
			detail = "before_request auth hook (blueprint/app scope)"
		}
		return Posture{Kind: KindAuthenticated, Detail: detail}, true
	case "false":
		return Posture{Kind: KindPublic, Detail: "auth_required=false (no auth decorator / before_request)"}, true
	}

	// Recognised as Flask but no decodable gate → unknown (never false-public).
	return Posture{Kind: KindUnknown, Detail: "Flask view with no decodable auth decorator or before_request hook"}, true
}

// decodeFlaskSource scans a view source body for a Flask auth decorator in
// roles ▸ permission ▸ admin/superuser ▸ login_required ▸ before_request priority.
func decodeFlaskSource(src string) (Posture, bool) {
	if m := flaskRolesRequiredRe.FindStringSubmatch(src); m != nil {
		if strings.EqualFold(m[1], "superuser") || strings.EqualFold(m[1], "staff") {
			return Posture{Kind: KindSuperuser, Detail: "@roles_required('" + m[1] + "')"}, true
		}
		return Posture{Kind: KindRole, Literal: m[1], Detail: "@roles_required('" + m[1] + "')"}, true
	}
	if m := flaskPermRequiredRe.FindStringSubmatch(src); m != nil {
		return Posture{Kind: KindAction, Literal: m[1], Detail: "@permission_required('" + m[1] + "')"}, true
	}
	if flaskSuperuserRe.MatchString(src) {
		return Posture{Kind: KindSuperuser, Detail: "@superuser_required"}, true
	}
	if flaskAdminRe.MatchString(src) {
		return Posture{Kind: KindRole, Literal: "admin", Detail: "@admin_required"}, true
	}
	if flaskLoginRequiredRe.MatchString(src) {
		return Posture{Kind: KindAuthenticated, Detail: "@login_required"}, true
	}
	if flaskBeforeRequestRe.MatchString(src) {
		return Posture{Kind: KindAuthenticated, Detail: "before_request auth hook (blueprint/app scope)"}, true
	}
	return Posture{}, false
}

// hasFlaskAuth reports whether src carries a recognisable Flask auth construct.
func hasFlaskAuth(src string) bool {
	if src == "" {
		return false
	}
	return flaskLoginRequiredRe.MatchString(src) ||
		flaskRolesRequiredRe.MatchString(src) ||
		flaskPermRequiredRe.MatchString(src) ||
		flaskAdminRe.MatchString(src) ||
		flaskBeforeRequestRe.MatchString(src)
}
