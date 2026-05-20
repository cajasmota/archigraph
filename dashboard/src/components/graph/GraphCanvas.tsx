import { useRef, useEffect, useCallback, memo, useMemo, useState } from 'react'
import { Cosmograph } from '@cosmograph/react'
import type { CosmographRef } from '@cosmograph/react'
import { communityColor } from '@/hooks/graph/useCommunityColors'
import { repoColor } from '@/lib/colors'
import type { GraphNode, GraphEdge } from '@/types/api'
import { useGraphCameraStore } from '@/store/graphCameraStore'

// ---------------------------------------------------------------------------
// Semantic layout helpers (#1072 / #1106 repo-first layout)
// ---------------------------------------------------------------------------

/**
 * Derive a module key from a source_file path.
 * `src/upvate_core/serializers/foo.py` → `upvate_core/serializers`
 * Returns an empty string when source_file is absent.
 */
function moduleKey(sourceFile: string | undefined): string {
  if (!sourceFile) return ''
  const parts = sourceFile.replace(/\\/g, '/').split('/')
  // Drop the filename (last segment); keep up to last 2 directory segments
  const dirs = parts.slice(0, -1)
  return dirs.slice(-2).join('/')
}

/**
 * Stable 16-bit hash of a string.  Produces values in [0, 999].
 */
function hashMod1000(s: string): number {
  let h = 0
  for (let i = 0; i < s.length; i++) {
    h = ((h << 5) - h + s.charCodeAt(i)) | 0
  }
  return Math.abs(h) % 1000
}

/**
 * #1106 — Repo-first composite cluster id:
 *
 *   repoIdx * 10_000_000  +  community_id * 1000  +  moduleHash % 1000
 *
 * This makes REPO the dominant clustering signal so the force simulation
 * pulls same-repo nodes toward their repo's canvas region first, then
 * sub-clusters them by community and module within that region.
 *
 * The multiplier 10_000_000 ensures no aliasing between repos even when
 * community_id values run into the thousands.
 */
function clusterIdFor(n: GraphNode, repoIdx: number): number {
  const mod = hashMod1000(moduleKey(n.source_file))
  const cid = n.community_id ?? 0
  return repoIdx * 10_000_000 + cid * 1000 + mod
}

/**
 * #1106 — Build a repo → canvas-center map.
 *
 * Repos are sorted deterministically (alphabetically) so positions are
 * stable across re-renders. N repos are placed evenly on a large circle
 * whose radius scales with sqrt(nodeCount) so the islands don't overlap.
 *
 * Returns { repoName → {x, y} } for use in initial position seeding.
 */
function buildRepoCenters(
  nodes: GraphNode[],
): Map<string, { x: number; y: number }> {
  const repos = Array.from(new Set(nodes.map((n) => n.repo ?? ''))).sort()
  const N = repos.length
  if (N === 0) return new Map()
  // Scale radius with graph size so regions don't overlap at 20k+ nodes
  const R = Math.max(1500, Math.sqrt(nodes.length) * 30)
  return new Map(
    repos.map((repo, i) => {
      const angle = (i / N) * 2 * Math.PI
      return [repo, { x: R * Math.cos(angle), y: R * Math.sin(angle) }]
    }),
  )
}

export interface GraphCanvasProps {
  nodes: GraphNode[]
  edges: GraphEdge[]
  selectedNodeId: string | null
  hoveredNodeId: string | null
  onNodeClick: (node: GraphNode) => void
  onNodeHover: (node: GraphNode | null) => void
  /** Called when the cursor moves over the canvas — provides screen coords for tooltip */
  onCursorMove?: (x: number, y: number) => void
  /** Called when user clicks empty canvas (no node hit) */
  onEmptyClick?: () => void
  onZoomChange?: (zoom: number) => void
  /** High-contrast mode — wider edges, higher opacity */
  highContrast?: boolean
  /** Current theme — drives canvas background color */
  isDark?: boolean
  /** When true, filter links to cross-repo edges only (#1065) */
  crossRepoOnly?: boolean
  /**
   * When true, the force simulation is paused (nodes frozen).
   * When false/undefined, simulation runs and auto-pauses after first settle.
   * Pass true to resume layout after the user clicks "Resume layout".
   */
  simulationRunning?: boolean
  /** Called when the internal simulation-running state changes (e.g. auto-paused after settle) */
  onSimulationRunningChange?: (running: boolean) => void
  className?: string
  /**
   * #1069: client-side repo filter. When non-null, only nodes whose `repo`
   * is in this set are shown. Non-matching nodes are hidden via Cosmograph
   * selection greyout (pointGreyoutOpacity=0) without triggering a refetch
   * or re-running the force layout.
   *
   * null / undefined = show all repos.
   */
  activeRepos?: Set<string> | null
}

