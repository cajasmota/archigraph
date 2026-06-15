/* ============================================================
   components/ui/coverage-kind-overlay.tsx — coverage-kind node overlay for the
   diagram surfaces (#5147).

   Two pieces, both pure functions of the resolved {@link CoverageKindState}:

     1. {@link coverageKindRingStyle} — an inline CSS `boxShadow` ring (+ optional
        merge with an existing shadow) keyed to the kind's tone color, applied to
        a React-Flow HTML node so each node carries the kind decoration the ticket
        asks for. Degrades to NO ring for the neutral capability default so we
        never paint a misleading authoritative tint on a node with no real
        coverage signal.

     2. {@link CoverageKindOverlayToggle} — a small corner control that toggles
        the overlay and, when on, shows the shared {@link CoverageKindIndicator}
        legend so the ring's meaning (Line ▸ Reach ▸ Capability) is always
        legible — the kind is conveyed by icon + tone + tooltip, never color
        alone. Reuses the #5067 indicator verbatim.

   Both surfaces (flow-dag / topology / iac) reuse THESE, so the node tint, the
   legend, and the Quality-tab row chip share one source of truth.
   ============================================================ */

import { Eye, EyeOff } from "lucide-react";
import { cn } from "@/lib/utils";
import { CoverageKindIndicator } from "@/components/ui/coverage-kind-indicator";
import type { CoverageKindState } from "@/hooks/use-coverage-kind";

/**
 * Inline style fragment that decorates a node with the coverage-kind tone ring.
 * `enabled` is the per-surface toggle. When the resolved kind is the neutral
 * `capability` default (no real coverage signal) we render NO ring — honest
 * "no decoration" rather than a fake green. An existing node `boxShadow` (e.g.
 * a selection ring) is preserved by composing in front of the kind ring.
 */
export function coverageKindRingStyle(
  state: CoverageKindState,
  enabled: boolean,
  existingShadow?: string,
): { boxShadow?: string } {
  if (!enabled) return existingShadow ? { boxShadow: existingShadow } : {};
  // capability = no authoritative/reach signal → no ring (never a fake tint).
  if (state.decoration.kind === "capability") {
    return existingShadow ? { boxShadow: existingShadow } : {};
  }
  const ring = `0 0 0 2px ${state.decoration.ringColor}`;
  return { boxShadow: existingShadow ? `${existingShadow}, ${ring}` : ring };
}

export interface CoverageKindOverlayToggleProps {
  state: CoverageKindState;
  enabled: boolean;
  onToggle: () => void;
  className?: string;
}

/**
 * Corner control: an eye toggle plus, when enabled, the shared coverage-kind
 * legend chip so the node tint reads unambiguously. Pure presentation — the
 * caller owns the `enabled` state and threads the ring into its node data.
 */
export function CoverageKindOverlayToggle({
  state,
  enabled,
  onToggle,
  className,
}: CoverageKindOverlayToggleProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md border border-border bg-surface/90 px-1.5 py-1 backdrop-blur",
        className,
      )}
    >
      <button
        type="button"
        onClick={onToggle}
        aria-pressed={enabled}
        className={cn(
          "inline-flex items-center gap-1 h-6 px-1.5 rounded text-[11px] transition-colors",
          enabled ? "bg-accent text-accent-text" : "bg-surface text-text-3 hover:bg-surface-2",
        )}
        title={
          enabled
            ? "Hide the coverage-kind ring on nodes"
            : "Tint nodes by which grafel coverage applies to this group (line / reach / capability)"
        }
      >
        {enabled ? <Eye size={12} /> : <EyeOff size={12} />}
        Coverage
      </button>
      {enabled && <CoverageKindIndicator state={state.state} iconOnly={false} />}
    </div>
  );
}
