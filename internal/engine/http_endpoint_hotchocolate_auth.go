// http_endpoint_hotchocolate_auth.go — HotChocolate (C#) GraphQL resolver
// authorization + request/response payload-shape attribution (#3961, epic #3872).
//
// synthesizeHotChocolate (http_endpoint_hotchocolate.go) emits one
// `http:GRAPHQL:/graphql/<Root>/<field>` synthetic per public resolver method
// with `source_handler=SCOPE.Operation:<Class>.<Method>`. That pass deliberately
// records only the endpoint identity + handler binding. This pass is the
// second, property-stamping half:
//
//   - AUTH: HotChocolate resolvers use the SAME `[Authorize]` /
//     `[Authorize(Roles="x")]` / `[Authorize(Policy="x")]` / `[AllowAnonymous]`
//     attributes ASP.NET Core uses (internal/custom/csharp/auth.go matches the
//     identical attribute set). A resolver method (or its enclosing resolver
//     class) carrying `[Authorize]` is auth-protected; the matched policy is
//     stamped onto its endpoint via the shared stampAuthPolicy contract so the
//     MCP grafel_auth_coverage tool's signal-1 property check fires
//     (auth_decorator) AND auth_required/auth_roles/auth_policy carry the
//     fine-grained verdict. `[AllowAnonymous]` proves the resolver is public.
//
//   - SHAPES: each resolver's typed C# argument list contributes a producer
//     REQUEST shape and its typed return contributes a producer RESPONSE shape,
//     surfaced through the existing C# payload-shape sniffer
//     (internal/substrate/payload_shapes_csharp.go) — see sniffHotChocolate*
//     there. This file owns AUTH only; shapes live in the substrate sniffer so
//     they flow through the standard payload-drift join.
//
// The pass mutates Properties in place and never adds or removes entities, so it
// cannot regress the synthesis pass. It is gated on the same HotChocolate
// file-signal as the synthesizer (no endpoints → no work).
package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/types"
)

// The hcAuthorize* regexes match a HotChocolate/ASP.NET `[Authorize]` attribute
// in its bare, role, policy, or positional-policy forms — the exact set the C#
// auth extractor (internal/custom/csharp/auth.go) recognises. Used to classify
// an attribute block sitting above a resolver method or class.
//
//	[Authorize]                         → protected, no roles/policy
//	[Authorize(Roles="Admin,Owner")]    → protected, roles
//	[Authorize(Policy="CanRead")]       → protected, policy
//	[Authorize("CanRead")]              → protected, positional policy
//	[AllowAnonymous]                    → explicit public (overrides Authorize)
var (
	hcAuthorizeRolesRe = regexp.MustCompile(
		`\[\s*Authorize\s*\(\s*Roles\s*=\s*"([^"]+)"`)
	hcAuthorizePolicyRe = regexp.MustCompile(
		`\[\s*Authorize\s*\(\s*Policy\s*=\s*"([^"]+)"`)
	hcAuthorizePositionalRe = regexp.MustCompile(
		`\[\s*Authorize\s*\(\s*"([^"]+)"\s*\)`)
	hcAuthorizeAnyRe   = regexp.MustCompile(`\[\s*Authorize\b`)
	hcAllowAnonymousRe = regexp.MustCompile(`\[\s*AllowAnonymous\s*\]`)
)

// hcAuthVerdict is the resolved authorization posture for one resolver,
// combining the method-level attribute (highest precedence) with the enclosing
// class-level attribute (fallback).
type hcAuthVerdict struct {
	decided  bool     // an explicit [Authorize]/[AllowAnonymous] was found
	required bool     // true → [Authorize]; false → [AllowAnonymous]
	roles    []string // from [Authorize(Roles="a,b")]
	policy   string   // from [Authorize(Policy="x")] or [Authorize("x")]
}

// classifyHCAuthBlock classifies an attribute block (the text immediately
// preceding a `class` or method declaration) into a verdict. `[AllowAnonymous]`
// wins over `[Authorize]` when both appear (explicit opt-out).
func classifyHCAuthBlock(block string) hcAuthVerdict {
	if block == "" {
		return hcAuthVerdict{}
	}
	if hcAllowAnonymousRe.MatchString(block) {
		return hcAuthVerdict{decided: true, required: false}
	}
	if !hcAuthorizeAnyRe.MatchString(block) {
		return hcAuthVerdict{}
	}
	v := hcAuthVerdict{decided: true, required: true}
	if m := hcAuthorizeRolesRe.FindStringSubmatch(block); m != nil {
		for _, r := range strings.Split(m[1], ",") {
			if r = strings.TrimSpace(r); r != "" {
				v.roles = append(v.roles, r)
			}
		}
	}
	if m := hcAuthorizePolicyRe.FindStringSubmatch(block); m != nil {
		v.policy = m[1]
	} else if m := hcAuthorizePositionalRe.FindStringSubmatch(block); m != nil {
		v.policy = m[1]
	}
	return v
}

