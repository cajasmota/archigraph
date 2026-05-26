package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// newBenchCaptureCmd returns the cobra command for `archigraph bench-capture`.
//
// The actual implementation lives in cmd/archigraph/bench_capture.go (package
// main) to keep internal/cli free of the heavy stdlib math + IO imports.
// The command is wired via activeHooks.RunBenchCapture set by cli.Execute().
//
// Surface: `archigraph bench-capture rpc [--log <path>] [--start-offset N] [--end-offset N]`
//
// Closes #2298.
func newBenchCaptureCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "bench-capture",
		Short:              "Capture daemon-log RPC metrics into JSON (bench skill helper)",
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if activeHooks.RunBenchCapture == nil {
				return errors.New("bench-capture handler not wired")
			}
			return activeHooks.RunBenchCapture(args)
		},
	}
}
