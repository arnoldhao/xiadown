import { describe, expect, test } from "bun:test";

import {
  normalizeListenLyricsClockTimeMs,
  normalizeListenLyricsPlaybackRate,
  resolveListenLyricsVisualClockAnchor,
  resolveListenLyricsVisualClockFrame,
  resolveListenLyricsVisualClockRenderTime,
  type ListenLyricsVisualClockAnchor,
} from "@/app/main/listen/lyrics-clock";

const runningClock: ListenLyricsVisualClockAnchor = {
  sourceMs: 1_000,
  anchorMs: 1_000,
  running: true,
  playbackRate: 1,
  key: "track-one",
};

describe("listen lyrics visual clock", () => {
  test("snaps to the new source on a timeline key switch, including the pre-effect render", () => {
    const nextClock = resolveListenLyricsVisualClockAnchor({
      clock: runningClock,
      sourceTimeMs: 240,
      timelineKey: "track-two",
      running: true,
      nowMs: 1_100,
    });

    expect(nextClock).toEqual({
      sourceMs: 240,
      anchorMs: 1_100,
      running: true,
      playbackRate: 1,
      key: "track-two",
    });
    expect(
      resolveListenLyricsVisualClockRenderTime({
        sourceTimeMs: 240,
        visualTimeMs: 95_000,
        clockKey: "track-one",
        timelineKey: "track-two",
      }),
    ).toBe(240);
  });

  test("keeps source, anchor, frame, and rendered values finite and non-negative", () => {
    expect(normalizeListenLyricsClockTimeMs(Number.NaN)).toBe(0);
    expect(normalizeListenLyricsClockTimeMs(Number.POSITIVE_INFINITY)).toBe(0);
    expect(normalizeListenLyricsClockTimeMs(Number.NEGATIVE_INFINITY)).toBe(0);
    expect(normalizeListenLyricsClockTimeMs(-1)).toBe(0);
    expect(normalizeListenLyricsPlaybackRate(Number.NaN)).toBe(1);
    expect(normalizeListenLyricsPlaybackRate(-1)).toBe(1);

    const invalidClock = resolveListenLyricsVisualClockAnchor({
      clock: {
        sourceMs: Number.NaN,
        anchorMs: Number.POSITIVE_INFINITY,
        running: true,
        playbackRate: Number.NaN,
        key: "track-one",
      },
      sourceTimeMs: Number.POSITIVE_INFINITY,
      timelineKey: "track-one",
      running: true,
      nowMs: Number.NaN,
    });
    expect(invalidClock.sourceMs).toBe(0);
    expect(invalidClock.anchorMs).toBe(0);
    expect(
      resolveListenLyricsVisualClockFrame({
        clock: {
          ...invalidClock,
          sourceMs: Number.MAX_VALUE,
          anchorMs: 0,
        },
        timelineKey: "track-one",
        nowMs: Number.MAX_VALUE,
      }),
    ).toBe(0);
    expect(
      resolveListenLyricsVisualClockRenderTime({
        sourceTimeMs: Number.NaN,
        visualTimeMs: Number.NEGATIVE_INFINITY,
        clockKey: "track-one",
        timelineKey: "track-one",
      }),
    ).toBe(0);
  });

  test("smooths only tiny transport jitter and snaps meaningful clock changes", () => {
    const smoothed = resolveListenLyricsVisualClockAnchor({
      clock: runningClock,
      sourceTimeMs: 1_180,
      timelineKey: "track-one",
      running: true,
      nowMs: 1_100,
    });
    const snapped = resolveListenLyricsVisualClockAnchor({
      clock: runningClock,
      sourceTimeMs: 1_181,
      timelineKey: "track-one",
      running: true,
      nowMs: 1_100,
    });

    expect(smoothed.sourceMs).toBe(1_140);
    expect(snapped.sourceMs).toBe(1_181);
  });

  test("snaps a 250ms user offset instead of gradually hiding it", () => {
    expect(
      resolveListenLyricsVisualClockAnchor({
        clock: runningClock,
        sourceTimeMs: 1_350,
        timelineKey: "track-one",
        running: true,
        nowMs: 1_100,
      }).sourceMs,
    ).toBe(1_350);
  });

  test("advances at the authoritative 0.5x and 2x playback rates", () => {
    const doubleSpeed = resolveListenLyricsVisualClockAnchor({
      clock: runningClock,
      sourceTimeMs: 1_100,
      timelineKey: "track-one",
      running: true,
      playbackRate: 2,
      nowMs: 1_100,
    });
    expect(doubleSpeed).toMatchObject({
      sourceMs: 1_100,
      anchorMs: 1_100,
      playbackRate: 2,
    });
    expect(
      resolveListenLyricsVisualClockFrame({
        clock: doubleSpeed,
        timelineKey: "track-one",
        nowMs: 1_200,
      }),
    ).toBe(1_300);

    expect(
      resolveListenLyricsVisualClockFrame({
        clock: { ...doubleSpeed, playbackRate: 0.5 },
        timelineKey: "track-one",
        nowMs: 1_200,
      }),
    ).toBe(1_150);
  });

  test("freezes for an arbitrary buffering interval and resumes from that point", () => {
    const bufferingClock = resolveListenLyricsVisualClockAnchor({
      clock: runningClock,
      sourceTimeMs: 1_200,
      timelineKey: "track-one",
      running: false,
      nowMs: 1_200,
    });
    expect(
      resolveListenLyricsVisualClockFrame({
        clock: bufferingClock,
        timelineKey: "track-one",
        nowMs: 31_200,
      }),
    ).toBe(1_200);

    const resumedClock = resolveListenLyricsVisualClockAnchor({
      clock: bufferingClock,
      sourceTimeMs: 1_200,
      timelineKey: "track-one",
      running: true,
      nowMs: 31_200,
    });
    expect(
      resolveListenLyricsVisualClockFrame({
        clock: resumedClock,
        timelineKey: "track-one",
        nowMs: 31_300,
      }),
    ).toBe(1_300);
  });
});
