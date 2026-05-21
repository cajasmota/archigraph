/**
 * Skills surface (#1354)
 *
 * Two tabs:
 *   Installed — skills present in skills/ directory with metrics + Remove button
 *   Marketplace — static catalog with search, Install button (disabled if installed)
 */

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  fetchSkillsInstalled,
  fetchSkillsAvailable,
  installSkill,
  uninstallSkill,
  type InstalledSkill,
  type CatalogSkill,
} from '@/api/client'
import {
  BookOpen, Download, Trash2, Search, RefreshCw,
  CheckCircle2, AlertTriangle, Package, Clock, Activity,
  ArrowUpCircle, ExternalLink,
} from 'lucide-react'

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

function relativeTime(isoStr: string | undefined): string {
  if (!isoStr) return 'never'
  const ms = Date.now() - new Date(isoStr).getTime()
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

function sourceBadge(source: string) {
  if (source === 'archigraph-bundled') {
    return (
      <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300">
        <Package className="w-3 h-3" /> bundled
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300">
      <ExternalLink className="w-3 h-3" /> community
    </span>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// Installed tab
// ─────────────────────────────────────────────────────────────────────────────

function InstalledTab() {
  const queryClient = useQueryClient()
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['skills', 'installed'],
    queryFn: fetchSkillsInstalled,
    staleTime: 30_000,
  })

  const uninstallMut = useMutation({
    mutationFn: (slug: string) => uninstallSkill(slug),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skills'] })
    },
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20 text-slate-400">
        <RefreshCw className="w-5 h-5 animate-spin mr-2" /> Loading installed skills…
      </div>
    )
  }

  if (isError) {
    return (
      <div className="flex items-center gap-2 rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 px-4 py-3 text-sm text-red-600 dark:text-red-400">
        <AlertTriangle className="w-4 h-4 shrink-0" />
        Failed to load installed skills. <button className="underline ml-1" onClick={() => refetch()}>Retry</button>
      </div>
    )
  }

  const skills = data?.skills ?? []
  const skillsDir = data?.skills_dir ?? 'skills/'

  if (skills.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-slate-400 gap-3">
        <BookOpen className="w-10 h-10 opacity-40" />
        <p className="text-sm">No skills installed. Browse the Marketplace tab to add some.</p>
        <p className="text-xs font-mono text-slate-500">{skillsDir}</p>
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-slate-500 dark:text-slate-400 font-mono">{skillsDir}</p>
      {skills.map((sk) => (
        <InstalledCard key={sk.slug} skill={sk} onRemove={() => uninstallMut.mutate(sk.slug)} removing={uninstallMut.isPending && uninstallMut.variables === sk.slug} />
      ))}
    </div>
  )
}

function InstalledCard({ skill, onRemove, removing }: { skill: InstalledSkill; onRemove: () => void; removing: boolean }) {
  return (
    <div
      data-testid={`skill-installed-${skill.slug}`}
      className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-3 flex gap-4 items-start"
    >
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 font-mono">{skill.slug}</h3>
          {skill.version && (
            <span className="text-xs text-slate-400">v{skill.version}</span>
          )}
          {skill.update_available && (
            <span className="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
              <ArrowUpCircle className="w-3 h-3" /> update available
            </span>
          )}
        </div>
        <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5 line-clamp-2">{skill.description || '—'}</p>
        <div className="flex items-center gap-4 mt-1.5 text-xs text-slate-400">
          <span className="flex items-center gap-1">
            <Clock className="w-3 h-3" />
            last invoked: {relativeTime(skill.last_invoked_at)}
          </span>
          <span className="flex items-center gap-1">
            <Activity className="w-3 h-3" />
            {skill.total_invocations} invocations
          </span>
        </div>
      </div>
      <button
        data-testid={`btn-remove-${skill.slug}`}
        onClick={onRemove}
        disabled={removing}
        className="shrink-0 flex items-center gap-1 px-2.5 py-1.5 rounded text-xs font-medium text-red-500 hover:bg-red-50 dark:hover:bg-red-900/30 border border-red-200 dark:border-red-800 disabled:opacity-50 transition-colors"
      >
        {removing ? <RefreshCw className="w-3 h-3 animate-spin" /> : <Trash2 className="w-3 h-3" />}
        Remove
      </button>
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// Marketplace tab
// ─────────────────────────────────────────────────────────────────────────────

function MarketplaceTab() {
  const [query, setQuery] = useState('')
  const queryClient = useQueryClient()

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['skills', 'available'],
    queryFn: fetchSkillsAvailable,
    staleTime: 60_000,
  })

  const installMut = useMutation({
    mutationFn: (slug: string) => installSkill(slug),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skills'] })
    },
  })

  const skills = data?.skills ?? []
  const filtered = query.trim()
    ? skills.filter((s) =>
        s.name.toLowerCase().includes(query.toLowerCase()) ||
        s.description.toLowerCase().includes(query.toLowerCase()),
      )
    : skills

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20 text-slate-400">
        <RefreshCw className="w-5 h-5 animate-spin mr-2" /> Loading marketplace…
      </div>
    )
  }

  if (isError) {
    return (
      <div className="flex items-center gap-2 rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 px-4 py-3 text-sm text-red-600 dark:text-red-400">
        <AlertTriangle className="w-4 h-4 shrink-0" />
        Failed to load marketplace. <button className="underline ml-1" onClick={() => refetch()}>Retry</button>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Search */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400 pointer-events-none" />
        <input
          data-testid="marketplace-search"
          type="search"
          placeholder="Search skills…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="w-full pl-9 pr-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-sm text-slate-800 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-sky-400"
        />
      </div>

      {filtered.length === 0 && (
        <p className="text-sm text-slate-400 text-center py-10">No skills match "{query}".</p>
      )}

      <div className="grid gap-3 sm:grid-cols-2">
        {filtered.map((sk) => (
          <CatalogCard
            key={sk.slug}
            skill={sk}
            onInstall={() => installMut.mutate(sk.slug)}
            installing={installMut.isPending && installMut.variables === sk.slug}
          />
        ))}
      </div>
    </div>
  )
}

