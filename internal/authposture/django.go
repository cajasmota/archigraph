// django.go — the Django DRF auth-posture resolver, encoding the §10
// get_permissions DECODE CONTRACT (ticket #4422) exactly, plus the EFFECTIVE
// permission precedence (ticket #4675 — the DRF analog of the NestJS
// effective-guard fix #4667/#4676).
//
// EFFECTIVE PERMISSION PRECEDENCE (most-specific wins, mirroring how DRF itself
// resolves a request's permissions — #4675):
//
//  1. METHOD/ACTION level — a `@action(detail=…, permission_classes=[X])`
//     decorator on a ViewSet extra-action, OR the per-action arm of a branchy
//     `get_permissions(self)` keyed on `self.action`. Applies to THAT action
//     only; siblings keep the class default. (DRF resolves `get_permissions`
//     per request, and the @action kwarg overrides the class attribute for that
//     route.) NOTE the framework asymmetry: an APIView `get`/`post` method
//     CANNOT carry its own `permission_classes` (APIView is class-scoped), so for
//     an APIView the class value is already the effective one — only ViewSet
//     @action carries a method-level override.
//  2. CLASS level — the ViewSet/APIView `permission_classes = [...]` attribute
//     (or the else/default arm of `get_permissions`). Applies to actions without
//     their own override.
//  3. GLOBAL default — REST_FRAMEWORK["DEFAULT_PERMISSION_CLASSES"], harvested as
//     the `drf_default_permission_classes` Signal prop. Applies when NEITHER a
//     method nor a class permission is specified.
//
// The empty-vs-AllowAny distinction is load-bearing for the precedence:
//   - `permission_classes=[AllowAny]` ▸ PUBLIC, overriding the global default.
//   - `permission_classes=[]` (empty) ▸ NO explicit grant → falls through to the
//     GLOBAL default (NOT public). The extractor stamps no `permission_classes`
//     prop for an empty list, so "absent prop" naturally means "fall to global".
//
// The oracle's authorization for a ViewSet action lives in a branchy
// get_permissions(self) override that returns a different permission list per
// self.action. The §10 contract maps each branch arm to an effective grant:
//
//	| pattern in get_permissions                          | resolves to          |
//	|-----------------------------------------------------|----------------------|
//	| CustomPagePermissionCheck(PERMISSION_PAGES[X])      | page grant on slug X |
//	| else: ... CustomActionPermissionCheck               | action grant (DEFAULT|
//	|                                                     |  arm — the fallthrough)|
//	| bare [IsAuthenticated] / TODO arm                   | authenticated-only   |
//	| elif self.action == [<list>]  (scalar == list)      | DEAD CODE → falls    |
//	|                                                     |  through to else      |
//	| elif self.action in [<list>]                        | live arm per body     |
//	| superuser check (is_superuser / IsAdminUser)        | superuser            |
//
// CORRECTNESS IS LOAD-BEARING. Two mis-decodes produce a FALSE `equivalent`
// that hides a real RBAC regression, so they are encoded with care:
//
//   - The `else` arm is the DEFAULT ACTION GRANT, NOT authenticated-only. A
//     resolver that returns authenticated for the else arm would call a v3
//     @RequireAction "equivalent" to an authenticated-only oracle.
//   - `self.action == [list]` (scalar compared to a LIST) can NEVER be true, so
//     that arm is DEAD CODE and the action falls through to whatever a live arm
//     or the else catches. Evaluating it as live would attribute the wrong grant
//     to the action. (`in [list]` is the live form.)
//
// Verified live on ClientViewSet.get_permissions. The decode is purely textual
// (no Python AST dependency) so it runs identically on the get_permissions
// source body the MCP tool harvests from the graph.
package authposture

import (
	"regexp"
	"strings"
)

// djangoDRFResolver implements the §10 get_permissions decode plus the simpler
// class-attribute permission_classes posture.
type djangoDRFResolver struct{}

func (djangoDRFResolver) Name() string { return "django-drf" }

