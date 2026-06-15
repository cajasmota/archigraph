// Package java — regex-based GraphQL-server extractor for the two dominant
// JVM GraphQL frameworks: Spring for GraphQL and Netflix DGS.
//
// Both frameworks are *annotation-driven* and code-first: a controller method
// becomes a root field of the Query / Mutation / Subscription operation when it
// carries the framework's mapping annotation. This extractor recognises those
// annotations and emits, for each resolver method, ONE synthetic GraphQL
// endpoint in the canonical grafel shape
//
//	SCOPE.Operation  name "GRAPHQL /graphql/<Operation>/<field>"
//	                 route_path "/graphql/<Operation>/<field>"  verb GRAPHQL
//
// — IDENTICAL to the gqlgen (Go), HotChocolate (C#), graphql-kotlin and the
// JS/TS Apollo resolver synthesis. Matching this shape is what lets the GraphQL
// client-link pass (#3667) and the cross-repo HTTP linker join a client
// operation `query { users }` to its JVM resolver. A HANDLES edge is emitted
// from the endpoint to the resolver method so handler attribution is recorded.
//
// Spring for GraphQL (org.springframework.graphql)
// -------------------------------------------------
//
//	@Controller
//	class UserController {
//	    @QueryMapping        public List<User> users() { ... }   // Query.users
//	    @MutationMapping     public User createUser(...) { ... }  // Mutation.createUser
//	    @SubscriptionMapping public Flux<Event> events() { ... }  // Subscription.events
//	    @QueryMapping(name = "allUsers") public List<User> users2() {...} // Query.allUsers
//	    @SchemaMapping(typeName = "Query", field = "node") public Node n() {...}
//	}
//
// The root field defaults to the METHOD name (lowerCamel, which Java methods
// already are). `@QueryMapping(name="x")` / `@SchemaMapping(field="x")` override
// it. `@SchemaMapping(typeName="Query")` selects the operation explicitly; a
// bare `@SchemaMapping` on a method whose typeName is a SDL type (not a root)
// is a per-type field resolver and is NOT a root operation — we only emit a
// root endpoint when typeName resolves to Query/Mutation/Subscription.
//
// Netflix DGS (com.netflix.graphql.dgs)
// -------------------------------------
//
//	@DgsComponent
//	class UserFetcher {
//	    @DgsQuery        public User user(...) { ... }            // Query.user
//	    @DgsMutation     public User addUser(...) { ... }         // Mutation.addUser
//	    @DgsSubscription public Publisher<E> events() { ... }     // Subscription.events
//	    @DgsQuery(field = "allUsers") public List<User> users() {...} // Query.allUsers
//	    @DgsData(parentType = "Query", field = "search") public R s() {...}
//	}
//
// `@DgsData(parentType="Query", field="x")` is the general form; the
// `@DgsQuery/@DgsMutation/@DgsSubscription` shorthands fix parentType and
// default field to the method name. As with Spring, a `@DgsData` whose
// parentType is a non-root SDL type is a field resolver, not a root operation,
// and is skipped for endpoint synthesis.
//
// HONEST LIMIT (schema / route = partial). Root-operation discovery is
// annotation-driven and file-local: a method is a root field iff its mapping
// annotation is present in the same file. Field resolvers on SDL object types
// (`@SchemaMapping(typeName="User", field="orders")`,
// `@DgsData(parentType="User", ...)`) are intentionally NOT emitted as
// /graphql/<root>/<field> endpoints — they resolve nested fields, not root
// operations. The transport mount path is assumed to be the conventional
// `/graphql`; a custom `spring.graphql.path` / DGS servlet mapping is not read.
//
// Frameworks: lang.java.framework.spring-graphql, lang.java.framework.dgs
// Issue #3615 (epic #3607) — Java GraphQL server extraction.
package java

import (
	"regexp"
	"strings"
)

