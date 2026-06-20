// aspnet.go — the ASP.NET Core auth-posture resolver (#4542; replaces the aspnet
// stub) with the EFFECTIVE-ATTRIBUTE PRECEDENCE ladder (method ▸ class ▸ global).
//
// ASP.NET Core expresses action/controller authorization through attributes the
// engine stamps onto the action/controller (with a raw-source fallback):
//
//   - [Authorize]                          → authenticated.
//   - [Authorize(Roles = "Admin")]         → role grant on the first role.
//   - [Authorize(Policy = "CanEdit")]      → role/policy grant on the policy name.
//   - [Authorize(Roles="Admin,Mgr")]       → role grant on the first role.
//   - [AllowAnonymous]                     → public (OVERRIDES an outer [Authorize]).
//   - global app.UseAuthorization() + a
//     FallbackPolicy.RequireAuthenticated  → authenticated default for actions with
//     no attribute of their own.
//
// EFFECTIVE PRECEDENCE (most-specific wins, mirroring ASP.NET Core's own
// evaluation — #4542):
//
//  1. METHOD level — an [Authorize]/[AllowAnonymous] on the ACTION method. A
//     method [AllowAnonymous] ALWAYS wins (it short-circuits authorization for the
//     action even when the controller carries [Authorize]). A method [Authorize]
//     tightens beyond a class one.
//  2. CLASS level — [Authorize]/[AllowAnonymous] on the controller class. Applies
//     to actions WITHOUT their own attribute.
//  3. GLOBAL level — a configured `FallbackPolicy` (app.UseAuthorization with a
//     default policy / AddAuthorization(o => o.FallbackPolicy = ...)). Applies only
//     when NEITHER a method nor a class attribute covers the action.
//
// The attribute is read first from the reconciled props the engine stamps
// (auth_required/auth_roles/auth_policy/auth_method + the aspnet_class_* class-level
// props) and falls back to scanning the action/controller source. Output normalises
// into the shared {Kind, Literal} vocabulary so the diff core compares an ASP.NET
// posture against the Django oracle or a NestJS posture directly.
package authposture

import (
	"regexp"
	"strings"
)

type aspnetResolver struct{}

func (aspnetResolver) Name() string { return "aspnet" }

var (
	// [AllowAnonymous]
	aspnetAllowAnonRe = regexp.MustCompile(`\[\s*AllowAnonymous\s*\]`)
	// [Authorize(Roles = "Admin,Mgr")]  — capture the first role.
	aspnetRolesRe = regexp.MustCompile(`\[\s*Authorize\s*\([^)]*Roles\s*=\s*"([^"]+)"`)
	// [Authorize(Policy = "CanEdit")]  — capture the policy name.
	aspnetPolicyRe = regexp.MustCompile(`\[\s*Authorize\s*\([^)]*Policy\s*=\s*"([^"]+)"`)
	// A bare [Authorize] / [Authorize(AuthenticationSchemes=...)] with no Roles/Policy.
	aspnetAuthorizeRe = regexp.MustCompile(`\[\s*Authorize\b`)
)

// Resolve decodes an ASP.NET Core auth signal. Recognises the framework when the
// entity carries an ASP.NET framework hint OR an [Authorize]/[AllowAnonymous]
// attribute in its props or source. It resolves the EFFECTIVE posture down the
// most-specific-wins ladder (method ▸ class ▸ global).
func (a aspnetResolver) Resolve(sig Signal) (Posture, bool) {
	if !a.recognises(sig) {
		return Posture{}, false
	}

	// (1) METHOD level — most specific. A method [AllowAnonymous] always wins.
	if p, ok := a.methodPosture(sig); ok {
		return p, true
	}
	// (2) CLASS level — applies to actions without their own attribute.
	if p, ok := a.classPosture(sig); ok {
		return p, true
	}
	// (3) GLOBAL level — a configured FallbackPolicy.
	if p, ok := a.globalPosture(sig); ok {
		return p, true
	}

	// Recognised as ASP.NET but no attribute / fallback decodable. An action with
	// NO [Authorize] and no fallback policy is open, but a weak hint that landed us
	// here is reported unknown (never false-public).
	return Posture{Kind: KindUnknown, Detail: "ASP.NET Core action with no decodable [Authorize]/[AllowAnonymous] or fallback policy"}, true
}

// recognises reports whether the signal belongs to the ASP.NET resolver.
func (a aspnetResolver) recognises(sig Signal) bool {
	fw := strings.ToLower(firstNonEmpty(sig.Framework, sig.prop("framework")))
	method := strings.ToLower(sig.prop("auth_method"))
	return strings.Contains(fw, "aspnet") || strings.Contains(fw, "asp.net") ||
		strings.Contains(fw, "dotnet") || strings.Contains(fw, "csharp") ||
		strings.Contains(method, "authorize") || method == "aspnet" ||
		sig.prop("auth_roles") != "" || sig.prop("auth_policy") != "" ||
		sig.prop("auth_required") != "" ||
		sig.prop("aspnet_class_authorize") != "" || sig.prop("aspnet_class_allow_anonymous") != "" ||
		sig.prop("aspnet_fallback_policy") != "" ||
		hasAspnetAttribute(sig.Source)
}

// --- (1) METHOD level -------------------------------------------------------

