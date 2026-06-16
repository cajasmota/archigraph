package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// buildWindowsPipeName derives the root-scoped Windows named-pipe path from a
// raw username and a daemon root directory. It is split out from
// WindowsPipeName (which is //go:build windows because it calls
// os/user.Current) so the name-derivation logic can be unit-tested on any
// GOOS — the selftest wedge it fixes (#5264) is a Windows-only failure but the
// derivation is pure string logic.
//
// Form: \\.\pipe\grafel-daemon-<username>-<rootHash>
//
// The username is lower-cased and stripped of any domain prefix so that
// "DOMAIN\User" → "user". The root hash makes the pipe name unique per daemon
// root, mirroring the Unix transport where the socket already lives under the
// root at <root>/sockets/daemon.sock. An empty root degrades to the legacy
// user-only name so a degenerate caller still produces a valid, stable path.
func buildWindowsPipeName(username, root string) string {
	if username == "" {
		username = "daemon"
	}
	// Strip domain prefix (e.g. "DESKTOP-123\alice" → "alice").
	if idx := strings.LastIndex(username, `\`); idx >= 0 {
		username = username[idx+1:]
	}
	username = strings.ToLower(username)

	name := `\\.\pipe\grafel-daemon-` + username
	if h := rootHash(root); h != "" {
		name += "-" + h
	}
	return name
}

// rootHash returns a short, deterministic, pipe-name-safe identifier for a
// daemon root directory, or "" when root is empty.
//
// The path is lexically cleaned and lower-cased before hashing so that the
// listen side and the dial side — which may pass casing-variant or
// non-canonical spellings of the same root on case-insensitive NTFS — always
// derive the same pipe name. (This mirrors the intent of the daemon package's
// repoStateHash/canonicalizePath, kept self-contained here to avoid an import
// cycle, since the daemon package imports transport.)
//
// 16 hex chars (64 bits) is collision-resistant for the handful of daemon
// roots on any single host and keeps the full pipe name well under the
// Windows ~256-char pipe-name limit.
func rootHash(root string) string {
	if root == "" {
		return ""
	}
	canonical := strings.ToLower(filepath.Clean(root))
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:8]) // 16 hex chars
}
