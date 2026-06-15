package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// newQualityCmd is the cobra shim for `grafel quality <fixture-dir>`.
// Implementation lives in cmd/grafel because it pulls in the indexer.
func newQualityCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "quality <fixture-dir>",
		Short:              "Measure extraction recall against a golden fixture",
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if activeHooks.RunQuality == nil {
				return errors.New("quality handler not wired")
			}
			return activeHooks.RunQuality(args)
		},
	}
}