// pagePermRe matches a page-permission check capturing the slug key, e.g.
//
//	CustomPagePermissionCheck(PERMISSION_PAGES["client_admin"])
//	CustomPagePermissionCheck(PERMISSION_PAGES['client-admin'])
//	CustomPagePermissionCheck(PERMISSION_PAGES.CLIENT_ADMIN)
var pagePermRe = regexp.MustCompile(`CustomPagePermissionCheck\s*\(\s*PERMISSION_PAGES\s*(?:\[\s*["']([^"']+)["']\s*\]|\.\s*([A-Za-z0-9_]+))`)

// actionPermRe matches an action-permission check capturing an optional
// codename literal, e.g. CustomActionPermissionCheck("export_clients").
var actionPermRe = regexp.MustCompile(`CustomActionPermissionCheck\s*\(\s*(?:["']([^"']+)["'])?`)

// superuserRe matches the common DRF superuser/staff gate signatures.
var superuserRe = regexp.MustCompile(`is_superuser|IsAdminUser|IsSuperUser`)

// authenticatedRe matches the authenticated-only marker.
var authenticatedRe = regexp.MustCompile(`IsAuthenticated`)

// allowAnyRe matches the explicit public marker.
var allowAnyRe = regexp.MustCompile(`AllowAny`)

// elifScalarEqListRe matches the DEAD-CODE arm `... self.action == [ ... ]`
// (scalar action compared to a LIST literal — never true). The `==` with a
// bracket RHS is the dead form; `in [ ... ]` is the live form and is NOT matched
// here.
var elifScalarEqListRe = regexp.MustCompile(`self\.action\s*==\s*\[`)

// Resolve decodes a Django DRF auth signal. It recognises the framework when
// the entity carries any DRF permission marker (has_get_permissions,
// has_permission_classes, permission_classes, get_permissions_classes) OR a
// get_permissions source body. Returns ok=false otherwise so the registry tries
// the next resolver.
func (d djangoDRFResolver) Resolve(sig Signal) (Posture, bool) {
	isDRF := sig.hasProp("has_get_permissions") ||
		sig.hasProp("has_permission_classes") ||
		sig.prop("permission_classes") != "" ||
		sig.prop("get_permissions_classes") != "" ||
		sig.prop("drf_default_permission_classes") != "" ||
		looksLikeGetPermissions(sig.Source)
	if !isDRF {
		return Posture{}, false
	}

	// (1) METHOD/ACTION level — most specific, wins over class + global (#4675).
	//
	// (1a) The branchy `get_permissions(self)` per-action decode IS a method-level
	// override (it returns a different list per self.action). The §10 decoder
	// attributes the effective grant to sig.Action; its else/default arm is the
	// CLASS-level fallthrough for un-named actions.
	if looksLikeGetPermissions(sig.Source) {
		if p, ok := decodeGetPermissions(sig.Source, sig.Action); ok {
			return p, true
		}
	}
	// (1b) An `@action(..., permission_classes=[X])` decorator override. The
	// action endpoint entity carries the per-action `permission_classes` the
	// extractor stamped from the decorator kwarg; it overrides the class default
	// for THAT action only. (Siblings without the decorator carry no
	// `permission_classes` prop and fall through to class/global below.) An empty
	// `permission_classes=[]` stamps NO prop, so it correctly does NOT match here
	// and falls to the global default — the empty-vs-AllowAny distinction.
	if v := sig.prop("permission_classes"); v != "" {
		return decodePermissionClasses(v), true
	}

	// (2) CLASS level — the ViewSet/APIView class-attribute permission list the
	// extractor stamps (get_permissions_classes when a non-branchy get_permissions
	// referenced classes, else the class permission_classes attribute, already
	// handled at (1b)).
	if v := sig.prop("get_permissions_classes"); v != "" {
		return decodePermissionClasses(v), true
	}

	// (3) GLOBAL default — REST_FRAMEWORK["DEFAULT_PERMISSION_CLASSES"]. Applies
	// only when neither a method (1) nor a class (2) permission was specified.
	if v := sig.prop("drf_default_permission_classes"); v != "" {
		p := decodePermissionClasses(v)
		p.Detail = "global DEFAULT_PERMISSION_CLASSES: " + v + " (" + string(p.Kind) + ")"
		return p, true
	}

	// Recognised as DRF but no method/class/global permission resolved → unknown
	// (never false-public).
	return Posture{Kind: KindUnknown, Detail: "DRF endpoint with no resolvable permission_classes / get_permissions / global default"}, true
}

