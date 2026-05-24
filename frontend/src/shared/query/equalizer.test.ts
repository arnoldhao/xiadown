import { describe, expect, test } from "bun:test";

import {
  emptyEqualizerVisualizerFrame,
  normalizeEqualizerSnapshot,
  normalizeEqualizerVisualizerFrame,
  resolveEqualizerVisualizerPollDelay,
  shouldCommitEqualizerVisualizerFrame,
} from "@/shared/query/equalizer-normalize";

describe("equalizer query normalizers", () => {
  test("migrates legacy visualizer modes into placement memories", () => {
    const snapshot = normalizeEqualizerSnapshot({
      settings: {
        enabled: true,
        preampDb: 1.5,
        preset: "custom",
        visualizerMode: "waveform",
      },
      status: {
        code: "active",
        running: true,
        supported: true,
      },
    });

    expect(snapshot.settings.visualizerPlacement).toBe("spectrum");
    expect(snapshot.settings.visualizerMode).toBe("waveform");
    expect(snapshot.settings.spectrumVisualizerMode).toBe("waveform");
    expect(snapshot.settings.artworkVisualizerMode).toBe("ring");
  });

  test("keeps stored visualizer effect memories when placement is off", () => {
    const snapshot = normalizeEqualizerSnapshot({
      settings: {
        visualizerMode: "neonPulse",
        visualizerPlacement: "off",
        artworkVisualizerMode: "pondRipple",
        spectrumVisualizerMode: "heatmap",
      },
      status: { code: "unknown" },
    });

    expect(snapshot.settings.visualizerPlacement).toBe("off");
    expect(snapshot.settings.visualizerMode).toBe("off");
    expect(snapshot.settings.artworkVisualizerMode).toBe("pondRipple");
    expect(snapshot.settings.spectrumVisualizerMode).toBe("heatmap");
    expect(snapshot.status.code).toBe("unsupported");
  });

  test("normalizes visualizer frame spectrum data", () => {
    const frame = normalizeEqualizerVisualizerFrame(
      {
        running: true,
        sequence: 3,
        level: 1.4,
        bands: [-1, 0.5, 2, "bad"],
        waveform: [-2, -0.5, 2, "bad"],
        analysisTimeSeconds: 12.5,
        frameTimeOffsetSeconds: -0.027,
      },
      1234,
    );

    expect(frame.running).toBe(true);
    expect(frame.level).toBe(1);
    expect(frame.bands).toHaveLength(32);
    expect(frame.bands.slice(0, 4)).toEqual([0, 0.5, 1, 0]);
    expect(frame.waveform).toHaveLength(64);
    expect(frame.waveform.slice(0, 4)).toEqual([-1, -0.5, 1, 0]);
    expect(frame.analysisTimeSeconds).toBe(12.5);
    expect(frame.frameTimeOffsetSeconds).toBe(-0.027);
    expect(frame.receivedAtMs).toBe(1234);
  });

  test("bounds visualizer sequence and timing values", () => {
    const frame = normalizeEqualizerVisualizerFrame({
      running: true,
      sequence: -1,
      analysisTimeSeconds: -12,
      frameTimeOffsetSeconds: -0.5,
      receivedAtMs: -8,
    });

    expect(frame.sequence).toBe(0);
    expect(frame.analysisTimeSeconds).toBe(0);
    expect(frame.frameTimeOffsetSeconds).toBe(-0.25);
    expect(frame.receivedAtMs).toBe(0);
  });

  test("commits visualizer frames only when frame identity changes", () => {
    const current = {
      ...emptyEqualizerVisualizerFrame(),
      running: true,
      sequence: 12,
      receivedAtMs: 1000,
      frameTimeOffsetSeconds: 0.02,
    };
    const sameFrameReadLater = {
      ...current,
      receivedAtMs: 1033,
      frameTimeOffsetSeconds: 0.053,
    };
    const nextSequence = {
      ...sameFrameReadLater,
      sequence: 13,
    };
    const stopped = {
      ...sameFrameReadLater,
      running: false,
    };

    expect(shouldCommitEqualizerVisualizerFrame(current, sameFrameReadLater)).toBe(false);
    expect(shouldCommitEqualizerVisualizerFrame(current, nextSequence)).toBe(true);
    expect(shouldCommitEqualizerVisualizerFrame(current, stopped)).toBe(true);
  });

  test("backs off visualizer polling when the document is hidden", () => {
    expect(resolveEqualizerVisualizerPollDelay(true, false)).toBe(33);
    expect(resolveEqualizerVisualizerPollDelay(false, false)).toBe(120);
    expect(resolveEqualizerVisualizerPollDelay(true, true)).toBe(250);
    expect(resolveEqualizerVisualizerPollDelay(false, true)).toBe(1000);
  });
});
