/* ============================================================
   components/graph/module-overview.tsx — the COLLAPSED module-level
   canvas (#1386, closes epic #1380 alongside #1384).

   At module scope a corpus is small (12 modules on acme, ~30 on
   polyglot per repo) so we skip cosmos.gl entirely and lay the graph
   out in SVG. The render is deterministic + screenshot-friendly:

     • Force-directed simulation runs synchronously for ~200 ticks on
       mount, then freezes — no rAF loop, no GPU pressure. The graph
       is small enough that this finishes in <5 ms even on slow
       machines, and the result is identical across reloads (we seed
       node positions on a circle keyed by sorted module ID).
     • Node radius scales with PageRank (sqrt of count-mass + a base).
     • Node color tints SCC members so the "cycle group" reads at a
       glance; non-cycle modules use a per-repo pastel.
     • Edge width scales with aggregated weight; cycle-internal edges
       (SCCInternal=true) are tinted with the SCC color.
     • One click selects a module (shows the right-hand metrics card);
       a second click on the SAME module expands it back into the
       entity-level view, filtered to that module's entities.

   This component is dependency-free apart from the v2 design tokens
   (CSS vars on :root) and the existing graph color helpers, so it
   stays light to maintain and easy to screenshot in E2E.
   ============================================================ */

import { useEffect, useMemo, useRef, useState, useCallback } from "react";
import type {
  ModuleAnalysisResponse,
  ModuleAnalysisRepoWire,
  ModuleCentralityWire,
  ModuleEdgeWire,
} from "@/data/types";

/* ─────────────────────────────────────────────────────────────────
   Internal node + layout types.
   ───────────────────────────────────────────────────────────────── */

interface OverviewNode {
  /** Repo-prefixed module ID — globally unique. */
  id: string;
  repo: string;
  /** Module display name (e.g. "core/views"). */
  name: string;
  pagerank: number;
  betweenness: number;
  inDegree: number;
  outDegree: number;
  inCycle: boolean;
  /** SCC ID (≥0 when in a cycle, -1 otherwise). */
  sccId: number;
  /** Live simulation position (mutated in-place by the force loop). */
  x: number;
  y: number;
  /** Velocity components for the simple Verlet step. */
  vx: number;
  vy: number;
  /** Settled radius in viewbox units. */
  radius: number;
}

interface OverviewEdge {
  from: string;
  to: string;
  weight: number;
  sccInternal: boolean;
  sccId: number;
}

/* ─────────────────────────────────────────────────────────────────
   Layout: synchronous force-directed simulation in SVG units.
   ───────────────────────────────────────────────────────────────── */

const VIEWBOX = 1000;
const PADDING = 60;
const SIM_TICKS = 220;

/** Stable hash for ring-seeding so we never spawn nodes at the exact
 *  same coordinate (which produces an Infinity force vector). */
function seedHash(s: string): number {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return (h >>> 0) / 0x100000000;
}

