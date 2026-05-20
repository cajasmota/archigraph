import { useMemo, useState, useEffect } from 'react'
import type { GraphNode } from '@/types/api'

const DEBOUNCE_MS = 150
const MAX_RESULTS = 20

/**
 * Debounced typeahead search over graph node names.
 * Pure in-memory — no fetch; the full node list is already in cache.
 *
 * Returns up to MAX_RESULTS matching nodes sorted by:
 *   1. Exact prefix match on label
 *   2. Substring match anywhere
 *   3. Pagerank (descending)
 */
export function useGraphSearch(
  query: string,
  nodes: GraphNode[],
): {
  results: GraphNode[]
  isSearching: boolean
} {
  const [debouncedQuery, setDebouncedQuery] = useState(query)

  useEffect(() => {
    if (!query.trim()) {
      setDebouncedQuery('')
      return
    }
    const t = setTimeout(() => setDebouncedQuery(query.trim().toLowerCase()), DEBOUNCE_MS)
    return () => clearTimeout(t)
  }, [query])

  const results = useMemo(() => {
    if (!debouncedQuery) return []

    const q = debouncedQuery
    const matches = nodes.filter((n) =>
      n.label.toLowerCase().includes(q) ||
      n.id.toLowerCase().includes(q),
    )

    // Sort: prefix match first, then pagerank descending
    matches.sort((a, b) => {
      const aPrefix = a.label.toLowerCase().startsWith(q) ? 0 : 1
      const bPrefix = b.label.toLowerCase().startsWith(q) ? 0 : 1
      if (aPrefix !== bPrefix) return aPrefix - bPrefix
      return (b.pagerank ?? 0) - (a.pagerank ?? 0)
    })

    return matches.slice(0, MAX_RESULTS)
  }, [debouncedQuery, nodes])

  return {
    results,
    isSearching: query.trim().toLowerCase() !== debouncedQuery,
  }
}