// methodPosture resolves the method-level (action) posture. A method
// [AllowAnonymous] is the highest-priority override; then a method [Authorize]
// with Roles/Policy; then the engine's reconciled per-action auth props; then a
// source scan of the action body.
func (a aspnetResolver) methodPosture(sig Signal) (Posture, bool) {
	// (1a) Method [AllowAnonymous] override — wins over everything below.
	if sig.prop("allow_anonymous") == "true" || aspnetAllowAnonRe.MatchString(actionSrc(sig.Source)) {
		return Posture{Kind: KindPublic, Detail: "[AllowAnonymous] (method override)"}, true
	}
	// (1b) Reconciled method roles / policy props.
	if roles := sig.prop("auth_roles"); roles != "" {
		return aspnetRolePosture(firstCSV(roles), "auth_roles="+roles), true
	}
	if pol := sig.prop("auth_policy"); pol != "" {
		return Posture{Kind: KindRole, Literal: pol, Detail: "auth_policy=" + pol}, true
	}
	// (1c) Source-attribute fallback on the action body.
	if p, ok := decodeAspnetSource(actionSrc(sig.Source), "method"); ok {
		return p, true
	}
	// (1d) Reconciled auth_required without a decodable attribute.
	switch sig.prop("auth_required") {
	case "true":
		return Posture{Kind: KindAuthenticated, Detail: "auth_required=true ([Authorize], no Roles/Policy)"}, true
	case "false":
		return Posture{Kind: KindPublic, Detail: "auth_required=false ([AllowAnonymous] / no [Authorize])"}, true
	}
	return Posture{}, false
}

// --- (2) CLASS level --------------------------------------------------------

// classPosture resolves the controller-class-level posture, applied to actions
// without their own attribute. A class [AllowAnonymous] is public; a class
// [Authorize(Roles/Policy)] grants the role/policy. Read from the aspnet_class_*
// props the engine stamps from the controller attribute block.
func (a aspnetResolver) classPosture(sig Signal) (Posture, bool) {
	if sig.prop("aspnet_class_allow_anonymous") == "true" {
		return Posture{Kind: KindPublic, Detail: "class [AllowAnonymous]"}, true
	}
	if roles := sig.prop("aspnet_class_roles"); roles != "" {
		return aspnetRolePosture(firstCSV(roles), "class [Authorize(Roles="+roles+")]"), true
	}
	if pol := sig.prop("aspnet_class_policy"); pol != "" {
		return Posture{Kind: KindRole, Literal: pol, Detail: "class [Authorize(Policy=" + pol + ")]"}, true
	}
	if ca := sig.prop("aspnet_class_authorize"); ca != "" {
		if p, ok := decodeAspnetSource(ca, "class"); ok {
			return p, true
		}
		return Posture{Kind: KindAuthenticated, Detail: "class [Authorize]"}, true
	}
	return Posture{}, false
}

// --- (3) GLOBAL level -------------------------------------------------------

// globalPosture resolves the global FallbackPolicy the engine stamps when
// app.UseAuthorization() configures a default authenticated policy. Applies only
// when no method/class attribute covers the action.
func (a aspnetResolver) globalPosture(sig Signal) (Posture, bool) {
	fb := strings.ToLower(sig.prop("aspnet_fallback_policy"))
	if fb == "" {
		return Posture{}, false
	}
	if strings.Contains(fb, "anonymous") || strings.Contains(fb, "permitall") || strings.Contains(fb, "allowall") {
		return Posture{Kind: KindPublic, Detail: "global FallbackPolicy=" + fb}, true
	}
	// RequireAuthenticatedUser / any non-anonymous fallback → authenticated default.
	return Posture{Kind: KindAuthenticated, Detail: "global FallbackPolicy (RequireAuthenticatedUser)"}, true
}

// --- shared decoders --------------------------------------------------------

// decodeAspnetSource scans a source body for an ASP.NET attribute in
// [AllowAnonymous] ▸ [Authorize(Roles)] ▸ [Authorize(Policy)] ▸ [Authorize]
// priority order. tier is "method" or "class" (for the detail string only).
func decodeAspnetSource(src, tier string) (Posture, bool) {
	if src == "" {
		return Posture{}, false
	}
	if aspnetAllowAnonRe.MatchString(src) {
		return Posture{Kind: KindPublic, Detail: tier + " [AllowAnonymous]"}, true
	}
	if m := aspnetRolesRe.FindStringSubmatch(src); m != nil {
		return aspnetRolePosture(firstCSV(m[1]), tier+" [Authorize(Roles=\""+m[1]+"\")]"), true
	}
	if m := aspnetPolicyRe.FindStringSubmatch(src); m != nil {
		return Posture{Kind: KindRole, Literal: m[1], Detail: tier + " [Authorize(Policy=\"" + m[1] + "\")]"}, true
	}
	if aspnetAuthorizeRe.MatchString(src) {
		return Posture{Kind: KindAuthenticated, Detail: tier + " [Authorize]"}, true
	}
	return Posture{}, false
}

// aspnetRolePosture builds a role posture, folding a superuser/admin-root role to
// KindSuperuser the same way the Spring resolver folds SUPERUSER/ROOT.
func aspnetRolePosture(role, detail string) Posture {
	role = strings.TrimSpace(role)
	if strings.EqualFold(role, "Superuser") || strings.EqualFold(role, "Root") || strings.EqualFold(role, "SuperAdmin") {
		return Posture{Kind: KindSuperuser, Detail: detail}
	}
	return Posture{Kind: KindRole, Literal: role, Detail: detail}
}

// actionSrc returns the source body for method-level scanning (the whole stamped
// source; the action attribute and its body are co-located).
func actionSrc(src string) string { return src }

// hasAspnetAttribute reports whether src carries a recognisable ASP.NET attribute.
func hasAspnetAttribute(src string) bool {
	if src == "" {
		return false
	}
	return aspnetAllowAnonRe.MatchString(src) || aspnetAuthorizeRe.MatchString(src)
}
