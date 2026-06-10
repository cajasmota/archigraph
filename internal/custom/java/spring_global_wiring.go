package java

import (
	"regexp"
	"strings"

	"github.com/cajasmota/archigraph/internal/types"
)

// Spring global cross-cutting wiring extractor (#4377).
//
// Spring registers cross-cutting behaviour app-wide through several idioms that
// the per-class DI extractor (spring_boot.go) does not capture, leaving the
// registered interceptor / filter / advice class with NO inbound edge — it
// looks orphan / dead and the app-wide scope is invisible. This generalises the
// NestJS global-DI fix (#4329) and the Django settings fix (#4379) to Spring,
// reusing the same convention: a config/app-scope carrier entity → target class
// USES edge marked `global=true` with a `di_role`.
//
// Shapes covered:
//
//  1. WebMvcConfigurer.addInterceptors(InterceptorRegistry r) {
//        r.addInterceptor(new AuthInterceptor()).addPathPatterns("/**");
//     }
//     → MvcConfig (the WebMvcConfigurer impl) → AuthInterceptor USES,
//       di_role=interceptor, capturing the path patterns when present.
//
//  2. @Component @Order(1) class LoggingFilter implements Filter
//     → spring_app → LoggingFilter USES, di_role=filter, order=1.
//
//  3. @Bean FilterRegistrationBean<X> reg(){ registration.setFilter(new X()); }
//     → owning @Configuration → X USES, di_role=filter, order=<setOrder(n)>.
//
//  4. @ControllerAdvice / @RestControllerAdvice class GlobalExceptionHandler {
//        @ExceptionHandler(FooException.class) ... }
//     → spring_app → GlobalExceptionHandler USES, di_role=exception_advice;
//       plus GlobalExceptionHandler → FooException USES (di_role=handles_exception)
//       for each @ExceptionHandler(X.class) it declares.
//
// Resolution: every target is emitted as a bare `Class:<Name>` stub on the edge
// ToID, which the real resolve.BuildIndex symbol table binds to the in-repo
// class node by name (merge-stable: resolves whether the class survives as a
// base tree-sitter node or a custom stereotype node). The carrier config/app
// entity owns the edge so it is retained even when nothing else references it.
//
// Spring Security's SecurityFilterChain @Bean / WebSecurityConfigurerAdapter
// filter wiring is a deliberate follow-up (epic #4334) — see the note at the
// bottom of this file.

