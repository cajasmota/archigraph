/* ============================================================
   hooks/use-di.ts — Dependency-Injection data hook (#4266).

   Wraps api.getDI in TanStack Query. Backed by the route
   GET /api/di/{group} (handlers_di.go → handleDI), which walks
   the graph for INJECTED_INTO edges (provider INJECTED_INTO
   consumer) and returns a raw-JSON DIReport: providers grouped
   by DI framework (nestjs / spring / angular / …), each listing
   the consumers it injects into, with file refs.

   Pure static graph read. Honest-empty when the group has no
   INJECTED_INTO edges (no DI frameworks indexed).
   ============================================================ */

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export const diQueryKey = (groupId: string) => ["di", groupId] as const;

/** Dependency-Injection report for the group. */
export function useDI(groupId: string) {
  return useQuery({
    queryKey: diQueryKey(groupId),
    queryFn: () => api.getDI(groupId),
    enabled: !!groupId,
    staleTime: 30_000,
  });
}
