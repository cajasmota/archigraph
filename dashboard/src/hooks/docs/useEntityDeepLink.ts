import { useCallback } from 'react'
import { useParams } from 'react-router-dom'

/**
 * Returns a function that builds a Surface 1 deep-link URL for a given entity ID.
 * URL format: /graph/{group}?entity={entityId}
 */
export function useEntityDeepLink() {
  const { group = 'fixture-a' } = useParams<{ group: string }>()

  const buildLink = useCallback(
    (entityId: string) => `/graph/${group}?entity=${encodeURIComponent(entityId)}`,
    [group],
  )

  return buildLink
}
