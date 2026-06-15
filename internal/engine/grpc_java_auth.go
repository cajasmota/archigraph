// gRPC-Java server-auth detection (#4041, epic #3872).
//
// The Java auth sniffers (internal/engine/java_annotation_routes.go +
// java_auth_policy.go) are HTTP-route/annotation-keyed: they resolve auth on
// JAX-RS / Spring-MVC handlers attached to a URL route. A gRPC-Java service
// carries none of that — a gRPC server enforces auth one of two ways, neither
// of which is an HTTP route:
//
//  1. A transport-level io.grpc.ServerInterceptor wired into the server. The
//     interceptor reads the call Metadata (the request headers) and, on a
//     failed credential check, closes the call with a gRPC auth status —
//     Status.UNAUTHENTICATED / Status.PERMISSION_DENIED:
//
//     class AuthInterceptor implements ServerInterceptor {
//     public <Q,A> ServerCall.Listener<Q> interceptCall(
//     ServerCall<Q,A> call, Metadata headers, ServerCallHandler<Q,A> next) {
//     if (headers.get(AUTHZ) == null) {
//     call.close(Status.UNAUTHENTICATED.withDescription("no token"), new Metadata());
//     return new ServerCall.Listener<Q>() {};
//     }
//     return next.startCall(call, headers);
//     }
//     }
//
//     It is bound to a service via ServerInterceptors.intercept(service, ...)
//     or globally via ServerBuilder.intercept(...) /
//     .addService(ServerInterceptors.intercept(service, new AuthInterceptor())).
//
//  2. grpc-spring-boot-starter (net.devh): an @GrpcService class whose RPC
//     methods (or the class itself) carry a Spring-Security / Jakarta-Security
//     authorization annotation — @PreAuthorize("hasRole('ADMIN')"),
//     @Secured("ROLE_ADMIN"), @RolesAllowed({...}), @DenyAll, or the
//     grpc-spring-boot-starter @Authenticated marker. These annotations name
//     the exact method (and its required roles), so they bind precisely.
//
// Because applyGRPCEdges (grpc_edges.go) already emits one SCOPE.GrpcService
// per server impl and one SCOPE.GrpcMethod per handler method, this resolver
// re-walks the SAME file and returns the auth contract to stamp on the service
// and its methods. Same-file, signal-based, append-property-only — it never
// adds or removes entities.
//
// Output keys mirror the HTTP Java auth stamping (java_annotation_routes.go) so
// grafel_auth_coverage signal-1 + the security dashboard light up:
//
//	auth_required   — "true"
//	auth_method     — "grpc_interceptor" (path 1) | "annotation" (path 2)
//	auth_confidence — "high"
//	auth_roles      — CSV of required roles (path 2)
//	auth_permissions / auth_scopes — fine-grained grants (path 2)
//	auth_middleware — the interceptor symbol (MCP signal-1, path 1)
//	auth_policy     — JSON-encoded AuthPolicy (dashboard source chain)
//
// HONEST LIMITS (documented, not papered over):
//
//   - Cross-file interceptor binding. The ServerInterceptors.intercept /
//     ServerBuilder.intercept wiring must live in the SAME file as the service
//     impl (the same same-file boundary the rest of grpc_edges.go synthesis
//     lives in). The common layout — service impl in FooService.java, server
//     bootstrap in GrpcServer.java — is NOT stamped by the interceptor path. It
//     is stamped by the Spring-Security path when the methods carry annotations.
//   - Interceptor definition cross-file. An interceptor referenced by name but
//     defined in another file is credited only when it is bound by the
//     conventional (auth|authentication|jwt|security)…Interceptor naming
//     (MEDIUM confidence); an arbitrarily-named cross-file interceptor is not
//     chased.
package engine

import (
	"regexp"
	"sort"
	"strings"
)

// grpcJavaInterceptorAuthMethod is the auth_method stamped on a gRPC-Java
// service/method served by an auth-enforcing ServerInterceptor. Distinct from
// the Go "grpc_interceptor" value only in that it is the same string — the
// dashboard groups gRPC-interceptor auth across languages under one method.
const grpcJavaInterceptorAuthMethod = "grpc_interceptor"

