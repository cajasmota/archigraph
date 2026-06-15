// http_endpoint_match.go — structural path normalization for matching
// http_endpoint_call synthetics to http_endpoint_definition synthetics
// (issue #1615).
//
// Background
// ----------
// The Phase-2 resolve pass (http_endpoint_resolve.go) links a call synthetic to
// a definition synthetic by an EXACT match on the canonical synthetic Name
// (`http:<VERB>:<path>`). On real polyglot codebases that exact match is far too
// strict: a frontend/mobile client calls `/inspections/{id}/create-deficiencies/`
// while the backend mounts the route at `/api/v1/inspections/{pk}/create-deficiencies`.
// The two Names differ on three independent axes —
//
//	(a) the configurable API mount prefix (`/api`, `/api/vN`, `/vN`),
//	(b) the path-parameter token name (`{id}` vs `{pk}` vs `{slug}` …),
//	(c) the trailing slash + letter case,
//
// — so the exact match misses and the call-site is counted as an orphan even
// though it clearly targets the backend route. On the upvate corpus this
// accounted for the bulk of intra-repo HTTP orphans (143 of 144 unresolved
// call synthetics in the backend repo become resolvable once these three axes
// are normalized away).
//
// Strategy
// --------
// normalizeEndpointPath collapses (b) and (c): every path-parameter placeholder
// — `{pk}`, `{id}`, `{userId}`, `:id`, `<int:id>`, … — becomes the uniform
// token `{*}`, the result is lower-cased and the trailing slash is stripped.
// stripEndpointAPIPrefix collapses (a): a leading `/api`, `/api/vN` or bare
// `/vN` segment is removed so a prefixed producer and an unprefixed consumer
// bucket together.
//
// This is intentionally CONSERVATIVE to avoid false matches:
//   - Only the well-known `api` / `vN` first segments are stripped — never an
//     arbitrary first segment, which would collapse genuinely-distinct routes.
//   - The verb must still be compatible (verbsMatchCompat): `ANY` matches any
//     specific verb (Django ViewSets emit ANY), but two DIFFERENT specific
//     verbs never match (DELETE/{*} must not resolve to PATCH/{*}).
//   - The exact-Name match in the resolve pass always runs FIRST; this
//     normalized match is a fallback used only when the exact match misses.
//
// Prefix-normalization extension (#2547)
// ----------------------------------------
// Frontend HTTP call extractors (axios, fetch, callApi wrappers) frequently
// emit raw paths such as `/searchBuildings` or `/groups/filter` without the
// `/api/v1/` mount prefix that Django+DRF backend endpoints use. The
// definitionByPath index registers backend routes under BOTH their full path
// AND a prefix-stripped alias (via endpointMatchKeys / stripEndpointAPIPrefix),
// but that only helps when the BACKEND is the prefixed side. When the FRONTEND
// omits the prefix entirely, the stripped alias is the same as the raw path and
// no key collision occurs with the prefixed backend route.
//
// resolveCallByPath therefore adds a second fallback tier (after the structural
// path-shape match): it tries prepending each of the well-known API prefix
// candidates [/api/v1, /api/v2, /api, /v1] to the call's normalized path and
// probes definitionByPath with those keys. The first match wins; the resolved
// edge is stamped with prefix_normalized=<candidate> so the match is traceable
// in the graph. This is resolution-time normalization — no changes are made to
// the extracted entity names or paths, keeping the raw extractor output intact
// for debugging.
//
// Resolution-time vs extraction-time: resolution-time was chosen because
// (a) it requires touching only one file (the linker, not every extractor),
// (b) it leaves the extracted paths intact for human inspection,
// (c) the prefix is inherently a cross-repo / context-dependent property —
//
//	the same frontend call `/users` could target `/api/v1/users` in one
//	group and `/v2/users` in another; the resolution pass sees both sides
//	simultaneously and can pick the best match, whereas an extractor working
//	on the frontend alone cannot know the backend's URL scheme.
package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/types"
)

// endpointPathParamRe matches a path-parameter placeholder in any of the three
// notations the various extractors emit:
//   - {pk}, {id}, {userId}            (OpenAPI / Django / Express curly form)
//   - :id, :pk, :userId              (Express / Rails colon form)
//   - <int:id>, <slug>, <uuid:pk>    (Django / Flask angle form)
//
// Each is replaced with the canonical token {*} so matching is on path
// STRUCTURE, not parameter name. Mirrors internal/links/http_pass.go.
var endpointPathParamRe = regexp.MustCompile(`\{[^}]+\}|:[a-zA-Z][a-zA-Z0-9_]*|<[^>]+>`)

// prefixCandidates is the ordered list of well-known API mount prefixes tried
// when a call path has no prefix of its own and the structural match misses
// (#2547). Order matters: more specific (longer) prefixes are tried first so
// `/api/v1` is preferred over `/api` when both would match — avoids landing on
// a coarser route in multi-version APIs.
var prefixCandidates = []string{"/api/v1", "/api/v2", "/api", "/v1"}

// endpointAPIPrefixRe matches a leading API/version mount prefix. Only the
// FIRST segment group is matched and only the well-known `api` / `vN` forms —
// an arbitrary first segment is never stripped. Go's RE2 has no lookahead so
// group 1 captures the boundary `/` (kept out of the strip) or end-of-string.
// Mirrors internal/links/http_pass.go.
var endpointAPIPrefixRe = regexp.MustCompile(`^/(?:api(?:/v\d+)?|v\d+)(/|$)`)

