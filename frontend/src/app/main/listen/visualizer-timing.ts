import type { EqualizerVisualizerMode } from "@/shared/contracts/equalizer";

export const VISUALIZER_CLOCK_LATENCY_COMPENSATION_SECONDS = 0;
export const MAX_VISUALIZER_CANVAS_DIMENSION = 4096;
export const MAX_VISUALIZER_CANVAS_PIXELS = 8_388_608;
export const MAX_VISUALIZER_FRAME_TIME_OFFSET_SECONDS = 0.25;
export const ARTWORK_PULSE_TIMING_CONFIG = {
  attackSeconds: 0.11,
  baselineAttackRatio: 0.045,
  baselineReleaseRatio: 0.16,
  duration: {
    baseSeconds: 0.88,
    energySeconds: 0.28,
  },
  dynamicThresholdOffset: 0.032,
  maxStartCatchupSeconds: 0.12,
  minCumulativeEnergyRise: 0.042,
  minEnergyRise: 0.014,
  minEventEnergy: 0.085,
  minPeakIntervalSeconds: 0.16,
  minSpectralFlux: 0.095,
  neon: {
    dynamicThresholdOffset: 0.010,
    minCumulativeEnergyRise: 0.018,
    minEnergyRise: 0.006,
    minEventEnergy: 0.028,
    minSpectralFlux: 0.052,
    quietMaxEnergy: 0.18,
    quietMinSpectralFlux: 0.018,
    spectralFluxRampEndEnergy: 0.36,
  },
  referenceReleaseRatio: 0.12,
  spectralFlux: {
    averageEnergyWeight: 0.05,
    averageRiseWeight: 2.8,
    strongestRiseWeight: 0.36,
  },
  startLookbackSeconds: 0.045,
  targetScale: {
    base: 1.06,
    energy: 0.44,
  },
} as const;

export const ARTWORK_PULSE_MIN_PEAK_INTERVAL_SECONDS = ARTWORK_PULSE_TIMING_CONFIG.minPeakIntervalSeconds;
export const ARTWORK_PULSE_START_LOOKBACK_SECONDS = ARTWORK_PULSE_TIMING_CONFIG.startLookbackSeconds;
export const ARTWORK_PULSE_MAX_START_CATCHUP_SECONDS = ARTWORK_PULSE_TIMING_CONFIG.maxStartCatchupSeconds;
export const ARTWORK_PULSE_MIN_EVENT_ENERGY = ARTWORK_PULSE_TIMING_CONFIG.minEventEnergy;
export const ARTWORK_PULSE_NEON_MIN_EVENT_ENERGY = ARTWORK_PULSE_TIMING_CONFIG.neon.minEventEnergy;
export const ARTWORK_PULSE_DYNAMIC_THRESHOLD_OFFSET = ARTWORK_PULSE_TIMING_CONFIG.dynamicThresholdOffset;
export const ARTWORK_PULSE_NEON_DYNAMIC_THRESHOLD_OFFSET = ARTWORK_PULSE_TIMING_CONFIG.neon.dynamicThresholdOffset;
export const ARTWORK_PULSE_MIN_ENERGY_RISE = ARTWORK_PULSE_TIMING_CONFIG.minEnergyRise;
export const ARTWORK_PULSE_NEON_MIN_ENERGY_RISE = ARTWORK_PULSE_TIMING_CONFIG.neon.minEnergyRise;
export const ARTWORK_PULSE_MIN_CUMULATIVE_ENERGY_RISE = ARTWORK_PULSE_TIMING_CONFIG.minCumulativeEnergyRise;
export const ARTWORK_PULSE_NEON_MIN_CUMULATIVE_ENERGY_RISE =
  ARTWORK_PULSE_TIMING_CONFIG.neon.minCumulativeEnergyRise;
export const ARTWORK_PULSE_MIN_SPECTRAL_FLUX = ARTWORK_PULSE_TIMING_CONFIG.minSpectralFlux;
export const ARTWORK_PULSE_NEON_MIN_SPECTRAL_FLUX = ARTWORK_PULSE_TIMING_CONFIG.neon.minSpectralFlux;
export const ARTWORK_PULSE_NEON_QUIET_MIN_SPECTRAL_FLUX = ARTWORK_PULSE_TIMING_CONFIG.neon.quietMinSpectralFlux;
export const ARTWORK_PULSE_BASELINE_ATTACK_RATIO = ARTWORK_PULSE_TIMING_CONFIG.baselineAttackRatio;
export const ARTWORK_PULSE_BASELINE_RELEASE_RATIO = ARTWORK_PULSE_TIMING_CONFIG.baselineReleaseRatio;
export const ARTWORK_PULSE_REFERENCE_RELEASE_RATIO = ARTWORK_PULSE_TIMING_CONFIG.referenceReleaseRatio;

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
  const targetScale = ARTWORK_PULSE_TIMING_CONFIG.targetScale;
  return targetScale.base + clampVisualizerUnit(energy) * targetScale.energy;
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
  let strongestRise = 0;
  for (let index = 0; index < length; index += 1) {
    const current = clampVisualizerUnit(bands[index] ?? 0);
    const previous = clampVisualizerUnit(previousBands[index] ?? 0);
    const rise = Math.max(0, current - previous);
    positiveRise += rise;
    strongestRise = Math.max(strongestRise, rise);
    currentEnergy += current;
  }
  const averageRise = positiveRise / length;
  const averageEnergy = currentEnergy / length;
  const spectralFlux = ARTWORK_PULSE_TIMING_CONFIG.spectralFlux;
  return clampVisualizerUnit(
    averageRise * spectralFlux.averageRiseWeight +
      strongestRise * spectralFlux.strongestRiseWeight +
      averageEnergy * spectralFlux.averageEnergyWeight,
  );
}

