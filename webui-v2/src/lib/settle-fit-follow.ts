/* ============================================================
   lib/settle-fit-follow.ts — pure decisions for the during-settle camera
   auto-follow window (#5462).

   The graph canvas keeps the camera framed while a force layout explodes/settles
   via a throttled fitView tracker. Two sticky/transient flags govern it:

     • userInteracted — STICKY: latches true on the first GENUINE user camera move
       (pan / wheel-zoom / drag) or canvas click, and never resets. Used so the
       steady-state never re-fits behind a manual pan.
     • programmaticFollow — the "auto-follow window": armed when a PROGRAMMATIC
       re-settle starts (Reset, group-by/layout change, deep-link re-explode,
       stream-finalize). While armed the tracker follows the explode even though
       userInteracted has long since latched.

   The bug these pure helpers guard against: a programmatic re-settle is almost
   always initiated AFTER the user has already clicked/panned something, so the
   sticky userInteracted latch was suppressing the tracker the instant Reset began
   → the graph spread with a frozen camera and drifted off-screen. The fix lets the
   auto-follow window override the latch, and uses cosmos.gl's `userDriven` flag
   (d3 sourceEvent !== null) to tell a REAL user move from the tracker's OWN
   programmatic fitView (which also emits zoom events).
   ============================================================ */

/**
 * Should the during-settle camera tracker run / keep running this frame?
 *
 * The sticky `userInteracted` latch is IGNORED while the programmatic auto-follow
 * window is armed — that's the whole point: a Reset issued after an earlier click
 * must still follow the explode. Outside the window the latch governs as before
 * (never fight a manual pan). Owners that always win (focus/ego view, a camera
 * restore in progress) hard-stop tracking regardless.
 */
export function shouldTrackSettleFit(opts: {
  programmaticFollow: boolean;
  userInteracted: boolean;
  isFocusView: boolean;
  suppressFit: boolean;
}): boolean {
  if (opts.isFocusView || opts.suppressFit) return false;
  if (opts.programmaticFollow) return true; // window armed → override the latch
  return !opts.userInteracted; // outside the window: never fight a manual pan
}

/**
 * Does this camera event represent a GENUINE user move that should latch
 * `userInteracted` and cancel the auto-follow window?
 *
 * cosmos.gl reports `userDriven` (true ⇔ d3 sourceEvent is present) on zoom
 * events: a real wheel/pinch/pan is user-driven; the tracker's OWN fitView glide
 * is NOT (sourceEvent === null). If we treated the tracker's programmatic fits as
 * user input they'd cancel the very window they're serving on the first frame —
 * re-introducing the drift. Pointer drags and canvas clicks are inherently
 * user-driven (only a real pointer produces them), so callers pass `userDriven:
 * true` for those.
 */
export function isGenuineUserCameraMove(userDriven: boolean): boolean {
  return userDriven === true;
}
