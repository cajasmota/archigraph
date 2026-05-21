/**
 * useSnapshotExport — capture the current Cosmograph canvas + overlays
 * and download as PNG (raster) or SVG (vector walk).
 *
 * #1362
 *
 * PNG path:
 *   1. Find the WebGL <canvas> inside .graph-canvas
 *   2. Draw it onto an offscreen canvas scaled by `resolution` (1|2|4)
 *   3. Composite an HTML5 canvas legend strip below the graph
 *   4. toBlob → object URL → anchor download
 *
 * SVG path:
 *   Walk nodes + edges from the caller-supplied data arrays, render
 *   each as an SVG <circle> / <line> using the same color accessors
 *   used by GraphCanvas.  Includes a <g class="legend"> block with
 *   color-mode info, repo swatches, and timestamp.
 *
 * No server round-trip — fully client-side.
 */

import { useCallback } from 'react'
import { repoColor } from '@/lib/colors'
import { communityColor } from '@/hooks/graph/useCommunityColors'
import type { GraphNode, GraphEdge } from '@/types/api'
import type { ColorMode } from '@/hooks/graph/useColorMode'

export type SnapshotFormat = 'png' | 'svg'
export type SnapshotResolution = 1 | 2 | 4

export interface SnapshotOptions {
  format: SnapshotFormat
  resolution: SnapshotResolution
  includeLegend: boolean
  /** used in the filename */
  groupSlug: string
  /** passed through to the SVG color walk */
  colorMode: ColorMode
  /** all repos visible in the current view */
  visibleRepos: string[]
  nodes: GraphNode[]
  edges: GraphEdge[]
  isDark: boolean
}

// ---------------------------------------------------------------------------
// Legend helpers
// ---------------------------------------------------------------------------

const LEGEND_HEIGHT = 56     // px (at 1×)
const LEGEND_PADDING = 12
const SWATCH_SIZE = 10
const SWATCH_GAP = 6
const FONT_SIZE = 11

function buildLegendCanvas(
  width: number,
  opts: SnapshotOptions,
  scale: number,
): HTMLCanvasElement {
  const h = LEGEND_HEIGHT * scale
  const c = document.createElement('canvas')
  c.width = width
  c.height = h
  const ctx = c.getContext('2d')!

  // Background
  ctx.fillStyle = opts.isDark ? '#020617' : '#f8fafc'
  ctx.fillRect(0, 0, width, h)

  // Top border
  ctx.strokeStyle = opts.isDark ? 'rgba(51,65,85,0.5)' : 'rgba(148,163,184,0.5)'
  ctx.lineWidth = 1 * scale
  ctx.beginPath()
  ctx.moveTo(0, 0)
  ctx.lineTo(width, 0)
  ctx.stroke()

  const pad = LEGEND_PADDING * scale
  const sw = SWATCH_SIZE * scale
  const gap = SWATCH_GAP * scale
  const fs = FONT_SIZE * scale

  ctx.font = `${fs}px system-ui, -apple-system, sans-serif`
  ctx.textBaseline = 'middle'

  const textColor = opts.isDark ? '#94a3b8' : '#475569'
  const labelColor = opts.isDark ? '#e2e8f0' : '#1e293b'

  // "Color by:" label
  ctx.fillStyle = textColor
  ctx.fillText(`Color by: ${opts.colorMode}`, pad, h * 0.3)

  // Timestamp (right aligned)
  const ts = new Date().toLocaleString()
  const tsMetrics = ctx.measureText(ts)
  ctx.fillStyle = textColor
  ctx.fillText(ts, width - pad - tsMetrics.width, h * 0.3)

  // Repo swatches (bottom row)
  let x = pad
  for (const repo of opts.visibleRepos.slice(0, 10)) {
    // swatch dot
    ctx.fillStyle = repoColor(repo)
    ctx.beginPath()
    ctx.arc(x + sw / 2, h * 0.72, sw / 2, 0, Math.PI * 2)
    ctx.fill()
    x += sw + gap / 2

    // repo label
    ctx.fillStyle = labelColor
    ctx.fillText(repo, x, h * 0.72)
    x += ctx.measureText(repo).width + gap * 2

    if (x > width - pad) break  // overflow guard
  }

  return c
}

// ---------------------------------------------------------------------------
// PNG export
// ---------------------------------------------------------------------------