// springGraphQLFrameworks gates ExtractSpringGraphQL. Both Spring-for-GraphQL
// and DGS run on java sources; the dispatch hands us the candidate framework
// token (see patterns_dispatch.go frameworkMarkers).
var springGraphQLFrameworks = map[string]bool{
	"spring_graphql": true, "spring-graphql": true, "springgraphql": true,
	"dgs": true, "netflix_dgs": true, "netflix-dgs": true,
	// The Spring/DGS annotations frequently coexist with a plain spring_boot
	// candidate token; accept it so a @Controller + @QueryMapping file is not
	// missed when only the generic spring marker fired.
	"spring_boot": true,
}

// annArgsRe matches an annotation argument list `(...)` that may contain ONE
// level of nested parens — e.g. `@PreAuthorize("hasRole('ADMIN')")` or
// `@PreAuthorize("hasRole('A') and hasAuthority('B')")`. The previous
// `\([^)]*\)` form stopped at the first inner `)` and so silently dropped the
// WHOLE resolver endpoint whenever a SpEL `@PreAuthorize` was interleaved
// between the mapping annotation and the method (#3862 only tested @Secured /
// pre-method @PreAuthorize, which avoided the nested-paren case). Used by every
// Spring/DGS mapping regex below so endpoint synthesis survives a SpEL guard.
const annArgsRe = `\((?:[^()]|\([^()]*\))*\)`

var (
	// Spring for GraphQL mapping annotations whose operation is fixed by the
	// annotation name. Capture group 1 = annotation simple name, group 2 = the
	// optional argument list (without parens), group 3..5 = the method
	// signature (return type / name / params) following the annotation.
	//
	// Matches e.g.:
	//   @QueryMapping public List<User> users() {
	//   @MutationMapping(name = "createUser") User create(@Argument In in) {
	//   @SubscriptionMapping Flux<Event> events() {
	reSpringGQLMapping = regexp.MustCompile(
		`(?s)@(QueryMapping|MutationMapping|SubscriptionMapping)\b\s*(` + annArgsRe + `)?\s*` +
			`(?:@\w+(?:` + annArgsRe + `)?\s*)*` +
			`(?:(?:public|protected|private)\s+)?(?:static\s+)?(?:final\s+)?` +
			`(?:<[^>]*>\s*)?[\w.$<>\[\], ?]+?\s+(\w+)\s*\(`,
	)
	// Spring @SchemaMapping(typeName="Query", field="x") — explicit form whose
	// operation comes from typeName and field from field. Group 1 = arg list,
	// group 2 = method name (used as the field fallback when field= absent).
	reSpringSchemaMapping = regexp.MustCompile(
		`(?s)@SchemaMapping\b\s*(` + annArgsRe + `)?\s*` +
			`(?:@\w+(?:` + annArgsRe + `)?\s*)*` +
			`(?:(?:public|protected|private)\s+)?(?:static\s+)?(?:final\s+)?` +
			`(?:<[^>]*>\s*)?[\w.$<>\[\], ?]+?\s+(\w+)\s*\(`,
	)
	// Netflix DGS shorthand annotations whose operation is fixed by name.
	// The (?:@\w+...)* segment tolerates a security annotation
	// (@Secured / @PreAuthorize / @RolesAllowed) interleaved between the DGS
	// mapping annotation and the resolver method (#3862).
	reDgsShorthand = regexp.MustCompile(
		`(?s)@(DgsQuery|DgsMutation|DgsSubscription)\b\s*(` + annArgsRe + `)?\s*` +
			`(?:@\w+(?:` + annArgsRe + `)?\s*)*` +
			`(?:(?:public|protected|private)\s+)?(?:static\s+)?(?:final\s+)?` +
			`(?:<[^>]*>\s*)?[\w.$<>\[\], ?]+?\s+(\w+)\s*\(`,
	)
	// Netflix DGS general @DgsData(parentType="Query", field="x").
	reDgsData = regexp.MustCompile(
		`(?s)@DgsData\b\s*(` + annArgsRe + `)?\s*` +
			`(?:@\w+(?:` + annArgsRe + `)?\s*)*` +
			`(?:(?:public|protected|private)\s+)?(?:static\s+)?(?:final\s+)?` +
			`(?:<[^>]*>\s*)?[\w.$<>\[\], ?]+?\s+(\w+)\s*\(`,
	)

	// Annotation-argument extractors for name / field / typeName / parentType.
	reGQLArgName       = regexp.MustCompile(`\bname\s*=\s*"([^"]*)"`)
	reGQLArgField      = regexp.MustCompile(`\bfield\s*=\s*"([^"]*)"`)
	reGQLArgTypeName   = regexp.MustCompile(`\btypeName\s*=\s*"([^"]*)"`)
	reGQLArgParentType = regexp.MustCompile(`\bparentType\s*=\s*"([^"]*)"`)
)

