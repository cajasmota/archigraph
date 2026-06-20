// rails.go — the Rails auth-posture resolver (#4538; replaces the rails stub).
//
// Rails expresses controller authorization through `before_action` filter
// callbacks declared at CLASS level (optionally scoped to a subset of actions
// with `only:`/`except:`), plus authorization libraries (Pundit, CanCanCan)
// invoked per-action. The engine stamps the reconciled posture onto the
// handler/endpoint (with a raw controller-source fallback):
//
//   - before_action :authenticate_user!                  → authenticated (class,
//     applies to every action unless only:/except: scopes it).
//   - skip_before_action :authenticate_user!             → public OVERRIDE on the
//     scoped actions (the most-specific wins — see precedence below).
//   - before_action :require_admin / :require_superuser  → role / superuser.
//   - Pundit `authorize @x` / `verify_authorized`        → policy (action grant).
//   - CanCanCan `load_and_authorize_resource` /
//     `authorize! :action, @x`                            → policy (action grant).
//   - no auth before_action + no skip                    → public/unknown (honest:
//     a weak hint that landed us here is unknown, never false-public).
//
// EFFECTIVE PRECEDENCE (most-specific wins, mirroring how Rails evaluates the
// filter chain for a given action — #4538):
//
//  1. PER-ACTION override — a `skip_before_action :authenticate_user!` that names
//     THIS action (via only:) opens it (public), or a per-action Pundit/CanCanCan
//     `authorize`/`authorize!` grant. Applies to THAT action only.
//  2. CLASS before_action — the controller-level `before_action :authenticate_user!`
//     (and :require_admin etc.), honoring its only:/except: action scoping.
//
// Action scoping: a `before_action ..., only: [:create]` applies ONLY to create;
// a `..., except: [:index]` applies to every action but index. The resolver
// reads the engine-reconciled per-action posture (auth_required/auth_roles/
// auth_method/auth_guard) first — the engine resolves only/except scoping when it
// stamps the per-action posture — and falls back to scanning the controller source
// for the action when the props are absent. Output normalises into the shared
// {Kind, Literal} vocabulary so the diff core compares a Rails posture against the
// Django oracle or a NestJS posture directly.
package authposture

import (
	"regexp"
	"strings"
)

type railsResolver struct{}

func (railsResolver) Name() string { return "rails" }

var (
	// before_action :authenticate_user!  (Devise) / :authenticate / :require_login.
	// Anchored to the start of a line (after optional indent) so the `before_action`
	// substring inside a `skip_before_action` line does NOT match.
	railsAuthBeforeRe = regexp.MustCompile(`(?m)^\s*before_action\s+:?\s*(authenticate_user!?|authenticate!?|require_login|login_required|require_user)`)
	// skip_before_action :authenticate_user!  → public override.
	railsSkipAuthRe = regexp.MustCompile(`skip_before_action\s+:?\s*(?:authenticate_user!?|authenticate!?|require_login|login_required|require_user)`)
	// before_action :require_admin / :require_superuser / :ensure_admin → role/superuser.
	railsAdminBeforeRe = regexp.MustCompile(`(?m)^\s*before_action\s+:?\s*(require_admin|ensure_admin|require_superuser|admin_only|authorize_admin|require_staff)`)
	// before_action :require_role_<x> / :require_<role>_role → role.
	railsRoleBeforeRe = regexp.MustCompile(`(?m)^\s*before_action\s+:?\s*require_([a-z0-9_]+?)_?role\b`)
	// Pundit: authorize @record  /  authorize @record, :update?  /  verify_authorized
	railsPunditAuthorizeRe  = regexp.MustCompile(`\bauthorize\s+@?[\w.]+(?:\s*,\s*:([\w?]+))?`)
	railsVerifyAuthorizedRe = regexp.MustCompile(`\bverify_authorized\b|\bpolicy_scope\b`)
	// CanCanCan: authorize! :update, @x  /  load_and_authorize_resource  /  authorize_resource
	railsCanCanBangRe    = regexp.MustCompile(`\bauthorize!\s+:([\w?]+)`)
	railsLoadAuthorizeRe = regexp.MustCompile(`\bload_and_authorize_resource\b|\bauthorize_resource\b`)
)