var (
	// springWebMvcConfigClassRE matches a class implementing WebMvcConfigurer,
	// capturing the config class name (group 1). `(?s)` lets the annotation
	// block + class header span lines; matching stops before the body `{`.
	springWebMvcConfigClassRE = regexp.MustCompile(
		`(?s)class\s+(\w+)\b[^{]*?\bimplements\b[^{]*?\bWebMvcConfigurer\b`)

	// springAddInterceptorRE matches a `.addInterceptor(new Foo(...))` registry
	// call inside addInterceptors(...), capturing the interceptor class name
	// (group 1). Both `new Foo()` and a bare bean reference `fooInterceptor`
	// would appear here; we capture the `new Foo` form (the dominant idiom) and
	// the bare PascalCase identifier form.
	springAddInterceptorRE = regexp.MustCompile(
		`\.addInterceptor\s*\(\s*(?:new\s+)?([A-Z]\w*)\b`)

	// springAddPathPatternsRE captures the literal path patterns chained off an
	// interceptor registration: .addPathPatterns("/a/**", "/b"). Group 1 is the
	// raw comma-separated argument list (string literals).
	springAddPathPatternsRE = regexp.MustCompile(
		`\.addPathPatterns\s*\(([^)]*)\)`)

	// springStringLiteralRE pulls double-quoted literals out of an argument list.
	springStringLiteralRE = regexp.MustCompile(`"([^"]*)"`)

	// springFilterClassRE matches a class implementing the servlet Filter
	// interface (jakarta.servlet.Filter / javax.servlet.Filter), capturing the
	// class name (group 1). Requires a registration signal (handled by the
	// caller checking for @Component / @WebFilter in the preceding window).
	springFilterClassRE = regexp.MustCompile(
		`(?s)class\s+(\w+)\b[^{]*?\bimplements\b[^{]*?\bFilter\b`)

	// springOrderAnnotationRE captures the @Order(n) value preceding a class.
	springOrderAnnotationRE = regexp.MustCompile(`@Order\s*\(\s*(\d+)\s*\)`)

	// springSetFilterRE matches `...setFilter(new Foo(...))` on a
	// FilterRegistrationBean, capturing the filter class name (group 1).
	springSetFilterRE = regexp.MustCompile(
		`\.setFilter\s*\(\s*(?:new\s+)?([A-Z]\w*)\b`)

	// springRegistrationBeanFilterRE matches the generic type argument of a
	// FilterRegistrationBean<Foo> declaration, capturing the filter class name
	// (group 1) — the type-parameter form of filter registration that does not
	// always call setFilter explicitly.
	springRegistrationBeanFilterRE = regexp.MustCompile(
		`FilterRegistrationBean\s*<\s*([A-Z]\w*)\b`)

	// springSetOrderRE captures `registration.setOrder(n)` to record the filter
	// order on the registration-bean edge.
	springSetOrderRE = regexp.MustCompile(`\.setOrder\s*\(\s*(\d+)\s*\)`)

	// springControllerAdviceRE matches a @ControllerAdvice / @RestControllerAdvice
	// class, capturing the advice class name (group 1).
	springControllerAdviceRE = regexp.MustCompile(
		`(?s)@(?:Rest)?ControllerAdvice\b[^{]*?\bclass\s+(\w+)`)

	// springExceptionHandlerTypesRE matches @ExceptionHandler(Foo.class, Bar.class)
	// capturing the raw class-list argument (group 1). The `value =` attribute
	// form and a single arg are both covered.
	springExceptionHandlerTypesRE = regexp.MustCompile(
		`@ExceptionHandler\s*\(\s*(?:value\s*=\s*)?\{?([^)}]*)\}?\s*\)`)

	// springDotClassRE pulls each `Foo.class` token out of an exception-handler
	// argument list, capturing the exception class name (group 1).
	springDotClassRE = regexp.MustCompile(`([A-Z]\w*)\s*\.\s*class\b`)
)

// springGlobalWiringFrameworks gates the Spring frameworks for which global
// cross-cutting wiring is extracted. Mirrors springBootFrameworks / aopFrameworks.
var springGlobalWiringFrameworks = map[string]bool{
	"spring_boot": true, "spring-boot": true, "springboot": true,
	"spring_mvc": true, "spring-mvc": true, "springmvc": true,
	"spring_webflux": true, "spring-webflux": true, "springwebflux": true,
}

// springAppEntityName is the synthetic owner name for app-level Spring global
// wiring that has no owning config class (servlet @Component filters and
// @ControllerAdvice apply app-wide). Mirrors the NestJS `app` entity (#4329)
// and the Django `django_settings` entity (#4379).
const springAppEntityName = "spring_app"

// classStub returns the resolvable `Class:<Name>` symbol-table stub for a class
// name. resolve.BuildIndex binds this to the in-repo class node by name,
// merge-stably (whether the class survives as a base tree-sitter node or a
// custom stereotype node).
func classStub(name string) string { return "Class:" + name }