// normalizeEndpointPath canonicalizes every path-parameter placeholder to the
// uniform token {*}, lower-cases the result and strips a trailing slash so that
// route shapes from different extractors compare without caring about parameter
// name, case or trailing-slash convention.
//
//	/users/{pk}                    → /users/{*}
//	/Users/{userId}/               → /users/{*}
//	/users/<int:id>                → /users/{*}
//	/users/{userId}/posts/{postId} → /users/{*}/posts/{*}
func normalizeEndpointPath(path string) string {
	path = endpointPathParamRe.ReplaceAllString(path, "{*}")
	path = strings.ToLower(path)
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
		if path == "" {
			path = "/"
		}
	}
	return path
}

// stripEndpointAPIPrefix removes a leading `/api`, `/api/vN`, or `/vN` segment
// from an already-normalized path. Returns ("", false) when no such prefix is
// present. The returned path always keeps a leading slash.
func stripEndpointAPIPrefix(normalizedPath string) (string, bool) {
	m := endpointAPIPrefixRe.FindStringSubmatchIndex(normalizedPath)
	if m == nil {
		return "", false
	}
	stripped := normalizedPath[m[2]:]
	if stripped == "" {
		stripped = "/"
	}
	return stripped, stripped != normalizedPath
}

// endpointMatchKeys returns the set of normalized keys under which a synthetic
// with the given canonical path should be registered / probed: the normalized
// path, plus its API/version-prefix-stripped form when one is present. Keys are
// deduplicated.
func endpointMatchKeys(path string) []string {
	if path == "" {
		return nil
	}
	nk := normalizeEndpointPath(path)
	keys := []string{nk}
	if stripped, ok := stripEndpointAPIPrefix(nk); ok && stripped != nk {
		keys = append(keys, stripped)
	}
	return keys
}

// resolveCallByPath is the structural fallback used by the Phase-2 resolve pass
// when the exact-Name match for an http_endpoint_call synthetic misses. It
// looks the call's normalized path keys up in definitionByPath and returns the
// first definition (in merged order, for determinism) whose verb is compatible.
//
// Two matching tiers are attempted in order:
//
//  1. Structural normalization (original #1615 behavior): normalize path-param
//     tokens to {*}, lower-case, strip trailing slash, and also try the
//     prefix-stripped form — so `/api/v1/users/{pk}` and `/users/{id}/` both
//     normalize to `/users/{*}`.
//
//  2. Prefix-injection (#2547): when tier 1 misses AND the call path carries no
//     API/version prefix of its own, retry by prepending each prefix candidate
//     from prefixCandidates to the normalized call path and probing
//     definitionByPath. This handles the upvate pattern where the frontend
//     extractor emits `/searchBuildings` but the backend mounts the route at
//     `/api/v1/searchBuildings`.
//
// Returns (index, prefixUsed, true) on a match, (0, "", false) otherwise.
// prefixUsed is non-empty only when tier 2 matched, and carries the raw prefix
// string (e.g. "/api/v1") so the caller can stamp prefix_normalized on the
// emitted edge for traceability.
//
// Determinism: definitionByPath slices are appended in `merged` iteration
// order, which is canonical by contract of ResolveHTTPEndpointHandlers, so the
// first compatible candidate is stable across runs.
func resolveCallByPath(call *types.EntityRecord, merged []types.EntityRecord, definitionByPath map[string][]int) (int, string, bool) {
	callVerb := propOr(call, "verb", "")
	callPath := propOr(call, "path", "")

	// Tier 1: structural normalization (path-param tokens + prefix stripping).
	for _, key := range endpointMatchKeys(callPath) {
		for _, idx := range definitionByPath[key] {
			if verbsMatchCompat(callVerb, propOr(&merged[idx], "verb", "")) {
				return idx, "", true
			}
		}
	}

	// Tier 2: prefix-injection (#2547).
	// Only attempt when the call path has no API/version prefix itself —
	// if it already has one and tier 1 missed, adding another prefix would
	// produce a nonsensical double-prefixed path.
	normCallPath := normalizeEndpointPath(callPath)
	if _, hasPrefix := stripEndpointAPIPrefix(normCallPath); !hasPrefix {
		for _, pfx := range prefixCandidates {
			prefixed := pfx + normCallPath
			// The definition index uses the same normalizeEndpointPath
			// convention, so probe with the already-normalized prefixed path.
			for _, idx := range definitionByPath[prefixed] {
				if verbsMatchCompat(callVerb, propOr(&merged[idx], "verb", "")) {
					// Return the prefix without leading slash for the
					// prefix_normalized property value (e.g. "api/v1").
					return idx, strings.TrimPrefix(pfx, "/"), true
				}
			}
		}
	}

	return 0, "", false
}

// verbsMatchCompat reports whether a call verb and a definition verb may refer
// to the same logical endpoint. ANY (case-insensitive) on either side matches
// everything — Django ViewSets wire HTTP methods at the View level and emit
// ANY-verb route synthetics. An empty verb is treated as ANY. Two different
// specific verbs never match.
func verbsMatchCompat(callVerb, defVerb string) bool {
	c := strings.ToUpper(strings.TrimSpace(callVerb))
	d := strings.ToUpper(strings.TrimSpace(defVerb))
	if c == "" || c == "ANY" || d == "" || d == "ANY" {
		return true
	}
	return c == d
}
