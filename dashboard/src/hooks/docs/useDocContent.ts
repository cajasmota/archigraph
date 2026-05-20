import { useQuery } from '@tanstack/react-query'
import { fetchDocContent } from '@/api/client'
import type { DocContentResponse } from '@/types/docs'

export function useDocContent(group: string, docPath: string | undefined) {
  return useQuery<DocContentResponse, Error>({
    queryKey: ['doc-content', group, docPath],
    queryFn: () => fetchDocContent(group, docPath!),
    staleTime: 2 * 60 * 1000,
    enabled: Boolean(group) && Boolean(docPath),
  })
}
