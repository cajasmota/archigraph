package rust

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

// juniper is a code-first GraphQL server library for Rust (#4964). It is the
// sibling of async-graphql but uses a different macro vocabulary:
//
//	#[derive(GraphQLObject)]
//	struct User { id: i32, name: String }
//
//	#[derive(GraphQLInputObject)]
//	struct NewUser { name: String }
//
//	#[derive(GraphQLEnum)]
//	enum Episode { NewHope, Empire }
//
//	struct Query;
//	#[graphql_object]
//	impl Query {
//	    fn user(&self, id: i32) -> User { ... }
//	}
//
//	struct Mutation;
//	#[graphql_object]
//	impl Mutation {
//	    fn create_user(&self, new: NewUser) -> User { ... }
//	}
//
//	let schema = RootNode::new(Query, Mutation, Subscription);
//	// or: Schema::new(Query, EmptyMutation::new(), EmptySubscription::new());
//
// Each resolver method on a Query/Mutation/Subscription root maps to a synthetic
// GraphQL endpoint with verb GRAPHQL and path /graphql/<Root>/<field>, the EXACT
// canonical shape async-graphql (Rust), gqlgen (Go), Strawberry (Python),
// Apollo (JS), and Absinthe (Elixir) emit, so cross-repo client links join. Each
// GraphQLObject/GraphQLInputObject struct becomes a SCOPE.Schema DTO; GraphQLEnum
// becomes a DTO with role "enum". RootNode::new / Schema::new captures the schema
// root and its operation-root type arguments as a SCOPE.Service.
//
// HONEST LIMIT: identical to the async-graphql extractor. When the resolver impl
// block is not one of the three recognised roots (Query/Mutation/Subscription)
// the operation root is inferred as "Object" and fields are still emitted but are
// not addressable from a top-level GraphQL path (juniper field resolvers on a
// `#[graphql_object] impl User`). Cross-file schema composition (root types
// declared in another module than RootNode::new) is not chased. The
// `#[graphql(name = "...")]` field-rename attribute is not yet honoured for the
// GraphQL field name.

func init() {
	extractor.Register("custom_rust_juniper", &juniperExtractor{})
}

type juniperExtractor struct{}

func (e *juniperExtractor) Language() string { return "custom_rust_juniper" }

var (
	// `#[graphql_object]` (optionally with arguments like
	// `#[graphql_object(context = Ctx)]`) immediately preceding an `impl <Root>`
	// block, possibly with intervening attributes. The `#[graphql_subscription]`
	// variant is also accepted (subscription roots). Capture group 1 is the
	// implemented type name, which is the GraphQL operation root.
	reJuniperObjectImpl = regexp.MustCompile(
		`#\[graphql_(?:object|subscription)[^\]]*\]\s*(?:#\[[^\]]*\]\s*)*impl\s+(?:<[^>]*>\s*)?([A-Za-z_]\w*)`,
	)
	// A resolver method inside a `#[graphql_object] impl` block. juniper resolvers
	// are usually plain `fn`, but `async fn` is allowed on async runtimes. Capture
	// group 1 (async) or group 2 (sync) is the field name.
	reJuniperResolverFn = regexp.MustCompile(
		`(?m)^\s*(?:pub\s+)?async\s+fn\s+([A-Za-z_]\w*)\s*\(|^\s*(?:pub\s+)?fn\s+([A-Za-z_]\w*)\s*\(`,
	)
	// `#[derive(... GraphQLObject ...)] struct Name` and the GraphQLInputObject
	// variant. Capture group 1 is the derive list, group 2 is the type name.
	reJuniperDeriveType = regexp.MustCompile(
		`#\[derive\s*\(([^)]*\b(?:GraphQLObject|GraphQLInputObject)\b[^)]*)\)\]\s*(?:#\[[^\]]*\]\s*)*(?:pub\s+)?struct\s+([A-Za-z_]\w*)`,
	)
	// `#[derive(... GraphQLEnum ...)] enum Name` — juniper GraphQL enum.
	reJuniperDeriveEnum = regexp.MustCompile(
		`#\[derive\s*\(([^)]*\bGraphQLEnum\b[^)]*)\)\]\s*(?:#\[[^\]]*\]\s*)*(?:pub\s+)?enum\s+([A-Za-z_]\w*)`,
	)
	// `RootNode::new(` or `Schema::new(` — the call head. The argument list is
	// read with a paren-balanced scan (juniperCallArgs) so nested constructor
	// calls like `EmptyMutation::new()` do not truncate the root list.
	reJuniperSchemaBuildHead = regexp.MustCompile(
		`(?:RootNode|Schema)::new\s*\(`,
	)
)

