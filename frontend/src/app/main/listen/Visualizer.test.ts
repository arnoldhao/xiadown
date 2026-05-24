import { describe, expect, test } from "bun:test";

import {
  MAX_VISUALIZER_CANVAS_DIMENSION,
  MAX_VISUALIZER_CANVAS_PIXELS,
  artworkPulseAttackSeconds,
  artworkPulseDurationSeconds,
  resolveArtworkPulseEnergyBaseline,
  resolveArtworkPulseProgress,
  resolveArtworkPulseReferenceEnergy,
  resolveArtworkPulseSpectralFlux,
  resolveArtworkPulseTargetScale,
  resolveArtworkPulseTimingDecision,
  resolveVisualizerCanvasPixelSize,
  resolveVisualizerAudioTime,
  type ArtworkPulseEvent,
} from "@/app/main/listen/visualizer-timing";

function pulse(overrides: Partial<ArtworkPulseEvent> = {}): ArtworkPulseEvent {
  return {
    durationSeconds: 1.2,
    energy: 0.7,
    id: 1,
    startTimeSeconds: 10,
    tone: 0,
    ...overrides,
  };
}

describe("artwork visualizer timing helpers", () => {
  test("estimates audio time with bounded native offset compensation", () => {
    expect(resolveVisualizerAudioTime(12, 1000, 1033, 0.032)).toBeCloseTo(12.065, 6);
    expect(resolveVisualizerAudioTime(12, 1000, 1033, 0.032, 0.021)).toBeCloseTo(12.086, 6);
    expect(resolveVisualizerAudioTime(12, 1000, 1033, 0.032, -0.021)).toBeCloseTo(12.044, 6);
    expect(resolveVisualizerAudioTime(12, 1000, 1033, 0.032, 1)).toBeCloseTo(12.315, 6);
    expect(resolveVisualizerAudioTime(12, 1000, 990, 0.032)).toBe(0);
    expect(resolveVisualizerAudioTime(0, 1000, 1033, 0.032)).toBe(0);
  });

  test("caps canvas backing store size for large player surfaces", () => {
    expect(resolveVisualizerCanvasPixelSize(100, 50, 2)).toEqual({ width: 200, height: 100 });
    expect(resolveVisualizerCanvasPixelSize(0, 50, 2)).toBeNull();
    expect(resolveVisualizerCanvasPixelSize(100, 50, Number.NaN)).toEqual({ width: 100, height: 50 });

    const huge = resolveVisualizerCanvasPixelSize(10_000, 10_000, 2);
    expect(huge).not.toBeNull();
    expect(huge!.width).toBeLessThanOrEqual(MAX_VISUALIZER_CANVAS_DIMENSION);
    expect(huge!.height).toBeLessThanOrEqual(MAX_VISUALIZER_CANVAS_DIMENSION);
    expect(huge!.width * huge!.height).toBeLessThanOrEqual(MAX_VISUALIZER_CANVAS_PIXELS);

    const wide = resolveVisualizerCanvasPixelSize(5_000, 500, 2);
    expect(wide).not.toBeNull();
    expect(wide!.width).toBeLessThanOrEqual(MAX_VISUALIZER_CANVAS_DIMENSION);
    expect(wide!.width * wide!.height).toBeLessThanOrEqual(MAX_VISUALIZER_CANVAS_PIXELS);
  });

  test("uses the current artwork energy value for cover pulse strength", () => {
    expect(resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10,
      energy: 0.12,
      energyBaseline: 0,
      lastPulseEnergy: 0,
      lastPulseTimeSeconds: -Infinity,
      mode: "neonPulse",
      previousEnergy: 0,
      spectralFlux: 0,
      visualizerTimeSeconds: 10,
    })?.energy).toBeCloseTo(0.12, 6);
    expect(resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10,
      energy: 0.75,
      energyBaseline: 0.3,
      lastPulseEnergy: 0.7,
      lastPulseTimeSeconds: -Infinity,
      mode: "pondRipple",
      previousEnergy: 0.68,
      spectralFlux: 0,
      visualizerTimeSeconds: 10,
    })?.energy).toBeCloseTo(0.75, 6);
    expect(resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10,
      energy: 2,
      energyBaseline: 0.4,
      lastPulseEnergy: 0.94,
      lastPulseTimeSeconds: -Infinity,
      mode: "pondRipple",
      previousEnergy: 0.86,
      spectralFlux: 0,
      visualizerTimeSeconds: 10,
    })?.energy).toBe(1);
  });

  test("samples cover pulse events from artwork energy peaks", () => {
    const silence = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10,
      energy: 0.02,
      energyBaseline: 0,
      lastPulseEnergy: 0,
      lastPulseTimeSeconds: -Infinity,
      mode: "neonPulse",
      previousEnergy: 0,
      spectralFlux: 0,
      visualizerTimeSeconds: 10,
    });
    expect(silence).toBeNull();

    const belowDynamicThreshold = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10,
      energy: 0.2,
      energyBaseline: 0.18,
      lastPulseEnergy: 0.18,
      lastPulseTimeSeconds: 9.2,
      mode: "neonPulse",
      previousEnergy: 0.08,
      spectralFlux: 0,
      visualizerTimeSeconds: 10,
    });
    expect(belowDynamicThreshold).toBeNull();

    const firstPulse = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10,
      energy: 0.42,
      energyBaseline: 0,
      lastPulseEnergy: 0,
      lastPulseTimeSeconds: -Infinity,
      mode: "pondRipple",
      previousEnergy: 0.1,
      spectralFlux: 0,
      visualizerTimeSeconds: 10,
    });
    expect(firstPulse).toMatchObject({
      startTimeSeconds: 10,
    });
    expect(firstPulse?.energy).toBeCloseTo(0.42, 6);

    const tooSoon = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10.12,
      energy: 0.9,
      energyBaseline: 0.3,
      lastPulseEnergy: 0.42,
      lastPulseTimeSeconds: 10,
      mode: "neonPulse",
      previousEnergy: 0.42,
      spectralFlux: 0,
      visualizerTimeSeconds: 10.12,
    });
    expect(tooSoon).toBeNull();

    const closerPeak = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10.17,
      energy: 0.55,
      energyBaseline: 0.3,
      lastPulseEnergy: 0.42,
      lastPulseTimeSeconds: 10,
      mode: "pondRipple",
      previousEnergy: 0.45,
      spectralFlux: 0,
      visualizerTimeSeconds: 10.17,
    });
    expect(closerPeak).toMatchObject({
      startTimeSeconds: 10.17,
    });

    const sustainedEnergyWithoutPeak = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10.3,
      energy: 0.43,
      energyBaseline: 0.3,
      lastPulseEnergy: 0.42,
      lastPulseTimeSeconds: 10,
      mode: "neonPulse",
      previousEnergy: 0.42,
      spectralFlux: 0,
      visualizerTimeSeconds: 10.3,
    });
    expect(sustainedEnergyWithoutPeak).toBeNull();

    const risingPeak = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10.3,
      energy: 0.5,
      energyBaseline: 0.3,
      lastPulseEnergy: 0.42,
      lastPulseTimeSeconds: 10,
      mode: "neonPulse",
      previousEnergy: 0.43,
      spectralFlux: 0,
      visualizerTimeSeconds: 10.3,
    });
    expect(risingPeak).toMatchObject({
      startTimeSeconds: 10.3,
    });
    expect(risingPeak?.energy).toBeCloseTo(0.5, 6);
  });

  test("samples slow ballad swells from cumulative energy rise", () => {
    const slowSwell = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 12,
      energy: 0.21,
      energyBaseline: 0.12,
      lastPulseEnergy: 0.16,
      lastPulseTimeSeconds: 11,
      mode: "pondRipple",
      previousEnergy: 0.202,
      spectralFlux: 0,
      visualizerTimeSeconds: 12,
    });
    expect(slowSwell).toMatchObject({
      startTimeSeconds: 12,
    });
    expect(slowSwell?.energy).toBeCloseTo(0.21, 6);
  });

  test("samples high-energy rock platforms from spectral flux", () => {
    const rockHit = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 20,
      energy: 0.88,
      energyBaseline: 0.84,
      lastPulseEnergy: 0.9,
      lastPulseTimeSeconds: 19.5,
      mode: "neonPulse",
      previousEnergy: 0.88,
      spectralFlux: 0.115,
      visualizerTimeSeconds: 20,
    });
    expect(rockHit).toMatchObject({
      startTimeSeconds: 20,
    });
    expect(rockHit?.energy).toBeCloseTo(0.88, 6);
  });

  test("keeps low-tide music breathing with sparse ambient pulses", () => {
    const tooSoonAmbient = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 30,
      energy: 0.05,
      energyBaseline: 0.05,
      lastPulseEnergy: 0.05,
      lastPulseTimeSeconds: 29.2,
      mode: "pondRipple",
      previousEnergy: 0.05,
      spectralFlux: 0,
      visualizerTimeSeconds: 30,
    });
    expect(tooSoonAmbient).toBeNull();

    const ambientPulse = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 30.6,
      energy: 0.05,
      energyBaseline: 0.05,
      lastPulseEnergy: 0.05,
      lastPulseTimeSeconds: 29.2,
      mode: "pondRipple",
      previousEnergy: 0.05,
      spectralFlux: 0,
      visualizerTimeSeconds: 30.6,
    });
    expect(ambientPulse).toMatchObject({
      startTimeSeconds: 30.6,
    });
  });

  test("keeps non-cover pulse modes out of pulse scheduling", () => {
    expect(resolveArtworkPulseTimingDecision({
      analysisTimeSeconds: 10,
      energy: 0.9,
      energyBaseline: 0,
      lastPulseEnergy: 0,
      lastPulseTimeSeconds: -Infinity,
      mode: "ring",
      previousEnergy: 0,
      spectralFlux: 0,
      visualizerTimeSeconds: 10,
    })).toBeNull();
  });

  test("tracks pulse energy baseline slowly upward and faster downward", () => {
    expect(resolveArtworkPulseEnergyBaseline(0.8, 0.2)).toBeCloseTo(0.227, 6);
    expect(resolveArtworkPulseEnergyBaseline(0.2, 0.8)).toBeCloseTo(0.704, 6);
    expect(resolveArtworkPulseEnergyBaseline(Number.NaN, 0.5)).toBeCloseTo(0.42, 6);
  });

  test("lets previous pulse reference relax after quieter passages", () => {
    expect(resolveArtworkPulseReferenceEnergy(0.2, 0.6)).toBeCloseTo(0.552, 6);
    expect(resolveArtworkPulseReferenceEnergy(0.7, 0.6)).toBeCloseTo(0.6, 6);
  });

  test("measures positive spectral movement between visualizer bands", () => {
    expect(resolveArtworkPulseSpectralFlux([0.8, 0.6, 0.9, 0.7], [0.55, 0.6, 0.72, 0.7]))
      .toBeCloseTo(0.4255, 6);
    expect(resolveArtworkPulseSpectralFlux([0.4, 0.4], [0.6, 0.5])).toBeCloseTo(0.032, 6);
    expect(resolveArtworkPulseSpectralFlux([0.4], [])).toBe(0);
  });

  test("derives canvas pulse progress from audio time", () => {
    expect(resolveArtworkPulseProgress(10, 1.2, 9.9)).toBe(0);
    expect(resolveArtworkPulseProgress(10, 1.2, 10)).toBe(0);
    expect(resolveArtworkPulseProgress(10, 1.2, 10.6)).toBeCloseTo(0.5, 6);
    expect(resolveArtworkPulseProgress(10, 1.2, 11.4)).toBe(1);
    expect(resolveArtworkPulseProgress(0, 1.2, 10)).toBe(0);
    expect(resolveArtworkPulseTargetScale(0)).toBeCloseTo(1.06, 6);
    expect(resolveArtworkPulseTargetScale(1)).toBeCloseTo(1.5, 6);
  });

  test("keeps pulse expansion duration bounded", () => {
    expect(artworkPulseAttackSeconds()).toBeCloseTo(0.18, 6);
    expect(artworkPulseDurationSeconds(0)).toBeCloseTo(1.05, 6);
    expect(artworkPulseDurationSeconds(1)).toBeCloseTo(1.39, 6);
    expect(artworkPulseDurationSeconds(pulse({ energy: 0.5 }).energy)).toBeCloseTo(1.22, 6);
  });
});
