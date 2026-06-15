package cli

// ref_resolver.go — shared --ref flag resolution for CLI commands (issue #2219).
//
// The --ref flag is added to: status, rebuild, index, list, doctor, remove.
//
// Sentinel values:
//   - ""         (flag not set) — behaves identically to @current: uses the
//                 current HEAD ref. Zero regressions for existing callers.
//   - "@current" — explicit alias for the current HEAD; normalised to "".
//   - "@all"     — read-only commands iterate all known refs; destructive
//                 commands (rebuild, index, remove) refuse with a clear error.
//   - anything else — a named ref (branch / tag). The resolver validates it
//                 exists in the store and returns a clean error if not.
//
// Public surface (used by the 6 command files):
//
//	resolveRef(rawRef string, allowAll bool) (resolved string, isAll bool, err error)
//	knownRefNames() ([]string, error)
//	refFlagUsage string

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cajasmota/grafel/internal/daemon"
	"github.com/cajasmota/grafel/internal/registry"
)

// refFlagUsage is the canonical --ref help string shared across all commands
// that support the flag.
const refFlagUsage = `operate on a specific git ref (branch/tag).
Use @current for the active HEAD (default), or @all for all known refs
(read-only commands only: list, doctor, status).`

// resolveRef normalises a raw --ref flag value and validates it.
//
//   - rawRef == "" or rawRef == "@current" → returns ("", false, nil).
//     Callers interpret "" as "use the current HEAD" — identical to
//     pre-#2219 behaviour.
//   - rawRef == "@all" and allowAll == true → returns ("", true, nil).
//   - rawRef == "@all" and allowAll == false → returns an error.
//   - any other rawRef → validates the ref exists in the store; returns
//     (rawRef, false, nil) on success or an error listing available refs.
func resolveRef(rawRef string, allowAll bool) (resolved string, isAll bool, err error) {
	switch rawRef {
	case "", "@current":
		return "", false, nil
	case "@all":
		if !allowAll {
			return "", false, fmt.Errorf(
				"--ref @all is only supported on read-only commands (list, doctor, status); " +
					"this command operates on a specific ref — use a named ref or @current",
			)
		}
		return "", true, nil
	}

	// Named ref: validate it exists in the store.
	known, listErr := knownRefNames()
	if listErr != nil {
		// If we can't enumerate the store, accept the ref optimistically;
		// the actual RPC or file access will fail with a useful error if
		// the ref doesn't exist.
		return rawRef, false, nil
	}

	for _, k := range known {
		if k == rawRef {
			return rawRef, false, nil
		}
	}

	// Not found.
	if len(known) == 0 {
		return "", false, fmt.Errorf(
			"ref %q not found in the graph store (no refs indexed yet; run 'grafel index')",
			rawRef,
		)
	}
	return "", false, fmt.Errorf(
		"ref %q not found; known refs: %s",
		rawRef, formatRefList(known),
	)
}

// knownRefNames returns all ref names that have a state directory in the
// graph store across all registered groups. The list is deduplicated and
// sorted. Returns nil (not an error) when the store is empty.
func knownRefNames() ([]string, error) {
	groups, err := registry.Groups()
	if err != nil {
		return nil, fmt.Errorf("registry: %w", err)
	}

	storeRoot := daemon.StoreDir()
	seen := make(map[string]struct{})

	for _, g := range groups {
		cfg, cfgErr := registry.LoadGroupConfig(g.ConfigPath)
		if cfgErr != nil {
			continue // skip misconfigured groups
		}
		for _, repo := range cfg.Repos {
			repoBase := repoBaseForSlug(storeRoot, repo)
			refsDir := filepath.Join(repoBase, "refs")
			entries, rdErr := os.ReadDir(refsDir)
			if rdErr != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				ref := daemon.RefSafeDecode(e.Name())
				if ref == "" {
					continue // skip _unknown sentinel
				}
				seen[ref] = struct{}{}
			}
		}
	}

	if len(seen) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	sort.Strings(names)
	return names, nil
}

// formatRefList formats a slice of ref names for inline display in errors.
// Caps at 10 entries to keep error messages readable.
func formatRefList(refs []string) string {
	const maxShow = 10
	if len(refs) <= maxShow {
		result := ""
		for i, r := range refs {
			if i > 0 {
				result += ", "
			}
			result += r
		}
		return result
	}
	result := ""
	for i := 0; i < maxShow; i++ {
		if i > 0 {
			result += ", "
		}
		result += refs[i]
	}
	return fmt.Sprintf("%s … (%d more)", result, len(refs)-maxShow)
}
