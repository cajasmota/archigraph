/* ============================================================
   lib/ego-bfs.ts — undirected N-hop ego-subgraph BFS for the Graph screen.

   #4857 — the click-to-focus ego view must reach a node's INBOUND neighbors,
   not just outbound ones. HTTP endpoint nodes (http_endpoint_definition) are
   pure SINKS: a handler `IMPLEMENTS → endpoint` and a module `CONTAINS →
   endpoint`, while the endpoint itself emits no edges. A directional
   (from → to) BFS from such a sink therefore finds ZERO neighbors at any hop
   count, so the focus view shows just the single node even though the entity
   API reports neighbors.

   The fix: traverse edges UNDIRECTED. We build the adjacency from BOTH
   `from → to` AND `to → from` so an ego BFS from a sink reaches its inbound
   handler/module.

   This lives in its own module (pure, dependency-free) so the BFS is unit
   testable without mounting the React route.
   ============================================================ */

/** Minimal edge shape the BFS needs — a directed (source → target) pair. */
export interface EgoEdge {
  source: string;
  target: string;
}

/**
 * Build an UNDIRECTED adjacency map from a directed edge list.
 *
 * Every edge `a → b` contributes both `a ↔ b` so the ego BFS reaches inbound
 * neighbors. Optionally restrict the adjacency to a `nodeIds` allow-set: edges
 * whose endpoints are not both in the set are skipped, so the ego subgraph
 * never reaches a node that is not in the rendered graph (e.g. a hub-pruned or
 * min-degree-hidden node). When `nodeIds` is omitted, all edges are used.
 *
 * O(E) once; cheap to memoize against the edge set.
 */
export function buildUndirectedAdjacency(
  edges: readonly EgoEdge[],
  nodeIds?: ReadonlySet<string>,
): Map<string, Set<string>> {
  const m = new Map<string, Set<string>>();
  const link = (a: string, b: string) => {
    let s = m.get(a);
    if (!s) {
      s = new Set<string>();
      m.set(a, s);
    }
    s.add(b);
  };
  for (const e of edges) {
    if (nodeIds && (!nodeIds.has(e.source) || !nodeIds.has(e.target))) continue;
    link(e.source, e.target);
    link(e.target, e.source);
  }
  return m;
}

/**
 * N-hop ego BFS over an undirected adjacency map.
 *
 * Returns the set of node ids within `hops` of `id` (inclusive of `id`).
 * Because `adjacency` is undirected (see buildUndirectedAdjacency), sink nodes
 * such as HTTP endpoints expand to their inbound handler/module neighbors.
 */
export function bfsEgo(
  adjacency: Map<string, Set<string>>,
  id: string,
  hops: number,
): Set<string> {
  const set = new Set<string>([id]);
  let frontier = [id];
  for (let h = 0; h < hops; h++) {
    const next: string[] = [];
    for (const f of frontier) {
      const nbrs = adjacency.get(f);
      if (!nbrs) continue;
      for (const nb of nbrs) {
        if (!set.has(nb)) {
          set.add(nb);
          next.push(nb);
        }
      }
    }
    if (next.length === 0) break;
    frontier = next;
  }
  return set;
}
