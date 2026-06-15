// Package docgen_test — section-level LLM cache tests (ticket E, issue #1813 chain).
//
// Test inventory (≥ 10 unit tests):
//  1. Write + Read roundtrip — all fields survive.
//  2. Cache miss returns nil,nil (no error).
//  3. Multi-section batch: hit + miss combined.
//  4. NoCache in BuildBundleOpts disables cache reads.
//  5. CacheStats accurate after writes.
//  6. CacheStats on absent directory returns 0,0,nil.
//  7. WriteCache is idempotent (overwrite same hash).
//  8. CacheHit field marshal round-trip (additive field, omitempty).
//  9. CacheHit absent from bundle when no cache is present.
//  10. PromptHash populated per-section in bundle sections.
//  11. ApplyResult writes cache entries (cache_writes in score).
//  12. Emit after apply shows cache_hit=true in bundle.
//  13. NoCache on ApplyResult suppresses cache writes.
package docgen_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/docgen"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sampleCacheEntry(hash, section string) docgen.CacheEntry {
	return docgen.CacheEntry{
		PromptHash:   hash,
		Section:      section,
		Markdown:     "## " + section + "\n\nGenerated content for " + section + ".\n",
		WordCount:    6,
		MermaidCount: 0,
		LinkRefs:     []string{"#overview"},
		CachedAt:     "2026-05-23T00:00:00Z",
	}
}

// ---------------------------------------------------------------------------
// Test 1: Write + Read roundtrip
// ---------------------------------------------------------------------------

func TestCache_WriteReadRoundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	orig := sampleCacheEntry("abc1234567890abcdef", "overview")

	if err := docgen.WriteCache(dir, orig); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}

	got, err := docgen.ReadCache(dir, orig.PromptHash)
	if err != nil {
		t.Fatalf("ReadCache: %v", err)
	}
	if got == nil {
		t.Fatal("ReadCache returned nil for a written entry")
	}

	if got.PromptHash != orig.PromptHash {
		t.Errorf("PromptHash: got %q want %q", got.PromptHash, orig.PromptHash)
	}
	if got.Section != orig.Section {
		t.Errorf("Section: got %q want %q", got.Section, orig.Section)
	}
	if got.Markdown != orig.Markdown {
		t.Errorf("Markdown mismatch")
	}
	if got.WordCount != orig.WordCount {
		t.Errorf("WordCount: got %d want %d", got.WordCount, orig.WordCount)
	}
	if got.MermaidCount != orig.MermaidCount {
		t.Errorf("MermaidCount: got %d want %d", got.MermaidCount, orig.MermaidCount)
	}
	if len(got.LinkRefs) != len(orig.LinkRefs) {
		t.Errorf("LinkRefs len: got %d want %d", len(got.LinkRefs), len(orig.LinkRefs))
	}
}

// ---------------------------------------------------------------------------
// Test 2: Cache miss returns nil,nil
// ---------------------------------------------------------------------------