// ExtractSpringGlobalWiring runs the Spring global cross-cutting wiring
// extractor. Accepts Java and Kotlin Spring sources (the annotation/registry
// idioms are regex-equivalent in Kotlin).
func ExtractSpringGlobalWiring(ctx PatternContext) PatternResult {
	var result PatternResult
	if (ctx.Language != "java" && ctx.Language != "kotlin") ||
		!springGlobalWiringFrameworks[ctx.Framework] {
		return result
	}

	source := ctx.Source
	fp := ctx.FilePath
	seenRefs := make(map[string]bool)
	seenRels := make(map[relKey]bool)

	// appRef lazily materialises the synthetic app-scope carrier the first time
	// an app-wide (non-config-owned) edge needs an owner.
	var appRef string
	ensureApp := func() string {
		if appRef != "" {
			return appRef
		}
		appRef = "scope:pattern:spring_app:" + fp + ":" + springAppEntityName
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: springAppEntityName, Kind: "SCOPE.Pattern", Subtype: "application",
			SourceFile: fp, LineStart: 1, LineEnd: 1,
			Provenance: "INFERRED_FROM_SPRING_GLOBAL_WIRING",
			Ref:        appRef,
			Properties: map[string]any{
				"framework": "spring_boot",
				"scope":     "application",
			},
		})
		return appRef
	}

	// ---- 1. WebMvcConfigurer interceptor registration ----------------------
	// Each WebMvcConfigurer impl owns a config carrier entity; the interceptors
	// it registers via r.addInterceptor(new X()) become config → X USES edges.
	type cfgInfo struct {
		name   string
		offset int
		ref    string
	}
	var mvcConfigs []cfgInfo
	for _, m := range springWebMvcConfigClassRE.FindAllStringSubmatchIndex(source, -1) {
		name := source[m[2]:m[3]]
		ref := "scope:pattern:spring_webmvc_config:" + fp + ":" + name
		if addEntity(&result, seenRefs, SecondaryEntity{
			Name: name, Kind: "SCOPE.Pattern", Subtype: "mvc_config",
			SourceFile: fp, LineStart: lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_SPRING_WEBMVC_CONFIGURER", Ref: ref,
			Properties: map[string]any{"framework": "spring_boot"},
		}) {
			mvcConfigs = append(mvcConfigs, cfgInfo{name, m[0], ref})
		}
	}

	// Owning WebMvcConfigurer for an offset (nearest preceding config class).
	ownerMvcConfig := func(offset int) (string, string) {
		var bestName, bestRef string
		bestOff := -1
		for _, c := range mvcConfigs {
			if c.offset <= offset && c.offset > bestOff {
				bestName, bestRef, bestOff = c.name, c.ref, c.offset
			}
		}
		return bestName, bestRef
	}

	if len(mvcConfigs) > 0 {
		for _, m := range springAddInterceptorRE.FindAllStringSubmatchIndex(source, -1) {
			cls := source[m[2]:m[3]]
			if primitiveTypes[cls] {
				continue
			}
			_, ownerRef := ownerMvcConfig(m[0])
			if ownerRef == "" {
				continue
			}
			props := map[string]string{
				"framework": "spring_boot",
				"di_role":   "interceptor",
				"di_scope":  "global",
				"global":    "true",
				"via":       "spring_webmvc_add_interceptor",
			}
			// Path patterns chained off this addInterceptor(...) call.
			if pp := springInterceptorPathPatterns(source, m[1]); pp != "" {
				props["path_patterns"] = pp
			}
			addRel(&result, seenRels, Relationship{
				SourceRef:        ownerRef,
				TargetRef:        classStub(cls),
				RelationshipType: string(types.RelationshipKindUses),
				Properties:       props,
			})
		}
	}

	// ---- 2. Servlet @Component / @Order Filter classes ---------------------
	// A class implementing Filter, registered as a bean (@Component / @WebFilter),
	// is applied app-wide. The synthetic spring_app entity owns the edge.
	for _, m := range springFilterClassRE.FindAllStringSubmatchIndex(source, -1) {
		cls := source[m[2]:m[3]]
		// Require a registration signal in the preceding annotation window so we
		// don't link arbitrary Filter implementations that are wired elsewhere.
		win := windowBefore(source, m[0], 300)
		if !springHasComponentSignal(win) {
			continue
		}
		props := map[string]string{
			"framework": "spring_boot",
			"di_role":   "filter",
			"di_scope":  "global",
			"global":    "true",
			"via":       "spring_servlet_filter_component",
		}
		if om := springOrderAnnotationRE.FindStringSubmatch(win); om != nil {
			props["order"] = om[1]
		}
		addRel(&result, seenRels, Relationship{
			SourceRef:        ensureApp(),
			TargetRef:        classStub(cls),
			RelationshipType: string(types.RelationshipKindUses),
			Properties:       props,
		})
	}

	// ---- 3. FilterRegistrationBean @Bean filter registration ---------------
	// @Bean FilterRegistrationBean<X> reg(){ registration.setFilter(new X()); }
	// registers X app-wide. The owning @Configuration class carries the edge;
	// when none is detected in-file, fall back to the synthetic app entity.
	registerFilter := func(offset int, cls string) {
		if cls == "" || primitiveTypes[cls] {
			return
		}
		ownerRef := springOwningConfigRef(source, offset, fp, &result)
		if ownerRef == "" {
			ownerRef = ensureApp()
		}
		props := map[string]string{
			"framework": "spring_boot",
			"di_role":   "filter",
			"di_scope":  "global",
			"global":    "true",
			"via":       "spring_filter_registration_bean",
		}
		// setOrder(n) within a bounded window after the registration call.
		if om := springSetOrderRE.FindStringSubmatch(windowAfter(source, offset, 600)); om != nil {
			props["order"] = om[1]
		}
		addRel(&result, seenRels, Relationship{
			SourceRef:        ownerRef,
			TargetRef:        classStub(cls),
			RelationshipType: string(types.RelationshipKindUses),
			Properties:       props,
		})
	}
	for _, m := range springSetFilterRE.FindAllStringSubmatchIndex(source, -1) {
		registerFilter(m[0], source[m[2]:m[3]])
	}
	for _, m := range springRegistrationBeanFilterRE.FindAllStringSubmatchIndex(source, -1) {
		registerFilter(m[0], source[m[2]:m[3]])
	}

	// ---- 4. @ControllerAdvice / @RestControllerAdvice ----------------------
	// A global advice class applies app-wide; the synthetic app entity owns the
	// edge. Each @ExceptionHandler(X.class) inside it links the advice to the
	// exception type it handles.
	for _, m := range springControllerAdviceRE.FindAllStringSubmatchIndex(source, -1) {
		adviceCls := source[m[2]:m[3]]
		addRel(&result, seenRels, Relationship{
			SourceRef:        ensureApp(),
			TargetRef:        classStub(adviceCls),
			RelationshipType: string(types.RelationshipKindUses),
			Properties: map[string]string{
				"framework":    "spring_boot",
				"di_role":      "exception_advice",
				"di_scope":     "global",
				"global":       "true",
				"advice_class": adviceCls,
				"via":          "spring_controller_advice",
			},
		})

		// advice → exception-type USES edges. The advice class is the carrier;
		// FromName binds the edge source to the advice class by name even though
		// the carrier entity is the synthetic app (so the edge originates from the
		// advice, mirroring the #4367 FromName convention).
		adviceStub := classStub(adviceCls)
		body := springClassBody(source, m[0])
		for _, em := range springExceptionHandlerTypesRE.FindAllStringSubmatch(body, -1) {
			for _, cm := range springDotClassRE.FindAllStringSubmatch(em[1], -1) {
				exc := cm[1]
				addRel(&result, seenRels, Relationship{
					SourceRef:        ensureApp(),
					FromName:         adviceStub,
					TargetRef:        classStub(exc),
					RelationshipType: string(types.RelationshipKindUses),
					Properties: map[string]string{
						"framework":    "spring_boot",
						"di_role":      "handles_exception",
						"global":       "true",
						"advice_class": adviceCls,
						"via":          "spring_exception_handler",
					},
				})
			}
		}
	}

	return result
}

