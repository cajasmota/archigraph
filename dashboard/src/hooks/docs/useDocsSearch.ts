import { useQuery } from '@tanstack/react-query'
import { fetchDocsSearch } from '@/api/client'
import type { DocSearchResponse } from '@/types/docs'
import { useState, useEffect } from 'react'

export function useDocsSearch(group: string, query: string) {
  const [debouncedQuery, setDebouncedQuery] = useState(query)

  useEffect(() => {
    const id = setTimeout(() => setDebouncedQuery(query), 250)
    return () => clearTimeout(id)
  }, [query])

  return useQuery<DocSearchResponse, Error>({
    queryKey: ['docs-search', group, debouncedQuery],
    queryFn: () => fetchDocsSearch(group, debouncedQuery),
    staleTime: 30 * 1000,
    enabled: Boolean(group) && debouncedQuery.trim().length >= 2,
  })
}
