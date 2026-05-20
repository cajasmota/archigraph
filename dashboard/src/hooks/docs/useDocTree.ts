import { useQuery } from '@tanstack/react-query'
import { fetchDocTree } from '@/api/client'
import type { DocTreeResponse } from '@/types/docs'

export function useDocTree(group: string) {
  return useQuery<DocTreeResponse, Error>({
    queryKey: ['doc-tree', group],
    queryFn: () => fetchDocTree(group),
    staleTime: 2 * 60 * 1000,
    enabled: Boolean(group),
  })
}
