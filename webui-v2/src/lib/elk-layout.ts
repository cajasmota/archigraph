/* ============================================================
   lib/elk-layout.ts — shared elkjs layout helper for React Flow diagrams.

   Foundation of the elkjs adoption epic (#4824/#4825). The dashboard has four
   React-Flow diagrams (compound-topology, iac-diagram, flow-dag, flows) that are
   all NESTED-CONTAINMENT graphs (compound zones / IaC groups). dagre fakes
   compound layout via manual setParent + offset math; elkjs (the Eclipse Layout
   Kernel JS port) has first-class hierarchical layout (`hierarchyHandling:
   INCLUDE_CHILDREN`) and orthogonal edge routing — a far better fit.

   This module is intentionally GENERIC: it knows nothing about topology zones,
   IaC groups, tiers, etc. Consumers pass plain React-Flow nodes/edges (with the
   compound parent/child structure already expressed via `parentId`) plus a few
   layout options, and get back laid-out positions + container sizes. The other
   three diagrams (#4826/#4827/#4828) consume the SAME `layoutWithElk` /
   `useElkLayout` here — do not fork topology-specific logic in.

   Engine note: we import `elkjs/lib/elk.bundled.js` (the self-contained build)
   rather than wiring elk's Web Worker through vite. The bundled build runs
   identically in the browser and under node/vitest, layout() is already async
   (Promise-based) so it does not block paint for our graph sizes, and it bundles
   cleanly with zero vite worker config. The public API here is worker-agnostic,
   so a `new Worker(new URL('elkjs/lib/elk-worker.js'))` backend can be swapped in
   later (epic follow-up) without touching any consumer.
   ============================================================ */

import ELK, {
  type ElkNode,
  type ElkExtendedEdge,
  type LayoutOptions,
} from "elkjs/lib/elk.bundled.js";

/** A single ELK layout engine instance, shared across all callers. */
const elk = new ELK();

/** Minimal shape of the React-Flow nodes we lay out (subset of @xyflow Node). */
export interface ElkLayoutNode {
  id: string;
  /** Compound parent id (React Flow `parentId`). Empty/undefined ⇒ root. */
  parentId?: string;
  /** Measured/initial size. Containers may omit; ELK sizes them from children. */
  width?: number;
  height?: number;
  /**
   * Optional lane/order hint. When `laneOf` is supplied to the layout call,
   * nodes are biased into ordered lanes (e.g. tier columns) along the layout
   * direction. The value is an integer rank (lower ⇒ earlier in the flow).
   */
  lane?: number;
  /** True for container/group nodes (zones, IaC groups). Sized from children. */
  isContainer?: boolean;
}

/** Minimal shape of the React-Flow edges we lay out. */
export interface ElkLayoutEdge {
  id: string;
  source: string;
  target: string;
}

/** Layout result for one node: absolute-within-parent position + final size. */
export interface ElkLayoutPosition {
  id: string;
  /** Position relative to the node's parent (React Flow convention). */
  x: number;
  y: number;
  /** Final size. For containers this is ELK's computed bounding box. */
  width: number;
  height: number;
}

export type ElkDirection = "RIGHT" | "LEFT" | "DOWN" | "UP";