// Request/response shape extraction (#3873). A GraphQL resolver's typed input
// args and return type are its request/response shapes — the GraphQL analogue
// of Spring MVC's @RequestBody / return type that spring_request_response.go
// already credits. We emit:
//
//   - ACCEPTS_INPUT  endpoint → DTO schema, for each @Argument / @InputArgument
//     parameter whose type is a non-scalar input object (an input DTO). Scalar
//     args (Long id, String q) are NOT shape DTOs — they are skipped, matching
//     the Spring MVC sniffer's srrSkipTypes filter.
//   - RETURNS         endpoint → DTO schema, for the unwrapped resolver return
//     type (List<User>/Mono<User>/Flux<Event> → User/Event), reusing the exact
//     unwrapReturnType helper Spring MVC uses.
//
// HONEST LIMIT (partial): shapes resolve from the resolver method SIGNATURE
// (DTO-typed @Argument params + return type). Inline field-selection sets are
// not recovered, and a DTO declared in another file contributes only its type
// name (no member fields) — identical to the Spring MVC single-file limit.
var (
	// gqlMethodSigRe captures a resolver method's return type (group 1), name
	// (group 2) and parameter block (group 3) starting at the mapping
	// annotation. The leading annotation run (mapping + any interleaved
	// security annotation) is skipped via annArgsRe-aware tolerance.
	gqlMethodSigRe = regexp.MustCompile(
		`(?s)@(?:Query|Mutation|Subscription)Mapping\b\s*(?:` + annArgsRe + `)?\s*` +
			`(?:@\w+(?:` + annArgsRe + `)?\s*)*` +
			`(?:(?:public|protected|private)\s+)?(?:static\s+)?(?:final\s+)?` +
			`((?:<[^>]*>\s*)?[\w.$<>\[\], ?]+?)\s+(\w+)\s*\(([^)]*)\)`)
	gqlDgsMethodSigRe = regexp.MustCompile(
		`(?s)@Dgs(?:Query|Mutation|Subscription|Data)\b\s*(?:` + annArgsRe + `)?\s*` +
			`(?:@\w+(?:` + annArgsRe + `)?\s*)*` +
			`(?:(?:public|protected|private)\s+)?(?:static\s+)?(?:final\s+)?` +
			`((?:<[^>]*>\s*)?[\w.$<>\[\], ?]+?)\s+(\w+)\s*\(([^)]*)\)`)
	gqlSchemaMethodSigRe = regexp.MustCompile(
		`(?s)@SchemaMapping\b\s*(?:` + annArgsRe + `)?\s*` +
			`(?:@\w+(?:` + annArgsRe + `)?\s*)*` +
			`(?:(?:public|protected|private)\s+)?(?:static\s+)?(?:final\s+)?` +
			`((?:<[^>]*>\s*)?[\w.$<>\[\], ?]+?)\s+(\w+)\s*\(([^)]*)\)`)
	// gqlArgParamRe matches a single `@Argument [Type] name` or
	// `@InputArgument [Type] name` resolver parameter. Group 1 = the type name
	// (base, possibly generic). Inner annotations (@Valid, @Argument(name=...))
	// are tolerated before the type.
	gqlArgParamRe = regexp.MustCompile(
		`@(?:Argument|InputArgument)\b(?:\s*\([^)]*\))?\s*(?:@\w+(?:\([^)]*\))?\s*)*` +
			`([A-Za-z_$][\w$.]*(?:\s*<[^>]*>)?)\s+[A-Za-z_$][\w$]*`)
)

