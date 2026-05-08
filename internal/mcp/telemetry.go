package mcp

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

// Telemetry records per-tool call counts and latency buckets, plus reload counts.
type Telemetry struct {
	mu        sync.Mutex
	startedAt time.Time
	tools     map[string]*toolStats
	reloads   int
	errors    int
	debug     int
}

type toolStats struct {
	Calls      int           `json:"calls"`
	Errors     int           `json:"errors"`
	TotalLatNS time.Duration `json:"-"`
	MaxLatNS   time.Duration `json:"-"`
}

// NewTelemetry constructs a fresh telemetry recorder.
func NewTelemetry(debugLevel int) *Telemetry {
	return &Telemetry{
		startedAt: time.Now(),
		tools:     map[string]*toolStats{},
		debug:     debugLevel,
	}
}

// Begin records a tool entry; the returned closure should be called on exit.
func (t *Telemetry) Begin(tool string) func(err bool) {
	start := time.Now()
	return func(err bool) {
		t.mu.Lock()
		defer t.mu.Unlock()
		s := t.tools[tool]
		if s == nil {
			s = &toolStats{}
			t.tools[tool] = s
		}
		s.Calls++
		dur := time.Since(start)
		s.TotalLatNS += dur
		if dur > s.MaxLatNS {
			s.MaxLatNS = dur
		}
		if err {
			s.Errors++
			t.errors++
		}
	}
}

// MarkReload records that a graph reload happened.
func (t *Telemetry) MarkReload(n int) {
	if n <= 0 {
		return
	}
	t.mu.Lock()
	t.reloads += n
	t.mu.Unlock()
}

// Snapshot returns a JSON-able summary of current telemetry.
func (t *Telemetry) Snapshot() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	tools := map[string]any{}
	names := make([]string, 0, len(t.tools))
	for k := range t.tools {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		s := t.tools[n]
		avgUS := int64(0)
		if s.Calls > 0 {
			avgUS = (s.TotalLatNS / time.Duration(s.Calls)).Microseconds()
		}
		tools[n] = map[string]any{
			"calls":  s.Calls,
			"errors": s.Errors,
			"avg_us": avgUS,
			"max_us": s.MaxLatNS.Microseconds(),
		}
	}
	return map[string]any{
		"uptime_seconds": int(time.Since(t.startedAt).Seconds()),
		"reloads":        t.reloads,
		"errors":         t.errors,
		"tools":          tools,
	}
}

// SnapshotJSON renders the telemetry summary as pretty JSON.
func (t *Telemetry) SnapshotJSON() string {
	data, _ := json.MarshalIndent(t.Snapshot(), "", "  ")
	return string(data)
}
