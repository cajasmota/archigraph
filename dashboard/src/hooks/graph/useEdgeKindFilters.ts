import { useSearchParams } from 'react-router-dom'
import { useMemo } from 'react'
import type { RelationshipKind } from '@/types/api'

const PARAM = 'edge_kinds'

/**
 * Reads and writes edge-kind filter state from URL search params.
 * Keeps the URL as single source of truth for shareability.
 *
 * URL format: ?edge_kinds=CALLS,IMPORTS,FETCHES
 */
export function useEdgeKindFilters(): {
  activeKinds: Set<RelationshipKind>
  toggle: (kind: RelationshipKind) => void
  setAll: (kinds: RelationshipKind[]) => void
  clearAll: () => void
  isActive: (kind: RelationshipKind) => boolean
} {
  const [params, setParams] = useSearchParams()

  const activeKinds = useMemo<Set<RelationshipKind>>(() => {
    const raw = params.get(PARAM)
    if (!raw) return new Set()
    return new Set(raw.split(',') as RelationshipKind[])
  }, [params])

  function toggle(kind: RelationshipKind) {
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      const current = new Set(
        (prev.get(PARAM) ?? '').split(',').filter(Boolean) as RelationshipKind[],
      )
      if (current.has(kind)) {
        current.delete(kind)
      } else {
        current.add(kind)
      }
      if (current.size === 0) {
        next.delete(PARAM)
      } else {
        next.set(PARAM, [...current].join(','))
      }
      return next
    })
  }

  function setAll(kinds: RelationshipKind[]) {
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      if (kinds.length === 0) {
        next.delete(PARAM)
      } else {
        next.set(PARAM, kinds.join(','))
      }
      return next
    })
  }

  function clearAll() {
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.delete(PARAM)
      return next
    })
  }

  return {
    activeKinds,
    toggle,
    setAll,
    clearAll,
    isActive: (kind) => activeKinds.has(kind),
  }
}
