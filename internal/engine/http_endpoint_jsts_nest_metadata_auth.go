// NestJS metadata-decorator auth recognition (deploy-9).
//
// Many production NestJS apps do NOT gate routes with `@UseGuards(...)` on each
// controller/method. Instead they register a single authentication guard and a
// single RBAC `PermissionsGuard` GLOBALLY (`{ provide: APP_GUARD, ... }`) and
// declare each route's requirement with a thin `SetMetadata`-based decorator —
// e.g. `@RequirePage('devices.read')`, `@Authenticated()`, `@AnyPage(...)` — that
// the global guard reads via `Reflector.getAllAndOverride([handler, class])`.
// `@Public()` is the explicit opt-out (legacy AllowAny).
//
// The pre-existing #2852 resolver only recognised `@UseGuards`, so on such an
// app EVERY endpoint resolved to method="unknown" and grafel_auth_coverage
// reported `covered: 0` for genuinely-protected routes (the deploy-9 finding on
// core-backend-v2 / upvate-v2: 0 / 305).
//
// This pass recognises the metadata-decorator family at BOTH method level (binds
// to that handler) and class level (the controller-level default applies to all
// its methods, mirroring `getAllAndOverride([handler, class])` precedence:
// method overrides class). A protective decorator → auth_required=true with
// method="guard" (the route IS gated by the metadata-driven guard) and the page
// /permission carried as auth_permissions + auth_page. `@Public()`/`@AllowAnonymous`
// → an explicit public verdict that suppresses inheritance, so the tool does NOT
// mislabel a genuinely-public login route as protected.
//
// Honest-partial: a route carrying NO recognised metadata decorator at method or
// class level is left to the existing resolver chain (it may still be gated by a
// truly global APP_GUARD declared in another file — cross-file, not visible to
// this per-file pass). In core-backend-v2 every route carries a per-route or
// per-class decorator (154 verb decorators ↔ 153 auth decorators), so this pass
// greens the real numbers without fabricating coverage.
//
// Refs deploy-9.
package engine

import (
	"regexp"
	"strings"
)

// nestMetaAuthKind classifies a recognised metadata decorator.
type nestMetaAuthKind int

const (
	nestMetaNone      nestMetaAuthKind = iota
	nestMetaPublic                     // @Public() / @AllowAnonymous() — explicit opt-out
	nestMetaProtected                  // any protective requirement decorator
)

// nestMetaAuthResult is the resolved metadata-decorator posture for a method or
// class, with the page/permission slugs the decorator names (when literal).
type nestMetaAuthResult struct {
	kind        nestMetaAuthKind
	decorator   string   // canonical decorator text for the source chain (e.g. "@RequirePage('buildings')")
	permissions []string // literal page/permission slugs (RequirePage/AnyPage/HasPage/OwnerAndPage)
	roles       []string // literal role names (@Roles)
}

// nestPublicDecoratorRe matches the explicit public opt-out decorators. These
// are the metadata equivalents of DRF AllowAny — the route skips authentication
// entirely, so it must NOT inherit a class-level or global requirement.
var nestPublicDecoratorRe = regexp.MustCompile(`@(?:Public|AllowAnonymous|AllowAny|SkipAuth|NoAuth)\s*\(`)

// nestProtectiveMetaDecoratorRe matches the metadata-driven requirement
// decorators recognised across the common RBAC conventions. The set covers the
// core-backend-v2 family (@RequirePage / @Authenticated / @AnyPage / @OwnerOnly /
// @OwnerAndPage / @HasPage / @RequireAction / @InternalKeyOrAuth) plus the
// generic nest-access-control / casl spellings (@Permissions / @RequirePermission(s)
// / @CheckPolicies / @UseRoles). Group 1 = decorator name, group 2 = raw args.
var nestProtectiveMetaDecoratorRe = regexp.MustCompile(
	`@(RequireAnyPage|RequirePage|AuthenticatedOrInternalKey|Authenticated|RequireSuperuser|AnyPage|OwnerOnly|OwnerAndPage|HasPage|HasPermission|RequireAction|InternalKeyOrAuth|RequirePermissions|RequirePermission|Permissions|CheckPermissions|CheckPolicies|UseRoles|RequireScopes|RequireScope|Scopes)\s*\(([^)]*)\)`,
)

// nestPageBearingDecorators are the protective decorators whose quoted argument
// names a permission page / slug we surface as auth_permissions + auth_page.
var nestPageBearingDecorators = map[string]bool{
	"RequirePage":        true,
	"RequireAnyPage":     true,
	"AnyPage":            true,
	"OwnerAndPage":       true,
	"HasPage":            true,
	"HasPermission":      true,
	"RequirePermissions": true,
	"RequirePermission":  true,
	"Permissions":        true,
	"CheckPermissions":   true,
	"RequireScopes":      true,
	"RequireScope":       true,
	"Scopes":             true,
}