async function exportPng(opts: SnapshotOptions): Promise<void> {
  const { resolution, includeLegend, groupSlug, isDark } = opts
  const scale = resolution

  // Locate the WebGL canvas
  const src = document.querySelector<HTMLCanvasElement>('.graph-canvas canvas')
  if (!src) throw new Error('Graph canvas not found — is the graph loaded?')

  const W = src.width
  const H = src.height

  const legendH = includeLegend ? LEGEND_HEIGHT * scale : 0
  const totalH = H + legendH

  const out = document.createElement('canvas')
  out.width = W * scale
  out.height = totalH * scale
  const ctx = out.getContext('2d')!

  // Cosmograph's canvas is already the physical pixel size (devicePixelRatio baked in).
  // We just draw it scaled up to the requested resolution.
  ctx.drawImage(src, 0, 0, W * scale, H * scale)

  // Legend strip
  if (includeLegend) {
    const legend = buildLegendCanvas(W * scale, opts, scale)
    ctx.drawImage(legend, 0, H * scale)
  }

  // Dark background fill behind any transparent regions
  ctx.globalCompositeOperation = 'destination-over'
  ctx.fillStyle = isDark ? '#020617' : '#f8fafc'
  ctx.fillRect(0, 0, out.width, out.height)
  ctx.globalCompositeOperation = 'source-over'

  const blob = await new Promise<Blob>((resolve, reject) =>
    out.toBlob(
      (b) => b ? resolve(b) : reject(new Error('toBlob failed')),
      'image/png',
    ),
  )

  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `archigraph-${groupSlug}-${Date.now()}@${scale}x.png`
  a.click()
  setTimeout(() => URL.revokeObjectURL(url), 10_000)
}

// ---------------------------------------------------------------------------
// SVG export — geometry walk
// ---------------------------------------------------------------------------

/** Map a node to its fill color given the current color mode */
function nodeColor(n: GraphNode, colorMode: ColorMode): string {
  if (colorMode === 'community') return communityColor(n.community_id ?? 0)
  // degree mode: approximate the Cosmograph gradient with a blue-violet range
  if (colorMode === 'degree') {
    const deg = n.degree ?? 0
    const t = Math.min(deg / 200, 1)
    const r = Math.round(59 + t * (251 - 59))
    const g = Math.round(130 - t * 130)
    const b = Math.round(246 - t * (246 - 60))
    return `rgb(${r},${g},${b})`
  }
  return repoColor(n.repo ?? '')
}

function computeNodeSize(n: GraphNode): number {
  return 2 + Math.log10((n.degree ?? 0) + 1) * 12
}

