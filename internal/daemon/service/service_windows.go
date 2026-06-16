//go:build windows

package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cajasmota/grafel/internal/daemon/transport"
)

// resolvePlatformPaths fills in SocketPath and LogDir for Windows using
// %APPDATA%\grafel as the base directory and a named-pipe path for
// SocketPath.
func resolvePlatformPaths(opts *Options) error {
	// Resolve the daemon root the same way daemon.DefaultLayout does so the
	// installed service's named pipe is scoped to the same root the daemon
	// and client derive (issue #5264). Honour GRAFEL_DAEMON_ROOT first, then
	// fall back to %APPDATA%\grafel (or the home dir when APPDATA is unset).
	root := os.Getenv("GRAFEL_DAEMON_ROOT")
	if root == "" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("os.UserHomeDir: %w", err)
			}
			appData = home
		}
		root = filepath.Join(appData, "grafel")
	}

	if opts.SocketPath == "" {
		opts.SocketPath = transport.WindowsPipeName(root)
	}
	if opts.LogDir == "" {
		opts.LogDir = filepath.Join(root, "logs")
	}
	return nil
}
