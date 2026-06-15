// Benchmarks for Phase-D MCP query latency. These compare the cache-
// backed FlatBuffers path (this package) to the legacy graph.json
// re-parse path. The fixture file is configurable via the env var
// GRAFEL_BENCH_FIXTURE_FB (a path to a graph.fb). When unset the
// bench skips so `go test ./...` stays green in CI.
//
// Run with:
//
//	GRAFEL_BENCH_FIXTURE_FB=/path/to/graph.fb \
//	  go test ./internal/daemon/mcp/ -bench=. -benchmem -run=^$ -count=3
//
// The companion script scripts/bench-mcp-latency.sh materializes a
// graph.fb from a fixture repo and invokes this benchmark.

package mcp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
)

func benchFB(tb testing.TB) string {
	p := os.Getenv("GRAFEL_BENCH_FIXTURE_FB")
	if p == "" {
		tb.Skip("GRAFEL_BENCH_FIXTURE_FB not set")
	}
	if _, err := os.Stat(p); err != nil {
		tb.Skipf("fixture %s missing: %v", p, err)
	}
	return p
}

// pickEntityID returns the first non-empty entity id from a graph.fb
// so the bench has a real target without scanning at bench start.
func pickEntityID(tb testing.TB, fbPath string) string {
	c := NewCache(2)
	defer c.Close()
	r, rel, err := c.Get(fbPath)
	if err != nil {
		tb.Fatalf("get: %v", err)
	}
	defer rel()
	if r.EntityCount() == 0 {
		tb.Skip("graph has no entities")
	}
	e := r.EntityAt(0)
	return string(e.Id())
}

func BenchmarkReadEntity_FBCache(b *testing.B) {
	fb := benchFB(b)
	id := pickEntityID(b, fb)
	cache := NewCache(4)
	defer cache.Close()
	q := NewQueryService(cache)
	// Prime so we measure warm-cache latency.
	_, _ = q.ReadEntity(fb, id)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := q.ReadEntity(fb, id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindReferences_FBCache(b *testing.B) {
	fb := benchFB(b)
	id := pickEntityID(b, fb)
	cache := NewCache(4)
	defer cache.Close()
	q := NewQueryService(cache)
	_, _ = q.FindReferences(fb, id)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := q.FindReferences(fb, id); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadEntity_JSONReparse models today's MCP path: re-read +
// re-unmarshal graph.json per call. Requires GRAFEL_BENCH_FIXTURE
// (the matching graph.json sibling). Skips otherwise.
func BenchmarkReadEntity_JSONReparse(b *testing.B) {
	jsonPath := os.Getenv("GRAFEL_BENCH_FIXTURE")
	if jsonPath == "" {
		b.Skip("GRAFEL_BENCH_FIXTURE not set")
	}
	if _, err := os.Stat(jsonPath); err != nil {
		b.Skipf("fixture %s missing: %v", jsonPath, err)
	}
	// Pick a real entity ID from the JSON so the lookup hits.
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		b.Fatal(err)
	}
	var first graph.Document
	if err := json.Unmarshal(data, &first); err != nil {
		b.Fatal(err)
	}
	if len(first.Entities) == 0 {
		b.Skip("graph has no entities")
	}
	targetID := first.Entities[0].ID
	if strings.TrimSpace(targetID) == "" {
		b.Skip("empty target id")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			b.Fatal(err)
		}
		var doc graph.Document
		if err := json.Unmarshal(data, &doc); err != nil {
			b.Fatal(err)
		}
		var found *graph.Entity
		for j := range doc.Entities {
			if doc.Entities[j].ID == targetID {
				found = &doc.Entities[j]
				break
			}
		}
		if found == nil {
			b.Fatal("not found")
		}
	}
}