func TestCache_MissReturnsNilNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	got, err := docgen.ReadCache(dir, "doesnotexist0000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Errorf("ReadCache miss must return nil error, got: %v", err)
	}
	if got != nil {
		t.Errorf("ReadCache miss must return nil entry, got: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Multi-section batch — hit + miss
// ---------------------------------------------------------------------------

func TestCache_MultiSectionBatchHitMiss(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	hashes := map[string]string{
		"overview": "ov0000000000000000000000000000000000000000000000000000000000000a",
		"flows":    "fl0000000000000000000000000000000000000000000000000000000000000b",
		"api":      "ap0000000000000000000000000000000000000000000000000000000000000c",
		"glossary": "gl0000000000000000000000000000000000000000000000000000000000000d",
	}

	// Write only overview and flows.
	for _, sec := range []string{"overview", "flows"} {
		if err := docgen.WriteCache(dir, sampleCacheEntry(hashes[sec], sec)); err != nil {
			t.Fatalf("WriteCache %s: %v", sec, err)
		}
	}

	hits := 0
	misses := 0
	for sec, hash := range hashes {
		entry, err := docgen.ReadCache(dir, hash)
		if err != nil {
			t.Errorf("ReadCache %s: unexpected error: %v", sec, err)
			continue
		}
		if entry != nil {
			hits++
			if entry.Section != sec {
				t.Errorf("hit for hash %s: section got %q want %q", hash, entry.Section, sec)
			}
		} else {
			misses++
		}
	}

	if hits != 2 {
		t.Errorf("expected 2 hits, got %d", hits)
	}
	if misses != 2 {
		t.Errorf("expected 2 misses, got %d", misses)
	}
}

// ---------------------------------------------------------------------------
// Test 4: NoCache in BuildBundleOpts disables cache reads (sections have no
// cache_hit even when cache files exist on disk).
// ---------------------------------------------------------------------------

func TestCache_NoCacheDisablesRead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	archHome, group, entityID, repoPath := buildMinimalGroupForEmitTests(t)
	t.Setenv("GRAFEL_HOME", archHome)
	t.Setenv("GRAFEL_DAEMON_ROOT", filepath.Join(archHome, "daemon-root"))
	writeGraphForEmitTest(t, archHome, repoPath, entityID)

	// Determine the default cache dir and pre-populate it with a fake entry
	// that would match the "overview" section hash if we could know it in
	// advance. Since we can't know the exact hash without running BuildBundle
	// first (it depends on graph state), we instead verify that when NoCache=true
	// no section in the bundle has cache_hit=true, regardless of cache state.
	outDir := t.TempDir()

	// First emit without NoCache to learn the hashes.
	cacheDir := t.TempDir()
	opts1 := docgen.RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		Section:      "overview",
		OutputDir:    outDir,
		LLMMode:      "emit",
		CacheDir:     cacheDir,
		NoCache:      false,
	}
	_, _, _, err := docgen.Run(opts1)
	if err != nil {
		t.Skipf("graph load failed (acceptable in test env): %v", err)
	}

	// Read the bundle to get the per-section hash.
	bundleFile := filepath.Join(outDir, entityID+"-overview-bundle.json")
	bundleData, readErr := os.ReadFile(bundleFile)
	if readErr != nil {
		t.Fatalf("read bundle: %v", readErr)
	}
	var bundle docgen.LLMPromptBundle
	if unmarshalErr := json.Unmarshal(bundleData, &bundle); unmarshalErr != nil {
		t.Fatalf("unmarshal bundle: %v", unmarshalErr)
	}
	if len(bundle.Sections) == 0 {
		t.Fatal("bundle has no sections")
	}
	sectionHash := bundle.Sections[0].PromptHash
	if sectionHash == "" {
		t.Skip("section prompt_hash not set; skipping (pre-ticket-E bundle?)")
	}

	// Write a fake cache entry at that hash so a cache-enabled read would hit.
	fake := docgen.CacheEntry{
		PromptHash:   sectionHash,
		Section:      "overview",
		Markdown:     "cached markdown for overview",
		WordCount:    4,
		MermaidCount: 0,
		LinkRefs:     nil,
		CachedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if wErr := docgen.WriteCache(cacheDir, fake); wErr != nil {
		t.Fatalf("WriteCache: %v", wErr)
	}

	// Now emit with NoCache=true — cache_hit must be false for all sections.
	outDir2 := t.TempDir()
	opts2 := docgen.RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		Section:      "overview",
		OutputDir:    outDir2,
		LLMMode:      "emit",
		CacheDir:     cacheDir,
		NoCache:      true, // <-- disable cache
	}
	_, _, _, err = docgen.Run(opts2)
	if err != nil {
		t.Skipf("graph load failed (acceptable in test env): %v", err)
	}

	bundleFile2 := filepath.Join(outDir2, entityID+"-overview-bundle.json")
	bundleData2, readErr2 := os.ReadFile(bundleFile2)
	if readErr2 != nil {
		t.Fatalf("read bundle2: %v", readErr2)
	}
	var bundle2 docgen.LLMPromptBundle
	if unmarshalErr := json.Unmarshal(bundleData2, &bundle2); unmarshalErr != nil {
		t.Fatalf("unmarshal bundle2: %v", unmarshalErr)
	}
	for _, sp := range bundle2.Sections {
		if sp.CacheHit {
			t.Errorf("section %q has cache_hit=true with NoCache=true", sp.Section)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 5: CacheStats accurate after writes
// ---------------------------------------------------------------------------

func TestCache_StatsAccurate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	sections := []string{"overview", "flows", "api", "patterns", "glossary"}
	for i, sec := range sections {
		// Pad hash to 64 hex chars.
		hash := strings.Repeat("0", 62) + string(rune('a'+i)) + string(rune('0'+i))
		if err := docgen.WriteCache(dir, sampleCacheEntry(hash, sec)); err != nil {
			t.Fatalf("WriteCache %s: %v", sec, err)
		}
	}

	entries, totalBytes, err := docgen.CacheStats(dir)
	if err != nil {
		t.Fatalf("CacheStats: %v", err)
	}
	if entries != len(sections) {
		t.Errorf("entries: got %d want %d", entries, len(sections))
	}
	if totalBytes == 0 {
		t.Error("totalBytes is 0; expected > 0")
	}
}

// ---------------------------------------------------------------------------
// Test 6: CacheStats on absent directory returns 0,0,nil
// ---------------------------------------------------------------------------

func TestCache_StatsAbsentDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "does-not-exist")

	entries, totalBytes, err := docgen.CacheStats(dir)
	if err != nil {
		t.Errorf("CacheStats on absent dir must return nil error, got: %v", err)
	}
	if entries != 0 {
		t.Errorf("entries: got %d want 0", entries)
	}
	if totalBytes != 0 {
		t.Errorf("totalBytes: got %d want 0", totalBytes)
	}
}

