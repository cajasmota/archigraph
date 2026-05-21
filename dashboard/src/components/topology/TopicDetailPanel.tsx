import { useState, useEffect, useCallback } from 'react'
import {
  X, ArrowUp, ArrowDown, ArrowRight, ExternalLink, Copy, Check,
  ChevronDown, ChevronRight, Info, Clock, Network, GitFork,
  Activity, AlertTriangle, BarChart2, Printer,
} from 'lucide-react'
import { RepoChip } from '@/components/shared/RepoChip'
import { PROTOCOL_COLORS } from '@/lib/colors'
import { useTopicDetailFetch } from '@/hooks/topology/useTopicDetailFetch'
import type { TopicDetailData } from '@/hooks/topology/useTopicDetail'
import type { TopologyEntityStub, TopicNode, QueueNode, NatsSubject } from '@/types/api'
import type { TopicDetailV2, TopicDetailEntityStub, LifecycleState } from '@/types/api'

interface TopicDetailPanelProps {
  detail: TopicDetailData
  group: string
  onClose: () => void
  onNavigateToTopic: (id: string) => void
}

// ── Collapsible section ───────────────────────────────────────────────────────

interface SectionProps {
  id: string
  title: string
  icon: React.ReactNode
  count?: number
  defaultOpen?: boolean
  children: React.ReactNode
}

function Section({ id, title, icon, count, defaultOpen = true, children }: SectionProps) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <section
      aria-labelledby={`tpanel-section-${id}`}
      className="border-b border-slate-200 dark:border-slate-800 last:border-b-0"
    >
      <button
        type="button"
        id={`tpanel-section-${id}`}
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="w-full flex items-center gap-2 px-4 py-2.5 text-xs font-semibold text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-900/60 transition-colors focus:outline-none focus-visible:ring-1 focus-visible:ring-sky-500"
      >
        <span className="text-slate-400 dark:text-slate-500 flex-shrink-0">{icon}</span>
        <span className="flex-1 text-left uppercase tracking-wide text-[10px]">{title}</span>
        {count !== undefined && count > 0 && (
          <span className="inline-flex items-center justify-center min-w-[18px] px-1 py-0.5 text-[10px] rounded-full bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400">
            {count}
          </span>
        )}
        {open
          ? <ChevronDown className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" aria-hidden />
          : <ChevronRight className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" aria-hidden />
        }
      </button>
      {open && <div className="px-4 pb-3 pt-0.5">{children}</div>}
    </section>
  )
}

// ── Copy button ───────────────────────────────────────────────────────────────

function CopyButton({ value, label }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false)
  const copy = useCallback(() => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }, [value])
  return (
    <button
      type="button"
      onClick={copy}
      aria-label={label ?? `Copy ${value}`}
      title={copied ? 'Copied!' : 'Copy ID'}
      className="p-1 rounded text-slate-400 dark:text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors flex-shrink-0"
    >
      {copied ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
    </button>
  )
}

// ── Lifecycle badge ───────────────────────────────────────────────────────────

const LIFECYCLE_STYLES: Record<LifecycleState, { badge: string; label: string; hint: string }> = {
  active: {
    badge: 'bg-emerald-900/40 text-emerald-300 border-emerald-700',
    label: 'ACTIVE',
    hint: 'Topic has both producers and consumers.',
  },
  orphan_publisher: {
    badge: 'bg-amber-900/40 text-amber-300 border-amber-700',
    label: 'ORPHAN PUBLISHER',
    hint: 'Topic is published to but has no subscribers — messages may be lost.',
  },
  orphan_subscriber: {
    badge: 'bg-orange-900/40 text-orange-300 border-orange-700',
    label: 'ORPHAN SUBSCRIBER',
    hint: 'Topic is subscribed to but has no publishers — consumers will never receive messages.',
  },
  orphan: {
    badge: 'bg-red-900/40 text-red-300 border-red-700',
    label: 'ORPHAN',
    hint: 'Topic has neither producers nor consumers. May be unused or extraction may be incomplete.',
  },
}

