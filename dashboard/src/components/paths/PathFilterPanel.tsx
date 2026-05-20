import { X } from 'lucide-react'
import type { PathFilters } from '@/types/api'

const FRAMEWORKS = ['drf', 'django', 'fastapi', 'gin', 'express', 'flask', 'rails']
const REPOS = ['core-api', 'admin-service', 'gateway']

interface PathFilterPanelProps {
  filters: PathFilters
  setFilter: <K extends keyof PathFilters>(key: K, value: PathFilters[K]) => void
  clearFilters: () => void
}

export function PathFilterPanel({ filters, setFilter, clearFilters }: PathFilterPanelProps) {
  const hasActiveFilters = !!(
    filters.repo || filters.framework || filters.status_code || filters.is_webhook !== undefined
  )

  return (
    <div className="flex flex-wrap items-center gap-2 px-4 py-2 border-b border-slate-800 text-xs">
      {/* Repo chips */}
      <span className="text-slate-500 mr-1">Repo:</span>
      {REPOS.map((repo) => (
        <FilterChip
          key={repo}
          label={repo}
          active={filters.repo === repo}
          onClick={() => setFilter('repo', filters.repo === repo ? undefined : repo)}
        />
      ))}

      <span className="text-slate-700 mx-1">|</span>

      {/* Framework chips */}
      <span className="text-slate-500 mr-1">Framework:</span>
      {FRAMEWORKS.map((fw) => (
        <FilterChip
          key={fw}
          label={fw}
          active={filters.framework === fw}
          onClick={() => setFilter('framework', filters.framework === fw ? undefined : fw)}
        />
      ))}

      <span className="text-slate-700 mx-1">|</span>

      {/* Webhook toggle */}
      <FilterChip
        label="Webhooks only"
        active={filters.is_webhook === true}
        onClick={() =>
          setFilter('is_webhook', filters.is_webhook === true ? undefined : true)
        }
      />

      {/* Clear all */}
      {hasActiveFilters && (
        <button
          type="button"
          className="ml-auto inline-flex items-center gap-1 px-2 py-0.5 rounded text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
          onClick={clearFilters}
          aria-label="Clear all filters"
        >
          <X className="w-3 h-3" />
          Clear
        </button>
      )}
    </div>
  )
}

interface FilterChipProps {
  label: string
  active: boolean
  onClick: () => void
}

function FilterChip({ label, active, onClick }: FilterChipProps) {
  return (
    <button
      type="button"
      aria-pressed={active}
      className={[
        'px-2 py-0.5 rounded border transition-colors font-mono',
        active
          ? 'bg-sky-900/50 border-sky-600 text-sky-300'
          : 'bg-slate-900 border-slate-700 text-slate-400 hover:border-slate-500 hover:text-slate-300',
      ].join(' ')}
      onClick={onClick}
    >
      {label}
    </button>
  )
}
