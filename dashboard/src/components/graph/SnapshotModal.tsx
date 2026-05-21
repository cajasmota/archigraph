/**
 * SnapshotModal — "Snapshot view" dialog for /graph.
 *
 * Lets the user choose:
 *   - Format: PNG (raster) or SVG (vector)
 *   - Resolution: 1× / 2× / 4×  (PNG only — SVG is always vector)
 *   - Include legend: yes / no
 *
 * Calls back with the chosen options on confirm; the parent is
 * responsible for calling useSnapshotExport with the full options bag.
 *
 * #1362
 */

import { useState, useCallback } from 'react'
import { X, Camera, Download } from 'lucide-react'
import type { SnapshotFormat, SnapshotResolution } from '@/hooks/graph/useSnapshotExport'

// ── Props ─────────────────────────────────────────────────────────────────────

export interface SnapshotModalProps {
  isOpen: boolean
  onClose: () => void
  /** Called when the user clicks "Download".  Parent kicks off the export. */
  onConfirm: (opts: { format: SnapshotFormat; resolution: SnapshotResolution; includeLegend: boolean }) => void
  /** Whether export is in progress (shows spinner on the button) */
  isExporting?: boolean
  /** Error message from last export attempt */
  exportError?: string
}

// ── Component ─────────────────────────────────────────────────────────────────

const btnBase = [
  'flex items-center gap-1.5 px-3 py-1.5 rounded text-sm font-medium transition-colors',
  'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-400',
].join(' ')

const optionBtn = (active: boolean) =>
  [
    'px-3 py-1.5 rounded border text-xs font-medium transition-colors',
    'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-400',
    active
      ? 'bg-sky-500/20 text-sky-300 border-sky-500/50'
      : 'bg-transparent text-slate-400 border-slate-600 hover:border-slate-500 hover:text-slate-200',
  ].join(' ')

export function SnapshotModal({
  isOpen,
  onClose,
  onConfirm,
  isExporting = false,
  exportError,
}: SnapshotModalProps) {
  const [format, setFormat] = useState<SnapshotFormat>('png')
  const [resolution, setResolution] = useState<SnapshotResolution>(2)
  const [includeLegend, setIncludeLegend] = useState(true)

  const handleConfirm = useCallback(() => {
    onConfirm({ format, resolution, includeLegend })
  }, [format, resolution, includeLegend, onConfirm])

  if (!isOpen) return null

  const isSvg = format === 'svg'

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm"
        aria-hidden
        onClick={onClose}
      />

      {/* Dialog */}
      <div
        role="dialog"
        aria-modal
        aria-label="Snapshot view options"
        className="fixed z-50 inset-0 flex items-center justify-center p-4"
      >
        <div
          className="relative w-full max-w-sm rounded-xl border border-slate-700 bg-slate-900 shadow-2xl p-5"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Header */}
          <div className="flex items-center gap-2 mb-4">
            <Camera className="w-4 h-4 text-slate-400" aria-hidden />
            <h2 className="text-sm font-semibold text-slate-200 flex-1">Snapshot view</h2>
            <button
              type="button"
              onClick={onClose}
              className="p-1 rounded text-slate-500 hover:text-slate-200 hover:bg-slate-800 transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-400"
              aria-label="Close snapshot dialog"
            >
              <X className="w-4 h-4" />
            </button>
          </div>

          {/* Format */}
          <fieldset className="mb-4">
            <legend className="text-[10px] uppercase tracking-wider text-slate-500 font-semibold mb-2">
              Format
            </legend>
            <div className="flex gap-2">
              <button
                type="button"
                className={optionBtn(format === 'png')}
                aria-pressed={format === 'png'}
                onClick={() => setFormat('png')}
                data-testid="snapshot-format-png"
              >
                PNG
                <span className="ml-1 text-[9px] text-slate-500">raster</span>
              </button>
              <button
                type="button"
                className={optionBtn(format === 'svg')}
                aria-pressed={format === 'svg'}
                onClick={() => setFormat('svg')}
                data-testid="snapshot-format-svg"
              >
                SVG
                <span className="ml-1 text-[9px] text-slate-500">vector</span>
              </button>
            </div>
          </fieldset>

          {/* Resolution (PNG only) */}
          {!isSvg && (
            <fieldset className="mb-4">
              <legend className="text-[10px] uppercase tracking-wider text-slate-500 font-semibold mb-2">
                Resolution
              </legend>
              <div className="flex gap-2">
                {([1, 2, 4] as SnapshotResolution[]).map((r) => (
                  <button
                    key={r}
                    type="button"
                    className={optionBtn(resolution === r)}
                    aria-pressed={resolution === r}
                    onClick={() => setResolution(r)}
                    data-testid={`snapshot-res-${r}x`}
                  >
                    {r}×
                  </button>
                ))}
              </div>
              {resolution === 4 && (
                <p className="mt-1.5 text-[10px] text-amber-400/80">
                  4× may produce files &gt;5 MB on large graphs.
                </p>
              )}
            </fieldset>
          )}

          {/* Include legend */}
          <div className="mb-5">
            <label className="flex items-center gap-2 cursor-pointer select-none group">
              <input
                type="checkbox"
                checked={includeLegend}
                onChange={(e) => setIncludeLegend(e.target.checked)}
                className="w-3.5 h-3.5 rounded border border-slate-600 bg-slate-800 accent-sky-500 cursor-pointer focus-visible:ring-1 focus-visible:ring-sky-400"
                data-testid="snapshot-include-legend"
              />
              <span className="text-xs text-slate-400 group-hover:text-slate-200 transition-colors">
                Include legend (color mode, repos, timestamp)
              </span>
            </label>
          </div>

          {/* SVG note */}
          {isSvg && (
            <p className="mb-4 text-[11px] text-slate-500 leading-relaxed">
              SVG export walks the node and edge data to produce a scalable vector image.
              Exact Cosmograph positions are approximated — open in Inkscape or Figma for
              precise layout.
            </p>
          )}

          {/* Error */}
          {exportError && (
            <p className="mb-3 text-xs text-red-400 bg-red-950/30 border border-red-800/40 rounded px-2.5 py-1.5">
              {exportError}
            </p>
          )}

          {/* Actions */}
          <div className="flex gap-2 justify-end">
            <button
              type="button"
              onClick={onClose}
              className={`${btnBase} text-slate-400 hover:text-slate-200 hover:bg-slate-800`}
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleConfirm}
              disabled={isExporting}
              className={`${btnBase} bg-sky-600 hover:bg-sky-500 text-white disabled:opacity-60 disabled:cursor-not-allowed`}
              data-testid="snapshot-download-btn"
            >
              {isExporting
                ? <>
                    <span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" aria-hidden />
                    Exporting…
                  </>
                : <>
                    <Download className="w-3.5 h-3.5" />
                    Download
                  </>
              }
            </button>
          </div>
        </div>
      </div>
    </>
  )
}
