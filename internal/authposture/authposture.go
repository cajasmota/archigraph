// Package authposture is the framework-agnostic CORE of auth_posture_diff
// (ticket #4422, epic #4419 P0 — the BLOCKING RBAC-drift class).
//
// The problem: a behavioral ORACLE (e.g. Django) and a greenfield V3 rewrite
// (e.g. NestJS) each express endpoint authorization in their own dialect. The
// rewrite's own tests assert the shape it wrote, NOT equivalence to the oracle,
// so an RBAC drift (a page-grant silently downgraded to authenticated-only, or
// a slug typo "core_admin" → "core-admin") passes every gate the consumer runs.
// The only place the oracle↔v3 comparison can live is the co-resident graph.
//
// This package implements that comparison as TWO decoupled halves:
//
//  1. A PLUGGABLE auth-posture RESOLVER registry (resolvers.go). Each resolver
//     maps ONE framework's auth signal (Django get_permissions, NestJS guards,
//     Spring @PreAuthorize, …) into the SHARED {Kind, Literal} vocabulary below.
//     This is the all-framework mandate (epic #4419): the diff is NOT a
//     hardcoded Django↔NestJS pair — any framework that has a resolver slots in.
//
//  2. A framework-AGNOSTIC Diff (this file) that compares two already-resolved
//     Postures and emits a conservative verdict. Once both sides resolve into
//     the common vocabulary, the diff knows nothing about either framework.
//
// Correctness is load-bearing: the §10 Django get_permissions decode contract
// (django.go) MUST be encoded exactly — treating the `else` arm as
// authenticated-only, or a `== [list]` scalar-vs-list comparison as live code,
// mis-decodes the oracle and produces a FALSE `equivalent` verdict that hides a
// real RBAC regression. Every decode/verdict path is unit-tested.
package authposture

import (
	"github.com/cajasmota/grafel/internal/literalparity"
)

// Kind is the shared auth-posture vocabulary. Every framework resolver
// normalises its native signal into exactly one of these kinds, so the diff
// core can compare a Django posture against a NestJS posture (or any pair)
// without knowing either dialect.
//
// Ordering matters for the stricter/looser verdict: STRENGTH below is the
// monotone "how much access this posture DEMANDS" lattice. A higher strength
// means a tighter gate.
type Kind string

const (
	// KindPublic is no authorization required (DRF AllowAny, Nest @Public).
	KindPublic Kind = "public"
	// KindAuthenticated requires only a logged-in principal (IsAuthenticated,
	// @Authenticated) with no further page/action/role check.
	KindAuthenticated Kind = "authenticated"
	// KindScope requires a specific OAuth/token scope.
	KindScope Kind = "scope"
	// KindRole requires a named role.
	KindRole Kind = "role"
	// KindPage requires a specific PAGE permission (the Django
	// CustomPagePermissionCheck(PERMISSION_PAGES[X]) grant; Nest @RequirePage).
	KindPage Kind = "page"
	// KindAction requires a specific ACTION permission (the Django
	// get_permissions `else` default arm CustomActionPermissionCheck; Nest
	// @RequireAction).
	KindAction Kind = "action"
	// KindSuperuser requires superuser/staff (the tightest gate).
	KindSuperuser Kind = "superuser"
	// KindUnknown means the resolver could not classify the signal. A diff
	// against an unknown side is always reported as a non-equivalent
	// "kind_mismatch" so an unresolved posture never masquerades as equivalent.
	KindUnknown Kind = "unknown"
)

// strength is the access-demand lattice used for stricter/looser. Higher =
// tighter gate. Two kinds at the SAME strength but different identity (e.g.
// page vs action — both per-permission) are NOT comparable on the lattice and
// fall to kind_mismatch instead of stricter/looser.
var strength = map[Kind]int{
	KindUnknown:       -1,
	KindPublic:        0,
	KindAuthenticated: 1,
	KindScope:         2,
	KindRole:          2,
	KindPage:          3,
	KindAction:        3,
	KindSuperuser:     4,
}

// litComparable reports whether a Kind carries a meaningful literal (a slug,
// role name, or scope) that must match for equivalence. public / authenticated
// / superuser are nullary — they have no literal to compare.
func litComparable(k Kind) bool {
	switch k {
	case KindPage, KindAction, KindRole, KindScope:
		return true
	default:
		return false
	}
}

