// Package php — regex-based Lighthouse (Laravel GraphQL) SDL extractor.
//
// Lighthouse (nuwave/lighthouse) is the schema-first Laravel GraphQL framework.
// It sits ON TOP of webonyx/graphql-php (modelled separately in graphql.go,
// issue #3544): instead of constructing ObjectType instances in PHP, Lighthouse
// reads a `.graphql` SDL schema in which server-side directives — `@all`,
// `@paginate`, `@find`, `@create`, `@field(resolver: ...)`, `@hasMany`, … —
// declare how each field is resolved. A typical schema.graphql:
//
//	type Query {
//	    users: [User!]! @paginate
//	    user(id: ID! @eq): User @find
//	    me: User @field(resolver: "App\\GraphQL\\Queries\\Me")
//	}
//
//	type Mutation {
//	    createUser(name: String!): User @create
//	    deleteUser(id: ID!): User @delete
//	}
//
//	type User {
//	    id: ID!
//	    name: String!
//	    posts: [Post!]! @hasMany
//	}
//
// Mapping (mirrors webonyx graphql.go / rust async-graphql / jsts
// graphql-resolvers):
//
//   - Each field of the root operation types Query / Mutation / Subscription
//     becomes a synthetic GRAPHQL endpoint with verb GRAPHQL and path
//     /graphql/<Operation>/<field>. The Lighthouse resolver directive on the
//     field (@all, @paginate, @field, …) is the handler attribution; for
//     @field(resolver: "Class@method") the resolver class is captured.
//   - Each non-root SDL `type` becomes a SCOPE.Schema DTO. Its relation fields
//     carrying an Eloquent-relation directive (@hasMany, @belongsTo, …) record
//     the directive so the field's resolver provenance is honest.
//
// HONEST LIMIT: this is file-local and SDL-structural. It reads the Lighthouse
// directive vocabulary off the `.graphql` schema; it does NOT chase the PHP
// resolver class named by @field(resolver:) into its definition (recorded by
// name only), nor schema-stitching across multiple imported `.graphql` files,
// nor programmatically-registered directives. PHP `#[...]` attribute resolvers
// (custom directive classes) are recorded by name when the field cites them via
// @field, but their PHP body is not parsed here.
//
// Registration key: "custom_php_lighthouse". Lighthouse SDL lives in `.graphql`
// files (language "graphql"), so this extractor gates on language "graphql"
// plus a Lighthouse-directive signal — unlike the php-language extractors in
// this package — and is reached because the dispatch routes the "graphql"
// language to the custom_php_ extractor set (mirrors prisma/sql → JS routing).
//
// Issue #3556 (epic #3505) — PHP Lighthouse (Laravel GraphQL) coverage.
package php

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_php_lighthouse", &lighthouseExtractor{})
}

type lighthouseExtractor struct{}

func (e *lighthouseExtractor) Language() string { return "custom_php_lighthouse" }

var (
	// `type Foo {` — start of an SDL type definition. Group 1 is the type name.
	// The byte offset of the `{` lets the caller balance the field block.
	reLHType = regexp.MustCompile(`(?m)^\s*(?:extend\s+)?type\s+([A-Za-z_]\w*)\s*(?:implements[^\{]*)?\{`)
	// A field declaration inside a type body: `name(args): ReturnType @dir`.
	// Group 1 is the field name; group 2 the optional arg list (without the
	// surrounding parens); group 3 the return type + any directives. Args in
	// parens are captured (group 2) so the request shape can be recovered.
	reLHField = regexp.MustCompile(`(?m)^\s*([A-Za-z_]\w*)\s*(?:\(([^)]*)\))?\s*:\s*([^\n]+)`)
	// A GraphQL return type at the head of a field's RHS, e.g. `[User!]!` /
	// `User` / `Int!`. Group 1 is the bare type name (brackets/bangs stripped).
	reLHReturnType = regexp.MustCompile(`^\s*([\[]*)([A-Za-z_]\w*)`)
	// A server-side resolver directive on a Query/Mutation/Subscription field.
	// These are the Lighthouse directives that make a field addressable.
	reLHResolverDir = regexp.MustCompile(`@(all|paginate|find|first|count|create|update|upsert|delete|forceDelete|restore|field|aggregate|hasMany|hasOne|belongsTo|belongsToMany|morphMany|morphOne|morphTo)\b`)
	// `@field(resolver: "App\\GraphQL\\Queries\\Me")` — captures the resolver
	// class/callable string named by an explicit @field directive. Group 1 is
	// the resolver reference (quote-stripped by the caller).
	reLHFieldResolver = regexp.MustCompile(`@field\s*\(\s*resolver\s*:\s*"([^"]+)"`)
	// `@guard` / `@guard(with: ["api"])` — Lighthouse's auth directive that
	// requires an authenticated user before the field resolver runs. Group 1 is
	// the optional `with:` guard list body (raw), used to record the guard name.
	reLHGuard = regexp.MustCompile(`@guard\b(?:\s*\(\s*with\s*:\s*\[([^\]]*)\])?`)
	// `@can(ability: "viewAny", model: "App\\User")` — Lighthouse's policy-gate
	// directive. The new (v5+) argument key is `ability:`; the legacy key is
	// `if:`. Group 1 / group 2 capture whichever ability string is present.
	reLHCan = regexp.MustCompile(`@can\w*\s*\(\s*(?:ability|if)\s*:\s*"([^"]+)"`)
	// A single-quoted / array-of element inside a `with:` guard list, e.g.
	// `["api", "web"]` → api, web. Group 1 is one guard name.
	reLHGuardName = regexp.MustCompile(`"([A-Za-z0-9_]+)"`)
	// An SDL argument inside a field's `( ... )` parameter list: `id: ID!` /
	// `name: String! @rules(...)`. Group 1 is the arg name, group 2 the type
	// up to a comma / close-paren / directive. Used for the request shape.
	reLHArg = regexp.MustCompile(`([A-Za-z_]\w*)\s*:\s*([\[\]!A-Za-z_]\w*[\]!]*)`)
)

