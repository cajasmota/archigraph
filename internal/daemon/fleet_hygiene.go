// fleet_hygiene.go — fleet config path validation and store generation pruning.
//
// Issue #2084: Relative paths in fleet config re-resolve against the daemon's
// cwd when the original worktree is deleted, causing infinite watcher respawns.
// ResolveFleетRepoPaths turns every path absolute and drops entries whose
// directory does not exist, emitting a warning log line for each skip.
//
// Issue #2085: Each content-hash change creates a new store slot but never
// removes the previous one. PruneStaleGenerations sweeps old generations —
// keeping the most recent N (default 2, configurable via
// GRAFEL_STORE_KEEP_GENERATIONS) — after a successful index.
package daemon

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// defaultKeepGenerations is the number of store generations to retain per
// repo (current + one previous). Users can override via the env var
// GRAFEL_STORE_KEEP_GENERATIONS.
const defaultKeepGenerations = 2

// KeepGenerations returns the configured number of store generations to keep.
// It reads GRAFEL_STORE_KEEP_GENERATIONS and falls back to defaultKeepGenerations.
func KeepGenerations() int {
	if v := os.Getenv("GRAFEL_STORE_KEEP_GENERATIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			return n
		}
	}
	return defaultKeepGenerations
}

// ResolveFleetRepoPaths takes a raw list of repo paths (as stored in fleet
// config — potentially relative), resolves each to an absolute path, and
// returns only those that point to an existing directory. Entries that cannot
// be resolved or whose directory does not exist are dropped and logged at Warn
// level so the daemon never spawns a watcher for a deleted worktree (#2084).
//
// logger may be nil; in that case skipped paths are silently dropped.
func ResolveFleetRepoPaths(rawPaths []string, logger *slog.Logger) []string {
	out := make([]string, 0, len(rawPaths))
	for _, raw := range rawPaths {
		if raw == "" {
			continue
		}
		abs, err := filepath.Abs(raw)
		if err != nil {
			if logger != nil {
				logger.Warn("fleet: cannot resolve repo path — skipping",
					"raw_path", raw, "err", err)
			}
			continue
		}
		abs = filepath.Clean(abs)
		fi, err := os.Stat(abs)
		if err != nil || !fi.IsDir() {
			if logger != nil {
				logger.Warn("fleet: repo path does not exist or is not a directory — skipping watcher (deleted worktree?)",
					"path", abs)
			}
			continue
		}
		out = append(out, abs)
	}
	return out
}

// PruneStaleGenerations scans storeDir for old generation slots of any repo
// whose canonical path appears in activePaths. For each repo basename, all
// slug-hash slots that share the same base name but whose mtime is not among
// the keepN most recent are deleted.
//
// The function is deliberately conservative:
//   - It only removes directories that match the "<base>-<16hex>" naming
//     convention produced by repoSlug.
//   - It always keeps at least keepN entries even when their paths are not in
//     activePaths (i.e. it prunes ALL repos in the store, not just active ones,
//     but still honours keepN so recent index data for removed repos is not
//     immediately discarded).
//   - Errors during removal are logged but never returned — a failed prune is
//     non-fatal.
//
// Returns the number of directories removed and the total bytes freed.
func PruneStaleGenerations(storeDir string, keepN int, logger *slog.Logger) (removed int, freedBytes int64) {
	if keepN < 1 {
		keepN = defaultKeepGenerations
	}
	if storeDir == "" {
		return
	}
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		return
	}

	// Group store entries by base name (the part before the last "-<16hex>" suffix).
	type slotInfo struct {
		name  string // full directory name
		mtime int64  // most recent artifact mtime inside the slot (ns)
	}
	byBase := make(map[string][]slotInfo)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		base, ok := extractSlotBase(name)
		if !ok {
			continue
		}
		mtime := latestMtime(filepath.Join(storeDir, name))
		byBase[base] = append(byBase[base], slotInfo{name: name, mtime: mtime})
	}

	for _, slots := range byBase {
		if len(slots) <= keepN {
			continue // nothing to prune
		}
		// Sort newest first (highest mtime first).
		sort.Slice(slots, func(i, j int) bool {
			return slots[i].mtime > slots[j].mtime
		})
		// Delete everything after the first keepN.
		for _, s := range slots[keepN:] {
			dir := filepath.Join(storeDir, s.name)
			sz, _ := dirSizeHygiene(dir)
			if err := os.RemoveAll(dir); err != nil {
				if logger != nil {
					logger.Warn("store: prune old generation — remove failed (non-fatal)",
						"dir", dir, "err", err)
				}
				continue
			}
			if logger != nil {
				logger.Info("store: pruned old generation",
					"dir", dir, "freed_bytes", sz)
			}
			removed++
			freedBytes += sz
		}
	}
	return
}

// extractSlotBase extracts the human-readable base from a store slot name of
// the form "<base>-<16hex>". Returns ("", false) when the name does not match.
func extractSlotBase(name string) (string, bool) {
	// A valid slot ends with "-<exactly 16 hex chars>".
	if len(name) < 18 { // at least 1 char base + "-" + 16 hex
		return "", false
	}
	dash := len(name) - 17 // index of "-" before the 16-hex suffix
	if name[dash] != '-' {
		return "", false
	}
	suffix := name[dash+1:]
	for _, c := range suffix {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return "", false
		}
	}
	base := name[:dash]
	if base == "" {
		return "", false
	}
	return base, true
}

// latestMtime returns the most recent modification time (ns since epoch) of
// any file inside dir. Useful for ranking generations when the slot directory's
// own mtime may be stale on some filesystems.
func latestMtime(dir string) int64 {
	var latest int64
	// Recurse at most one level (refs/<ref>/) to capture per-ref graph files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		if fi, sterr := os.Stat(dir); sterr == nil {
			return fi.ModTime().UnixNano()
		}
		return 0
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if t := info.ModTime().UnixNano(); t > latest {
			latest = t
		}
		if e.IsDir() {
			// One additional level (refs/<ref>/) only.
			sub := filepath.Join(dir, e.Name())
			subs, serr := os.ReadDir(sub)
			if serr != nil {
				continue
			}
			for _, se := range subs {
				si, err := se.Info()
				if err != nil {
					continue
				}
				if t := si.ModTime().UnixNano(); t > latest {
					latest = t
				}
			}
		}
	}
	return latest
}

// dirSizeHygiene returns the total byte count of a directory tree.
// Separate from daemon.dirSize (in service.go) to avoid a naming collision
// within the package — both live in package daemon but in different files.
func dirSizeHygiene(dir string) (int64, error) {
	var total int64
	err := filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total, err
}

// slugBaseName returns the human-readable base component of a store slot name
// for the given absolute repo path. Exported so tests can construct expected
// store slot prefixes without replicating the slug formula.
func slugBaseName(absRepoPath string) string {
	base := filepath.Base(absRepoPath)
	base = unsafeSlugChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-._")
	if base == "" {
		base = "repo"
	}
	if len(base) > 48 {
		base = base[:48]
	}
	return base
}
