// Package kotlin — regex-based graphql-kotlin (Expedia Group) extractor.
//
// graphql-kotlin is a code-first GraphQL server library for Kotlin. Schema
// roots are ordinary classes that implement one of the three marker
// interfaces `Query`, `Mutation`, or `Subscription`; their public member
// functions become the GraphQL fields of that operation:
//
//	class UserQuery : Query {
//	    fun user(id: ID): User { ... }
//	    fun users(): List<User> { ... }
//	}
//
//	class UserMutation : Mutation {
//	    @GraphQLName("createUser")
//	    fun addUser(input: NewUser): User { ... }
//	}
//
// Each public resolver function on a Query/Mutation/Subscription root maps to
// a synthetic GraphQL endpoint with verb GRAPHQL and path
// /graphql/<Operation>/<field>, mirroring the rust async-graphql and
// jsts/strawberry GraphQL models. The function is the resolver handler for
// that field (handler_name = <Class>.<fun>).
//
// Annotations recognised on classes and functions:
//   - @GraphQLIgnore    → the function/class is excluded from the schema (skip)
//   - @GraphQLName("x") → the function/class is renamed to "x" in the schema
//   - @GraphQLDescription("...") → documentation, captured as a property
//
// Types referenced by the schema — `data class` DTOs and classes annotated
// with @GraphQLName — become SCOPE.Schema DTO entities.
//
// HONEST LIMIT: resolver discovery is file-local and structural. A field's
// return type that lives in another file is not resolved to its DTO here, and
// kotlin-graphql's reflection-based extension functions (top-level
// `fun User.fullName()` field resolvers attached to a type defined elsewhere)
// are not associated with their target type. Only classes that directly
// declare a Query/Mutation/Subscription supertype are treated as operation
// roots — a root composed via SchemaGeneratorConfig in another module is not
// chased.
//
// Registration key: "custom_kotlin_graphql_kotlin"
// Issue #3537 (epic #3505) — Kotlin graphql-kotlin coverage.
package kotlin

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("custom_kotlin_graphql_kotlin", &graphqlKotlinExtractor{})
}

type graphqlKotlinExtractor struct{}

func (e *graphqlKotlinExtractor) Language() string { return "custom_kotlin_graphql_kotlin" }

var (
	// `class <Name>(...)? : <supertypes>` capturing the class name (group 1)
	// and the raw supertype list (group 2). `: ` is followed by everything up
	// to the opening brace of the class body. Matches both
	// `class UserQuery : Query {` and `class UserQuery() : Query, Loggable {`.
	reGQLKClassDecl = regexp.MustCompile(
		`(?m)^[ \t]*(?:(?:public|internal|open|final|abstract|sealed|data)\s+)*class\s+([A-Za-z_]\w*)\s*(?:\([^)]*\))?\s*:\s*([^{]+?)\s*\{`,
	)
	// A public member function inside a class body:
	//   fun user(id: ID): User { ... }
	//   suspend fun users(): List<User> { ... }
	// Capture group 1 is the function name. Functions explicitly marked
	// `private`/`protected`/`internal` are excluded (not exposed in the
	// schema). Only top-of-line declarations are matched so nested local funs
	// and lambda bodies are ignored at the lexical depth handled by the caller.
	reGQLKFun = regexp.MustCompile(
		`(?m)^[ \t]*(?:(?:public|open|override|final|suspend|inline)\s+)*fun\s+([A-Za-z_]\w*)\s*\(`,
	)
	// reGQLKFunSig captures the full resolver signature from the fun name through
	// to the body/expression start: group 1 = fun name, group 2 = the parenthesised
	// argument list (without the outer parens), group 3 = the optional `: ReturnType`
	// return-type slice (everything between the closing arg paren and the next `{`
	// or `=`). Used for Relay/connection pagination detection (first/after/last/
	// before args, *Connection return type).
	reGQLKFunSig = regexp.MustCompile(
		`(?m)^[ \t]*(?:(?:public|open|override|final|suspend|inline)\s+)*fun\s+([A-Za-z_]\w*)\s*\(([^)]*)\)\s*(?::\s*([^={\n]+))?`,
	)
	// reGQLKConnReturn matches a Relay-style connection return type — a generic or
	// plain type whose name ends in `Connection` (e.g. `UserConnection`,
	// `Connection<User>`). Anchored on a word boundary so `Disconnection` does not
	// match.
	reGQLKConnReturn = regexp.MustCompile(`\b([A-Za-z_]\w*)?Connection\b`)
	// reGQLKPageArg matches a Relay forward/backward pagination argument by name
	// (first/after/last/before) in a resolver argument list. Matched as a whole
	// identifier immediately followed by `:` so a substring like `firstName` is not
	// a false positive.
	reGQLKPageArg = regexp.MustCompile(`(?m)(?:^|[\s,(])\s*(first|after|last|before)\s*:`)
	// Non-exposed function modifiers — if any precede `fun`, the function is
	// not a GraphQL field.
	reGQLKNonPublicFun = regexp.MustCompile(
		`(?m)^[ \t]*(?:[A-Za-z]+\s+)*(?:private|protected|internal)\s+(?:[A-Za-z]+\s+)*fun\s+`,
	)
	// `data class <Name>` — a Kotlin DTO that becomes a GraphQL object/input
	// type when referenced from the schema. Capture group 1 is the type name.
	reGQLKDataClass = regexp.MustCompile(
		`(?m)^[ \t]*(?:(?:public|internal|open)\s+)*data\s+class\s+([A-Za-z_]\w*)`,
	)
	// `@GraphQLName("x")` annotation argument extraction.
	reGQLKName = regexp.MustCompile(`@GraphQLName\s*\(\s*"([^"]*)"\s*\)`)
	// `@GraphQLDescription("...")` annotation argument extraction.
	reGQLKDescription = regexp.MustCompile(`@GraphQLDescription\s*\(\s*"([^"]*)"\s*\)`)
)

