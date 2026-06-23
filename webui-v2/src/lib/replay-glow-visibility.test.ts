import { describe, it, expect } from "vitest";
import { shouldFitReplayStep } from "./replay-glow-visibility";

const base = {
  inViewCount: 0,
  resolvedCount: 3,
  haveViewport: true,
  isFocusView: false,
  suppressFit: false,
};

describe("shouldFitReplayStep", () => {
  it("pans when the step is fully off-screen (the #5457 bug case)", () => {
    expect(shouldFitReplayStep(base)).toBe(true);
  });

  it("does NOT pan when at least one node is already in view", () => {
    expect(shouldFitReplayStep({ ...base, inViewCount: 1 })).toBe(false);
  });

  it("does NOT pan when the step resolved to no glowable node", () => {
    expect(shouldFitReplayStep({ ...base, resolvedCount: 0 })).toBe(false);
  });

  it("does NOT pan without a real viewport", () => {
    expect(shouldFitReplayStep({ ...base, haveViewport: false })).toBe(false);
  });

  it("does NOT pan in a focus/ego view (the ego fit owns the camera)", () => {
    expect(shouldFitReplayStep({ ...base, isFocusView: true })).toBe(false);
  });

  it("does NOT pan while a camera restore is suppressing the fit", () => {
    expect(shouldFitReplayStep({ ...base, suppressFit: true })).toBe(false);
  });

  it("every replayed off-screen step is eligible (so each event glows in turn)", () => {
    // Three consecutive off-screen steps all want a pan → each visibly glows.
    const steps = [
      { ...base, resolvedCount: 2 },
      { ...base, resolvedCount: 5 },
      { ...base, resolvedCount: 1 },
    ];
    expect(steps.every(shouldFitReplayStep)).toBe(true);
  });
});
