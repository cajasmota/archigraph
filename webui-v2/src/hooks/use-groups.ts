/* ============================================================
   hooks/use-groups.ts — Landing-screen data hooks.

   Wraps the typed api client (the only network door) in TanStack
   Query. Screens never call api.* directly outside hooks.
   ============================================================ */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Group } from "@/data/types";

export const groupsQueryKey = ["groups"] as const;

/** The rich group list that drives the Landing cards grid. */
export function useGroups() {
  return useQuery({
    queryKey: groupsQueryKey,
    queryFn: () => api.listGroups(),
  });
}

/** Create-group mutation; invalidates the list so the new card appears. */
export function useCreateGroup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => api.createGroup(name),
    onSuccess: (created: Group) => {
      qc.setQueryData<Group[]>(groupsQueryKey, (prev) =>
        prev ? [...prev, created] : [created],
      );
      void qc.invalidateQueries({ queryKey: groupsQueryKey });
    },
  });
}
