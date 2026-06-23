import { describe, it, expect } from "vitest";
import { shouldTrackSettleFit, isGenuineUserCameraMove } from "./settle-fit-follow";

describe("shouldTrackSettleFit (#5462)", () => {
  const base = {
    programmaticFollow: false,
    userInteracted: false,
    isFocusView: false,
    suppressFit: false,
  };

  it("tracks on a clean initial load (no interaction, no window)", () => {
    expect(shouldTrackSettleFit(base)).toBe(true);
  });

  it("does NOT track once the user has latched, outside an auto-follow window", () => {
    // Steady-state: a manual pan latched userInteracted; nothing should re-fit.
    expect(shouldTrackSettleFit({ ...base, userInteracted: true })).toBe(false);
  });

  it("THE BUG: a programmatic re-settle tracks even though userInteracted latched", () => {
    // Reset pressed after the user had already clicked/panned a node. Before the
    // fix the sticky latch killed the tracker → drift. The armed window overrides.
    expect(
      shouldTrackSettleFit({ ...base, programmaticFollow: true, userInteracted: true }),
    ).toBe(true);
  });

  it("tracks during a programmatic window even with no prior interaction", () => {
    expect(shouldTrackSettleFit({ ...base, programmaticFollow: true })).toBe(true);
  });

  it("never tracks in focus/ego view (ego fit / camera restore owns the view)", () => {
    expect(
      shouldTrackSettleFit({ ...base, programmaticFollow: true, isFocusView: true }),
    ).toBe(false);
    expect(shouldTrackSettleFit({ ...base, isFocusView: true })).toBe(false);
  });

  it("never tracks while a camera-restore fit is suppressed", () => {
    expect(
      shouldTrackSettleFit({ ...base, programmaticFollow: true, suppressFit: true }),
    ).toBe(false);
  });
});

describe("isGenuineUserCameraMove (#5462)", () => {
  it("treats a user-driven (real pointer/wheel) move as genuine", () => {
    // d3 sourceEvent present → cancels the auto-follow window.
    expect(isGenuineUserCameraMove(true)).toBe(true);
  });

  it("treats a programmatic fitView transition as NOT a user move", () => {
    // The tracker's own glide (sourceEvent === null) must not cancel the window
    // it is serving — otherwise it self-cancels on the first frame.
    expect(isGenuineUserCameraMove(false)).toBe(false);
  });
});
