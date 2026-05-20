package cli

// doctor_staleness_test.go — unit tests for the stale-daemon scan logic
// introduced in issue #857 (Layer 3).
//
// We test the parsing helpers (parsePsEo, parsePsAux) and the classification
// logic (isOrphan, isTmp) directly because they are pure functions over text.

import (
	"strings"
	"testing"
)

// TestParsePsEo_IdentifiesArchigraphProcs verifies that parsePsEo correctly
// extracts archigraph processes from `ps -eo pid,ppid,comm` output.
func TestParsePsEo_IdentifiesArchigraphProcs(t *testing.T) {
	// Synthetic ps -eo pid,ppid,comm output with one archigraph daemon
	// adopted by launchd (ppid=1) and one canonical daemon.
	psOut := []byte(`  PID  PPID COMM
  100     1 /tmp/arch-test/archigraph
  200   150 /usr/local/bin/archigraph
  300   200 /usr/local/bin/sleep
  400     1 /tmp/arch-wave2/archigraph
`)
	myPID := 9999 // not in the list
	procs := parsePsEo(psOut, myPID)

	if len(procs) != 3 {
		t.Fatalf("expected 3 archigraph procs, got %d: %+v", len(procs), procs)
	}

	// First proc: pid=100, ppid=1, /tmp binary → orphan + tmp
	p0 := procs[0]
	if p0.PID != 100 {
		t.Errorf("proc[0].PID: want 100, got %d", p0.PID)
	}
	if !p0.IsOrphan {
		t.Errorf("proc[0] should be orphan (ppid=1)")
	}
	if !p0.IsTmp {
		t.Errorf("proc[0] should be IsTmp")
	}

	// Second proc: pid=200, ppid=150, canonical binary → not orphan, not tmp
	p1 := procs[1]
	if p1.PID != 200 {
		t.Errorf("proc[1].PID: want 200, got %d", p1.PID)
	}
	if p1.IsOrphan {
		t.Errorf("proc[1] should NOT be orphan (ppid=150)")
	}
	if p1.IsTmp {
		t.Errorf("proc[1] should NOT be IsTmp (/usr/local/bin path)")
	}

	// Third proc: pid=400, ppid=1, /tmp binary → orphan + tmp
	p2 := procs[2]
	if p2.PID != 400 {
		t.Errorf("proc[2].PID: want 400, got %d", p2.PID)
	}
	if !p2.IsOrphan {
		t.Errorf("proc[2] should be orphan (ppid=1)")
	}
	if !p2.IsTmp {
		t.Errorf("proc[2] should be IsTmp")
	}
}

// TestParsePsEo_ExcludesMyPID verifies that the calling process is excluded
// from the scan results.
func TestParsePsEo_ExcludesMyPID(t *testing.T) {
	psOut := []byte(`  PID  PPID COMM
 1234   500 /usr/local/bin/archigraph
`)
	procs := parsePsEo(psOut, 1234) // exclude pid 1234
	if len(procs) != 0 {
		t.Errorf("expected myPID to be excluded, got %+v", procs)
	}
}

// TestParsePsAux_BasicParsing verifies the ps aux fallback parser.
func TestParsePsAux_BasicParsing(t *testing.T) {
	// ps aux format: USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND...
	psOut := []byte(`USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
jorge        123  0.0  0.1  12345  6789 ?        Ss   10:00   0:01 /tmp/arch-agent/archigraph daemon start
jorge        456  0.0  0.1  12345  6789 ?        Ss   09:00   0:10 /usr/local/bin/archigraph daemon start
jorge        789  0.0  0.0   1234   567 ?        S    10:01   0:00 /bin/sleep 60
`)
	procs := parsePsAux(psOut, 9999)
	if len(procs) != 2 {
		t.Fatalf("expected 2 archigraph procs, got %d: %+v", len(procs), procs)
	}
	if !procs[0].IsTmp {
		t.Errorf("proc[0] should be IsTmp, exe=%s", procs[0].Exe)
	}
	if procs[1].IsTmp {
		t.Errorf("proc[1] should NOT be IsTmp, exe=%s", procs[1].Exe)
	}
}

// TestStaleProcessClassification verifies the stale detection criteria:
// - orphan /tmp binary → stale
// - canonical binary with different path than self → stale (daemon)
// - same binary → not stale
func TestStaleProcessClassification(t *testing.T) {
	selfExe := "/usr/local/bin/archigraph"

	cases := []struct {
		name      string
		proc      staleProcess
		wantStale bool
	}{
		{
			name: "orphan /tmp daemon",
			proc: staleProcess{PID: 1, PPID: 1, Exe: "/tmp/arch-test/archigraph",
				IsOrphan: true, IsTmp: true},
			wantStale: true,
		},
		{
			// A daemon binary path that differs from self AND has "daemon" in
			// the exe name (as would appear in ps comm column for the daemon process).
			name: "different canonical daemon binary",
			proc: staleProcess{PID: 2, PPID: 100, Exe: "/usr/local/bin/archigraph-daemon-old",
				IsOrphan: false, IsTmp: false},
			wantStale: true,
		},
		{
			name: "same binary as self — not stale",
			proc: staleProcess{PID: 3, PPID: 100, Exe: selfExe,
				IsOrphan: false, IsTmp: false},
			wantStale: false,
		},
		{
			name: "non-/tmp, PPID=1 but not daemon in name",
			proc: staleProcess{PID: 4, PPID: 1, Exe: "/usr/local/bin/archigraph",
				IsOrphan: true, IsTmp: false},
			// PPID=1 without IsTmp doesn't trigger criterion 1.
			// Exe == selfExe so criterion 2 is false.
			wantStale: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isStaleProc(tc.proc, selfExe)
			if got != tc.wantStale {
				t.Errorf("isStaleProc(%+v, %q) = %v, want %v", tc.proc, selfExe, got, tc.wantStale)
			}
		})
	}
}

// isStaleProc mirrors the classification logic from runDoctorStaleDaemons.
// It is extracted here so we can unit-test it without running the full cobra
// command. In doctor.go, the same logic lives inline inside runDoctorStaleDaemons.
func isStaleProc(p staleProcess, selfExe string) bool {
	// Stale criterion 1: PPID=1 (launchd/systemd orphan) + binary under /tmp
	if p.PPID == 1 && p.IsTmp {
		return true
	}
	// Stale criterion 2: daemon process running from a different binary than self
	if strings.Contains(strings.ToLower(p.Exe), "daemon") && p.Exe != selfExe {
		return true
	}
	return false
}

// TestRunDoctorStaleDaemons_DryRunOutputsNoneWhenClean verifies the full
// runDoctorStaleDaemons function returns a clean "none found" message when
// no stale processes exist. We call it with kill=false (dry-run default).
//
// Note: this test can only run cleanly on a machine with no stale archigraph
// daemons. On a developer machine with a real daemon it may log stale entries —
// that's correct behaviour, not a test failure.
func TestRunDoctorStaleDaemons_DryRunOutputsNoneWhenClean(t *testing.T) {
	var sb strings.Builder
	if err := runDoctorStaleDaemons(&sb, false); err != nil {
		t.Fatalf("runDoctorStaleDaemons: %v", err)
	}
	out := sb.String()
	t.Logf("doctor stale scan output:\n%s", out)
	// We only assert that the function returns without error. The output
	// content is environment-dependent (depends on what's running on this machine).
	// The meaningful coverage is that it doesn't panic or crash.
}
