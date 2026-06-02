package python_test

// caching_test.go — value-asserting tests for the python_caching extractor
// (#3692, epic #3628, area #18). Asserts the actual cache region and CACHES
// edge, not len>0.

import (
	"context"
	"testing"

	_ "github.com/cajasmota/archigraph/internal/custom/python"
	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

func runPyCaching(t *testing.T, src string) []types.EntityRecord {
	t.Helper()
	e, ok := extractor.Get("python_caching")
	if !ok {
		t.Fatal("python_caching not registered")
	}
	ents, err := e.Extract(context.Background(),
		extractor.FileInput{Path: "svc.py", Language: "python", Content: []byte(src)})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return ents
}

// findCachesEdge returns the CACHES edge whose target ref == wantRef, or nil.
func findCachesEdge(ents []types.EntityRecord, wantRef string) *types.RelationshipRecord {
	for i := range ents {
		for j := range ents[i].Relationships {
			r := &ents[i].Relationships[j]
			if r.Kind == string(types.RelationshipKindCaches) && r.ToID == wantRef {
				return r
			}
		}
	}
	return nil
}

func hasCacheRegion(ents []types.EntityRecord, ref string) bool {
	for _, e := range ents {
		if e.Kind == "SCOPE.Datastore" && e.Subtype == "cache_region" && e.Name != "" {
			// region entities are keyed by ref via their downstream ID; assert by
			// matching the region label embedded in the ref.
			if "cache:"+e.Properties["framework"]+":"+e.Properties["region"] == ref {
				return true
			}
		}
	}
	return false
}

func TestPyCaching_LruCache_InProcess(t *testing.T) {
	src := `
import functools

@functools.lru_cache(maxsize=128)
def fib(n):
    return n
`
	ents := runPyCaching(t, src)
	ref := "cache:lru_cache:fn:fib"
	if !hasCacheRegion(ents, ref) {
		t.Fatalf("expected in-process cache region %q", ref)
	}
	e := findCachesEdge(ents, ref)
	if e == nil {
		t.Fatalf("expected fib CACHES region fn:fib")
	}
	if e.Properties["mode"] != "in_process" {
		t.Errorf("mode = %q, want in_process", e.Properties["mode"])
	}
}

func TestPyCaching_FlaskCaching_KeyPrefix(t *testing.T) {
	src := `
@app.route("/users")
@cache.cached(timeout=60, key_prefix='view/users')
def list_users():
    return query()
`
	ents := runPyCaching(t, src)
	ref := "cache:flask_caching:view/users"
	e := findCachesEdge(ents, ref)
	if e == nil {
		t.Fatalf("expected list_users CACHES region view/users (read-through)")
	}
	if e.Properties["mode"] != "read_through" {
		t.Errorf("mode = %q, want read_through", e.Properties["mode"])
	}
	if e.Properties["dynamic"] == "true" {
		t.Errorf("static key_prefix should not be dynamic")
	}
}

func TestPyCaching_FlaskCaching_NoKeyPrefix_Dynamic(t *testing.T) {
	src := `
@cache.cached(timeout=30)
def home():
    return render()
`
	ents := runPyCaching(t, src)
	ref := "cache:flask_caching:<request_path>"
	e := findCachesEdge(ents, ref)
	if e == nil {
		t.Fatalf("expected honest-partial dynamic CACHES edge")
	}
	if e.Properties["dynamic"] != "true" {
		t.Errorf("missing key_prefix should be dynamic")
	}
}

func TestPyCaching_Cachetools(t *testing.T) {
	src := `
@cached(cache, key=hashkey)
def expensive(x):
    return x
`
	ents := runPyCaching(t, src)
	ref := "cache:cachetools:fn:expensive"
	if findCachesEdge(ents, ref) == nil {
		t.Fatalf("expected expensive CACHES region fn:expensive")
	}
}

// Negative: a plain function with no cache decorator must emit no cache edge.
func TestPyCaching_PlainFunction_NoEdge(t *testing.T) {
	src := `
def get_user(uid):
    return db.query(uid)
`
	ents := runPyCaching(t, src)
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == string(types.RelationshipKindCaches) {
				t.Fatalf("plain function should emit no CACHES edge, got %+v", r)
			}
		}
	}
}
