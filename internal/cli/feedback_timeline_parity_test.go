package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadParity_Convergence(t *testing.T) {
	tmp := t.TempDir()
	pf := filepath.Join(tmp, "parity.json")
	body := `{
		"old_group": "legacy-backend",
		"new_group": "new-backend",
		"snapshots": [
			{"ts":"2026-06-01T00:00:00Z","group":"legacy-backend","endpoints":554},
			{"ts":"2026-06-05T00:00:00Z","group":"new-backend","endpoints":40},
			{"ts":"2026-06-20T00:00:00Z","group":"new-backend","endpoints":277}
		]
	}`
	if err := os.WriteFile(pf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := loadParity(pf, "legacy-backend", "new-backend", "")
	if err != nil {
		t.Fatalf("loadParity: %v", err)
	}
	if rep.Denominator != 554 {
		t.Errorf("denominator = %d, want 554", rep.Denominator)
	}
	if len(rep.Curve) != 2 {
		t.Fatalf("curve len = %d, want 2", len(rep.Curve))
	}
	// 277/554 = 50.0%
	if rep.LatestPct != 50.0 {
		t.Errorf("latest pct = %.1f, want 50.0", rep.LatestPct)
	}
	if rep.Curve[0].Pct <= 0 || rep.Curve[0].Pct >= rep.LatestPct {
		t.Errorf("curve should be increasing: %+v", rep.Curve)
	}
}

func TestLoadParity_NoBaseline(t *testing.T) {
	tmp := t.TempDir()
	pf := filepath.Join(tmp, "parity.json")
	// Only a new-group snapshot — no denominator.
	body := `{"snapshots":[{"ts":"2026-06-05T00:00:00Z","group":"new-backend","endpoints":40}]}`
	if err := os.WriteFile(pf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := loadParity(pf, "legacy-backend", "new-backend", "")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Denominator != 0 || rep.LatestPct != 0 {
		t.Errorf("expected zero denominator/pct without baseline, got %+v", rep)
	}
}

func TestLoadParity_SinceFilter(t *testing.T) {
	tmp := t.TempDir()
	pf := filepath.Join(tmp, "parity.json")
	body := `{"old_group":"legacy-backend","new_group":"new-backend","snapshots":[
		{"ts":"2026-05-01T00:00:00Z","group":"new-backend","endpoints":10},
		{"ts":"2026-06-01T00:00:00Z","group":"legacy-backend","endpoints":554},
		{"ts":"2026-06-10T00:00:00Z","group":"new-backend","endpoints":100}
	]}`
	if err := os.WriteFile(pf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := loadParity(pf, "legacy-backend", "new-backend", "2026-05-15")
	if err != nil {
		t.Fatal(err)
	}
	// The 2026-05-01 new-backend snapshot is excluded by --since.
	if len(rep.Curve) != 1 || rep.Curve[0].Endpoints != 100 {
		t.Errorf("since filter wrong: %+v", rep.Curve)
	}
}

func TestTimeline_WithParityFile_RendersSection(t *testing.T) {
	tmp := t.TempDir()
	eventsDir := filepath.Join(tmp, "events")
	outDir := filepath.Join(tmp, "out")
	writeEventsFile(t, eventsDir, "feedback-events-2026-06-01.jsonl", []string{
		`{"ts":"2026-06-01T09:00:00Z","group":"new-backend","phase":"planning","outcome":"milestone","note":"kickoff"}`,
	})
	pf := filepath.Join(tmp, "parity.json")
	if err := os.WriteFile(pf, []byte(`{"old_group":"legacy-backend","new_group":"new-backend","snapshots":[
		{"ts":"2026-06-01T00:00:00Z","group":"legacy-backend","endpoints":554},
		{"ts":"2026-06-20T00:00:00Z","group":"new-backend","endpoints":277}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := runFeedbackTimeline(cmd, timelineOpts{
		eventsDir: eventsDir, outDir: outDir, rpcLog: filepath.Join(tmp, "nolog"),
		parityFile: pf, oldGroup: "legacy-backend", newGroup: "new-backend",
	}); err != nil {
		t.Fatalf("timeline: %v", err)
	}

	jb, err := os.ReadFile(filepath.Join(outDir, "timeline.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc timelineDoc
	if err := json.Unmarshal(jb, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Parity == nil || doc.Parity.LatestPct != 50.0 {
		t.Fatalf("expected parity 50.0%% in doc, got %+v", doc.Parity)
	}
	md, err := os.ReadFile(filepath.Join(outDir, "timeline.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(md, []byte("Parity (1:1 endpoint convergence)")) {
		t.Error("timeline.md missing parity section")
	}
}
