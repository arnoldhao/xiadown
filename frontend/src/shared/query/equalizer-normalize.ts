import {
  DEFAULT_EQUALIZER_ARTWORK_VISUALIZER_MODE,
  DEFAULT_EQUALIZER_SPECTRUM_VISUALIZER_MODE,
  equalizerVisualizerModeForPlacement,
  equalizerVisualizerPlacement,
  isEqualizerArtworkVisualizerMode,
  isEqualizerSpectrumVisualizerMode,
  type EqualizerArtworkVisualizerMode,
  type EqualizerSnapshot,
  type EqualizerSpectrumVisualizerMode,
  type EqualizerVisualizerFrame,
  type EqualizerVisualizerMode,
  type EqualizerVisualizerPlacement,
} from "@/shared/contracts/equalizer";

export function normalizeEqualizerSnapshot(raw: unknown): EqualizerSnapshot {
  const snapshot = raw as Partial<EqualizerSnapshot>;
  const settings = (snapshot.settings ?? {}) as Partial<EqualizerSnapshot["settings"]>;
  const status = (snapshot.status ?? {}) as Partial<EqualizerSnapshot["status"]>;
  const visualizerMode = normalizeVisualizerMode(settings.visualizerMode);
  const visualizerPlacement = normalizeVisualizerPlacement(settings.visualizerPlacement, visualizerMode);
  const artworkVisualizerMode = normalizeArtworkVisualizerMode(settings.artworkVisualizerMode, visualizerMode);
  const spectrumVisualizerMode = normalizeSpectrumVisualizerMode(settings.spectrumVisualizerMode, visualizerMode);
  return {
    settings: {
      enabled: settings.enabled === true,
      preampDb: numberOrZero(settings.preampDb),
      bandGainsDb: Array.isArray(settings.bandGainsDb) ? settings.bandGainsDb.map(numberOrZero) : [],
      preset: typeof settings.preset === "string" && settings.preset ? settings.preset : "flat",
      visualizerMode: equalizerVisualizerModeForPlacement(
        visualizerPlacement,
        artworkVisualizerMode,
        spectrumVisualizerMode,
      ),
      visualizerPlacement,
      artworkVisualizerMode,
      spectrumVisualizerMode,
    },
    status: {
      code: normalizeStatusCode(status.code),
      running: status.running === true,
      supported: status.supported === true,
      permissionRequired: status.permissionRequired === true,
      message: typeof status.message === "string" ? status.message : "",
      detail: typeof status.detail === "string" ? status.detail : "",
    },
    bands: Array.isArray(snapshot.bands) ? snapshot.bands : [],
    presets: Array.isArray(snapshot.presets) ? snapshot.presets : [],
  };
}

export function normalizeEqualizerVisualizerFrame(raw: unknown, receivedAtMs = 0): EqualizerVisualizerFrame {
  const frame = raw as Partial<EqualizerVisualizerFrame>;
  return {
    running: frame.running === true,
    sequence: nonNegativeNumberOrZero(frame.sequence),
    level: clampUnit(numberOrZero(frame.level)),
    bands: normalizeUnitArray(frame.bands, 32),
    waveform: normalizeWaveformArray(frame.waveform, 64),
    analysisTimeSeconds: nonNegativeNumberOrZero(frame.analysisTimeSeconds),
    frameTimeOffsetSeconds: clampFrameTimeOffset(frame.frameTimeOffsetSeconds),
    receivedAtMs: nonNegativeNumberOrZero(receivedAtMs || frame.receivedAtMs),
  };
}

export function emptyEqualizerVisualizerFrame(): EqualizerVisualizerFrame {
  return {
    running: false,
    sequence: 0,
    level: 0,
    bands: new Array<number>(32).fill(0),
    waveform: new Array<number>(64).fill(0),
    analysisTimeSeconds: 0,
    frameTimeOffsetSeconds: 0,
    receivedAtMs: 0,
  };
}

export function shouldCommitEqualizerVisualizerFrame(
  currentFrame: EqualizerVisualizerFrame,
  nextFrame: EqualizerVisualizerFrame,
) {
  return currentFrame.running !== nextFrame.running || currentFrame.sequence !== nextFrame.sequence;
}

export function resolveEqualizerVisualizerPollDelay(running: boolean, documentHidden: boolean) {
  if (documentHidden) {
    return running ? 250 : 1_000;
  }
  return running ? 33 : 120;
}

function normalizeStatusCode(value: unknown): EqualizerSnapshot["status"]["code"] {
  switch (value) {
    case "active":
    case "standby":
    case "permission_needed":
    case "unsupported":
    case "error":
    case "off":
      return value;
    default:
      return "unsupported";
  }
}

function numberOrZero(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function nonNegativeNumberOrZero(value: unknown) {
  return Math.max(0, numberOrZero(value));
}

function normalizeVisualizerMode(value: unknown): EqualizerVisualizerMode {
  switch (value) {
    case "off":
    case "halo":
    case "ring":
    case "neonPulse":
    case "pondRipple":
    case "bars":
    case "mirror":
    case "waveform":
    case "heatmap":
      return value;
    default:
      return "ring";
  }
}

function normalizeVisualizerPlacement(
  value: unknown,
  activeMode: EqualizerVisualizerMode,
): EqualizerVisualizerPlacement {
  switch (value) {
    case "artwork":
    case "spectrum":
    case "off":
      return value;
    default:
      return equalizerVisualizerPlacement(activeMode);
  }
}

function normalizeArtworkVisualizerMode(
  value: unknown,
  activeMode: EqualizerVisualizerMode,
): EqualizerArtworkVisualizerMode {
  const normalized = normalizeVisualizerMode(value);
  if (isEqualizerArtworkVisualizerMode(normalized)) {
    return normalized;
  }
  return isEqualizerArtworkVisualizerMode(activeMode) ? activeMode : DEFAULT_EQUALIZER_ARTWORK_VISUALIZER_MODE;
}

function normalizeSpectrumVisualizerMode(
  value: unknown,
  activeMode: EqualizerVisualizerMode,
): EqualizerSpectrumVisualizerMode {
  const normalized = normalizeVisualizerMode(value);
  if (isEqualizerSpectrumVisualizerMode(normalized)) {
    return normalized;
  }
  return isEqualizerSpectrumVisualizerMode(activeMode) ? activeMode : DEFAULT_EQUALIZER_SPECTRUM_VISUALIZER_MODE;
}

function normalizeUnitArray(value: unknown, length: number) {
  const result = new Array<number>(length).fill(0);
  if (!Array.isArray(value)) {
    return result;
  }
  value.slice(0, length).forEach((item, index) => {
    result[index] = clampUnit(numberOrZero(item));
  });
  return result;
}

function normalizeWaveformArray(value: unknown, length: number) {
  const result = new Array<number>(length).fill(0);
  if (!Array.isArray(value)) {
    return result;
  }
  value.slice(0, length).forEach((item, index) => {
    result[index] = clampWaveform(numberOrZero(item));
  });
  return result;
}

function clampUnit(value: number) {
  return Math.min(1, Math.max(0, value));
}

function clampWaveform(value: number) {
  return Math.min(1, Math.max(-1, value));
}

function clampFrameTimeOffset(value: unknown) {
  return Math.min(0.25, Math.max(-0.25, numberOrZero(value)));
}
