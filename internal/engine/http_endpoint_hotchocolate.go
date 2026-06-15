// http_endpoint_hotchocolate.go — HotChocolate (C#/.NET) GraphQL server
// → http_endpoint_definition synthesis.
//
// HotChocolate is the dominant .NET GraphQL framework. The canonical,
// code-first style declares the three GraphQL root types as plain C#
// classes whose public methods become root fields:
//
//	[QueryType]
//	public class Query
//	{
//	    public User GetUser(int id) => ...;   // → field `user`
//	    public IEnumerable<User> GetUsers() => ...;  // → field `users`
//	}
//
//	[MutationType]
//	public class Mutation
//	{
//	    public User CreateUser(string name) => ...;  // → field `createUser`
//	}
//
// The root classes are recognised three ways:
//
//   - the `[QueryType]` / `[MutationType]` / `[SubscriptionType]` marker
//     attributes (HotChocolate ≥ v13 annotation-based registration);
//   - `[ExtendObjectType(typeof(Query))]` / `[ExtendObjectType("Query")]`
//     type extensions that contribute additional root fields;
//   - classes registered fluently in Program.cs / Startup via
//     `.AddQueryType<Query>()` / `.AddMutationType<Mutation>()` /
//     `.AddSubscriptionType<Subscription>()`.
//
// Each public resolver method is mapped to the SAME canonical endpoint shape
// the JS (synthesizeGraphQLResolvers), Python (synthesizeStrawberry), Go
// (gqlgen) and Elixir (synthesizeAbsinthe) GraphQL servers emit:
//
//	http:GRAPHQL:/graphql/<Query|Mutation|Subscription>/<field>
//
// The field name follows HotChocolate's default naming convention: the
// leading `Get` prefix is stripped and the first letter is lower-cased
// (`GetUser` → `user`, `GetUsersByTeam` → `usersByTeam`, `CreateUser`
// → `createUser`). This matches the cross-stack endpoint shape so the
// GraphQL client links (#3667) + cross-repo linker join to these endpoints.
//
// Handler attribution mirrors synthesizeASPNetCore (#2692): each endpoint
// carries `source_handler=SCOPE.Operation:<Class>.<Method>`, which the C#
// extractor names identically (buildOperation in
// internal/extractors/csharp/csharp.go), so ResolveHTTPEndpointHandlers
// rebinds source_file / start_line to the resolver method — producing the
// HANDLES edge endpoint → resolver method.
//
// Detection is gated on a HotChocolate file-signal (`HotChocolate` import or
// `.AddGraphQLServer()`) so the synthesizer is a no-op on plain ASP.NET Core
// / gRPC / Blazor C# files. The root-type registration is `honest-partial`
// where it is indirect (fluent `.AddQueryType<T>()` in a *different* file
// than the resolver class): in that case the marker-attribute path or the
// same-file `.AddQueryType<T>()` recovers the class→root mapping; a resolver
// class with neither marker nor same-file registration is not emitted.
//
// Refs #3617 (epic #3607).
package engine

import (
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/engine/httproutes"
)

// hotChocolateHasSignal reports whether a C# file shows any sign of a
// HotChocolate GraphQL server. Used as a fast pre-filter so the regex
// machinery doesn't run on every C# file in the index.
func hotChocolateHasSignal(content string) bool {
	return strings.Contains(content, "HotChocolate") ||
		strings.Contains(content, ".AddGraphQLServer()") ||
		strings.Contains(content, ".AddGraphQLServer(")
}

// hcRootMarker is the canonical GraphQL root-type name a registration maps to.
type hcRootMarker = string

const (
	hcQuery        hcRootMarker = "Query"
	hcMutation     hcRootMarker = "Mutation"
	hcSubscription hcRootMarker = "Subscription"
)

// hcMarkerAttrRe captures a HotChocolate root-type marker attribute
// (`[QueryType]` / `[MutationType]` / `[SubscriptionType]`, optionally
// fully-qualified or with parentheses) immediately preceding a class
// declaration (allowing intervening attributes between the marker and the
// `class` line).
//
// Capture groups:
//
//	1 = root kind (Query | Mutation | Subscription)
//	2 = class name
var hcMarkerAttrRe = regexp.MustCompile(
	`\[\s*(Query|Mutation|Subscription)Type\s*(?:\([^)]*\))?\s*\]` +
		`\s*[\r\n]+(?:\s*\[[^\]\r\n]+\]\s*[\r\n]+)*` +
		`\s*(?:public|internal|sealed|abstract|partial|static|\s)*` +
		`class\s+([A-Za-z_]\w*)`,
)