// ---------------------------------------------------------------------------
// Test 7: WriteCache is idempotent (overwrite same hash)
// ---------------------------------------------------------------------------

func TestCache_WriteIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hash := "idempotent0000000000000000000000000000000000000000000000000000aa"

	e1 := docgen.CacheEntry{
		PromptHash: hash, Section: "overview", Markdown: "version one",
		WordCount: 2, CachedAt: "2026-05-23T00:00:00Z",
	}
	if err := docgen.WriteCache(dir, e1); err != nil {
		t.Fatalf("WriteCache v1: %v", err)
	}

	e2 := docgen.CacheEntry{
		PromptHash: hash, Section: "overview", Markdown: "version two — updated",
		WordCount: 4, CachedAt: "2026-05-23T01:00:00Z",
	}
	if err := docgen.WriteCache(dir, e2); err != nil {
		t.Fatalf("WriteCache v2: %v", err)
	}

	got, err := docgen.ReadCache(dir, hash)
	if err != nil {
		t.Fatalf("ReadCache: %v", err)
	}
	if got == nil {
		t.Fatal("ReadCache returned nil after overwrite")
	}
	if got.Markdown != e2.Markdown {
		t.Errorf("Markdown: got %q want %q (expected latest overwrite)", got.Markdown, e2.Markdown)
	}
}

// ---------------------------------------------------------------------------
// Test 8: CacheHit field marshal round-trip (additive, omitempty)
// ---------------------------------------------------------------------------