// gqlShapeBaseType strips a generic wrapper (List<X>, Optional<X>, Mono<X>,
// Flux<X>, Set<X>) down to its element base type, returning the base type name
// (or "" if the type bottoms out at a scalar/skip type). Reuses srrSkipTypes
// (the Spring MVC scalar/container skip set) for parity.
func gqlShapeBaseType(raw string) string {
	raw = strings.TrimSpace(raw)
	// Peel one or two generic layers (List<Mono<User>> is pathological but
	// unwrapReturnType handles return types; for args one peel suffices).
	for range 2 {
		if m := srrBaseGenericRE.FindStringSubmatch(raw); m != nil {
			base, inner := m[1], m[2]
			if srrSkipTypes[base] && inner != "" {
				raw = strings.TrimSpace(inner)
				continue
			}
			if srrSkipTypes[base] {
				return ""
			}
			return base
		}
		break
	}
	return ""
}

// springGQLOperationForMapping maps a Spring/DGS shorthand annotation name to
// its GraphQL root operation. Returns "" for an unrecognised name.
func springGQLOperationForMapping(annName string) string {
	switch annName {
	case "QueryMapping", "DgsQuery":
		return "Query"
	case "MutationMapping", "DgsMutation":
		return "Mutation"
	case "SubscriptionMapping", "DgsSubscription":
		return "Subscription"
	}
	return ""
}

// normalizeRootType returns the canonical Query/Mutation/Subscription root name
// for an explicit typeName/parentType argument, or "" when the value is a
// non-root SDL object type (a field-resolver, not a root operation).
func normalizeRootType(v string) string {
	switch v {
	case "Query", "Mutation", "Subscription":
		return v
	}
	return ""
}

// firstArg returns the first submatch of re in args, or "".
func firstArg(re *regexp.Regexp, args string) string {
	if m := re.FindStringSubmatch(args); m != nil {
		return m[1]
	}
	return ""
}

// Spring Security annotation regexes used to gate GraphQL resolver methods.
var (
	gqlSecuredRE      = regexp.MustCompile(`@Secured\s*\(\s*([^)]*)\)`)
	gqlPreAuthorizeRE = regexp.MustCompile(`@PreAuthorize\s*\(\s*"([^"]*)"\s*\)`)
	gqlRolesAllowedRE = regexp.MustCompile(`@RolesAllowed\s*\(\s*([^)]*)\)`)
	gqlPermitAllRE    = regexp.MustCompile(`@PermitAll\b`)
	gqlQuotedRE       = regexp.MustCompile(`"([^"]+)"`)
	gqlSpELRoleRE     = regexp.MustCompile(`(?:hasRole|hasAnyRole)\s*\(\s*([^)]+)\)`)
	gqlSpELAuthRE     = regexp.MustCompile(`(?:hasAuthority|hasAnyAuthority)\s*\(\s*([^)]+)\)`)
	gqlSpELQuotedRE   = regexp.MustCompile(`['"]([^'"]+)['"]`)
)