/** Build sized nodes + edges from a single repo's analysis payload. */
function buildOverview(
  repo: ModuleAnalysisRepoWire,
  pageRankBySCC: Map<number, number>,
): { nodes: OverviewNode[]; edges: OverviewEdge[] } {
  // Map module_id → centrality for fast O(1) lookup while building edges.
  const cents = new Map<string, ModuleCentralityWire>();
  for (const c of repo.modules) cents.set(c.module_id, c);

  // PageRank-based radius: sqrt(pr * 4000) + 14, clamped to [14, 60].
  const nodes: OverviewNode[] = repo.modules.map((c) => {
    const seed = seedHash(c.module_id);
    const ringR = 0.32 * VIEWBOX;
    const cx = VIEWBOX / 2;
    const cy = VIEWBOX / 2;
    const idx = Array.from(cents.keys()).indexOf(c.module_id);
    const total = repo.modules.length || 1;
    const ang = (idx / total) * Math.PI * 2 + seed * 0.5;
    const radius = Math.min(
      60,
      Math.max(14, Math.sqrt(Math.max(c.pagerank, 0.001) * 4000) + 14),
    );
    return {
      id: c.module_id,
      repo: repo.repo,
      name: c.module_name,
      pagerank: c.pagerank,
      betweenness: c.betweenness,
      inDegree: c.in_degree,
      outDegree: c.out_degree,
      inCycle: c.in_cycle,
      sccId: c.in_cycle ? findSCC(repo, c.module_id) : -1,
      x: cx + ringR * Math.cos(ang),
      y: cy + ringR * Math.sin(ang),
      vx: 0,
      vy: 0,
      radius,
    };
  });

  const edges: OverviewEdge[] = repo.edges.map((e: ModuleEdgeWire) => ({
    from: e.from_module,
    to: e.to_module,
    weight: e.weight,
    sccInternal: e.scc_internal,
    sccId: e.scc_id,
  }));

  // Use pageRankBySCC to surface the dominant SCC visually (no-op when
  // there are no SCCs — present for symmetry with the legend code).
  void pageRankBySCC;

  return { nodes, edges };
}

/** Find which SCC a module belongs to by scanning the SCCs list. */
function findSCC(repo: ModuleAnalysisRepoWire, moduleId: string): number {
  // The daemon serializes empty slices as JSON `null` (Go zero-value), so
  // defensively coalesce to [] before iterating — never trust the wire
  // shape's "non-empty array" guarantee.
  for (const s of repo.sccs ?? []) {
    if (s.members.includes(moduleId)) return s.id;
  }
  return -1;
}

/** Run SIM_TICKS of a tiny force simulation in-place on `nodes`. */
function runSimulation(nodes: OverviewNode[], edges: OverviewEdge[]) {
  if (nodes.length === 0) return;
  const cx = VIEWBOX / 2;
  const cy = VIEWBOX / 2;
  const byId = new Map(nodes.map((n) => [n.id, n] as const));

  // Repulsion strength scales DOWN with node count so big graphs don't fly
  // apart and small graphs (12 nodes) don't collapse into the center.
  const k = Math.max(900, 14000 / Math.sqrt(nodes.length));
  const springK = 0.012;
  const damping = 0.82;
  const centerPull = 0.0035;

  for (let tick = 0; tick < SIM_TICKS; tick++) {
    // O(n²) repulsion — fine at n≤80.
    for (let i = 0; i < nodes.length; i++) {
      const a = nodes[i];
      for (let j = i + 1; j < nodes.length; j++) {
        const b = nodes[j];
        const dx = a.x - b.x;
        const dy = a.y - b.y;
        const distSq = Math.max(dx * dx + dy * dy, 25);
        const force = k / distSq;
        const inv = 1 / Math.sqrt(distSq);
        const fx = dx * inv * force;
        const fy = dy * inv * force;
        a.vx += fx;
        a.vy += fy;
        b.vx -= fx;
        b.vy -= fy;
      }
    }

    // Spring along edges. Resting length scales gently with weight so heavy
    // edges pull harder (but never collapse the two endpoints onto each other).
    for (const e of edges) {
      const a = byId.get(e.from);
      const b = byId.get(e.to);
      if (!a || !b) continue;
      const rest = 120 + 40 / Math.max(e.weight, 1);
      const dx = b.x - a.x;
      const dy = b.y - a.y;
      const dist = Math.sqrt(dx * dx + dy * dy) || 1;
      const stretch = (dist - rest) * springK;
      const fx = (dx / dist) * stretch;
      const fy = (dy / dist) * stretch;
      a.vx += fx;
      a.vy += fy;
      b.vx -= fx;
      b.vy -= fy;
    }

    // Center pull + damping integration.
    for (const n of nodes) {
      n.vx += (cx - n.x) * centerPull;
      n.vy += (cy - n.y) * centerPull;
      n.vx *= damping;
      n.vy *= damping;
      n.x += n.vx;
      n.y += n.vy;
      // Soft viewbox clamp (keep nodes visible).
      n.x = Math.max(PADDING, Math.min(VIEWBOX - PADDING, n.x));
      n.y = Math.max(PADDING, Math.min(VIEWBOX - PADDING, n.y));
    }
  }
}

