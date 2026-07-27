import { describe, expect, test } from "bun:test";

import {
  capturePointerZoomAnchor,
  normalizeWheelDeltaY,
  restorePointerZoomAnchor,
  zoomAfterWheel,
} from "./library-pointer-zoom";

describe("Library pointer-anchored zoom", () => {
  test("normalizes pixel, line and page wheel deltas before zooming", () => {
    expect(normalizeWheelDeltaY(2, 0, 600)).toBe(2);
    expect(normalizeWheelDeltaY(2, 1, 600)).toBe(32);
    expect(normalizeWheelDeltaY(2, 2, 600)).toBe(1_200);
    expect(
      zoomAfterWheel(1, -2, 1, 600, (value) =>
        Math.min(3, Math.max(0.5, value))),
    ).toBeGreaterThan(1);
  });

  test("keeps the content under the pointer stable as scroll extents change", () => {
    const stage = {
      clientHeight: 300,
      clientWidth: 400,
      scrollHeight: 600,
      scrollLeft: 100,
      scrollTop: 50,
      scrollWidth: 800,
      getBoundingClientRect: () => ({
        left: 20,
        top: 30,
      }),
    } as unknown as HTMLElement;

    const anchor = capturePointerZoomAnchor(stage, 220, 180);
    expect(anchor.contentRatioX).toBe(0.375);
    expect(anchor.contentRatioY).toBeCloseTo(1 / 3);

    Object.assign(stage, { scrollHeight: 1_200, scrollWidth: 1_600 });
    restorePointerZoomAnchor(stage, anchor);
    expect(stage.scrollLeft).toBe(400);
    expect(stage.scrollTop).toBe(250);
  });
});