// looksLikeGetPermissions reports whether src is a get_permissions method body.
func looksLikeGetPermissions(src string) bool {
	return strings.Contains(src, "get_permissions") ||
		(strings.Contains(src, "self.action") && strings.Contains(src, "return"))
}

// decodeGetPermissions is the §10 decoder. It walks the get_permissions body
// arm-by-arm, attributing the effective grant to `action`. When action is empty
// it returns the DEFAULT (else-arm) grant — the posture an un-named action
// receives. Returns ok=false only when the body has no decodable permission
// signal at all (caller then falls back to class-attribute decode).
//
// Arm model: the body is split into guarded arms (if/elif … ) plus the trailing
// else/default. For the target action we find the FIRST LIVE arm whose guard
// matches it; dead `== [list]` arms are skipped (the action falls through).
// If no live arm matches, the else/default arm applies.
func decodeGetPermissions(src, action string) (Posture, bool) {
	arms := splitArms(src)
	if len(arms) == 0 {
		return Posture{}, false
	}

	var elseArm *permArm
	for i := range arms {
		a := &arms[i]
		if a.isElse {
			elseArm = a
			continue
		}
		// DEAD-CODE arm: `self.action == [list]` can never be true. Skip — the
		// action falls through to the next arm / else (§10 dead-code rule).
		if a.deadScalarEqList {
			continue
		}
		// Live arm: does its guard name this action?
		if action != "" && a.matchesAction(action) {
			return decodeArmBody(a.body, "matched live arm: "+a.guard), true
		}
	}

	// No live arm matched (or no action given) → the else/default arm. Per §10
	// the else arm is the DEFAULT ACTION GRANT, not authenticated-only.
	if elseArm != nil {
		return decodeArmBody(elseArm.body, "else/default arm"), true
	}

	// No else arm: decode the whole body as a flat return (e.g. a single-line
	// get_permissions that just returns a list).
	return decodeArmBody(src, "flat get_permissions body"), true
}

// permArm is one branch of a get_permissions if/elif/else chain.
type permArm struct {
	guard            string // the condition text (empty for else)
	body             string // the arm's return/body text
	isElse           bool
	deadScalarEqList bool // guard is `self.action == [list]` (dead code)
}

// matchesAction reports whether this arm's guard selects the named action via a
// live `self.action == "x"`, `self.action in [..., "x", ...]`, or membership in
// a captured list. Dead `== [list]` arms are excluded upstream.
func (a *permArm) matchesAction(action string) bool {
	g := a.guard
	// self.action == "x"  (scalar == scalar — live)
	if m := regexp.MustCompile(`self\.action\s*==\s*["']([^"']+)["']`).FindStringSubmatch(g); m != nil {
		return m[1] == action
	}
	// self.action in [ "a", "b", ... ]  (live membership)
	if strings.Contains(g, "self.action") && strings.Contains(g, " in ") {
		for _, lit := range stringLiterals(g) {
			if lit == action {
				return true
			}
		}
	}
	return false
}

