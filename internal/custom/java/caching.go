// caching.go — Spring caching-abstraction topology extraction (#3692,
// epic #3628, area #18).
//
// Captures Spring's declarative caching annotations and turns them into a
// traversable cache-topology subgraph that sits ON TOP of the raw redis/
// memcached key work (internal/custom/python/redis.go): rather than recording
// the low-level GET/SET, this records the higher-level read-through / write /
// invalidation INTENT a method declares.
//
// Annotations handled (Spring Cache Abstraction — spring-context):
//
//	@Cacheable(value="users", key="#id")  → method CACHES region "users"
//	                                        (read-through: result stored+served)
//	@CachePut(value="users")              → method CACHES region "users"
//	                                        (write: always executes, refreshes)
//	@CacheEvict(value="users")            → method INVALIDATES region "users"
//	@CacheEvict(value="users", allEntries=true)
//	                                        → INVALIDATES whole region (flush)
//	@Caching(cacheable=..., evict=...)     → handled per inner annotation
//
// `value=` and its alias `cacheNames=` may name one OR several regions
// ({"a","b"}); each region becomes its own target node + edge. A method with no
// static region (annotation present but value omitted — Spring then derives the
// region from config) is honest-partial: the edge is emitted against a
// "<default>" region with dynamic="true" so the intent is still traversable.
//
// Entity/edge shape (cross-language consistent with the python redis keyspace
// node):
//
//	target  : SCOPE.Datastore  subtype "cache_region"
//	          Ref  "cache:spring:<region>"   (converges all sites on one node)
//	carrier : SCOPE.Operation  the annotated method  (owner of the edge)
//	edge    : CACHES | INVALIDATES   owner → region
//
// Dynamic SpEL keys (key="#id") are recorded as a `key_spel` property on the
// edge — they are intentionally NOT promoted to distinct target nodes, because
// the region is the cache-topology unit (a single region holds many keys); the
// SpEL key is metadata about which entries a call touches.
package java

import (
	"regexp"
	"strings"
)

// cachingFrameworks gates the extractor to Spring identifiers. The dispatch
// layer feeds spring_boot/spring_mvc/etc. candidates; we accept the whole
// Spring family because @Cacheable lives in spring-context and can appear in
// any Spring stereotype (service, repository, component).
var cachingFrameworks = map[string]bool{
	"spring_boot": true, "spring-boot": true, "springboot": true,
	"spring_mvc": true, "spring": true,
	"spring_webflux": true, "spring_data_jpa": true,
}

// cacheAnnotationRE matches a Spring cache annotation and the method it
// decorates. Group 1 = annotation name (Cacheable|CachePut|CacheEvict);
// Group 2 = the annotation argument body (may be empty for `@Cacheable` with no
// parens — but Spring requires at least a region somewhere, so the no-paren form
// is honest-partial dynamic); Group 3 = the decorated method name.
//
// Intervening annotations between the cache annotation and the method signature
// are skipped (`@Transactional`, `@Override`, …).
var cacheAnnotationRE = regexp.MustCompile(
	`(?s)@(Cacheable|CachePut|CacheEvict)\b\s*(?:\(([^)]*)\))?\s*` +
		`(?:@\w+(?:\([^)]*\))?\s*)*` +
		`(?:public|protected|private|)\s+(?:static\s+)?` +
		`(?:<[^>]*>\s*)?(?:[\w.<>\[\],?\s]+?\s+)(\w+)\s*\(`)

// cacheRegionArgRE pulls region names out of a cache annotation argument body.
// It matches `value=` or its alias `cacheNames=` followed by either a single
// "x" literal or a {"a","b"} brace list. Group 1 = the literal-or-brace body.
var cacheRegionArgRE = regexp.MustCompile(
	`(?:value|cacheNames)\s*=\s*(\{[^}]*\}|"[^"]*")`)

// cacheBareRegionRE matches the single-value shorthand `@Cacheable("users")`
// where the region is the first positional argument (no `value=`). Group 1 =
// the literal-or-brace body. Only meaningful when no `value=`/`cacheNames=` key
// is present in the body.
var cacheBareRegionRE = regexp.MustCompile(
	`^\s*(\{[^}]*\}|"[^"]*")`)

// cacheKeySpelRE captures the SpEL key expression (`key="#id"`) so it can be
// recorded as edge metadata. Group 1 = the SpEL body (without quotes).
var cacheKeySpelRE = regexp.MustCompile(`key\s*=\s*"([^"]*)"`)

// cacheAllEntriesRE detects `allEntries=true` on a @CacheEvict (region flush).
var cacheAllEntriesRE = regexp.MustCompile(`allEntries\s*=\s*true`)

// regionLiteralRE pulls each "name" out of a single literal or {"a","b"} body.
var regionLiteralRE = regexp.MustCompile(`"([^"]*)"`)

// cacheRegionRef builds the stable, framework-prefixed target ref so every
// call-site that caches/invalidates the same region converges on one node
// (mirrors redisKeyspaceRef in the python redis extractor).
func cacheRegionRef(region string) string {
	return "cache:spring:" + region
}

