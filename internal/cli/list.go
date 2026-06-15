package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cajasmota/grafel/internal/registry"
)

func newListCmd() *cobra.Command {
	var refFlag string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List registered groups",
		Long: `List registered groups.

When --ref is supplied the output notes which ref is being targeted.
Use --ref @all to show a note that all known refs are in scope (the
group list itself is ref-independent — refs live inside groups).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolvedRef, isAll, err := resolveRef(refFlag, true /* @all ok — list is read-only */)
			if err != nil {
				return err
			}
			groups, err := registry.Groups()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if resolvedRef != "" {
				fmt.Fprintf(out, "Note: showing groups for ref %q.\n\n", resolvedRef)
			} else if isAll {
				fmt.Fprintf(out, "Note: --ref @all — showing groups across all known refs.\n\n")
			}
			if len(groups) == 0 {
				fmt.Fprintln(out, "No groups registered. Run `grafel wizard` to create one.")
				return nil
			}
			fmt.Fprintf(out, "%-24s  %s\n", "GROUP", "CONFIG")
			for _, g := range groups {
				fmt.Fprintf(out, "%-24s  %s\n", g.Name, g.ConfigPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&refFlag, "ref", "", refFlagUsage)
	return cmd
}