// gqlMethodAuth resolves the auth posture of a GraphQL resolver method from the
// Spring Security annotations in the annotation block that decorates it.
// `mapOffset` is the start offset of the resolver's mapping annotation
// (@DgsQuery / @QueryMapping / ...); security annotations sit adjacent to it, so
// we scan a window from a few lines before the mapping annotation up to the
// method's opening paren. Returns the zero authStamp (method == "") when no
// security annotation is present — an unauthenticated resolver stamps nothing.
//
// Mirrors the Spring MVC contract: @Secured/@PreAuthorize roles strip the
// ROLE_ prefix; @PreAuthorize hasAuthority splits into scopes/permissions;
// @PermitAll marks the operation explicitly public.
func gqlMethodAuth(src string, mapOffset int, _ string) authStamp {
	// The auth annotations for this resolver live in the contiguous annotation
	// block that decorates the method: a run of `@Annotation(...)` tokens
	// (possibly multi-line) ending at the method declaration. We collect that
	// block starting from the mapping annotation (mapOffset) and walking forward
	// over annotation tokens and modifiers until the method's return type +
	// name + '(' — stopping BEFORE the next method so we never bleed across
	// resolvers.
	window := gqlMethodAnnotationBlock(src, mapOffset)

	if gqlPermitAllRE.MatchString(window) {
		return authStamp{required: false, method: "annotation", confidence: "high", guard: "PermitAll"}
	}
	var roles, scopes, perms []string
	guard := ""
	// @Secured("ROLE_ADMIN")
	if m := gqlSecuredRE.FindStringSubmatch(window); m != nil {
		guard = "Secured"
		for _, q := range gqlQuotedRE.FindAllStringSubmatch(m[1], -1) {
			classifyAuthority(q[1], &roles, &scopes, &perms)
		}
	}
	// @RolesAllowed({"ADMIN"})
	if m := gqlRolesAllowedRE.FindStringSubmatch(window); m != nil {
		if guard == "" {
			guard = "RolesAllowed"
		}
		for _, q := range gqlQuotedRE.FindAllStringSubmatch(m[1], -1) {
			roles = append(roles, q[1])
		}
	}
	// @PreAuthorize("hasRole('ADMIN') and hasAuthority('SCOPE_read')")
	if m := gqlPreAuthorizeRE.FindStringSubmatch(window); m != nil {
		if guard == "" {
			guard = "PreAuthorize"
		}
		expr := m[1]
		for _, rm := range gqlSpELRoleRE.FindAllStringSubmatch(expr, -1) {
			for _, q := range gqlSpELQuotedRE.FindAllStringSubmatch(rm[1], -1) {
				roles = append(roles, strings.TrimPrefix(q[1], "ROLE_"))
			}
		}
		for _, am := range gqlSpELAuthRE.FindAllStringSubmatch(expr, -1) {
			for _, q := range gqlSpELQuotedRE.FindAllStringSubmatch(am[1], -1) {
				classifyAuthority(q[1], &roles, &scopes, &perms)
			}
		}
	}
	if guard == "" {
		return authStamp{}
	}
	return authStamp{
		required: true, method: "annotation", confidence: "high",
		guard: guard, roles: roles, scopes: scopes, permissions: perms,
	}
}

// gqlMethodAnnotationBlock returns the source span of the annotation block that
// decorates the resolver method whose mapping annotation begins at mapOffset.
// It walks BACK from mapOffset over any preceding annotation lines (so a
// @PreAuthorize placed above the @DgsQuery is included) until a statement
// boundary ('}' / ';' closing the previous member), and FORWARD from mapOffset
// to the method's own parameter-list '(' (skipping annotation argument parens),
// so a @Secured placed between @DgsQuery and the method is included — without
// ever spilling into the next or previous resolver.
func gqlMethodAnnotationBlock(src string, mapOffset int) string {
	// Backward: stop at the nearest preceding '}' or ';' (end of the prior
	// member). Everything after it up to mapOffset is leading annotations.
	start := mapOffset
	for i := mapOffset - 1; i >= 0; i-- {
		if src[i] == '}' || src[i] == ';' || src[i] == '{' {
			start = i + 1
			break
		}
		if i == 0 {
			start = 0
		}
	}
	// Forward: from mapOffset, find the method param-list '(' at paren depth 0,
	// skipping balanced annotation argument lists.
	end := mapOffset
	for end < len(src) {
		c := src[end]
		if c == '(' {
			// Is this paren an annotation argument list? It is when the token
			// chain immediately before it traces back to an '@name'. We detect a
			// simpler sufficient condition: the identifier before '(' is preceded
			// (ignoring whitespace/identifier chars) by '@'. If so, skip the
			// balanced parens; otherwise this is the method param list.
			if gqlIsAnnotationParen(src, start, end) {
				end = gqlSkipBalancedParens(src, end)
				continue
			}
			break
		}
		if c == '{' || c == ';' || c == '}' {
			break
		}
		end++
	}
	if end > len(src) {
		end = len(src)
	}
	if start > end {
		start = end
	}
	return src[start:end]
}

