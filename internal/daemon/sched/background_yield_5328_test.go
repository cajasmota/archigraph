package sched

import (
	"testing"

	"github.com/cajasmota/grafel/internal/indexstate"
)

// TestBackgroundYieldGOMAXPROCSDefaultAndEnv verifies the yield-cap resolver
// (#5328): default is backgroundYieldGOMAXPROCSDefault (1), overridable by a
// strictly-positive GRAFEL_BACKGROUND_YIELD_GOMAXPROCS, with invalid values
// falling back to the default.
func TestBackgroundYieldGOMAXPROCSDefaultAndEnv(t *testing.T) {
	t.Setenv("GRAFEL_BACKGROUND_YIELD_GOMAXPROCS", "")
	if got := BackgroundYieldGOMAXPROCS(); got != backgroundYieldGOMAXPROCSDefault {
		t.Fatalf("default = %d, want %d", got, backgroundYieldGOMAXPROCSDefault)
	}
	t.Setenv("GRAFEL_BACKGROUND_YIELD_GOMAXPROCS", "2")
	if got := BackgroundYieldGOMAXPROCS(); got != 2 {
		t.Fatalf("env override = %d, want 2", got)
	}
	t.Setenv("GRAFEL_BACKGROUND_YIELD_GOMAXPROCS", "0")
	if got := BackgroundYieldGOMAXPROCS(); got != backgroundYieldGOMAXPROCSDefault {
		t.Fatalf("non-positive env = %d, want default %d", got, backgroundYieldGOMAXPROCSDefault)
	}
	t.Setenv("GRAFEL_BACKGROUND_YIELD_GOMAXPROCS", "garbage")
	if got := BackgroundYieldGOMAXPROCS(); got != backgroundYieldGOMAXPROCSDefault {
		t.Fatalf("garbage env = %d, want default %d", got, backgroundYieldGOMAXPROCSDefault)
	}
}

// TestBackgroundYieldGatedOnForegroundActive is the core #5328 yield proof: a
// background reindex caps its child's GOMAXPROCS to the yield value ONLY while a
// foreground (interactive) index is active, and runs at its normal cap (no
// override) otherwise. The published foreground-active count is the signal — no
// real CPU load is spawned.
func TestBackgroundYieldGatedOnForegroundActive(t *testing.T) {
	t.Setenv("GRAFEL_BACKGROUND_YIELD_GOMAXPROCS", "1")

	// Clean baseline: no index activity → no yield.
	indexstate.SetIndexConcurrency(0, 0, 2, 0)
	if n, yield := backgroundYieldGOMAXPROCS(); yield {
		t.Fatalf("no foreground active: got yield=true n=%d, want yield=false", n)
	}

	// A background-only index running (active=1) but no FOREGROUND → still no yield.
	indexstate.SetIndexConcurrency(1, 0, 2, 0)
	if n, yield := backgroundYieldGOMAXPROCS(); yield {
		t.Fatalf("background-only active: got yield=true n=%d, want yield=false", n)
	}

	// A foreground index is active → background yields to the configured cap.
	indexstate.SetIndexConcurrency(1, 0, 2, 1)
	n, yield := backgroundYieldGOMAXPROCS()
	if !yield {
		t.Fatalf("foreground active: got yield=false, want yield=true")
	}
	if n != 1 {
		t.Fatalf("yield GOMAXPROCS = %d, want 1", n)
	}

	// Foreground finished (back to 0) → background concurrency restores (no override).
	indexstate.SetIndexConcurrency(0, 0, 2, 0)
	if _, yield := backgroundYieldGOMAXPROCS(); yield {
		t.Fatalf("after foreground finished: got yield=true, want yield=false (restored)")
	}

	// Reset shared state for other tests in the package.
	indexstate.SetIndexConcurrency(0, 0, 0, 0)
}