export interface ElkLayoutOptions {
  /** Flow direction. RIGHT = left→right architecture-diagram reading. */
  direction?: ElkDirection;
  /** ELK algorithm. "layered" is the hierarchical default. */
  algorithm?: string;
  /** Edge routing style. */
  edgeRouting?: "ORTHOGONAL" | "POLYLINE" | "SPLINES";
  /** Spacing between sibling nodes in the same layer. */
  nodeSpacing?: number;
  /** Spacing between layers (ranks). */
  layerSpacing?: number;
  /** Inner padding of container nodes: "top,right,bottom,left" (px each). */
  padding?: { top: number; right: number; bottom: number; left: number };
  /** Default size applied to leaf nodes that don't carry width/height. */
  defaultNodeWidth?: number;
  defaultNodeHeight?: number;
  /**
   * Optional tier-lane ordering hint. Given a node id, return its lane rank
   * (lower ⇒ earlier along `direction`). Nodes are pinned into ordered lanes so
   * e.g. client·edge·auth·compute·data·messaging·external read left→right. When
   * omitted, ELK orders nodes purely from edge structure.
   *
   * Lanes are applied per nesting level (only across siblings sharing a parent),
   * so a child zone's internal nodes still order independently.
   */
  laneOf?: (nodeId: string) => number | undefined;
  /** Extra raw ELK layoutOptions, merged last (escape hatch / per-diagram). */
  extraLayoutOptions?: LayoutOptions;
}

const DEFAULTS: Required<
  Omit<ElkLayoutOptions, "laneOf" | "extraLayoutOptions" | "padding">
> & { padding: NonNullable<ElkLayoutOptions["padding"]> } = {
  direction: "RIGHT",
  algorithm: "layered",
  edgeRouting: "ORTHOGONAL",
  nodeSpacing: 28,
  layerSpacing: 72,
  padding: { top: 34, right: 16, bottom: 14, left: 16 },
  defaultNodeWidth: 180,
  defaultNodeHeight: 52,
};

const finite = (v: number | undefined, fallback = 0): number =>
  typeof v === "number" && Number.isFinite(v) ? v : fallback;

/**
 * buildElkTree turns the flat React-Flow node list (with `parentId` links) into
 * the nested ElkNode tree ELK expects. Edges are attached at the lowest common
 * ancestor of their endpoints so cross-container edges route correctly.
 */
function buildElkTree(
  nodes: ElkLayoutNode[],
  edges: ElkLayoutEdge[],
  opts: Required<Pick<ElkLayoutOptions, "defaultNodeWidth" | "defaultNodeHeight">> & {
    laneOf?: ElkLayoutOptions["laneOf"];
  },
): ElkNode {
  const byId = new Map<string, ElkLayoutNode>();
  for (const n of nodes) byId.set(n.id, n);

  // Build empty ElkNode shells, indexed by id.
  const elkById = new Map<string, ElkNode>();
  for (const n of nodes) {
    const isContainer = n.isContainer ?? false;
    const shell: ElkNode = {
      id: n.id,
      children: [],
      edges: [],
      ...(isContainer
        ? {}
        : {
            width: finite(n.width, opts.defaultNodeWidth),
            height: finite(n.height, opts.defaultNodeHeight),
          }),
    };
    // Per-node lane hint → ELK position constraint within its layer.
    const lane = opts.laneOf?.(n.id) ?? n.lane;
    if (typeof lane === "number" && Number.isFinite(lane)) {
      shell.layoutOptions = {
        ...shell.layoutOptions,
        // Pins the node's layer (rank) so lanes stay ordered along direction.
        "elk.layered.layering.layerChoiceConstraint": String(lane),
      };
    }
    elkById.set(n.id, shell);
  }

  const root: ElkNode = { id: "__elk_root__", children: [], edges: [] };

  // Attach each node under its parent (or root).
  for (const n of nodes) {
    const shell = elkById.get(n.id)!;
    const parent = n.parentId ? elkById.get(n.parentId) : undefined;
    (parent?.children ?? root.children!).push(shell);
  }

  // Depth lookup for lowest-common-ancestor edge attachment.
  const parentOf = (id: string): string | undefined => byId.get(id)?.parentId;
  const ancestry = (id: string): string[] => {
    const chain: string[] = [];
    let cur: string | undefined = id;
    const seen = new Set<string>();
    while (cur && !seen.has(cur)) {
      seen.add(cur);
      chain.push(cur);
      cur = parentOf(cur);
    }
    return chain;
  };
  const lca = (a: string, b: string): string | undefined => {
    const aChain = new Set(ancestry(a));
    for (const id of ancestry(b)) if (aChain.has(id)) return id;
    return undefined;
  };

  for (const e of edges) {
    if (!byId.has(e.source) || !byId.has(e.target)) continue;
    const common = lca(e.source, e.target);
    // Attach the edge at the LCA container, or root when they share none / are
    // in different subtrees (cross-container edge).
    let host: ElkNode = root;
    if (common && elkById.has(common) && common !== e.source && common !== e.target) {
      host = elkById.get(common)!;
    } else {
      // Walk up from the source to the first common container, else root.
      const target: ElkExtendedEdge = {
        id: e.id,
        sources: [e.source],
        targets: [e.target],
      };
      root.edges!.push(target);
      continue;
    }
    host.edges!.push({ id: e.id, sources: [e.source], targets: [e.target] });
  }

  return root;
}

