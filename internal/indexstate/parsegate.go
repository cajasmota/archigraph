// parsegate.go — the in-process tree-sitter parse CPU/concurrency cap (#5630).
//
// PROBLEM. The reactive scheduler's incremental reindex (extractors.
// TryIncremental) and the opt-out in-process full index re-parse changed files
// INSIDE the daemon process. That parse goes through neither of the existing
// throttles:
//
//   - the IndexGate (#5493) — only rebuildWorkerPool acquires it, so the
//     reactive/incremental path never registers in concurrency.indexing; and
//   - the #5602 reindex CPU ceiling — that sets GOMAXPROCS only on the
//     index-internal SUBPROCESS, so an in-process parse runs at the daemon's
//     own GOMAXPROCS (= host core count).
//
// Result: a daemon parsing in-process can monopolise the box (the multi-hour
// 5–7 core burn in #5630) while index_status reports idle.
//
// FIX. A single process-wide gate bounds how many in-process tree-sitter
// parses run CONCURRENTLY. treesitter.ParserFactory.Parse acquires a slot
// around every parse, so ALL in-process parsing — regardless of caller — is
// bounded by one daemon-wide ceiling. Excess parses block until a slot frees.
//
// The gate is a leaf-package primitive (indexstate imports nothing) so the
// low-level treesitter package can depend on it without an import cycle. It is
// off by default (cap 0 = unbounded) so non-daemon callers — plain `grafel
// index`, the extract subprocesses, tests — are unaffected; the daemon installs
// a real cap at startup via SetParseConcurrency.

package indexstate

import "sync"

var (
	parseGateMu sync.Mutex
	// parseGateCap is the max concurrent in-process parses. 0 means unbounded
	// (the default for non-daemon processes). The daemon sets a positive cap.
	parseGateCap int
	// parseGateActive is the number of parse slots currently held.
	parseGateActive int
	// parseGateWaiters is a FIFO of blocked acquirers, each woken with a slot
	// already charged to it.
	parseGateWaiters []chan struct{}
)

// SetParseConcurrency installs the daemon-wide cap on concurrent in-process
// tree-sitter parses (#5630). A cap <= 0 disables the gate (unbounded), which
// is the default for non-daemon processes. The daemon calls this once at
// startup with a resource-safe value (≈ the reindex CPU budget). Safe to call
// from any goroutine; lowering the cap does not preempt parses already running.
func SetParseConcurrency(cap int) {
	parseGateMu.Lock()
	if cap < 0 {
		cap = 0
	}
	parseGateCap = cap
	// Raising the cap may free slots for queued waiters; wake any now-eligible.
	wakeParseWaitersLocked()
	parseGateMu.Unlock()
}

// ParseConcurrencyCap returns the configured in-process parse cap (0 = unbounded).
func ParseConcurrencyCap() int {
	parseGateMu.Lock()
	defer parseGateMu.Unlock()
	return parseGateCap
}

// AcquireParseSlot blocks until an in-process parse slot is free, then returns.
// The caller MUST call ReleaseParseSlot exactly once when the parse completes.
// When the gate is unbounded (cap 0) it returns immediately without queueing.
// It also brackets the in-process parse accounting (ParseBegin/ParseEnd) so a
// single call site — treesitter.ParserFactory.Parse — makes every in-process
// parse both observable (busy signal) and bounded (CPU ceiling).
func AcquireParseSlot() {
	ParseBegin()
	parseGateMu.Lock()
	if parseGateCap <= 0 || parseGateActive < parseGateCap {
		parseGateActive++
		parseGateMu.Unlock()
		return
	}
	ticket := make(chan struct{})
	parseGateWaiters = append(parseGateWaiters, ticket)
	parseGateMu.Unlock()
	<-ticket // woken with a slot already charged to us
}

// ReleaseParseSlot frees one parse slot and wakes the next waiter (FIFO), and
// records the parse as complete. MUST be paired with a successful
// AcquireParseSlot.
func ReleaseParseSlot() {
	parseGateMu.Lock()
	if parseGateActive > 0 {
		parseGateActive--
	}
	wakeParseWaitersLocked()
	parseGateMu.Unlock()
	ParseEnd()
}

// wakeParseWaitersLocked promotes queued waiters into free slots (FIFO). Each
// promoted ticket has a slot charged to it before being signalled. MUST hold
// parseGateMu. With cap 0 (unbounded) every waiter is drained.
func wakeParseWaitersLocked() {
	for len(parseGateWaiters) > 0 && (parseGateCap <= 0 || parseGateActive < parseGateCap) {
		t := parseGateWaiters[0]
		parseGateWaiters = parseGateWaiters[1:]
		parseGateActive++
		close(t)
	}
}