// splitArms parses a get_permissions body into ordered arms. It is a textual,
// indentation-tolerant splitter (not a full Python parser): it scans lines,
// opening a new arm on `if`/`elif`/`else`, accumulating subsequent lines as the
// arm body until the next arm keyword at the same-or-lesser indent.
func splitArms(src string) []permArm {
	lines := strings.Split(src, "\n")
	var arms []permArm
	var cur *permArm

	ifRe := regexp.MustCompile(`^\s*(if|elif)\b(.*?):\s*$`)
	elseRe := regexp.MustCompile(`^\s*else\s*:\s*$`)

	flush := func() {
		if cur != nil {
			cur.body = strings.TrimSpace(cur.body)
			arms = append(arms, *cur)
			cur = nil
		}
	}

	for _, ln := range lines {
		switch {
		case ifRe.MatchString(ln):
			flush()
			m := ifRe.FindStringSubmatch(ln)
			guard := strings.TrimSpace(m[2])
			cur = &permArm{
				guard:            guard,
				deadScalarEqList: elifScalarEqListRe.MatchString(guard),
			}
		case elseRe.MatchString(ln):
			flush()
			cur = &permArm{isElse: true}
		default:
			if cur != nil {
				cur.body += ln + "\n"
			}
		}
	}
	flush()
	return arms
}

// decodeArmBody classifies a single arm's body text into a Posture per §10
// priority: superuser > page > action > authenticated > public > unknown.
// Superuser is checked first (tightest), then the specific page/action grants,
// then the bare authenticated/public markers.
func decodeArmBody(body, detail string) Posture {
	if superuserRe.MatchString(body) {
		return Posture{Kind: KindSuperuser, Detail: detail + ": superuser check"}
	}
	if m := pagePermRe.FindStringSubmatch(body); m != nil {
		slug := m[1]
		if slug == "" {
			slug = m[2] // PERMISSION_PAGES.ATTR form
		}
		return Posture{Kind: KindPage, Literal: slug, Detail: detail + ": CustomPagePermissionCheck(PERMISSION_PAGES[" + slug + "])"}
	}
	if m := actionPermRe.FindStringSubmatch(body); m != nil {
		return Posture{Kind: KindAction, Literal: m[1], Detail: detail + ": CustomActionPermissionCheck(" + m[1] + ")"}
	}
	// bare [IsAuthenticated] / TODO arm → authenticated-only (§10).
	if authenticatedRe.MatchString(body) {
		return Posture{Kind: KindAuthenticated, Detail: detail + ": IsAuthenticated"}
	}
	if allowAnyRe.MatchString(body) {
		return Posture{Kind: KindPublic, Detail: detail + ": AllowAny"}
	}
	return Posture{Kind: KindUnknown, Detail: detail + ": no decodable permission marker"}
}

// decodePermissionClasses decodes a flat, comma-joined permission-class list
// (the class-attribute path) into a Posture. Priority mirrors decodeArmBody.
func decodePermissionClasses(classList string) Posture {
	if classList == "" {
		// No explicit class list and no decodable body → DRF default is
		// AllowAny, but we report unknown rather than assert public, because an
		// absent class list here means "could not resolve", not "proven open".
		return Posture{Kind: KindUnknown, Detail: "no permission_classes resolved"}
	}
	if superuserRe.MatchString(classList) {
		return Posture{Kind: KindSuperuser, Detail: "permission_classes: superuser"}
	}
	if m := pagePermRe.FindStringSubmatch(classList); m != nil {
		slug := m[1]
		if slug == "" {
			slug = m[2]
		}
		return Posture{Kind: KindPage, Literal: slug, Detail: "permission_classes: page " + slug}
	}
	if m := actionPermRe.FindStringSubmatch(classList); m != nil {
		return Posture{Kind: KindAction, Literal: m[1], Detail: "permission_classes: action " + m[1]}
	}
	if allowAnyRe.MatchString(classList) {
		return Posture{Kind: KindPublic, Detail: "permission_classes: AllowAny"}
	}
	if authenticatedRe.MatchString(classList) {
		return Posture{Kind: KindAuthenticated, Detail: "permission_classes: IsAuthenticated"}
	}
	// A non-empty, non-AllowAny class list is at minimum authenticated.
	return Posture{Kind: KindAuthenticated, Detail: "permission_classes: " + classList + " (non-public)"}
}

// stringLiterals extracts the "..."/'...' literals from a fragment, in order.
func stringLiterals(s string) []string {
	re := regexp.MustCompile(`["']([^"']+)["']`)
	ms := re.FindAllStringSubmatch(s, -1)
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m[1])
	}
	return out
}
