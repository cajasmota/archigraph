// Shared cross-group HTTP endpoint join normalizer (#4550).
//
// The rewrite-parity tools (stub_detector, auth_posture_diff, and any future
// oracle↔v3 endpoint-pairing tool) must join an oracle endpoint to its v3
// counterpart on the SAME structural identity, or the join silently returns
// nothing. Before this file each tool carried its OWN path normalizer and they
// diverged: stub_detector stripped the /api[/vN] prefix and canonicalised
// path-params to {*}, but auth_posture_diff did NOT strip the api-prefix and
// additionally folded a DRF #action suffix into the key — so on the real
// acme (DRF /api/v1/...) ↔ acme-v3 (NestJS /v1/...) groups its join matched
// ZERO endpoints while stub_detector matched 420/446 on the same data.
//
// This is now the ONE normalizer both tools call. It mirrors the cross-repo
// HTTP link pass canonicalisation (internal/links/http_pass.go
// normalizePathForIndex + stripAPIPrefix) so MCP join keys bucket identically
// to how the link resolver buckets route shapes:
//
//   - path-parameter segments ({id}, :id, <int:pk>, {pk}) → {*}
//   - lower-cased
//   - trailing slash folded
//   - a leading /api[/vN] or /vN prefix stripped (so /api/v1/orders/{*} buckets
//     with /v1/orders/{*} buckets with /orders/{*})
//
// Crucially it does NOT fold a framework-specific action suffix into the key:
// the oracle (DRF) stamps an `action` while the v3 (NestJS) does not, so a
// #action key never matched and was the proximate cause of the 0-join bug.
package mcp

import (
	"regexp"
	"strings"
)

// endpointPathParamRe / endpointAPIPrefixRe mirror the links-pass
// canonicalisation. Kept here (not in the links package) to avoid widening that
// package's API; the regexes are the literal source of truth for both the path
// normalizer below and the join keys built on top of it.
var (
	endpointPathParamRe = regexp.MustCompile(`\{[^}]+\}|:[a-zA-Z][a-zA-Z0-9_]*|<[^>]+>`)
	endpointAPIPrefixRe = regexp.MustCompile(`^/(?:api(?:/v\d+)?|v\d+)(/|$)`)
)

// endpointJoinKey is the normalized (method, path) identity used to match an
// oracle endpoint to its v3 counterpart across groups. This is the single key
// type both stub_detector and auth_posture_diff join on.
type endpointJoinKey struct {
	method string
	path   string
}

// String renders the join key as "VERB /path" for display / map-string use.
func (k endpointJoinKey) String() string {
	return endpointLabel(k.method, k.path)
}

// normalizeEndpointJoinPath canonicalises an endpoint path for cross-group
// joining: path-params → {*}, lower-cased, trailing slash folded, and a leading
// /api[/vN] or /vN prefix stripped. This is the ONE path normalizer the
// rewrite-parity endpoint join uses.
func normalizeEndpointJoinPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = endpointPathParamRe.ReplaceAllString(path, "{*}")
	path = strings.ToLower(path)
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
		if path == "" {
			path = "/"
		}
	}
	// Strip a leading /api[/vN] or /vN prefix so /api/v1/orders/{*} buckets with
	// /orders/{*}.
	if m := endpointAPIPrefixRe.FindStringSubmatchIndex(path); m != nil {
		stripped := path[m[2]:]
		if stripped == "" {
			stripped = "/"
		}
		if !strings.HasPrefix(stripped, "/") {
			stripped = "/" + stripped
		}
		path = stripped
	}
	return path
}

// newEndpointJoinKey builds a normalized join key from a raw verb + path.
func newEndpointJoinKey(verb, path string) endpointJoinKey {
	return endpointJoinKey{
		method: strings.ToUpper(strings.TrimSpace(verb)),
		path:   normalizeEndpointJoinPath(path),
	}
}

// parseEndpointFilter parses a "GET /api/orders/{id}" filter into a normalized
// join key. A bare path with no leading verb leaves method empty (matches any
// method on that path).
func parseEndpointFilter(raw string) endpointJoinKey {
	raw = strings.TrimSpace(raw)
	method := ""
	path := raw
	if fields := strings.Fields(raw); len(fields) == 2 {
		method = strings.ToUpper(fields[0])
		path = fields[1]
	}
	return newEndpointJoinKey(method, path)
}

// endpointLabel renders a human "VERB /path" label, falling back gracefully
// when the verb or path is missing.
func endpointLabel(method, path string) string {
	method = strings.TrimSpace(method)
	path = strings.TrimSpace(path)
	switch {
	case method != "" && path != "":
		return method + " " + path
	case path != "":
		return path
	case method != "":
		return method
	default:
		return "(unknown endpoint)"
	}
}
