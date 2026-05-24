import type { EqualizerVisualizerMode } from "@/shared/contracts/equalizer";

export const VISUALIZER_CLOCK_LATENCY_COMPENSATION_SECONDS = 0;
export const MAX_VISUALIZER_CANVAS_DIMENSION = 4096;
export const MAX_VISUALIZER_CANVAS_PIXELS = 8_388_608;
export const MAX_VISUALIZER_FRAME_TIME_OFFSET_SECONDS = 0.25;
export const ARTWORK_PULSE_MIN_PEAK_INTERVAL_SECONDS = 0.16;
export const ARTWORK_PULSE_MIN_AMBIENT_INTERVAL_SECONDS = 1.35;
export const ARTWORK_PULSE_AMBIENT_MIN_EVENT_ENERGY = 0.035;
export const ARTWORK_PULSE_MIN_EVENT_ENERGY = 0.085;
export const ARTWORK_PULSE_DYNAMIC_THRESHOLD_OFFSET = 0.032;
export const ARTWORK_PULSE_MIN_ENERGY_RISE = 0.014;
export const ARTWORK_PULSE_MIN_CUMULATIVE_ENERGY_RISE = 0.042;
export const ARTWORK_PULSE_MIN_SPECTRAL_FLUX = 0.095;
export const ARTWORK_PULSE_BASELINE_ATTACK_RATIO = 0.045;
export const ARTWORK_PULSE_BASELINE_RELEASE_RATIO = 0.16;
export const ARTWORK_PULSE_REFERENCE_RELEASE_RATIO = 0.12;

export type ArtworkPulseEvent = {
  durationSeconds: number;
  energy: number;
  id: number;
  startTimeSeconds: number;
  tone: number;
};

export type ArtworkPulseTimingCandidate = {
  energy: number;
  startTimeSeconds: number;
};

export type ArtworkPulseTimingDecision = ArtworkPulseTimingCandidate | null;

export type ArtworkPulseTimingInput = {
  analysisTimeSeconds: number;
  energyBaseline: number;
  energy: number;
  lastPulseEnergy: number;
  lastPulseTimeSeconds: number;
  mode: EqualizerVisualizerMode;
  previousEnergy: number;
  spectralFlux: number;
  visualizerTimeSeconds: number;
};

export function resolveArtworkPulseProgress(
  startTimeSeconds: number,
  durationSeconds: number,
  visualizerTimeSeconds: number,
) {
  if (startTimeSeconds <= 0 || durationSeconds <= 0 || visualizerTimeSeconds <= 0) {
    return 0;
  }
  return clampVisualizerUnit((visualizerTimeSeconds - startTimeSeconds) / durationSeconds);
}

export function resolveArtworkPulseTargetScale(energy: number) {
  return 1.06 + clampVisualizerUnit(energy) * 0.44;
}

export function resolveArtworkPulseEnergyBaseline(energy: number, previousBaseline: number) {
  const clampedEnergy = clampVisualizerUnit(energy);
  const clampedBaseline = clampVisualizerUnit(previousBaseline);
  const ratio = clampedEnergy > clampedBaseline
    ? ARTWORK_PULSE_BASELINE_ATTACK_RATIO
    : ARTWORK_PULSE_BASELINE_RELEASE_RATIO;
  return clampedBaseline + (clampedEnergy - clampedBaseline) * ratio;
}

export function resolveArtworkPulseReferenceEnergy(energy: number, previousReference: number) {
  const clampedEnergy = clampVisualizerUnit(energy);
  const clampedReference = clampVisualizerUnit(previousReference);
  if (clampedEnergy >= clampedReference) {
    return clampedReference;
  }
  return clampedReference + (clampedEnergy - clampedReference) * ARTWORK_PULSE_REFERENCE_RELEASE_RATIO;
}

export function resolveArtworkPulseSpectralFlux(
  bands: readonly number[],
  previousBands: readonly number[],
) {
  const length = Math.min(bands.length, previousBands.length);
  if (length === 0) {
    return 0;
  }
  let positiveRise = 0;
  let currentEnergy = 0;
  for (let index = 0; index < length; index += 1) {
    const current = clampVisualizerUnit(bands[index] ?? 0);
    const previous = clampVisualizerUnit(previousBands[index] ?? 0);
    positiveRise += Math.max(0, current - previous);
    currentEnergy += current;
  }
  const averageRise = positiveRise / length;
  const averageEnergy = currentEnergy / length;
  return clampVisualizerUnit(averageRise * 3.4 + averageEnergy * 0.08);
}

