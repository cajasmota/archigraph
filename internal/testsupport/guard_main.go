package testsupport

import (
	"fmt"
	"os"
)

// GuardRealHomeMain is the TestMain-time, fail-closed guard for an entire
// package whose tests resolve config/state/socket paths from the environment.
//
// It does NOT redirect HOME (individual tests must still call IsolateHome for
// that). Instead, when ARCHIGRAPH_TEST_REQUIRE_ISOLATED_HOME=1 is set (e.g. in
// CI), it aborts the whole test binary before a single test runs if the
// process started with HOME pointing at the real user home and no isolation
// env is in effect — turning a "this test corrupted my live config" incident
// into a loud, immediate failure.
//
// Usage:
//
//	func TestMain(m *testing.M) {
//	    testsupport.GuardRealHomeMain()
//	    os.Exit(m.Run())
//	}
//
// By default (env not set) it is a no-op so local `go test` keeps working even
// from a real HOME — the per-test IsolateHome/GuardRealHome guards still apply.
func GuardRealHomeMain() {
	if os.Getenv("ARCHIGRAPH_TEST_REQUIRE_ISOLATED_HOME") != "1" {
		return
	}
	// If an isolation env is already in effect, the binary is sandboxed.
	if os.Getenv(envDaemonRoot) != "" {
		return
	}
	if eff := effectiveHome(); realUserHome != "" && eff == realUserHome {
		fmt.Fprintf(os.Stderr,
			"testsupport: REFUSING to run — HOME (%q) is the real user home and "+
				"ARCHIGRAPH_TEST_REQUIRE_ISOLATED_HOME=1. These tests can corrupt the live "+
				"~/.claude.json / ~/.codeium / ~/.archigraph or kill the live daemon. Run under a "+
				"sandbox HOME (export HOME=$(mktemp -d); export ARCHIGRAPH_DAEMON_ROOT=$HOME/.archigraph).\n",
			eff,
		)
		os.Exit(2)
	}
}