/* ─────────────────────────────────────────────────────────────────
   Color palette for SCC groups — deterministic by sccId.
   ───────────────────────────────────────────────────────────────── */

const SCC_COLORS = [
  "#f97316", // orange
  "#ec4899", // pink
  "#a855f7", // purple
  "#3b82f6", // blue
  "#10b981", // emerald
  "#eab308", // yellow
  "#ef4444", // red
  "#06b6d4", // cyan
];

function sccColor(sccId: number): string {
  if (sccId < 0) return "var(--color-accent, #38bdf8)";
  return SCC_COLORS[sccId % SCC_COLORS.length];
}

/* ─────────────────────────────────────────────────────────────────
   The component.
   ───────────────────────────────────────────────────────────────── */

export interface ModuleOverviewProps {
  /** Wire payload from useModuleAnalysis. */
  data: ModuleAnalysisResponse;
  /** Optional repo slug filter — when set, only this repo's modules render. */
  activeRepo?: string | null;
  /** When the user expands a module, the entity-level view should take over. */
  onExpandModule: (m: { repo: string; moduleName: string }) => void;
  /** Theme — affects label + edge colors. */
  isDark: boolean;
}

interface SelectedModule {
  repo: string;
  node: OverviewNode;
}

export function ModuleOverview({
  data,
  activeRepo,
  onExpandModule,
  isDark,
}: ModuleOverviewProps) {
  // Pick which repo(s) to render. Default: the first one with modules. If the
  // graph store has an activeRepo filter, honor it. Defensive `?? []` on the
  // wire slices — the Go daemon serializes empty slices as JSON null.
  const repos = useMemo(() => {
    const withModules = (data.repos ?? [])
      .map((r) => ({
        ...r,
        modules: r.modules ?? [],
        edges: r.edges ?? [],
        sccs: r.sccs ?? [],
        top_pagerank: r.top_pagerank ?? [],
        top_betweenness: r.top_betweenness ?? [],
      }))
      .filter((r) => r.modules.length > 0);
    if (activeRepo) {
      const m = withModules.find((r) => r.repo === activeRepo);
      return m ? [m] : withModules;
    }
    return withModules;
  }, [data, activeRepo]);

  const [activeRepoSlug, setActiveRepoSlug] = useState<string | null>(
    repos[0]?.repo ?? null,
  );

  // When the repos list changes (e.g. activeRepo prop), reset selection.
  useEffect(() => {
    if (!activeRepoSlug || !repos.find((r) => r.repo === activeRepoSlug)) {
      setActiveRepoSlug(repos[0]?.repo ?? null);
    }
  }, [repos, activeRepoSlug]);

  const repo = useMemo(
    () => repos.find((r) => r.repo === activeRepoSlug) ?? repos[0] ?? null,
    [repos, activeRepoSlug],
  );

  // Synchronous layout: build the node/edge sets + run the simulation once
  // per repo change. Memoized so re-renders for hover/select don't re-layout.
  const layout = useMemo(() => {
    if (!repo) return { nodes: [], edges: [] };
    const built = buildOverview(repo, new Map());
    runSimulation(built.nodes, built.edges);
    return built;
  }, [repo]);

  const [selected, setSelected] = useState<SelectedModule | null>(null);
  const [hovered, setHovered] = useState<string | null>(null);

  // Reset selection when repo changes.
  useEffect(() => {
    setSelected(null);
    setHovered(null);
  }, [repo?.repo]);

  const onNodeClick = useCallback(
    (node: OverviewNode) => {
      // First click = select (show side card). Second click on SAME node =
      // expand back into the entity-level view filtered to this module.
      if (selected?.node.id === node.id) {
        onExpandModule({ repo: node.repo, moduleName: node.name });
        return;
      }
      setSelected({ repo: node.repo, node });
    },
    [selected, onExpandModule],
  );

  // Click-on-background clears selection.
  const svgRef = useRef<SVGSVGElement | null>(null);

  /* ───────────────────────────────────────────────────────────────
     Zoom / pan. The main entity graph (graph-canvas.tsx) pans + wheel-
     zooms; this hand-rolled SVG had neither. We manage a {k, tx, ty}
     viewport transform and apply it to a single wrapping <g>, so node
     hit-testing stays correct at any zoom (the browser maps pointer
     events through the same transform automatically).

     Click-vs-drag separation: a pointerdown on empty canvas starts a
     potential pan; we only treat it as a pan once the pointer moves past
     DRAG_THRESHOLD px. If it never crosses the threshold, the trailing
     `click` fires normally (select / clear). A node's own click handler
     calls stopPropagation, so node interactions are unaffected by pan.
     ─────────────────────────────────────────────────────────────── */
  const MIN_K = 0.2;
  const MAX_K = 8;
  const DRAG_THRESHOLD = 4; // px in screen space before a press becomes a pan

  const [view, setView] = useState({ k: 1, tx: 0, ty: 0 });
  // Reset the viewport when the rendered repo changes (fresh layout).
  useEffect(() => {
    setView({ k: 1, tx: 0, ty: 0 });
  }, [repo?.repo]);

  // Pan gesture state (refs — never needs to trigger a re-render itself).
  const panState = useRef<{
    pointerId: number;
    startX: number;
    startY: number;
    startTx: number;
    startTy: number;
    moved: boolean;
  } | null>(null);
  // True only while an actual pan is in progress — used to suppress the
  // trailing background `click` that would otherwise clear the selection.
  const didPanRef = useRef(false);

  /** Convert a clientX/clientY into viewBox (pre-transform) coordinates. */
  const clientToViewBox = useCallback((clientX: number, clientY: number) => {
    const svg = svgRef.current;
    if (!svg) return { x: 0, y: 0 };
    const rect = svg.getBoundingClientRect();
    // The SVG uses preserveAspectRatio="xMidYMid meet", so the viewBox is
    // letterboxed inside the element. Recover the actual rendered scale +
    // offset of the VIEWBOX×VIEWBOX square within the element rect.
    const scale = Math.min(rect.width / VIEWBOX, rect.height / VIEWBOX);
    const drawnW = VIEWBOX * scale;
    const drawnH = VIEWBOX * scale;
    const offX = (rect.width - drawnW) / 2;
    const offY = (rect.height - drawnH) / 2;
    const px = (clientX - rect.left - offX) / scale;
    const py = (clientY - rect.top - offY) / scale;
    return { x: px, y: py };
  }, []);

  const onWheel = useCallback(
    (e: React.WheelEvent<SVGSVGElement>) => {
      e.preventDefault();
      const { x: vx, y: vy } = clientToViewBox(e.clientX, e.clientY);
      setView((v) => {
        const factor = Math.exp(-e.deltaY * 0.0015);
        const nk = Math.min(MAX_K, Math.max(MIN_K, v.k * factor));
        if (nk === v.k) return v;
        // Keep the point under the cursor stationary: solve for tx,ty such
        // that (vx,vy) maps to the same screen position before/after.
        const tx = vx - ((vx - v.tx) / v.k) * nk;
        const ty = vy - ((vy - v.ty) / v.k) * nk;
        return { k: nk, tx, ty };
      });
    },
    [clientToViewBox],
  );

  const onPointerDown = useCallback(
    (e: React.PointerEvent<SVGSVGElement>) => {
      // Only start a pan from a press on the empty canvas (the bare <svg> or
      // the viewport <g>), never from a node — nodes stopPropagation anyway,
      // but guard on the primary button + non-node target for safety.
      if (e.button !== 0) return;
      didPanRef.current = false;
      panState.current = {
        pointerId: e.pointerId,
        startX: e.clientX,
        startY: e.clientY,
        startTx: view.tx,
        startTy: view.ty,
        moved: false,
      };
    },
    [view.tx, view.ty],
  );

  const onPointerMove = useCallback((e: React.PointerEvent<SVGSVGElement>) => {
    const ps = panState.current;
    if (!ps || ps.pointerId !== e.pointerId) return;
    const dxScreen = e.clientX - ps.startX;
    const dyScreen = e.clientY - ps.startY;
    if (
      !ps.moved &&
      Math.hypot(dxScreen, dyScreen) < DRAG_THRESHOLD
    ) {
      return; // below threshold — still a potential click, not a pan
    }
    if (!ps.moved) {
      ps.moved = true;
      didPanRef.current = true;
      svgRef.current?.setPointerCapture(e.pointerId);
    }
    // Convert the screen delta into viewBox units (account for letterboxing).
    const svg = svgRef.current;
    const rect = svg?.getBoundingClientRect();
    const scale = rect
      ? Math.min(rect.width / VIEWBOX, rect.height / VIEWBOX)
      : 1;
    setView((v) => ({
      ...v,
      tx: ps.startTx + dxScreen / scale,
      ty: ps.startTy + dyScreen / scale,
    }));
  }, []);

  const endPan = useCallback((e: React.PointerEvent<SVGSVGElement>) => {
    const ps = panState.current;
    if (ps && ps.pointerId === e.pointerId) {
      if (svgRef.current?.hasPointerCapture?.(e.pointerId)) {
        svgRef.current.releasePointerCapture(e.pointerId);
      }
      panState.current = null;
    }
  }, []);

  const resetView = useCallback(() => setView({ k: 1, tx: 0, ty: 0 }), []);

  if (!repo) {
    return (
      <div className="grid h-full place-items-center bg-bg">
        <div className="text-center">
          <p className="text-md text-text">No modules in this group.</p>
          <p className="mt-1 text-sm text-text-3">
            Module-level analysis requires Module entities (re-index the repo with
            the module aggregator enabled).
          </p>
        </div>
      </div>
    );
  }

  // Build the edge-render order: cycle edges drawn LAST so they sit on top.
  const sortedEdges = useMemo(() => {
    return [...layout.edges].sort((a, b) =>
      Number(a.sccInternal) - Number(b.sccInternal),
    );
  }, [layout.edges]);

  // Min/max weight for edge stroke-width scaling.
  const maxWeight = Math.max(1, ...layout.edges.map((e) => e.weight));

  const nodeById = useMemo(
    () => new Map(layout.nodes.map((n) => [n.id, n] as const)),
    [layout.nodes],
  );

  const edgeColor = isDark ? "rgba(148, 163, 184, 0.45)" : "rgba(71, 85, 105, 0.45)";
  const labelColor = isDark ? "#e2e8f0" : "#0f172a";

  return (
    <div className="relative flex h-full w-full">
      {/* Repo tabs — when the group has more than one repo with modules. */}
      {repos.length > 1 ? (
        <div className="pointer-events-auto absolute left-3 top-3 z-20 flex gap-1 rounded-md border border-border bg-surface/85 p-1 backdrop-blur-sm">
          {repos.map((r) => (
            <button
              key={r.repo}
              onClick={() => setActiveRepoSlug(r.repo)}
              aria-pressed={r.repo === repo.repo}
              className={`rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                r.repo === repo.repo
                  ? "bg-accent-soft text-accent-strong"
                  : "text-text-2 hover:bg-surface-2"
              }`}
            >
              {r.repo}{" "}
              <span className="text-text-4 tabular-nums">({r.num_modules})</span>
            </button>
          ))}
        </div>
      ) : null}

      {/* Top-right stats card — corpus-level summary from #1384. */}
      <div className="pointer-events-none absolute right-3 top-3 z-20 rounded-md border border-border bg-surface/85 px-3 py-2 text-xs text-text-2 backdrop-blur-sm">
        <div className="flex items-center gap-3 tabular-nums">
          <Stat label="modules" value={repo.num_modules} />
          <span className="h-3 w-px bg-border" />
          <Stat label="edges" value={repo.num_module_edges} />
          <span className="h-3 w-px bg-border" />
          <Stat
            label="cycles"
            value={repo.num_sccs}
            tone={repo.num_sccs > 0 ? "warn" : undefined}
          />
          {repo.largest_scc_size > 0 ? (
            <>
              <span className="h-3 w-px bg-border" />
              <Stat label="largest SCC" value={repo.largest_scc_size} tone="warn" />
            </>
          ) : null}
        </div>
      </div>

      {/* The canvas. */}
      <svg
        ref={svgRef}
        viewBox={`0 0 ${VIEWBOX} ${VIEWBOX}`}
        preserveAspectRatio="xMidYMid meet"
        className="h-full w-full touch-none"
        style={{ cursor: panState.current?.moved ? "grabbing" : "grab" }}
        onClick={(e) => {
          // A pan that crossed the threshold should not clear the selection.
          if (didPanRef.current) {
            didPanRef.current = false;
            return;
          }
          if (e.target === svgRef.current) setSelected(null);
        }}
        onWheel={onWheel}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endPan}
        onPointerCancel={endPan}
        role="img"
        aria-label={`Module overview for ${repo.repo}: ${repo.num_modules} modules, ${repo.num_module_edges} edges, ${repo.num_sccs} cycles`}
        data-testid="module-overview-svg"
      >
        <defs>
          {/* Arrowhead per SCC color group + the default. */}
          {SCC_COLORS.map((c, i) => (
            <marker
              key={i}
              id={`mod-arrow-scc-${i}`}
              viewBox="0 0 10 10"
              refX="9"
              refY="5"
              markerWidth="5"
              markerHeight="5"
              orient="auto-start-reverse"
            >
              <path d="M 0 0 L 10 5 L 0 10 z" fill={c} />
            </marker>
          ))}
          <marker
            id="mod-arrow-default"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="5"
            markerHeight="5"
            orient="auto-start-reverse"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill={edgeColor} />
          </marker>
        </defs>

        {/* Viewport — the single <g> that the zoom/pan transform applies to.
            Wrapping ALL content here keeps node hit-testing correct at any
            zoom: the browser maps pointer events through this transform. */}
        <g transform={`translate(${view.tx}, ${view.ty}) scale(${view.k})`}>
        {/* Edges. */}
        <g>
          {sortedEdges.map((e) => {
            const a = nodeById.get(e.from);
            const b = nodeById.get(e.to);
            if (!a || !b) return null;
            const dx = b.x - a.x;
            const dy = b.y - a.y;
            const len = Math.sqrt(dx * dx + dy * dy) || 1;
            // Trim the line endpoints to the circle radius so the arrowhead
            // lands on the node edge, not inside it.
            const ax = a.x + (dx / len) * a.radius;
            const ay = a.y + (dy / len) * a.radius;
            const bx = b.x - (dx / len) * (b.radius + 4);
            const by = b.y - (dy / len) * (b.radius + 4);
            const isCycle = e.sccInternal;
            const stroke = isCycle ? sccColor(e.sccId) : edgeColor;
            const width = 0.8 + (e.weight / maxWeight) * 3;
            const opacity = isCycle ? 0.95 : 0.6;
            const markerId = isCycle
              ? `mod-arrow-scc-${e.sccId % SCC_COLORS.length}`
              : "mod-arrow-default";
            return (
              <line
                key={`${e.from}->${e.to}`}
                x1={ax}
                y1={ay}
                x2={bx}
                y2={by}
                stroke={stroke}
                strokeWidth={width}
                strokeOpacity={opacity}
                markerEnd={`url(#${markerId})`}
              />
            );
          })}
        </g>

        {/* Nodes. */}
        <g>
          {layout.nodes.map((n) => {
            const isSelected = selected?.node.id === n.id;
            const isHovered = hovered === n.id;
            const fill = sccColor(n.sccId);
            const stroke = isSelected
              ? "var(--color-accent, #38bdf8)"
              : n.inCycle
                ? fill
                : isDark
                  ? "#475569"
                  : "#cbd5e1";
            const strokeWidth = isSelected ? 3 : n.inCycle ? 2 : 1.2;
            const opacity = isHovered || isSelected ? 1 : n.inCycle ? 0.95 : 0.78;
            return (
              <g
                key={n.id}
                transform={`translate(${n.x}, ${n.y})`}
                onClick={(e) => {
                  e.stopPropagation();
                  onNodeClick(n);
                }}
                onMouseEnter={() => setHovered(n.id)}
                onMouseLeave={() => setHovered((cur) => (cur === n.id ? null : cur))}
                style={{ cursor: "pointer" }}
                role="button"
                aria-label={`Module ${n.name}, PageRank ${n.pagerank.toFixed(3)}${
                  n.inCycle ? ", in cycle" : ""
                }`}
              >
                <circle
                  r={n.radius}
                  fill={n.inCycle ? fill : isDark ? "#1e293b" : "#f1f5f9"}
                  stroke={stroke}
                  strokeWidth={strokeWidth}
                  fillOpacity={n.inCycle ? 0.35 : 1}
                  opacity={opacity}
                />
                {/* Label inside / below the node. */}
                <text
                  textAnchor="middle"
                  dy={n.radius < 22 ? n.radius + 12 : 4}
                  fill={labelColor}
                  fontSize={n.radius < 22 ? 11 : 12}
                  fontFamily="ui-sans-serif, system-ui, -apple-system, sans-serif"
                  fontWeight={isSelected ? 600 : 500}
                  style={{ pointerEvents: "none", userSelect: "none" }}
                >
                  {truncate(n.name, n.radius < 22 ? 18 : 14)}
                </text>
              </g>
            );
          })}
        </g>
        </g>
      </svg>

      {/* Side panel — module metrics + expand action. */}
      {selected ? (
        <ModuleDetailsCard
          repo={selected.repo}
          node={selected.node}
          onClose={() => setSelected(null)}
          onExpand={() =>
            onExpandModule({
              repo: selected.node.repo,
              moduleName: selected.node.name,
            })
          }
          sccs={repo.sccs}
        />
      ) : null}

      {/* Legend — bottom-left, shows cycle groups + a render hint. */}
      <div className="pointer-events-none absolute bottom-3 left-3 z-20 rounded-md border border-border bg-surface/85 px-3 py-2 text-xs text-text-3 backdrop-blur-sm">
        <div className="flex items-center gap-3">
          <span className="font-medium text-text-2">Module overview</span>
          <span className="h-3 w-px bg-border" />
          <span>{layout.nodes.length} modules</span>
          {(repo.sccs ?? []).length > 0 ? (
            <>
              <span className="h-3 w-px bg-border" />
              <span className="inline-flex items-center gap-1.5">
                {(repo.sccs ?? []).slice(0, 4).map((s) => (
                  <span
                    key={s.id}
                    className="inline-flex items-center gap-1"
                    title={`SCC ${s.id} — ${s.size} modules`}
                  >
                    <span
                      className="inline-block h-2 w-2 rounded-full"
                      style={{ background: sccColor(s.id) }}
                    />
                    cycle ×{s.size}
                  </span>
                ))}
              </span>
            </>
          ) : null}
          <span className="h-3 w-px bg-border" />
          <span className="text-text-4">scroll → zoom · drag → pan</span>
          {view.k !== 1 || view.tx !== 0 || view.ty !== 0 ? (
            <>
              <span className="h-3 w-px bg-border" />
              <button
                onClick={resetView}
                className="pointer-events-auto rounded px-1.5 py-0.5 text-text-2 hover:bg-surface-2"
                data-testid="module-overview-reset-view"
              >
                reset view
              </button>
            </>
          ) : null}
        </div>
      </div>
    </div>
  );
}