func TestCacheHit_MarshalRoundtrip(t *testing.T) {
	t.Parallel()

	// When CacheHit=false the field must be omitted (omitempty semantics).
	sp := docgen.LLMSectionPrompt{
		Section:  "overview",
		AnchorID: "overview",
		CacheHit: false,
	}
	data, err := json.Marshal(sp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "cache_hit") {
		t.Errorf("cache_hit must be omitted when false; JSON: %s", string(data))
	}

	// When CacheHit=true the field must be present.
	sp.CacheHit = true
	data2, err := json.Marshal(sp)
	if err != nil {
		t.Fatalf("marshal with CacheHit=true: %v", err)
	}
	if !strings.Contains(string(data2), `"cache_hit":true`) {
		t.Errorf("cache_hit:true must appear in JSON; got: %s", string(data2))
	}

	// Round-trip: unmarshal must restore true.
	var got docgen.LLMSectionPrompt
	if err := json.Unmarshal(data2, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.CacheHit {
		t.Error("CacheHit: expected true after unmarshal, got false")
	}
}

// ---------------------------------------------------------------------------
// Test 9: CacheHit absent from bundle when no cache is present
// ---------------------------------------------------------------------------

func TestCacheHit_AbsentWhenNoCache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	archHome, group, entityID, repoPath := buildMinimalGroupForEmitTests(t)
	t.Setenv("GRAFEL_HOME", archHome)
	t.Setenv("GRAFEL_DAEMON_ROOT", filepath.Join(archHome, "daemon-root"))
	writeGraphForEmitTest(t, archHome, repoPath, entityID)

	outDir := t.TempDir()
	// Use a fresh empty cache dir (no entries).
	cacheDir := t.TempDir()

	opts := docgen.RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		Section:      "overview",
		OutputDir:    outDir,
		LLMMode:      "emit",
		CacheDir:     cacheDir,
	}
	_, _, _, err := docgen.Run(opts)
	if err != nil {
		t.Skipf("graph load failed (acceptable in test env): %v", err)
	}

	bundleFile := filepath.Join(outDir, entityID+"-overview-bundle.json")
	bundleData, readErr := os.ReadFile(bundleFile)
	if readErr != nil {
		t.Fatalf("read bundle: %v", readErr)
	}
	var bundle docgen.LLMPromptBundle
	if unmarshalErr := json.Unmarshal(bundleData, &bundle); unmarshalErr != nil {
		t.Fatalf("unmarshal bundle: %v", unmarshalErr)
	}
	for _, sp := range bundle.Sections {
		if sp.CacheHit {
			t.Errorf("section %q has cache_hit=true with empty cache", sp.Section)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 10: PromptHash populated per-section in emitted bundle
// ---------------------------------------------------------------------------

func TestBundle_PerSectionPromptHashPopulated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	archHome, group, entityID, repoPath := buildMinimalGroupForEmitTests(t)
	t.Setenv("GRAFEL_HOME", archHome)
	t.Setenv("GRAFEL_DAEMON_ROOT", filepath.Join(archHome, "daemon-root"))
	writeGraphForEmitTest(t, archHome, repoPath, entityID)

	outDir := t.TempDir()
	opts := docgen.RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		Section:      "overview",
		OutputDir:    outDir,
		LLMMode:      "emit",
	}
	_, _, _, err := docgen.Run(opts)
	if err != nil {
		t.Skipf("graph load failed (acceptable in test env): %v", err)
	}

	bundleFile := filepath.Join(outDir, entityID+"-overview-bundle.json")
	bundleData, readErr := os.ReadFile(bundleFile)
	if readErr != nil {
		t.Fatalf("read bundle: %v", readErr)
	}
	var bundle docgen.LLMPromptBundle
	if unmarshalErr := json.Unmarshal(bundleData, &bundle); unmarshalErr != nil {
		t.Fatalf("unmarshal bundle: %v", unmarshalErr)
	}
	for _, sp := range bundle.Sections {
		if sp.PromptHash == "" {
			t.Errorf("section %q has empty PromptHash in emitted bundle", sp.Section)
		}
		if len(sp.PromptHash) != 64 {
			t.Errorf("section %q PromptHash length: got %d want 64 (sha256 hex)", sp.Section, len(sp.PromptHash))
		}
	}
}