// gqlkOperationForSupertypes returns the GraphQL operation kind if the class's
// supertype list contains one of the three graphql-kotlin marker interfaces,
// or "" when the class is not an operation root. The marker interfaces are
// matched as whole identifiers so `MyQuery` or `QueryBuilder` do not match.
func gqlkOperationForSupertypes(supertypes string) string {
	for _, raw := range strings.Split(supertypes, ",") {
		t := strings.TrimSpace(raw)
		// Drop any constructor-call / generic suffix: `Query()` or `Query<X>`.
		if i := strings.IndexAny(t, "(<"); i >= 0 {
			t = strings.TrimSpace(t[:i])
		}
		switch t {
		case "Query":
			return "Query"
		case "Mutation":
			return "Mutation"
		case "Subscription":
			return "Subscription"
		}
	}
	return ""
}

// gqlkAnnotationBlock returns the source slice covering the annotation lines
// immediately preceding the entity declaration at declOffset within src. It
// walks backwards over contiguous lines that are blank or begin (after
// indentation) with '@'. This lets us inspect @GraphQLIgnore / @GraphQLName /
// @GraphQLDescription that decorate the following class or fun.
func gqlkAnnotationBlock(src string, declOffset int) string {
	end := declOffset
	i := declOffset
	for i > 0 {
		// Find the start of the line preceding position i.
		lineEnd := i
		if lineEnd > 0 && src[lineEnd-1] == '\n' {
			lineEnd--
		}
		lineStart := strings.LastIndexByte(src[:lineEnd], '\n') + 1
		line := strings.TrimSpace(src[lineStart:lineEnd])
		if line == "" || strings.HasPrefix(line, "@") {
			i = lineStart
			continue
		}
		break
	}
	if i >= end {
		return ""
	}
	return src[i:end]
}

// gqlkRelayPosture inspects a resolver's argument list and return-type slice for
// the Relay/connection-pagination shape and returns ("", "") when neither signal
// is present. The posture string is "relay_connection" when a *Connection return
// type and/or any forward/backward Relay pagination argument (first/after/last/
// before) is found. The second return is the comma-joined set of detected Relay
// argument names (evidence for the dashboard), or "" when only the return type
// signalled. Honest-partial: this is a structural shape match — a hand-rolled
// cursor scheme that does not follow the Relay naming is not detected.
func gqlkRelayPosture(args, returnType string) (posture, pageArgs string) {
	var found []string
	for _, m := range reGQLKPageArg.FindAllStringSubmatch(args, -1) {
		found = append(found, m[1])
	}
	connReturn := reGQLKConnReturn.MatchString(returnType)
	if len(found) == 0 && !connReturn {
		return "", ""
	}
	return "relay_connection", strings.Join(found, ",")
}

