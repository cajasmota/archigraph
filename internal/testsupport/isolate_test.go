package testsupport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsolateHomeRedirectsAllEnv(t *testing.T) {
	home := IsolateHome(t)

	if got := os.Getenv("HOME"); got != home {
		t.Fatalf("HOME = %q, want %q", got, home)
	}
	if home == RealUserHome() {
		t.Fatalf("isolated home %q equals real user home — isolation failed", home)
	}
	for _, ev := range []string{"GRAFEL_DAEMON_ROOT", "GRAFEL_HOME", "XDG_CONFIG_HOME", "XDG_RUNTIME_DIR"} {
		v := os.Getenv(ev)
		if v == "" {
			t.Errorf("%s not set", ev)
			continue
		}
		if !strings.HasPrefix(filepath.Clean(v), home) {
			t.Errorf("%s = %q not under isolated home %q", ev, v, home)
		}
	}
	// A path resolved from the (now isolated) home must satisfy AssertUnderHome.
	AssertUnderHome(t, filepath.Join(home, ".claude.json"))
}

func TestGuardRealHomeFailsWhenHomeIsRealHome(t *testing.T) {
	rh := RealUserHome()
	if rh == "" {
		t.Skip("no real user home captured")
	}
	// Point HOME back at the real user home and assert GuardRealHome fails.
	t.Setenv("HOME", rh)
	t.Setenv("USERPROFILE", rh)

	bt := &testing.T{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = recover() }() // Fatalf calls runtime.Goexit
		GuardRealHome(bt)
	}()
	<-done
	if !bt.Failed() {
		t.Fatal("GuardRealHome did not fail when HOME == real user home")
	}
}

func TestAssertUnderHomeRejectsEscape(t *testing.T) {
	home := IsolateHome(t)
	_ = home

	bt := &testing.T{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = recover() }()
		AssertUnderHome(bt, "/etc/passwd")
	}()
	<-done
	if !bt.Failed() {
		t.Fatal("AssertUnderHome accepted a path outside the isolated home")
	}
}
