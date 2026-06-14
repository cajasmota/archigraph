// Package caps provides runtime-reloadable CPU / concurrency caps for the
// daemon (issue #5137, follow-up to #5135).
//
// # Problem
//
// #5135 (PR #5136) made the CPU/concurrency knobs env-driven, but env vars are
// captured at process start and cannot be changed in a running daemon — every
// tweak required `archigraph restart`. The original #5135 ask was to let the
// operator change the caps "at any time".
//
// # Design
//
// caps adds a small JSON config file, `~/.archigraph/cpu.json`, that the daemon
// re-reads cheaply on demand. The precedence the daemon resolves each cap by is:
//
//	explicit flag/field  >  env var  >  config file  >  built-in default
//
// The config file is the only RUNTIME-mutable surface: editing it and either
// SIGHUP'ing the daemon (for the in-process daemon GOMAXPROCS) or simply
// triggering the next reindex (for the per-subprocess extract caps) applies the
// new value with no restart.
//
// # Cheap re-read
//
// The config file sits on the reindex hot path, so the Store caches the parsed
// file keyed on its (mtime, size) and only re-parses when the file actually
// changes. A missing file is a valid state (all caps fall through to env →
// default) and is cached as such so a steady "no cpu.json" daemon does not
// stat-and-fail on every reindex beyond the first.
//
// The Store is safe for concurrent use: Load() may be called from the scheduler
// worker pool and the SIGHUP handler simultaneously.
package caps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// FileName is the basename of the runtime cap config under the daemon root.
const FileName = "cpu.json"

// Config is the JSON schema of cpu.json. Every field is a pointer so the
// daemon can distinguish "unset" (fall through to env/default) from an explicit
// zero. A non-positive value is treated as unset by the resolvers.
type Config struct {
	// ExtractGOMAXPROCS overrides ARCHIGRAPH_EXTRACT_GOMAXPROCS — the
	// per-subprocess GOMAXPROCS cap for BACKGROUND (watch/churn) reindexes.
	ExtractGOMAXPROCS *int `json:"extract_gomaxprocs,omitempty"`
	// RebuildGOMAXPROCS overrides ARCHIGRAPH_REBUILD_GOMAXPROCS — the
	// per-subprocess cap for EXPLICIT foreground rebuilds.
	RebuildGOMAXPROCS *int `json:"rebuild_gomaxprocs,omitempty"`
	// ExtractConcurrency overrides ARCHIGRAPH_EXTRACT_CONCURRENCY — the number
	// of concurrent extract subprocesses.
	ExtractConcurrency *int `json:"extract_concurrency,omitempty"`
	// DaemonGOMAXPROCS overrides ARCHIGRAPH_DAEMON_GOMAXPROCS — the daemon's
	// own in-process Go-runtime parallelism. Applied live via runtime.GOMAXPROCS
	// on SIGHUP.
	DaemonGOMAXPROCS *int `json:"daemon_gomaxprocs,omitempty"`
}

// fileKey identifies a file version cheaply (mtime + size). A zero key means
// "no file present" (or never loaded).
type fileKey struct {
	modUnixNano int64
	size        int64
}

// Store caches the parsed cpu.json keyed on its (mtime,size) so the daemon can
// re-read it on the reindex hot path without re-parsing JSON on every call.
type Store struct {
	path string

	mu     sync.Mutex
	key    fileKey
	cfg    Config
	loaded bool
}

// NewStore returns a Store backed by the cpu.json at path. path is typically
// filepath.Join(layout.Root, FileName). A Store with an empty path is valid and
// always resolves to the empty Config (every cap falls through to env/default).
func NewStore(path string) *Store {
	return &Store{path: path}
}

// DefaultPath returns the conventional cpu.json path under daemonRoot.
func DefaultPath(daemonRoot string) string {
	if daemonRoot == "" {
		return ""
	}
	return filepath.Join(daemonRoot, FileName)
}

// Load returns the current Config, re-parsing cpu.json only when the file's
// (mtime,size) has changed since the last Load. A missing/empty/unreadable file
// yields the zero Config and a nil error — absence is a valid state. A present
// but malformed file returns the last good Config (or zero) plus the parse
// error so the caller can log it; the daemon never wedges on bad JSON.
func (s *Store) Load() (Config, error) {
	if s == nil || s.path == "" {
		return Config{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	fi, statErr := os.Stat(s.path)
	if statErr != nil {
		// Missing file is the steady-state "no overrides" case. Cache the
		// empty config under the zero key so we don't re-stat-and-fail-parse
		// pointlessly, and report no error.
		if os.IsNotExist(statErr) {
			s.key = fileKey{}
			s.cfg = Config{}
			s.loaded = true
			return Config{}, nil
		}
		// Other stat errors (permissions, IO): return last good config, surface
		// the error so the caller can log once.
		if s.loaded {
			return s.cfg, statErr
		}
		return Config{}, statErr
	}

	cur := fileKey{modUnixNano: fi.ModTime().UnixNano(), size: fi.Size()}
	if s.loaded && cur == s.key {
		return s.cfg, nil // unchanged — serve cached parse.
	}

	raw, readErr := os.ReadFile(s.path)
	if readErr != nil {
		if s.loaded {
			return s.cfg, readErr
		}
		return Config{}, readErr
	}
	var parsed Config
	if err := json.Unmarshal(raw, &parsed); err != nil {
		// Keep last good config; do NOT advance the key, so a corrected file is
		// re-read on the next Load.
		if s.loaded {
			return s.cfg, err
		}
		return Config{}, err
	}
	s.key = cur
	s.cfg = parsed
	s.loaded = true
	return parsed, nil
}

// posval dereferences a *int override, returning it only when strictly
// positive. nil or non-positive yields 0 ("unset").
func posval(p *int) int {
	if p == nil || *p <= 0 {
		return 0
	}
	return *p
}

// ExtractGOMAXPROCS returns the config-file override for the background extract
// cap, or 0 when unset.
func (c Config) ExtractGOMAXPROCSValue() int { return posval(c.ExtractGOMAXPROCS) }

// RebuildGOMAXPROCSValue returns the config-file override for the foreground
// rebuild cap, or 0 when unset.
func (c Config) RebuildGOMAXPROCSValue() int { return posval(c.RebuildGOMAXPROCS) }

// ExtractConcurrencyValue returns the config-file override for the subprocess
// fan-out, or 0 when unset.
func (c Config) ExtractConcurrencyValue() int { return posval(c.ExtractConcurrency) }

// DaemonGOMAXPROCSValue returns the config-file override for the daemon's own
// GOMAXPROCS, or 0 when unset.
func (c Config) DaemonGOMAXPROCSValue() int { return posval(c.DaemonGOMAXPROCS) }
