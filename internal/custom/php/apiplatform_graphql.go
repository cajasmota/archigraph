// Package php — regex-based API Platform GraphQL-resource extractor.
//
// API Platform (api-platform/core) can expose a resource over GraphQL in
// addition to (or instead of) REST. GraphQL is opted into on the `#[ApiResource]`
// attribute via the `graphQlOperations:` argument, which lists the GraphQL
// operation objects to generate:
//
//	#[ApiResource(
//	    graphQlOperations: [
//	        new Query(),
//	        new QueryCollection(),
//	        new Mutation(name: 'create', security: "is_granted('ROLE_ADMIN')"),
//	        new DeleteMutation(name: 'delete'),
//	    ]
//	)]
//	class Book
//	{
//	    public int $id;
//	    public string $title;
//	    public ?Author $author = null;
//	}
//
// A bare `graphQlOperations: []` (empty array) — or the documented shorthand
// `graphql: true` — generates the default GraphQL operation set for the resource
// (item Query, collection QueryCollection, create/update/delete Mutations).
//
// Mapping (mirrors the canonical GRAPHQL endpoint shape shared with
// graphql.go / lighthouse.go / gqlgen / Strawberry — SCOPE.Operation endpoints
// with http_method=GRAPHQL and path /graphql/<Operation>/<field>):
//
//   - Each declared GraphQL operation becomes a SCOPE.Operation GRAPHQL
//     endpoint. Query / QueryCollection map to the Query root; Mutation /
//     DeleteMutation map to the Mutation root; Subscription to the Subscription
//     root. The field name is the operation's `name:` when given, else derived
//     from the resource short-name (Book → "book" for item Query, "books" for
//     collection, "createBook" / "deleteBook" for mutations).
//   - The operation's `security:` Symfony expression is parsed for auth: any
//     `is_granted('ROLE_*')` / `is_granted('ROLE_*', object)` clauses become
//     auth_roles, and presence of any security expression makes the operation
//     auth_required. The raw expression is recorded for provenance.
//   - The resource's typed public properties are the response shape; for
//     mutations the same typed properties (minus the server-managed id) are the
//     request (input) shape. Honest-partial: only statically-typed public
//     properties are recovered, not custom input/output DTO classes named via
//     `input:`/`output:` on the operation.
//
// HONEST LIMIT: file-local and structural. The REST side of #[ApiResource] is
// handled by apiplatform.go (this extractor only fires when graphQlOperations /
// graphql is present). Custom input/output DTO classes, resolver classes named
// via `resolver:`, `graphql: true` field-set customisation, and Symfony voter
// bodies behind `is_granted(...)` are not chased. Default-set field names use a
// lightweight pluraliser shared with the REST extractor.
//
// Registration key: "custom_php_api_platform_graphql".
// Epic #3872 — PHP GraphQL parity (Lighthouse / API Platform GraphQL / webonyx).
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
	extractor.Register("custom_php_api_platform_graphql", &apiPlatformGraphQLExtractor{})
}

type apiPlatformGraphQLExtractor struct{}

func (e *apiPlatformGraphQLExtractor) Language() string {
	return "custom_php_api_platform_graphql"
}

var (
	// `#[ApiResource(` — start of the resource attribute. The argument body is
	// extracted by balancing brackets from the `(` (gqlpBalanced), which handles
	// the arbitrary nesting of graphQlOperations: [ new Mutation(security:
	// "is_granted(...)") ] that a single-level regex cannot.
	reAPGQLResourceStart = regexp.MustCompile(`#\[ApiResource\b\s*\(`)
	// `class Foo` — the class the attribute decorates. Group 1 is the class name.
	reAPGQLClass = regexp.MustCompile(`(?m)^\s*(?:final\s+|abstract\s+)*class\s+([A-Za-z_]\w*)`)
	// `graphQlOperations:` — the GraphQL opt-in argument. The byte offset of the
	// following `[` lets the caller balance the operations array.
	reAPGQLOpsKey = regexp.MustCompile(`graphQlOperations\s*:\s*\[`)
	// `graphql: true` — the shorthand opt-in that generates the default GraphQL
	// operation set.
	reAPGQLShorthand = regexp.MustCompile(`graphql\s*:\s*true\b`)
	// `new Query(...)` / `new QueryCollection()` / `new Mutation(...)` /
	// `new DeleteMutation(...)` / `new Subscription(...)` — a GraphQL operation
	// constructor. Group 1 is the operation class, group "body" its arg body.
	reAPGQLOpNew = regexp.MustCompile(`new\s+(Query|QueryCollection|Mutation|DeleteMutation|Subscription)\s*\((?P<body>(?:[^()]|\([^()]*\))*)\)`)
	// `name: 'create'` inside an operation body. Group 1 is the operation name.
	reAPGQLName = regexp.MustCompile(`\bname\s*:\s*['"]([^'"]+)['"]`)
	// `security: "is_granted('ROLE_ADMIN')"` — the Symfony security expression on
	// an operation. Group 1 is the raw expression (double-quote delimited).
	reAPGQLSecurity = regexp.MustCompile(`\bsecurity\s*:\s*"((?:[^"\\]|\\.)*)"`)
	// `is_granted('ROLE_ADMIN')` inside a security expression. Group 1 is the
	// granted attribute (typically a ROLE_* string, but any literal is captured).
	reAPGQLIsGranted = regexp.MustCompile(`is_granted\s*\(\s*['"]([^'"]+)['"]`)
	// A typed public property: `public string $title;` / `public ?Author $author`.
	// Group 1 is the (nullable-stripped) type, group 2 the property name.
	reAPGQLProp = regexp.MustCompile(`(?m)^\s*public\s+(?:readonly\s+)?\??([A-Za-z_][\w\\]*)\s+\$([A-Za-z_]\w*)`)
)

