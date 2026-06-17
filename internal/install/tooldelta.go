package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cajasmota/grafel/internal/install/mcpreg"
	"github.com/cajasmota/grafel/internal/install/rulesfiles"
	"github.com/cajasmota/grafel/internal/install/tooladapter"
	"github.com/cajasmota/grafel/internal/registry"
)

// ToolDeltaOps abstracts the four filesystem primitives ApplyToolDelta needs
// so tests can inject mocks and never touch the real (live) machine. The
// production wiring (defaultToolDeltaOps) delegates to rulesfiles + mcpreg.
type ToolDeltaOps struct {
	// WriteRules writes the marker block into the given rules-file targets of
	// a repo (newly-enabled tools). Mirrors Apply's rulesfiles.WriteTargets.
	WriteRules func(repoRoot string, targets []string) error
	// RemoveRules strips the marker block from the given rules-file targets of
	// a repo (newly-disabled tools). Mirrors uninstall's RemoveTargets.
	RemoveRules func(repoRoot string, targets []string) error
	// RegisterMCP registers the grafel MCP entry for a tool (newly-enabled).
	RegisterMCP func(tool mcpreg.Tool) error
	// UnregisterMCP removes the grafel MCP entry for a tool (newly-disabled).
	UnregisterMCP func(tool mcpreg.Tool) error
}

// defaultToolDeltaOps returns the production ToolDeltaOps wired to rulesfiles
// + mcpreg, in-process (NO subprocess, NO daemon stop/start).
func defaultToolDeltaOps(group, binPath string) ToolDeltaOps {
	return ToolDeltaOps{
		WriteRules: func(repoRoot string, targets []string) error {
			_, err := rulesfiles.WriteTargets(repoRoot, rulesfiles.WriteOptions{GroupName: group}, targets)
			return err
		},
		RemoveRules: func(repoRoot string, targets []string) error {
			_, err := rulesfiles.RemoveTargets(repoRoot, targets)
			return err
		},
		RegisterMCP: func(tool mcpreg.Tool) error {
			_, err := mcpreg.Register(tool, binPath, "")
			// Missing parent dir for an uninstalled tool is fine.
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		},
		UnregisterMCP: func(tool mcpreg.Tool) error {
			err := mcpreg.Unregister(tool)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		},
	}
}

// ToolDeltaResult reports, per affected tool, which artifacts ApplyToolDelta
// touched, so the CLI can print a per-tool summary.
type ToolDeltaResult struct {
	// Enabled/Disabled echo the delta that was applied, in registry order.
	Enabled  []string
	Disabled []string
	// RulesWritten/RulesRemoved map repo path → rules-file targets touched.
	RulesWritten map[string][]string
	RulesRemoved map[string][]string
	// MCPRegistered/MCPUnregistered list the mcpreg tools touched.
	MCPRegistered   []mcpreg.Tool
	MCPUnregistered []mcpreg.Tool
}

