import { useQuery } from '@tanstack/react-query'
import { fetchRegistry } from '@/api/client'
import type { Registry } from '@/types/api'

export function useRegistry() {
  return useQuery<Registry, Error>({
    queryKey: ['registry'],
    queryFn: fetchRegistry,
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}
