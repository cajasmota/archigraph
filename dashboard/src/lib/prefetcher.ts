/**
 * prefetcher.ts — background prefetch for surface data (#1257)
 *
 * Two triggers:
 *   1. Hover  — prefetch the surface the user is about to click
 *   2. Idle   — after IDLE_DELAY ms without a prefetch, warm the most-likely
 *               next surface based on a localStorage visit-frequency histogram
 *
 * Uses react-query's prefetchQuery so results land in the shared QueryClient
 * cache; the surface routes pick them up via useQuery with the same key and
 * render instantly.
 *
 * Cache invalidation:
 *   - All graph/flows/topology cache entries are invalidated by calling
 *     invalidateSurfaceCaches(queryClient, group) after rebuild mutations.
 *     This is wired in via the existing postGroupRebuild / postRepoRebuild
 *     success handlers in the System route.
 */

import type { QueryClient } from '@tanstack/react-query'
import { fetchGraph, fetchFlows, fetchTopology } from '@/api/client'

/* ── Constants ──────────────────────────────────────────────────────────────── */

/** Milliseconds of hover before a prefetch fires (debounce). */
export const HOVER_DELAY_MS = 120

/** Milliseconds of idle before predictive prefetch fires. */
export const IDLE_DELAY_MS = 2_000

/** localStorage key for the visit histogram. */
const HISTORY_KEY = 'archigraph:nav-history'

/** Surface names that have group-scoped data we can prefetch. */
export type PrefetchSurface = 'graph' | 'flows' | 'topology'

const ALL_SURFACES: PrefetchSurface[] = ['graph', 'flows', 'topology']

/* ── Visit history ──────────────────────────────────────────────────────────── */

type VisitHistogram = Record<PrefetchSurface, number>

function readHistogram(): VisitHistogram {
  try {
    const raw = localStorage.getItem(HISTORY_KEY)
    if (!raw) return { graph: 0, flows: 0, topology: 0 }
    return JSON.parse(raw) as VisitHistogram
  } catch {
    return { graph: 0, flows: 0, topology: 0 }
  }
}

/** Record a visit to the given surface. Called by the layout on route change. */
export function recordVisit(surface: PrefetchSurface): void {
  try {
    const hist = readHistogram()
    hist[surface] = (hist[surface] ?? 0) + 1
    localStorage.setItem(HISTORY_KEY, JSON.stringify(hist))
  } catch {
    // localStorage may be unavailable in some environments — no-op
  }
}

/**
 * Returns surfaces ordered by visit frequency, most-visited first,
 * excluding the currently-active surface.
 */
export function predictedOrder(activeSurface: PrefetchSurface | null): PrefetchSurface[] {
  const hist = readHistogram()
  return ALL_SURFACES
    .filter((s) => s !== activeSurface)
    .sort((a, b) => (hist[b] ?? 0) - (hist[a] ?? 0))
}

/* ── Prefetch helpers ────────────────────────────────────────────────────────── */

/**
 * Prefetch a single surface into the QueryClient cache.
 * No-ops if the data is already fresh (react-query skips the network call).
 *
 * Each branch is typed independently so TypeScript can match the queryKey
 * to the correct return type without union inference issues.
 */
export async function prefetchSurface(
  queryClient: QueryClient,
  surface: PrefetchSurface,
  group: string,
): Promise<void> {
  switch (surface) {
    case 'graph':
      await queryClient.prefetchQuery({
        queryKey: ['graph', group, undefined, undefined] as const,
        queryFn: () => fetchGraph(group, {}),
        staleTime: 5 * 60 * 1000,
      })
      break
    case 'flows':
      await queryClient.prefetchQuery({
        queryKey: ['flows', group, {}] as const,
        queryFn: () => fetchFlows(group, {}),
        staleTime: 60 * 1000,
      })
      break
    case 'topology':
      await queryClient.prefetchQuery({
        queryKey: ['topology', group, {}] as const,
        queryFn: () => fetchTopology(group, {}),
        staleTime: 60 * 1000,
      })
      break
  }
}

/**
 * Prefetch all surfaces not currently active, ordered by predicted likelihood.
 * Used by the idle trigger.
 */
export async function prefetchIdle(
  queryClient: QueryClient,
  group: string,
  activeSurface: PrefetchSurface | null,
): Promise<void> {
  const order = predictedOrder(activeSurface)
  // Fire all prefetches; react-query deduplicates if the keys are already cached
  await Promise.allSettled(
    order.map((s) => prefetchSurface(queryClient, s, group)),
  )
}

/* ── Cache invalidation ──────────────────────────────────────────────────────── */

/**
 * Invalidate all surface caches for a group after a rebuild mutation.
 * Call this in the onSuccess handler of postGroupRebuild / postRepoRebuild.
 */
export function invalidateSurfaceCaches(queryClient: QueryClient, group: string): void {
  queryClient.invalidateQueries({ queryKey: ['graph', group] })
  queryClient.invalidateQueries({ queryKey: ['flows', group] })
  queryClient.invalidateQueries({ queryKey: ['topology', group] })
}

/* ── Hover debounce factory ──────────────────────────────────────────────────── */

/**
 * Returns { onMouseEnter, onMouseLeave } handlers that fire a prefetch after
 * HOVER_DELAY_MS on the given surface.  Cancels the timer on leave so fast
 * mouse-throughs do not spam the server.
 */
export function makeHoverPrefetchHandlers(
  queryClient: QueryClient,
  surface: PrefetchSurface,
  group: string,
) {
  let timer: ReturnType<typeof setTimeout> | null = null

  return {
    onMouseEnter() {
      timer = setTimeout(() => {
        prefetchSurface(queryClient, surface, group).catch(() => {
          // prefetch errors are intentionally swallowed — surface will fetch on demand
        })
      }, HOVER_DELAY_MS)
    },
    onMouseLeave() {
      if (timer !== null) {
        clearTimeout(timer)
        timer = null
      }
    },
  }
}