// apGQLOperation describes one generated GraphQL operation: its operation class,
// the GraphQL root it lives under, and whether it is a collection query.
type apGQLOperation struct {
	class      string // API Platform GraphQL operation class
	root       string // Query | Mutation | Subscription
	collection bool   // true → collection query
	mutation   bool   // true → a write operation (Mutation/DeleteMutation)
}

// apGQLOperationMeta maps a GraphQL operation class name to its root and shape.
func apGQLOperationMeta(class string) (apGQLOperation, bool) {
	switch class {
	case "Query":
		return apGQLOperation{class, "Query", false, false}, true
	case "QueryCollection":
		return apGQLOperation{class, "Query", true, false}, true
	case "Mutation":
		return apGQLOperation{class, "Mutation", false, true}, true
	case "DeleteMutation":
		return apGQLOperation{class, "Mutation", false, true}, true
	case "Subscription":
		return apGQLOperation{class, "Subscription", false, false}, true
	}
	return apGQLOperation{}, false
}

// apGQLDefaultOps is the operation set a bare `graphQlOperations: []` or
// `graphql: true` shorthand generates.
var apGQLDefaultOps = []apGQLOperation{
	{"Query", "Query", false, false},
	{"QueryCollection", "Query", true, false},
	{"Mutation", "Mutation", false, true},
	{"DeleteMutation", "Mutation", false, true},
}

// apGQLFieldName derives the GraphQL field name for an operation when the
// operation declares no explicit `name:`. API Platform's default field naming is
// the lower-camel resource short-name: item Query → "book", collection →
// pluralised "books", create Mutation → "createBook", delete → "deleteBook".
func apGQLFieldName(op apGQLOperation, className string) string {
	lower := strings.ToLower(className[:1]) + className[1:]
	switch {
	case op.collection:
		// Reuse the REST pluraliser then lower-camel it.
		return strings.TrimPrefix(apResourcePath(className), "/")
	case op.class == "DeleteMutation":
		return "delete" + className
	case op.mutation:
		return "create" + className
	default:
		return lower
	}
}

