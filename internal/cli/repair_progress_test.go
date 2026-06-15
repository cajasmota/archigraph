package cli

// Unit tests for #802 — rebuild progress helpers.
// These tests exercise the non-RPC parts (formatting, JSON shape) so
// they can run without a live daemon.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cajasmota/grafel/internal/daemon/proto"
)

// ---------------------------------------------------------------------------
// fmtDuration
// ---------------------------------------------------------------------------

func TestFmtDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{time.Minute, "1m00s"},
		{90 * time.Second, "1m30s"},
		{time.Hour, "1h00m00s"},
		{3611 * time.Second, "1h00m11s"},
		{2*time.Hour + 3*time.Minute + 4*time.Second, "2h03m04s"},
	}
	for _, tc := range cases {
		got := fmtDuration(tc.d)
		if got != tc.want {
			t.Errorf("fmtDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// printProgressLine
// ---------------------------------------------------------------------------

func TestPrintProgressLine_Queued(t *testing.T) {
	// Queued phase is suppressed on TTY; on a bytes.Buffer (non-TTY) it prints.
	var buf bytes.Buffer
	printProgressLine(&buf, proto.RepoProgressState{
		Slug:  "core",
		Index: 1,
		Total: 3,
		Phase: proto.PhaseQueued,
	})
	got := buf.String()
	// Non-TTY output must contain the slug and "queued".
	if !strings.Contains(got, "core") || !strings.Contains(got, "queued") {
		t.Errorf("unexpected queued line: %q", got)
	}
}

func TestPrintProgressLine_Started(t *testing.T) {
	var buf bytes.Buffer
	printProgressLine(&buf, proto.RepoProgressState{
		Slug:  "frontend",
		Index: 2,
		Total: 3,
		Phase: proto.PhaseStarted,
	})
	got := buf.String()
	// New format: "frontend: starting\n" on non-TTY.
	if !strings.Contains(got, "frontend") || !strings.Contains(got, "start") {
		t.Errorf("unexpected started line: %q", got)
	}
}

func TestPrintProgressLine_Completed_WithStats(t *testing.T) {
	var buf bytes.Buffer
	printProgressLine(&buf, proto.RepoProgressState{
		Slug:       "mobile",
		Index:      3,
		Total:      3,
		Phase:      proto.PhaseCompleted,
		ElapsedSec: 32.0,
		Entities:   1819,
		Rels:       4012,
	})
	got := buf.String()
	// New format: "mobile: DONE 32s  (1,819 entities, 4,012 relationships)\n"
	if !strings.Contains(got, "mobile") {
		t.Errorf("missing slug in completed line: %q", got)
	}
	if !strings.Contains(got, "DONE") {
		t.Errorf("missing 'DONE' in completed line: %q", got)
	}
	if !strings.Contains(got, "1,819") {
		t.Errorf("missing entity count (1,819) in completed line: %q", got)
	}
	if !strings.Contains(got, "4,012") {
		t.Errorf("missing rel count (4,012) in completed line: %q", got)
	}
}

func TestPrintProgressLine_Failed(t *testing.T) {
	var buf bytes.Buffer
	printProgressLine(&buf, proto.RepoProgressState{
		Slug:   "broken",
		Index:  1,
		Total:  1,
		Phase:  proto.PhaseFailed,
		ErrMsg: "permission denied",
	})
	got := buf.String()
	// New format: "broken: FAILED — permission denied\n"
	if !strings.Contains(got, "FAILED") {
		t.Errorf("missing 'FAILED': %q", got)
	}
	if !strings.Contains(got, "permission denied") {
		t.Errorf("missing error message: %q", got)
	}
}

func TestPrintProgressLine_NoTotal(t *testing.T) {
	// Slug must always appear regardless of Total.
	var buf bytes.Buffer
	printProgressLine(&buf, proto.RepoProgressState{
		Slug:  "myrepo",
		Phase: proto.PhaseStarted,
	})
	got := buf.String()
	if !strings.Contains(got, "myrepo") {
		t.Errorf("missing repo name: %q", got)
	}
}

// ---------------------------------------------------------------------------
// emitJSONProgressState
// ---------------------------------------------------------------------------

func TestEmitJSONProgressState_Shape(t *testing.T) {
	var buf bytes.Buffer
	emitJSONProgressState(&buf, "tok123", proto.RepoProgressState{
		Slug:       "core",
		Path:       "/repos/core",
		Phase:      proto.PhaseCompleted,
		Index:      1,
		Total:      2,
		ElapsedSec: 92.0,
		Entities:   7821,
		Rels:       26583,
	})
	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
	}
	checks := map[string]string{
		"event": "progress",
		"token": "tok123",
		"slug":  "core",
		"phase": proto.PhaseCompleted,
	}
	for k, want := range checks {
		got, ok := m[k]
		if !ok {
			t.Errorf("missing field %q", k)
			continue
		}
		if gotStr, ok := got.(string); !ok || gotStr != want {
			t.Errorf("field %q = %v, want %q", k, got, want)
		}
	}
	// elapsed must be present and non-empty for ElapsedSec > 0.
	if elapsed, ok := m["elapsed"].(string); !ok || elapsed == "" {
		t.Errorf("expected non-empty elapsed, got %v", m["elapsed"])
	}
	// entities must be > 0.
	if ent, ok := m["entities"].(float64); !ok || ent != 7821 {
		t.Errorf("expected entities=7821, got %v", m["entities"])
	}
}

func TestEmitJSONProgressState_NoElapsedWhenZero(t *testing.T) {
	var buf bytes.Buffer
	emitJSONProgressState(&buf, "t", proto.RepoProgressState{
		Slug:       "x",
		Phase:      proto.PhaseStarted,
		ElapsedSec: 0,
	})
	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if elapsed, ok := m["elapsed"]; ok {
		if s, ok := elapsed.(string); ok && s != "" {
			t.Errorf("expected empty or missing elapsed for zero duration, got %q", s)
		}
	}
}

// ---------------------------------------------------------------------------
// emitJSONEvent (heartbeat)
// ---------------------------------------------------------------------------

func TestEmitJSONEvent_Heartbeat(t *testing.T) {
	var buf bytes.Buffer
	emitJSONEvent(&buf, "heartbeat", "mygroup", "")
	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["event"] != "heartbeat" {
		t.Errorf("event = %v, want 'heartbeat'", m["event"])
	}
	if m["group"] != "mygroup" {
		t.Errorf("group = %v, want 'mygroup'", m["group"])
	}
}

// ---------------------------------------------------------------------------
// progressToken
// ---------------------------------------------------------------------------

func TestProgressToken_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok := progressToken()
		if tok == "" {
			t.Fatalf("empty token at iteration %d", i)
		}
		if seen[tok] {
			t.Fatalf("duplicate token %q at iteration %d", tok, i)
		}
		seen[tok] = true
	}
}

// ---------------------------------------------------------------------------
// fmtDuration edge cases
// ---------------------------------------------------------------------------

func TestFmtDuration_SubSecond(t *testing.T) {
	// Durations < 1s should truncate to 0s.
	got := fmtDuration(500 * time.Millisecond)
	if got != "0s" {
		t.Errorf("got %q, want '0s'", got)
	}
}