function CatalogCard({ skill, onInstall, installing }: { skill: CatalogSkill; onInstall: () => void; installing: boolean }) {
  return (
    <div
      data-testid={`skill-catalog-${skill.slug}`}
      className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-3 flex flex-col gap-2"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="text-sm font-semibold text-slate-800 dark:text-slate-100 font-mono">{skill.slug}</h3>
            {sourceBadge(skill.source)}
            {skill.version && skill.version !== 'bundled' && (
              <span className="text-xs text-slate-400">v{skill.version}</span>
            )}
          </div>
          <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5 line-clamp-2">{skill.description}</p>
        </div>
      </div>

      {skill.when_to_use && (
        <p className="text-xs text-slate-400 italic line-clamp-1">When: {skill.when_to_use}</p>
      )}

      <div className="flex items-center justify-between gap-2 pt-0.5">
        {skill.install_url && (
          <a
            href={skill.install_url}
            target="_blank"
            rel="noopener noreferrer"
            className="text-xs text-sky-500 hover:underline flex items-center gap-0.5"
          >
            <ExternalLink className="w-3 h-3" /> source
          </a>
        )}
        <div className="ml-auto">
          {skill.installed ? (
            <span className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 className="w-3 h-3" /> Installed
            </span>
          ) : (
            <button
              data-testid={`btn-install-${skill.slug}`}
              onClick={onInstall}
              disabled={installing}
              className="flex items-center gap-1 px-2.5 py-1 rounded text-xs font-medium bg-sky-500 hover:bg-sky-600 text-white disabled:opacity-50 transition-colors"
            >
              {installing ? <RefreshCw className="w-3 h-3 animate-spin" /> : <Download className="w-3 h-3" />}
              Install
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// Page root
// ─────────────────────────────────────────────────────────────────────────────

type Tab = 'installed' | 'marketplace'

export default function SkillsPage() {
  const [tab, setTab] = useState<Tab>('installed')

  const tabCls = (t: Tab) =>
    [
      'px-4 py-2 text-sm font-medium rounded-t-lg border-b-2 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400',
      tab === t
        ? 'border-sky-500 text-sky-600 dark:text-sky-400'
        : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200',
    ].join(' ')

  return (
    <div className="max-w-3xl mx-auto px-4 py-6 space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <BookOpen className="w-6 h-6 text-sky-500 shrink-0" />
        <div>
          <h1 className="text-xl font-bold text-slate-800 dark:text-slate-100">Skills</h1>
          <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            Manage AI agent skills — installed automations and the available marketplace.
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-slate-200 dark:border-slate-700 flex gap-1">
        <button data-testid="tab-installed" className={tabCls('installed')} onClick={() => setTab('installed')}>
          Installed
        </button>
        <button data-testid="tab-marketplace" className={tabCls('marketplace')} onClick={() => setTab('marketplace')}>
          Marketplace
        </button>
      </div>

      {/* Content */}
      {tab === 'installed' ? <InstalledTab /> : <MarketplaceTab />}
    </div>
  )
}