// javaServerInterceptorDeclRe matches a class declaring it implements
// io.grpc.ServerInterceptor. Group 1 = the interceptor class name.
var javaServerInterceptorDeclRe = regexp.MustCompile(
	`(?m)class\s+(\w+)[^\{]*\bimplements\b[^\{]*\bServerInterceptor\b`)

// javaGrpcMetadataReadRe detects an interceptor reading the call Metadata —
// the canonical way a gRPC-Java interceptor obtains the request credentials.
// headers.get(...) / Metadata.Key / metadata.get(...) all qualify.
var javaGrpcMetadataReadRe = regexp.MustCompile(
	`\b(?:headers|metadata|md)\.get\s*\(|\bMetadata\.Key\b|AUTHORIZATION_KEY|AUTHORIZATION_METADATA_KEY`)

// javaGrpcAuthRejectRe detects a rejection with a gRPC authentication /
// authorization status — the decisive evidence the interceptor gates access.
// Status.UNAUTHENTICATED / Status.PERMISSION_DENIED, and their
// .asException()/.asRuntimeException() throw forms.
var javaGrpcAuthRejectRe = regexp.MustCompile(
	`\bStatus\.(?:UNAUTHENTICATED|PERMISSION_DENIED)\b`)

// javaServerInterceptorsInterceptRe matches a per-service interceptor binding
// `ServerInterceptors.intercept(<service>, <interceptors...>)`. Group 1 = the
// argument list (service ref + interceptor refs).
var javaServerInterceptorsInterceptRe = regexp.MustCompile(
	`\bServerInterceptors\s*\.\s*intercept\s*\(`)

// javaServerBuilderInterceptRe matches a global builder interceptor binding
// `.intercept(<interceptor>)` on a ServerBuilder chain.
var javaServerBuilderInterceptRe = regexp.MustCompile(
	`\.intercept\s*\(`)

// javaGrpcConventionalInterceptorRe matches a conventionally-named auth
// interceptor identifier (cross-file definitions credited by name, MEDIUM
// confidence). e.g. AuthInterceptor, JwtAuthInterceptor, AuthServerInterceptor,
// SecurityInterceptor, AuthenticationInterceptor.
var javaGrpcConventionalInterceptorRe = regexp.MustCompile(
	`(?i)\b(?:auth|authentication|jwt|security)\w*interceptor\b`)

// javaAuthenticatedAnnoRe matches the grpc-spring-boot-starter / Quarkus
// @Authenticated marker — "any authenticated principal", no specific role.
var javaAuthenticatedAnnoRe = regexp.MustCompile(`@Authenticated\b`)

// javaGrpcMethodDeclRe matches a gRPC handler method declaration on a service
// impl, tolerant of annotations interleaved between @Override and the
// signature (which the @Override-anchored scan in grpc_edges.go misses). It
// keys off the StreamObserver response parameter that every server-side gRPC
// handler takes. Group 1 = method name.
var javaGrpcMethodDeclRe = regexp.MustCompile(
	`(?m)public\s+void\s+(\w+)\s*\([^)]*StreamObserver\s*<`)

// grpcJavaAuth is the resolved per-file gRPC-Java auth verdict.
type grpcJavaAuth struct {
	// serviceEnforced is true when an auth-enforcing ServerInterceptor is
	// bound to the service(s) in this file (per-service or global builder).
	serviceEnforced bool
	// serviceSymbol is the interceptor symbol credited (auth_middleware).
	serviceSymbol string
	// serviceConfidence is "high" (in-file auth interceptor) or "medium"
	// (conventionally-named, possibly cross-file interceptor).
	serviceConfidence string
	// classPolicy, when present, is a class-level Spring/Jakarta-Security
	// annotation applying to every method.
	classPolicy *AuthPolicy
	// methodPolicies maps a handler method name to its method-level
	// Spring/Jakarta-Security annotation policy.
	methodPolicies map[string]AuthPolicy
}