// lhResolverDirective returns the first Lighthouse server-side resolver
// directive present on a field's line (e.g. "paginate", "all", "field"), or ""
// when the field carries no such directive (a plain data field).
func lhResolverDirective(line string) string {
	if m := reLHResolverDir.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// lhTypeBody returns the byte range [open+1, close) of the `{ ... }` block whose
// opening brace is the first `{` at or after from in src, balancing nested
// braces. Returns (-1,-1) when no balanced block is found.
func lhTypeBody(src string, from int) (int, int) {
	open := strings.IndexByte(src[from:], '{')
	if open < 0 {
		return -1, -1
	}
	open += from
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return open + 1, i
			}
		}
	}
	return -1, -1
}

// lhOperationForType maps an SDL type name to its GraphQL operation kind, or ""
// when the type is an ordinary object type (a DTO). Lighthouse/GraphQL
// convention names the roots Query / Mutation / Subscription.
func lhOperationForType(name string) string {
	switch name {
	case "Query", "Mutation", "Subscription":
		return name
	default:
		return ""
	}
}

// lhStampAuth stamps the canonical auth-policy property contract onto a
// resolver-field endpoint when the field's SDL line carries a Lighthouse auth
// directive. @guard requires an authenticated principal (method "guard"); @can
// gates the field on one or more policy abilities (method "policy",
// auth_permissions = the abilities). When both are present @can wins the method
// (it implies authentication) but @guard's guard list is still recorded. No-op
// when the line carries neither directive (the field stays unauthenticated).
func lhStampAuth(ent *types.EntityRecord, line string) {
	guardM := reLHGuard.FindStringSubmatch(line)
	canM := reLHCan.FindStringSubmatch(line)
	if guardM == nil && canM == nil {
		return
	}
	setProps(ent, "auth_required", "true", "auth_confidence", "high")
	if canM != nil {
		setProps(ent, "auth_method", "policy",
			"auth_permissions", canM[1],
			"auth_directive", "@can")
	} else {
		setProps(ent, "auth_method", "guard", "auth_directive", "@guard")
	}
	if guardM != nil {
		setProps(ent, "auth_guard", "@guard")
		if guardM[1] != "" {
			var guards []string
			for _, g := range reLHGuardName.FindAllStringSubmatch(guardM[1], -1) {
				guards = append(guards, g[1])
			}
			if len(guards) > 0 {
				setProps(ent, "auth_guards", strings.Join(guards, ","))
			}
		}
	}
}

// lhRequestShape renders a field's SDL argument list into a compact, stable
// request-shape string: "name:Type,other:Type" in source order. SDL directives
// on individual args (@eq, @rules, …) are stripped — only the arg name and its
// declared GraphQL type are kept. Returns "" for an empty / argument-less field.
func lhRequestShape(args string) string {
	args = strings.TrimSpace(args)
	if args == "" {
		return ""
	}
	var parts []string
	seen := map[string]bool{}
	// Split on top-level commas; each segment is `name: Type @dir...`. Take the
	// leading `name: Type` via the arg regex, anchored at the segment head.
	for _, seg := range strings.Split(args, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		m := reLHArg.FindStringSubmatch(seg)
		if m == nil || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		parts = append(parts, m[1]+":"+m[2])
	}
	return strings.Join(parts, ",")
}

