/* ============================================================
   components/flow-dag/FlowchartEdge.tsx — custom React Flow edge for the
   Flowchart view (#4819).

   Draws a control-flow edge styled + labelled by its kind:
     - branch_true   → "yes"  (green-ish), the taken branch off a diamond
     - branch_false  → "no"   (muted),     the not-taken branch
     - loop_back     → "loop" (amber, dashed), the body-tail → header back-edge
     - exit          → "exit" (danger, dashed), an early return/throw → end
     - seq           → unlabelled neutral fall-through

   Follows ELK's orthogonal route (right-angle H/V segments) when present, else
   falls back to a smooth-step path — never a diagonal bezier, so the diagram
   reads like a classic flowchart.
   ============================================================ */

import { memo } from "react";
import {
  BaseEdge,
  EdgeLabelRenderer,
  getSmoothStepPath,
  type EdgeProps,
} from "@xyflow/react";
import { orthogonalPath } from "@/lib/elk-layout";
import type { ControlFlowEdgeKind } from "@/data/types";

export interface FlowchartEdgeData extends Record<string, unknown> {
  kind: ControlFlowEdgeKind;
  elkPoints?: { x: number; y: number }[];
}

interface EdgeStyle {
  label: string;
  stroke: string;
  dash?: string;
}

function edgeStyleFor(kind: ControlFlowEdgeKind): EdgeStyle {
  switch (kind) {
    case "branch_true":
      return { label: "yes", stroke: "var(--success, #2ea043)" };
    case "branch_false":
      return { label: "no", stroke: "var(--text-3)" };
    case "loop_back":
      return { label: "loop", stroke: "var(--warning, #d4a72c)", dash: "4 3" };
    case "exit":
      return { label: "exit", stroke: "var(--danger)", dash: "4 3" };
    default:
      return { label: "", stroke: "var(--border)" };
  }
}

function FlowchartEdgeImpl({
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  markerEnd,
  data,
}: EdgeProps) {
  const ed = data as FlowchartEdgeData | undefined;
  const kind = ed?.kind ?? "seq";
  const es = edgeStyleFor(kind);

  const elk = orthogonalPath(ed?.elkPoints ?? []);
  let path: string;
  let labelX: number;
  let labelY: number;
  if (elk) {
    ({ path, labelX, labelY } = elk);
  } else {
    [path, labelX, labelY] = getSmoothStepPath({
      sourceX,
      sourceY,
      targetX,
      targetY,
      sourcePosition,
      targetPosition,
    });
  }

  return (
    <>
      <BaseEdge
        path={path}
        markerEnd={markerEnd}
        style={{
          stroke: es.stroke,
          strokeWidth: kind === "seq" ? 1.5 : 1.75,
          strokeDasharray: es.dash,
        }}
      />
      {es.label && (
        <EdgeLabelRenderer>
          <div
            className="absolute pointer-events-none rounded px-1 py-px text-[9px] font-semibold leading-none"
            style={{
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              background: "var(--surface)",
              color: es.stroke,
              border: `1px solid color-mix(in srgb, ${es.stroke} 45%, transparent)`,
            }}
          >
            {es.label}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
}

export const FlowchartEdge = memo(FlowchartEdgeImpl);
