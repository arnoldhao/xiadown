import { describe, expect, test } from "bun:test";

import { nextRSSBilibiliVideoSurfaceSequence } from "./bilibili-video-surface-sequence";

describe("RSS Bilibili video surface sequence", () => {
  test("stays strictly monotonic across same-millisecond remounts", () => {
    const now = Date.now() + 60_000;
    const first = nextRSSBilibiliVideoSurfaceSequence(now);
    const second = nextRSSBilibiliVideoSurfaceSequence(now);
    const clockRollback = nextRSSBilibiliVideoSurfaceSequence(now - 1_000);

    expect(second).toBe(first + 1);
    expect(clockRollback).toBe(second + 1);
  });

  test("advances to a later wall-clock diagnostic range", () => {
    const later = Date.now() + 120_000;
    expect(nextRSSBilibiliVideoSurfaceSequence(later)).toBeGreaterThanOrEqual(
      Math.trunc(later) * 1_000,
    );
  });
});
