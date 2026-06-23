/* ============================================================
   lib/replay-glow-visibility.ts — pure visibility predicate for the
   MCP-activity replay glow (#5457).

   BUG (#5457): replaying MCP activity (replay-all OR clicking a single event)
   lands every step on the SAME static settled camera. Each step's nodes are
   almost always OFF-SCREEN, so the canvas glow falls back to an off-screen
   sample that pulses INVISIBLY — the user only ever sees the one step whose
   nodes happen to sit in the viewport.

   The canvas fixes this by panning the camera to a step's nodes when the step
   is fully off-screen. The decision "should we pan to this step?" is the pure
   predicate below, extracted so it can be unit-tested without a WebGL canvas.
   ============================================================ */

export interface ReplayStepVisibility {
  /** How many of the step's matched nodes are currently inside the viewport. */
  inViewCount: number;
  /** How many matched nodes the step resolved to a glowable index (capped). */
  resolvedCount: number;
  /** Viewport is known + non-empty (clientWidth/Height > 0, positions present). */
  haveViewport: boolean;
  /** A focus/ego view is active — its own fit/restore owns the camera. */
  isFocusView: boolean;
  /** A camera restore is in progress (suppress auto-fit). */
  suppressFit: boolean;
}

/**
 * True when the canvas should PAN/FIT the camera to a replay step's nodes so the
 * glow is visible. We only pan when:
 *   • the step resolved to at least one glowable node, AND
 *   • NONE of them are currently in view (a fully off-screen step — the exact
 *     case where the glow would otherwise be invisible), AND
 *   • we have a real viewport to fit into, AND
 *   • we're not in a focus/ego view or mid camera-restore (those own the view).
 *
 * When at least one node is already in view we deliberately leave the camera
 * alone so we don't yank a view the user is already reading.
 */
export function shouldFitReplayStep(v: ReplayStepVisibility): boolean {
  return (
    v.resolvedCount > 0 &&
    v.inViewCount === 0 &&
    v.haveViewport &&
    !v.isFocusView &&
    !v.suppressFit
  );
}