export function resolveArtworkPulseTimingDecision(
  input: ArtworkPulseTimingInput,
): ArtworkPulseTimingDecision {
  if (input.mode !== "neonPulse" && input.mode !== "pondRipple") {
    return null;
  }
  const energy = clampVisualizerUnit(input.energy);
  if (energy < ARTWORK_PULSE_AMBIENT_MIN_EVENT_ENERGY) {
    return null;
  }
  const startTimeSeconds = resolveArtworkPulseStartTime(
    input.analysisTimeSeconds,
    input.visualizerTimeSeconds,
  );
  if (startTimeSeconds <= 0) {
    return null;
  }
  const secondsSincePulse = startTimeSeconds - input.lastPulseTimeSeconds;
  const peakIntervalElapsed =
    !Number.isFinite(secondsSincePulse) || secondsSincePulse >= ARTWORK_PULSE_MIN_PEAK_INTERVAL_SECONDS;
  const ambientIntervalElapsed =
    !Number.isFinite(secondsSincePulse) || secondsSincePulse >= ARTWORK_PULSE_MIN_AMBIENT_INTERVAL_SECONDS;
  const previousEnergy = clampVisualizerUnit(input.previousEnergy);
  const energyBaseline = clampVisualizerUnit(input.energyBaseline);
  const spectralFlux = clampVisualizerUnit(input.spectralFlux);
  const triggerThreshold = Math.max(
    ARTWORK_PULSE_MIN_EVENT_ENERGY,
    energyBaseline + ARTWORK_PULSE_DYNAMIC_THRESHOLD_OFFSET,
  );
  const energyRise = energy - previousEnergy;
  const crossesThreshold = previousEnergy < triggerThreshold && energy >= triggerThreshold;
  const cumulativeRise = energy - clampVisualizerUnit(input.lastPulseEnergy);
  const accumulatesNewPeak = cumulativeRise >= ARTWORK_PULSE_MIN_CUMULATIVE_ENERGY_RISE;
  const energyPeak =
    energy >= triggerThreshold &&
    (crossesThreshold || accumulatesNewPeak || energyRise >= ARTWORK_PULSE_MIN_ENERGY_RISE);
  const fluxPeak = energy >= ARTWORK_PULSE_MIN_EVENT_ENERGY && spectralFlux >= ARTWORK_PULSE_MIN_SPECTRAL_FLUX;
  const ambientPulse = energy >= ARTWORK_PULSE_AMBIENT_MIN_EVENT_ENERGY && ambientIntervalElapsed;
  if (energyPeak || fluxPeak) {
    if (!peakIntervalElapsed) {
      return null;
    }
  } else if (!ambientPulse) {
    return null;
  }
  return {
    energy: fluxPeak ? Math.max(energy, spectralFlux) : energy,
    startTimeSeconds,
  };
}

function resolveArtworkPulseStartTime(
  analysisTimeSeconds: number,
  visualizerTimeSeconds: number,
) {
  if (visualizerTimeSeconds > 0) {
    return visualizerTimeSeconds;
  }
  return analysisTimeSeconds > 0 ? analysisTimeSeconds : 0;
}

export function artworkPulseAttackSeconds() {
  return 0.18;
}

export function resolveVisualizerAudioTime(
  analysisTimeSeconds: number,
  receivedAtMs: number,
  nowMs: number,
  latencyCompensationSeconds = VISUALIZER_CLOCK_LATENCY_COMPENSATION_SECONDS,
  frameTimeOffsetSeconds = 0,
) {
  if (analysisTimeSeconds <= 0 || receivedAtMs <= 0 || nowMs < receivedAtMs) {
    return 0;
  }
  const elapsedSeconds = (nowMs - receivedAtMs) / 1000;
  const nativeOffsetSeconds = Number.isFinite(frameTimeOffsetSeconds)
    ? Math.min(
      MAX_VISUALIZER_FRAME_TIME_OFFSET_SECONDS,
      Math.max(-MAX_VISUALIZER_FRAME_TIME_OFFSET_SECONDS, frameTimeOffsetSeconds),
    )
    : 0;
  return analysisTimeSeconds + nativeOffsetSeconds + elapsedSeconds + Math.max(0, latencyCompensationSeconds);
}

export function resolveVisualizerCanvasPixelSize(
  cssWidth: number,
  cssHeight: number,
  devicePixelRatio: number,
) {
  if (
    !Number.isFinite(cssWidth) ||
    !Number.isFinite(cssHeight) ||
    cssWidth <= 0 ||
    cssHeight <= 0
  ) {
    return null;
  }
  let pixelRatio = Math.min(
    Number.isFinite(devicePixelRatio) && devicePixelRatio > 0 ? devicePixelRatio : 1,
    2,
  );
  const cssArea = cssWidth * cssHeight;
  if (cssArea * pixelRatio * pixelRatio > MAX_VISUALIZER_CANVAS_PIXELS) {
    pixelRatio = Math.sqrt(MAX_VISUALIZER_CANVAS_PIXELS / cssArea);
  }
  const desiredWidth = cssWidth * pixelRatio;
  const desiredHeight = cssHeight * pixelRatio;
  if (desiredWidth > MAX_VISUALIZER_CANVAS_DIMENSION || desiredHeight > MAX_VISUALIZER_CANVAS_DIMENSION) {
    pixelRatio *= Math.min(
      MAX_VISUALIZER_CANVAS_DIMENSION / desiredWidth,
      MAX_VISUALIZER_CANVAS_DIMENSION / desiredHeight,
    );
  }
  return {
    width: Math.max(1, Math.floor(cssWidth * pixelRatio)),
    height: Math.max(1, Math.floor(cssHeight * pixelRatio)),
  };
}

export function artworkPulseDurationSeconds(energy: number) {
  const clampedEnergy = clampVisualizerUnit(energy);
  return 1.05 + clampedEnergy * 0.34;
}

function clampVisualizerUnit(value: number) {
  return Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : 0;
}
