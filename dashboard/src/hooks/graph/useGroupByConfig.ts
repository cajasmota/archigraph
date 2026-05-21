/**
 * useGroupByConfig — persisted "Group by" clustering dimension for the graph.
 *
 * #1392: The owner wants the multi-repo graph to settle into distinct ISLANDS
 * with the cross-repo edges visibly bridging them. cosmos.gl has a per-point
 * cluster force; this hook chooses WHICH dimension feeds each node's cluster id:
 *
 *   'repo'      — one island per repository (DEFAULT — best multi-repo view)
 *   'community' — one island per community_id cluster
 *   'module'    — one island per source-file module (derived from source_file)
 *   'none'      — no clustering; free force layout (legacy behavior)
 *
 * When a grouping dimension is active, GraphCanvas raises the cluster strength
 * substantially so each group contracts into a tight, distinct island. 'none'
 * disables the cluster force entirely.
 *
 * Persisted to localStorage so the user's choice survives reload.
 */
import { useState, useCallback } from 'react'

export type GroupByMode = 'repo' | 'community' | 'module' | 'none'

const STORAGE_KEY = 'archigraph.graph.groupBy'
const VALID: GroupByMode[] = ['repo', 'community', 'module', 'none']

function readStored(): GroupByMode {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw && VALID.includes(raw as GroupByMode)) return raw as GroupByMode
  } catch {
    // Private browsing — fall through to default
  }
  return 'repo'
}

export interface UseGroupByConfigReturn {
  groupBy: GroupByMode
  setGroupBy: (mode: GroupByMode) => void
}

export function useGroupByConfig(): UseGroupByConfigReturn {
  const [groupBy, setGroupByState] = useState<GroupByMode>(readStored)

  const setGroupBy = useCallback((mode: GroupByMode) => {
    setGroupByState(mode)
    try {
      localStorage.setItem(STORAGE_KEY, mode)
    } catch {
      // Ignore quota / private-mode errors
    }
  }, [])

  return { groupBy, setGroupBy }
}
