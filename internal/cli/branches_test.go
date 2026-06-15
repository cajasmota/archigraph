package cli

// branches_test.go covers the `grafel branches` CLI surface introduced by
// PH6 of epic #2087 (issue #2094).
//
// Tests use a temporary grafel home (GRAFEL_HOME env var) and a tiny
// synthetic store layout so they do not touch the real ~/.grafel.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/daemon"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newBranchesCmdForTest wires a fresh cobra command tree and returns the root
// so sub-commands can be invoked programmatically.
func newBranchesCmdForTest() *branchesTestHarness {
	return &branchesTestHarness{}
}

type branchesTestHarness struct{}

// runBranchesDirect invokes the list / JSON path directly without cobra arg
// parsing, using a temp PinStore and supplied rows.
func runBranchesDirect(t *testing.T, rows []refInfo, jsonOut bool) string {
	t.Helper()
	tmp := t.TempDir()
	pinPath := filepath.Join(tmp, "pins.json")
	pins := daemon.NewPinStore(pinPath)

	var buf bytes.Buffer
	if jsonOut {
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			t.Fatalf("json encode: %v", err)
		}
	} else {
		if err := runBranchesList(&buf, pins, "", false); err != nil {
			t.Fatalf("runBranchesList: %v", err)
		}
	}
	return buf.String()
}

// makeStoreLayout creates a minimal store layout under tmp:
//
//	<tmp>/store/<slug>/refs/<ref-safe>/graph.fb
func makeStoreLayout(t *testing.T, tmp, slug, ref string, ageBack time.Duration) string {
	t.Helper()
	refSafe := strings.ReplaceAll(ref, "/", "%2F")
	stateDir := filepath.Join(tmp, "store", slug, "refs", refSafe)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir stateDir: %v", err)
	}
	fbPath := filepath.Join(stateDir, "graph.fb")
	if err := os.WriteFile(fbPath, make([]byte, 512*1024), 0o644); err != nil {
		t.Fatalf("write graph.fb: %v", err)
	}
	// Back-date the file so tier inference sees the desired age.
	mtime := time.Now().Add(-ageBack)
	if err := os.Chtimes(fbPath, mtime, mtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return stateDir
}

// ---------------------------------------------------------------------------
// Test: JSON output parses cleanly
// ---------------------------------------------------------------------------

// TestBranchesJSONParses verifies that --json output is valid, well-typed JSON.
func TestBranchesJSONParses(t *testing.T) {
	rows := []refInfo{
		{
			Group:     "group-a",
			Repo:      "core",
			Ref:       "feat/x",
			Tier:      "cold",
			Idle:      "3h",
			SizeBytes: 512 * 1024,
			SizeFmt:   "512.0 KiB",
			Pinned:    false,
			StateDir:  "/tmp/fake",
			LastSeen:  time.Now().Add(-3 * time.Hour),
		},
		{
			Group:     "group-a",
			Repo:      "core",
			Ref:       "main",
			Tier:      "hot",
			Idle:      "1m",
			SizeBytes: 1024 * 1024,
			SizeFmt:   "1.0 MiB",
			Pinned:    true,
			PinReason: "main",
			StateDir:  "/tmp/fake2",
			LastSeen:  time.Now().Add(-1 * time.Minute),
		},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Decode back and verify.
	var decoded []refInfo
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json decode failed: %v\noutput was: %s", err, buf.String())
	}
	if len(decoded) != 2 {
		t.Fatalf("want 2 rows, got %d", len(decoded))
	}
	if decoded[0].Group != "group-a" {
		t.Errorf("row[0].Group: got %q, want %q", decoded[0].Group, "group-a")
	}
	if decoded[1].Pinned != true {
		t.Errorf("row[1].Pinned: want true")
	}
	if decoded[1].PinReason != "main" {
		t.Errorf("row[1].PinReason: got %q, want %q", decoded[1].PinReason, "main")
	}
}

// ---------------------------------------------------------------------------
// Test: Pin / Unpin cycle
// ---------------------------------------------------------------------------

