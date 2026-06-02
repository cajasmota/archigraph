package java

import "testing"

// caching_test.go — value-asserting tests for ExtractJavaCaching (#3692, epic
// #3628, area #18). Asserts the actual region + CACHES/INVALIDATES edge, not
// len>0.

func runCaching(src string) PatternResult {
	return ExtractJavaCaching(PatternContext{
		Source: src, Language: "java", Framework: "spring_boot", FilePath: "UserService.java",
	})
}

// findCacheRel returns the relationship of kind relType targeting the given
// region ref, or nil.
func findCacheRel(res PatternResult, relType, targetRef string) *Relationship {
	for i := range res.Relationships {
		r := &res.Relationships[i]
		if r.RelationshipType == relType && r.TargetRef == targetRef {
			return r
		}
	}
	return nil
}

func hasRegionEntity(res PatternResult, ref string) bool {
	for _, e := range res.Entities {
		if e.Ref == ref && e.Kind == "SCOPE.Datastore" && e.Subtype == "cache_region" {
			return true
		}
	}
	return false
}

func TestJavaCaching_Cacheable_ReadThrough(t *testing.T) {
	src := `
@Service
class UserService {
    @Cacheable(value="users", key="#id")
    public User getUser(Long id) { return repo.find(id); }
}
`
	res := runCaching(src)
	ref := "cache:spring:users"
	if !hasRegionEntity(res, ref) {
		t.Fatalf("expected cache_region entity %q", ref)
	}
	r := findCacheRel(res, "CACHES", ref)
	if r == nil {
		t.Fatalf("expected getUser CACHES region users")
	}
	if r.Properties["mode"] != "read_through" {
		t.Errorf("mode = %q, want read_through", r.Properties["mode"])
	}
	if r.Properties["key_spel"] != "#id" {
		t.Errorf("key_spel = %q, want #id", r.Properties["key_spel"])
	}
	// Carrier owner must be the method.
	if findCacheRel(res, "CACHES", ref).SourceRef == "" {
		t.Errorf("CACHES edge missing source carrier")
	}
}

func TestJavaCaching_CacheEvict_Invalidates(t *testing.T) {
	src := `
@Service
class UserService {
    @CacheEvict(value="users")
    public void updateUser(User u) { repo.save(u); }
}
`
	res := runCaching(src)
	ref := "cache:spring:users"
	r := findCacheRel(res, "INVALIDATES", ref)
	if r == nil {
		t.Fatalf("expected updateUser INVALIDATES region users")
	}
	if r.Properties["mode"] != "evict" {
		t.Errorf("mode = %q, want evict", r.Properties["mode"])
	}
}

func TestJavaCaching_CacheEvict_AllEntries(t *testing.T) {
	src := `
class UserService {
    @CacheEvict(value="users", allEntries=true)
    public void clear() {}
}
`
	res := runCaching(src)
	r := findCacheRel(res, "INVALIDATES", "cache:spring:users")
	if r == nil || r.Properties["mode"] != "evict_all" {
		t.Fatalf("expected evict_all mode, got %+v", r)
	}
}

func TestJavaCaching_BareRegion_Shorthand(t *testing.T) {
	src := `
class UserService {
    @Cacheable("profiles")
    public Profile get(Long id) { return null; }
}
`
	res := runCaching(src)
	if findCacheRel(res, "CACHES", "cache:spring:profiles") == nil {
		t.Fatalf("expected bare @Cacheable(\"profiles\") to CACHES region profiles")
	}
}

func TestJavaCaching_MultiRegion(t *testing.T) {
	src := `
class UserService {
    @Cacheable(cacheNames={"users","admins"})
    public User get(Long id) { return null; }
}
`
	res := runCaching(src)
	if findCacheRel(res, "CACHES", "cache:spring:users") == nil {
		t.Errorf("expected CACHES region users")
	}
	if findCacheRel(res, "CACHES", "cache:spring:admins") == nil {
		t.Errorf("expected CACHES region admins")
	}
}

func TestJavaCaching_DynamicRegion_HonestPartial(t *testing.T) {
	src := `
class UserService {
    @Cacheable(key="#id")
    public User get(Long id) { return null; }
}
`
	res := runCaching(src)
	r := findCacheRel(res, "CACHES", "cache:spring:<default>")
	if r == nil {
		t.Fatalf("expected honest-partial dynamic CACHES edge")
	}
	if r.Properties["dynamic"] != "true" {
		t.Errorf("region-less annotation should be dynamic")
	}
}

// Negative: a plain method with no cache annotation emits no cache edge.
func TestJavaCaching_PlainMethod_NoEdge(t *testing.T) {
	src := `
@Service
class UserService {
    public User getUser(Long id) { return repo.find(id); }
}
`
	res := runCaching(src)
	for _, r := range res.Relationships {
		if r.RelationshipType == "CACHES" || r.RelationshipType == "INVALIDATES" {
			t.Fatalf("plain method should emit no cache edge, got %+v", r)
		}
	}
}

// Negative: the extractor must reject non-Spring frameworks.
func TestJavaCaching_NonSpring_NoOp(t *testing.T) {
	src := `@Cacheable("users") public User getUser(Long id) { return null; }`
	res := ExtractJavaCaching(PatternContext{
		Source: src, Language: "java", Framework: "quarkus", FilePath: "X.java",
	})
	if len(res.Entities) != 0 || len(res.Relationships) != 0 {
		t.Fatalf("non-Spring framework must be a no-op, got %+v", res)
	}
}