// gqlIsAnnotationParen reports whether the '(' at parenPos opens an annotation
// argument list (rather than a method parameter list). It walks back over the
// identifier preceding the paren; if that identifier is immediately preceded by
// '@' it is an annotation invocation.
func gqlIsAnnotationParen(src string, lo, parenPos int) bool {
	i := parenPos - 1
	for i >= lo && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i--
	}
	// Skip the identifier characters.
	end := i
	for i >= lo && (isWordByte(src[i])) {
		i--
	}
	if end == i { // no identifier before '('
		return false
	}
	// Skip whitespace between '@' and the annotation name.
	for i >= lo && (src[i] == ' ' || src[i] == '\t') {
		i--
	}
	return i >= lo && src[i] == '@'
}

// gqlSkipBalancedParens returns the offset just past the matching ')' for the
// '(' at open. Falls back to open+1 on imbalance.
func gqlSkipBalancedParens(src string, open int) int {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return open + 1
}

// isWordByte reports whether b is a Java identifier byte.
func isWordByte(b byte) bool {
	return b == '_' || b == '$' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// ExtractSpringGraphQL emits canonical GraphQL operation endpoints for Spring
// for GraphQL and Netflix DGS resolver methods, plus a HANDLES edge from each
// endpoint to its resolver method.
func ExtractSpringGraphQL(ctx PatternContext) PatternResult {
	var result PatternResult
	if ctx.Language != "java" || !springGraphQLFrameworks[ctx.Framework] {
		return result
	}

	src := ctx.Source
	fp := ctx.FilePath

	// File-signal gate: require at least one Spring/DGS GraphQL mapping
	// annotation so this is a no-op on plain Spring MVC / JPA files.
	if !strings.Contains(src, "QueryMapping") &&
		!strings.Contains(src, "MutationMapping") &&
		!strings.Contains(src, "SubscriptionMapping") &&
		!strings.Contains(src, "SchemaMapping") &&
		!strings.Contains(src, "@Dgs") {
		return result
	}

	seenEnt := make(map[string]bool)
	seenRel := make(map[relKey]bool)

	// emit records one resolver method as a canonical GRAPHQL endpoint plus a
	// resolver-method entity and a HANDLES edge between them. `framework` is the
	// record-citing framework label ("spring-graphql" or "dgs").
	// ensureGQLDTO registers a request/response DTO schema entity (one per type
	// per file) so ACCEPTS_INPUT/RETURNS edges have a stable target, mirroring
	// the Spring MVC ensureDTO contract (scope:schema:spring_dto:...).
	ensureGQLDTO := func(dtoName string, lineNo int) string {
		ref := "scope:schema:spring_dto:" + fp + ":" + dtoName
		addEntity(&result, seenEnt, SecondaryEntity{
			Name: dtoName, Kind: "SCOPE.Schema", SourceFile: fp,
			LineStart: lineNo, LineEnd: lineNo,
			Provenance: "INFERRED_FROM_SPRING_GRAPHQL_SHAPE", Ref: ref,
			Properties: map[string]any{"kind": "dto", "framework": "graphql"},
		})
		return ref
	}

	emit := func(operation, field, methodName, framework, provenance string, offset int, returnType, paramsBlock string) {
		if operation == "" || field == "" {
			return
		}
		owner := findEnclosingClass(src, offset)
		handlerName := methodName
		if owner != "" {
			handlerName = owner + "." + methodName
		}
		lineNo := lineOf(src, offset)
		path := "/graphql/" + operation + "/" + field
		name := "GRAPHQL " + path

		endpointProps := map[string]any{
			"framework":         framework,
			"http_method":       "GRAPHQL",
			"verb":              "GRAPHQL",
			"route_path":        path,
			"path":              path,
			"graphql_operation": operation,
			"graphql_root":      owner,
			"graphql_field":     field,
			"resolver_method":   methodName,
			"handler_name":      handlerName,
		}
		// Auth (#3862, DGS/Spring-for-GraphQL): @Secured / @PreAuthorize /
		// @RolesAllowed on the resolver method (Spring Security under DGS) gate
		// the GraphQL operation endpoint. Resolve the method's annotation block
		// and stamp the flat auth contract on the endpoint, matching Spring MVC.
		gqlMethodAuth(src, offset, owner+"."+methodName).stamp(endpointProps)

		endpointRef := "scope:operation:" + framework + "_endpoint:" + fp + ":" + operation + "." + field
		addEntity(&result, seenEnt, SecondaryEntity{
			Name:       name,
			Kind:       "SCOPE.Operation",
			Subtype:    "endpoint",
			SourceFile: fp,
			LineStart:  lineNo,
			LineEnd:    lineNo,
			Provenance: provenance,
			Ref:        endpointRef,
			Properties: endpointProps,
		})

		// Resolver-method entity + HANDLES edge (endpoint → resolver).
		resolverRef := "scope:operation:" + framework + "_resolver:" + fp + ":" + handlerName
		addEntity(&result, seenEnt, SecondaryEntity{
			Name:       handlerName,
			Kind:       "SCOPE.Operation",
			Subtype:    "graphql_resolver",
			SourceFile: fp,
			LineStart:  lineNo,
			LineEnd:    lineNo,
			Provenance: provenance,
			Ref:        resolverRef,
			Properties: map[string]any{
				"framework":         framework,
				"graphql_operation": operation,
				"graphql_field":     field,
				"resolver_method":   methodName,
				"resolver_class":    owner,
			},
		})
		addRel(&result, seenRel, Relationship{
			SourceRef:        endpointRef,
			TargetRef:        resolverRef,
			RelationshipType: "HANDLES",
			Properties: map[string]string{
				"framework":     framework,
				"graphql_field": field,
				"graphql_root":  operation,
				"match_source":  "graphql_resolver_annotation",
			},
		})

		// Request shape (#3873): each DTO-typed @Argument/@InputArgument param
		// is an ACCEPTS_INPUT to its input DTO. Scalar args (Long/String/int)
		// are not shapes and are skipped via gqlShapeBaseType/srrSkipTypes.
		for _, pm := range gqlArgParamRe.FindAllStringSubmatch(paramsBlock, -1) {
			dto := gqlShapeBaseType(pm[1])
			if dto == "" {
				continue
			}
			dtoRef := ensureGQLDTO(dto, lineNo)
			addRel(&result, seenRel, Relationship{
				SourceRef:        endpointRef,
				TargetRef:        dtoRef,
				RelationshipType: "ACCEPTS_INPUT",
				Properties: map[string]string{
					"framework":    framework,
					"dto_type":     dto,
					"match_source": "graphql_argument",
				},
			})
		}

		// Response shape (#3873): the unwrapped resolver return type
		// (List<User>/Mono<User>/Flux<Event> → User/Event) is a RETURNS to its
		// output DTO. Reuses the exact Spring MVC unwrapReturnType helper.
		if dto := unwrapReturnType(strings.TrimSpace(returnType)); dto != "" {
			dtoRef := ensureGQLDTO(dto, lineNo)
			addRel(&result, seenRel, Relationship{
				SourceRef:        endpointRef,
				TargetRef:        dtoRef,
				RelationshipType: "RETURNS",
				Properties: map[string]string{
					"framework":       framework,
					"dto_type":        dto,
					"return_type_raw": strings.TrimSpace(returnType),
					"match_source":    "graphql_return_type",
				},
			})
		}
	}

	// sigAt extracts (returnType, paramsBlock) for the resolver whose mapping
	// annotation begins at off, applying sigRe to a window from off. Returns
	// ("","") when the signature does not re-match (shapes are then skipped).
	sigAt := func(sigRe *regexp.Regexp, off int) (string, string) {
		if sm := sigRe.FindStringSubmatch(src[off:]); sm != nil {
			return sm[1], sm[3]
		}
		return "", ""
	}

	// 1. Spring for GraphQL shorthand mappings (@QueryMapping etc.).
	for _, m := range reSpringGQLMapping.FindAllStringSubmatchIndex(src, -1) {
		annName := src[m[2]:m[3]]
		args := ""
		if m[4] >= 0 {
			args = src[m[4]:m[5]]
		}
		methodName := src[m[6]:m[7]]
		operation := springGQLOperationForMapping(annName)
		field := methodName
		if n := firstArg(reGQLArgName, args); n != "" {
			field = n
		}
		rt, pb := sigAt(gqlMethodSigRe, m[0])
		emit(operation, field, methodName, "spring-graphql",
			"INFERRED_FROM_SPRING_GRAPHQL_MAPPING", m[0], rt, pb)
	}

	// 2. Spring for GraphQL @SchemaMapping(typeName=..., field=...).
	for _, m := range reSpringSchemaMapping.FindAllStringSubmatchIndex(src, -1) {
		args := ""
		if m[2] >= 0 {
			args = src[m[2]:m[3]]
		}
		methodName := src[m[4]:m[5]]
		// typeName defaults to the field-resolver case; only a Query/Mutation/
		// Subscription typeName denotes a ROOT operation we synthesize.
		operation := normalizeRootType(firstArg(reGQLArgTypeName, args))
		if operation == "" {
			continue // per-type field resolver — not a root operation.
		}
		field := methodName
		if f := firstArg(reGQLArgField, args); f != "" {
			field = f
		}
		rt, pb := sigAt(gqlSchemaMethodSigRe, m[0])
		emit(operation, field, methodName, "spring-graphql",
			"INFERRED_FROM_SPRING_GRAPHQL_SCHEMA_MAPPING", m[0], rt, pb)
	}

	// 3. Netflix DGS shorthand (@DgsQuery / @DgsMutation / @DgsSubscription).
	for _, m := range reDgsShorthand.FindAllStringSubmatchIndex(src, -1) {
		annName := src[m[2]:m[3]]
		args := ""
		if m[4] >= 0 {
			args = src[m[4]:m[5]]
		}
		methodName := src[m[6]:m[7]]
		operation := springGQLOperationForMapping(annName)
		field := methodName
		if f := firstArg(reGQLArgField, args); f != "" {
			field = f
		}
		rt, pb := sigAt(gqlDgsMethodSigRe, m[0])
		emit(operation, field, methodName, "dgs",
			"INFERRED_FROM_DGS_MAPPING", m[0], rt, pb)
	}

	// 4. Netflix DGS general @DgsData(parentType=..., field=...).
	for _, m := range reDgsData.FindAllStringSubmatchIndex(src, -1) {
		args := ""
		if m[2] >= 0 {
			args = src[m[2]:m[3]]
		}
		methodName := src[m[4]:m[5]]
		operation := normalizeRootType(firstArg(reGQLArgParentType, args))
		if operation == "" {
			continue // field resolver on a non-root SDL type — skip.
		}
		field := methodName
		if f := firstArg(reGQLArgField, args); f != "" {
			field = f
		}
		rt, pb := sigAt(gqlDgsMethodSigRe, m[0])
		emit(operation, field, methodName, "dgs",
			"INFERRED_FROM_DGS_DATA", m[0], rt, pb)
	}

	return result
}
