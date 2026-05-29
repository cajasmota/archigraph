package java

import "regexp"

// Helidon MP middleware / filter extractor — middleware_coverage cell.
//
// Helidon MP uses the standard JAX-RS filter model (ContainerRequestFilter /
// ContainerResponseFilter) annotated with @Provider, plus the
// io.helidon.webserver.Handler functional interface for Helidon SE routing
// handlers registered programmatically.
//
// Coverage cells delivered (#3088):
//   - lang.java.framework.helidon → Middleware.middleware_coverage (missing → partial)
//
// Detected patterns:
//   1. @Provider + implements ContainerRequestFilter  → request filter
//   2. @Provider + implements ContainerResponseFilter → response filter
//   3. @NameBinding meta-annotation on a custom filter binding annotation
//   4. io.helidon.webserver.Handler implementation (Helidon SE functional handler)

var helidonFrameworks = map[string]bool{
	"helidon": true,
}

var (
	// JAX-RS @Provider class — ContainerRequestFilter or ContainerResponseFilter.
	helidonProviderFilterRE = regexp.MustCompile(
		`(?s)@Provider\b[^{]*?` +
			`(?:public\s+)?(?:(?:abstract|final)\s+)?class\s+(\w+)` +
			`[^{]*?implements\s+[^{]*?(ContainerRequestFilter|ContainerResponseFilter)\b`)

	// Reverse form: implements first, @Provider anywhere in the preceding 300 chars.
	helidonFilterClassRE = regexp.MustCompile(
		`(?s)class\s+(\w+)\s+(?:extends\s+\w+\s+)?implements\s+[^{]*(ContainerRequestFilter|ContainerResponseFilter)\b`)

	// @NameBinding meta-annotation on a custom annotation type.
	// Uses .*? (dotall) to skip any intermediate annotations like
	// @Retention / @Target before the @interface declaration.
	helidonNameBindingRE = regexp.MustCompile(
		`(?s)@NameBinding\b.*?(?:public\s+)?@interface\s+(\w+)`)

	// Helidon SE Handler: class implements Handler with a single-method accept().
	helidonSEHandlerRE = regexp.MustCompile(
		`(?s)class\s+(\w+)\s+(?:extends\s+\w+\s+)?implements\s+[^{]*\bio\.helidon\.webserver\.Handler\b`)

	// @PreMatching filter variant (JAX-RS).
	helidonPreMatchingRE = regexp.MustCompile(
		`(?s)@PreMatching\b[^{]*?` +
			`(?:public\s+)?(?:(?:abstract|final)\s+)?class\s+(\w+)` +
			`[^{]*?implements\s+[^{]*?ContainerRequestFilter\b`)
)

// ExtractHelidonFilters runs the Helidon middleware/filter extractor.
func ExtractHelidonFilters(ctx PatternContext) PatternResult {
	var result PatternResult
	if ctx.Language != "java" || !helidonFrameworks[ctx.Framework] {
		return result
	}

	source := ctx.Source
	fp := ctx.FilePath
	seenRefs := make(map[string]bool)

	// 1. @Provider + ContainerRequestFilter / ContainerResponseFilter (canonical form).
	for _, m := range helidonProviderFilterRE.FindAllStringSubmatchIndex(source, -1) {
		className := source[m[2]:m[3]]
		filterType := source[m[4]:m[5]]
		ref := "scope:component:helidon_jaxrs_filter:" + fp + ":" + className
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: className, Kind: "SCOPE.Component", SourceFile: fp,
			LineStart: lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_HELIDON_JAXRS_FILTER", Ref: ref,
			Properties: map[string]any{
				"filter_type": toLowerCase(filterType),
				"framework":   "helidon",
				"middleware":  "jaxrs_provider_filter",
			},
		})
	}

	// 2. Filter class without leading @Provider (e.g. @Provider on prior line
	//    in the annotation block — check for @Provider anywhere in the preceding
	//    300-char window).
	for _, m := range helidonFilterClassRE.FindAllStringSubmatchIndex(source, -1) {
		className := source[m[2]:m[3]]
		filterType := source[m[4]:m[5]]
		ref := "scope:component:helidon_jaxrs_filter:" + fp + ":" + className
		if seenRefs[ref] {
			continue
		}
		// Only record if @Provider appears in the preceding window (avoids
		// emitting raw non-registered filters).
		windowStart := m[0]
		if windowStart > 300 {
			windowStart = m[0] - 300
		} else {
			windowStart = 0
		}
		window := source[windowStart:m[0]]
		if !providerAnnotRE.MatchString(window) {
			continue
		}
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: className, Kind: "SCOPE.Component", SourceFile: fp,
			LineStart: lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_HELIDON_JAXRS_FILTER", Ref: ref,
			Properties: map[string]any{
				"filter_type": toLowerCase(filterType),
				"framework":   "helidon",
				"middleware":  "jaxrs_provider_filter",
			},
		})
	}

	// 3. @NameBinding custom filter binding annotation.
	for _, m := range helidonNameBindingRE.FindAllStringSubmatchIndex(source, -1) {
		annName := source[m[2]:m[3]]
		ref := "scope:pattern:helidon_name_binding:" + fp + ":" + annName
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: annName, Kind: "SCOPE.Pattern", SourceFile: fp,
			LineStart: lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_HELIDON_NAME_BINDING", Ref: ref,
			Properties: map[string]any{
				"framework":  "helidon",
				"middleware": "name_binding",
			},
		})
	}

	// 4. @PreMatching filter (subset of ContainerRequestFilter).
	for _, m := range helidonPreMatchingRE.FindAllStringSubmatchIndex(source, -1) {
		className := source[m[2]:m[3]]
		ref := "scope:component:helidon_jaxrs_filter:" + fp + ":" + className
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: className, Kind: "SCOPE.Component", SourceFile: fp,
			LineStart: lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_HELIDON_JAXRS_FILTER", Ref: ref,
			Properties: map[string]any{
				"filter_type": "prematching_request_filter",
				"framework":   "helidon",
				"middleware":  "jaxrs_prematching_filter",
			},
		})
	}

	// 5. Helidon SE Handler functional interface.
	for _, m := range helidonSEHandlerRE.FindAllStringSubmatchIndex(source, -1) {
		className := source[m[2]:m[3]]
		ref := "scope:component:helidon_se_handler:" + fp + ":" + className
		addEntity(&result, seenRefs, SecondaryEntity{
			Name: className, Kind: "SCOPE.Component", SourceFile: fp,
			LineStart: lineOf(source, m[0]), LineEnd: lineOf(source, m[0]),
			Provenance: "INFERRED_FROM_HELIDON_SE_HANDLER", Ref: ref,
			Properties: map[string]any{
				"framework":  "helidon",
				"middleware": "helidon_se_handler",
			},
		})
	}

	return result
}

// providerAnnotRE matches @Provider in an annotation block preceding a class.
var providerAnnotRE = regexp.MustCompile(`@Provider\b`)
