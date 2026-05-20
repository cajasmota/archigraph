import { useQuery } from '@tanstack/react-query'
import { fetchPaths } from '@/api/client'
import type { PathListResponse, PathFilters } from '@/types/api'

export function usePathList(group: string, filters: PathFilters = {}) {
  return useQuery<PathListResponse, Error>({
    queryKey: ['paths', group, filters],
    queryFn: () => fetchPaths(group, filters),
    placeholderData: (prev) => prev, // keeps stale data while loading next page
    staleTime: 60 * 1000,
  })
}