func (e *apiPlatformGraphQLExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/php")
	_, span := tracer.Start(ctx, "indexer.api_platform_graphql_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "api-platform-graphql"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "php" {
		return nil, nil
	}

	src := string(file.Content)

	// File-signal gate: require the GraphQL opt-in (graphQlOperations: or the
	// `graphql: true` shorthand) so this extractor is a no-op on REST-only
	// #[ApiResource] (owned by apiplatform.go) and on plain PHP.
	if !strings.Contains(src, "graphQlOperations") && !reAPGQLShorthand.MatchString(src) {
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

	for _, rm := range reAPGQLResourceStart.FindAllStringIndex(src, -1) {
		// Balance the #[ApiResource( ... )] argument body from its opening paren.
		bs, be := gqlpBalanced(src, rm[1]-1)
		if bs < 0 {
			continue
		}
		resBody := src[bs:be]
		// The attribute ends at the `]` after the balanced `)`; the class follows.
		afterAttr := be + 1
		hasOpsKey := reAPGQLOpsKey.MatchString(resBody)
		hasShorthand := reAPGQLShorthand.MatchString(resBody)
		if !hasOpsKey && !hasShorthand {
			continue // a REST-only resource sharing the file — skip.
		}

		clsM := reAPGQLClass.FindStringSubmatchIndex(src[afterAttr:])
		if clsM == nil {
			continue
		}
		className := src[afterAttr+clsM[2] : afterAttr+clsM[3]]
		classOff := afterAttr + clsM[0]
		line := lineOf(src, rm[0])

		// Recover the resource's typed public properties (the response/output
		// shape). Bound the scan to the class body.
		respShape := apGQLPropertyShape(src, classOff)

		// Collect declared operations from graphQlOperations: [ ... ]. When the
		// list is empty OR only the shorthand is present, fall back to the
		// default operation set.
		type opEntry struct {
			op       apGQLOperation
			name     string
			security string
			roles    []string
		}
		var ops []opEntry
		if hasOpsKey {
			if loc := reAPGQLOpsKey.FindStringIndex(resBody); loc != nil {
				as, ae := gqlpBalanced(resBody, loc[1]-1)
				if as >= 0 {
					opsBody := resBody[as:ae]
					for _, om := range reAPGQLOpNew.FindAllStringSubmatch(opsBody, -1) {
						meta, ok := apGQLOperationMeta(om[1])
						if !ok {
							continue
						}
						body := om[2]
						entry := opEntry{op: meta}
						if nm := reAPGQLName.FindStringSubmatch(body); nm != nil {
							entry.name = nm[1]
						}
						if sm := reAPGQLSecurity.FindStringSubmatch(body); sm != nil {
							entry.security = sm[1]
							for _, gm := range reAPGQLIsGranted.FindAllStringSubmatch(sm[1], -1) {
								entry.roles = append(entry.roles, gm[1])
							}
						}
						ops = append(ops, entry)
					}
				}
			}
		}
		if len(ops) == 0 {
			// Empty graphQlOperations: [] or graphql: true shorthand → default set.
			for _, op := range apGQLDefaultOps {
				ops = append(ops, opEntry{op: op})
			}
		}

		for _, x := range ops {
			field := x.name
			if field == "" {
				field = apGQLFieldName(x.op, className)
			}
			path := "/graphql/" + x.op.root + "/" + field
			name := "GRAPHQL " + path
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, line)
			setProps(&ent, "framework", "api-platform-graphql",
				"provenance", "INFERRED_FROM_API_PLATFORM_GRAPHQL",
				"http_method", "GRAPHQL", "verb", "GRAPHQL",
				"route_path", path, "graphql_operation", x.op.root,
				"graphql_root", x.op.root, "graphql_field", field,
				"handler_name", className+"."+field,
				"resource_class", className,
				"api_platform_graphql_operation", x.op.class)
			if x.op.collection {
				setProps(&ent, "graphql_target", "collection")
			}

			// Auth (#3872): a `security:` expression makes the operation
			// auth-protected; is_granted('ROLE_*') clauses become auth_roles.
			if x.security != "" {
				setProps(&ent, "auth_required", "true",
					"auth_method", "expression",
					"auth_confidence", "high",
					"auth_expression", x.security)
				if len(x.roles) > 0 {
					setProps(&ent, "auth_roles", strings.Join(x.roles, ","))
				}
			}

			// Response shape (#3872): the resource's typed public properties.
			if respShape != "" {
				setProps(&ent, "response_shape", respShape,
					"response_shape_source", "api_platform_resource_props")
			}
			// Request shape (#3872): mutations take the same typed properties as
			// input (minus the server-managed id).
			if x.op.mutation {
				if rs := apGQLInputShape(respShape); rs != "" {
					setProps(&ent, "request_shape", rs,
						"request_shape_source", "api_platform_resource_props")
				}
			}
			add(ent)
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// apGQLPropertyShape renders a resource class's typed public properties into a
// compact shape string "name:Type,other:Type" in declaration order. The scan
// starts at the class declaration offset and is bounded by the matching `}` of
// the class body so properties of an adjacent class in the same file are not
// mixed in.
func apGQLPropertyShape(src string, classOff int) string {
	bodyOpen := strings.IndexByte(src[classOff:], '{')
	if bodyOpen < 0 {
		return ""
	}
	bodyOpen += classOff
	// Balance the class body braces.
	depth := 0
	end := len(src)
	for i := bodyOpen; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
				goto bounded
			}
		}
	}
bounded:
	body := src[bodyOpen:end]
	var parts []string
	seen := map[string]bool{}
	for _, pm := range reAPGQLProp.FindAllStringSubmatch(body, -1) {
		typ, nameP := pm[1], pm[2]
		// Drop method-modifier false positives: a `public function` declares a
		// method, not a property — its "type" capture would be "function".
		if typ == "function" || typ == "const" || typ == "static" {
			continue
		}
		if seen[nameP] {
			continue
		}
		seen[nameP] = true
		// Use the short class name for namespaced/object types (App\Author →
		// Author) for a readable shape.
		if idx := strings.LastIndex(typ, "\\"); idx >= 0 {
			typ = typ[idx+1:]
		}
		parts = append(parts, nameP+":"+typ)
	}
	return strings.Join(parts, ",")
}

// apGQLInputShape derives a mutation's input shape from the resource shape by
// dropping a leading server-managed `id:` field. Returns the (possibly
// unchanged) shape, or "" when nothing remains.
func apGQLInputShape(respShape string) string {
	if respShape == "" {
		return ""
	}
	parts := strings.Split(respShape, ",")
	var kept []string
	for _, p := range parts {
		if strings.HasPrefix(p, "id:") {
			continue
		}
		kept = append(kept, p)
	}
	return strings.Join(kept, ",")
}