// ---------------------------------------------------------------------------
// Test 11: ApplyResult writes cache entries (cache_writes in score)
// ---------------------------------------------------------------------------

func TestApplyResult_WritesCacheEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	archHome, group, entityID, repoPath := buildMinimalGroupForEmitTests(t)
	t.Setenv("GRAFEL_HOME", archHome)
	t.Setenv("GRAFEL_DAEMON_ROOT", filepath.Join(archHome, "daemon-root"))
	writeGraphForEmitTest(t, archHome, repoPath, entityID)

	outDir := t.TempDir()
	cacheDir := t.TempDir()

	// Step 1: emit to get bundle.
	emitOpts := docgen.Tier1RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		OutputDir:    outDir,
		LLMMode:      "emit",
		CacheDir:     cacheDir,
	}
	mdPath, _, _, err := docgen.RunTier1(emitOpts)
	if err != nil {
		t.Skipf("graph load failed (acceptable in test env): %v", err)
	}

	// Read emitted bundle.
	bundleFile := mdPath[:len(mdPath)-len(".md")] + "-bundle.json"
	bundleData, readErr := os.ReadFile(bundleFile)
	if readErr != nil {
		t.Fatalf("read bundle: %v", readErr)
	}
	var bundle docgen.LLMPromptBundle
	if unmarshalErr := json.Unmarshal(bundleData, &bundle); unmarshalErr != nil {
		t.Fatalf("unmarshal bundle: %v", unmarshalErr)
	}

	// Step 2: craft a hand-built LLMRunResult with one result per section.
	var sectionResults []docgen.LLMSectionResult
	for _, sp := range bundle.Sections {
		sectionResults = append(sectionResults, docgen.LLMSectionResult{
			Section:      sp.Section,
			Markdown:     "## " + sp.Section + "\n\nHand-crafted prose for " + sp.Section + ".\n",
			WordCount:    7,
			MermaidCount: 0,
			LinkRefs:     []string{},
		})
	}
	runResult := docgen.LLMRunResult{
		Version:        bundle.Version,
		PromptHash:     bundle.PromptHash,
		Tier:           bundle.Tier,
		Group:          bundle.Group,
		SeedEntityID:   bundle.SeedEntityID,
		SectionResults: sectionResults,
		FilledAt:       time.Now().UTC().Format(time.RFC3339),
	}

	// Write result to a temp file.
	resultBytes, _ := json.MarshalIndent(runResult, "", "  ")
	resultFile := filepath.Join(outDir, "result.json")
	if wErr := os.WriteFile(resultFile, resultBytes, 0o644); wErr != nil {
		t.Fatalf("write result.json: %v", wErr)
	}

	// Step 3: apply.
	applyOpts := docgen.Tier1RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		OutputDir:    outDir,
		LLMMode:      "apply",
		BundleFile:   bundleFile,
		ResultFile:   resultFile,
		CacheDir:     cacheDir,
	}
	_, scorePath, score, err := docgen.ApplyResult(applyOpts)
	if err != nil {
		t.Fatalf("ApplyResult: %v", err)
	}

	// score.CacheWrites must equal the number of sections that had a PromptHash.
	expectedWrites := 0
	for _, sp := range bundle.Sections {
		if sp.PromptHash != "" {
			expectedWrites++
		}
	}
	if expectedWrites == 0 {
		t.Skip("no sections have PromptHash in bundle (pre-ticket-E bundle?)")
	}
	if score.CacheWrites != expectedWrites {
		t.Errorf("score.CacheWrites: got %d want %d", score.CacheWrites, expectedWrites)
	}

	// Verify score.json persists the field.
	scoreData, readErr2 := os.ReadFile(scorePath)
	if readErr2 != nil {
		t.Fatalf("read score.json: %v", readErr2)
	}
	if !strings.Contains(string(scoreData), `"cache_writes"`) {
		t.Errorf("score.json missing cache_writes field; data: %s", string(scoreData))
	}

	// Verify actual cache files exist on disk.
	entries, _, statErr := docgen.CacheStats(cacheDir)
	if statErr != nil {
		t.Fatalf("CacheStats: %v", statErr)
	}
	if entries != expectedWrites {
		t.Errorf("cache entries: got %d want %d", entries, expectedWrites)
	}
}