// Resolve decodes a Rails auth signal. Recognises the framework when the entity
// carries a Rails framework hint OR a before_action/Pundit/CanCanCan construct in
// its props or source. Reconciled per-action props win (the engine already
// resolves only:/except: scoping); source scanning is the fallback.
func (r railsResolver) Resolve(sig Signal) (Posture, bool) {
	fw := strings.ToLower(firstNonEmpty(sig.Framework, sig.prop("framework")))
	roles := sig.prop("auth_roles")
	authReq := sig.prop("auth_required")
	method := strings.ToLower(sig.prop("auth_method"))
	guard := sig.prop("auth_guard")
	src := sig.Source

	isRails := strings.Contains(fw, "rails") || strings.Contains(fw, "ruby") ||
		strings.Contains(method, "before_action") || strings.Contains(method, "pundit") ||
		strings.Contains(method, "cancancan") || strings.Contains(method, "devise") ||
		sig.prop("pundit_policy") != "" || sig.prop("cancancan_ability") != "" ||
		roles != "" || authReq != "" ||
		hasRailsAuth(src)
	if !isRails {
		return Posture{}, false
	}

	// (1) PER-ACTION OVERRIDE — a skip_before_action that opens THIS action.
	// The engine stamps auth_required=false / auth_method=skip_before_action when a
	// skip covers the action; the source fallback detects a skip line.
	if authReq == "false" && (strings.Contains(method, "skip") || method == "" || method == "skip_before_action") {
		return Posture{Kind: KindPublic, Detail: "skip_before_action :authenticate_user! (public override)"}, true
	}
	if src != "" && railsSkipAuthRe.MatchString(src) && !railsAuthBeforeRe.MatchString(src) {
		return Posture{Kind: KindPublic, Detail: "skip_before_action :authenticate_user!"}, true
	}

	// (2) Reconciled role / superuser props (engine-flattened before_action role).
	if guard != "" {
		if p, ok := decodeRailsRoleToken(guard, "auth_guard="+guard); ok {
			return p, true
		}
	}
	if roles != "" {
		if p, ok := decodeRailsRoleToken(roles, "auth_roles="+roles); ok {
			return p, true
		}
		return Posture{Kind: KindRole, Literal: firstCSV(roles), Detail: "auth_roles=" + roles}, true
	}

	// (3) Pundit / CanCanCan reconciled props → policy (action grant).
	if pol := sig.prop("pundit_policy"); pol != "" {
		if act := sig.prop("pundit_action"); act != "" {
			return Posture{Kind: KindAction, Literal: stripRubyQ(act), Detail: "Pundit authorize " + pol + "#" + act}, true
		}
		return Posture{Kind: KindAction, Literal: railsPolicyLiteral(pol, sig.Action), Detail: "Pundit policy " + pol}, true
	}
	if ab := sig.prop("cancancan_ability"); ab != "" {
		return Posture{Kind: KindAction, Literal: stripRubyQ(ab), Detail: "CanCanCan ability " + ab}, true
	}

	// (4) Source fallback — scan the controller/action body.
	if src != "" {
		if p, ok := decodeRailsSource(src, sig.Action); ok {
			return p, true
		}
	}

	// (5) Reconciled auth_required without a decodable filter.
	switch authReq {
	case "true":
		return Posture{Kind: KindAuthenticated, Detail: "auth_required=true (before_action authenticate, no decodable role/policy)"}, true
	case "false":
		return Posture{Kind: KindPublic, Detail: "auth_required=false (no auth before_action)"}, true
	}

	// Recognised as Rails but no decodable filter → unknown (never false-public).
	return Posture{Kind: KindUnknown, Detail: "Rails controller with no decodable before_action / Pundit / CanCanCan grant"}, true
}