// resolveJavaGRPCAuth inspects a Java gRPC source file and returns the auth
// contract to stamp on its service + methods. Same-file, signal-based.
func resolveJavaGRPCAuth(src, path string) grpcJavaAuth {
	out := grpcJavaAuth{methodPolicies: map[string]AuthPolicy{}}

	// ---- Path 1: ServerInterceptor auth ----
	sym, conf := javaInterceptorAuthBinding(src)
	if sym != "" {
		out.serviceEnforced = true
		out.serviceSymbol = sym
		out.serviceConfidence = conf
	}

	// ---- Path 2: Spring/Jakarta-Security annotations ----
	// Class-level annotation (applies to all methods).
	if cp, ok := resolveJavaGRPCClassAuth(src, path); ok {
		out.classPolicy = &cp
	}
	// Method-level annotations, keyed to the method they precede.
	for name, pol := range resolveJavaGRPCMethodAuth(src, path) {
		out.methodPolicies[name] = pol
	}

	return out
}

// javaInterceptorAuthBinding returns the crediting interceptor symbol and the
// confidence when an auth-enforcing ServerInterceptor is bound to a service in
// this file, else ("", "").
//
//   - HIGH: an in-file `class X implements ServerInterceptor` whose body BOTH
//     reads the call Metadata AND rejects with Status.UNAUTHENTICATED /
//     PERMISSION_DENIED, where X is referenced in a ServerInterceptors.intercept
//     / ServerBuilder.intercept binding in the same file.
//   - MEDIUM: a binding argument that is a conventionally-named auth interceptor
//     (…AuthInterceptor / …SecurityInterceptor) whose definition may be
//     cross-file — credited by naming convention.
func javaInterceptorAuthBinding(src string) (symbol, confidence string) {
	// Must have at least one interceptor binding site to bind anything to.
	hasBinding := javaServerInterceptorsInterceptRe.MatchString(src) ||
		javaServerBuilderInterceptRe.MatchString(src)
	if !hasBinding {
		return "", ""
	}

	// In-file auth-enforcing interceptor classes (HIGH).
	highNames := javaAuthEnforcingInterceptorNames(src)

	// Collect the identifier arguments of every binding site.
	bound := javaBoundInterceptorIdents(src)

	// HIGH: a bound identifier names an in-file auth-enforcing interceptor.
	for _, b := range bound {
		if highNames[b.className] {
			return b.className, "high"
		}
	}
	// MEDIUM: a bound identifier is a conventionally-named auth interceptor
	// (definition may be cross-file).
	for _, b := range bound {
		if javaGrpcConventionalInterceptorRe.MatchString(b.raw) {
			return b.className, "medium"
		}
	}
	return "", ""
}

// javaAuthEnforcingInterceptorNames returns the set of in-file class names that
// implement ServerInterceptor and whose body BOTH reads Metadata AND rejects
// with a gRPC auth status — the decisive in-file auth interceptor signal.
func javaAuthEnforcingInterceptorNames(src string) map[string]bool {
	names := map[string]bool{}
	for _, loc := range javaServerInterceptorDeclRe.FindAllStringSubmatchIndex(src, -1) {
		name := src[loc[2]:loc[3]]
		// Body span: from the class decl to the matching close brace.
		body, ok := javaBalancedBraceBody(src, loc[1])
		if !ok {
			// Fall back to a bounded window so a brace-walk miss does not drop
			// a real auth interceptor; the regexes are still anchored to it.
			end := loc[1] + 4000
			if end > len(src) {
				end = len(src)
			}
			body = src[loc[1]:end]
		}
		if javaGrpcMetadataReadRe.MatchString(body) && javaGrpcAuthRejectRe.MatchString(body) {
			names[name] = true
		}
	}
	return names
}

// boundInterceptor is one interceptor reference passed to a binding site.
type boundInterceptor struct {
	raw       string // the raw argument text, e.g. "new AuthInterceptor()"
	className string // the resolved class identifier, e.g. "AuthInterceptor"
}

// javaGrpcNewIdentRe extracts the class name from `new Foo(` and a bare
// identifier `foo`. Group 1 (new form) or group 2 (bare) is the name.
var javaGrpcNewIdentRe = regexp.MustCompile(`\bnew\s+([A-Za-z_]\w*)\s*\(|^\s*([A-Za-z_][\w.]*)\b`)