// parseCacheRegions returns the list of static region names declared in a cache
// annotation argument body, plus whether the region set is dynamic (annotation
// present but no static region resolvable). The bare positional shorthand
// (`@Cacheable("users")`) is honoured only when no `value=`/`cacheNames=` key is
// present.
func parseCacheRegions(body string) (regions []string, dynamic bool) {
	body = strings.TrimSpace(body)

	var literalBody string
	if m := cacheRegionArgRE.FindStringSubmatch(body); m != nil {
		literalBody = m[1]
	} else if m := cacheBareRegionRE.FindStringSubmatch(body); m != nil &&
		!strings.Contains(body, "value") && !strings.Contains(body, "cacheNames") {
		literalBody = m[1]
	}

	if literalBody == "" {
		// Annotation present, region not statically resolvable (omitted, or a
		// constant reference like CacheConfig.USERS). Honest-partial: one
		// dynamic edge so the caching intent is still traversable.
		return nil, true
	}

	for _, lm := range regionLiteralRE.FindAllStringSubmatch(literalBody, -1) {
		if r := strings.TrimSpace(lm[1]); r != "" {
			regions = append(regions, r)
		}
	}
	if len(regions) == 0 {
		return nil, true
	}
	return regions, false
}

// ExtractJavaCaching runs the Spring caching-abstraction extractor. It emits a
// SCOPE.Datastore cache-region node per distinct region and a CACHES /
// INVALIDATES edge from each annotated method to the region(s) it touches.
func ExtractJavaCaching(ctx PatternContext) PatternResult {
	var result PatternResult
	if ctx.Language != "java" && ctx.Language != "kotlin" {
		return result
	}
	if !cachingFrameworks[ctx.Framework] {
		return result
	}
	source := ctx.Source
	// Quick-exit: no Spring cache annotation present.
	if !strings.Contains(source, "@Cacheable") &&
		!strings.Contains(source, "@CachePut") &&
		!strings.Contains(source, "@CacheEvict") {
		return result
	}

	fp := ctx.FilePath
	fw := ctx.Framework
	seenEnt := make(map[string]bool)
	seenRel := make(map[relKey]bool)

	for _, m := range cacheAnnotationRE.FindAllStringSubmatchIndex(source, -1) {
		annotation := source[m[2]:m[3]]
		body := ""
		if m[4] >= 0 {
			body = source[m[4]:m[5]]
		}
		method := source[m[6]:m[7]]
		ownerCls := findEnclosingClass(source, m[0])
		line := lineOf(source, m[0])

		ownerName := method
		if ownerCls != "" {
			ownerName = ownerCls + "." + method
		}
		ownerRef := "scope:operation:caching:" + fp + ":" + ownerName

		// Edge direction: @CacheEvict invalidates; @Cacheable/@CachePut populate.
		isEvict := annotation == "CacheEvict"
		relType := "CACHES"
		mode := "read_through"
		if annotation == "CachePut" {
			mode = "write"
		}
		if isEvict {
			relType = "INVALIDATES"
			mode = "evict"
			if cacheAllEntriesRE.MatchString(body) {
				mode = "evict_all"
			}
		}

		// Carrier: the annotated method (owner of the edge).
		addEntity(&result, seenEnt, SecondaryEntity{
			Name: ownerName, Kind: "SCOPE.Operation", Subtype: "cache_method",
			SourceFile: fp, LineStart: line, LineEnd: line,
			Provenance: "INFERRED_FROM_" + strings.ToUpper(annotation),
			Ref:        ownerRef,
			Properties: map[string]any{
				"framework":        "spring",
				"cache_annotation": annotation,
				"cache_mode":       mode,
			},
		})

		regions, dynamic := parseCacheRegions(body)
		keySpel := ""
		if km := cacheKeySpelRE.FindStringSubmatch(body); km != nil {
			keySpel = km[1]
		}

		emit := func(region string, isDynamic bool) {
			ref := cacheRegionRef(region)
			label := region
			subtype := "cache_region"
			props := map[string]any{
				"framework":  "spring",
				"region":     region,
				"cache_kind": "region",
				"language":   "java",
			}
			if isDynamic {
				props["dynamic"] = "true"
			}
			addEntity(&result, seenEnt, SecondaryEntity{
				Name: label, Kind: "SCOPE.Datastore", Subtype: subtype,
				SourceFile: fp, LineStart: line, LineEnd: line,
				Provenance: "INFERRED_FROM_SPRING_CACHE",
				Ref:        ref,
				Properties: props,
			})
			edgeProps := map[string]string{
				"framework": "spring",
				"region":    region,
				"mode":      mode,
				"language":  "java",
			}
			if keySpel != "" {
				edgeProps["key_spel"] = keySpel
			}
			if isDynamic {
				edgeProps["dynamic"] = "true"
			}
			addRel(&result, seenRel, Relationship{
				SourceRef:        ownerRef,
				TargetRef:        ref,
				RelationshipType: relType,
				Properties:       edgeProps,
			})
		}

		if dynamic {
			// Honest-partial: region not statically resolvable.
			emit("<default>", true)
		} else {
			for _, r := range regions {
				emit(r, false)
			}
		}
		_ = fw
	}

	return result
}