// decodeRailsSource scans a controller/action source body, honouring the
// per-action ▸ class precedence: a skip_before_action override first, then a
// per-action Pundit/CanCanCan grant, then the class before_action.
func decodeRailsSource(src, action string) (Posture, bool) {
	// skip_before_action (when no competing class authenticate is also in scope).
	if railsSkipAuthRe.MatchString(src) && !railsAuthBeforeRe.MatchString(src) {
		return Posture{Kind: KindPublic, Detail: "skip_before_action :authenticate_user!"}, true
	}
	// admin/superuser before_action.
	if m := railsAdminBeforeRe.FindStringSubmatch(src); m != nil {
		if strings.Contains(m[1], "superuser") || strings.Contains(m[1], "staff") {
			return Posture{Kind: KindSuperuser, Detail: "before_action :" + m[1]}, true
		}
		return Posture{Kind: KindRole, Literal: "admin", Detail: "before_action :" + m[1]}, true
	}
	if m := railsRoleBeforeRe.FindStringSubmatch(src); m != nil {
		return Posture{Kind: KindRole, Literal: m[1], Detail: "before_action :require_" + m[1] + "_role"}, true
	}
	// CanCanCan authorize! :action  /  load_and_authorize_resource.
	if m := railsCanCanBangRe.FindStringSubmatch(src); m != nil {
		return Posture{Kind: KindAction, Literal: stripRubyQ(m[1]), Detail: "authorize! :" + m[1] + " (CanCanCan)"}, true
	}
	if railsLoadAuthorizeRe.MatchString(src) {
		return Posture{Kind: KindAction, Literal: railsActionLiteral(action), Detail: "load_and_authorize_resource (CanCanCan)"}, true
	}
	// Pundit authorize @x[, :action?]  /  verify_authorized.
	if m := railsPunditAuthorizeRe.FindStringSubmatch(src); m != nil {
		lit := stripRubyQ(m[1])
		if lit == "" {
			lit = railsActionLiteral(action)
		}
		return Posture{Kind: KindAction, Literal: lit, Detail: "Pundit authorize"}, true
	}
	if railsVerifyAuthorizedRe.MatchString(src) {
		return Posture{Kind: KindAction, Literal: railsActionLiteral(action), Detail: "Pundit verify_authorized"}, true
	}
	// Class before_action authenticate (least specific).
	if railsAuthBeforeRe.MatchString(src) {
		return Posture{Kind: KindAuthenticated, Detail: "before_action :authenticate_user!"}, true
	}
	return Posture{}, false
}

// decodeRailsRoleToken classifies an engine-flattened role/guard token (the value
// of a :require_admin / :require_<role> before_action) into role/superuser.
func decodeRailsRoleToken(tok, detail string) (Posture, bool) {
	low := strings.ToLower(tok)
	if strings.Contains(low, "superuser") || strings.Contains(low, "staff") || strings.Contains(low, "root") {
		return Posture{Kind: KindSuperuser, Detail: detail}, true
	}
	if strings.Contains(low, "admin") {
		return Posture{Kind: KindRole, Literal: "admin", Detail: detail}, true
	}
	if m := railsRoleTokenRe.FindStringSubmatch(tok); m != nil {
		return Posture{Kind: KindRole, Literal: m[1], Detail: detail}, true
	}
	return Posture{}, false
}

// railsRoleTokenRe matches a bare engine-flattened role token like
// `require_editor_role` (no `before_action` prefix), capturing the role name.
var railsRoleTokenRe = regexp.MustCompile(`require_([a-z0-9_]+?)_?role\b`)

// railsPolicyLiteral derives a policy action literal: prefer the DRF-style action
// the posture is being resolved for, else the policy class name.
func railsPolicyLiteral(policy, action string) string {
	if a := railsActionLiteral(action); a != "" {
		return a
	}
	return strings.TrimSuffix(policy, "Policy")
}

// railsActionLiteral returns the action name as the policy/ability codename when
// present (Pundit/CanCanCan grant the controller action).
func railsActionLiteral(action string) string {
	return strings.TrimSpace(action)
}

// stripRubyQ trims a Ruby symbol/string token's leading `:` and trailing `?`
// and surrounding quotes (`:update?` → `update`, `"create"` → `create`).
func stripRubyQ(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	s = strings.TrimPrefix(s, ":")
	s = strings.TrimSuffix(s, "?")
	return s
}

// hasRailsAuth reports whether src carries a recognisable Rails auth construct.
func hasRailsAuth(src string) bool {
	if src == "" {
		return false
	}
	return railsAuthBeforeRe.MatchString(src) ||
		railsSkipAuthRe.MatchString(src) ||
		railsAdminBeforeRe.MatchString(src) ||
		railsRoleBeforeRe.MatchString(src) ||
		railsPunditAuthorizeRe.MatchString(src) ||
		railsVerifyAuthorizedRe.MatchString(src) ||
		railsCanCanBangRe.MatchString(src) ||
		railsLoadAuthorizeRe.MatchString(src)
}
