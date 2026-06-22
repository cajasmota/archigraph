/* ============================================================
   hooks/use-index-progress.ts — real-time per-repo indexing progress feed
   (#1527; one-row-per-repo dedup #5326).

   Subscribes to the daemon SSE stream at /api/index-progress/:group and
   collapses the firehose of progress.Event records into a stable list of
   ROWS — one per repo (each monorepo package is already registered as its own
   repo, so it gets its own row). The wizard's Index step and the Operations
   screen render these rows so the user sees concrete, granular progress instead
   of a single coarse bar.

   Why a hook (not a query): EventSource is push-based and long-lived. We keep
   the accumulated rows in component state and tear the connection down when
   the group changes or the component unmounts.

   Resilience: if the EventSource errors the browser auto-reconnects; we keep
   the last-known rows so the UI doesn't flash empty. The job poller
   (useWizardJob) is the primary source of truth for terminal state; the feed's
   own `terminal` flag is a FALLBACK so the wizard never freezes (#5326 bug 1).
   ============================================================ */

import { useEffect, useState } from "react";

import { api } from "@/lib/api";
import type { ProgressEvent, ProgressRow } from "@/data/types";
import { fold, rowsTerminal, sortRows } from "@/lib/index-progress-fold";

export interface UseIndexProgress {
  /** One row per repo (latest status), sorted for stable rendering. */
  rows: ProgressRow[];
  /** True while the SSE connection is open. */
  connected: boolean;
  /** True once at least one progress event has arrived. */
  hasData: boolean;
  /**
   * True once ALL expected repo rows have reached a terminal (done/error)
   * phase. Used by the wizard as a FALLBACK terminal signal so the button
   * reaches "Done" even if the job poller is slow to flip (#5326 bug 1). Gated
   * on the expected repo count so it can't fire after only the first of several
   * repos finishes (#5326 multi-repo regression).
   */
  terminal: boolean;
}

/**
 * Subscribe to the per-repo progress stream for `group`. Pass a falsy group or
 * `enabled === false` to stay disconnected (e.g. before the job has a group, or
 * after the wizard closes).
 *
 * `expectedRepos` is how many repos this index registered (selected child git
 * repos, or selected monorepo packages, or 1). It gates the `terminal` flag so
 * feed-terminal cannot fire after the FIRST repo finishes while later repos are
 * still streaming (#5326). The EventSource stays subscribed the whole time
 * `enabled` is true (the wizard gates that on the JOB being active, not on
 * `terminal`), so every repo's events are received even if one finishes early.
 */
export function useIndexProgress(
  group: string | undefined,
  enabled = true,
  expectedRepos?: number,
): UseIndexProgress {
  const [rows, setRows] = useState<Map<string, ProgressRow>>(new Map());
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!enabled || !group) {
      setConnected(false);
      return;
    }
    // Fresh group → fresh rows.
    setRows(new Map());

    const es = new EventSource(api.progressStreamUrl(group));

    es.addEventListener("connected", () => setConnected(true));
    es.addEventListener("progress", (ev) => {
      try {
        const e = JSON.parse((ev as MessageEvent).data) as ProgressEvent;
        // The wildcard daemon stream can carry other groups; the per-group
        // endpoint already filters, but guard anyway.
        if (e.group_slug && group && e.group_slug !== group) return;
        setRows((prev) => fold(prev, e));
      } catch {
        /* malformed event — skip */
      }
    });
    es.addEventListener("close", () => {
      setConnected(false);
      es.close();
    });
    es.onerror = () => setConnected(false);

    return () => {
      es.close();
      setConnected(false);
    };
  }, [group, enabled]);

  const sorted = sortRows(rows.values());

  return {
    rows: sorted,
    connected,
    hasData: rows.size > 0,
    terminal: rowsTerminal(sorted, expectedRepos),
  };
}
