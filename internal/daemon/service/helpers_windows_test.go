//go:build windows

package service_test

import (
	"testing"
)

// renderPlist is a no-op stub on Windows: launchd plist tests only run
// on macOS. Returning a non-empty string lets the shared test file
// compile, but the tests are skipped at runtime.
func renderPlist(t *testing.T) string {
	t.Skip("launchd plist tests only run on darwin")
	return ""
}

// renderUnit is a no-op stub on Windows: systemd unit tests only run
// on Linux. Returning a non-empty string lets the shared test file
// compile, but the tests are skipped at runtime.
func renderUnit(t *testing.T) string {
	t.Skip("systemd unit tests only run on linux")
	return ""
}
