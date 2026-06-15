// sweeper.go — background docgen cleanup goroutine for the grafel daemon
// (issue #2216, epic #2207).
//
// StartDocgenSweeper launches a goroutine that calls an injected CleanupFn at
// startup and every Interval. The function injection avoids the import cycle
// that would arise from importing internal/docgen here (internal/docgen itself
// imports internal/daemon for StateDirForRepo).
//
// The Config.DocgenSweep hook is populated by cmd/grafel (which imports
// both packages) and passed down into daemon.Run.  Disabled via the
// --no-auto-cleanup flag on `grafel start`.
package daemon

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

// DocgenSweeperConfig controls the background docgen cleanup goroutine.
type DocgenSweeperConfig struct {
	// CleanupFn is the function that performs the actual cleanup.
	// It is injected from cmd/grafel to avoid the import cycle
	// internal/daemon → internal/docgen → internal/daemon.
	//
	// The function returns (removedCount, freedBytes, error).
	// Non-nil errors are logged but do not stop the sweeper.
	CleanupFn func() (int, int64, error)

	// Interval between sweeps. Default (zero value): 24 hours.
	Interval time.Duration

	// Logger is used for sweep diagnostics. When nil, a default stderr slog.Logger is used.
	Logger *slog.Logger
}

// StartDocgenSweeper launches the background docgen cleanup goroutine and
// returns immediately. The goroutine runs until stopCh is closed.
//
// Call once at daemon startup after the daemon is ready to serve.
// If cfg.CleanupFn is nil, this function is a no-op.
func StartDocgenSweeper(cfg DocgenSweeperConfig, stopCh <-chan struct{}) {
	if cfg.CleanupFn == nil {
		return
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil)).With("pkg", "sweeper")
	}

	go runDocgenSweeper(interval, cfg.CleanupFn, logger, stopCh)
}

// runDocgenSweeper is the goroutine body. It performs an initial sweep on
// entry (after a short startup delay) and then sweeps on a ticker.
func runDocgenSweeper(interval time.Duration, cleanupFn func() (int, int64, error), logger *slog.Logger, stopCh <-chan struct{}) {
	// Short startup delay so the daemon socket is live before we do I/O.
	startupDelay := 30 * time.Second
	select {
	case <-stopCh:
		return
	case <-time.After(startupDelay):
	}

	sweep := func() {
		n, freed, err := cleanupFn()
		if err != nil {
			logger.Error("docgen sweeper: cleanup error", "err", err)
			return
		}
		logger.Info("docgen sweeper: removed items", "count", n, "freed", sweeperHumanBytes(freed))
	}

	// Initial sweep.
	sweep()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			sweep()
		}
	}
}

// sweeperHumanBytes formats a byte count as a human-readable string.
func sweeperHumanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
