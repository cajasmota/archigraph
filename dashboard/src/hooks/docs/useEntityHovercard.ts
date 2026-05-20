import { useQuery } from '@tanstack/react-query'
import { fetchEntityHovercard } from '@/api/client'
import type { EntityCard } from '@/types/docs'

/**
 * Lazy-fetches entity metadata for hovercards.
 * Only fires when the hovercard is triggered (enabled=true).
 */
export function useEntityHovercard(entityId: string | null, enabled: boolean) {
  return useQuery<EntityCard, Error>({
    queryKey: ['entity-hovercard', entityId],
    queryFn: () => fetchEntityHovercard(entityId!),
    enabled: enabled && Boolean(entityId),
    staleTime: 5 * 60 * 1000,
  })
}
