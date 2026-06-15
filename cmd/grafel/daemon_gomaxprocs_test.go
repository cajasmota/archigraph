package main

import "testing"

// TestResolveDaemonGOMAXPROCS verifies the #5135 native in-process daemon CPU
// knob: GRAFEL_DAEMON_GOMAXPROCS caps the daemon's own Go-runtime
// parallelism, returning 0 ("leave the Go default untouched") when unset/empty,
// invalid, or already >= the host core count. envPositiveInt2 treats an empty
// or whitespace-only value as unset, so the empty-string cases exercise the
// unset branch deterministically without an os.Unsetenv dance.
func TestResolveDaemonGOMAXPROCS(t *testing.T) {
	const host = 12

	cases := []struct {
		name string
		env  string
		want int
	}{
		{"empty", "", 0},
		{"blank", "   ", 0},
		{"valid-below-host", "3", 3},
		{"one", "1", 1},
		{"equal-host-noop", "12", 0},
		{"above-host-noop", "20", 0},
		{"zero-ignored", "0", 0},
		{"negative-ignored", "-4", 0},
		{"garbage-ignored", "abc", 0},
		{"float-ignored", "2.5", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GRAFEL_DAEMON_GOMAXPROCS", tc.env)
			if got := resolveDaemonGOMAXPROCS(host); got != tc.want {
				t.Fatalf("resolveDaemonGOMAXPROCS(env=%q, host=%d) = %d, want %d", tc.env, host, got, tc.want)
			}
		})
	}

	// host=0 (unknown core count) must not panic and should still return the
	// requested cap (no host ceiling to compare against).
	t.Setenv("GRAFEL_DAEMON_GOMAXPROCS", "4")
	if got := resolveDaemonGOMAXPROCS(0); got != 4 {
		t.Fatalf("resolveDaemonGOMAXPROCS(host=0) = %d, want 4", got)
	}
}

// TestEnvPositiveInt2 covers the local env helper used by the daemon CPU knob.
func TestEnvPositiveInt2(t *testing.T) {
	cases := map[string]int{
		"":        0,
		"   ":     0,
		"5":       5,
		" 7 ":     7,
		"0":       0,
		"-3":      0,
		"abc":     0,
		"3.5":     0,
		"1000000": 1000000,
	}
	for raw, want := range cases {
		t.Setenv("AG_TEST_ENV_POSINT2", raw)
		if got := envPositiveInt2("AG_TEST_ENV_POSINT2"); got != want {
			t.Errorf("envPositiveInt2(%q) = %d, want %d", raw, got, want)
		}
	}
}
