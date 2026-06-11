/* ============================================================
   components/flow-dag/flowchart-shapes.ts — pure shape/caption helpers for the
   Flowchart view (#4819), split out so they're unit-testable without React Flow.

   These map a CFG node to its layout box and a human caption. The actual glyph
   drawing lives in FlowchartNode.tsx; this is just the deterministic geometry +
   label logic the layout pass and tests share.
   ============================================================ */

import type { ControlFlowNode } from "@/data/types";

/** Per-shape layout box (fed to ELK). Diamonds get a squarer box so the rotated
 *  inner square reads as a diamond; terminals are short pills; process boxes are
 *  wider rectangles to fit a source line + effect badges. */
export function boxFor(shape: ControlFlowNode["shape"]): { w: number; h: number } {
  switch (shape) {
    case "decision":
    case "loop":
      return { w: 150, h: 96 };
    case "start":
    case "end":
    case "return":
    case "throw":
      return { w: 132, h: 48 };
    default:
      return { w: 180, h: 64 };
  }
}

/** A short caption per node. Prefers the source label (full detail), else a
 *  shape-derived default — so outline/decisions levels (which ship no label)
 *  still read clearly. Decisions/loops fall back to their condition text. */
export function defaultCaption(n: ControlFlowNode): string {
  if (n.label) return n.label;
  switch (n.shape) {
    case "start":
      return "Start";
    case "end":
      return "End";
    case "return":
      return "Return";
    case "throw":
      return "Throw";
    case "decision":
      return n.condition || "decision";
    case "loop":
      return n.condition || "loop";
    default:
      return n.line ? `line ${n.line}` : "step";
  }
}
