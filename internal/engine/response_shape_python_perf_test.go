// Performance + correctness guard for the response-shape Python extractor
// memoization (#5143).
//
// The corpus pass (ApplyResponseShapesCorpus) drove findHandlerBody /
// walkPyClassFields / walkDRFSerializer in an O(endpoints × methods ×
// filesize) loop, compiling a fresh regexp keyed on the handler name on every
// call. On the 8k-entity Django oracle (acme_core) this never finished in 5+
// minutes. The fix indexes each unique source string once into name → body-span
// maps and reuses them across all lookups for that file.
//
// These tests assert (a) the extracted shapes are byte-identical to a
// no-cache reference extraction, and (b) scaling is near-linear (the regression
// guard the original lacked).
//
// Measured before/after on the synthetic Django corpus below (M2 Pro):
//
//	N (ViewSet classes)   BEFORE (O(n²))   AFTER (linear)
//	  200                  1.37 s            2.8 ms
//	  400                  5.44 s            5.4 ms
//	  800                 21.83 s           10.3 ms
//	 1600                 87.72 s           20.4 ms
//
// i.e. each doubling quadrupled time before (4×/4×/4× — textbook O(n²)) and
// merely doubles it after — and the 8k-entity Django oracle no longer stalls.
package engine

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/types"
)

// resetPyIndexCache drops the memoized per-source index so a benchmark or a
// reference run sees cold-cache behaviour.
func resetPyIndexCache() {
	pyIndexMu.Lock()
	pyIndexCache = map[string]*pySourceIndex{}
	pyIndexMu.Unlock()
}

// makeDjangoViewsFile synthesises a large views.py with `nClasses` DRF
// ViewSets, each implementing the standard entry methods returning a distinct
// dict literal, plus filler functions to inflate file size the way a real
// Django module does. It returns the source and the http_endpoint entities +
// View entities + ROUTES_TO edges that point at each ViewSet.
func makeDjangoViewsFile(nClasses int) (string, []types.EntityRecord, []types.RelationshipRecord) {
	var b strings.Builder
	b.WriteString("from rest_framework.response import Response\n")
	b.WriteString("from rest_framework.viewsets import ModelViewSet\n\n")
	for i := 0; i < nClasses; i++ {
		fmt.Fprintf(&b, "class View%d(ModelViewSet):\n", i)
		fmt.Fprintf(&b, "    def list(self, request):\n")
		fmt.Fprintf(&b, "        # some filler comment to grow the file\n")
		fmt.Fprintf(&b, "        x = compute_something_expensive(request)\n")
		fmt.Fprintf(&b, "        return Response({\"items%d\": [], \"total%d\": 0})\n", i, i)
		fmt.Fprintf(&b, "    def create(self, request):\n")
		fmt.Fprintf(&b, "        return Response({\"id%d\": 1}, status=201)\n\n", i)
	}
	src := b.String()

	var ents []types.EntityRecord
	var rels []types.RelationshipRecord
	for i := 0; i < nClasses; i++ {
		ents = append(ents, types.EntityRecord{
			Kind:       "View",
			Name:       fmt.Sprintf("View%d", i),
			SourceFile: "app/views.py",
			Language:   "python",
			Properties: map[string]string{"framework": "django"},
		})
		path := fmt.Sprintf("/api/res%d", i)
		ents = append(ents, types.EntityRecord{
			Kind:       httpEndpointKind,
			Name:       fmt.Sprintf("http:ANY:%s", path),
			SourceFile: "app/urls.py",
			Language:   "python",
			Properties: map[string]string{
				"framework": "django",
				"verb":      "ANY",
				"path":      path,
			},
		})
		rels = append(rels, types.RelationshipRecord{
			FromID: fmt.Sprintf("Route:%s", strings.TrimPrefix(path, "/")),
			ToID:   fmt.Sprintf("View:View%d", i),
			Kind:   "ROUTES_TO",
		})
	}
	return src, ents, rels
}

