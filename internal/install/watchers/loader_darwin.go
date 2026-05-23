//go:build darwin

package watchers

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// darwinLoader implements Loader using launchctl for macOS LaunchAgents.
type darwinLoader struct{}

// NewLoader returns the macOS launchctl-based Loader.
func NewLoader() Loader { return darwinLoader{} }

// Load writes the plist (via Write) and bootstraps it into the current user's
// launchd domain. If the unit is already running it is a no-op.
func (darwinLoader) Load(u Unit) error {
	path, err := UnitPath(u)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("unit file not found — call Write(u) first: %s", path)
	}

	uid := strconv.Itoa(os.Getuid())
	// Bootout any stale entry so bootstrap succeeds cleanly.
	_ = exec.Command("launchctl", "bootout", "gui/"+uid+"/"+u.Label()).Run()

	out, err := exec.Command("launchctl", "bootstrap", "gui/"+uid, path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap %s: %w\n%s", u.Label(), err, out)
	}
	return nil
}

// Unload bootouts the LaunchAgent for the given unit. Errors are suppressed
// when the unit was never loaded.
func (darwinLoader) Unload(u Unit) error {
	uid := strconv.Itoa(os.Getuid())
	out, err := exec.Command("launchctl", "bootout", "gui/"+uid+"/"+u.Label()).CombinedOutput()
	if err != nil {
		s := string(out)
		if strings.Contains(s, "No such process") ||
			strings.Contains(s, "Could not find specified service") ||
			strings.Contains(s, "No domain") {
			return nil // already gone
		}
		return fmt.Errorf("launchctl bootout %s: %w\n%s", u.Label(), err, out)
	}
	return nil
}

// Status queries launchctl list for the watcher label.
func (darwinLoader) Status(u Unit) (WatcherStatus, error) {
	path, err := UnitPath(u)
	if err != nil {
		return WatcherStatus{TaskName: u.Label()}, err
	}

	ws := WatcherStatus{TaskName: u.Label()}

	if _, serr := os.Stat(path); !os.IsNotExist(serr) {
		ws.Installed = true
	}

	// launchctl list <label> prints: <pid | -> <exit> <label>
	out, err := exec.Command("launchctl", "list", u.Label()).Output()
	if err != nil {
		return ws, nil // not loaded — not running
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) >= 1 && fields[0] != "-" {
		ws.Running = true
		if pid, perr := strconv.Atoi(fields[0]); perr == nil {
			ws.PID = pid
		}
	}
	return ws, nil
}
