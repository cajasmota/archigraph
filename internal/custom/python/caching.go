package python

// caching.go — Python caching-decorator topology extraction (#3692, epic
// #3628, area #18).
//
// Sits ON TOP of the raw redis key work (redis.go): instead of low-level
// GET/SET it records the read-through caching INTENT a decorated function
// declares. Three idioms are covered:
//
//	@lru_cache / @cache / @functools.lru_cache  (stdlib, in-process memoisation)
//	    → function CACHES an in-process region keyed by the function's qualname.
//	      mode=in_process. No external key — the cache key is the call args.
//
//	@cache.cached(timeout=60, key_prefix='view/%s')   (Flask-Caching)
//	    → function CACHES region <key_prefix>. mode=read_through.
//	      Missing key_prefix → Flask derives it from the request path → dynamic.
//
//	@cached(cache, key=...)  /  @cachetools.cached(...)   (cachetools)
//	    → function CACHES an in-process region keyed by the function qualname.
//	      A `key=` callable is dynamic; recorded honest-partial.
//
// Entity/edge shape (cross-language consistent with the redis keyspace node and
// the Spring cache-region node):
//
//	target  : SCOPE.Datastore  subtype "cache_region"
//	          Ref  "cache:<framework>:<region>"  (converges sites on one node)
//	carrier : the decorated function operation (owner of the edge)
//	edge    : CACHES   owner → region
//
// Python has no first-class declarative eviction decorator (eviction is an
// imperative `cache.delete(...)` call already modelled as a redis WRITES_TO
// op), so this extractor only emits CACHES; INVALIDATES is owned by the Java
// (@CacheEvict) and Rails (Rails.cache.delete) passes.

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func init() {
	extractor.Register("python_caching", &CachingExtractor{})
}

// CachingExtractor extracts Python caching decorators and the regions they
// populate.
type CachingExtractor struct{}

func (e *CachingExtractor) Language() string { return "python_caching" }

var (
	// cacheLruRe matches @lru_cache / @cache / @functools.lru_cache /
	// @functools.cache (with or without parens) immediately above a def.
	// Group 1 = decorated function name.
	cacheLruRe = regexp.MustCompile(
		`(?m)@(?:functools\.)?(?:lru_cache|cache)\b\s*(?:\([^)]*\))?\s*\n\s*(?:async\s+)?def\s+(\w+)\s*\(`)

	// cacheFlaskRe matches @cache.cached(...) / @<something>.cached(...)
	// (Flask-Caching). Group 1 = the argument body; Group 2 = function name.
	cacheFlaskRe = regexp.MustCompile(
		`(?m)@\w+\.cached\s*\(([^)]*)\)\s*\n\s*(?:async\s+)?def\s+(\w+)\s*\(`)

	// cacheToolsRe matches @cached(...) or @cachetools.cached(...) (cachetools).
	// Group 1 = argument body; Group 2 = function name. The leading `\w+\.`
	// alternation lets it match both the bare and module-qualified forms while
	// NOT overlapping the `\w+.cached` Flask form (handled above) — we
	// post-filter so a `.cached` match is not double-counted.
	cacheToolsRe = regexp.MustCompile(
		`(?m)@(?:cachetools\.)?cached\s*\(([^)]*)\)\s*\n\s*(?:async\s+)?def\s+(\w+)\s*\(`)

	// flaskKeyPrefixRe pulls key_prefix='view/%s' out of a Flask-Caching body.
	// Group 1 = the key-prefix literal body.
	flaskKeyPrefixRe = regexp.MustCompile(`key_prefix\s*=\s*["']([^"']*)["']`)
)

// cacheRegionRef builds the stable target ref so multiple decorated functions
// that share a region converge on one node (mirrors redisKeyspaceRef).
func cacheRegionRef(framework, region string) string {
	return fmt.Sprintf("cache:%s:%s", framework, region)
}