// javaBoundInterceptorIdents collects the interceptor identifier arguments of
// every ServerInterceptors.intercept(...) and ServerBuilder.intercept(...)
// binding in src. For ServerInterceptors.intercept(service, i1, i2) the first
// argument is the service, the rest interceptors; for builder .intercept(i) the
// single argument is the interceptor.
func javaBoundInterceptorIdents(src string) []boundInterceptor {
	var out []boundInterceptor

	add := func(arg string) {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			return
		}
		cls := arg
		if m := javaGrpcNewIdentRe.FindStringSubmatch(arg); m != nil {
			if m[1] != "" {
				cls = m[1]
			} else if m[2] != "" {
				// bare identifier — strip any leading qualifier path.
				cls = m[2]
				if i := strings.LastIndex(cls, "."); i >= 0 {
					cls = cls[i+1:]
				}
			}
		}
		out = append(out, boundInterceptor{raw: arg, className: cls})
	}

	// ServerInterceptors.intercept(service, ...interceptors)
	for _, loc := range javaServerInterceptorsInterceptRe.FindAllStringIndex(src, -1) {
		args, ok := javaBalancedParenArgs(src, loc[1]-1)
		if !ok {
			continue
		}
		parts := splitTopLevelArgs(args)
		// Skip the first arg (the service); the rest are interceptors.
		for i := 1; i < len(parts); i++ {
			add(parts[i])
		}
	}
	// Builder .intercept(interceptor)
	for _, loc := range javaServerBuilderInterceptRe.FindAllStringIndex(src, -1) {
		args, ok := javaBalancedParenArgs(src, loc[1]-1)
		if !ok {
			continue
		}
		for _, p := range splitTopLevelArgs(args) {
			add(p)
		}
	}
	return out
}

// resolveJavaGRPCClassAuth returns a class-level Spring/Jakarta-Security policy
// when one decorates the @GrpcService impl class itself (applies to every
// method). The annotation block scanned is the run of annotations immediately
// preceding the `class` keyword.
func resolveJavaGRPCClassAuth(src, path string) (AuthPolicy, bool) {
	m := javaClassDeclRe.FindStringIndex(src)
	if m == nil {
		return AuthPolicy{}, false
	}
	// Annotation run preceding the class declaration (bounded window).
	start := m[0] - 1200
	if start < 0 {
		start = 0
	}
	annoBlock := src[start:m[0]]
	line := lineOfOffset(src, m[0])
	if pol, ok := matchAnnotationPolicy(annoBlock, line, path, ""); ok {
		return pol, true
	}
	if javaAuthenticatedAnnoRe.MatchString(annoBlock) {
		return javaAuthenticatedPolicy(line, path), true
	}
	return AuthPolicy{}, false
}

// resolveJavaGRPCMethodAuth returns the per-method Spring/Jakarta-Security
// policies for every gRPC handler method that carries an authorization
// annotation. The annotation block is the run of annotations between the
// previous declaration and the method signature.
func resolveJavaGRPCMethodAuth(src, path string) map[string]AuthPolicy {
	out := map[string]AuthPolicy{}
	decls := javaGrpcMethodDeclRe.FindAllStringSubmatchIndex(src, -1)
	prevEnd := 0
	for _, d := range decls {
		name := src[d[2]:d[3]]
		// Annotation block: from the end of the previous method (or class
		// start) up to this method signature.
		annoBlock := src[prevEnd:d[0]]
		prevEnd = d[1]
		// Only the trailing run after the last `}` / `;` is this method's
		// annotation set — earlier text belongs to prior members.
		if i := strings.LastIndexAny(annoBlock, "};"); i >= 0 {
			annoBlock = annoBlock[i+1:]
		}
		line := lineOfOffset(src, d[0])
		if pol, ok := matchAnnotationPolicy(annoBlock, line, path, ""); ok {
			out[name] = pol
			continue
		}
		if javaAuthenticatedAnnoRe.MatchString(annoBlock) {
			out[name] = javaAuthenticatedPolicy(line, path)
		}
	}
	return out
}

