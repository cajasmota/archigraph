package main

import (
	"os"
	"testing"
)

// TestFmtCheckDetectsCiteDrift locks in the guard against the #2907/#2912
// class of breakage: a registry whose indentation is fine but whose cites
// are not in canonical sorted order must be reported as non-canonical by
// `fmt --check` (saveRegistry sorts cites, so any later `update` would
// re-sort them and produce a spurious cross-record diff). `fmt` must then
// rewrite it to canonical so a subsequent `--check` passes.
func TestFmtCheckDetectsCiteDrift(t *testing.T) {
	tmp := copyFixture(t)

	// Introduce cite-order drift: reverse a known-sorted 2-cite cell and
	// write it back WITHOUT sorting (marshalRegistry alone, not saveRegistry).
	reg, err := loadRegistry(tmp)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	const id, cap = "lang.python.framework.django-drf", "endpoint_synthesis"
	rec := findRecord(reg, id)
	if rec == nil {
		t.Fatalf("fixture missing record %q", id)
	}
	cell := rec.Capabilities[cap]
	if len(cell.Cites) < 2 {
		t.Fatalf("fixture cell %s/%s needs >=2 cites, got %v", id, cap, cell.Cites)
	}
	cell.Cites[0], cell.Cites[1] = cell.Cites[1], cell.Cites[0] // now descending
	rec.Capabilities[cap] = cell
	buf, err := marshalRegistry(reg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// --check must FAIL on the drifted file.
	if _, _, err := runCmd(t, "fmt", "--check", "--file", tmp); err == nil {
		t.Fatal("fmt --check accepted a registry with unsorted cites; expected non-canonical error")
	}

	// fmt (write) canonicalizes it.
	if _, _, err := runCmd(t, "fmt", "--file", tmp); err != nil {
		t.Fatalf("fmt write: %v", err)
	}

	// --check must now PASS.
	if _, _, err := runCmd(t, "fmt", "--check", "--file", tmp); err != nil {
		t.Fatalf("fmt --check rejected a freshly-formatted registry: %v", err)
	}

	// And the cites are back in ascending (canonical) order.
	reg2, err := loadRegistry(tmp)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := findRecord(reg2, id).Capabilities[cap].Cites
	if got[0] > got[1] {
		t.Fatalf("cites not canonically sorted after fmt: %v", got)
	}
}
