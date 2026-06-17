package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cajasmota/grafel/internal/install"
	"github.com/cajasmota/grafel/internal/install/tooladapter"
	"github.com/cajasmota/grafel/internal/registry"
)

// newToolsCmd returns the `grafel tools` command group (#5256): inspect and
// change which AI coding tools grafel targets for a group, applying the
// per-tool artifact delta IN-PROCESS (no `grafel install` subprocess, no
// daemon restart).
func newToolsCmd() *cobra.Command {
	var group string

	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List or change the AI coding tools grafel targets",
		Long: `tools inspects and changes which AI coding tools grafel installs
into for a group (rules files, MCP entries, skills/hooks).

  grafel tools                 list all tools with enabled/detected state
  grafel tools list            (same as above)
  grafel tools enable <id...>  enable tools and write their artifacts
  grafel tools disable <id...> disable tools and remove their artifacts

enable/disable update GroupConfig.Tools and re-apply only the changed tools'
artifacts in-process — they never run 'grafel install' as a subprocess and
never stop/start the daemon.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runToolsList(cmd.OutOrStdout(), group)
		},
	}
	cmd.PersistentFlags().StringVar(&group, "group", "", "group name (default: the only registered group)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all tools with enabled/detected state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runToolsList(cmd.OutOrStdout(), group)
		},
	}

	enableCmd := &cobra.Command{
		Use:   "enable <id...>",
		Short: "Enable tools and write their artifacts",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runToolsToggle(cmd.OutOrStdout(), group, args, true)
		},
	}

	disableCmd := &cobra.Command{
		Use:   "disable <id...>",
		Short: "Disable tools and remove their artifacts",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runToolsToggle(cmd.OutOrStdout(), group, args, false)
		},
	}

	cmd.AddCommand(listCmd, enableCmd, disableCmd)
	return cmd
}

// runToolsList prints every registered adapter with its enabled state (for
// the resolved group) and its DetectInstalled() signal.
func runToolsList(out io.Writer, group string) error {
	name, err := resolveGroup(group)
	if err != nil {
		return err
	}
	cfg, err := loadGroupConfigByName(name)
	if err != nil {
		return err
	}
	enabled := map[string]bool{}
	for _, id := range tooladapter.EnabledTools(cfg) {
		enabled[id] = true
	}
	explicit := len(cfg.Tools) > 0

	fmt.Fprintf(out, "tools for group %q", name)
	if !explicit {
		fmt.Fprintf(out, " (no explicit selection — all tools enabled by default)")
	}
	fmt.Fprintln(out, ":")
	for _, a := range tooladapter.All() {
		state := "disabled"
		if enabled[a.ID()] {
			state = "enabled"
		}
		det := ""
		if a.DetectInstalled() {
			det = "  (detected)"
		}
		fmt.Fprintf(out, "  %-12s %-8s %s%s\n", a.ID(), state, a.DisplayName(), det)
	}
	return nil
}

// runToolsToggle enables or disables the given tool IDs for the resolved
// group: it validates the IDs, updates GroupConfig.Tools, persists it, and
// re-applies the artifact delta in-process via install.ApplyToolDelta.
func runToolsToggle(out io.Writer, group string, ids []string, enable bool) error {
	for _, id := range ids {
		if _, ok := tooladapter.Lookup(strings.ToLower(strings.TrimSpace(id))); !ok {
			return fmt.Errorf("unknown tool %q; valid tools: %s",
				id, strings.Join(tooladapter.AllIDs(), ", "))
		}
	}
	name, err := resolveGroup(group)
	if err != nil {
		return err
	}
	cfgPath, err := registeredConfigPath(name)
	if err != nil {
		return err
	}
	cfg, err := registry.LoadGroupConfig(cfgPath)
	if err != nil || cfg == nil {
		return fmt.Errorf("load group %q config: %w", name, err)
	}

	prev := tooladapter.EnabledTools(cfg)
	next := applyToggle(prev, ids, enable)

	if equalIDSet(prev, next) {
		fmt.Fprintf(out, "no change: tools already %v\n", tooladapter.SortedIDs(next))
		return nil
	}

	cfg.Tools = next
	if err := registry.SaveGroupConfig(cfgPath, cfg); err != nil {
		return fmt.Errorf("save group config: %w", err)
	}

	bin, _ := os.Executable()
	res, err := install.ApplyToolDelta(cfg, name, bin, prev, next, nil)
	if err != nil {
		return fmt.Errorf("apply tool delta: %w", err)
	}

	verb := "enabled"
	if !enable {
		verb = "disabled"
	}
	fmt.Fprintf(out, "%s %v for group %q\n", verb, ids, name)
	fmt.Fprintf(out, "  tools now: %v\n", next)
	if len(res.Enabled) > 0 {
		fmt.Fprintf(out, "  wrote artifacts for: %v\n", res.Enabled)
	}
	if len(res.Disabled) > 0 {
		fmt.Fprintf(out, "  removed artifacts for: %v\n", res.Disabled)
	}
	for repo, t := range res.RulesWritten {
		fmt.Fprintf(out, "    %s: wrote %v\n", repo, t)
	}
	for repo, t := range res.RulesRemoved {
		fmt.Fprintf(out, "    %s: removed %v\n", repo, t)
	}
	if len(res.MCPRegistered) > 0 {
		fmt.Fprintf(out, "    MCP registered: %v\n", res.MCPRegistered)
	}
	if len(res.MCPUnregistered) > 0 {
		fmt.Fprintf(out, "    MCP unregistered: %v\n", res.MCPUnregistered)
	}
	return nil
}

// applyToggle returns the new explicit tool-ID set after enabling (or
// disabling) the given ids relative to prev. The result is normalized to
// known IDs in registry order. Pure: testable without any FS.
func applyToggle(prev, ids []string, enable bool) []string {
	set := map[string]bool{}
	for _, id := range prev {
		set[id] = true
	}
	for _, id := range ids {
		set[strings.ToLower(strings.TrimSpace(id))] = enable
	}
	var keep []string
	for id, on := range set {
		if on {
			keep = append(keep, id)
		}
	}
	return tooladapter.NormalizeSelection(keep)
}

// equalIDSet reports whether a and b contain the same IDs (order-insensitive).
func equalIDSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := append([]string{}, a...)
	bs := append([]string{}, b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

// registeredConfigPath returns the config path the registry recorded for the
// named group (the source of truth), falling back to ConfigPathFor's computed
// path only when the group is not yet registered.
func registeredConfigPath(name string) (string, error) {
	groups, err := registry.Groups()
	if err != nil {
		return "", fmt.Errorf("read registry: %w", err)
	}
	for _, g := range groups {
		if g.Name == name {
			return g.ConfigPath, nil
		}
	}
	return registry.ConfigPathFor(name)
}

// loadGroupConfigByName loads a group's GroupConfig by group name, returning a
// non-nil (possibly empty) config so callers can resolve enablement.
func loadGroupConfigByName(name string) (*registry.GroupConfig, error) {
	cfgPath, err := registeredConfigPath(name)
	if err != nil {
		return nil, err
	}
	cfg, err := registry.LoadGroupConfig(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load group %q config: %w", name, err)
	}
	if cfg == nil {
		cfg = &registry.GroupConfig{Name: name}
	}
	return cfg, nil
}