// Posture is one resolved auth requirement: a Kind, an optional Literal (the
// page slug / action codename / role / scope when the Kind carries one), and
// human Detail describing how the resolver decoded it. Framework is the
// resolver that produced it (for provenance in the diff output).
type Posture struct {
	Kind      Kind   `json:"kind"`
	Literal   string `json:"literal,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Framework string `json:"framework,omitempty"`
}

// Verdict is the conservative comparison outcome between a v3 posture and the
// oracle posture it is meant to reproduce.
type Verdict string

const (
	// VerdictEquivalent: same Kind AND (for literal-bearing kinds) same Literal
	// after slug normalisation. The rewrite reproduces the oracle's grant.
	VerdictEquivalent Verdict = "equivalent"
	// VerdictStricter: v3 demands MORE access than the oracle (e.g. oracle
	// authenticated-only, v3 requires superuser). Safer than the oracle, but
	// still a behavioral divergence worth surfacing.
	VerdictStricter Verdict = "stricter"
	// VerdictLooser: v3 demands LESS access than the oracle (e.g. oracle
	// page-grant, v3 authenticated-only) — the dangerous RBAC regression.
	VerdictLooser Verdict = "looser"
	// VerdictSlugMismatch: SAME kind, DIFFERENT literal after normalisation
	// (e.g. oracle page "core_admin" vs v3 page "core-admin"). The grant TYPE
	// matches but the specific permission identifier diverged.
	VerdictSlugMismatch Verdict = "slug_mismatch"
	// VerdictKindMismatch: different, non-lattice-comparable kinds (e.g.
	// page-vs-action, or anything vs unknown). The grant TYPE itself diverged.
	VerdictKindMismatch Verdict = "kind_mismatch"
)

// DiffResult is the full per-endpoint diff record.
type DiffResult struct {
	Verdict       Verdict `json:"verdict"`
	Detail        string  `json:"detail"`
	OraclePosture Posture `json:"oracle_resolved"`
	V3Posture     Posture `json:"v3_stack"`
}

// Diff compares a v3 posture against the oracle posture it is meant to
// reproduce and returns a conservative verdict. The comparison is symmetric in
// inputs but the verdict is ASYMMETRIC by design: stricter/looser are framed
// from v3's perspective relative to the oracle (v3 stricter = v3 demands more).
//
// Decision order (first match wins):
//
//  1. Either side unknown            → kind_mismatch (never false-equivalent).
//  2. Same kind:
//     a. literal-bearing kind & literals differ (after NormalizeKey)
//     → slug_mismatch.
//     b. otherwise                   → equivalent.
//  3. Different kinds:
//     a. both on the strength lattice & strengths differ
//     → stricter (v3>oracle) / looser (v3<oracle).
//     b. same strength but different identity (page vs action), or one side
//     off-lattice                 → kind_mismatch.
func Diff(v3, oracle Posture) DiffResult {
	res := DiffResult{OraclePosture: oracle, V3Posture: v3}

	// (1) Unknown on either side is never equivalent — a resolver that could
	// not classify must NOT be silently treated as a match.
	if v3.Kind == KindUnknown || oracle.Kind == KindUnknown {
		res.Verdict = VerdictKindMismatch
		res.Detail = "unresolved posture: v3=" + string(v3.Kind) + " oracle=" + string(oracle.Kind)
		return res
	}

	// (2) Same kind.
	if v3.Kind == oracle.Kind {
		if litComparable(v3.Kind) && !literalsEqual(v3.Literal, oracle.Literal) {
			res.Verdict = VerdictSlugMismatch
			res.Detail = string(v3.Kind) + " grant on both sides but literal differs: oracle=" +
				q(oracle.Literal) + " v3=" + q(v3.Literal)
			return res
		}
		res.Verdict = VerdictEquivalent
		if litComparable(v3.Kind) {
			res.Detail = "both " + string(v3.Kind) + " grant on " + q(oracle.Literal)
		} else {
			res.Detail = "both " + string(v3.Kind)
		}
		return res
	}

	// (3) Different kinds — try the strength lattice.
	sv, okv := strength[v3.Kind]
	so, oko := strength[oracle.Kind]
	if okv && oko && sv >= 0 && so >= 0 && sv != so {
		if sv > so {
			res.Verdict = VerdictStricter
			res.Detail = "v3 " + string(v3.Kind) + " demands more than oracle " + string(oracle.Kind)
		} else {
			res.Verdict = VerdictLooser
			res.Detail = "v3 " + string(v3.Kind) + " demands LESS than oracle " + string(oracle.Kind) +
				" — RBAC regression"
		}
		return res
	}

	// Same strength, different identity (page vs action), or off-lattice.
	res.Verdict = VerdictKindMismatch
	res.Detail = "grant kind diverged: oracle=" + string(oracle.Kind) + " v3=" + string(v3.Kind)
	return res
}

// literalsEqual compares two posture literals using literalparity.NormalizeKey
// (the shared slug-normalisation reused from #4421): case-fold + separator-fold,
// so "core_admin" and "core-admin" and "Core Admin" all align. Empty literals
// compare equal only to each other.
func literalsEqual(a, b string) bool {
	return literalparity.NormalizeKey(a) == literalparity.NormalizeKey(b)
}

// q wraps a literal in quotes for human detail, rendering empty as "<none>".
func q(s string) string {
	if s == "" {
		return "<none>"
	}
	return "\"" + s + "\""
}