func (e *graphqlKotlinExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("archigraph/custom/kotlin")
	_, span := tracer.Start(ctx, "indexer.graphql_kotlin_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "graphql-kotlin"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "kotlin" {
		return nil, nil
	}

	src := string(file.Content)

	// File-signal gate: require a graphql-kotlin marker so this extractor is a
	// no-op on plain Kotlin / Ktor / Spring files. The Expedia annotations and
	// the marker-interface supertypes are the unambiguous signals.
	if !strings.Contains(src, "GraphQL") &&
		!strings.Contains(src, ": Query") &&
		!strings.Contains(src, ": Mutation") &&
		!strings.Contains(src, ": Subscription") {
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

	// 1. Operation roots: a class implementing Query/Mutation/Subscription.
	//    Each public member fun → one GRAPHQL endpoint
	//    /graphql/<Operation>/<field> with the fun as resolver handler.
	for _, m := range reGQLKClassDecl.FindAllStringSubmatchIndex(src, -1) {
		className := src[m[2]:m[3]]
		supertypes := src[m[4]:m[5]]
		operation := gqlkOperationForSupertypes(supertypes)
		if operation == "" {
			continue
		}

		// The class declaration's opening brace is the last char of the match.
		bodyOpen := m[1] - 1
		bodyEnd := matchBraceKotlin(src, bodyOpen)
		if bodyEnd < 0 {
			continue
		}
		body := src[bodyOpen+1 : bodyEnd]

		// A @GraphQLIgnore on the class removes the whole root from the schema.
		if strings.Contains(gqlkAnnotationBlock(src, m[0]), "@GraphQLIgnore") {
			continue
		}

		for _, fm := range reGQLKFun.FindAllStringSubmatchIndex(body, -1) {
			funOff := fm[0]
			// Skip explicitly non-public functions.
			lineStart := strings.LastIndexByte(body[:funOff], '\n') + 1
			funLine := body[lineStart:fm[1]]
			if reGQLKNonPublicFun.MatchString(funLine) {
				continue
			}

			funName := body[fm[2]:fm[3]]
			annBlock := gqlkAnnotationBlock(body, lineStart)
			if strings.Contains(annBlock, "@GraphQLIgnore") {
				continue
			}

			// @GraphQLName rename overrides the schema field name.
			fieldName := funName
			if nm := reGQLKName.FindStringSubmatch(annBlock); nm != nil {
				fieldName = nm[1]
			}

			path := "/graphql/" + operation + "/" + fieldName
			name := "GRAPHQL " + path
			ent := makeEntity(name, "SCOPE.Operation", "endpoint", file.Path, file.Language, lineOf(src, bodyOpen+1+funOff))
			setProps(&ent, "framework", "graphql-kotlin",
				"provenance", "INFERRED_FROM_GRAPHQL_KOTLIN_RESOLVER",
				"http_method", "GRAPHQL", "verb", "GRAPHQL",
				"route_path", path, "graphql_operation", operation,
				"graphql_root", className, "graphql_field", fieldName,
				"resolver_fun", funName,
				"handler_name", className+"."+funName)
			if desc := reGQLKDescription.FindStringSubmatch(annBlock); desc != nil {
				setProps(&ent, "graphql_description", desc[1])
			}

			// GraphQL-native field deprecation (#5010): a @Deprecated annotation /
			// KDoc @deprecated tag in the annotation block above the resolver fun
			// marks the FIELD as deprecated. This is the GraphQL-schema-directive
			// analogue of REST endpoint_deprecation_versioning — distinct, and
			// stamped on the synthesised GraphQL endpoint. Reuses the shared Kotlin
			// deprecation resolver/contract (endpoint_deprecation.go) so the
			// deprecated / deprecated_since / deprecated_replacement / deprecation_source
			// property names match the flagship contract.
			if verdict, dep := ktResolveAnnotationDeprecation(annBlock); dep {
				ktStampDeprecation(&ent, verdict)
				setProps(&ent, "graphql_deprecated", "true")
			}

			// GraphQL-native Relay/connection pagination (#5010): a resolver
			// returning a *Connection type and/or taking forward/backward Relay
			// args (first/after/last/before) carries the Relay connection
			// pagination posture. Distinct from REST limit+offset query-param
			// detection — stamped on the GraphQL field.
			sigEnd := fm[1] + 400
			if sigEnd > len(body) {
				sigEnd = len(body)
			}
			sig := reGQLKFunSig.FindStringSubmatch(body[lineStart:sigEnd])
			if sig != nil && sig[1] == funName {
				if posture, pageArgs := gqlkRelayPosture(sig[2], sig[3]); posture != "" {
					setProps(&ent, "graphql_pagination", posture)
					if pageArgs != "" {
						setProps(&ent, "graphql_pagination_args", pageArgs)
					}
				}
			}
			add(ent)
		}
	}

	// 2. DTOs: `data class` types and classes carrying @GraphQLName become
	//    SCOPE.Schema entities. Operation roots (handled above) are not DTOs.
	for _, m := range reGQLKDataClass.FindAllStringSubmatchIndex(src, -1) {
		typeName := src[m[2]:m[3]]
		annBlock := gqlkAnnotationBlock(src, m[0])
		if strings.Contains(annBlock, "@GraphQLIgnore") {
			continue
		}
		schemaName := typeName
		if nm := reGQLKName.FindStringSubmatch(annBlock); nm != nil {
			schemaName = nm[1]
		}
		// Relay/connection pagination shape (#5010): a `data class` named
		// *Connection / *Edge / *PageInfo is the GraphQL cursor-pagination
		// wire model. Tag its dto role with the connection part so the schema's
		// pagination posture is queryable from the type side as well as the
		// field side.
		dtoRole := "object"
		paginationRole := ""
		switch {
		case strings.HasSuffix(typeName, "Connection"):
			dtoRole, paginationRole = "connection", "connection"
		case strings.HasSuffix(typeName, "Edge"):
			dtoRole, paginationRole = "edge", "edge"
		case typeName == "PageInfo" || strings.HasSuffix(typeName, "PageInfo"):
			dtoRole, paginationRole = "page_info", "page_info"
		}

		ent := makeEntity("graphql_dto:"+schemaName, "SCOPE.Schema", "dto", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "graphql-kotlin",
			"provenance", "INFERRED_FROM_GRAPHQL_KOTLIN_DTO",
			"dto_name", schemaName, "dto_source_class", typeName,
			"graphql_dto_role", dtoRole)
		if paginationRole != "" {
			setProps(&ent, "graphql_pagination", "relay_connection",
				"graphql_pagination_role", paginationRole)
		}
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
