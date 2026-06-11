/**
 * Unit tests for the undirected ego-subgraph BFS (lib/ego-bfs).
 *
 * #4857 — the regression these guard against: a directional (from → to) BFS
 * from a SINK node (e.g. an HTTP endpoint, where handler `IMPLEMENTS →
 * endpoint` and module `CONTAINS → endpoint` but the endpoint emits nothing)
 * finds ZERO neighbors at any hop count. The undirected adjacency must reach
 * those inbound neighbors.
 *
 * Run with: npx vitest run src/lib/ego-bfs.test.ts
 */
import { describe, it, expect } from "vitest";
import { buildUndirectedAdjacency, bfsEgo, type EgoEdge } from "./ego-bfs";

// The canonical #4857 shape: `endpoint` is a pure sink — only inbound edges.
const SINK_EDGES: EgoEdge[] = [
  { source: "handler", target: "endpoint" }, // IMPLEMENTS
  { source: "module", target: "endpoint" }, // CONTAINS
];

describe("buildUndirectedAdjacency", () => {
  it("links both directions for every edge", () => {
    const adj = buildUndirectedAdjacency(SINK_EDGES);
    expect(adj.get("endpoint")).toEqual(new Set(["handler", "module"]));
    expect(adj.get("handler")).toEqual(new Set(["endpoint"]));
    expect(adj.get("module")).toEqual(new Set(["endpoint"]));
  });

  it("skips edges whose endpoints are not both in the allow-set", () => {
    // `module` is NOT rendered → its CONTAINS edge is dropped, but the
    // handler↔endpoint link survives.
    const rendered = new Set(["endpoint", "handler"]);
    const adj = buildUndirectedAdjacency(SINK_EDGES, rendered);
    expect(adj.get("endpoint")).toEqual(new Set(["handler"]));
    expect(adj.get("module")).toBeUndefined();
  });
});

describe("bfsEgo (undirected)", () => {
  it("reaches the inbound neighbors of a sink node (the #4857 bug)", () => {
    const adj = buildUndirectedAdjacency(SINK_EDGES);
    const ego = bfsEgo(adj, "endpoint", 1);
    expect(ego).toEqual(new Set(["endpoint", "handler", "module"]));
  });

  it("a directional adjacency would have returned only the sink (regression contrast)", () => {
    // Build a DIRECTIONAL adjacency to prove the old behaviour was broken.
    const directional = new Map<string, Set<string>>();
    for (const e of SINK_EDGES) {
      if (!directional.has(e.source)) directional.set(e.source, new Set());
      directional.get(e.source)!.add(e.target);
    }
    const ego = bfsEgo(directional, "endpoint", 6);
    expect(ego).toEqual(new Set(["endpoint"])); // 0 neighbors even at 6 hops
  });

  it("expands outward N hops and stops early when the frontier is exhausted", () => {
    const chain: EgoEdge[] = [
      { source: "a", target: "b" },
      { source: "b", target: "c" },
      { source: "c", target: "d" },
    ];
    const adj = buildUndirectedAdjacency(chain);
    expect(bfsEgo(adj, "a", 1)).toEqual(new Set(["a", "b"]));
    expect(bfsEgo(adj, "a", 2)).toEqual(new Set(["a", "b", "c"]));
    // Undirected: from the middle node both sides are reachable.
    expect(bfsEgo(adj, "c", 1)).toEqual(new Set(["b", "c", "d"]));
    // More hops than the graph is deep → still terminates with the component.
    expect(bfsEgo(adj, "a", 99)).toEqual(new Set(["a", "b", "c", "d"]));
  });

  it("returns just the root for an isolated node", () => {
    const adj = buildUndirectedAdjacency(SINK_EDGES);
    expect(bfsEgo(adj, "orphan", 3)).toEqual(new Set(["orphan"]));
  });
});