function LifecycleBadge({ state }: { state: LifecycleState }) {
  const spec = LIFECYCLE_STYLES[state]
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold border tracking-wider ${spec.badge}`}
      aria-label={`Lifecycle: ${spec.label}`}
    >
      {spec.label}
    </span>
  )
}

// ── Entity row (producers / consumers) ───────────────────────────────────────

function EntityRow({
  stub,
  onNavigateToTopic,
}: {
  stub: TopicDetailEntityStub
  onNavigateToTopic?: (id: string) => void
}) {
  return (
    <li className="flex items-start gap-2 px-3 py-2 rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-800">
      <div className="flex-1 min-w-0">
        <p className="font-mono text-xs text-slate-800 dark:text-slate-200 truncate" title={stub.name}>
          {stub.name}
        </p>
        <p className="font-mono text-[10px] text-slate-400 dark:text-slate-500 truncate" title={`${stub.source_file}:${stub.start_line}`}>
          {stub.source_file}:{stub.start_line}
        </p>
      </div>
      <RepoChip repo={stub.repo} className="flex-shrink-0 mt-0.5" />
      {onNavigateToTopic && (
        <button
          type="button"
          aria-label={`Go to entity ${stub.name}`}
          onClick={() => onNavigateToTopic(stub.entity_id)}
          className="p-1 rounded text-slate-400 dark:text-slate-500 hover:text-sky-400 hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors flex-shrink-0"
        >
          <ExternalLink className="w-3 h-3" />
        </button>
      )}
    </li>
  )
}

// ── Fallback entity row (from base TopicDetailData) ──────────────────────────

function LegacyEntitySection({
  title,
  icon,
  entities,
  emptyMessage,
}: {
  title: string
  icon: React.ReactNode
  entities: TopologyEntityStub[]
  emptyMessage: string
}) {
  return (
    <Section id={`legacy-${title.toLowerCase()}`} title={title} icon={icon} count={entities.length}>
      {entities.length === 0 ? (
        <p className="text-xs text-slate-500 dark:text-slate-600 italic">{emptyMessage}</p>
      ) : (
        <ul className="space-y-1.5">
          {entities.map((e) => (
            <li
              key={e.id}
              className="flex items-start gap-2 px-3 py-2 rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-800"
            >
              <div className="flex-1 min-w-0">
                <p className="font-mono text-xs text-slate-800 dark:text-slate-200 truncate" title={e.label}>
                  {e.label}
                </p>
                <p className="font-mono text-[10px] text-slate-400 dark:text-slate-500 truncate" title={e.source_file}>
                  {e.source_file}:{e.start_line}
                </p>
              </div>
              <RepoChip repo={e.repo} className="flex-shrink-0 mt-0.5" />
            </li>
          ))}
        </ul>
      )}
    </Section>
  )
}

// ── Transform chains (from base detail) ──────────────────────────────────────

function TransformSection({
  transformsTo,
  onNavigateToTopic,
}: {
  transformsTo: Array<TopicNode | QueueNode | NatsSubject>
  onNavigateToTopic: (id: string) => void
}) {
  if (transformsTo.length === 0) return null
  return (
    <Section id="transforms" title="Transforms To" icon={<ArrowRight className="w-3.5 h-3.5" />} count={transformsTo.length} defaultOpen={false}>
      <ul className="space-y-1.5">
        {transformsTo.map((t) => (
          <li key={t.id} className="flex items-center gap-2 px-3 py-2 rounded-lg bg-amber-950/20 border border-amber-900/40">
            <p className="font-mono text-xs text-amber-200 flex-1 truncate" title={t.label}>
              {t.label}
            </p>
            <button
              type="button"
              aria-label={`Navigate to ${t.label}`}
              onClick={() => onNavigateToTopic(t.id)}
              className="p-1 rounded text-amber-500 hover:text-amber-300 hover:bg-amber-900/30 transition-colors"
            >
              <ExternalLink className="w-3 h-3" />
            </button>
          </li>
        ))}
      </ul>
    </Section>
  )
}

// ── V2 rich panel ─────────────────────────────────────────────────────────────

interface RichPanelProps {
  v2: TopicDetailV2
  base: TopicDetailData
  group: string
  onClose: () => void
  onNavigateToTopic: (id: string) => void
}

function RichTopicPanel({ v2, base, group, onClose, onNavigateToTopic }: RichPanelProps) {
  const node = base.node
  if (!node) return null

  const rawProtocol = 'broker' in node
    ? (node as TopicNode | QueueNode | NatsSubject).broker
    : 'channel_type' in node
      ? (node as { channel_type: string }).channel_type
      : 'graphql_subscription'
  const hasFramework = 'framework' in node && !!(node as QueueNode).framework
  const protocol = (!rawProtocol || hasFramework) ? 'task-queue' : rawProtocol
  const spec = PROTOCOL_COLORS[protocol as keyof typeof PROTOCOL_COLORS] ?? PROTOCOL_COLORS['task-queue']

  const lifecycleSpec = LIFECYCLE_STYLES[v2.lifecycle_state]

  // Keyboard: Esc closes
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <aside
      className="w-80 flex flex-col border-l border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 overflow-y-auto"
      aria-label={`Topic detail: ${v2.label}`}
      data-testid="topic-detail-panel"
    >
      {/* ── Header ─────────────────────────────────────────────────────── */}
      <div className={`flex items-start gap-2 px-4 py-3 border-b border-slate-200 dark:border-slate-800 ${spec.bg}`}>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-1.5 mb-1 flex-wrap">
            <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium border ${spec.bg} ${spec.text} ${spec.border}`}>
              {spec.label}
            </span>
            {v2.framework && (
              <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-lime-900/30 text-lime-300 border border-lime-700">
                {v2.framework}
              </span>
            )}
            {v2.scheduled && (
              <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-violet-900/30 text-violet-300 border border-violet-700">
                <Clock className="w-2.5 h-2.5" aria-hidden />
                scheduled
              </span>
            )}
            <LifecycleBadge state={v2.lifecycle_state} />
          </div>

          <p className="font-mono text-sm font-semibold text-slate-900 dark:text-slate-100 truncate" title={v2.label}>
            {v2.label}
          </p>
        </div>

        <div className="flex items-center gap-0.5 flex-shrink-0">
          <CopyButton value={v2.id} label="Copy topic ID" />
          <button
            type="button"
            aria-label="Print-friendly view"
            onClick={() => window.print()}
            className="p-1.5 rounded text-slate-400 dark:text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors"
          >
            <Printer className="w-3.5 h-3.5" />
          </button>
          <button
            type="button"
            aria-label="Close topic detail panel"
            onClick={onClose}
            className="p-1.5 rounded text-slate-400 dark:text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* ── Inline actions ─────────────────────────────────────────────── */}
      <div className="flex items-center gap-2 px-4 py-2 border-b border-slate-200 dark:border-slate-800 flex-wrap">
        <a
          href={`/graph/${group}?highlight=${encodeURIComponent(v2.id)}`}
          className="inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-sky-900/20 text-sky-300 border border-sky-800 hover:bg-sky-900/40 transition-colors"
          title="Open in graph view with this node pre-selected"
        >
          <Network className="w-3 h-3" aria-hidden />
          Open in graph
        </a>

        {v2.flow_count > 0 && (
          <a
            href={`/flows/${group}?topic=${encodeURIComponent(v2.id)}`}
            className="inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-indigo-900/20 text-indigo-300 border border-indigo-800 hover:bg-indigo-900/40 transition-colors"
            title={`Used in ${v2.flow_count} process flow${v2.flow_count !== 1 ? 's' : ''}`}
          >
            <BarChart2 className="w-3 h-3" aria-hidden />
            {v2.flow_count} flow{v2.flow_count !== 1 ? 's' : ''}
          </a>
        )}

        {(v2.lifecycle_state === 'orphan_publisher' || v2.lifecycle_state === 'orphan') && (
          <button
            type="button"
            title="Mark as expected orphan (stub — no backend yet)"
            className="inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-slate-800 text-slate-400 border border-slate-700 hover:bg-slate-700 transition-colors cursor-not-allowed opacity-60"
            disabled
            aria-disabled="true"
          >
            <Check className="w-3 h-3" aria-hidden />
            Mark expected orphan
          </button>
        )}
      </div>

      {/* ── Scrollable body ────────────────────────────────────────────── */}
      <div className="flex-1 overflow-y-auto divide-y-0">

        {/* 1. AI Summary */}
        <Section id="ai-summary" title="AI Summary" icon={<Info className="w-3.5 h-3.5" />} defaultOpen>
          {v2.docs_summary ? (
            <div className="flex flex-col gap-1">
              <p className="text-xs text-slate-700 dark:text-slate-300 leading-relaxed">{v2.docs_summary}</p>
              <span className="self-start mt-1 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-emerald-900/30 text-emerald-300 border border-emerald-700">
                Enriched
              </span>
            </div>
          ) : (
            <div className="flex items-start gap-2 px-3 py-2.5 rounded-lg bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800">
              <Info className="w-3.5 h-3.5 text-slate-400 flex-shrink-0 mt-0.5" aria-hidden />
              <p className="text-xs text-slate-400 dark:text-slate-500 italic">
                No enrichment yet. Run{' '}
                <code className="font-mono text-[10px] text-sky-400">/generate-docs</code>{' '}
                to enable AI summaries.
              </p>
            </div>
          )}
        </Section>

        {/* 2. Message Schema */}
        {v2.message_schema && (
          <Section id="schema" title="Message Schema" icon={<GitFork className="w-3.5 h-3.5" />} defaultOpen>
            <pre className="text-[10px] font-mono text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap break-all">
              {(() => {
                try { return JSON.stringify(JSON.parse(v2.message_schema), null, 2) }
                catch { return v2.message_schema }
              })()}
            </pre>
          </Section>
        )}

        {/* 3. Schedule */}
        {v2.scheduled && v2.schedule && (
          <Section id="schedule" title="Schedule" icon={<Clock className="w-3.5 h-3.5" />} defaultOpen>
            <div className="flex flex-col gap-1">
              <code className="font-mono text-xs text-violet-300 bg-violet-900/20 border border-violet-800 rounded px-2 py-1">
                {v2.schedule}
              </code>
              <p className="text-[10px] text-slate-400 dark:text-slate-500">
                Cron expression — scheduled via {v2.framework ?? 'task scheduler'}
              </p>
            </div>
          </Section>
        )}

        {/* 4. Producers */}
        <Section
          id="producers"
          title="Producers"
          icon={<ArrowUp className="w-3.5 h-3.5" />}
          count={v2.producers.length}
          defaultOpen={v2.producers.length > 0}
        >
          {v2.producers.length === 0 ? (
            <p className="text-xs text-slate-500 dark:text-slate-600 italic">No producers found.</p>
          ) : (
            <ul className="space-y-1.5" aria-label="Producer entities">
              {v2.producers.map((s) => (
                <EntityRow key={s.entity_id} stub={s} onNavigateToTopic={onNavigateToTopic} />
              ))}
            </ul>
          )}
        </Section>

        {/* 5. Consumers */}
        <Section
          id="consumers"
          title="Consumers"
          icon={<ArrowDown className="w-3.5 h-3.5" />}
          count={v2.consumers.length}
          defaultOpen={v2.consumers.length > 0}
        >
          {v2.consumers.length === 0 ? (
            <p className="text-xs text-slate-500 dark:text-slate-600 italic">No consumers found.</p>
          ) : (
            <ul className="space-y-1.5" aria-label="Consumer entities">
              {v2.consumers.map((s) => (
                <EntityRow key={s.entity_id} stub={s} onNavigateToTopic={onNavigateToTopic} />
              ))}
            </ul>
          )}
        </Section>

        {/* 6. Lifecycle */}
        <Section id="lifecycle" title="Lifecycle" icon={<Activity className="w-3.5 h-3.5" />} defaultOpen>
          <div className="flex flex-col gap-2">
            <LifecycleBadge state={v2.lifecycle_state} />
            <p className="text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
              {lifecycleSpec.hint}
            </p>
          </div>
        </Section>

        {/* 7. Cross-repo */}
        {v2.cross_repo && (
          <Section id="cross-repo" title="Cross-repo" icon={<GitFork className="w-3.5 h-3.5" />} defaultOpen>
            <div className="flex items-start gap-2 px-3 py-2.5 rounded-lg bg-indigo-900/20 border border-indigo-800">
              <AlertTriangle className="w-3.5 h-3.5 text-indigo-400 flex-shrink-0 mt-0.5" aria-hidden />
              <div>
                <p className="text-xs text-indigo-300 font-medium mb-0.5">Cross-repository traffic</p>
                <p className="text-[10px] text-indigo-400/80 leading-relaxed">
                  {v2.producers.length > 0 && (
                    <>Producers in: {[...new Set(v2.producers.map((p) => p.repo))].join(', ')}<br /></>
                  )}
                  {v2.consumers.length > 0 && (
                    <>Consumers in: {[...new Set(v2.consumers.map((c) => c.repo))].join(', ')}</>
                  )}
                </p>
              </div>
            </div>
          </Section>
        )}

        {/* 8. Related Topics */}
        {v2.related_topics.length > 0 && (
          <Section
            id="related-topics"
            title="Related Topics"
            icon={<Network className="w-3.5 h-3.5" />}
            count={v2.related_topics.length}
            defaultOpen={false}
          >
            <div className="flex flex-wrap gap-1.5">
              {v2.related_topics.map((rt) => (
                <button
                  key={rt.id}
                  type="button"
                  onClick={() => onNavigateToTopic(rt.id)}
                  className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-mono bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 hover:border-sky-600 hover:text-sky-400 transition-colors"
                  title={`Navigate to ${rt.label}`}
                >
                  {rt.label}
                </button>
              ))}
            </div>
          </Section>
        )}

        {/* 9. Enrichment gaps + volume */}
        {v2.enrichment && (v2.enrichment.gaps?.length || v2.enrichment.volume_estimate) && (
          <Section id="enrichment" title="Enrichment Details" icon={<BarChart2 className="w-3.5 h-3.5" />} defaultOpen={false}>
            <div className="space-y-2">
              {v2.enrichment.volume_estimate && (
                <p className="text-xs text-slate-500 dark:text-slate-400">
                  Volume estimate: <span className="text-slate-700 dark:text-slate-300 font-medium">{v2.enrichment.volume_estimate}</span>
                  {v2.enrichment.typical_payload_size_bytes !== undefined && (
                    <> · Typical payload: <span className="font-medium">{v2.enrichment.typical_payload_size_bytes} B</span></>
                  )}
                </p>
              )}
              {(v2.enrichment.gaps?.length ?? 0) > 0 && (
                <>
                  <p className="text-[10px] text-slate-400 uppercase tracking-wide font-semibold">Documentation gaps</p>
                  <ul className="space-y-0.5">
                    {v2.enrichment.gaps!.map((gap, i) => (
                      <li key={i} className="flex items-start gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                        <span className="text-slate-400 flex-shrink-0 mt-0.5">•</span>
                        {gap}
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </div>
          </Section>
        )}

        {/* 10. Transform chains (from base data) */}
        <TransformSection transformsTo={base.transformsTo} onNavigateToTopic={onNavigateToTopic} />

        {/* Last-rebuild timestamp */}
        {v2.last_rebuilt && (
          <div className="px-4 py-2 border-t border-slate-200 dark:border-slate-800 flex items-center gap-1.5">
            <Clock className="w-3 h-3 text-slate-400 dark:text-slate-600 flex-shrink-0" aria-hidden />
            <span className="text-[10px] text-slate-400 dark:text-slate-600">
              Last rebuilt {new Date(v2.last_rebuilt).toLocaleString()}
            </span>
          </div>
        )}
      </div>
    </aside>
  )
}

// ── Thin fallback panel (same as before, no v2 data yet) ─────────────────────

function LegacyTopicPanel({
  detail,
  onClose,
  onNavigateToTopic,
}: {
  detail: TopicDetailData
  onClose: () => void
  onNavigateToTopic: (id: string) => void
}) {
  const { node, producers, consumers, transformsTo } = detail
  if (!node) return null

  const rawProtocol = 'broker' in node
    ? (node as TopicNode | QueueNode | NatsSubject).broker
    : 'channel_type' in node
      ? (node as { channel_type: string }).channel_type
      : 'graphql_subscription'
  const hasFramework = 'framework' in node && !!(node as QueueNode).framework
  const protocol = (!rawProtocol || hasFramework) ? 'task-queue' : rawProtocol
  const spec = PROTOCOL_COLORS[protocol as keyof typeof PROTOCOL_COLORS] ?? PROTOCOL_COLORS['task-queue']
  const label = node.label

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <aside
      className="w-80 flex flex-col border-l border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 overflow-y-auto"
      aria-label={`Topic detail: ${label}`}
      data-testid="topic-detail-panel"
    >
      <div className={`flex items-start gap-2 px-4 py-3 border-b border-slate-200 dark:border-slate-800 ${spec.bg}`}>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-0.5">
            <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium ${spec.bg} ${spec.text} ${spec.border} border`}>
              {spec.label}
            </span>
          </div>
          <p className="font-mono text-sm text-slate-900 dark:text-slate-100 truncate" title={label}>
            {label}
          </p>
        </div>
        <button
          type="button"
          aria-label="Close topic detail panel"
          onClick={onClose}
          className="p-1.5 rounded text-slate-400 dark:text-slate-500 hover:text-slate-700 dark:hover:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors flex-shrink-0 mt-0.5"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      <div className="flex items-center gap-3 px-4 py-2 border-b border-slate-200 dark:border-slate-800 text-xs">
        <RepoChip repo={node.repo} />
        <span className="text-slate-500 dark:text-slate-600">
          {producers.length} producer{producers.length !== 1 ? 's' : ''}
        </span>
        <span className="text-slate-500 dark:text-slate-600">
          {consumers.length} consumer{consumers.length !== 1 ? 's' : ''}
        </span>
      </div>

      <div className="flex-1 overflow-y-auto divide-y-0">
        <LegacyEntitySection
          title="Producers"
          icon={<ArrowUp className="w-3.5 h-3.5" />}
          entities={producers}
          emptyMessage="No producers found"
        />
        <LegacyEntitySection
          title="Consumers"
          icon={<ArrowDown className="w-3.5 h-3.5" />}
          entities={consumers}
          emptyMessage="No consumers found"
        />
        <TransformSection transformsTo={transformsTo} onNavigateToTopic={onNavigateToTopic} />
      </div>
    </aside>
  )
}

// ── Loading skeleton ──────────────────────────────────────────────────────────

function PanelSkeleton({ onClose }: { onClose: () => void }) {
  return (
    <aside
      className="w-80 flex flex-col border-l border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950"
      data-testid="topic-detail-panel"
    >
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800">
        <div className="h-4 w-32 bg-slate-200 dark:bg-slate-800 rounded animate-pulse" />
        <button type="button" aria-label="Close" onClick={onClose} className="p-1.5 rounded text-slate-400 hover:text-slate-700">
          <X className="w-4 h-4" />
        </button>
      </div>
      <div className="p-4 space-y-3">
        {[80, 60, 90, 50].map((w, i) => (
          <div key={i} className={`h-3 bg-slate-200 dark:bg-slate-800 rounded animate-pulse`} style={{ width: `${w}%` }} />
        ))}
      </div>
    </aside>
  )
}

// ── Public component ──────────────────────────────────────────────────────────

/**
 * Rich topic detail panel — fetches the v2 endpoint (#1138) and renders
 * all sections. Falls back to the thin legacy panel while loading or on error.
 * Prop interface is backward-compatible with #1117 partial work.
 */
export function TopicDetailPanel({ detail, group, onClose, onNavigateToTopic }: TopicDetailPanelProps) {
  const topicId = detail.node?.id ?? null
  const { data: v2, isLoading } = useTopicDetailFetch(group, topicId)

  if (isLoading && !v2) {
    return <PanelSkeleton onClose={onClose} />
  }

  if (v2) {
    return (
      <RichTopicPanel
        v2={v2}
        base={detail}
        group={group}
        onClose={onClose}
        onNavigateToTopic={onNavigateToTopic}
      />
    )
  }

  // Fallback: v2 fetch failed or data absent — show legacy panel
  return (
    <LegacyTopicPanel
      detail={detail}
      onClose={onClose}
      onNavigateToTopic={onNavigateToTopic}
    />
  )
}