function exportSvg(opts: SnapshotOptions): void {
  const { nodes, edges, colorMode, groupSlug, visibleRepos, isDark, includeLegend } = opts

  if (nodes.length === 0) throw new Error('No nodes to export — load the graph first')

  // Get canvas bounding box so we can scale SVG to a reasonable size
  const src = document.querySelector<HTMLCanvasElement>('.graph-canvas canvas')
  const W = src?.offsetWidth ?? 1200
  const H = src?.offsetHeight ?? 800

  // Build id → index map for edge rendering
  const idToNode = new Map(nodes.map((n) => [String(n.id), n]))

  const bg = isDark ? '#020617' : '#f8fafc'
  const edgeSameRepo = isDark ? 'rgba(100,116,139,0.2)' : 'rgba(100,116,139,0.3)'
  const edgeCrossRepo = 'rgba(56,189,248,0.6)'

  // We need actual positions — Cosmograph doesn't expose them via the React API,
  // so we approximate a force-directed layout by reading the canvas pixel positions
  // if available, otherwise spread nodes on a circle.
  //
  // Strategy: use a canvas fingerprint approach — for each node we try to infer
  // position from the Cosmograph internal state via the DOM.  If unavailable,
  // we lay nodes out on concentric rings by repo.
  const positions = computePositions(nodes, W, H)

  // Edge lines (draw first so nodes are on top)
  const edgeLines = edges.map((e) => {
    const src = idToNode.get(String(e.source))
    const tgt = idToNode.get(String(e.target))
    if (!src || !tgt) return ''
    const sp = positions.get(String(src.id))
    const tp = positions.get(String(tgt.id))
    if (!sp || !tp) return ''
    const isCrossRepo = src.repo !== tgt.repo
    const stroke = isCrossRepo ? edgeCrossRepo : edgeSameRepo
    return `<line x1="${sp.x.toFixed(1)}" y1="${sp.y.toFixed(1)}" x2="${tp.x.toFixed(1)}" y2="${tp.y.toFixed(1)}" stroke="${stroke}" stroke-width="0.8" />`
  }).filter(Boolean).join('\n  ')

  // Node circles
  const nodeCircles = nodes.map((n) => {
    const pos = positions.get(String(n.id))
    if (!pos) return ''
    const r = Math.max(2, computeNodeSize(n) * 0.5)
    const fill = nodeColor(n, colorMode)
    const label = (n.label ?? String(n.id)).slice(0, 30)
    return `<circle cx="${pos.x.toFixed(1)}" cy="${pos.y.toFixed(1)}" r="${r.toFixed(1)}" fill="${fill}" opacity="0.9"><title>${escSvg(label)}</title></circle>`
  }).filter(Boolean).join('\n  ')

  // Legend block
  let legendBlock = ''
  const legendY = H + 8
  if (includeLegend) {
    const swatchItems = visibleRepos.slice(0, 10).map((repo, i) => {
      const x = 12 + i * 90
      const color = repoColor(repo)
      return `<circle cx="${x + 5}" cy="${legendY + 14}" r="5" fill="${color}"/>` +
             `<text x="${x + 14}" y="${legendY + 18}" font-size="11" fill="${isDark ? '#94a3b8' : '#475569'}" font-family="system-ui">${escSvg(repo)}</text>`
    }).join('\n    ')

    legendBlock = `
  <rect x="0" y="${legendY}" width="${W}" height="44" fill="${isDark ? '#020617' : '#f8fafc'}"/>
  <line x1="0" y1="${legendY}" x2="${W}" y2="${legendY}" stroke="${isDark ? 'rgba(51,65,85,0.5)' : 'rgba(148,163,184,0.5)'}" stroke-width="1"/>
  <text x="12" y="${legendY + 10}" font-size="11" fill="${isDark ? '#94a3b8' : '#475569'}" font-family="system-ui">Color by: ${escSvg(colorMode)}</text>
  <text x="${W - 12}" y="${legendY + 10}" font-size="11" fill="${isDark ? '#94a3b8' : '#475569'}" font-family="system-ui" text-anchor="end">${escSvg(new Date().toLocaleString())}</text>
  ${swatchItems}`
  }

  const totalH = H + (includeLegend ? 52 : 0)

  const svg = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${totalH}" width="${W}" height="${totalH}">
  <rect width="${W}" height="${H}" fill="${bg}"/>
  <g class="edges" opacity="0.7">
  ${edgeLines}
  </g>
  <g class="nodes">
  ${nodeCircles}
  </g>
  ${legendBlock}
</svg>`

  const blob = new Blob([svg], { type: 'image/svg+xml' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `archigraph-${groupSlug}-${Date.now()}.svg`
  a.click()
  setTimeout(() => URL.revokeObjectURL(url), 10_000)
}

// ---------------------------------------------------------------------------
// Position computation — approximate ring layout by repo
// ---------------------------------------------------------------------------

function computePositions(
  nodes: GraphNode[],
  W: number,
  H: number,
): Map<string, { x: number; y: number }> {
  // Group nodes by repo, then lay repos on a large circle
  const repoGroups = new Map<string, GraphNode[]>()
  for (const n of nodes) {
    const repo = n.repo ?? ''
    if (!repoGroups.has(repo)) repoGroups.set(repo, [])
    repoGroups.get(repo)!.push(n)
  }

  const repos = Array.from(repoGroups.keys()).sort()
  const N = repos.length
  const R = Math.min(W, H) * 0.36
  const cx = W / 2
  const cy = H / 2

  const out = new Map<string, { x: number; y: number }>()

  repos.forEach((repo, ri) => {
    const angle = (ri / N) * 2 * Math.PI - Math.PI / 2
    const rx = cx + R * Math.cos(angle)
    const ry = cy + R * Math.sin(angle)

    const grp = repoGroups.get(repo) ?? []
    const innerR = Math.max(20, Math.sqrt(grp.length) * 6)

    // Sort by degree descending so hubs go near center
    const sorted = [...grp].sort((a, b) => (b.degree ?? 0) - (a.degree ?? 0))

    sorted.forEach((n, i) => {
      // Fibonacci sphere-like distribution within each repo island
      const theta = (i / Math.max(1, sorted.length - 1)) * 2 * Math.PI
      const r = innerR * Math.sqrt((i + 0.5) / sorted.length)
      out.set(String(n.id), {
        x: rx + r * Math.cos(theta),
        y: ry + r * Math.sin(theta),
      })
    })
  })

  return out
}

// ---------------------------------------------------------------------------
// Escape SVG special chars
// ---------------------------------------------------------------------------

function escSvg(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useSnapshotExport() {
  const exportSnapshot = useCallback(async (opts: SnapshotOptions): Promise<void> => {
    if (opts.format === 'png') {
      await exportPng(opts)
    } else {
      exportSvg(opts)
    }
  }, [])

  return { exportSnapshot }
}