func (e *juniperExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/rust")
	_, span := tracer.Start(ctx, "indexer.juniper_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "juniper"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "rust" {
		return nil, nil
	}

	src := string(file.Content)

	// File-signal gate: require a juniper marker so this extractor is a no-op on
	// plain Rust / async-graphql / axum files. The juniper-specific macros are
	// unambiguous; we deliberately do NOT key on bare `RootNode`/`Schema` alone.
	if !strings.Contains(src, "#[graphql_object") &&
		!strings.Contains(src, "#[graphql_subscription") &&
		!strings.Contains(src, "GraphQLObject") &&
		!strings.Contains(src, "GraphQLInputObject") &&
		!strings.Contains(src, "GraphQLEnum") {
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

	// 1. #[graphql_object] impl <Root> { fn field(...) } -> one GRAPHQL endpoint
	//    per resolver method, path /graphql/<Root>/<field>.
	for _, m := range reJuniperObjectImpl.FindAllStringSubmatchIndex(src, -1) {
		root := src[m[2]:m[3]]
		bodyStart, bodyEnd := agqlBlockBody(src, m[1])
		if bodyStart < 0 {
			continue
		}
		body := src[bodyStart:bodyEnd]

		operation := agqlOperationForRoot(root)

		for _, fm := range reJuniperResolverFn.FindAllStringSubmatchIndex(body, -1) {
			var field string
			if fm[2] >= 0 {
				field = body[fm[2]:fm[3]]
			} else if fm[4] >= 0 {
				field = body[fm[4]:fm[5]]
			}
			if field == "" {
				continue
			}
			// juniper convenience constructors are not resolver fields.
			if field == "new" {
				continue
			}
			fieldOff := bodyStart + fm[0]
			path := rustNormalizePath("/graphql/" + root + "/" + field)
			name := "GRAPHQL " + path
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, fieldOff))
			setProps(&ent, "framework", "juniper",
				"provenance", "INFERRED_FROM_JUNIPER_RESOLVER",
				"http_method", "GRAPHQL", "verb", "GRAPHQL",
				"route_path", path, "graphql_operation", operation,
				"graphql_root", root, "graphql_field", field,
				"handler_name", root+"."+field)
			add(ent)
		}
	}

	// 2. #[derive(GraphQLObject/GraphQLInputObject)] struct -> SCOPE.Schema DTO.
	for _, m := range reJuniperDeriveType.FindAllStringSubmatchIndex(src, -1) {
		derives := strings.TrimSpace(src[m[2]:m[3]])
		typeName := src[m[4]:m[5]]
		role := juniperDTORole(derives)
		ent := makeEntity("graphql_dto:"+typeName, "SCOPE.Schema", "dto", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "juniper",
			"provenance", "INFERRED_FROM_JUNIPER_DTO",
			"dto_name", typeName, "graphql_dto_role", role, "derives", derives)
		add(ent)
	}

	// 3. #[derive(GraphQLEnum)] enum -> SCOPE.Schema DTO (graphql enum).
	for _, m := range reJuniperDeriveEnum.FindAllStringSubmatchIndex(src, -1) {
		derives := strings.TrimSpace(src[m[2]:m[3]])
		typeName := src[m[4]:m[5]]
		ent := makeEntity("graphql_dto:"+typeName, "SCOPE.Schema", "dto", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "juniper",
			"provenance", "INFERRED_FROM_JUNIPER_DTO",
			"dto_name", typeName, "graphql_dto_role", "enum", "derives", derives)
		add(ent)
	}

	// 4. RootNode::new(Query, Mutation, Subscription) -> SCOPE.Service schema root.
	for _, m := range reJuniperSchemaBuildHead.FindAllStringIndex(src, -1) {
		rawArgs, ok := juniperCallArgs(src, m[1]-1) // m[1]-1 points at the '('
		if !ok {
			continue
		}
		roots := juniperSplitRoots(rawArgs)
		if len(roots) == 0 {
			continue
		}
		ent := makeEntity("graphql_schema:"+strings.Join(roots, ","), "SCOPE.Service", "graphql_schema", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "juniper",
			"provenance", "INFERRED_FROM_JUNIPER_SCHEMA",
			"schema_roots", strings.Join(roots, ","))
		if len(roots) >= 1 {
			setProps(&ent, "query_root", roots[0])
		}
		if len(roots) >= 2 {
			setProps(&ent, "mutation_root", roots[1])
		}
		if len(roots) >= 3 {
			setProps(&ent, "subscription_root", roots[2])
		}
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// juniperCallArgs returns the raw text inside the call parentheses whose opening
// `(` is at index openParen, read with paren-balancing so nested constructor
// calls (e.g. `EmptyMutation::new()`) are kept intact. Returns ("", false) when
// the parentheses are unbalanced.
func juniperCallArgs(src string, openParen int) (string, bool) {
	if openParen < 0 || openParen >= len(src) || src[openParen] != '(' {
		return "", false
	}
	depth := 0
	for i := openParen; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return src[openParen+1 : i], true
			}
		}
	}
	return "", false
}

// juniperSplitRoots splits a RootNode::new / Schema::new argument list into the
// individual root-type identifiers, splitting on TOP-LEVEL commas only (so a
// nested `EmptyMutation::new()` argument is one root) and stripping the
// constructor-call tail (`EmptyMutation::new()` -> `EmptyMutation`).
func juniperSplitRoots(raw string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '(', '<', '[':
			depth++
		case ')', '>', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, raw[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, raw[start:])

	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Drop any `::...`, `(...)`, or generic `<...>` tail; keep leading ident.
		if i := strings.IndexAny(p, ":(<"); i >= 0 {
			p = p[:i]
		}
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// juniperDTORole classifies a DTO by its juniper derive list.
func juniperDTORole(derives string) string {
	switch {
	case strings.Contains(derives, "GraphQLInputObject"):
		return "input"
	case strings.Contains(derives, "GraphQLObject"):
		return "object"
	default:
		return "object"
	}
}