// springInterceptorPathPatterns returns the comma-joined path patterns chained
// off an interceptor registration starting at `from` (just past the matched
// .addInterceptor(...) head), scanning a bounded window. Empty when none.
func springInterceptorPathPatterns(source string, from int) string {
	win := windowAfter(source, from, 300)
	// Only consider the first addPathPatterns chained before a ';' (statement end).
	if semi := strings.IndexByte(win, ';'); semi >= 0 {
		win = win[:semi]
	}
	pm := springAddPathPatternsRE.FindStringSubmatch(win)
	if pm == nil {
		return ""
	}
	var pats []string
	for _, lm := range springStringLiteralRE.FindAllStringSubmatch(pm[1], -1) {
		pats = append(pats, lm[1])
	}
	return strings.Join(pats, ",")
}

// springHasComponentSignal reports whether a preceding-annotation window marks a
// class as a registered Spring bean filter (@Component / @WebFilter / @Service).
func springHasComponentSignal(window string) bool {
	return strings.Contains(window, "@Component") ||
		strings.Contains(window, "@WebFilter") ||
		strings.Contains(window, "@Service") ||
		strings.Contains(window, "@Order")
}

// springOwningConfigRef returns the ref of the nearest preceding @Configuration
// class for offset, materialising a config carrier entity if one is not already
// emitted. Returns "" when no @Configuration precedes the offset in-file.
func springOwningConfigRef(source string, offset int, fp string, result *PatternResult) string {
	bestName := ""
	bestOff := -1
	for _, m := range sbConfigurationClassRE.FindAllStringSubmatchIndex(source, -1) {
		if m[0] <= offset && m[0] > bestOff {
			bestName = source[m[2]:m[3]]
			bestOff = m[0]
		}
	}
	if bestName == "" {
		return ""
	}
	// Reuse the spring_boot config ref shape so this carrier merges with the
	// ExtractSpringBoot @Configuration entity (same ref → deduped by addEntity).
	ref := "scope:pattern:spring_boot_config:" + fp + ":" + bestName
	result.Entities = append(result.Entities, SecondaryEntity{
		Name: bestName, Kind: "SCOPE.Pattern", SourceFile: fp,
		LineStart: lineOf(source, bestOff), LineEnd: lineOf(source, bestOff),
		Provenance: "INFERRED_FROM_SPRING_BOOT_CONFIGURATION", Ref: ref,
		Properties: map[string]any{"framework": "spring_boot"},
	})
	return ref
}