// ApplyToolDelta applies a tool-selection delta IN-PROCESS: it writes the
// newly-enabled tools' per-repo rules files + MCP entries and removes the
// newly-disabled tools' artifacts, reusing the same rulesfiles + mcpreg
// primitives that Apply / Uninstall use. It NEVER shells out to `grafel
// install`, never restarts the daemon, and never touches OS services.
//
// Rules-file removal is computed against the SURVIVING tool set: a rules
// file (e.g. AGENTS.md) shared by two tools is only stripped when the LAST
// tool that reads it is disabled. MCP entries are likewise only unregistered
// when no surviving tool still registers that mcpreg.Tool.
//
// prevTools/nextTools are the explicit (already-resolved) enabled ID sets;
// the delta is literal (an empty nextTools disables everything). cfg supplies
// the repos to operate on. ops is injectable for testing; pass the zero value
// to use the production wiring.
func ApplyToolDelta(cfg *registry.GroupConfig, group, binPath string, prevTools, nextTools []string, ops *ToolDeltaOps) (*ToolDeltaResult, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if group == "" {
		return nil, errors.New("group is required")
	}
	var opv ToolDeltaOps
	if ops != nil {
		opv = *ops
	} else {
		opv = defaultToolDeltaOps(group, binPath)
	}

	delta := tooladapter.ComputeDelta(prevTools, nextTools)
	res := &ToolDeltaResult{
		Enabled:      delta.Enabled,
		Disabled:     delta.Disabled,
		RulesWritten: map[string][]string{},
		RulesRemoved: map[string][]string{},
	}

	// Surviving rules-file targets / MCP tools = those still read/registered
	// by a tool in nextTools. A shared artifact stays until its last owner is
	// disabled.
	survivingRules := targetsFor(nextTools)
	survivingMCP := mcpToolsFor(nextTools)

	// ── Rules files (per repo) ─────────────────────────────────────────────
	enabledRules := subtractTargets(targetsFor(delta.Enabled), nil) // all newly-enabled targets are (re)written
	// Disabled rules to remove = targets owned by disabled tools that NO
	// surviving tool still reads.
	removeRules := subtractTargets(targetsFor(delta.Disabled), survivingRules)

	for _, r := range cfg.Repos {
		repo := absRepo(r.Path)
		if len(enabledRules) > 0 {
			if err := opv.WriteRules(repo, enabledRules); err != nil {
				return nil, fmt.Errorf("write rules for %s: %w", repo, err)
			}
			res.RulesWritten[repo] = append([]string{}, enabledRules...)
		}
		if len(removeRules) > 0 {
			if err := opv.RemoveRules(repo, removeRules); err != nil {
				return nil, fmt.Errorf("remove rules for %s: %w", repo, err)
			}
			res.RulesRemoved[repo] = append([]string{}, removeRules...)
		}
	}

	// ── MCP entries (user-global, once each) ───────────────────────────────
	for _, t := range mcpToolsFor(delta.Enabled) {
		if err := opv.RegisterMCP(t); err != nil {
			return nil, fmt.Errorf("register mcp %s: %w", t, err)
		}
		res.MCPRegistered = append(res.MCPRegistered, t)
	}
	survSet := map[mcpreg.Tool]bool{}
	for _, t := range survivingMCP {
		survSet[t] = true
	}
	for _, t := range mcpToolsFor(delta.Disabled) {
		if survSet[t] {
			continue // a surviving tool still registers this entry
		}
		if err := opv.UnregisterMCP(t); err != nil {
			return nil, fmt.Errorf("unregister mcp %s: %w", t, err)
		}
		res.MCPUnregistered = append(res.MCPUnregistered, t)
	}

	return res, nil
}

// targetsFor returns the union of rules-file targets across the given tool
// IDs, ordered by rulesfiles.Targets (matching Apply's ordering).
func targetsFor(ids []string) []string {
	want := map[string]bool{}
	for _, id := range ids {
		a, ok := tooladapter.Lookup(id)
		if !ok {
			continue
		}
		for _, t := range a.RulesFileTargets() {
			want[t] = true
		}
	}
	out := make([]string, 0, len(want))
	for _, t := range rulesfiles.Targets {
		if want[t] {
			out = append(out, t)
		}
	}
	return out
}

// mcpToolsFor returns the distinct mcpreg.Tool entries registered by the
// given tool IDs, in registry order.
func mcpToolsFor(ids []string) []mcpreg.Tool {
	idSet := map[string]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	seen := map[mcpreg.Tool]bool{}
	var out []mcpreg.Tool
	for _, a := range tooladapter.All() {
		if !idSet[a.ID()] || !a.SupportsMCP() {
			continue
		}
		t := a.MCPTool()
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// subtractTargets returns targets in a that are NOT in b, preserving a's order.
func subtractTargets(a, b []string) []string {
	skip := map[string]bool{}
	for _, t := range b {
		skip[t] = true
	}
	var out []string
	for _, t := range a {
		if !skip[t] {
			out = append(out, t)
		}
	}
	return out
}

// absRepo resolves a (possibly relative) repo path to absolute, best-effort.
func absRepo(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}