func (e *CachingExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("custom.python_caching")
	_, span := tracer.Start(ctx, "custom.python_caching")
	defer span.End()
	span.SetAttributes(attribute.String("file", file.Path))

	if len(file.Content) == 0 {
		return nil, nil
	}
	source := string(file.Content)
	if !strings.Contains(source, "cache") && !strings.Contains(source, "cached") {
		return nil, nil
	}

	var out []types.EntityRecord
	seenRegion := make(map[string]bool)
	seenOwner := make(map[string]bool)

	// emitRegion adds one SCOPE.Datastore cache-region node per distinct region
	// (deduplicated across the file). Returns the target ref for the edge.
	emitRegion := func(framework, region string, dynamic bool, line int) string {
		ref := cacheRegionRef(framework, region)
		if !seenRegion[ref] {
			seenRegion[ref] = true
			props := map[string]string{
				"framework":  framework,
				"region":     region,
				"cache_kind": "region",
				"language":   "python",
			}
			if dynamic {
				props["dynamic"] = "true"
			}
			out = append(out, entity(ref, "SCOPE.Datastore", "cache_region", file.Path, line, props))
		}
		return ref
	}

	// emitCache appends the cache-region target (if new) and the decorated-
	// function carrier with its CACHES edge already attached. Region is appended
	// BEFORE the owner so the owner is the last element — no live pointer is held
	// across a slice append (which would reallocate and silently drop the edge).
	emitCache := func(framework, fn, region, mode string, dynamic bool, line int) {
		ownerRef := fmt.Sprintf("cache_fn:%s:%s:%d", framework, file.Path, line)
		if seenOwner[ownerRef] {
			return
		}
		seenOwner[ownerRef] = true

		targetRef := emitRegion(framework, region, dynamic, line)

		edgeProps := map[string]string{
			"framework": framework,
			"region":    region,
			"mode":      mode,
			"language":  "python",
		}
		if dynamic {
			edgeProps["dynamic"] = "true"
		}
		owner := entity(ownerRef, "SCOPE.Operation", "cache_method", file.Path, line,
			map[string]string{
				"framework":    framework,
				"cache_mode":   mode,
				"cached_fn":    fn,
				"language":     "python",
				"pattern_type": "cache_decorator",
			})
		owner.Relationships = append(owner.Relationships, types.RelationshipRecord{
			ToID:       targetRef,
			Kind:       string(types.RelationshipKindCaches),
			Properties: edgeProps,
		})
		out = append(out, owner)
	}

	// 1. @lru_cache / @cache — in-process memoisation. Region = function qualname.
	for _, m := range cacheLruRe.FindAllStringSubmatchIndex(source, -1) {
		fn := source[m[2]:m[3]]
		line := lineOf(source, m[0])
		emitCache("lru_cache", fn, "fn:"+fn, "in_process", false, line)
	}

	// 2. @cache.cached(...) — Flask-Caching. Region = key_prefix or dynamic.
	flaskLines := make(map[int]bool)
	for _, m := range cacheFlaskRe.FindAllStringSubmatchIndex(source, -1) {
		body := source[m[2]:m[3]]
		fn := source[m[4]:m[5]]
		line := lineOf(source, m[0])
		flaskLines[line] = true
		region := ""
		dynamic := false
		if km := flaskKeyPrefixRe.FindStringSubmatch(body); km != nil && strings.TrimSpace(km[1]) != "" {
			region = km[1]
		} else {
			region = "<request_path>"
			dynamic = true
		}
		emitCache("flask_caching", fn, region, "read_through", dynamic, line)
	}

	// 3. @cached(...) / @cachetools.cached(...) — cachetools. Region = fn qualname.
	for _, m := range cacheToolsRe.FindAllStringSubmatchIndex(source, -1) {
		line := lineOf(source, m[0])
		if flaskLines[line] {
			continue // already claimed by the Flask `.cached` form.
		}
		fn := source[m[4]:m[5]]
		emitCache("cachetools", fn, "fn:"+fn, "in_process", false, line)
	}

	span.SetAttributes(attribute.Int("entity_count", len(out)))
	return out, nil
}
