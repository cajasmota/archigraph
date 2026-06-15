package watchers

import (
	"strings"
	"testing"
)

// TestWatcherStatusString_NotInstalled verifies the String() representation
// for a unit that has never been installed.
func TestWatcherStatusString_NotInstalled(t *testing.T) {
	ws := WatcherStatus{TaskName: "com.grafel.watcher.demo.core"}
	s := ws.String()
	if !strings.Contains(s, "not installed") {
		t.Errorf("expected 'not installed' in %q", s)
	}
	if !strings.Contains(s, "com.grafel.watcher.demo.core") {
		t.Errorf("expected task name in %q", s)
	}
}

// TestWatcherStatusString_Running verifies the String() representation for a
// running unit includes the PID when available.
func TestWatcherStatusString_Running(t *testing.T) {
	ws := WatcherStatus{
		TaskName:  "com.grafel.watcher.demo.core",
		Installed: true,
		Running:   true,
		PID:       1234,
	}
	s := ws.String()
	if !strings.Contains(s, "running") {
		t.Errorf("expected 'running' in %q", s)
	}
	if !strings.Contains(s, "1234") {
		t.Errorf("expected pid=1234 in %q", s)
	}
}

// TestWatcherStatusString_InstalledNotRunning verifies the String() for an
// installed but not-yet-started unit.
func TestWatcherStatusString_InstalledNotRunning(t *testing.T) {
	ws := WatcherStatus{
		TaskName:  "com.grafel.watcher.demo.core",
		Installed: true,
		Running:   false,
	}
	s := ws.String()
	if !strings.Contains(s, "installed") {
		t.Errorf("expected 'installed' in %q", s)
	}
	if strings.Contains(s, "running") && !strings.Contains(s, "not running") {
		t.Errorf("expected 'not running' in %q", s)
	}
}

// TestNewLoaderNotNil verifies that NewLoader() returns a non-nil Loader on
// every platform (including the unsupported stub).
func TestNewLoaderNotNil(t *testing.T) {
	l := NewLoader()
	if l == nil {
		t.Fatal("NewLoader() returned nil")
	}
}

// TestLoaderInterface_CurrentPlatform is a compile-time assertion that
// NewLoader() returns a type satisfying the Loader interface. The assignment
// is evaluated by the compiler on every platform.
var _ Loader = NewLoader()
