//go:build !windows

package service

import (
	"fmt"
	"os"
)

// resolvePlatformPaths fills in SocketPath and LogDir for Unix platforms
// using ~/.grafel as the base directory.
func resolvePlatformPaths(opts *Options) error {
	if opts.SocketPath != "" && opts.LogDir != "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("os.UserHomeDir: %w", err)
	}
	if opts.SocketPath == "" {
		opts.SocketPath = home + "/.grafel/sockets/daemon.sock"
	}
	if opts.LogDir == "" {
		opts.LogDir = home + "/.grafel/logs"
	}
	return nil
}
