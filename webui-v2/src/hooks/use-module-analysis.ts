/* ============================================================
   hooks/use-module-analysis.ts — module-level GDS for the Graph
   screen's "Module overview" mode (#1386, closes #1380 alongside
   #1384 which shipped the algorithms + endpoint).

   Thin TanStack Query wrapper around api.getModuleAnalysis. The
   payload is small (12 modules on acme, ~30 on polyglot per repo)
   so we keep it warm for the session and let the UI re-derive
   normalized shapes from the wire response.
   ============================================================ */

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { ModuleAnalysisResponse } from "@/data/types";

export const moduleAnalysisQueryKey = (
  groupId: string,
  topN?: number,
  repoFilter?: string[],
) =>
  [
    "module-analysis",
    groupId,
    topN ?? 10,
    repoFilter?.slice().sort().join(",") ?? "",
  ] as const;

export function useModuleAnalysis(
  groupId: string,
  opts?: { topN?: number; repoFilter?: string[]; enabled?: boolean },
) {
  return useQuery<ModuleAnalysisResponse>({
    queryKey: moduleAnalysisQueryKey(groupId, opts?.topN, opts?.repoFilter),
    queryFn: () =>
      api.getModuleAnalysis(groupId, {
        topN: opts?.topN ?? 10,
        repoFilter: opts?.repoFilter,
      }),
    // Module-level GDS is deterministic given the indexed graph; we keep it
    // warm across mode-toggles within a session.
    staleTime: 5 * 60 * 1000,
    enabled: opts?.enabled ?? true,
  });
}
