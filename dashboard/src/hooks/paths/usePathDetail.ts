import { useQuery } from '@tanstack/react-query'
import { fetchPathDetail } from '@/api/client'
import type { PathDetailResponse } from '@/types/api'

export function usePathDetail(group: string, pathHash: string | null) {
  return useQuery<PathDetailResponse, Error>({
    queryKey: ['path-detail', group, pathHash],
    queryFn: () => fetchPathDetail(group, pathHash!),
    enabled: !!pathHash,
    staleTime: 5 * 60 * 1000,
  })
}
