/* ============================================================
   hooks/use-coverage-kind.ts — shared coverage-kind state for the diagram
   node overlays (#5147).

   #5067 landed the per-ROW coverage-kind indicator on the Quality tab. #5147
   extends that same disambiguation to the diagram NODE surfaces (flow-dag /
   topology / iac). Every one of those surfaces needs to know WHICH of the three
   grafel coverages applies to the current group so it can tint its nodes
   accordingly — but they must NOT issue a second coverage fetch.

   This hook reads the EXISTING `useQualityCoverage(groupId)` query (#5066). That
   query is keyed `["quality", groupId, "coverage"]`, so any surface calling it
   shares TanStack Query's cache with the Quality tab: if the tab already loaded
   coverage, this resolves synchronously from cache; otherwise it triggers the
   same single in-flight request (deduped by key) — never a duplicate fetch.

   The result is projected through the SAME `coverageStateFromReport` +
   `resolveCoverageKindNodeDecoration` helpers the rows use, so the node tint and
   the row chip can never disagree about the kind. When coverage hasn't loaded
   (or errored) the state is `null`, which the resolver degrades to the neutral
   capability treatment — never a fake authoritative green.
   ============================================================ */

import { useMemo } from "react";
import { useQualityCoverage } from "@/hooks/use-quality";
import {
  coverageStateFromReport,
  resolveCoverageKindNodeDecoration,
  type CoverageSourceState,
  type CoverageKindNodeDecoration,
} from "@/lib/coverage-provenance";

export interface CoverageKindState {
  /**
   * Render-ready node decoration (ring color + kind + tooltip) for the current
   * group's best-available coverage. Always defined: a missing report degrades
   * to the neutral capability treatment via the resolver's default.
   */
  decoration: CoverageKindNodeDecoration;
  /** The raw resolved source state (fed to the legend's CoverageKindIndicator). */
  state: CoverageSourceState;
  /** True once the underlying coverage query has settled (loaded or errored). */
  ready: boolean;
}

/**
 * Resolve the group's coverage-kind decoration from the shared coverage query.
 * Reuses #5066's data layer (no extra fetch) and #5067's resolver semantics.
 */
export function useCoverageKind(groupId: string): CoverageKindState {
  const { data, isLoading } = useQualityCoverage(groupId);

  return useMemo(() => {
    const state = coverageStateFromReport(data?.line_coverage);
    return {
      state,
      decoration: resolveCoverageKindNodeDecoration(state),
      ready: !isLoading,
    };
  }, [data?.line_coverage, isLoading]);
}