// hcMethodAuth maps a `<Class>.<Method>` key (matching the synthesizer's
// source_handler operation) to its resolved auth verdict.
func buildHCMethodAuth(content string) map[string]hcAuthVerdict {
	out := map[string]hcAuthVerdict{}
	// Walk each resolver class (any class with a body); within it, record the
	// class-level verdict and each public method's method-level verdict.
	for _, cm := range hcPlainClassRe.FindAllStringSubmatchIndex(content, -1) {
		className := content[cm[2]:cm[3]]
		classBlock := hcAttrBlockBefore(content, cm[0])
		classVerdict := classifyHCAuthBlock(classBlock)
		body := hcClassBody(content, cm[1])
		if body == "" {
			continue
		}
		// Locate each public resolver method inside the class body and read the
		// attribute block immediately preceding it.
		for _, mm := range hcResolverMethodRe.FindAllStringSubmatchIndex(body, -1) {
			method := body[mm[2]:mm[3]]
			if method == className {
				continue // constructor
			}
			// hcResolverMethodRe consumes the method's own leading `[...]`
			// attribute block (the `(?:\[...\]\s*)*` prefix), so the method-level
			// attributes live INSIDE the match (body[mm[0]:nameStart]). Read them
			// from there; fall back to a backward scan for any attributes the
			// bounded prefix did not reach (multi-line role/policy args).
			methodBlock := body[mm[0]:mm[2]]
			if before := hcAttrBlockBefore(body, mm[0]); before != "" {
				methodBlock = before + methodBlock
			}
			v := classifyHCAuthBlock(methodBlock)
			if !v.decided {
				// Inherit the enclosing class verdict (HotChocolate honours a
				// type-level [Authorize] for every field on the type).
				v = classVerdict
			}
			if v.decided {
				out[className+"."+method] = v
			}
		}
	}
	return out
}

// hcAttrBlockBefore returns the contiguous run of `[...]` attribute lines (and
// intervening blank/comment-free whitespace) immediately preceding byte offset
// `at`. Stops at the previous non-attribute, non-whitespace line (e.g. the prior
// `}` or statement), so a method's block never bleeds into an earlier method.
func hcAttrBlockBefore(content string, at int) string {
	// Walk backwards over lines; collect a trailing run that is entirely
	// attribute(s) or whitespace.
	start := at
	for start > 0 {
		// Find start of the line preceding `start`.
		lineEnd := start
		lineStart := strings.LastIndexByte(content[:lineEnd], '\n')
		if lineStart < 0 {
			lineStart = 0
		} else {
			lineStart++
		}
		line := strings.TrimSpace(content[lineStart:lineEnd])
		if line == "" {
			// blank line — keep scanning upward
			if lineStart == 0 {
				break
			}
			start = lineStart - 1
			continue
		}
		// An attribute line both starts with `[` and ends with `]`.
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			start = lineStart
			if lineStart == 0 {
				break
			}
			start = lineStart - 1
			continue
		}
		break
	}
	if start < 0 {
		start = 0
	}
	return content[start:at]
}

// applyHotChocolateAuthShapes stamps the resolved [Authorize] policy onto every
// HotChocolate GRAPHQL endpoint emitted for this file (indices >= before). It is
// a no-op when the file carries no HotChocolate signal or emitted no endpoints.
func applyHotChocolateAuthShapes(content, path string, entities []types.EntityRecord, before int) {
	if before >= len(entities) || !hotChocolateHasSignal(content) {
		return
	}
	methodAuth := buildHCMethodAuth(content)
	if len(methodAuth) == 0 {
		return
	}
	for i := before; i < len(entities); i++ {
		e := &entities[i]
		if e.SourceFile != path || e.Properties == nil {
			continue
		}
		if e.Properties["framework"] != "hotchocolate" {
			continue
		}
		// source_handler = "SCOPE.Operation:<Class>.<Method>"
		sh := e.Properties["source_handler"]
		op := sh
		if c := strings.LastIndexByte(sh, ':'); c >= 0 {
			op = sh[c+1:]
		}
		v, ok := methodAuth[op]
		if !ok {
			continue
		}
		stampHCAuth(e.Properties, v)
	}
}

// stampHCAuth writes the verdict onto an endpoint's Properties using the shared
// auth contract (stampAuthPolicy) plus the signal-1 `auth_decorator` key so the
// MCP grafel_auth_coverage cheap property check fires. `[AllowAnonymous]`
// stamps auth_required=false (explicit public) without a guard symbol.
func stampHCAuth(props map[string]string, v hcAuthVerdict) {
	if !v.decided {
		return
	}
	policy := AuthPolicy{
		Method:     "annotation",
		Required:   v.required,
		Roles:      v.roles,
		Confidence: "high",
		SourceChain: []AuthSignal{{
			Kind: "annotation",
			Text: "[Authorize]",
		}},
	}
	if v.policy != "" {
		policy.Permissions = []string{v.policy}
	}
	stampAuthPolicy(props, policy)
	if v.required {
		// signal-1 key for the MCP auth_coverage property check: an attribute-
		// based guard, mirroring how decorator/attribute frameworks surface auth.
		props["auth_decorator"] = "Authorize"
	}
}