/* ─────────────────────────────────────────────────────────────────
   Side card — metric breakdown for the selected module.
   ───────────────────────────────────────────────────────────────── */

interface ModuleDetailsCardProps {
  repo: string;
  node: OverviewNode;
  onClose: () => void;
  onExpand: () => void;
  sccs: ModuleAnalysisRepoWire["sccs"];
}

function ModuleDetailsCard({
  repo,
  node,
  onClose,
  onExpand,
  sccs,
}: ModuleDetailsCardProps) {
  const scc = node.inCycle ? sccs.find((s) => s.members.includes(node.id)) : null;
  return (
    <aside
      className="absolute right-3 bottom-3 top-16 z-30 flex w-80 flex-col overflow-hidden rounded-lg border border-border bg-surface shadow-lg"
      data-testid="module-overview-card"
    >
      <header className="flex items-start justify-between gap-2 border-b border-border px-3 py-2">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-text" title={node.name}>
            {node.name}
          </div>
          <div className="truncate text-xs text-text-3">{repo}</div>
        </div>
        <button
          onClick={onClose}
          aria-label="Close"
          className="rounded p-1 text-text-3 hover:bg-surface-2"
        >
          ×
        </button>
      </header>
      <div className="flex-1 overflow-y-auto px-3 py-2 text-sm">
        <Row label="PageRank" value={node.pagerank.toFixed(4)} mono />
        <Row label="Betweenness" value={node.betweenness.toFixed(4)} mono />
        <Row label="In-degree" value={String(node.inDegree)} mono />
        <Row label="Out-degree" value={String(node.outDegree)} mono />
        {node.inCycle && scc ? (
          <div className="mt-3 rounded border border-amber-500/30 bg-amber-500/10 px-2 py-1.5">
            <div className="text-xs font-medium text-amber-700 dark:text-amber-300">
              In dependency cycle · SCC #{scc.id}
            </div>
            <div className="mt-1 text-xs text-text-2">
              {scc.size} modules · {scc.edges.length} internal edges
            </div>
            <ul className="mt-1.5 max-h-32 space-y-0.5 overflow-y-auto text-xs text-text-3">
              {scc.member_names.map((m) => (
                <li key={m} className="truncate" title={m}>
                  · {m}
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>
      <footer className="border-t border-border px-3 py-2">
        <button
          onClick={onExpand}
          className="w-full rounded bg-accent px-3 py-1.5 text-sm font-medium text-white hover:bg-accent-strong"
          data-testid="module-overview-expand"
        >
          Expand into entity view
        </button>
        <p className="mt-1 text-center text-[10px] text-text-4">
          tip: double-click a module to expand
        </p>
      </footer>
    </aside>
  );
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between py-0.5">
      <span className="text-xs text-text-3">{label}</span>
      <span
        className={`text-xs text-text-2 ${mono ? "font-mono tabular-nums" : ""}`}
      >
        {value}
      </span>
    </div>
  );
}

function Stat({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "warn";
}) {
  return (
    <span className="inline-flex items-center gap-1">
      <span
        className={`font-semibold ${
          tone === "warn"
            ? "text-amber-600 dark:text-amber-400"
            : "text-text"
        }`}
      >
        {value.toLocaleString()}
      </span>
      <span className="text-text-3">{label}</span>
    </span>
  );
}

function truncate(s: string, n: number): string {
  if (s.length <= n) return s;
  return s.slice(0, Math.max(1, n - 1)) + "…";
}