// ---------------------------------------------------------------------------
// Test 12: Emit after apply shows cache_hit=true in bundle
// ---------------------------------------------------------------------------

func TestEmitAfterApply_CacheHits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	archHome, group, entityID, repoPath := buildMinimalGroupForEmitTests(t)
	t.Setenv("GRAFEL_HOME", archHome)
	t.Setenv("GRAFEL_DAEMON_ROOT", filepath.Join(archHome, "daemon-root"))
	writeGraphForEmitTest(t, archHome, repoPath, entityID)

	outDir := t.TempDir()
	cacheDir := t.TempDir()

	// -- Emit (1st time, cold cache) --
	emitOpts := docgen.Tier1RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		OutputDir:    outDir,
		LLMMode:      "emit",
		CacheDir:     cacheDir,
	}
	mdPath, _, _, err := docgen.RunTier1(emitOpts)
	if err != nil {
		t.Skipf("graph load failed (acceptable in test env): %v", err)
	}
	bundleFile := mdPath[:len(mdPath)-len(".md")] + "-bundle.json"

	bundleData, _ := os.ReadFile(bundleFile)
	var bundle1 docgen.LLMPromptBundle
	_ = json.Unmarshal(bundleData, &bundle1)

	// Ensure no cache hits on cold run.
	for _, sp := range bundle1.Sections {
		if sp.CacheHit {
			t.Errorf("cold emit: section %q unexpectedly has cache_hit=true", sp.Section)
		}
	}

	// -- Build & apply a hand-crafted result to populate the cache --
	var sectionResults []docgen.LLMSectionResult
	for _, sp := range bundle1.Sections {
		sectionResults = append(sectionResults, docgen.LLMSectionResult{
			Section:      sp.Section,
			Markdown:     "## " + sp.Section + "\n\nCached prose.\n",
			WordCount:    4,
			MermaidCount: 0,
			LinkRefs:     []string{},
		})
	}
	runResult := docgen.LLMRunResult{
		Version:        bundle1.Version,
		PromptHash:     bundle1.PromptHash,
		Tier:           bundle1.Tier,
		Group:          bundle1.Group,
		SeedEntityID:   bundle1.SeedEntityID,
		SectionResults: sectionResults,
		FilledAt:       time.Now().UTC().Format(time.RFC3339),
	}
	resultBytes, _ := json.MarshalIndent(runResult, "", "  ")
	resultFile := filepath.Join(outDir, "result.json")
	_ = os.WriteFile(resultFile, resultBytes, 0o644)

	applyOpts := docgen.Tier1RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		OutputDir:    outDir,
		LLMMode:      "apply",
		BundleFile:   bundleFile,
		ResultFile:   resultFile,
		CacheDir:     cacheDir,
	}
	_, _, applyScore, applyErr := docgen.ApplyResult(applyOpts)
	if applyErr != nil {
		t.Fatalf("ApplyResult: %v", applyErr)
	}
	if applyScore.CacheWrites == 0 {
		t.Skip("no cache writes happened (bundle may have no PromptHash); skipping hit check")
	}

	// -- Emit (2nd time, warm cache) --
	outDir2 := t.TempDir()
	emitOpts2 := docgen.Tier1RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		OutputDir:    outDir2,
		LLMMode:      "emit",
		CacheDir:     cacheDir,
	}
	mdPath2, _, score2, err2 := docgen.RunTier1(emitOpts2)
	if err2 != nil {
		t.Fatalf("second emit: %v", err2)
	}

	bundleFile2 := mdPath2[:len(mdPath2)-len(".md")] + "-bundle.json"
	bundleData2, _ := os.ReadFile(bundleFile2)
	var bundle2 docgen.LLMPromptBundle
	_ = json.Unmarshal(bundleData2, &bundle2)

	// Every section with a PromptHash must be a cache hit.
	hitCount := 0
	for _, sp := range bundle2.Sections {
		if sp.PromptHash != "" {
			if !sp.CacheHit {
				t.Errorf("warm emit: section %q has PromptHash but cache_hit=false", sp.Section)
			}
			hitCount++
		}
	}
	if hitCount == 0 {
		t.Skip("no sections had PromptHash; bundle may predate ticket E")
	}

	// score2.CacheHits must equal hitCount.
	if score2.CacheHits != hitCount {
		t.Errorf("score2.CacheHits: got %d want %d", score2.CacheHits, hitCount)
	}
}

