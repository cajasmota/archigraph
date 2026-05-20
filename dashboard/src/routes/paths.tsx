import { useEffect, useRef } from 'react'
import { useParams, Outlet, useNavigate } from 'react-router-dom'
import { PathTreeSidebar } from '@/components/paths/PathTreeSidebar'
import { PathRow } from '@/components/paths/PathRow'
import { PathFilterPanel } from '@/components/paths/PathFilterPanel'
import { PathSearchInput } from '@/components/paths/PathSearchInput'
import { Pagination } from '@/components/paths/Pagination'
import { EmptyState } from '@/components/shared/EmptyState'
import { PathListSkeleton } from '@/components/shared/LoadingState'
import { ErrorBoundary } from '@/components/shared/ErrorBoundary'
import { usePathList } from '@/hooks/paths/usePathList'
import { usePathTree } from '@/hooks/paths/usePathTree'
import { usePathFilters } from '@/hooks/paths/usePathFilters'
import { Globe } from 'lucide-react'

/**
 * Surface 4 — API & Contracts Explorer.
 *
 * Layout:
 *   [PathTreeSidebar] | [PathList + detail panel via Outlet]
 *
 * Keyboard shortcuts:
 *   /  → focus search input
 *   ↑↓ → move selection in path list
 *   Enter → drill into selected path
 */
export function PathsRoute() {
  const { group = 'fixture-a' } = useParams<{ group: string }>()
  const { filters, setFilter, clearFilters } = usePathFilters()
  const { data, isLoading, isFetching } = usePathList(group, filters)
  const { tree, isLoading: treeLoading } = usePathTree(group)
  const navigate = useNavigate()
  const searchRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  // Keyboard shortcut: / to focus search
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === '/' && document.activeElement?.tagName !== 'INPUT') {
        e.preventDefault()
        const input = searchRef.current?.querySelector('input')
        input?.focus()
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [])

  // Arrow key navigation in the path list
  useEffect(() => {
    const list = listRef.current
    if (!list) return

    const handler = (e: KeyboardEvent) => {
      if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
      const rows = Array.from(list.querySelectorAll('[role="row"]')) as HTMLElement[]
      const focused = document.activeElement as HTMLElement
      const idx = rows.indexOf(focused)
      if (idx === -1) {
        rows[0]?.focus()
        return
      }
      if (e.key === 'ArrowDown' && idx < rows.length - 1) {
        e.preventDefault()
        rows[idx + 1].focus()
      }
      if (e.key === 'ArrowUp' && idx > 0) {
        e.preventDefault()
        rows[idx - 1].focus()
      }
    }
    list.addEventListener('keydown', handler)
    return () => list.removeEventListener('keydown', handler)
  }, [])

  const paths = data?.paths ?? []
  const totalLabel = data ? `${data.total} paths` : ''

  return (
    <div className="flex h-full overflow-hidden">
      {/* Sidebar — prefix tree */}
      <aside
        className="w-52 flex-shrink-0 border-r border-slate-800 bg-slate-900/60 overflow-hidden"
        aria-label="API path groups"
      >
        <div className="px-3 py-2 border-b border-slate-800">
          <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
            Prefixes
          </h2>
        </div>
        <PathTreeSidebar
          tree={tree}
          isLoading={treeLoading}
          activePrefix={filters.prefix}
          onPrefixSelect={(prefix) => setFilter('prefix', prefix)}
        />
      </aside>

      {/* Main content — list + detail */}
      <div className="flex flex-1 overflow-hidden">
        {/* Path list panel */}
        <div className="flex flex-col w-[520px] flex-shrink-0 border-r border-slate-800 overflow-hidden">
          {/* Search + header */}
          <div className="flex items-center gap-2 px-3 py-2 border-b border-slate-800 bg-slate-900/80">
            <div ref={searchRef} className="flex-1">
              <PathSearchInput
                value={filters.q ?? ''}
                onChange={(q) => setFilter('q', q || undefined)}
              />
            </div>
            {totalLabel && (
              <span className="text-xs text-slate-500 flex-shrink-0 tabular-nums">
                {isFetching && !isLoading ? '↻ ' : ''}{totalLabel}
              </span>
            )}
          </div>

          {/* Filters */}
          <ErrorBoundary>
            <PathFilterPanel
              filters={filters}
              setFilter={setFilter}
              clearFilters={clearFilters}
            />
          </ErrorBoundary>

          {/* Path list */}
          <div
            ref={listRef}
            className="flex-1 overflow-y-auto"
            role="grid"
            aria-label="API paths"
            aria-busy={isLoading}
          >
            {isLoading ? (
              <PathListSkeleton count={12} />
            ) : paths.length === 0 ? (
              <EmptyState
                icon={Globe}
                title="No paths match"
                message="Try adjusting the search or filters."
                action={
                  <button
                    type="button"
                    className="text-sm text-sky-400 hover:underline"
                    onClick={clearFilters}
                  >
                    Clear filters
                  </button>
                }
              />
            ) : (
              <div role="rowgroup">
                {paths.map((path) => (
                  <PathRow
                    key={path.path_hash}
                    path={path}
                    group={group}
                    onSelect={() => navigate(`/api/${group}/${path.path_hash}`)}
                  />
                ))}
              </div>
            )}
          </div>

          {/* Pagination */}
          {!isLoading && data && data.total > (data.page_size ?? 50) && (
            <Pagination
              page={data.page}
              pageSize={data.page_size}
              total={data.total}
              hasMore={data.has_more}
              onPageChange={(p) => setFilter('page', p)}
            />
          )}
        </div>

        {/* Detail panel — rendered by nested route */}
        <div className="flex-1 overflow-hidden bg-slate-950">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </div>
      </div>
    </div>
  )
}