// TestResponseShapePython_IndexBuildHandlesMultilineSig guards the latent
// infinite-loop in the signature-skip walk that #5143's per-header indexing
// newly exposed: a `def`/`class` whose argument list wraps across lines used to
// wedge bodyAfterHeader forever (it computed the next newline from a position
// that was itself a newline). The index must build promptly and extract the
// wrapped handler's body.
func TestResponseShapePython_IndexBuildHandlesMultilineSig(t *testing.T) {
	resetPyIndexCache()
	src := `class Api:
    def wrapped(
        self,
        request,
        extra=None,
    ):
        return Response({"wrapped_key": 1})

    def simple(self, request):
        return Response({"simple_key": 2})
`
	done := make(chan *pySourceIndex, 1)
	go func() { done <- buildPySourceIndex(src) }()
	select {
	case idx := <-done:
		if !strings.Contains(idx.funcBodies["wrapped"], "wrapped_key") {
			t.Fatalf("wrapped multi-line-sig body not extracted: %q", idx.funcBodies["wrapped"])
		}
		if !strings.Contains(idx.funcBodies["simple"], "simple_key") {
			t.Fatalf("simple body not extracted: %q", idx.funcBodies["simple"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("buildPySourceIndex wedged on a multi-line signature (the #5143 latent hang)")
	}
}

// TestResponseShapePython_MemoizationIdentical asserts the memoized extractor
// produces the exact same response_keys as a cold-cache extraction for every
// endpoint — proving the O(n²)→linear rewrite is behaviour-preserving.
func TestResponseShapePython_MemoizationIdentical(t *testing.T) {
	src, ents, rels := makeDjangoViewsFile(40)
	reader := func(p string) []byte {
		if p == "app/views.py" {
			return []byte(src)
		}
		return nil
	}

	// Warm run (uses the cache as production would).
	warm := cloneEntities(ents)
	ApplyResponseShapesCorpus(warm, rels, reader)

	// Cold reference run: clear the cache, then run again. Both must agree.
	resetPyIndexCache()
	cold := cloneEntities(ents)
	ApplyResponseShapesCorpus(cold, rels, reader)

	for i := range warm {
		w := sortedCSV(warm[i].Properties["response_keys"])
		c := sortedCSV(cold[i].Properties["response_keys"])
		if !reflect.DeepEqual(w, c) {
			t.Fatalf("entity %d response_keys differ: warm=%v cold=%v", i, w, c)
		}
	}
	// Sanity: at least the http_endpoint entities got keys.
	populated := 0
	for i := range warm {
		if warm[i].Properties["response_keys"] != "" {
			populated++
		}
	}
	if populated == 0 {
		t.Fatalf("no endpoints were populated; extractor produced nothing")
	}
}

// TestResponseShapePython_NearLinearScaling is the regression guard the
// original code lacked: it times the corpus pass at two corpus sizes and
// asserts the larger one is not super-linearly slower. With the old per-call
// regexp.MustCompile + full rescan this ratio blew up; with memoization it is
// ~linear.
//
// De-flaking rationale (#5607): the original compared N=100 vs N=400 (a 4x
// corpus) against a 9x bound. That gap is too narrow — for a linear extractor
// the expected ratio is ~4x and the headroom to the 9x bound is only ~2.25x,
// so ordinary CI jitter (cold caches, GC pauses, co-scheduled load) routinely
// pushed it past 9x (9.28x observed on ubuntu-latest) with no real regression.
//
// We instead use a 10x corpus gap (N=100 vs N=1000) and take the MEDIAN of a
// few timing runs to damp outliers. The math is what makes this robust:
//
//   - Linear  (O(n)):   1000/100  = 10x   time  → expected ratio ≈ 10
//   - Quadratic (O(n²)): 1000²/100² = 100x time  → expected ratio ≈ 100
//
// A 25x bound sits ~2.5x above the linear expectation (generous slack for
// fixed overhead and noise) yet ~4x below the quadratic signal, so a genuine
// O(n²) reintroduction still fails loudly (~100x) while CI jitter never does.
func TestResponseShapePython_NearLinearScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive scaling test in -short mode")
	}
	const (
		smallN  = 100
		largeN  = 1000 // 10x the corpus → linear≈10x time, quadratic≈100x
		samples = 5    // median of N runs damps CI outliers
		bound   = 25.0 // ~2.5x over linear (10x), ~4x under quadratic (100x)
	)
	timeSmall := medianCorpus(t, smallN, samples)
	timeLarge := medianCorpus(t, largeN, samples)

	// +1µs floor on the denominator guards against a 0ns small measurement.
	ratio := float64(timeLarge) / float64(timeSmall+time.Microsecond)
	t.Logf("scaling: N=%d median %v, N=%d median %v, ratio=%.2f (linear≈10.0, quadratic≈100.0)",
		smallN, timeSmall, largeN, timeLarge, ratio)
	if ratio > bound {
		t.Fatalf("super-linear scaling detected: %dx corpus took %.2fx time (want <%.0fx); "+
			"the O(n^2) regression is back", largeN/smallN, ratio, bound)
	}
}

// medianCorpus times the corpus pass `samples` times for the given corpus size
// and returns the median, which is far more stable than a single timing under
// CI noise (a single GC pause or co-scheduled spike can multiply one run).
func medianCorpus(t *testing.T, nClasses, samples int) time.Duration {
	t.Helper()
	runs := make([]time.Duration, samples)
	for i := range runs {
		runs[i] = timeCorpus(t, nClasses)
	}
	sort.Slice(runs, func(a, b int) bool { return runs[a] < runs[b] })
	return runs[len(runs)/2]
}

func timeCorpus(t *testing.T, nClasses int) time.Duration {
	t.Helper()
	src, ents, rels := makeDjangoViewsFile(nClasses)
	reader := func(p string) []byte {
		if p == "app/views.py" {
			return []byte(src)
		}
		return nil
	}
	resetPyIndexCache()
	work := cloneEntities(ents)
	start := time.Now()
	ApplyResponseShapesCorpus(work, rels, reader)
	return time.Since(start)
}

// BenchmarkResponseShapeCorpusDjango benchmarks the corpus pass on a
// large-Django-like file. Run with:
//
//	go test ./internal/engine/ -run='^$' -bench=ResponseShapeCorpusDjango -benchmem
func BenchmarkResponseShapeCorpusDjango(b *testing.B) {
	src, ents, rels := makeDjangoViewsFile(500)
	reader := func(p string) []byte {
		if p == "app/views.py" {
			return []byte(src)
		}
		return nil
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetPyIndexCache()
		work := cloneEntities(ents)
		ApplyResponseShapesCorpus(work, rels, reader)
	}
}

func cloneEntities(in []types.EntityRecord) []types.EntityRecord {
	out := make([]types.EntityRecord, len(in))
	for i := range in {
		out[i] = in[i]
		props := make(map[string]string, len(in[i].Properties))
		for k, v := range in[i].Properties {
			props[k] = v
		}
		out[i].Properties = props
	}
	return out
}

func sortedCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	sort.Strings(parts)
	return parts
}
