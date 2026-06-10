// nestjs.go — the NestJS auth-posture resolver (ticket #4422, flagship pair).
//
// NestJS expresses endpoint authorization through guard decorators plus the
// reconciled posture properties the engine stamps onto the handler/endpoint:
//
//   - auth_required   : "true" when a guard enforces authentication.
//   - auth_method     : the guard mechanism (e.g. "jwt", "guard").
//   - auth_guard      : the guard class name(s) (e.g. "PageGuard", "RolesGuard").
//   - auth_roles      : comma-joined role literals from @Roles(...).
//   - auth_scopes     : comma-joined scope literals.
//
// plus the v3-specific @Require* decorator literals the rewrite uses to mirror
// the Django page/action grants:
//
//   - @RequirePage("client_admin")   → page grant on that slug.
//   - @RequireAction("export")       → action grant on that codename.
//   - @Public()                      → public.
//   - @Authenticated() / @UseGuards  → authenticated-only.
//   - @RequireSuperuser()            → superuser.
//
// The decorator literals are read from the require_page / require_action /
// require_superuser / is_public properties the engine reconciled (these are the
// "now-accurate reconciled posture" the ticket references), with a source-text
// fallback that scans the handler decorators directly when the properties are
// absent. The output normalises into the same {Kind, Literal} vocabulary the
// Django resolver targets, so the diff core compares them directly.
package authposture

import (
	"regexp"
	"strings"
)

type nestJSResolver struct{}

func (nestJSResolver) Name() string { return "nestjs" }

var (
	requirePageRe       = regexp.MustCompile(`@RequirePage\s*\(\s*["']([^"']+)["']`)
	requireActionRe     = regexp.MustCompile(`@RequireAction\s*\(\s*["']([^"']+)["']`)
	requireRoleRe       = regexp.MustCompile(`@(?:Roles|RequireRole)\s*\(\s*["']([^"']+)["']`)
	requireSuperuserRe  = regexp.MustCompile(`@RequireSuperuser\s*\(`)
	publicDecoratorRe   = regexp.MustCompile(`@Public\s*\(`)
	authenticatedDecoRe = regexp.MustCompile(`@Authenticated\s*\(|@UseGuards\s*\(`)
)

// Resolve decodes a NestJS auth signal. Recognises the framework when the entity
// carries any Nest auth property OR Nest @Require*/@Public/@UseGuards decorators
// in its source. Property-derived posture wins; source decorators are the
// fallback.
func (n nestJSResolver) Resolve(sig Signal) (Posture, bool) {
	isNest := sig.prop("require_page") != "" ||
		sig.prop("require_action") != "" ||
		sig.prop("require_superuser") != "" ||
		sig.prop("is_public") != "" ||
		sig.prop("auth_guard") != "" ||
		sig.prop("auth_required") != "" ||
		hasNestDecorator(sig.Source)
	if !isNest {
		return Posture{}, false
	}

	// (1) Property-derived reconciled posture — tightest grant first.
	if sig.prop("require_superuser") == "true" {
		return Posture{Kind: KindSuperuser, Detail: "@RequireSuperuser (reconciled prop)"}, true
	}
	if v := sig.prop("require_page"); v != "" {
		return Posture{Kind: KindPage, Literal: v, Detail: "@RequirePage(" + v + ") (reconciled prop)"}, true
	}
	if v := sig.prop("require_action"); v != "" {
		return Posture{Kind: KindAction, Literal: v, Detail: "@RequireAction(" + v + ") (reconciled prop)"}, true
	}
	if v := sig.prop("auth_roles"); v != "" {
		return Posture{Kind: KindRole, Literal: firstCSV(v), Detail: "auth_roles=" + v}, true
	}
	if v := sig.prop("auth_scopes"); v != "" {
		return Posture{Kind: KindScope, Literal: firstCSV(v), Detail: "auth_scopes=" + v}, true
	}
	if sig.prop("is_public") == "true" {
		return Posture{Kind: KindPublic, Detail: "@Public (reconciled prop)"}, true
	}
	if sig.prop("auth_required") == "true" || sig.prop("auth_guard") != "" {
		return Posture{Kind: KindAuthenticated, Detail: "auth_required guard=" + sig.prop("auth_guard")}, true
	}

	// (2) Source-decorator fallback — same priority order.
	if src := sig.Source; src != "" {
		if requireSuperuserRe.MatchString(src) {
			return Posture{Kind: KindSuperuser, Detail: "@RequireSuperuser (decorator)"}, true
		}
		if m := requirePageRe.FindStringSubmatch(src); m != nil {
			return Posture{Kind: KindPage, Literal: m[1], Detail: "@RequirePage(" + m[1] + ")"}, true
		}
		if m := requireActionRe.FindStringSubmatch(src); m != nil {
			return Posture{Kind: KindAction, Literal: m[1], Detail: "@RequireAction(" + m[1] + ")"}, true
		}
		if m := requireRoleRe.FindStringSubmatch(src); m != nil {
			return Posture{Kind: KindRole, Literal: m[1], Detail: "@Roles(" + m[1] + ")"}, true
		}
		if publicDecoratorRe.MatchString(src) {
			return Posture{Kind: KindPublic, Detail: "@Public (decorator)"}, true
		}
		if authenticatedDecoRe.MatchString(src) {
			return Posture{Kind: KindAuthenticated, Detail: "@Authenticated/@UseGuards (decorator)"}, true
		}
	}

	// Recognised as Nest but no decodable grant → unknown (never false-public).
	return Posture{Kind: KindUnknown, Detail: "NestJS handler with no decodable auth decorator"}, true
}

// hasNestDecorator reports whether src carries a recognisable Nest auth decorator.
func hasNestDecorator(src string) bool {
	if src == "" {
		return false
	}
	return requirePageRe.MatchString(src) ||
		requireActionRe.MatchString(src) ||
		requireSuperuserRe.MatchString(src) ||
		requireRoleRe.MatchString(src) ||
		publicDecoratorRe.MatchString(src) ||
		authenticatedDecoRe.MatchString(src)
}

// firstCSV returns the first comma-separated token, trimmed.
func firstCSV(s string) string {
	if i := strings.IndexByte(s, ','); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
