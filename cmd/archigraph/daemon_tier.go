package main

// daemon_tier.go wires the tiered hibernation state machine (PH2 of epic
// #2087 / issue #2090, extended by PH3 #2091, PH6 #2094) into the daemon process.
//
// Process-global daemonTierMgr tracks HOT/WARM/COLD/EXPIRED state for every
// indexed (repoPath, ref) pair.  Integrations:
//
//   - tierAfterIndex: called after every successful index pass; registers the
//     slot as HOT (or re-activates it) and detects the default branch.
//
//   - MCP graph-cache AccessHook: wired in startDaemonTierManager; every
//     GetForRepoRef call updates lastAccessedAt via tierTouchRepoRef so
//     actively-queried graphs don't get prematurely evicted.
//
//   - Eviction (WARM→COLD): daemonMCPCache.Invalidate releases the mmap'd
//     fbreader.Reader; the dashboard cache ages out via its own TTL.
//
//   - Cold wake (COLD→HOT): the reload callback re-mmap's graph.fb by
//     calling daemonMCPCache.Get; the dashboard cache reloads lazily on the
//     next HTTP request.
//
//   - Disk eviction (COLD→EXPIRED, PH6): tierDiskEvictCallback deletes the
//     refs/<ref>/ sub-directory for the expired slot and logs freed bytes.

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/cajasmota/archigraph/internal/daemon"
	"github.com/cajasmota/archigraph/internal/daemon/tier"
)

// daemonTierMgr is the process-wide tiered hibernation state machine.
// Nil before startDaemonTierManager is called.
var daemonTierMgr *tier.Manager

// startDaemonTierManager constructs and starts the tier manager. Must be
// called once from runDaemon before the daemon begins serving requests.
func startDaemonTierManager(ctx context.Context, logger *log.Logger) {
	ttl := tier.EnvTTLConfig()
	daemonTierMgr = tier.NewManager(ctx, ttl, tierEvictCallback, tierReloadCallback, tierDiskEvictCallback, logger)

	// Wire the MCP graph-cache access hook so every GetForRepoRef call
	// updates lastAccessedAt in the tier manager without extra call-sites.
	daemonMCPCache.SetAccessHook(func(repoPath, ref string) {
		_ = tierTouchRepoRef(repoPath, ref)
	})
}

// tierAfterIndex is called after every successful index pass to register
// (or re-activate) the slot as HOT. Detects default branch for isPinnedMain.
// PH3 (#2091): slots are now annotated with SlotKind so the tier manager can
// apply the correct TTL policy.  Worktree slots are registered separately via
// tierAfterIndexWorktree.
func tierAfterIndex(repoPath, ref string) {
	if daemonTierMgr == nil {
		return
	}
	isPinned := tier.IsDefaultBranch(repoPath, ref)
	kind := tier.SlotKindBranchFeature
	if isPinned {
		kind = tier.SlotKindBranchMain
	}
	daemonTierMgr.Register(tier.SlotKey{RepoPath: repoPath, Ref: ref}, isPinned, kind)
}

// tierAfterIndexWorktree is like tierAfterIndex but uses SlotKindWorktree
// so the tier manager applies the aggressive 30-min WARM→COLD window.
// Called after indexing a linked worktree (discovered by PH3).
func tierAfterIndexWorktree(repoPath, ref string) {
	if daemonTierMgr == nil {
		return
	}
	daemonTierMgr.Register(tier.SlotKey{RepoPath: repoPath, Ref: ref}, false, tier.SlotKindWorktree)
}

// tierTouchRepoRef records an access for (repoPath, ref). If the slot is
// COLD, this triggers an in-place reload (via tierReloadCallback) and
// transitions the slot back to HOT.
func tierTouchRepoRef(repoPath, ref string) error {
	if daemonTierMgr == nil {
		return nil
	}
	return daemonTierMgr.Touch(tier.SlotKey{RepoPath: repoPath, Ref: ref})
}

// tierEvictCallback releases the in-memory graph for a WARM→COLD transition.
func tierEvictCallback(key tier.SlotKey) {
	// Invalidate the mmap'd fbreader in the MCP graph cache.
	stateDir := daemon.StateDirForRepoRef(key.RepoPath, key.Ref)
	fbPath := filepath.Join(stateDir, "graph.fb")
	daemonMCPCache.Invalidate(fbPath)
	// The dashboard GraphCache entry ages out via its own TTL on next access.
}

// tierReloadCallback reloads the mmap'd fbreader into the MCP graph cache
// when a COLD slot receives a query (cold wake).
func tierReloadCallback(key tier.SlotKey) error {
	stateDir := daemon.StateDirForRepoRef(key.RepoPath, key.Ref)
	fbPath := filepath.Join(stateDir, "graph.fb")
	// Prime the cache by opening and immediately releasing the reader.
	_, release, err := daemonMCPCache.Get(fbPath)
	if err != nil {
		return err
	}
	release()
	return nil
}

// tierDiskEvictCallback is the PH6 COLD→EXPIRED disk deletion hook.
// It deletes the refs/<ref-safe>/ directory for the expired slot and returns
// the bytes freed. Pinned-main slots never reach EXPIRED, so no guard needed
// here — the tier Manager already suppresses transitions for isPinnedMain slots.
func tierDiskEvictCallback(key tier.SlotKey) (int64, error) {
	stateDir := daemon.StateDirForRepoRef(key.RepoPath, key.Ref)
	freed, err := dirSize(stateDir)
	if err != nil {
		// Directory may not exist — not an error worth surfacing.
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if err := os.RemoveAll(stateDir); err != nil {
		return 0, err
	}
	return freed, nil
}

// dirSize returns the total byte size of all files under dir.
func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total, err
}
