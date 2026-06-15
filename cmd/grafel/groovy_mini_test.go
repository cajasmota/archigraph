package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cajasmota/grafel/internal/resolve"
)

// TestGroovyGrailsDynamic verifies that Grails controller DSL call stubs
// (findResource, bindData, setRollbackOnly) emitted by the Groovy extractor
// are classified as DispositionDynamic rather than DispositionBugExtractor
// after adding groovyDynamicPatterns + groovyAllPatterns (issue #44).
func TestGroovyGrailsDynamic(t *testing.T) {
	fixtureDir := filepath.Join("../../internal/quality/golden/groovy-grails-mini/src")
	abs, err := filepath.Abs(fixtureDir)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, serr := filepath.EvalSymlinks(abs); serr != nil {
		t.Skipf("groovy-grails-mini fixture not found at %s", abs)
	}

	idx := newTestIndexer(t, "groovy-grails-mini", nil, "")
	doc, err := idx.Run(context.Background(), abs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if doc == nil {
		t.Fatal("Run returned nil document")
	}

	bugCount := idx.finalDispositions.DispositionCounts[resolve.DispositionBugExtractor]
	dynCount := idx.finalDispositions.DispositionCounts[resolve.DispositionDynamic]

	// With the fix: Grails DSL stubs (findResource, bindData, setRollbackOnly,
	// count) must move from bug-extractor to dynamic.
	// The fixture has 3 × findResource, 2 × bindData, 2 × setRollbackOnly,
	// 1 × count, 1 × GrailsApp.run = 9 stubs that must become dynamic.
	// Pre-fix baseline: these 9 stubs are in bug-extractor.
	if dynCount == 0 {
		t.Errorf("dynamic count = 0, expected > 0 after groovyDynamicPatterns fix; bug-extractor = %d; samples = %v",
			bugCount, idx.finalDispositions.DispositionSamples[resolve.DispositionBugExtractor])
	}
	if bugCount >= 18 {
		samples := idx.finalDispositions.DispositionSamples[resolve.DispositionBugExtractor]
		t.Errorf("bug-extractor count = %d (want < 18, pre-fix baseline); dynamic = %d; samples = %v",
			bugCount, dynCount, samples)
	}
	t.Logf("post-fix: bug-extractor=%d dynamic=%d", bugCount, dynCount)
}
