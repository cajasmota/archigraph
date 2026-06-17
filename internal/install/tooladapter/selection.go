package tooladapter

import (
	"fmt"
	"sort"
	"strings"
)

// ParseToolsFlag parses the comma-separated value of `grafel install --tools`
// (e.g. "claude,windsurf,cursor") into a validated, de-duplicated, order-
// preserving slice of adapter IDs.
//
// Each token is trimmed and lower-cased before validation. Empty tokens
// (e.g. trailing commas) are ignored. Every remaining token must match a
// registered adapter ID — the first unknown token yields a clear error that
// lists the valid IDs. An input that is empty/whitespace-only yields an
// error too: a user who passed --tools meant to select something, so silently
// falling back to "all tools" would be surprising. (The no-flag case never
// reaches here.)
func ParseToolsFlag(raw string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, tok := range strings.Split(raw, ",") {
		id := strings.ToLower(strings.TrimSpace(tok))
		if id == "" {
			continue
		}
		if _, ok := Lookup(id); !ok {
			return nil, fmt.Errorf("unknown tool %q; valid tools: %s",
				id, strings.Join(AllIDs(), ", "))
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--tools requires at least one tool; valid tools: %s",
			strings.Join(AllIDs(), ", "))
	}
	return out, nil
}

// DetectedIDs returns the set of adapter IDs whose DetectInstalled() reports
// the tool is present on this machine, in registry order. Used by the CLI
// wizard to pre-check likely tools.
func DetectedIDs() []string {
	var out []string
	for _, a := range adapters() {
		if a.DetectInstalled() {
			out = append(out, a.ID())
		}
	}
	return out
}

// WizardChoice is one row presented by the interactive `--tools` wizard: a
// stable adapter ID, its human label, and whether DetectInstalled() flagged
// it (so the UI can pre-check it and show a "(detected)" hint).
type WizardChoice struct {
	ID          string
	DisplayName string
	Detected    bool
	// PreChecked is the initial checkbox state. It mirrors Detected today but
	// is kept distinct so callers can seed from an existing config selection
	// instead (e.g. re-running the wizard on an already-configured group).
	PreChecked bool
}

// WizardChoices builds the ordered choice list for the interactive selector.
// When preselected is non-nil it seeds PreChecked from that set (a previously
// persisted GroupConfig.Tools); otherwise PreChecked mirrors DetectInstalled.
// Detected is always the raw detection signal regardless of preselected, so
// the UI can show "(detected)" independently of the initial check state.
func WizardChoices(preselected []string) []WizardChoice {
	var pre map[string]bool
	if preselected != nil {
		pre = map[string]bool{}
		for _, id := range preselected {
			pre[id] = true
		}
	}
	var out []WizardChoice
	for _, a := range adapters() {
		det := a.DetectInstalled()
		checked := det
		if pre != nil {
			checked = pre[a.ID()]
		}
		out = append(out, WizardChoice{
			ID:          a.ID(),
			DisplayName: a.DisplayName(),
			Detected:    det,
			PreChecked:  checked,
		})
	}
	return out
}

// NormalizeSelection takes a raw selection (e.g. the IDs a user toggled on in
// the wizard) and returns it filtered to known adapter IDs, de-duplicated,
// and re-ordered to match the registry order. Unknown IDs are dropped. This
// is the pure core of "what did the user pick" — fed simulated toggles it is
// fully testable without a TTY.
func NormalizeSelection(selected []string) []string {
	want := map[string]bool{}
	for _, id := range selected {
		want[id] = true
	}
	var out []string
	for _, a := range adapters() {
		if want[a.ID()] {
			out = append(out, a.ID())
		}
	}
	return out
}

// ToolDelta is the difference between a previously-enabled tool set and a new
// one: which adapter IDs are newly enabled (their artifacts must be written)
// and which are newly disabled (their artifacts must be removed). Both lists
// are in registry order. Unchanged tools appear in neither list.
type ToolDelta struct {
	Enabled  []string // present in next, absent from prev
	Disabled []string // present in prev, absent from next
}

// ComputeDelta resolves prev and next through EnabledTools semantics is NOT
// applied here — callers pass the already-resolved explicit ID sets so the
// delta reflects the literal config change (an empty next means "disable
// everything", distinct from the EnabledTools back-compat fallback). Both
// inputs are normalized to known IDs in registry order before diffing.
func ComputeDelta(prev, next []string) ToolDelta {
	prevSet := map[string]bool{}
	for _, id := range NormalizeSelection(prev) {
		prevSet[id] = true
	}
	nextSet := map[string]bool{}
	for _, id := range NormalizeSelection(next) {
		nextSet[id] = true
	}
	var d ToolDelta
	for _, a := range adapters() {
		id := a.ID()
		switch {
		case nextSet[id] && !prevSet[id]:
			d.Enabled = append(d.Enabled, id)
		case prevSet[id] && !nextSet[id]:
			d.Disabled = append(d.Disabled, id)
		}
	}
	return d
}

// SortedIDs returns a lexicographically sorted copy of ids (handy for stable
// test assertions and display).
func SortedIDs(ids []string) []string {
	out := append([]string{}, ids...)
	sort.Strings(out)
	return out
}