// hcExtendObjectRe captures an `[ExtendObjectType(typeof(Query))]` or
// `[ExtendObjectType("Query")]` (or `OperationType.Query`) type-extension
// attribute and the class it decorates. The extension contributes its
// public methods as additional fields on the named root type.
//
// Capture groups:
//
//	1 = root type referenced (Query | Mutation | Subscription)
//	2 = class name
var hcExtendObjectRe = regexp.MustCompile(
	`\[\s*ExtendObjectType\s*\(\s*` +
		`(?:typeof\s*\(\s*(Query|Mutation|Subscription)\s*\)` +
		`|"(?:Query|Mutation|Subscription)"` +
		`|OperationType\.(Query|Mutation|Subscription))` +
		`[^)]*\)\s*\]` +
		`\s*[\r\n]+(?:\s*\[[^\]\r\n]+\]\s*[\r\n]+)*` +
		`\s*(?:public|internal|sealed|abstract|partial|static|\s)*` +
		`class\s+([A-Za-z_]\w*)`,
)

// hcFluentRegRe captures a fluent root-type registration in Program.cs /
// Startup: `.AddQueryType<Query>()` / `.AddMutationType<Mutation>()` /
// `.AddSubscriptionType<Subscription>()`. The type argument is the resolver
// class name; when that class lives in the same file its methods are mapped.
//
// Capture groups:
//
//	1 = root kind (Query | Mutation | Subscription)
//	2 = class name (the generic type argument)
var hcFluentRegRe = regexp.MustCompile(
	`\.Add(Query|Mutation|Subscription)Type\s*<\s*([A-Za-z_][\w.]*)\s*>\s*\(`,
)

// hcResolverMethodRe matches a public resolver method inside a class body.
// HotChocolate resolver methods must be `public` (non-public methods are not
// exposed as fields). The return-type chunk `[\w<>\[\],.\s?]+?` matches
// `User`, `Task<User>`, `IEnumerable<User>`, `IQueryable<User?>`, etc.
//
// Capture group 1 = method name.
var hcResolverMethodRe = regexp.MustCompile(
	`(?m)^\s*(?:\[[^\]\r\n]+\]\s*)*\s*public\s+` +
		`(?:static\s+|virtual\s+|override\s+|async\s+|sealed\s+)*` +
		`[\w<>\[\],.\s?]+?\s+([A-Za-z_]\w*)\s*\(`,
)

// hcFieldName converts a HotChocolate resolver method name to its default
// GraphQL field name: strip a leading `Get` prefix (only when followed by an
// upper-case letter, so `Get`/`Gettable` are not mangled) and lower-case the
// first letter.
//
//	GetUser        -> user
//	GetUsersByTeam -> usersByTeam
//	CreateUser     -> createUser
//	User           -> user
//	Get            -> get        (no upper-case follow → not a prefix)
func hcFieldName(method string) string {
	name := method
	if strings.HasPrefix(name, "Get") && len(name) > 3 {
		r := name[3]
		if r >= 'A' && r <= 'Z' {
			name = name[3:]
		}
	}
	if name == "" {
		return name
	}
	return strings.ToLower(name[:1]) + name[1:]
}

// hcClassBody returns the brace-delimited body of the class whose declaration
// ends at or after byte offset `declEnd`. It scans forward for the first `{`
// and returns the substring up to its matching `}` (exclusive of the braces).
// Returns "" when no balanced body is found.
func hcClassBody(content string, declEnd int) string {
	open := strings.IndexByte(content[declEnd:], '{')
	if open < 0 {
		return ""
	}
	open += declEnd
	close := findMatchingBrace(content, open)
	if close < 0 {
		return ""
	}
	return content[open+1 : close]
}