function graphLayoutOptions(o: typeof DEFAULTS, extra?: LayoutOptions): LayoutOptions {
  return {
    "elk.algorithm": o.algorithm,
    "elk.direction": o.direction,
    "elk.hierarchyHandling": "INCLUDE_CHILDREN",
    "elk.edgeRouting": o.edgeRouting,
    "elk.layered.spacing.nodeNodeBetweenLayers": String(o.layerSpacing),
    "elk.spacing.nodeNode": String(o.nodeSpacing),
    "elk.padding": `[top=${o.padding.top},left=${o.padding.left},bottom=${o.padding.bottom},right=${o.padding.right}]`,
    "elk.layered.considerModelOrder.strategy": "NODES_AND_EDGES",
    ...extra,
  };
}

/**
 * collectPositions flattens ELK's laid-out tree back into per-node positions.
 * ELK positions children relative to their parent already — which is exactly the
 * React-Flow `parentId` convention — so we return x/y as-is.
 */
function collectPositions(root: ElkNode, out: Map<string, ElkLayoutPosition>): void {
  const visit = (node: ElkNode) => {
    for (const child of node.children ?? []) {
      out.set(child.id, {
        id: child.id,
        x: finite(child.x),
        y: finite(child.y),
        width: finite(child.width, 0),
        height: finite(child.height, 0),
      });
      visit(child);
    }
  };
  visit(root);
}

/**
 * layoutWithElk runs ELK over a compound React-Flow graph and returns a map of
 * node id → laid-out position/size. Positions are PARENT-RELATIVE (React Flow's
 * convention for child nodes), and container sizes are ELK's computed bounding
 * boxes. Consumers map these back onto their own node objects.
 *
 * Async: ELK layout is Promise-based. Call from an effect and store the result.
 *
 * @example
 *   const pos = await layoutWithElk(nodes, edges, {
 *     direction: "RIGHT",
 *     laneOf: (id) => tierRankById.get(id),
 *   });
 *   const placed = nodes.map(n => ({ ...n, position: { x: pos.get(n.id)!.x, y: pos.get(n.id)!.y } }));
 */
export async function layoutWithElk(
  nodes: ElkLayoutNode[],
  edges: ElkLayoutEdge[],
  options: ElkLayoutOptions = {},
): Promise<Map<string, ElkLayoutPosition>> {
  const out = new Map<string, ElkLayoutPosition>();
  if (nodes.length === 0) return out;

  const o = {
    ...DEFAULTS,
    ...options,
    padding: { ...DEFAULTS.padding, ...(options.padding ?? {}) },
  } as typeof DEFAULTS;

  const root = buildElkTree(nodes, edges, {
    defaultNodeWidth: o.defaultNodeWidth,
    defaultNodeHeight: o.defaultNodeHeight,
    laneOf: options.laneOf,
  });
  root.layoutOptions = graphLayoutOptions(o, options.extraLayoutOptions);

  const laid = await elk.layout(root);
  collectPositions(laid, out);
  return out;
}
