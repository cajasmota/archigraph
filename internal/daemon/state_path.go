// Per-repo state path resolution for issue #745.
//
// Background: ADR-0007 co-locates per-repo state in `<repo>/.archigraph/`.
// That default is preserved for ordinary user installs. When multiple
// agents run with isolated daemons via ARCHIGRAPH_DAEMON_ROOT, the
// daemon socket and registry are already isolated, but the per-repo
// state directory is shared — two agents indexing the same fixture
// race on `<repo>/.archigraph/graph.json` and corrupt each other's
// results.
//
// When ARCHIGRAPH_DAEMON_ROOT is set, StateDirForRepo returns a
// daemon-private state directory at
//
//	$ARCHIGRAPH_DAEMON_ROOT/state/<sha256(abs_repo_path)[:16]>/
//
// instead of `<repo>/.archigraph/`. The fixture's own `.archigraph/`
// directory is never touched by the daemon under this mode, so a
// pristine read-only corpus stays pristine even across many parallel
// agents.
//
// Identifier choice: sha256 of the absolute repo path, first 16 hex
// chars. Reasons:
//   - Deterministic (same input → same output across processes & hosts).
//   - Filesystem-safe (16 hex chars, no separators or shell metachars).
//   - Collision-resistant (2^64 namespace; far above any realistic
//     fixture count on a single host).
//   - Opaque (does not leak repo paths into shared tmp).
//
// Group-level config that lives co-located by design (group.json
// manifests written by the wizard) is NOT routed through this helper —
// it stays at `<repo>/.archigraph/group.json` so it can be discovered
// by walking up from a CWD regardless of which daemon is running.
package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// repoStateHash returns a deterministic, path-safe identifier for a
// repo path. Callers MUST pass an absolute, lexically-clean path
// (filepath.Abs + filepath.Clean) so that the same repo always maps
// to the same hash regardless of how the caller addressed it.
func repoStateHash(absRepoPath string) string {
	sum := sha256.Sum256([]byte(absRepoPath))
	return hex.EncodeToString(sum[:8]) // 16 hex chars
}

// homeDir resolves the archigraph home directory, honouring the
// ARCHIGRAPH_HOME override (matching registry.HomeDir) and falling
// back to ~/.archigraph. Kept dependency-light so this hot-path
// helper does not pull in the registry package.
func homeDir() string {
	if override := os.Getenv("ARCHIGRAPH_HOME"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Last-ditch fallback so we never write into a repo by accident.
		return filepath.Join(os.TempDir(), ".archigraph")
	}
	return filepath.Join(home, ".archigraph")
}

// StoreDir returns the root of the daemon's external graph store —
// the single source of truth for where generated graph artifacts live
// when no isolated ARCHIGRAPH_DAEMON_ROOT is in effect.
//
//	$ARCHIGRAPH_HOME (or ~/.archigraph)/store
//
// Issue #1626: graph artifacts (graph.fb, graph.json, enrichments,
// links, metadata) are NEVER written into the repo working tree any
// more — they live under the store, keyed by repo. Keeping them out of
// the tree (a) stops them polluting user repos and (b) breaks the
// fb-vs-json mtime-drift reindex loop, since the watcher can no longer
// observe its own output.
func StoreDir() string {
	return filepath.Join(homeDir(), "store")
}

var unsafeSlugChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// repoSlug derives a short, human-readable, path-safe label from a repo
// path so the store layout is browsable (e.g. "my-service-1a2b3c4d…").
// The trailing hash guarantees uniqueness even when two repos share a
// basename.
func repoSlug(absRepoPath string) string {
	base := filepath.Base(absRepoPath)
	base = unsafeSlugChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-._")
	if base == "" {
		base = "repo"
	}
	if len(base) > 48 {
		base = base[:48]
	}
	return base + "-" + repoStateHash(absRepoPath)
}

// StateDirForRepo returns the directory that holds per-repo state
// (graph.fb, graph.json, repair.json, enrichment-*.json, links, …) for
// repoPath.
//
// Resolution (issue #1626):
//   - When ARCHIGRAPH_DAEMON_ROOT is set (isolated daemons, parallel
//     agents, tests): `$ARCHIGRAPH_DAEMON_ROOT/state/<hash>/`. This
//     preserves the issue-#745 isolation contract.
//   - Otherwise: the external store at
//     `$ARCHIGRAPH_HOME (or ~/.archigraph)/store/<slug>-<hash>/`.
//
// Graph artifacts are NO LONGER written into `<repo>/.archigraph/`.
// Pre-existing in-repo state is relocated transparently by
// MigrateInRepoState (called from the load path).
//
// The directory is NOT created here; callers that write should
// os.MkdirAll the returned path.
func StateDirForRepo(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		abs = repoPath
	}
	abs = filepath.Clean(abs)

	if root := os.Getenv(EnvRoot); root != "" {
		return filepath.Join(root, "state", repoStateHash(abs))
	}
	return filepath.Join(StoreDir(), repoSlug(abs))
}

// LegacyInRepoStateDir returns the historical co-located state directory
// `<repo>/.archigraph/`. Used only by the migration path to find and
// relocate pre-#1626 artifacts. New code MUST use StateDirForRepo.
func LegacyInRepoStateDir(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	return filepath.Join(repoPath, ".archigraph")
}

// GraphPathForRepo is a convenience wrapper that returns the
// canonical graph.json path inside the per-repo state directory.
func GraphPathForRepo(repoPath string) string {
	return filepath.Join(StateDirForRepo(repoPath), "graph.json")
}

// GraphFBPathForRepo returns the path to graph.fb (FlatBuffers binary format)
// inside the per-repo state directory.
func GraphFBPathForRepo(repoPath string) string {
	return filepath.Join(StateDirForRepo(repoPath), "graph.fb")
}

// FindGraphFile checks for the newest graph file (graph.fb preferred over
// graph.json) and returns its path and modification time. Returns ("", 0)
// if neither file exists. The returned modtime is in nanoseconds since epoch.
func FindGraphFile(repoPath string) (path string, modtime int64) {
	stateDir := StateDirForRepo(repoPath)

	fbPath := filepath.Join(stateDir, "graph.fb")
	jsonPath := filepath.Join(stateDir, "graph.json")

	fbInfo, fbErr := os.Stat(fbPath)
	jsonInfo, jsonErr := os.Stat(jsonPath)

	// Check graph.fb first (preferred)
	if fbErr == nil {
		fbMtime := fbInfo.ModTime().UnixNano()
		// If both exist, return whichever is newer
		if jsonErr == nil {
			jsonMtime := jsonInfo.ModTime().UnixNano()
			if fbMtime >= jsonMtime {
				return fbPath, fbMtime
			}
			return jsonPath, jsonMtime
		}
		return fbPath, fbMtime
	}

	// Fall back to graph.json if graph.fb doesn't exist
	if jsonErr == nil {
		return jsonPath, jsonInfo.ModTime().UnixNano()
	}

	return "", 0
}
