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
//
// "Already gone" is detected via the exit code of `launchctl list <label>`
// (locale-invariant) rather than by matching the localized bootout error text
// ("No such process" etc.), which breaks on non-English macOS. If the service
// is not listed there is nothing to bootout — the desired absent state is
// already reached, so we report success without shelling out to bootout.
func (darwinLoader) Unload(u Unit) error {
	uid := strconv.Itoa(os.Getuid())
	// launchctl list <label> exits non-zero when the label is not loaded; that
	// is the locale-invariant signal that the desired absent state already holds.
	if err := exec.Command("launchctl", "list", u.Label()).Run(); err != nil {
		return nil // not loaded — already gone
	}
	if out, err := exec.Command("launchctl", "bootout", "gui/"+uid+"/"+u.Label()).CombinedOutput(); err != nil {
		// Race: the service was listed above but disappeared before bootout.
		// Re-check via the exit code of `launchctl list`; if it is now gone,
		// the desired state is reached. Never match the localized error text.
		if lerr := exec.Command("launchctl", "list", u.Label()).Run(); lerr != nil {
			return nil // gone now — success-to-proceed
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
