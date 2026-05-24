export type EqualizerBandType = "lowShelf" | "peaking" | "highShelf";

export interface EqualizerBand {
  id: string;
  frequencyHz: number;
  q: number;
  type: EqualizerBandType;
  display: string;
  displayHz: string;
  description?: string;
}

export interface EqualizerPreset {
  id: string;
  name: string;
  gainsDb: number[];
}

export type EqualizerArtworkVisualizerMode = "halo" | "ring" | "neonPulse" | "pondRipple";
export type EqualizerSpectrumVisualizerMode = "bars" | "mirror" | "waveform" | "heatmap";
export type EqualizerVisualizerMode = "off" | EqualizerArtworkVisualizerMode | EqualizerSpectrumVisualizerMode;
export type EqualizerVisualizerPlacement = "artwork" | "spectrum" | "off";

export const DEFAULT_EQUALIZER_ARTWORK_VISUALIZER_MODE: EqualizerArtworkVisualizerMode = "ring";
export const DEFAULT_EQUALIZER_SPECTRUM_VISUALIZER_MODE: EqualizerSpectrumVisualizerMode = "bars";
export const EQUALIZER_ARTWORK_VISUALIZER_MODES: readonly EqualizerArtworkVisualizerMode[] = [
  "halo",
  "ring",
  "neonPulse",
  "pondRipple",
];
export const EQUALIZER_SPECTRUM_VISUALIZER_MODES: readonly EqualizerSpectrumVisualizerMode[] = [
  "bars",
  "mirror",
  "waveform",
  "heatmap",
];

export function isEqualizerArtworkVisualizerMode(mode: EqualizerVisualizerMode): mode is EqualizerArtworkVisualizerMode {
  return (EQUALIZER_ARTWORK_VISUALIZER_MODES as readonly string[]).includes(mode);
}

export function isEqualizerSpectrumVisualizerMode(mode: EqualizerVisualizerMode): mode is EqualizerSpectrumVisualizerMode {
  return (EQUALIZER_SPECTRUM_VISUALIZER_MODES as readonly string[]).includes(mode);
}

export function equalizerVisualizerPlacement(mode: EqualizerVisualizerMode): EqualizerVisualizerPlacement {
  if (mode === "off") {
    return "off";
  }
  return isEqualizerArtworkVisualizerMode(mode) ? "artwork" : "spectrum";
}

export function equalizerVisualizerModeForPlacement(
  placement: EqualizerVisualizerPlacement,
  artworkMode: EqualizerArtworkVisualizerMode,
  spectrumMode: EqualizerSpectrumVisualizerMode,
): EqualizerVisualizerMode {
  if (placement === "off") {
    return "off";
  }
  return placement === "artwork" ? artworkMode : spectrumMode;
}

export interface EqualizerSettings {
  enabled: boolean;
  preampDb: number;
  bandGainsDb: number[];
  preset: string;
  visualizerMode: EqualizerVisualizerMode;
  visualizerPlacement: EqualizerVisualizerPlacement;
  artworkVisualizerMode: EqualizerArtworkVisualizerMode;
  spectrumVisualizerMode: EqualizerSpectrumVisualizerMode;
}

export type EqualizerStatusCode =
  | "off"
  | "active"
  | "standby"
  | "permission_needed"
  | "unsupported"
  | "error";

export interface EqualizerStatus {
  code: EqualizerStatusCode;
  running: boolean;
  supported: boolean;
  permissionRequired: boolean;
  message?: string;
  detail?: string;
}

export interface EqualizerSnapshot {
  settings: EqualizerSettings;
  status: EqualizerStatus;
  bands: EqualizerBand[];
  presets: EqualizerPreset[];
}

export interface EqualizerVisualizerFrame {
  running: boolean;
  sequence: number;
  level: number;
  bands: number[];
  waveform: number[];
  analysisTimeSeconds: number;
  frameTimeOffsetSeconds: number;
  receivedAtMs: number;
}
