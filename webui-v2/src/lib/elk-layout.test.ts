/* elk-layout.test.ts — shared elkjs layout helper (#4825). Verifies the helper
   lays out a nested/compound graph (container + children + cross-container edge)
   and returns finite parent-relative positions + a sized container box. */

import { describe, it, expect } from "vitest";
import {
  layoutWithElk,
  type ElkLayoutNode,
  type ElkLayoutEdge,
} from "./elk-layout";

describe("layoutWithElk", () => {
  it("returns positions for a nested compound graph", async () => {
    // group A contains a + b; standalone c. Edges a→b (inside A) and b→c (cross).
    const nodes: ElkLayoutNode[] = [
      { id: "A", isContainer: true },
      { id: "a", parentId: "A", width: 120, height: 40, lane: 0 },
      { id: "b", parentId: "A", width: 120, height: 40, lane: 1 },
      { id: "c", width: 120, height: 40, lane: 2 },
    ];
    const edges: ElkLayoutEdge[] = [
      { id: "e1", source: "a", target: "b" },
      { id: "e2", source: "b", target: "c" },
    ];

    const pos = await layoutWithElk(nodes, edges, { direction: "RIGHT" });

    for (const id of ["A", "a", "b", "c"]) {
      const p = pos.get(id);
      expect(p, `position for ${id}`).toBeDefined();
      expect(Number.isFinite(p!.x)).toBe(true);
      expect(Number.isFinite(p!.y)).toBe(true);
    }

    // The container A is sized from its children (non-zero bounding box).
    const a = pos.get("A")!;
    expect(a.width).toBeGreaterThan(0);
    expect(a.height).toBeGreaterThan(0);

    // Children a/b carry their measured leaf size.
    expect(pos.get("a")!.width).toBe(120);
    expect(pos.get("b")!.height).toBe(40);
  });

  it("respects lane order along the flow direction (left→right)", async () => {
    // Three unconnected nodes with ascending lanes should land in lane order
    // along x (RIGHT direction) thanks to the layer constraint hint.
    const nodes: ElkLayoutNode[] = [
      { id: "n0", width: 100, height: 40, lane: 0 },
      { id: "n1", width: 100, height: 40, lane: 1 },
      { id: "n2", width: 100, height: 40, lane: 2 },
    ];
    const pos = await layoutWithElk(nodes, [], { direction: "RIGHT" });
    const x0 = pos.get("n0")!.x;
    const x1 = pos.get("n1")!.x;
    const x2 = pos.get("n2")!.x;
    expect(x0).toBeLessThanOrEqual(x1);
    expect(x1).toBeLessThanOrEqual(x2);
  });

  it("returns an empty map for no nodes", async () => {
    const pos = await layoutWithElk([], []);
    expect(pos.size).toBe(0);
  });
});