// hcEmitClassFields emits one GRAPHQL endpoint per public resolver method in
// the given class body, attributing each to root `rootKind`. Already-emitted
// (root, field) pairs are de-duplicated via `seen`.
func hcEmitClassFields(content string, declEnd int, className, rootKind string, seen map[string]bool, emit emitFn) {
	body := hcClassBody(content, declEnd)
	if body == "" {
		return
	}
	for _, mm := range hcResolverMethodRe.FindAllStringSubmatch(body, -1) {
		method := mm[1]
		// Skip constructors (method name == class name) and lifecycle/ctor-ish.
		if method == className {
			continue
		}
		field := hcFieldName(method)
		key := rootKind + "/" + field
		if field == "" || seen[key] {
			continue
		}
		seen[key] = true
		path := "/graphql/" + rootKind + "/" + field
		// FrameworkFastAPI canonicalisation preserves the path verbatim
		// (collapsing only redundant slashes) — identical to the shape the
		// Strawberry / Absinthe / gqlgen / JS GraphQL servers emit.
		canonical := httproutes.Canonicalize(httproutes.FrameworkFastAPI, path)
		// SCOPE.Operation:<Class>.<Method> matches the C# extractor's naming
		// (buildOperation), so the resolver rebinds to the resolver method
		// (HANDLES edge endpoint → method).
		emit("GRAPHQL", canonical, "hotchocolate", "SCOPE.Operation", className+"."+method)
	}
}

// synthesizeHotChocolate scans a C# source file for HotChocolate GraphQL
// server root types and emits one `http:GRAPHQL:/graphql/<Root>/<field>`
// synthetic per public resolver method, with a HANDLES edge to the resolver
// method. Gated on a HotChocolate file-signal so it no-ops on other C# files.
func synthesizeHotChocolate(content string, emit emitFn) {
	if !hotChocolateHasSignal(content) {
		return
	}

	// (className → rootKind) for classes registered fluently in this file via
	// `.AddQueryType<Query>()` etc. The bare generic type argument may be
	// namespace-qualified (`Types.Query`); key on the final segment.
	fluentRoots := map[string]string{}
	for _, fm := range hcFluentRegRe.FindAllStringSubmatch(content, -1) {
		rootKind := fm[1]
		arg := fm[2]
		if i := strings.LastIndexByte(arg, '.'); i >= 0 {
			arg = arg[i+1:]
		}
		if arg != "" {
			fluentRoots[arg] = rootKind
		}
	}

	// De-dup of emitted (root, field) pairs across all recognition paths so a
	// class that is BOTH marker-annotated and fluent-registered, or a field
	// that recurs across a base + extension, is emitted once.
	seen := map[string]bool{}

	// 1) Marker-attribute classes: [QueryType] / [MutationType] / [SubscriptionType].
	for _, m := range hcMarkerAttrRe.FindAllStringSubmatchIndex(content, -1) {
		rootKind := content[m[2]:m[3]]
		className := content[m[4]:m[5]]
		hcEmitClassFields(content, m[1], className, rootKind, seen, emit)
	}

	// 2) [ExtendObjectType(typeof(Query))] type-extension classes.
	for _, m := range hcExtendObjectRe.FindAllStringSubmatchIndex(content, -1) {
		// Group 1 (typeof) OR group 2 (OperationType.) carries the root kind;
		// the quoted-string alternative has no capture, so recover from text.
		rootKind := ""
		if m[2] >= 0 {
			rootKind = content[m[2]:m[3]]
		} else if m[4] >= 0 {
			rootKind = content[m[4]:m[5]]
		} else {
			// "Query" | "Mutation" | "Subscription" quoted-string form.
			seg := content[m[0]:m[1]]
			switch {
			case strings.Contains(seg, `"Query"`):
				rootKind = hcQuery
			case strings.Contains(seg, `"Mutation"`):
				rootKind = hcMutation
			case strings.Contains(seg, `"Subscription"`):
				rootKind = hcSubscription
			}
		}
		if rootKind == "" {
			continue
		}
		className := content[m[6]:m[7]]
		hcEmitClassFields(content, m[1], className, rootKind, seen, emit)
	}

	// 3) Fluent-registered classes declared in THIS file. The class declaration
	// itself carries no marker, so find each `class <Name>` whose name appears
	// in fluentRoots and map its methods to the registered root kind.
	if len(fluentRoots) > 0 {
		for _, cm := range hcPlainClassRe.FindAllStringSubmatchIndex(content, -1) {
			className := content[cm[2]:cm[3]]
			rootKind, ok := fluentRoots[className]
			if !ok {
				continue
			}
			hcEmitClassFields(content, cm[1], className, rootKind, seen, emit)
		}
	}
}

// hcPlainClassRe matches any class declaration (no marker required). Used to
// locate fluent-registered resolver classes by name.
//
// Capture group 1 = class name.
var hcPlainClassRe = regexp.MustCompile(
	`(?m)^\s*(?:public|internal|sealed|abstract|partial|static|\s)*` +
		`class\s+([A-Za-z_]\w*)`,
)