// springClassBody returns a bounded slice of source starting at the class
// declaration offset, used to scope @ExceptionHandler scanning to (roughly) the
// advice class body. A generous window covers typical advice classes without
// requiring brace-balanced parsing.
func springClassBody(source string, classOffset int) string {
	end := classOffset + 4000
	if end > len(source) {
		end = len(source)
	}
	return source[classOffset:end]
}

// windowBefore returns up to n bytes of source ending at offset.
func windowBefore(source string, offset, n int) string {
	start := offset - n
	if start < 0 {
		start = 0
	}
	return source[start:offset]
}

// windowAfter returns up to n bytes of source starting at offset.
func windowAfter(source string, offset, n int) string {
	end := offset + n
	if end > len(source) {
		end = len(source)
	}
	return source[offset:end]
}

// NOTE (deferred, epic #4334): Spring Security global wiring —
// @EnableGlobalMethodSecurity and the SecurityFilterChain @Bean /
// WebSecurityConfigurerAdapter.configure(HttpSecurity) graph (entry points,
// custom filters added via http.addFilterBefore(new X(), ...)) — is a tight
// follow-up. The carrier/edge convention here (config/app → Class:<X> USES,
// global=true, di_role) extends directly to it: emit, for each
// http.addFilterBefore/After/At(new X(), Y.class), a security-config → X USES
// edge with di_role=security_filter. Filed separately to keep this PR scoped to
// the MVC interceptor / servlet filter / controller-advice shapes.
