import { useQuery } from '@tanstack/react-query'
import { fetchEntityNeighbors } from '@/api/client'
import type { EntityNeighborResponse } from '@/types/api'

/**
 * Fetches an entity and its 1-hop neighbors for the EntityInspector panel.
 * Enabled only when entityId is non-null.
 */
export function useEntityInspector(
  group: string,
  entityId: string | null,
): {
  data: EntityNeighborResponse | undefined
  isLoading: boolean
  error: Error | null
} {
  const { data, isLoading, error } = useQuery<EntityNeighborResponse, Error>({
    queryKey: ['entity-neighbors', group, entityId],
    queryFn: () => fetchEntityNeighbors(group, entityId!),
    enabled: !!group && !!entityId,
    staleTime: 2 * 60 * 1000,
  })

  return { data, isLoading, error }
}
