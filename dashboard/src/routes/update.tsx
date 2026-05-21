import { useState, useCallback, useRef, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  fetchUpdateCheck, postUpdatesApply, postRefreshRules,
  type UpdateCheckReply,
} from '@/api/client'
import {
  Download, RefreshCw, CheckCircle2, AlertTriangle, XCircle,
  ArrowUpCircle, RotateCcw, Zap, ExternalLink,
} from 'lucide-react'

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

function relativeTime(isoStr: string | undefined): string {
  if (!isoStr) return 'unknown'
  const ms = Date.now() - new Date(isoStr).getTime()
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

// ─────────────────────────────────────────────────────────────────────────────
// Markdown renderer — minimal, handles ## headings and - list items
// ─────────────────────────────────────────────────────────────────────────────

function ReleaseNotes({ body }: { body: string }) {
  if (!body) return null
  const lines = body.split('\n')
  return (
    <div className="text-sm text-slate-600 dark:text-slate-400 space-y-1">
      {lines.map((line, i) => {
        if (line.startsWith('## ')) {
          return (
            <p key={i} className="font-semibold text-slate-800 dark:text-slate-200 mt-2 first:mt-0">
              {line.slice(3)}
            </p>
          )
        }
        if (line.startsWith('- ') || line.startsWith('* ')) {
          return (
            <p key={i} className="flex items-start gap-1.5 pl-1">
              <span className="text-sky-400 mt-0.5 shrink-0">•</span>
              {line.slice(2)}
            </p>
          )
        }
        if (line.trim() === '') return null
        return <p key={i}>{line}</p>
      })}
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// SSE output console
// ─────────────────────────────────────────────────────────────────────────────

interface ConsoleOutputProps {
  lines: string[]
  exitCode: number | null
  hasError: string | null
}

function ConsoleOutput({ lines, exitCode, hasError }: ConsoleOutputProps) {
  const bottomRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  return (
    <div className="mt-3 rounded border border-slate-200 dark:border-slate-700 bg-slate-950 text-slate-200 font-mono text-xs p-3 max-h-48 overflow-y-auto">
      {lines.map((l, i) => (
        <div key={i} className="leading-5">{l}</div>
      ))}
      {hasError && (
        <div className="text-red-400 mt-1">[error] {hasError}</div>
      )}
      {exitCode !== null && (
        <div className={exitCode === 0 ? 'text-emerald-400 mt-1' : 'text-red-400 mt-1'}>
          [exit {exitCode}]
        </div>
      )}
      <div ref={bottomRef} />
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// Confirm modal
// ─────────────────────────────────────────────────────────────────────────────

interface ConfirmModalProps {
  title: string
  body: string
  confirmLabel: string
  confirmClass: string
  onConfirm: () => void
  onCancel: () => void
}

function ConfirmModal({ title, body, confirmLabel, confirmClass, onConfirm, onCancel }: ConfirmModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-700 shadow-2xl p-6 w-full max-w-md">
        <h2 className="text-base font-semibold text-slate-900 dark:text-slate-100 mb-2">{title}</h2>
        <p className="text-sm text-slate-500 dark:text-slate-400 mb-5">{body}</p>
        <div className="flex items-center gap-3 justify-end">
          <button
            type="button"
            className="px-3 py-1.5 text-sm rounded border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            type="button"
            className={`px-3 py-1.5 text-sm rounded font-medium transition-colors ${confirmClass}`}
            onClick={onConfirm}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdateCard — main version management card
// ─────────────────────────────────────────────────────────────────────────────

type UpdatePhase =
  | 'idle'
  | 'applying'
  | 'done'
  | 'refresh-running'
  | 'refresh-done'

function UpdateCard({ data }: { data: UpdateCheckReply }) {
  const [phase, setPhase] = useState<UpdatePhase>('idle')
  const [showConfirm, setShowConfirm] = useState(false)
  const [outputLines, setOutputLines] = useState<string[]>([])
  const [exitCode, setExitCode] = useState<number | null>(null)
  const [streamError, setStreamError] = useState<string | null>(null)
  const cancelRef = useRef<(() => void) | null>(null)

  const appendLine = useCallback((l: string) => {
    setOutputLines(prev => [...prev, l])
  }, [])

  // Auto-reconnect after successful update: poll /api/updates/check every 2s
  // until the version changes (daemon restarted with new binary).
  useEffect(() => {
    if (phase !== 'done' || exitCode !== 0) return
    const orig = data.current_version
    const interval = setInterval(async () => {
      try {
        const res = await fetch('/api/updates/check')
        if (res.ok) {
          const next = await res.json() as UpdateCheckReply
          if (next.current_version !== orig) {
            clearInterval(interval)
            window.location.reload()
          }
        }
      } catch { /* ignore */ }
    }, 2000)
    return () => clearInterval(interval)
  }, [phase, exitCode, data.current_version])

  const handleApply = useCallback(() => {
    setShowConfirm(false)
    setPhase('applying')
    setOutputLines([])
    setExitCode(null)
    setStreamError(null)
    cancelRef.current = postUpdatesApply(
      appendLine,
      (code) => { setExitCode(code); setPhase('done') },
      (msg) => { setStreamError(msg); setPhase('done') },
    )
  }, [appendLine])

  const handleRefresh = useCallback(() => {
    setPhase('refresh-running')
    setOutputLines([])
    setExitCode(null)
    setStreamError(null)
    cancelRef.current = postRefreshRules(
      appendLine,
      (code) => { setExitCode(code); setPhase('refresh-done') },
      (msg) => { setStreamError(msg); setPhase('refresh-done') },
    )
  }, [appendLine])

  const isBusy = phase === 'applying' || phase === 'refresh-running'

  return (
    <div className="rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/50">
        <div className="flex items-center gap-2">
          <ArrowUpCircle className="w-4 h-4 text-slate-500" />
          <span className="font-semibold text-sm text-slate-800 dark:text-slate-200">Version</span>
          {data.update_available && (
            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold border bg-sky-50 dark:bg-sky-900/30 text-sky-700 dark:text-sky-300 border-sky-200 dark:border-sky-700">
              <Download className="w-3 h-3" />
              Update available
            </span>
          )}
          {!data.update_available && !data.fetch_error && (
            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold border bg-emerald-50 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300 border-emerald-200 dark:border-emerald-700">
              <CheckCircle2 className="w-3 h-3" />
              Up to date
            </span>
          )}
          {data.fetch_error && (
            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold border bg-yellow-50 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 border-yellow-200 dark:border-yellow-700">
              <AlertTriangle className="w-3 h-3" />
              Check failed
            </span>
          )}
        </div>
        <span className="text-xs text-slate-400 dark:text-slate-500">
          Checked {relativeTime(data.checked_at)}
        </span>
      </div>

      {/* Body */}
      <div className="px-4 py-4 space-y-4">
        {/* Current version info */}
        <div className="grid grid-cols-3 gap-4 text-sm">
          <div>
            <div className="text-xs text-slate-400 dark:text-slate-500 mb-0.5">Current version</div>
            <div className="font-mono text-slate-800 dark:text-slate-200">v{data.current_version}</div>
          </div>
          <div>
            <div className="text-xs text-slate-400 dark:text-slate-500 mb-0.5">Commit</div>
            <a
              href={`https://github.com/cajasmota/archigraph/commit/${data.current_commit}`}
              target="_blank"
              rel="noopener noreferrer"
              className="font-mono text-sky-500 hover:text-sky-400 transition-colors flex items-center gap-1"
            >
              {data.current_commit.slice(0, 7)}
              <ExternalLink className="w-3 h-3" />
            </a>
          </div>
          <div>
            <div className="text-xs text-slate-400 dark:text-slate-500 mb-0.5">Built</div>
            <div className="text-slate-600 dark:text-slate-400">{relativeTime(data.current_built_at)}</div>
          </div>
        </div>

        {/* Latest release info */}
        {data.update_available && data.latest_version && (
          <div className="rounded-lg border border-sky-200 dark:border-sky-800 bg-sky-50/50 dark:bg-sky-900/10 p-3 space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Download className="w-4 h-4 text-sky-500" />
                <span className="font-medium text-sm text-slate-800 dark:text-slate-200">
                  v{data.latest_version} available
                </span>
                {data.published_at && (
                  <span className="text-xs text-slate-400">
                    published {relativeTime(data.published_at)}
                  </span>
                )}
              </div>
              {data.latest_html_url && (
                <a
                  href={data.latest_html_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs text-sky-500 hover:text-sky-400 flex items-center gap-1 transition-colors"
                >
                  Release notes <ExternalLink className="w-3 h-3" />
                </a>
              )}
            </div>
            {data.latest_body && (
              <div className="pt-1 border-t border-sky-200/60 dark:border-sky-800/60">
                <ReleaseNotes body={data.latest_body} />
              </div>
            )}
          </div>
        )}

        {/* Fetch error detail */}
        {data.fetch_error && (
          <div className="flex items-start gap-2 text-xs text-yellow-700 dark:text-yellow-400 bg-yellow-50 dark:bg-yellow-900/20 rounded px-3 py-2 border border-yellow-200 dark:border-yellow-800">
            <AlertTriangle className="w-3.5 h-3.5 shrink-0 mt-0.5" />
            <span>Could not check for updates: {data.fetch_error}</span>
          </div>
        )}

        {/* Progress / output console */}
        {(phase === 'applying' || phase === 'done' || phase === 'refresh-running' || phase === 'refresh-done') && (
          <ConsoleOutput lines={outputLines} exitCode={exitCode} hasError={streamError} />
        )}

        {/* Post-update reconnect message */}
        {phase === 'done' && exitCode === 0 && (
          <div className="flex items-center gap-2 text-sm text-emerald-600 dark:text-emerald-400">
            <RefreshCw className="w-4 h-4 animate-spin" />
            Update complete — waiting for daemon to restart…
          </div>
        )}
        {phase === 'done' && exitCode !== null && exitCode !== 0 && (
          <div className="flex items-center gap-2 text-sm text-red-600 dark:text-red-400">
            <XCircle className="w-4 h-4" />
            Update failed (exit {exitCode}). Check the console above.
          </div>
        )}
        {phase === 'refresh-done' && exitCode === 0 && (
          <div className="flex items-center gap-2 text-sm text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="w-4 h-4" />
            Rules refreshed successfully.
          </div>
        )}

        {/* Action buttons */}
        <div className="flex items-center gap-2 pt-1">
          {data.update_available && (
            <button
              type="button"
              disabled={isBusy}
              className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded bg-sky-600 text-white font-medium hover:bg-sky-700 disabled:opacity-50 transition-colors"
              onClick={() => setShowConfirm(true)}
            >
              {phase === 'applying'
                ? <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                : <Download className="w-3.5 h-3.5" />
              }
              {phase === 'applying' ? 'Updating…' : 'Update now'}
            </button>
          )}

          {/* Refresh rules is shown when no full update is available or always as secondary */}
          <button
            type="button"
            disabled={isBusy}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50 transition-colors"
            onClick={handleRefresh}
          >
            {phase === 'refresh-running'
              ? <RefreshCw className="w-3.5 h-3.5 animate-spin" />
              : <Zap className="w-3.5 h-3.5" />
            }
            {phase === 'refresh-running' ? 'Refreshing rules…' : 'Refresh rules'}
          </button>

          {(phase === 'done' || phase === 'refresh-done') && (
            <button
              type="button"
              className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded border border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors"
              onClick={() => { setPhase('idle'); setOutputLines([]); setExitCode(null); setStreamError(null) }}
            >
              <RotateCcw className="w-3.5 h-3.5" />
              Reset
            </button>
          )}
        </div>
      </div>

      {/* Confirm modal */}
      {showConfirm && (
        <ConfirmModal
          title="Apply update?"
          body="This will restart the daemon. Indexing in progress will be paused. The page will reconnect automatically when the daemon is back."
          confirmLabel="Update now"
          confirmClass="bg-sky-600 text-white hover:bg-sky-700"
          onConfirm={handleApply}
          onCancel={() => setShowConfirm(false)}
        />
      )}
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// Skeleton
// ─────────────────────────────────────────────────────────────────────────────

function UpdateSkeleton() {
  return (
    <div className="rounded-lg border border-slate-200 dark:border-slate-700 h-40 bg-slate-100 dark:bg-slate-800 animate-pulse" />
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// Main route — /update
// ─────────────────────────────────────────────────────────────────────────────

export function UpdateRoute() {
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['update-check'],
    queryFn: fetchUpdateCheck,
    staleTime: 60_000,
    refetchInterval: 5 * 60 * 1000, // re-check every 5 min
  })

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-3xl mx-auto px-4 py-6 space-y-6">
        {/* Toolbar */}
        <div className="flex items-center justify-between">
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Update</h1>
          <button
            type="button"
            onClick={() => { void refetch() }}
            disabled={isFetching}
            className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs rounded border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50 transition-colors"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isFetching ? 'animate-spin' : ''}`} />
            Check for updates
          </button>
        </div>

        {isLoading && <UpdateSkeleton />}
        {data && <UpdateCard data={data} />}
      </div>
    </div>
  )
}