/** Truncate long labels at ~30 chars for layout legibility */
function truncateLabel(text: string): string {
  return text.length > 30 ? text.slice(0, 28) + '…' : text
}

/**
 * GPU-accelerated WebGL force-graph via Cosmograph.
 *
 * Replaces GraphCanvas3D + GraphCanvas2D (#1023).
 * - No LoD: receives pre-filtered nodes + edges from useGraphData
 * - Single canvas, 2D force simulation (60fps at 1M+ nodes)
 * - Drop-in prop interface — route component unchanged
 *
 * Cosmograph data model:
 *   points  = nodes array  (pointIdBy: 'id')
 *   links   = edges array  (linkSourceBy: 'source', linkTargetBy: 'target')
 * Both arrays must be stable references across renders to avoid full rebuilds;
 * we memoise them via useMemo in the calling component (useGraphData already does this).
 *
 * Click handling: Cosmograph provides the point _index_ in onClick.
 * We keep a ref mirror of `nodes` so we can do O(1) lookup without async API calls.
 *
 * Hover-to-focus (#1060): When a node is hovered, selectPoint with connected
 * neighbors is called so non-adjacent nodes are greyed out via Cosmograph's
 * built-in greyout system (pointGreyoutOpacity / linkGreyoutOpacity).
 * unselectAllPoints() restores full opacity on mouse leave.
 */