// recognizeNestMetadataAuth classifies a decorator block (the contiguous run of
// `@...` lines above a method or class). `@Public` wins outright (explicit
// opt-out). Otherwise the first protective metadata decorator decides, carrying
// any literal page/permission slugs. A block with neither yields nestMetaNone so
// the caller falls through to the existing resolver chain (honest-partial).
func recognizeNestMetadataAuth(block string) nestMetaAuthResult {
	if block == "" {
		return nestMetaAuthResult{kind: nestMetaNone}
	}
	// Explicit public opt-out takes precedence over any sibling requirement —
	// stacking @Public with a page decorator is a contradiction; the public
	// marker is the safe (auth-skipping) reading.
	if nestPublicDecoratorRe.MatchString(block) {
		return nestMetaAuthResult{kind: nestMetaPublic, decorator: "@Public()"}
	}
	res := nestMetaAuthResult{kind: nestMetaNone}
	for _, m := range nestProtectiveMetaDecoratorRe.FindAllStringSubmatch(block, -1) {
		name := m[1]
		args := strings.TrimSpace(m[2])
		res.kind = nestMetaProtected
		if res.decorator == "" {
			res.decorator = "@" + name + "(" + args + ")"
		}
		if nestPageBearingDecorators[name] {
			for _, q := range jsQuotedTokenRe.FindAllStringSubmatch(args, -1) {
				if tok := strings.TrimSpace(q[1]); tok != "" {
					res.permissions = append(res.permissions, tok)
				}
			}
		}
	}
	// @Roles can co-occur with a metadata requirement (or stand alone as the
	// requirement). Capture its literal roles.
	if rm := jsRolesDecoratorRe.FindStringSubmatch(block); rm != nil {
		for _, q := range jsQuotedTokenRe.FindAllStringSubmatch(rm[1], -1) {
			if tok := strings.TrimSpace(q[1]); tok != "" {
				res.roles = append(res.roles, tok)
			}
		}
		if res.kind == nestMetaNone {
			res.kind = nestMetaProtected
			res.decorator = "@Roles(" + strings.TrimSpace(rm[1]) + ")"
		}
	}
	return res
}

// indexNestMethodMetadataAuth attributes each handler method's own preceding
// decorator block to that method, recognising the metadata-decorator family
// (and @Public). Returns a map keyed by bare method name. Mirrors
// indexNestMethodGuards but for the SetMetadata RBAC decorators rather than
// @UseGuards. A method whose block carries a verb decorator but no recognised
// auth metadata yields no entry (it may inherit the class-level decorator).
func indexNestMethodMetadataAuth(content string) map[string]nestMetaAuthResult {
	out := map[string]nestMetaAuthResult{}
	if !nestHasMetadataAuthMarker(content) {
		return out
	}
	for _, loc := range nestHandlerMethodRe.FindAllStringSubmatchIndex(content, -1) {
		method := content[loc[2]:loc[3]]
		block := precedingDecoratorBlock(content, loc[0])
		if block == "" || !nestVerbDecoratorRe.MatchString(block) {
			continue
		}
		if res := recognizeNestMetadataAuth(block); res.kind != nestMetaNone {
			out[method] = res
		}
	}
	return out
}

// resolveNestClassMetadataAuth scans the controller class declarations and
// returns the class-level metadata posture (the controller-level default that
// applies to every method without its own override). When a file declares
// multiple controllers we keep the FIRST protective/public verdict found —
// per-method overrides still win at resolve time, so a coarse file-level class
// verdict is a safe medium-confidence fallback. Returns nestMetaNone when no
// controller carries a class-level metadata decorator.
func resolveNestClassMetadataAuth(content string) nestMetaAuthResult {
	if !nestHasMetadataAuthMarker(content) {
		return nestMetaAuthResult{kind: nestMetaNone}
	}
	for _, m := range jsClassDeclRe.FindAllStringSubmatchIndex(content, -1) {
		block := precedingDecoratorBlock(content, m[0])
		if block == "" {
			continue
		}
		if res := recognizeNestMetadataAuth(block); res.kind != nestMetaNone {
			return res
		}
	}
	return nestMetaAuthResult{kind: nestMetaNone}
}

// nestFirstControllerLine returns the 1-based line of the first @Controller
// declaration (for the class-level source-chain entry). 0 when absent.
func nestFirstControllerLine(content string) int {
	if i := strings.Index(content, "@Controller"); i >= 0 {
		return lineAtOffset(content, i)
	}
	return 0
}

// nestHasMetadataAuthMarker is the cheap substring fast-path: skip the regex
// passes unless the file mentions at least one recognised metadata decorator.
func nestHasMetadataAuthMarker(content string) bool {
	for _, marker := range []string{
		"@RequirePage", "@RequireAnyPage", "@Authenticated", "@AuthenticatedOrInternalKey",
		"@RequireSuperuser", "@AnyPage", "@OwnerOnly", "@OwnerAndPage",
		"@HasPage", "@HasPermission", "@RequireAction", "@InternalKeyOrAuth",
		"@RequirePermission", "@Permissions", "@CheckPermissions", "@CheckPolicies",
		"@UseRoles", "@RequireScope", "@Scopes", "@Roles",
		"@Public", "@AllowAnonymous", "@AllowAny", "@SkipAuth", "@NoAuth",
	} {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

// nestMetaPolicy builds an AuthPolicy from a resolved metadata posture at the
// given confidence. A protective verdict carries method="guard" (the route is
// gated by the metadata-driven global guard) with the page/permission slugs as
// permissions; a public verdict is an explicit Required=false config verdict.
func nestMetaPolicy(res nestMetaAuthResult, confidence, file string, line int) AuthPolicy {
	if res.kind == nestMetaPublic {
		return AuthPolicy{
			Required: false, Method: "config", Confidence: confidence,
			SourceChain: []AuthSignal{{
				Kind: "config", Text: res.decorator + " (explicit public)", File: file, Line: line,
			}},
		}
	}
	return AuthPolicy{
		Required: true, Method: "guard", Confidence: confidence,
		Roles:       dedupeNonEmpty(res.roles),
		Permissions: dedupeNonEmpty(res.permissions),
		SourceChain: []AuthSignal{{
			Kind: "guard", Text: res.decorator, File: file, Line: line,
		}},
	}
}

// dedupeNonEmpty returns a stable de-duplicated copy of in with empties removed,
// or nil when nothing remains (so an empty slice never sets an empty property).
func dedupeNonEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