// lhReturnTypeName extracts the bare GraphQL return-type name from a field's RHS
// line (everything after the `:`), stripping list brackets and non-null bangs:
// "[User!]! @paginate" → "User", "Int!" → "Int". Returns "" when no leading type
// name is present.
func lhReturnTypeName(line string) string {
	m := reLHReturnType.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return m[2]
}

func (e *lighthouseExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/php")
	_, span := tracer.Start(ctx, "indexer.lighthouse_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "lighthouse"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	// Lighthouse SDL lives in `.graphql` files (language "graphql"), not "php".
	if len(file.Content) == 0 || file.Language != "graphql" {
		return nil, nil
	}

	src := string(file.Content)

	// File-signal gate: require a Lighthouse-specific server-side directive so
	// this extractor is a no-op on plain GraphQL SDL (client schemas, Apollo,
	// federation, …). The directive vocabulary below is Lighthouse's own.
	if !reLHResolverDir.MatchString(src) {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	for _, m := range reLHType.FindAllStringSubmatchIndex(src, -1) {
		typeName := src[m[2]:m[3]]
		operation := lhOperationForType(typeName)

		bs, be := lhTypeBody(src, m[0])
		if bs < 0 {
			continue
		}
		body := src[bs:be]

		if operation == "" {
			// Ordinary object type → SCOPE.Schema DTO.
			ent := makeEntity("lighthouse_dto:"+typeName, "SCOPE.Schema", "dto", file.Path, file.Language, lineOf(src, m[0]))
			setProps(&ent, "framework", "lighthouse",
				"provenance", "INFERRED_FROM_LIGHTHOUSE_TYPE",
				"dto_name", typeName, "graphql_dto_role", "object")
			add(ent)
			continue
		}

		// Operation root → one GRAPHQL endpoint per field carrying a resolver
		// directive. Fields without a resolver directive on a root type are
		// rare but skipped (Lighthouse always resolves root fields via a
		// directive or a convention-matched resolver class).
		for _, fm := range reLHField.FindAllStringSubmatchIndex(body, -1) {
			field := body[fm[2]:fm[3]]
			args := ""
			if fm[4] >= 0 {
				args = body[fm[4]:fm[5]]
			}
			line := body[fm[6]:fm[7]]
			directive := lhResolverDirective(line)
			if directive == "" {
				continue
			}

			path := "/graphql/" + operation + "/" + field
			name := "GRAPHQL " + path
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, bs+fm[0]))
			setProps(&ent, "framework", "lighthouse",
				"provenance", "INFERRED_FROM_LIGHTHOUSE_RESOLVER",
				"http_method", "GRAPHQL", "verb", "GRAPHQL",
				"route_path", path, "graphql_operation", operation,
				"graphql_root", operation, "graphql_field", field,
				"handler_name", operation+"."+field,
				"lighthouse_directive", directive)
			if rm := reLHFieldResolver.FindStringSubmatch(line); rm != nil {
				setProps(&ent, "resolver_class", rm[1])
			}

			// Auth (#3872): @guard requires an authenticated user; @can gates
			// the field on a policy ability. Either makes the operation
			// auth-protected. Stamp the canonical auth contract (auth_required /
			// auth_method / auth_confidence / auth_permissions) shared with the
			// JS/TS + Java resolvers so grafel_auth_coverage answers "is this
			// resolver protected, and by what?".
			lhStampAuth(&ent, line)

			// Request shape (#3872): the field's SDL argument list
			// (`user(id: ID!, filter: String)`) is the request shape. Recover the
			// typed arg names → request_shape. Honest-partial: only the
			// SDL-declared arg types, not the PHP input class behind a custom
			// input object.
			if rs := lhRequestShape(args); rs != "" {
				setProps(&ent, "request_shape", rs,
					"request_shape_source", "lighthouse_sdl_args")
			}
			// Response shape (#3872): the field's SDL return type (`[User!]!` →
			// User) is the response shape.
			if rt := lhReturnTypeName(line); rt != "" {
				setProps(&ent, "response_shape", rt,
					"response_shape_source", "lighthouse_sdl_return")
			}
			add(ent)
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
