/* ============================================================
   hooks/use-meta.ts — daemon bootstrap (/api/v2/meta).

   Fetched once on mount; supplies the daemon version for the Landing
   info popover. staleTime: Infinity per the API_V2 contract.
   ============================================================ */

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export function useMeta() {
  return useQuery({
    queryKey: ["meta"],
    staleTime: Infinity,
    queryFn: () => api.getMeta(),
  });
}
