package mcp

import "strings"

// contextFilterSet converts the optional context_filter argument (list of
// edge kinds) into a set; empty input -> nil meaning "all kinds".
func contextFilterSet(kinds []string) map[string]bool {
	if len(kinds) == 0 {
		return nil
	}
	out := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		out[strings.ToUpper(strings.TrimSpace(k))] = true
	}
	return out
}

// stripScopePrefix drops a leading "SCOPE." from edge/kind names per ADR-0003.
func stripScopePrefix(s string) string {
	const p = "SCOPE."
	if strings.HasPrefix(s, p) {
		return s[len(p):]
	}
	return s
}