export function resolveArtworkPulseTimingDecision(
  input: ArtworkPulseTimingInput,
): ArtworkPulseTimingDecision {
  if (input.mode !== "neonPulse" && input.mode !== "pondRipple") {
    return null;
  }
  const energy = clampVisualizerUnit(input.energy);
  const isNeonPulse = input.mode === "neonPulse";
  const minEventEnergy = isNeonPulse ? ARTWORK_PULSE_NEON_MIN_EVENT_ENERGY : ARTWORK_PULSE_MIN_EVENT_ENERGY;
  const spectralFlux = clampVisualizerUnit(input.spectralFlux);
  const minSpectralFlux = resolveArtworkPulseMinSpectralFlux(isNeonPulse, energy);
  if (energy < minEventEnergy && (!isNeonPulse || spectralFlux < minSpectralFlux)) {
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
  const previousEnergy = clampVisualizerUnit(input.previousEnergy);
  const energyBaseline = clampVisualizerUnit(input.energyBaseline);
  const dynamicThresholdOffset = isNeonPulse
    ? ARTWORK_PULSE_NEON_DYNAMIC_THRESHOLD_OFFSET
    : ARTWORK_PULSE_DYNAMIC_THRESHOLD_OFFSET;
  const minEnergyRise = isNeonPulse
    ? (energy < 0.26 ? ARTWORK_PULSE_NEON_MIN_ENERGY_RISE : ARTWORK_PULSE_MIN_ENERGY_RISE)
    : ARTWORK_PULSE_MIN_ENERGY_RISE;
  const minCumulativeEnergyRise = isNeonPulse
    ? ARTWORK_PULSE_NEON_MIN_CUMULATIVE_ENERGY_RISE
    : ARTWORK_PULSE_MIN_CUMULATIVE_ENERGY_RISE;
  const triggerThreshold = Math.max(
    minEventEnergy,
    energyBaseline + dynamicThresholdOffset,
  );
  const energyRise = energy - previousEnergy;
  const crossesThreshold = previousEnergy < triggerThreshold && energy >= triggerThreshold;
  const cumulativeRise = energy - clampVisualizerUnit(input.lastPulseEnergy);
  const accumulatesNewPeak = cumulativeRise >= minCumulativeEnergyRise;
  const energyPeak =
    energy >= triggerThreshold &&
    (crossesThreshold || accumulatesNewPeak || energyRise >= minEnergyRise);
  const fluxPeak = spectralFlux >= minSpectralFlux && (isNeonPulse || energy >= minEventEnergy);
  if (!energyPeak && !fluxPeak) {
    return null;
  }
  if (!peakIntervalElapsed) {
    return null;
  }
  return {
    energy: fluxPeak ? Math.max(energy, spectralFlux) : energy,
    startTimeSeconds,
  };
}

function resolveArtworkPulseMinSpectralFlux(isNeonPulse: boolean, energy: number) {
  if (!isNeonPulse) {
    return ARTWORK_PULSE_MIN_SPECTRAL_FLUX;
  }
  const neon = ARTWORK_PULSE_TIMING_CONFIG.neon;
  const clampedEnergy = clampVisualizerUnit(energy);
  if (clampedEnergy <= neon.quietMaxEnergy) {
    return ARTWORK_PULSE_NEON_QUIET_MIN_SPECTRAL_FLUX;
  }
  if (clampedEnergy >= neon.spectralFluxRampEndEnergy) {
    return ARTWORK_PULSE_NEON_MIN_SPECTRAL_FLUX;
  }
  const progress = (clampedEnergy - neon.quietMaxEnergy) / (neon.spectralFluxRampEndEnergy - neon.quietMaxEnergy);
  return ARTWORK_PULSE_NEON_QUIET_MIN_SPECTRAL_FLUX +
    (ARTWORK_PULSE_NEON_MIN_SPECTRAL_FLUX - ARTWORK_PULSE_NEON_QUIET_MIN_SPECTRAL_FLUX) * progress;
}

function resolveArtworkPulseStartTime(
  analysisTimeSeconds: number,
  visualizerTimeSeconds: number,
) {
  const hasAnalysisTime = analysisTimeSeconds > 0;
  const hasVisualizerTime = visualizerTimeSeconds > 0;
  if (hasAnalysisTime && hasVisualizerTime) {
    const analyzedFrameTime = Math.max(0, analysisTimeSeconds - ARTWORK_PULSE_START_LOOKBACK_SECONDS);
    const earliestCatchupTime = Math.max(0, visualizerTimeSeconds - ARTWORK_PULSE_MAX_START_CATCHUP_SECONDS);
    return Math.max(earliestCatchupTime, Math.min(visualizerTimeSeconds, analyzedFrameTime));
  }
  if (hasAnalysisTime) {
    return Math.max(0, analysisTimeSeconds - ARTWORK_PULSE_START_LOOKBACK_SECONDS);
  }
  if (hasVisualizerTime) {
    return Math.max(0, visualizerTimeSeconds - ARTWORK_PULSE_START_LOOKBACK_SECONDS);
  }
  return 0;
}

export function artworkPulseAttackSeconds() {
  return ARTWORK_PULSE_TIMING_CONFIG.attackSeconds;
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
  return ARTWORK_PULSE_TIMING_CONFIG.duration.baseSeconds +
    clampedEnergy * ARTWORK_PULSE_TIMING_CONFIG.duration.energySeconds;
}

function clampVisualizerUnit(value: number) {
  return Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : 0;
}
