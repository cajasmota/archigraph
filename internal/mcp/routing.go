package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// resolveGroup implements the ADR-0008 cascade:
//
//  1. explicit `group` argument
//  2. CWD inference (walk up looking for .archigraph/group.json)
//  3. singleton-group fallback
//
// Returns the chosen group name, the source ("explicit"/"cwd"/"singleton"),
// or an error listing the registered groups when ambiguous.
func resolveGroup(s *State, explicit, cwd string) (string, string, error) {
	if explicit != "" {
		return explicit, "explicit", nil
	}
	if g := groupFromCWD(cwd); g != "" {
		// only honor it if the registry knows about it
		if _, ok := s.registry.Groups[g]; ok {
			return g, "cwd", nil
		}
	}
	if len(s.registry.Groups) == 1 {
		for g := range s.registry.Groups {
			return g, "singleton", nil
		}
	}
	if len(s.registry.Groups) == 0 {
		return "", "", errors.New("no groups registered (registry is empty)")
	}
	known := make([]string, 0, len(s.registry.Groups))
	for g := range s.registry.Groups {
		known = append(known, g)
	}
	return "", "", errors.New("ambiguous group; pass `group=<name>`. registered groups: " + strings.Join(known, ", "))
}

// groupFromCWD walks dir upward looking for .archigraph/group.json which
// encodes {"group": "<name>"}.
func groupFromCWD(dir string) string {
	if dir == "" {
		return ""
	}
	cur := dir
	for {
		marker := filepath.Join(cur, ".archigraph", "group.json")
		if data, err := os.ReadFile(marker); err == nil {
			var doc struct {
				Group string `json:"group"`
			}
			if err := json.Unmarshal(data, &doc); err == nil && doc.Group != "" {
				return doc.Group
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

// repoFromCWD walks dir upward looking for the repo's .archigraph dir; the
// repo's directory name is returned if found.
func repoFromCWD(dir string) string {
	if dir == "" {
		return ""
	}
	cur := dir
	for {
		if _, err := os.Stat(filepath.Join(cur, ".archigraph")); err == nil {
			return filepath.Base(cur)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}