const GraphCanvasInner = ({
  nodes,
  edges,
  selectedNodeId,
  hoveredNodeId,
  onNodeClick,
  onNodeHover,
  onCursorMove,
  onEmptyClick,
  onZoomChange,
  highContrast = false,
  isDark = true,
  crossRepoOnly = false,
  simulationRunning,
  onSimulationRunningChange,
  className = '',
  activeRepos,
}: GraphCanvasProps) => {
  const cosmographRef = useRef<CosmographRef>(undefined)
  const { setGraphRef, setZoomLevel } = useGraphCameraStore()

  // Track whether the first settle has happened so we only auto-pause once.
  const [hasSettled, setHasSettled] = useState(false)

  // Hard-stop timer ref — cleared on unmount and when sim settles naturally.
  const hardStopTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Mirror of nodes so click handler can resolve index → GraphNode synchronously
  const nodesRef = useRef<GraphNode[]>(nodes)
  nodesRef.current = nodes

  // Debounce timer for hover — prevents thrashing on rapid micro-movements
  const hoverDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Track the last hovered index so we can avoid redundant selectPoint calls
  const lastHoverIndexRef = useRef<number | null>(null)

  // #1069: client-side repo filter — compute numeric indices of visible nodes.
  // When activeRepos is null/undefined, all nodes are visible (selectPoints(null) = clear selection).
  // When activeRepos is a non-empty Set, only nodes in that set are selected so unselected
  // nodes get pointGreyoutOpacity=0 (invisible) without re-uploading data to DuckDB-WASM.
  const repoFilterActive = activeRepos != null
  const visibleIndices = useMemo<number[] | null>(() => {
    if (!activeRepos) return null  // no filter — show all
    return nodes
      .map((n, i) => (activeRepos.has(n.repo) ? i : -1))
      .filter((i) => i !== -1)
  }, [nodes, activeRepos])

  // Keep a ref to latest visibleIndices so handleMount can re-apply after mount
  const visibleIndicesRef = useRef<number[] | null>(visibleIndices)
  visibleIndicesRef.current = visibleIndices

  // Apply the repo filter via Cosmograph's imperative selection API.
  // We do this in a useEffect (not in render) because cosmographRef is populated
  // after mount. The effect runs whenever visibleIndices reference changes.
  useEffect(() => {
    const cosmo = cosmographRef.current
    if (!cosmo) return
    // null → clear selection (show all); array → select visible subset
    cosmo.selectPoints(visibleIndices)
  }, [visibleIndices])

  // Cosmograph requires a sequential numeric index column on both points and links.
  // We derive these from the incoming arrays rather than mutating the originals.
  //
  // #1072 / #1106: add __cluster_id (repo-first composite) and __cluster_strength
  // so the force simulation groups nodes by repo first, then community + module.
  // Repo is the dominant signal: repoIdx * 10_000_000 ensures clear separation.
  //
  // #1089: also compute __size via log scale so degree-750 god nodes are
  // visibly huge relative to degree-1 leaves.
  //   d=0  → 2 px  | d=10  → ~14 px
  //   d=100 → ~26 px | d=750 → ~36 px  (before pointSizeRange remapping)
  const computeSize = (d: number): number =>
    2 + Math.log10(d + 1) * 12

  // #1106: repo → index map (alphabetically sorted, deterministic)
  const repoToIdx = useMemo(() => {
    const repos = Array.from(new Set(nodes.map((n) => n.repo ?? ''))).sort()
    return new Map(repos.map((r, i) => [r, i]))
  }, [nodes])

  // #1106: repo → canvas-center positions placed on a large circle
  const repoCenters = useMemo(() => buildRepoCenters(nodes), [nodes])

  const cosmographPoints = useMemo(() => {
    // Compute per-node max pagerank for normalising cluster strength
    let maxPR = 0
    for (const n of nodes) { if ((n.pagerank ?? 0) > maxPR) maxPR = n.pagerank ?? 0 }
    if (maxPR === 0) maxPR = 1

    return nodes.map((n, i) => {
      const repoIdx = repoToIdx.get(n.repo ?? '') ?? 0
      const cid = clusterIdFor(n, repoIdx)
      // Hub nodes (top ~10% by pagerank) get stronger pull → stay near cluster center
      const normalizedPR = (n.pagerank ?? 0) / maxPR
      // #1106: keep cluster strength low (0.10-0.18) — the pre-positioned initial
      // positions + strong repulsion do the heavy lifting. Too-high strength collapses
      // each island into a single point rather than letting nodes spread naturally.
      const strength = 0.10 + normalizedPR * 0.08
      // #1089: pre-computed log-degree size for pointSizeByFn
      const __size = computeSize(n.degree ?? 0)
      // #1106: seed each node near its repo's canvas center so the sim starts in
      // the right region and converges without fighting a random-soup start state.
      // Small jitter (±50px) prevents all nodes from stacking at the exact center.
      const center = repoCenters.get(n.repo ?? '')
      const __x = center ? center.x + (Math.random() * 100 - 50) : 0
      const __y = center ? center.y + (Math.random() * 100 - 50) : 0
      return { ...n, __idx: i, __cluster_id: cid, __cluster_strength: strength, __size, __x, __y }
    })
  // repoCenters and repoToIdx are derived from nodes — listing all three would
  // fire on every nodes change anyway; suppress exhaustive-deps for the derived maps.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nodes, repoToIdx, repoCenters])

  const cosmographLinks = useMemo(() => {
    const idToIdx = new Map(nodes.map((n, i) => [String(n.id), i]))
    const idToRepo = new Map(nodes.map((n) => [String(n.id), n.repo ?? '']))
    return edges
      .map((e, i) => {
        const srcRepo = idToRepo.get(String(e.source)) ?? ''
        const tgtRepo = idToRepo.get(String(e.target)) ?? ''
        return {
          ...e,
          __idx: i,
          __srcIdx: idToIdx.get(String(e.source)),
          __tgtIdx: idToIdx.get(String(e.target)),
          // 1 = cross-repo, 0 = intra-repo; stored as number for DuckDB-WASM compatibility
          __crossRepo: srcRepo !== tgtRepo ? 1 : 0,
        }
      })
      .filter((e) => e.__srcIdx !== undefined && e.__tgtIdx !== undefined)
  }, [nodes, edges])

  // When crossRepoOnly is active, restrict to cross-repo edges only (#1065 Task 3)
  const visibleLinks = useMemo(
    () => crossRepoOnly ? cosmographLinks.filter((e) => e.__crossRepo === 1) : cosmographLinks,
    [cosmographLinks, crossRepoOnly],
  )

  // Expose a cosmograph-compatible ref to the camera store so
  // resetView / zoomToNode keep working with the new renderer.
  // Also re-apply the repo filter selection immediately on mount so that if
  // activeRepos was set before the canvas mounted, the filter still takes effect.
  const handleMount = useCallback((instance: NonNullable<CosmographRef>) => {
    cosmographRef.current = instance
    // Wrap the Cosmograph instance to match the camera store's ForceGraphInstance duck-type
    setGraphRef(instance as unknown as Parameters<typeof setGraphRef>[0])
    // Re-apply repo filter in case visibleIndices effect fired before mount
    const indices = visibleIndicesRef.current
    if (indices !== null) {
      instance.selectPoints(indices)
    }
  }, [setGraphRef])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      setGraphRef(null)
      if (hoverDebounceRef.current) clearTimeout(hoverDebounceRef.current)
      if (hardStopTimerRef.current) clearTimeout(hardStopTimerRef.current)
    }
  }, [setGraphRef])

  // Shared settle helper: called by onSimulationEnd OR the hard-stop timer,
  // whichever fires first.
  const doSettle = useCallback(() => {
    if (hardStopTimerRef.current) {
      clearTimeout(hardStopTimerRef.current)
      hardStopTimerRef.current = null
    }
    cosmographRef.current?.pause()
    setHasSettled(true)
    onSimulationRunningChange?.(false)
  }, [onSimulationRunningChange])

  // onSimulationEnd fires when Cosmograph's force layout reaches alpha ≈ 0.
  // On first settle we auto-pause and notify the parent so it can show
  // a "Resume layout" button.  After that, parent controls the state.
  const handleSimulationEnd = useCallback(() => {
    if (!hasSettled) {
      doSettle()
    }
  }, [hasSettled, doSettle])

  // Hard-stop: if onSimulationEnd never fires (large graph may never converge)
  // force-pause after 8 seconds. The timer is set on mount and cleared either by
  // doSettle (natural convergence wins) or on unmount.
  useEffect(() => {
    if (hasSettled) return
    hardStopTimerRef.current = window.setTimeout(() => {
      if (!hasSettled) {
        doSettle()
      }
    }, 8000)
    return () => {
      if (hardStopTimerRef.current) {
        clearTimeout(hardStopTimerRef.current)
        hardStopTimerRef.current = null
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])  // intentionally run once on mount; doSettle is stable via useCallback

  // React to parent-controlled simulationRunning changes after first settle.
  useEffect(() => {
    if (!hasSettled) return          // before first settle, Cosmograph owns it
    if (simulationRunning === true) {
      cosmographRef.current?.start()
    } else if (simulationRunning === false) {
      cosmographRef.current?.pause()
    }
  }, [simulationRunning, hasSettled])

  // Point color accessor — receives the value of the `pointColorBy` column ('id'),
  // so `value` here is the node id string.
  const pointColorByFn = useCallback((nodeId: string) => {
    if (nodeId === selectedNodeId) return '#38bdf8'   // sky-400 — selected
    if (nodeId === hoveredNodeId)  return '#e2e8f0'   // slate-200 — hovered
    // Find node to determine color strategy
    const node = nodesRef.current.find((n) => n.id === nodeId)
    if (!node) return '#64748b'
    if (node.is_centroid) return communityColor(node.community_id ?? 0)
    if (selectedNodeId) return repoColor(node.repo) + '66' // dimmed when another selected
    return repoColor(node.repo)
  }, [selectedNodeId, hoveredNodeId])

  // #1059: size nodes by degree (hub nodes appear larger than leaves).
  // CosmographPointSizeStrategy.Degree uses quantile-bounded degree distribution.
  // pointSizeRange controls min/max pixel sizes across the full degree spectrum.

  // Link color accessor — receives the value of the `linkColorBy` column ('__crossRepo').
  // __crossRepo is 1 for cross-repo edges, 0 for intra-repo (#1065).
  // Values come in as number, but DuckDB-WASM may convert them to string — handle both.
  const linkColorByFn = useCallback((crossRepo: unknown) => {
    const isCross = crossRepo === 1 || crossRepo === '1'
    if (isCross) {
      // sky-400 at 70% opacity — bright accent for cross-repo bridges
      return highContrast ? 'rgba(56,189,248,1.0)' : 'rgba(56,189,248,0.7)'
    }
    // slate-500 at reduced opacity for intra-repo
    return highContrast ? 'rgba(100,116,139,0.5)' : 'rgba(100,116,139,0.15)'
  }, [highContrast])

  // Click: Cosmograph provides the point index in the current `nodes` array.
  // index === undefined means click landed on empty canvas (Cosmograph fires onBackgroundClick
  // for that case, but guard here too for belt-and-suspenders).
  const handleClick = useCallback((index: number | undefined) => {
    if (index === undefined) {
      onEmptyClick?.()
      return
    }
    const node = nodesRef.current[index]
    if (!node) return
    // Toggle: clicking the already-selected node deselects it
    if (node.id === selectedNodeId) {
      onEmptyClick?.()
      return
    }
    onNodeClick(node)
  }, [onNodeClick, onEmptyClick, selectedNodeId])

  // Background click: clear hover + greyout + deselect node
  const handleBackgroundClick = useCallback(() => {
    if (hoverDebounceRef.current) clearTimeout(hoverDebounceRef.current)
    lastHoverIndexRef.current = null
    cosmographRef.current?.unselectAllPoints()
    onNodeHover(null)
    onEmptyClick?.()
  }, [onNodeHover, onEmptyClick])

  // Hover: Cosmograph provides the point index on mouse move.
  // Debounced 50 ms to avoid thrashing GPU on rapid micro-movements.
  // When a node is hovered, selectPoint with selectConnectedPoints=true activates
  // Cosmograph's greyout: non-selected/non-adjacent nodes get pointGreyoutOpacity.
  //
  // IMPORTANT: after the sim has settled (hasSettled=true) we re-pause immediately
  // after selectPoint because Cosmograph internally bumps alpha on selection changes,
  // which causes perpetual micro-motion if unchecked (#1071 side-effect).
  const hasSettledRef = useRef(false)
  hasSettledRef.current = hasSettled

  const handleMouseMove = useCallback((index: number | undefined) => {
    if (hoverDebounceRef.current) clearTimeout(hoverDebounceRef.current)

    if (index === undefined) {
      // Schedule clear with slight delay so leaving a node doesn't flicker
      hoverDebounceRef.current = setTimeout(() => {
        lastHoverIndexRef.current = null
        cosmographRef.current?.unselectAllPoints()
        // Re-pause if sim was settled — unselectAllPoints may nudge alpha
        if (hasSettledRef.current) cosmographRef.current?.pause()
        onNodeHover(null)
      }, 50)
      return
    }

    // Same node — no work needed
    if (index === lastHoverIndexRef.current) return

    hoverDebounceRef.current = setTimeout(() => {
      lastHoverIndexRef.current = index
      const node = nodesRef.current[index]
      if (!node) return
      // selectPoint with selectConnectedPoints=true highlights the hovered node
      // and its direct neighbors; all others get greyout opacity applied by Cosmograph.
      cosmographRef.current?.selectPoint(index, false, true)
      // Re-pause immediately so selectPoint's internal alpha bump doesn't restart motion
      if (hasSettledRef.current) cosmographRef.current?.pause()
      onNodeHover(node)
    }, 50)
  }, [onNodeHover])

  // Track raw screen cursor position for tooltip overlay
  const handleWrapperMouseMove = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    onCursorMove?.(e.clientX, e.clientY)
  }, [onCursorMove])

  // Zoom: Cosmograph's onZoom fires with a D3 zoom event; extract the k scale
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const handleZoom = useCallback((e: any) => {
    const k: number = e?.transform?.k ?? 1
    setZoomLevel(k)
    onZoomChange?.(k)
  }, [setZoomLevel, onZoomChange])

  // Theme-aware canvas background:
  //   dark mode  → deep slate-950 (#020617) — same as before, no visual regression
  //   light mode → slate-50 (#f8fafc) — avoids the jarring black-on-light layout
  const canvasBg = isDark ? '#020617' : '#f8fafc'

  // Label pill style — gives a semi-transparent background behind label text so
  // it reads over edges. Uses inline CSS (Cosmograph className prop accepts style strings).
  const labelPillStyle = isDark
    ? 'background: rgba(2,6,23,0.72); color: #e2e8f0; font-weight: 500; padding: 1px 5px; border-radius: 4px; font-size: 11px; white-space: nowrap;'
    : 'background: rgba(248,250,252,0.82); color: #1e293b; font-weight: 500; padding: 1px 5px; border-radius: 4px; font-size: 11px; white-space: nowrap;'

  return (
    <div
      className={['w-full h-full cursor-pointer relative', className].join(' ')}
      aria-label="Dependency graph"
      role="img"
      aria-describedby="graph-canvas-a11y-desc"
      onMouseMove={handleWrapperMouseMove}
    >
      <span id="graph-canvas-a11y-desc" className="sr-only">
        Interactive GPU-accelerated force-directed graph. Use the inspector panel to navigate nodes with keyboard.
      </span>

      <Cosmograph
        ref={cosmographRef}
        style={{ width: '100%', height: '100%' }}
        onMount={handleMount}

        // ── Data ──────────────────────────────────────────────────────────────
        points={cosmographPoints as unknown as Record<string, unknown>[]}
        pointIdBy="id"
        pointIndexBy="__idx"
        // Explicit allowlist guards against any future non-primitive field reaching
        // DuckDB-WASM (nested objects/arrays crash columnar type inference).
        // __idx is included so Cosmograph can resolve its numeric index lookups.
        // #1072: __cluster_id and __cluster_strength added for semantic layout.
        // #1106: __x and __y added for repo-first initial position seeding.
        pointIncludeColumns={['__idx', 'id', 'label', 'kind', 'repo', 'community_id', 'pagerank', 'is_centroid', 'centroid_size', 'source_file', 'start_line', 'degree', '__cluster_id', '__cluster_strength', '__size', '__x', '__y']}

        links={visibleLinks as unknown as Record<string, unknown>[]}
        linkSourceBy="source"
        linkSourceIndexBy="__srcIdx"
        linkTargetBy="target"
        linkTargetIndexBy="__tgtIdx"
        // __crossRepo carries the cross-repo flag for color/width differentiation (#1065)
        linkIncludeColumns={['kind', '__crossRepo']}

        // ── Repo-first semantic layout (#1072 / #1106) ───────────────────────
        // #1106: two-layer approach for guaranteed island separation:
        //
        // 1. pointXBy/pointYBy: SEED each node's initial position near its repo's
        //    canvas center (pre-computed in cosmographPoints as __x/__y).
        //    Nodes start already-separated — the sim doesn't have to fight
        //    through random-soup to discover repo boundaries.
        //    NOTE: when pointXBy/pointYBy are set, clusterPositionsMap has no
        //    effect per Cosmograph docs — so we omit it here.
        //
        // 2. pointClusterBy (repo-first cluster id) + pointClusterStrengthBy:
        //    cluster force keeps nodes near their repo region during the sim.
        //    The composite id encodes repo as the dominant key so the cluster
        //    force always pulls toward the correct repo region.
        pointXBy="__x"
        pointYBy="__y"
        pointClusterBy="__cluster_id"
        pointClusterStrengthBy="__cluster_strength"

        // ── Node appearance ────────────────────────────────────────────────
        pointColorBy="id"
        pointColorByFn={pointColorByFn as (value: unknown) => string}
        // #1089: log-scale sizing so degree-750 god nodes are dramatically larger
        // than degree-1 leaves. pointSizeByFn receives the __size pre-computed value;
        // pointSizeRange [2, 60] remaps the log output to final pixel sizes.
        // log formula: d=0 → 2 px, d=10 → ~14 px, d=100 → ~26 px, d=750 → ~36 px (before remap).
        // After remap the hub (deg=750, raw≈36) lands at ~60 px; leaf (deg=1, raw≈3.6) → ~2 px.
        pointSizeBy="__size"
        // Wider range so leaf nodes stay visible (min 5px) and hub nodes are
        // dramatically larger (max 80px). Fixes uniform-tiny-dot appearance.
        pointSizeRange={[5, 80]}
        pointLabelBy="label"

        // ── Labels ────────────────────────────────────────────────────────
        // #1059: show dynamic + top labels so hubs are named at a glance.
        // showDynamicLabels: evenly distributed visible nodes get labels automatically.
        // showTopLabels: highest-degree nodes always show labels regardless of viewport.
        // Truncate long entity names at 30 chars; pill background for readability.
        // showTopLabels: hub nodes always labelled; showDynamicLabels: evenly distributed.
        showLabels={true}
        showTopLabels={true}
        // #1089: increase to 50 so more hub labels are always visible at a glance.
        // Top nodes are ranked by degree (highest-degree always labelled first).
        showTopLabelsLimit={50}
        showDynamicLabels={true}
        showDynamicLabelsLimit={40}
        showHoveredPointLabel={true}
        showFocusedPointLabel={true}
        pointLabelFontSize={11}
        pointLabelFn={truncateLabel as (value: unknown) => string}
        pointLabelClassName={labelPillStyle}

        // ── Edge appearance ────────────────────────────────────────────────
        // Color driven by __crossRepo: cross-repo = sky-400, intra-repo = slate-500 (#1065)
        linkColorBy="__crossRepo"
        linkColorByFn={linkColorByFn as (value: unknown) => string}
        // Cross-repo edges drawn thicker: range maps to [intra-repo, cross-repo] width
        linkWidthRange={highContrast ? [1, 3] : [0.5, 2]}

        // ── Background ────────────────────────────────────────────────────
        backgroundColor={canvasBg}

        // ── Greyout opacity ────────────────────────────────────────────────
        // #1069: when repo filter is active, opacity=0 makes filtered-out nodes
        // and edges invisible. When no filter is active, hover-focus greyout (#1060)
        // uses 0.15 so non-adjacent nodes dim on hover.
        pointGreyoutOpacity={repoFilterActive ? 0 : 0.15}
        linkGreyoutOpacity={repoFilterActive ? 0 : 0.1}

        // ── Simulation ─────────────────────────────────────────────────────
        enableSimulation={true}
        preservePointPositionsOnDataUpdate={true}
        // #1106: Stay at default friction (0.85) for fast settle.
        simulationFriction={0.85}
        // #1106: Raise repulsion significantly (was 0.8) so nodes from different
        // repos physically repel each other, pushing the 3 repo islands apart.
        // 1.5 is strong but anchored because each node starts near its repo center.
        simulationRepulsion={1.5}
        // #1106: Mild per-node gravity toward origin prevents drift off-canvas.
        // With nodes pre-positioned at R=1500–3000px, 0.1 is gentle enough not
        // to collapse the islands back toward the center.
        simulationGravity={0.1}
        // Tiny center-mass force as a safety net against extreme drift
        simulationCenter={0.05}
        // #1106: Faster decay (was 2000) → simulation settles within the 8s hard-stop.
        simulationDecay={1500}

        // ── Selection / interaction ────────────────────────────────────────
        selectPointOnClick="single"
        focusPointOnClick={false}
        resetSelectionOnEmptyCanvasClick={false}

        // ── Simulation events ──────────────────────────────────────────────
        onSimulationEnd={handleSimulationEnd}

        // ── Events ────────────────────────────────────────────────────────
        onClick={handleClick}
        onBackgroundClick={handleBackgroundClick}
        onMouseMove={handleMouseMove}
        onZoom={handleZoom}

        // Suppress internal status messages — we have our own loading states
        statusIndicatorMode={false}
        disableLogging={true}
      />

      {/* Vignette overlay — radial gradient darker at edges for perceived depth.
          pointer-events:none so it doesn't block canvas interaction. */}
      <div
        aria-hidden
        style={{
          position: 'absolute',
          inset: 0,
          pointerEvents: 'none',
          background: isDark
            ? 'radial-gradient(ellipse at 50% 50%, transparent 55%, rgba(2,6,23,0.55) 100%)'
            : 'radial-gradient(ellipse at 50% 50%, transparent 55%, rgba(226,232,240,0.45) 100%)',
        }}
      />
    </div>
  )
}

export const GraphCanvas = memo(GraphCanvasInner)