// javaAuthenticatedPolicy builds the AuthPolicy for an @Authenticated marker —
// required, any authenticated principal, no specific role.
func javaAuthenticatedPolicy(line int, file string) AuthPolicy {
	return AuthPolicy{
		Required:   true,
		Method:     "annotation",
		Confidence: "high",
		SourceChain: []AuthSignal{{
			Kind: "annotation",
			Text: "@Authenticated",
			File: file,
			Line: line,
		}},
	}
}

// grpcJavaInterceptorProps returns the props to merge onto a service/method
// protected by an auth-enforcing ServerInterceptor.
func grpcJavaInterceptorProps(symbol, confidence string) map[string]string {
	if confidence == "" {
		confidence = "high"
	}
	props := map[string]string{
		"auth_required":   "true",
		"auth_method":     grpcJavaInterceptorAuthMethod,
		"auth_confidence": confidence,
	}
	if symbol != "" {
		props["auth_middleware"] = symbol
	}
	policy := AuthPolicy{
		Required:   true,
		Method:     "middleware",
		Confidence: confidence,
		SourceChain: []AuthSignal{{
			Kind: "middleware",
			Text: "grpc-interceptor: " + symbol,
		}},
	}
	if j := EncodeAuthPolicy(policy); j != "" {
		props["auth_policy"] = j
	}
	return props
}

// grpcJavaPolicyProps turns a resolved Spring/Jakarta-Security AuthPolicy into
// the flat companion props + auth_policy JSON, mirroring the HTTP Java stamping
// in java_annotation_routes.go.
func grpcJavaPolicyProps(policy AuthPolicy) map[string]string {
	props := map[string]string{}
	if j := EncodeAuthPolicy(policy); j != "" {
		props["auth_policy"] = j
	}
	props["auth_method"] = policy.Method
	props["auth_confidence"] = policy.Confidence
	if policy.Required {
		props["auth_required"] = "true"
	} else if policy.Method != "unknown" {
		props["auth_required"] = "false"
	}
	if len(policy.Roles) > 0 {
		props["auth_roles"] = strings.Join(policy.Roles, ",")
	}
	if len(policy.Permissions) > 0 {
		perms := append([]string(nil), policy.Permissions...)
		sort.Strings(perms)
		props["auth_permissions"] = strings.Join(perms, ",")
	}
	if len(policy.Scopes) > 0 {
		scs := append([]string(nil), policy.Scopes...)
		sort.Strings(scs)
		props["auth_scopes"] = strings.Join(scs, ",")
	}
	return props
}

// javaBalancedParenArgs returns the substring inside the parentheses whose
// opening `(` is at index openParen, walking balanced parens while skipping
// double-quoted, single-quoted and char spans. Excludes the outer parens.
func javaBalancedParenArgs(s string, openParen int) (string, bool) {
	if openParen < 0 || openParen >= len(s) || s[openParen] != '(' {
		return "", false
	}
	depth := 0
	i := openParen
	for i < len(s) {
		c := s[i]
		switch c {
		case '"', '\'':
			i = javaSkipQuoted(s, i)
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[openParen+1 : i], true
			}
		}
		i++
	}
	return "", false
}

// javaBalancedBraceBody returns the substring inside the first `{...}` block at
// or after index from, walking balanced braces while skipping quoted spans.
func javaBalancedBraceBody(s string, from int) (string, bool) {
	open := -1
	for i := from; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\'' {
			i = javaSkipQuoted(s, i) - 1
			continue
		}
		if c == '{' {
			open = i
			break
		}
	}
	if open < 0 {
		return "", false
	}
	depth := 0
	i := open
	for i < len(s) {
		c := s[i]
		switch c {
		case '"', '\'':
			i = javaSkipQuoted(s, i)
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[open+1 : i], true
			}
		}
		i++
	}
	return "", false
}

// javaSkipQuoted returns the index just past the quoted span starting at i
// (s[i] is the opening quote). Handles backslash escapes.
func javaSkipQuoted(s string, i int) int {
	quote := s[i]
	i++
	for i < len(s) {
		c := s[i]
		if c == '\\' {
			i += 2
			continue
		}
		if c == quote {
			return i + 1
		}
		i++
	}
	return len(s)
}
