// Package watchers — loader.go
//
// Loader is the platform abstraction for activating / deactivating a watcher
// unit that has already been written to disk by Write(). The underlying
// mechanism is OS-specific:
//
//   - macOS:   launchctl bootstrap gui/<uid> <plist>
//   - Linux:   systemctl --user enable --now <label>
//   - Windows: schtasks /create /xml <xml> /tn <taskname>
//
// NewLoader() returns the implementation for the current OS. Callers that
// only need XML/plist generation do not need to use a Loader at all; the
// pure render/write functions remain side-effect-free.
package watchers

import "fmt"

// WatcherStatus describes the activation state of a single watcher unit.
type WatcherStatus struct {
	// TaskName is the OS-level identifier (label / task name / service name).
	TaskName string
	// Installed is true when the unit file exists on disk.
	Installed bool
	// Running is true when the OS reports the watcher process is active.
	Running bool
	// PID is the process ID of the running watcher, 0 if unknown or not running.
	PID int
}

// String returns a compact, human-readable summary.
func (s WatcherStatus) String() string {
	if !s.Installed {
		return fmt.Sprintf("%s: not installed", s.TaskName)
	}
	if s.Running {
		pid := ""
		if s.PID > 0 {
			pid = fmt.Sprintf(" pid=%d", s.PID)
		}
		return fmt.Sprintf("%s: running%s", s.TaskName, pid)
	}
	return fmt.Sprintf("%s: installed, not running", s.TaskName)
}

// nonFatalError is implemented by errors that indicate partial success: the
// primary operation succeeded but a secondary step (e.g. immediate /run)
// failed. Callers can test for this with IsNonFatal.
type nonFatalError interface {
	IsNonFatal() bool
}

// IsNonFatal reports whether err (or any error in its chain) represents a
// partial-success condition — the unit was registered with the OS scheduler
// but the immediate start attempt failed. On Windows this means the watcher
// will activate at next logon. On other platforms this is never true.
func IsNonFatal(err error) bool {
	if err == nil {
		return false
	}
	type unwrapper interface{ Unwrap() error }
	for e := err; e != nil; {
		if nf, ok := e.(nonFatalError); ok && nf.IsNonFatal() {
			return true
		}
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			break
		}
	}
	return false
}

// Loader is the interface implemented by each platform backend.
// The three methods are idempotent: calling Load on an already-running
// unit or Unload on an absent unit must not return an error.
type Loader interface {
	// Load activates the watcher unit. The unit file must already exist on
	// disk (created by Write). After Load returns nil the unit is scheduled
	// to start at the next login; on some platforms it is started immediately.
	Load(u Unit) error

	// Unload deactivates and removes the watcher unit from the OS scheduler /
	// service manager. It does NOT remove the unit file from disk — callers
	// should also call Remove(u) if a full cleanup is desired.
	Unload(u Unit) error

	// Status returns the current activation state of the watcher unit.
	Status(u Unit) (WatcherStatus, error)
}