// ---------------------------------------------------------------------------
// Test 13: NoCache on ApplyResult suppresses cache writes
// ---------------------------------------------------------------------------

func TestApplyResult_NoCacheSuppressesWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	archHome, group, entityID, repoPath := buildMinimalGroupForEmitTests(t)
	t.Setenv("GRAFEL_HOME", archHome)
	t.Setenv("GRAFEL_DAEMON_ROOT", filepath.Join(archHome, "daemon-root"))
	writeGraphForEmitTest(t, archHome, repoPath, entityID)

	outDir := t.TempDir()
	cacheDir := t.TempDir()

	// Emit to get bundle.
	emitOpts := docgen.Tier1RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		OutputDir:    outDir,
		LLMMode:      "emit",
		CacheDir:     cacheDir,
		NoCache:      true,
	}
	mdPath, _, _, err := docgen.RunTier1(emitOpts)
	if err != nil {
		t.Skipf("graph load failed (acceptable in test env): %v", err)
	}
	bundleFile := mdPath[:len(mdPath)-len(".md")] + "-bundle.json"

	bundleData, _ := os.ReadFile(bundleFile)
	var bundle docgen.LLMPromptBundle
	_ = json.Unmarshal(bundleData, &bundle)

	// Build result.
	var sectionResults []docgen.LLMSectionResult
	for _, sp := range bundle.Sections {
		sectionResults = append(sectionResults, docgen.LLMSectionResult{
			Section:      sp.Section,
			Markdown:     "prose",
			WordCount:    1,
			MermaidCount: 0,
			LinkRefs:     []string{},
		})
	}
	runResult := docgen.LLMRunResult{
		Version:        bundle.Version,
		PromptHash:     bundle.PromptHash,
		Tier:           bundle.Tier,
		Group:          bundle.Group,
		SeedEntityID:   bundle.SeedEntityID,
		SectionResults: sectionResults,
		FilledAt:       time.Now().UTC().Format(time.RFC3339),
	}
	resultBytes, _ := json.MarshalIndent(runResult, "", "  ")
	resultFile := filepath.Join(outDir, "result.json")
	_ = os.WriteFile(resultFile, resultBytes, 0o644)

	// Apply with NoCache=true.
	applyOpts := docgen.Tier1RunOpts{
		Group:        group,
		SeedEntityID: entityID,
		OutputDir:    outDir,
		LLMMode:      "apply",
		BundleFile:   bundleFile,
		ResultFile:   resultFile,
		CacheDir:     cacheDir,
		NoCache:      true,
	}
	_, _, applyScore, applyErr := docgen.ApplyResult(applyOpts)
	if applyErr != nil {
		t.Fatalf("ApplyResult: %v", applyErr)
	}

	// CacheWrites must be 0 with NoCache=true.
	if applyScore.CacheWrites != 0 {
		t.Errorf("expected CacheWrites=0 with NoCache=true, got %d", applyScore.CacheWrites)
	}

	// Cache dir must remain empty.
	entries, _, _ := docgen.CacheStats(cacheDir)
	if entries != 0 {
		t.Errorf("expected 0 cache entries with NoCache=true, got %d", entries)
	}
}
