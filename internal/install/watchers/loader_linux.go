//go:build linux

package watchers

import (
	"fmt"
	"os"
	"os/exec"
)

// linuxLoader implements Loader using systemctl --user for Linux systemd units.
type linuxLoader struct{}

// NewLoader returns the Linux systemd-user-based Loader.
func NewLoader() Loader { return linuxLoader{} }

// Load enables and immediately starts the systemd user unit. The unit file
// must already exist on disk (placed by Write). If the unit is already
// running it is a no-op.
func (linuxLoader) Load(u Unit) error {
	path, err := UnitPath(u)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("unit file not found — call Write(u) first: %s", path)
	}

	// Reload the unit manager so it picks up the new file.
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user daemon-reload: %w\n%s", err, out)
	}

	// Enable + start atomically.
	out, err := exec.Command("systemctl", "--user", "enable", "--now", u.Label()+".service").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl --user enable --now %s: %w\n%s", u.Label(), err, out)
	}
	return nil
}

// Unload disables and stops the systemd user unit. Idempotent — if the unit
// is already disabled/stopped the call succeeds.
//
// "Already gone" is detected by stat-ing the unit file on disk (locale- and
// version-invariant) rather than by matching the localized `systemctl disable`
// error text ("No such file" etc.), which breaks on non-English locales. If the
// unit file is absent there is nothing to disable.
func (linuxLoader) Unload(u Unit) error {
	path, err := UnitPath(u)
	if err != nil {
		return err
	}
	if _, serr := os.Stat(path); os.IsNotExist(serr) {
		return nil // no unit file — already gone
	}
	// --now stops the unit in addition to disabling it.
	out, derr := exec.Command("systemctl", "--user", "disable", "--now", u.Label()+".service").CombinedOutput()
	if derr != nil {
		// Race: the unit file vanished between the stat above and disable.
		// Re-stat; if it is gone now, the desired absent state is reached.
		// Never match the localized error text.
		if _, serr := os.Stat(path); os.IsNotExist(serr) {
			return nil // gone now — success-to-proceed
		}
		return fmt.Errorf("systemctl --user disable %s: %w\n%s", u.Label(), derr, out)
	}
	return nil
}

// Status queries systemctl for the watcher unit state.
func (linuxLoader) Status(u Unit) (WatcherStatus, error) {
	path, err := UnitPath(u)
	if err != nil {
		return WatcherStatus{TaskName: u.Label()}, err
	}

	ws := WatcherStatus{TaskName: u.Label()}

	if _, serr := os.Stat(path); !os.IsNotExist(serr) {
		ws.Installed = true
	}

	// is-active exits 0 when the unit is active.
	if err := exec.Command("systemctl", "--user", "is-active", "--quiet", u.Label()+".service").Run(); err == nil {
		ws.Running = true
	}
	return ws, nil
}