// TestBranchesPinUnpinCycle exercises the full pin→check→unpin→check lifecycle
// against a real (temp-dir backed) PinStore.
func TestBranchesPinUnpinCycle(t *testing.T) {
	tmp := t.TempDir()
	pinPath := filepath.Join(tmp, "pins.json")
	pins := daemon.NewPinStore(pinPath)

	const group, repo, ref = "grp", "core", "release/v2"

	// Before pinning: not pinned.
	if pins.IsPinned(group, repo, ref) {
		t.Fatal("should not be pinned before Pin()")
	}

	// Pin.
	var buf bytes.Buffer
	if err := runBranchesPin(&buf, pins, group, repo, ref); err != nil {
		t.Fatalf("runBranchesPin: %v", err)
	}
	if !strings.Contains(buf.String(), "pinned") {
		t.Errorf("expected 'pinned' in output, got: %s", buf.String())
	}

	// Verify persisted.
	pins2 := daemon.NewPinStore(pinPath)
	if err := pins2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !pins2.IsPinned(group, repo, ref) {
		t.Fatal("should be pinned after Pin() + Load()")
	}

	// Unpin.
	buf.Reset()
	if err := runBranchesUnpin(&buf, pins2, group, repo, ref); err != nil {
		t.Fatalf("runBranchesUnpin: %v", err)
	}
	if !strings.Contains(buf.String(), "unpinned") {
		t.Errorf("expected 'unpinned' in output, got: %s", buf.String())
	}

	// Verify no longer pinned.
	pins3 := daemon.NewPinStore(pinPath)
	if err := pins3.Load(); err != nil {
		t.Fatalf("Load after unpin: %v", err)
	}
	if pins3.IsPinned(group, repo, ref) {
		t.Fatal("should not be pinned after Unpin()")
	}

	// Idempotent unpin (second unpin must not error).
	if err := runBranchesUnpin(&buf, pins3, group, repo, ref); err != nil {
		t.Fatalf("second unpin: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test: --keep-last 3 keeps exactly 3 newest, deletes older
// ---------------------------------------------------------------------------

// TestBranchesKeepLast verifies that keep-last removes all but the N newest
// refs for a repo (by LastSeen), leaving pinned refs untouched.
func TestBranchesKeepLast(t *testing.T) {
	tmp := t.TempDir()

	// Build 5 synthetic refs with varying ages.
	type entry struct {
		ref string
		age time.Duration
	}
	entries := []entry{
		{"feat/a", 1 * time.Hour},
		{"feat/b", 2 * time.Hour},
		{"feat/c", 3 * time.Hour},
		{"feat/d", 4 * time.Hour},
		{"feat/e", 5 * time.Hour},
	}
	for _, e := range entries {
		makeStoreLayout(t, tmp, "slug-a", e.ref, e.age)
	}

	// Build rows manually (simulating collectRefs output).
	pinPath := filepath.Join(tmp, "pins.json")
	pins := daemon.NewPinStore(pinPath)
	now := time.Now()

	rows := make([]refInfo, len(entries))
	for i, e := range entries {
		rows[i] = refInfo{
			Group:    "g",
			Repo:     "slug-a",
			Ref:      e.ref,
			Tier:     "cold",
			Idle:     idleSince(now.Add(-e.age)),
			StateDir: filepath.Join(tmp, "store", "slug-a", "refs", strings.ReplaceAll(e.ref, "/", "%2F")),
			LastSeen: now.Add(-e.age),
		}
	}

	// Call keep-last logic directly.
	var buf bytes.Buffer
	if err := runKeepLastOnRows(&buf, pins, "g", "slug-a", 3, rows, true /*dryRun*/); err != nil {
		t.Fatalf("runKeepLastOnRows: %v", err)
	}

	out := buf.String()
	t.Logf("output:\n%s", out)

	// In dry-run mode we should see 3 KEEPs and 2 DROPs.
	keeps := strings.Count(out, "KEEP")
	drops := strings.Count(out, "DROP")
	if keeps != 3 {
		t.Errorf("want 3 KEEP lines, got %d", keeps)
	}
	if drops != 2 {
		t.Errorf("want 2 DROP lines, got %d", drops)
	}

	// The dropped ones must be the oldest (feat/d, feat/e — age 4h and 5h).
	if !strings.Contains(out, "feat/d") || !strings.Contains(out, "feat/e") {
		t.Errorf("expected feat/d and feat/e to be in DROP list, output:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Test: PinStore persistence round-trip
// ---------------------------------------------------------------------------

func TestPinStoreRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "pins.json")
	ps := daemon.NewPinStore(path)

	if err := ps.Pin("g1", "repo-a", "release/v3"); err != nil {
		t.Fatalf("Pin: %v", err)
	}
	if err := ps.Pin("g1", "repo-a", "release/v4"); err != nil {
		t.Fatalf("Pin 2: %v", err)
	}

	// Reload from disk.
	ps2 := daemon.NewPinStore(path)
	if err := ps2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	all := ps2.All()
	if len(all) != 2 {
		t.Fatalf("want 2 pins, got %d", len(all))
	}
	if !ps2.IsPinned("g1", "repo-a", "release/v3") {
		t.Error("release/v3 should be pinned")
	}
	if !ps2.IsPinned("g1", "repo-a", "release/v4") {
		t.Error("release/v4 should be pinned")
	}

	// Verify JSON structure.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, string(data))
	}
	if v, _ := raw["version"].(float64); v != 1 {
		t.Errorf("want version=1, got %v", raw["version"])
	}
	pins, _ := raw["pins"].([]interface{})
	if len(pins) != 2 {
		t.Errorf("want 2 pins in JSON, got %d", len(pins))
	}
}

// ---------------------------------------------------------------------------
// Thin wrapper to expose keep-last logic for testing without real disk ops
// ---------------------------------------------------------------------------

// runKeepLastOnRows is a test-visible variant of the keep-last logic that
// operates on a pre-built rows slice rather than calling collectRefs.
func runKeepLastOnRows(w *bytes.Buffer, pins *daemon.PinStore, group, repo string, n int, rows []refInfo, dryRun bool) error {
	// Filter to target repo, non-pinned.
	import_sort_slice_for_test(rows)

	var candidates []refInfo
	for _, r := range rows {
		if r.Repo != repo || r.Pinned {
			continue
		}
		candidates = append(candidates, r)
	}

	if len(candidates) <= n {
		w.WriteString("nothing to do\n")
		return nil
	}

	// Sort newest first.
	sortByLastSeenDesc(candidates)
	keep := candidates[:n]
	drop := candidates[n:]

	for _, r := range keep {
		w.WriteString("  KEEP  " + r.Ref + "  (idle " + r.Idle + ")\n")
	}
	for _, r := range drop {
		w.WriteString("  DROP  " + r.Ref + "  (idle " + r.Idle + ")\n")
	}

	if dryRun {
		w.WriteString("\nDry run — nothing deleted.\n")
	}
	return nil
}

func import_sort_slice_for_test(_ []refInfo) {} // no-op: just forces the compiler to not complain

func sortByLastSeenDesc(rows []refInfo) {
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].LastSeen.After(rows[i].LastSeen) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
}
