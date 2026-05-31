import * as React from "react";

import {
  artworkPulseAttackSeconds,
  artworkPulseDurationSeconds,
  resolveArtworkPulseEnergyBaseline,
  resolveArtworkPulseProgress,
  resolveArtworkPulseReferenceEnergy,
  resolveArtworkPulseSpectralFlux,
  resolveArtworkPulseTimingDecision,
  resolveVisualizerCanvasPixelSize,
  resolveVisualizerAudioTime,
  type ArtworkPulseEvent,
} from "@/app/main/listen/visualizer-timing";
import { cn } from "@/lib/utils";
import {
  isEqualizerArtworkVisualizerMode,
  isEqualizerSpectrumVisualizerMode,
  type EqualizerVisualizerFrame,
  type EqualizerVisualizerMode,
} from "@/shared/contracts/equalizer";

type VisualizerVariant = "artwork" | "inline";
type ArtworkPulseVisualizerMode = "neonPulse" | "pondRipple";

type ArtworkFrameEnergyConfig = {
  averageWeight: number;
  fallbackLevelWeight: number;
  levelWeight: number;
  midAverageWeight: number;
  midEndRatio: number;
  midStartRatio: number;
  outputExponent?: number;
  peakWeight: number;
};

const ARTWORK_FRAME_ENERGY_CONFIG = {
  averageWeight: 1.9,
  fallbackLevelWeight: 1,
  levelWeight: 1.1,
  midAverageWeight: 1.55,
  midEndRatio: 0.72,
  midStartRatio: 0.22,
  outputExponent: 0.72,
  peakWeight: 0.72,
} as const satisfies ArtworkFrameEnergyConfig;

const ARTWORK_PULSE_RENDER_CONFIG = {
  cleanupGraceSeconds: 0.16,
  maxEvents: 7,
} as const;

const AMBIENT_HALO_CONFIG = {
  breath: {
    baselineAttackRatio: 0.028,
    baselineReleaseRatio: 0.14,
    body: {
      edgeEnd: 0.92,
      edgeStart: 0.14,
      exponent: 1.24,
      scale: 0.34,
    },
    frameEnergy: {
      averageWeight: 1.36,
      fallbackLevelWeight: 1,
      levelWeight: 0.92,
      midAverageWeight: 1.20,
      midEndRatio: 0.76,
      midStartRatio: 0.20,
      peakWeight: 0.62,
    },
    initial: {
      edgeEnd: 0.90,
      edgeStart: 0.12,
      exponent: 1.32,
      scale: 0.36,
    },
    transient: {
      edgeEnd: 0.20,
      edgeStart: 0.018,
      exponent: 0.82,
      instantRiseWeight: 0.80,
      relativeLiftWeight: 0.95,
      scale: 0.66,
      spectralFluxWeight: 0.86,
    },
  },
  glow: {
    base: 0.22,
    exponent: 0.82,
    range: 0.78,
  },
  style: {
    halo: {
      accentAlpha: { base: 0.070, breath: 0.12, fallbackBase: 0.14, fallbackGlow: 0.24, glow: 0.20 },
      accentBlur: { base: 18, breath: 12, fallback: 14 },
      accentSpread: { base: 5, breath: 5, fallback: 3 },
      opacity: { base: 0.38, breath: 0.24, fallbackBase: 0.34, fallbackGlow: 0.52, glow: 0.30 },
      secondaryAlpha: { base: 0.024, breath: 0.080, fallbackBase: 0.05, fallbackGlow: 0.16, glow: 0.12 },
      secondaryBlur: { base: 34, breath: 22, fallback: 20 },
      secondarySpread: { base: 10, breath: 8, fallback: 5 },
      tertiaryAlpha: { base: 0.016, breath: 0.065, fallbackBase: 0.04, fallbackGlow: 0.13, glow: 0.10 },
      tertiaryBlur: { base: 42, breath: 30, fallback: 24 },
      tertiarySpread: { base: 12, breath: 11, fallback: 6 },
    },
    rim: {
      opacity: { base: 0.12, breath: 0.18, fallbackBase: 0.12, fallbackGlow: 0.34, glow: 0.20 },
      shadowAlpha: { base: 0.036, breath: 0.10, fallbackBase: 0.08, fallbackGlow: 0.24, glow: 0.13 },
      shadowBlur: { base: 14, breath: 16, fallback: 18 },
    },
  },
} as const;

const NEON_PULSE_CONFIG = {
  eventEnergy: {
    chorusEdgeEnd: 0.72,
    chorusEdgeStart: 0.42,
    chorusExponent: 1.42,
    chorusScale: 0.46,
    climaxEnergyEdgeEnd: 0.94,
    climaxEnergyEdgeStart: 0.70,
    climaxEnergyExponent: 1.26,
    climaxScale: 0.96,
    climaxTransientEdgeEnd: 0.82,
    climaxTransientEdgeStart: 0.32,
    climaxTransientExponent: 0.92,
    fluxBase: 0.060,
    fluxRange: 0.22,
    fluxScale: 0.30,
    min: 0.14,
    quietEdgeEnd: 0.34,
    quietEdgeStart: 0.045,
    quietScale: 0.18,
    relativeBase: 0.012,
    relativeRange: 0.40,
    relativeScale: 0.32,
  },
  frameEnergy: {
    averageWeight: 1.46,
    fallbackLevelWeight: 0.62,
    levelWeight: 0.82,
    midAverageWeight: 1.20,
    midEndRatio: 0.72,
    midStartRatio: 0.24,
    peakWeight: 0.48,
  },
  response: {
    auraEdgeEnd: 0.58,
    auraEdgeStart: 0.30,
    auraExponent: 1.25,
    bloomEdgeEnd: 0.86,
    bloomEdgeStart: 0.64,
    bloomExponent: 2.28,
    burstEdgeEnd: 0.965,
    burstEdgeStart: 0.78,
    burstExponent: 2.58,
    expansionEdgeEnd: 0.92,
    expansionEdgeStart: 0.16,
    midGlowEdgeEnd: 0.58,
    midGlowEdgeStart: 0.38,
    midGlowExponent: 0.86,
    tubeEdgeEnd: 0.32,
    tubeEdgeStart: 0.04,
  },
  render: {
    alphaBase: 0.105,
    alphaEnergyEdgeEnd: 0.92,
    alphaEnergyEdgeStart: 0.08,
    alphaEnergyExponent: 1.22,
    alphaEnergyScale: 0.45,
    expansionProgressRange: 0.46,
    fadeExponentBase: 1.78,
    fadeExponentExpansion: 0.42,
    fadeOutBase: 0.48,
    fadeOutBurst: 0.08,
    glowOffsetAura: 0.003,
    glowOffsetBase: 0.002,
    glowOffsetBloom: 0.005,
    glowOffsetBurst: 0.006,
    scaleBase: 1.01,
  },
  sourceEnergy: {
    fullBodyEdgeEnd: 0.76,
    fullBodyEdgeStart: 0.045,
    fullBodyExponent: 1.08,
    quietKeyRatio: 0.55,
  },
  targetScale: {
    edgeEnd: 0.92,
    edgeStart: 0.16,
    max: 1.185,
    min: 1.075,
  },
  shadowBlurMaxRatio: {
    bloom: 0.086,
    inner: 0.044,
    outer: 0.058,
  },
} as const;

const POND_RIPPLE_CONFIG = {
  alpha: {
    base: 0.18,
    energy: 0.34,
    expansionBase: 0.72,
    expansion: 0.28,
    fadeExponent: 1.36,
    max: 0.72,
    secondaryMax: 0.34,
    secondaryScale: 0.66,
  },
  lineWidth: {
    baseRatio: 0.0042,
    energyRatio: 0.0012,
    secondaryRatio: 0.0030,
  },
  radius: {
    baseRatio: 0.505,
    initialScale: 0.985,
    secondaryMinRatio: 0.506,
    secondaryOffsetRatio: 0.032,
    targetBase: 1.015,
    targetEnergy: 0.17,
  },
  shadowBlur: {
    baseRatio: 0.012,
    energyRatio: 0.010,
  },
} as const;

type RenderedArtworkPulseEvent = ArtworkPulseEvent & {
  createdAtMs: number;
};
type ArtworkCanvasFrameMetrics = {
  x: number;
  y: number;
  width: number;
  height: number;
  radius: number;
};

function isAmbientHaloMode(mode: EqualizerVisualizerMode) {
  return mode === "halo" || mode === "neonPulse" || mode === "pondRipple";
}

function isArtworkPulseMode(mode: EqualizerVisualizerMode): mode is ArtworkPulseVisualizerMode {
  return mode === "neonPulse" || mode === "pondRipple";
}

export function ListenArtworkVisualizer(props: {
  mode: EqualizerVisualizerMode;
  frame: EqualizerVisualizerFrame;
  active: boolean;
  visible: boolean;
}) {
  const active = props.active && props.frame.running;
  const level = active ? props.frame.level : 0;
  const effectEnergy =
    isAmbientHaloMode(props.mode) && active
      ? resolveArtworkEnergy(props.frame)
      : level;
  const reducedMotion = usePrefersReducedMotion();
  const pulseActive =
    active &&
    props.visible &&
    !reducedMotion &&
    isArtworkPulseMode(props.mode);
  const audioClock = useVisualizerAudioClock(
    props.frame.analysisTimeSeconds,
    props.frame.receivedAtMs,
    props.frame.frameTimeOffsetSeconds,
  );
  const pulseEvents = useArtworkPulseEvents(
    props.mode,
    pulseActive,
    active ? resolveArtworkPulseSourceEnergy(props.mode, props.frame, effectEnergy) : 0,
    active ? props.frame.bands : [],
    props.frame.analysisTimeSeconds,
    audioClock.resolveNow,
  );
  const hasAmbientHalo = isAmbientHaloMode(props.mode);
  const glowEnergy = hasAmbientHalo ? resolveArtworkAmbientGlowEnergy(effectEnergy) : effectEnergy;
  const breathEnergy = useArtworkAmbientBreathEnergy(hasAmbientHalo && active, props.frame, effectEnergy);
  const style = resolveArtworkAmbientHaloStyle(hasAmbientHalo, {
    breathEnergy,
    effectEnergy,
    glowEnergy,
    level,
  });
  if (!isEqualizerArtworkVisualizerMode(props.mode) || !props.visible) {
    return null;
  }
  return (
    <div
      className={cn(
        "listen-artwork-visualizer pointer-events-none absolute z-[2] overflow-visible rounded-[2.85rem] text-[hsl(var(--primary))] transition-[transform] duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)]",
        hasAmbientHalo ? "-inset-16" : "inset-0",
      )}
      data-active={active ? "true" : "false"}
      data-mode={props.mode}
      style={style}
      aria-hidden="true"
    >
      <span className="listen-artwork-visualizer-ambient absolute inset-0 overflow-visible">
        <span
          className={cn(
            "listen-artwork-visualizer-halo absolute rounded-[2rem]",
            hasAmbientHalo ? "inset-24" : "inset-8",
          )}
        />
        <span
          className={cn(
            "listen-artwork-visualizer-rim absolute rounded-[2rem]",
            hasAmbientHalo ? "inset-24" : "inset-8",
          )}
        />
      </span>
      {isArtworkPulseMode(props.mode) ? (
        <ListenArtworkPulseCanvas
          mode={props.mode}
          active={pulseActive}
          events={pulseEvents}
          resolveVisualizerTimeSeconds={audioClock.resolveNow}
          className="listen-artwork-visualizer-pulse-canvas absolute inset-0 h-full w-full"
        />
      ) : null}
      {props.mode === "ring" ? (
        <ListenVisualizerCanvas
          mode={props.mode}
          frame={props.frame}
          active={props.active}
          variant="artwork"
          className="absolute inset-0 h-full w-full opacity-90"
        />
      ) : null}
    </div>
  );
}

function resolveArtworkAmbientHaloStyle(
  hasAmbientHalo: boolean,
  energy: {
    breathEnergy: number;
    effectEnergy: number;
    glowEnergy: number;
    level: number;
  },
) {
  const halo = AMBIENT_HALO_CONFIG.style.halo;
  const rim = AMBIENT_HALO_CONFIG.style.rim;
  return {
    "--listen-artwork-visualizer-ambient-breath": energy.breathEnergy.toFixed(3),
    "--listen-artwork-visualizer-effect-energy": energy.effectEnergy.toFixed(3),
    "--listen-artwork-visualizer-halo-accent-alpha": resolveAmbientAlpha(
      halo.accentAlpha,
      hasAmbientHalo,
      energy.glowEnergy,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-accent-blur": resolveAmbientPixels(
      halo.accentBlur,
      hasAmbientHalo,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-accent-spread": resolveAmbientPixels(
      halo.accentSpread,
      hasAmbientHalo,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-opacity": resolveAmbientAlpha(
      halo.opacity,
      hasAmbientHalo,
      energy.glowEnergy,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-secondary-alpha": resolveAmbientAlpha(
      halo.secondaryAlpha,
      hasAmbientHalo,
      energy.glowEnergy,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-secondary-blur": resolveAmbientPixels(
      halo.secondaryBlur,
      hasAmbientHalo,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-secondary-spread": resolveAmbientPixels(
      halo.secondarySpread,
      hasAmbientHalo,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-tertiary-alpha": resolveAmbientAlpha(
      halo.tertiaryAlpha,
      hasAmbientHalo,
      energy.glowEnergy,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-tertiary-blur": resolveAmbientPixels(
      halo.tertiaryBlur,
      hasAmbientHalo,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-halo-tertiary-spread": resolveAmbientPixels(
      halo.tertiarySpread,
      hasAmbientHalo,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-level": energy.level.toFixed(3),
    "--listen-artwork-visualizer-rim-opacity": resolveAmbientAlpha(
      rim.opacity,
      hasAmbientHalo,
      energy.glowEnergy,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-rim-shadow-alpha": resolveAmbientAlpha(
      rim.shadowAlpha,
      hasAmbientHalo,
      energy.glowEnergy,
      energy.breathEnergy,
    ),
    "--listen-artwork-visualizer-rim-shadow-blur": resolveAmbientPixels(
      rim.shadowBlur,
      hasAmbientHalo,
      energy.breathEnergy,
    ),
  } as React.CSSProperties;
}

function resolveAmbientAlpha(
  config: {
    base: number;
    breath: number;
    fallbackBase: number;
    fallbackGlow: number;
    glow: number;
  },
  hasAmbientHalo: boolean,
  glowEnergy: number,
  breathEnergy: number,
) {
  return formatVisualizerAlpha(
    hasAmbientHalo
      ? config.base + glowEnergy * config.glow + breathEnergy * config.breath
      : config.fallbackBase + glowEnergy * config.fallbackGlow,
  );
}

function resolveAmbientPixels(
  config: {
    base: number;
    breath: number;
    fallback: number;
  },
  hasAmbientHalo: boolean,
  breathEnergy: number,
) {
  return formatVisualizerPixels(hasAmbientHalo ? config.base + breathEnergy * config.breath : config.fallback);
}

function resolveArtworkEnergy(frame: EqualizerVisualizerFrame) {
  return resolveWeightedFrameEnergy(frame, ARTWORK_FRAME_ENERGY_CONFIG);
}

function resolveWeightedFrameEnergy(
  frame: EqualizerVisualizerFrame,
  config: ArtworkFrameEnergyConfig,
  fallbackEnergy?: number,
) {
  const bands = frame.bands.filter(Number.isFinite);
  if (bands.length === 0) {
    return clampVisualizerUnit((fallbackEnergy ?? frame.level) * config.fallbackLevelWeight);
  }
  const peak = bands.reduce((current, value) => Math.max(current, value), 0);
  const average = bands.reduce((sum, value) => sum + value, 0) / bands.length;
  const midStart = Math.floor(bands.length * config.midStartRatio);
  const midEnd = Math.max(midStart + 1, Math.ceil(bands.length * config.midEndRatio));
  const mids = bands.slice(midStart, midEnd);
  const midAverage = mids.reduce((sum, value) => sum + value, 0) / Math.max(1, mids.length);
  const energy = Math.max(
    frame.level * config.levelWeight,
    peak * config.peakWeight,
    average * config.averageWeight,
    midAverage * config.midAverageWeight,
  );
  const clampedEnergy = clampVisualizerUnit(energy);
  return config.outputExponent
    ? clampVisualizerUnit(Math.pow(clampedEnergy, config.outputExponent))
    : clampedEnergy;
}

function resolveNeonAmbientFrameEnergy(frame: EqualizerVisualizerFrame) {
  return resolveWeightedFrameEnergy(frame, NEON_PULSE_CONFIG.frameEnergy);
}

function resolveArtworkPulseSourceEnergy(
  mode: EqualizerVisualizerMode,
  frame: EqualizerVisualizerFrame,
  artworkEnergy: number,
) {
  if (mode !== "neonPulse") {
    return artworkEnergy;
  }
  const rawEnergy = resolveNeonAmbientFrameEnergy(frame);
  const source = NEON_PULSE_CONFIG.sourceEnergy;
  const quietKeyEnergy = rawEnergy * source.quietKeyRatio;
  const fullBodyEnergy = Math.pow(
    smoothStep(source.fullBodyEdgeStart, source.fullBodyEdgeEnd, rawEnergy),
    source.fullBodyExponent,
  );
  return clampVisualizerUnit(Math.max(quietKeyEnergy, fullBodyEnergy));
}

function resolveArtworkAmbientGlowEnergy(energy: number) {
  const clampedEnergy = clampVisualizerUnit(energy);
  const glow = AMBIENT_HALO_CONFIG.glow;
  return clampVisualizerUnit(glow.base + Math.pow(clampedEnergy, glow.exponent) * glow.range);
}

function useArtworkAmbientBreathEnergy(active: boolean, frame: EqualizerVisualizerFrame, fallbackEnergy: number) {
  const stateRef = React.useRef({
    baseline: 0,
    initialized: false,
    previousBands: [] as number[],
    previousEnergy: 0,
    sequence: -1,
    value: 0,
  });
  const state = stateRef.current;
  if (!active) {
    state.baseline = 0;
    state.initialized = false;
    state.previousBands = [];
    state.previousEnergy = 0;
    state.sequence = -1;
    state.value = 0;
    return 0;
  }
  if (state.sequence === frame.sequence) {
    return state.value;
  }

  const frameEnergy = resolveArtworkAmbientBreathFrameEnergy(frame, fallbackEnergy);
  const bands = frame.bands.map(clampVisualizerUnit);
  if (!state.initialized) {
    state.baseline = frameEnergy;
    state.initialized = true;
    state.previousBands = bands;
    state.previousEnergy = frameEnergy;
    state.sequence = frame.sequence;
    const initial = AMBIENT_HALO_CONFIG.breath.initial;
    state.value = Math.pow(
      smoothStep(initial.edgeStart, initial.edgeEnd, frameEnergy),
      initial.exponent,
    ) * initial.scale;
    return state.value;
  }

  const breath = AMBIENT_HALO_CONFIG.breath;
  const spectralFlux = resolveArtworkPulseSpectralFlux(bands, state.previousBands);
  const instantRise = Math.max(0, frameEnergy - state.previousEnergy);
  const relativeLift = Math.max(0, frameEnergy - state.baseline);
  const body = Math.pow(
    smoothStep(breath.body.edgeStart, breath.body.edgeEnd, frameEnergy),
    breath.body.exponent,
  ) * breath.body.scale;
  const transient = Math.pow(
    smoothStep(
      breath.transient.edgeStart,
      breath.transient.edgeEnd,
      relativeLift * breath.transient.relativeLiftWeight +
        instantRise * breath.transient.instantRiseWeight +
        spectralFlux * breath.transient.spectralFluxWeight,
    ),
    breath.transient.exponent,
  ) * breath.transient.scale;
  const nextBaselineRatio = frameEnergy > state.baseline
    ? breath.baselineAttackRatio
    : breath.baselineReleaseRatio;
  state.baseline += (frameEnergy - state.baseline) * nextBaselineRatio;
  state.previousBands = bands;
  state.previousEnergy = frameEnergy;
  state.sequence = frame.sequence;
  state.value = clampVisualizerUnit(body + transient);
  return state.value;
}

function resolveArtworkAmbientBreathFrameEnergy(frame: EqualizerVisualizerFrame, fallbackEnergy: number) {
  return resolveWeightedFrameEnergy(frame, AMBIENT_HALO_CONFIG.breath.frameEnergy, fallbackEnergy);
}

function useVisualizerAudioClock(analysisTimeSeconds: number, receivedAtMs: number, frameTimeOffsetSeconds: number) {
  const clockRef = React.useRef({ analysisTimeSeconds: 0, frameTimeOffsetSeconds: 0, receivedAtMs: 0 });

  if (
    analysisTimeSeconds > 0 &&
    receivedAtMs > 0 &&
    (
      clockRef.current.analysisTimeSeconds !== analysisTimeSeconds ||
      clockRef.current.frameTimeOffsetSeconds !== frameTimeOffsetSeconds ||
      clockRef.current.receivedAtMs !== receivedAtMs
    )
  ) {
    clockRef.current = {
      analysisTimeSeconds,
      frameTimeOffsetSeconds,
      receivedAtMs,
    };
  }

  const resolveNow = React.useCallback(() => {
    const clock = clockRef.current;
    const currentTimeMs = typeof performance !== "undefined" ? performance.now() : Date.now();
    return resolveVisualizerAudioTime(
      clock.analysisTimeSeconds,
      clock.receivedAtMs,
      currentTimeMs,
      undefined,
      clock.frameTimeOffsetSeconds,
    );
  }, []);

  return { resolveNow };
}

function usePrefersReducedMotion() {
  const [reducedMotion, setReducedMotion] = React.useState(false);

  React.useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const query = typeof window.matchMedia === "function"
      ? window.matchMedia("(prefers-reduced-motion: reduce)")
      : null;
    const sync = () => {
      const appMotionOff = document.documentElement.dataset.motion === "off";
      setReducedMotion(Boolean(query?.matches || appMotionOff));
    };
    sync();
    const observer = typeof MutationObserver === "function" ? new MutationObserver(sync) : null;
    observer?.observe(document.documentElement, { attributeFilter: ["data-motion"] });
    if (query && typeof query.addEventListener === "function") {
      query.addEventListener("change", sync);
      return () => {
        query.removeEventListener("change", sync);
        observer?.disconnect();
      };
    }
    if (query) {
      query.addListener(sync);
    }
    return () => {
      if (query) {
        query.removeListener(sync);
      }
      observer?.disconnect();
    };
  }, []);

  return reducedMotion;
}

function useArtworkPulseEvents(
  mode: EqualizerVisualizerMode,
  active: boolean,
  energy: number,
  bands: readonly number[],
  analysisTimeSeconds: number,
  resolveVisualizerTimeSeconds: () => number,
) {
  const [events, setEvents] = React.useState<RenderedArtworkPulseEvent[]>([]);
  const pulseRef = React.useRef({
    energyBaseline: 0,
    energyReference: 0,
    initialized: false,
    lastPulseTimeSeconds: -Number.POSITIVE_INFINITY,
    mode: "" as EqualizerVisualizerMode | "",
    nextId: 1,
    previousAnalysisTimeSeconds: 0,
    previousBands: [] as number[],
    previousEnergy: 0,
    tone: 0,
  });

  React.useEffect(() => {
    const state = pulseRef.current;
    if (state.mode !== mode) {
      state.energyBaseline = 0;
      state.energyReference = 0;
      state.initialized = false;
      state.lastPulseTimeSeconds = -Number.POSITIVE_INFINITY;
      state.mode = mode;
      state.previousAnalysisTimeSeconds = 0;
      state.previousBands = [];
      state.previousEnergy = 0;
      setEvents((current) => (current.length > 0 ? [] : current));
    }
    if (!isArtworkPulseMode(mode)) {
      state.energyBaseline = 0;
      state.energyReference = 0;
      state.initialized = false;
      state.previousAnalysisTimeSeconds = 0;
      state.previousBands = [];
      state.previousEnergy = 0;
      setEvents((current) => (current.length > 0 ? [] : current));
      return;
    }
    if (!active) {
      state.energyBaseline = 0;
      state.energyReference = 0;
      state.initialized = false;
      state.lastPulseTimeSeconds = -Number.POSITIVE_INFINITY;
      state.previousAnalysisTimeSeconds = 0;
      state.previousBands = [];
      state.previousEnergy = 0;
      setEvents((current) => (current.length > 0 ? [] : current));
      return;
    }

    const analysisRewound =
      state.previousAnalysisTimeSeconds > 0 &&
      analysisTimeSeconds > 0 &&
      analysisTimeSeconds + 0.25 < state.previousAnalysisTimeSeconds;
    if (analysisRewound) {
      state.energyBaseline = 0;
      state.energyReference = 0;
      state.initialized = false;
      state.lastPulseTimeSeconds = -Number.POSITIVE_INFINITY;
      state.previousBands = [];
      state.previousEnergy = 0;
      setEvents((current) => (current.length > 0 ? [] : current));
    }
    state.previousAnalysisTimeSeconds = analysisTimeSeconds;

    const clampedEnergy = clampVisualizerUnit(energy);
    const bandSnapshot = resolveArtworkPulseBandSnapshot(bands);
    if (!state.initialized) {
      state.energyBaseline = clampedEnergy;
      state.energyReference = clampedEnergy;
      state.previousBands = bandSnapshot;
      state.previousEnergy = clampedEnergy;
      state.initialized = true;
      return;
    }
    const spectralFlux = resolveArtworkPulseSpectralFlux(bandSnapshot, state.previousBands);
    const visualizerTimeSeconds = resolveVisualizerTimeSeconds();
    const energyBaseline = state.energyBaseline;
    const candidate = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds,
      energy: clampedEnergy,
      energyBaseline,
      lastPulseEnergy: state.energyReference,
      lastPulseTimeSeconds: state.lastPulseTimeSeconds,
      mode,
      previousEnergy: state.previousEnergy,
      spectralFlux,
      visualizerTimeSeconds,
    });
    state.energyBaseline = resolveArtworkPulseEnergyBaseline(clampedEnergy, state.energyBaseline);
    state.previousBands = bandSnapshot;
    state.previousEnergy = clampedEnergy;
    if (!candidate) {
      state.energyReference = resolveArtworkPulseReferenceEnergy(clampedEnergy, state.energyReference);
      return;
    }
    const eventEnergy = resolveArtworkPulseEventEnergy(mode, candidate.energy, energyBaseline, spectralFlux);
    const durationSeconds = artworkPulseDurationSeconds(eventEnergy);
    const event = {
      createdAtMs: performance.now(),
      durationSeconds,
      energy: eventEnergy,
      id: state.nextId,
      startTimeSeconds: candidate.startTimeSeconds,
      tone: state.tone % 3,
    };
    state.nextId += 1;
    state.tone += 1;
    state.energyReference = candidate.energy;
    state.lastPulseTimeSeconds = candidate.startTimeSeconds;
    setEvents((current) => [...current, event].slice(-ARTWORK_PULSE_RENDER_CONFIG.maxEvents));
  }, [
    active,
    analysisTimeSeconds,
    bands,
    energy,
    mode,
    resolveVisualizerTimeSeconds,
  ]);

  React.useEffect(() => {
    if (!active || events.length === 0) {
      return;
    }
    const nowMs = performance.now();
    const visualizerTimeSeconds = resolveVisualizerTimeSeconds();
    const nextRemainingSeconds = events.reduce((remainingSeconds, event) => {
      return Math.min(remainingSeconds, artworkPulseCleanupRemainingSeconds(event, visualizerTimeSeconds, nowMs));
    }, Number.POSITIVE_INFINITY);
    const timer = window.setTimeout(() => {
      const cleanupNowMs = performance.now();
      const cleanupVisualizerTimeSeconds = resolveVisualizerTimeSeconds();
      setEvents((current) => {
        const next = current.filter((event) => {
          return artworkPulseCleanupRemainingSeconds(event, cleanupVisualizerTimeSeconds, cleanupNowMs) > 0;
        });
        return next.length === current.length ? current : next;
      });
    }, Math.max(16, nextRemainingSeconds * 1000));
    return () => window.clearTimeout(timer);
  }, [active, events, resolveVisualizerTimeSeconds]);

  return events;
}

function resolveArtworkPulseEventEnergy(
  mode: EqualizerVisualizerMode,
  energy: number,
  energyBaseline: number,
  spectralFlux: number,
) {
  const clampedEnergy = clampVisualizerUnit(energy);
  if (mode !== "neonPulse") {
    return clampedEnergy;
  }
  const config = NEON_PULSE_CONFIG.eventEnergy;
  const quietLine = smoothStep(config.quietEdgeStart, config.quietEdgeEnd, clampedEnergy) * config.quietScale;
  const relativeLift = clampVisualizerUnit(
    (clampedEnergy - clampVisualizerUnit(energyBaseline) - config.relativeBase) / config.relativeRange,
  );
  const fluxLift = clampVisualizerUnit((clampVisualizerUnit(spectralFlux) - config.fluxBase) / config.fluxRange);
  const chorusLift =
    Math.pow(smoothStep(config.chorusEdgeStart, config.chorusEdgeEnd, clampedEnergy), config.chorusExponent) *
    config.chorusScale;
  const transientLift = Math.max(relativeLift, fluxLift);
  const climaxLift =
    Math.pow(
      smoothStep(config.climaxEnergyEdgeStart, config.climaxEnergyEdgeEnd, clampedEnergy),
      config.climaxEnergyExponent,
    ) *
    Math.pow(
      smoothStep(config.climaxTransientEdgeStart, config.climaxTransientEdgeEnd, transientLift),
      config.climaxTransientExponent,
    );
  return Math.max(
    config.min,
    quietLine,
    relativeLift * config.relativeScale,
    fluxLift * config.fluxScale,
    chorusLift,
    climaxLift * config.climaxScale,
  );
}

function resolveArtworkPulseBandSnapshot(bands: readonly number[]) {
  return bands.map(clampVisualizerUnit);
}

function artworkPulseCleanupRemainingSeconds(
  event: RenderedArtworkPulseEvent,
  visualizerTimeSeconds: number,
  nowMs: number,
) {
  if (visualizerTimeSeconds > 0) {
    return Math.max(
      0,
        event.startTimeSeconds +
        event.durationSeconds +
        ARTWORK_PULSE_RENDER_CONFIG.cleanupGraceSeconds -
        visualizerTimeSeconds,
    );
  }
  const wallElapsedSeconds = Math.max(0, (nowMs - event.createdAtMs) / 1000);
  return Math.max(0, event.durationSeconds + ARTWORK_PULSE_RENDER_CONFIG.cleanupGraceSeconds - wallElapsedSeconds);
}

function ListenArtworkPulseCanvas(props: {
  mode: ArtworkPulseVisualizerMode;
  active: boolean;
  events: RenderedArtworkPulseEvent[];
  resolveVisualizerTimeSeconds: () => number;
  className?: string;
}) {
  const canvasRef = React.useRef<HTMLCanvasElement | null>(null);
  const eventsRef = React.useRef(props.events);
  const activeRef = React.useRef(props.active);
  const modeRef = React.useRef(props.mode);
  const resolveVisualizerTimeSecondsRef = React.useRef(props.resolveVisualizerTimeSeconds);
  const wakeDrawRef = React.useRef<() => void>(() => {});

  React.useEffect(() => {
    eventsRef.current = props.events;
    activeRef.current = props.active;
    modeRef.current = props.mode;
    resolveVisualizerTimeSecondsRef.current = props.resolveVisualizerTimeSeconds;
    if (props.active || props.events.length > 0) {
      wakeDrawRef.current();
    }
  }, [props.active, props.events, props.mode, props.resolveVisualizerTimeSeconds]);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    let animationFrame = 0;
    let timer = 0;
    let disposed = false;
    const wakeDraw = () => {
      if (disposed) {
        return;
      }
      window.cancelAnimationFrame(animationFrame);
      window.clearTimeout(timer);
      animationFrame = window.requestAnimationFrame(draw);
    };
    const scheduleDraw = () => {
      if (activeRef.current || eventsRef.current.length > 0) {
        animationFrame = window.requestAnimationFrame(draw);
      } else {
        timer = window.setTimeout(() => {
          animationFrame = window.requestAnimationFrame(draw);
        }, 120);
      }
    };
    wakeDrawRef.current = wakeDraw;
    const draw = () => {
      const rect = canvas.getBoundingClientRect();
      const size = resolveCanvasPixelSize(rect);
      if (!size) {
        scheduleDraw();
        return;
      }
      const { width, height } = size;
      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width;
        canvas.height = height;
      }
      const context = canvas.getContext("2d");
      if (!context) {
        scheduleDraw();
        return;
      }
      context.clearRect(0, 0, width, height);
      const events = eventsRef.current;
      if (activeRef.current && events.length > 0) {
        const computed = window.getComputedStyle(canvas);
        const rootStyle = window.getComputedStyle(document.documentElement);
        const visualizerTimeSeconds = resolveVisualizerTimeSecondsRef.current();
        const nowMs = performance.now();
        const palette = resolveCanvasPalette(computed, rootStyle);
        const frame = resolveArtworkFrameCanvasMetrics(canvas, rect, width, height);
        if (modeRef.current === "neonPulse") {
          drawNeonPulseEvents(context, frame, events, visualizerTimeSeconds, nowMs, palette);
        } else {
          drawPondRippleEvents(context, frame, events, visualizerTimeSeconds, nowMs, palette);
        }
      }
      scheduleDraw();
    };
    draw();
    return () => {
      disposed = true;
      wakeDrawRef.current = () => {};
      window.cancelAnimationFrame(animationFrame);
      window.clearTimeout(timer);
    };
  }, []);

  return <canvas ref={canvasRef} className={cn("block", props.className)} aria-hidden="true" />;
}

type CanvasPalette = {
  accent: string;
  primary: string;
  secondary: string;
};

function resolveCanvasPalette(computed: CSSStyleDeclaration, rootStyle: CSSStyleDeclaration): CanvasPalette {
  const fallback = computed.color || "rgb(59, 130, 246)";
  return {
    primary: resolveCanvasHslTone(rootStyle, "--primary", fallback),
    accent: resolveCanvasHslTone(rootStyle, "--accent-foreground", fallback),
    secondary: resolveCanvasHslTone(rootStyle, "--chart-2", fallback),
  };
}

function resolveCanvasHslTone(style: CSSStyleDeclaration, token: string, fallback: string) {
  const value = style.getPropertyValue(token).trim();
  return value ? value : fallback;
}

function canvasTone(palette: CanvasPalette, tone: number) {
  const value = tone === 1 ? palette.accent : tone === 2 ? palette.secondary : palette.primary;
  if (value.startsWith("rgb") || value.startsWith("#") || value.startsWith("hsl(")) {
    return value;
  }
  return `hsl(${value})`;
}

function resolveArtworkPulseProgressForEvent(
  event: RenderedArtworkPulseEvent,
  visualizerTimeSeconds: number,
  nowMs: number,
) {
  if (visualizerTimeSeconds > 0) {
    return resolveArtworkPulseProgress(event.startTimeSeconds, event.durationSeconds, visualizerTimeSeconds);
  }
  return clampVisualizerUnit((nowMs - event.createdAtMs) / 1000 / event.durationSeconds);
}

function resolveArtworkPulseElapsedSecondsForEvent(
  event: RenderedArtworkPulseEvent,
  visualizerTimeSeconds: number,
  nowMs: number,
) {
  if (visualizerTimeSeconds > 0) {
    return Math.max(0, visualizerTimeSeconds - event.startTimeSeconds);
  }
  return Math.max(0, (nowMs - event.createdAtMs) / 1000);
}

function drawNeonPulseEvents(
  context: CanvasRenderingContext2D,
  frame: ArtworkCanvasFrameMetrics,
  events: RenderedArtworkPulseEvent[],
  visualizerTimeSeconds: number,
  nowMs: number,
  palette: CanvasPalette,
) {
  const base = Math.min(frame.width, frame.height);
  const centerX = frame.x + frame.width / 2;
  const centerY = frame.y + frame.height / 2;
  const baseSize = base;
  context.save();
  context.lineJoin = "round";
  context.lineCap = "round";
  context.globalCompositeOperation = "lighter";
  const render = NEON_PULSE_CONFIG.render;
  for (const event of events) {
    const progress = resolveArtworkPulseProgressForEvent(event, visualizerTimeSeconds, nowMs);
    if (progress <= 0 || progress >= 1) {
      continue;
    }
    const energy = clampVisualizerUnit(event.energy);
    const neon = resolveNeonPulseResponse(energy);
    const expansion = easeOutCubic(progress / render.expansionProgressRange);
    const fadeOutProgress = render.fadeOutBase + neon.burst * render.fadeOutBurst;
    const fadeProgress = clampVisualizerUnit(progress / fadeOutProgress);
    const fade = Math.pow(
      1 - fadeProgress,
      render.fadeExponentBase + neon.expansion * render.fadeExponentExpansion,
    );
    const targetScale = resolveNeonPulseTargetScale(energy);
    const scale = render.scaleBase + (targetScale - render.scaleBase) * expansion;
    const size = baseSize * scale;
    const x = centerX - size / 2;
    const y = centerY - size / 2;
    const radius = Math.max(0, frame.radius * (size / Math.max(1, base)));
    const alpha = (
      render.alphaBase +
      Math.pow(
        smoothStep(render.alphaEnergyEdgeStart, render.alphaEnergyEdgeEnd, energy),
        render.alphaEnergyExponent,
      ) * render.alphaEnergyScale
    ) * fade;
    if (alpha <= 0.01) {
      continue;
    }
    const tone = canvasTone(palette, event.tone);
    const sideTone = canvasTone(palette, event.tone === 2 ? 0 : event.tone + 1);
    const glowOffset = base * (
      render.glowOffsetBase +
      neon.aura * render.glowOffsetAura +
      neon.bloom * render.glowOffsetBloom +
      neon.burst * render.glowOffsetBurst
    );
    const glowX = x - glowOffset;
    const glowY = y - glowOffset;
    const glowSize = size + glowOffset * 2;
    const glowRadius = radius + glowOffset;

    if (neon.bloom > 0.01) {
      context.globalAlpha = Math.min(0.34, alpha * (neon.bloom * 0.56 + neon.burst * 0.36));
      context.strokeStyle = sideTone;
      context.shadowColor = sideTone;
      context.lineWidth = Math.max(1, base * (0.004 + neon.bloom * 0.026 + neon.burst * 0.014));
      context.shadowBlur = Math.min(
        base * (0.012 + neon.bloom * 0.046 + neon.burst * 0.024) * (0.32 + fade * 0.68),
        base * NEON_PULSE_CONFIG.shadowBlurMaxRatio.bloom,
      );
      roundRect(context, glowX, glowY, glowSize, glowSize, glowRadius);
      context.stroke();
    }

    if (neon.midGlow > 0.01) {
      context.globalAlpha = Math.min(0.34, alpha * (neon.midGlow * 0.38 + neon.bloom * 0.12));
      context.strokeStyle = tone;
      context.shadowColor = tone;
      context.lineWidth = Math.max(1, base * (0.003 + neon.midGlow * 0.010 + neon.bloom * 0.008));
      context.shadowBlur = Math.min(
        base * (0.014 + neon.midGlow * 0.036 + neon.bloom * 0.018) * (0.32 + fade * 0.68),
        base * NEON_PULSE_CONFIG.shadowBlurMaxRatio.outer,
      );
      roundRect(context, glowX, glowY, glowSize, glowSize, glowRadius);
      context.stroke();
    }

    if (neon.aura > 0.01) {
      context.globalAlpha = Math.min(0.34, alpha * (0.08 + neon.aura * 0.28 + neon.midGlow * 0.10));
      context.strokeStyle = tone;
      context.shadowColor = tone;
      context.lineWidth = Math.max(1, base * (0.002 + neon.aura * 0.006 + neon.midGlow * 0.004));
      context.shadowBlur = Math.min(
        base * (0.008 + neon.aura * 0.022 + neon.midGlow * 0.014) * (0.28 + fade * 0.72),
        base * NEON_PULSE_CONFIG.shadowBlurMaxRatio.outer,
      );
      roundRect(context, glowX, glowY, glowSize, glowSize, glowRadius);
      context.stroke();
    }

    context.globalAlpha = Math.min(0.72, alpha * (0.52 + neon.tube * 0.34));
    context.strokeStyle = tone;
    context.shadowColor = tone;
    context.lineWidth = Math.max(1, base * (0.0031 + neon.tube * 0.0038) * (0.62 + fade * 0.38));
    context.shadowBlur = Math.min(
      base * (0.0022 + neon.tube * 0.005 + neon.aura * 0.014) * (0.30 + fade * 0.70),
      base * NEON_PULSE_CONFIG.shadowBlurMaxRatio.inner,
    );
    roundRect(context, x, y, size, size, radius);
    context.stroke();

    context.globalAlpha = Math.min(0.48, alpha * (0.08 + neon.tube * 0.22 + neon.aura * 0.10) * (0.72 + fade * 0.28));
    context.strokeStyle = "rgba(255, 255, 255, 0.92)";
    context.shadowColor = "rgba(255, 255, 255, 0.82)";
    context.lineWidth = Math.max(1, base * (0.0008 + neon.tube * 0.0018));
    context.shadowBlur = Math.min(base * (0.001 + neon.tube * 0.004 + neon.aura * 0.006), base * 0.020);
    roundRect(context, x, y, size, size, radius);
    context.stroke();

    drawNeonPulseLightStreaks(context, x, y, size, radius, base, alpha, fade, neon, tone, sideTone);
  }
  context.restore();
}

function resolveNeonPulseResponse(energy: number) {
  const response = NEON_PULSE_CONFIG.response;
  const clampedEnergy = clampVisualizerUnit(energy);
  const tube = smoothStep(response.tubeEdgeStart, response.tubeEdgeEnd, clampedEnergy);
  const expansion = linearStep(response.expansionEdgeStart, response.expansionEdgeEnd, clampedEnergy);
  const aura = Math.pow(
    smoothStep(response.auraEdgeStart, response.auraEdgeEnd, clampedEnergy),
    response.auraExponent,
  );
  const midGlow =
    Math.pow(
      smoothStep(response.midGlowEdgeStart, response.midGlowEdgeEnd, clampedEnergy),
      response.midGlowExponent,
    );
  const bloom = Math.pow(
    smoothStep(response.bloomEdgeStart, response.bloomEdgeEnd, clampedEnergy),
    response.bloomExponent,
  );
  const burst = Math.pow(
    smoothStep(response.burstEdgeStart, response.burstEdgeEnd, clampedEnergy),
    response.burstExponent,
  );
  return { aura, bloom, burst, expansion, midGlow, tube };
}

function resolveNeonPulseTargetScale(energy: number) {
  const targetScale = NEON_PULSE_CONFIG.targetScale;
  const expansion = linearStep(targetScale.edgeStart, targetScale.edgeEnd, energy);
  return targetScale.min + (targetScale.max - targetScale.min) * expansion;
}

function drawNeonPulseLightStreaks(
  context: CanvasRenderingContext2D,
  x: number,
  y: number,
  size: number,
  radius: number,
  base: number,
  alpha: number,
  fade: number,
  neon: ReturnType<typeof resolveNeonPulseResponse>,
  tone: string,
  sideTone: string,
) {
  if (neon.aura <= 0.08) {
    return;
  }
  const inset = Math.max(radius * 0.55, base * 0.028);
  const run = Math.max(base * 0.10, size * 0.22);
  const lift = base * 0.026;
  context.lineWidth = Math.max(1, base * (0.001 + neon.bloom * 0.0018));
  context.shadowBlur = base * (0.003 + neon.aura * 0.010 + neon.bloom * 0.026);

  context.globalAlpha = Math.min(0.20, alpha * fade * (neon.aura * 0.10 + neon.bloom * 0.24));
  context.strokeStyle = tone;
  context.shadowColor = tone;
  context.beginPath();
  context.moveTo(x + inset, y - lift);
  context.lineTo(Math.min(x + size - inset, x + inset + run), y - lift);
  context.moveTo(x + size + lift, y + inset);
  context.lineTo(x + size + lift, Math.min(y + size - inset, y + inset + run * 0.88));
  context.stroke();

  context.globalAlpha = Math.min(0.16, alpha * fade * (neon.aura * 0.07 + neon.bloom * 0.20));
  context.strokeStyle = sideTone;
  context.shadowColor = sideTone;
  context.beginPath();
  context.moveTo(Math.max(x + inset, x + size - inset - run * 0.72), y + size + lift);
  context.lineTo(x + size - inset, y + size + lift);
  context.moveTo(x - lift, Math.max(y + inset, y + size - inset - run * 0.58));
  context.lineTo(x - lift, y + size - inset);
  context.stroke();
}

function resolveArtworkFrameCanvasMetrics(
  canvas: HTMLCanvasElement,
  rect: DOMRect,
  width: number,
  height: number,
) {
  const fallbackBase = Math.min(width, height) * 0.68;
  const fallback = {
    x: width / 2 - fallbackBase / 2,
    y: height / 2 - fallbackBase / 2,
    width: fallbackBase,
    height: fallbackBase,
    radius: fallbackBase * 0.11,
  };
  const shell = canvas.closest(".listen-artwork-shell");
  const frame = shell?.querySelector(".listen-artwork-frame");
  if (!(frame instanceof HTMLElement)) {
    return fallback;
  }
  const frameRect = frame.getBoundingClientRect();
  if (frameRect.width <= 0 || frameRect.height <= 0 || rect.width <= 0 || rect.height <= 0) {
    return fallback;
  }
  const pixelRatioX = width / rect.width;
  const pixelRatioY = height / rect.height;
  const frameStyle = window.getComputedStyle(frame);
  const cssRadius = parseCssPixelValue(frameStyle.borderTopLeftRadius);
  const frameWidth = frameRect.width * pixelRatioX;
  const frameHeight = frameRect.height * pixelRatioY;
  return {
    x: (frameRect.left - rect.left) * pixelRatioX,
    y: (frameRect.top - rect.top) * pixelRatioY,
    width: frameWidth,
    height: frameHeight,
    radius: cssRadius > 0 ? cssRadius * Math.min(pixelRatioX, pixelRatioY) : Math.min(frameWidth, frameHeight) * 0.11,
  };
}

function parseCssPixelValue(value: string) {
  const match = value.match(/-?\d*\.?\d+/);
  return match ? Math.max(0, Number.parseFloat(match[0])) : 0;
}

function drawPondRippleEvents(
  context: CanvasRenderingContext2D,
  frame: ArtworkCanvasFrameMetrics,
  events: RenderedArtworkPulseEvent[],
  visualizerTimeSeconds: number,
  nowMs: number,
  palette: CanvasPalette,
) {
  const base = Math.min(frame.width, frame.height);
  if (base <= 0) {
    return;
  }
  const centerX = frame.x + frame.width / 2;
  const centerY = frame.y + frame.height / 2;
  const baseRadius = base * POND_RIPPLE_CONFIG.radius.baseRatio;
  context.save();
  context.lineCap = "round";
  context.lineJoin = "round";
  context.globalCompositeOperation = "lighter";
  for (const event of events) {
    const progress = resolveArtworkPulseProgressForEvent(event, visualizerTimeSeconds, nowMs);
    if (progress <= 0 || progress >= 1) {
      continue;
    }
    const elapsedSeconds = resolveArtworkPulseElapsedSecondsForEvent(event, visualizerTimeSeconds, nowMs);
    const expansion = easeOutCubic(elapsedSeconds / artworkPulseAttackSeconds());
    const energy = clampVisualizerUnit(event.energy);
    const targetScale = resolvePondRippleTargetScale(energy);
    const radius = baseRadius * (
      POND_RIPPLE_CONFIG.radius.initialScale +
      (targetScale - POND_RIPPLE_CONFIG.radius.initialScale) * expansion
    );
    const secondaryRadius = Math.max(
      base * POND_RIPPLE_CONFIG.radius.secondaryMinRatio,
      radius - base * POND_RIPPLE_CONFIG.radius.secondaryOffsetRatio,
    );
    const alpha =
      (POND_RIPPLE_CONFIG.alpha.base + energy * POND_RIPPLE_CONFIG.alpha.energy) *
      Math.pow(1 - progress, POND_RIPPLE_CONFIG.alpha.fadeExponent) *
      (POND_RIPPLE_CONFIG.alpha.expansionBase + expansion * POND_RIPPLE_CONFIG.alpha.expansion);
    if (alpha <= 0.01) {
      continue;
    }
    const tone = canvasTone(palette, event.tone);
    context.strokeStyle = tone;
    context.shadowColor = canvasTone(palette, event.tone);
    context.shadowBlur = base * (
      POND_RIPPLE_CONFIG.shadowBlur.baseRatio +
      energy * POND_RIPPLE_CONFIG.shadowBlur.energyRatio
    );
    context.lineWidth = Math.max(
      1,
      base * (
        POND_RIPPLE_CONFIG.lineWidth.baseRatio +
        energy * POND_RIPPLE_CONFIG.lineWidth.energyRatio
      ),
    );
    context.globalAlpha = Math.min(POND_RIPPLE_CONFIG.alpha.max, alpha);
    drawIrregularRipplePath(context, centerX, centerY, radius, event.id, 1);
    context.stroke();

    context.globalAlpha = Math.min(
      POND_RIPPLE_CONFIG.alpha.secondaryMax,
      alpha * POND_RIPPLE_CONFIG.alpha.secondaryScale,
    );
    context.lineWidth = Math.max(1, base * POND_RIPPLE_CONFIG.lineWidth.secondaryRatio);
    drawIrregularRipplePath(context, centerX, centerY, secondaryRadius, event.id, -1);
    context.stroke();
  }
  context.restore();
}

function resolvePondRippleTargetScale(energy: number) {
  return POND_RIPPLE_CONFIG.radius.targetBase + clampVisualizerUnit(energy) * POND_RIPPLE_CONFIG.radius.targetEnergy;
}

function drawIrregularRipplePath(
  context: CanvasRenderingContext2D,
  centerX: number,
  centerY: number,
  radius: number,
  seed: number,
  direction: 1 | -1,
) {
  const segments = 96;
  const phase = seed * 1.618;
  context.beginPath();
  for (let index = 0; index <= segments; index += 1) {
    const angle = (index / segments) * Math.PI * 2;
    const wobble =
      Math.sin(angle * 3 + phase * direction) * 0.018 +
      Math.sin(angle * 5 - phase * 0.7) * 0.012 +
      Math.sin(angle * 9 + phase * 0.37) * 0.006;
    const r = radius * (1 + wobble);
    const x = centerX + Math.cos(angle) * r;
    const y = centerY + Math.sin(angle) * r;
    if (index === 0) {
      context.moveTo(x, y);
    } else {
      context.lineTo(x, y);
    }
  }
  context.closePath();
}

function easeOutCubic(value: number) {
  const clamped = clampVisualizerUnit(value);
  return 1 - Math.pow(1 - clamped, 3);
}

function smoothStep(edge0: number, edge1: number, value: number) {
  if (edge0 === edge1) {
    return value >= edge1 ? 1 : 0;
  }
  const progress = clampVisualizerUnit((value - edge0) / (edge1 - edge0));
  return progress * progress * (3 - 2 * progress);
}

function linearStep(edge0: number, edge1: number, value: number) {
  if (edge0 === edge1) {
    return value >= edge1 ? 1 : 0;
  }
  return clampVisualizerUnit((value - edge0) / (edge1 - edge0));
}

function clampVisualizerUnit(value: number) {
  return Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : 0;
}

function formatVisualizerAlpha(value: number) {
  return clampVisualizerUnit(value).toFixed(3);
}

function formatVisualizerPixels(value: number) {
  return `${Math.max(0, value).toFixed(2)}px`;
}

export function ListenInlineVisualizer(props: {
  mode: EqualizerVisualizerMode;
  frame: EqualizerVisualizerFrame;
  active: boolean;
  visible: boolean;
  className?: string;
}) {
  if (!isEqualizerSpectrumVisualizerMode(props.mode) || !props.visible) {
    return null;
  }
  return (
    <ListenVisualizerCanvas
      mode={props.mode}
      frame={props.frame}
      active={props.active}
      variant="inline"
      className={cn("h-full w-full text-[hsl(var(--primary))]", props.className)}
    />
  );
}

function ListenVisualizerCanvas(props: {
  mode: EqualizerVisualizerMode;
  frame: EqualizerVisualizerFrame;
  active: boolean;
  variant: VisualizerVariant;
  className?: string;
}) {
  const canvasRef = React.useRef<HTMLCanvasElement | null>(null);
  const frameRef = React.useRef(props.frame);
  const activeRef = React.useRef(props.active);
  const historyRef = React.useRef<number[][]>([]);
  const lastSequenceRef = React.useRef(0);

  React.useEffect(() => {
    historyRef.current = [];
    lastSequenceRef.current = 0;
  }, [props.mode]);

  React.useEffect(() => {
    frameRef.current = props.frame;
    activeRef.current = props.active && props.frame.running;
  }, [props.active, props.frame]);

  React.useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    let animationFrame = 0;
    let timer = 0;
    const scheduleDraw = (active: boolean) => {
      if (active) {
        animationFrame = window.requestAnimationFrame(draw);
      } else {
        timer = window.setTimeout(() => {
          animationFrame = window.requestAnimationFrame(draw);
        }, 120);
      }
    };
    const draw = () => {
      const rect = canvas.getBoundingClientRect();
      const size = resolveCanvasPixelSize(rect);
      if (!size) {
        scheduleDraw(activeRef.current);
        return;
      }
      const { width, height } = size;
      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width;
        canvas.height = height;
      }
      const context = canvas.getContext("2d");
      if (!context) {
        animationFrame = window.requestAnimationFrame(draw);
        return;
      }
      const computed = window.getComputedStyle(canvas);
      const color = computed.color || "rgb(59, 130, 246)";
      context.clearRect(0, 0, width, height);
      const frame = frameRef.current;
      const active = activeRef.current;
      if (props.mode === "ring") {
        drawRing(context, width, height, frame, color, active, props.variant);
      } else if (props.mode === "bars") {
        drawBars(context, width, height, frame, color, active, false);
      } else if (props.mode === "mirror") {
        drawBars(context, width, height, frame, color, active, true);
      } else if (props.mode === "waveform") {
        drawWaveform(context, width, height, frame, color, active);
      } else if (props.mode === "heatmap") {
        drawHeatmap(context, width, height, frame, color, active, historyRef, lastSequenceRef);
      }
      scheduleDraw(active);
    };
    draw();
    return () => {
      window.cancelAnimationFrame(animationFrame);
      window.clearTimeout(timer);
    };
  }, [props.mode, props.variant]);

  return (
    <canvas
      ref={canvasRef}
      className={cn("block", props.className)}
      aria-hidden="true"
    />
  );
}

function drawRing(
  context: CanvasRenderingContext2D,
  width: number,
  height: number,
  frame: EqualizerVisualizerFrame,
  color: string,
  active: boolean,
  variant: VisualizerVariant,
) {
  const centerX = width / 2;
  const centerY = height / 2;
  const base = Math.min(width, height);
  const radius = base * (variant === "artwork" ? 0.385 : 0.365);
  const maxBar = base * (variant === "artwork" ? 0.072 : 0.18);
  const bars = 128;
  const floor = variant === "artwork" ? 0.24 : 0.06;
  context.save();
  context.lineCap = "round";
  context.shadowBlur = base * (variant === "artwork" ? 0.024 : 0.065);
  context.shadowColor = color;
  context.strokeStyle = color;
  for (let index = 0; index < bars; index += 1) {
    const angle = -Math.PI / 2 + (index / bars) * Math.PI * 2;
    const band = active ? sampleBands(frame.bands, index / (bars - 1)) : 0;
    const level = Math.max(floor, band * 0.86 + (active ? frame.level : 0) * 0.14);
    const length = maxBar * (0.24 + level * 0.88);
    const inner = variant === "artwork" ? radius - length * 0.2 : radius - length * 0.18;
    const outer = radius + length;
    const opacity = variant === "artwork"
      ? Math.min(0.96, 0.42 + level * 0.48)
      : active
        ? 0.38 + level * 0.58
        : 0.18;
    context.globalAlpha = opacity;
    context.lineWidth = Math.max(1.5, base * (variant === "artwork" ? 0.01 : 0.0085));
    context.beginPath();
    context.moveTo(centerX + Math.cos(angle) * inner, centerY + Math.sin(angle) * inner);
    context.lineTo(centerX + Math.cos(angle) * outer, centerY + Math.sin(angle) * outer);
    context.stroke();
  }
  context.restore();
}

function resolveCanvasPixelSize(rect: DOMRect) {
  return resolveVisualizerCanvasPixelSize(rect.width, rect.height, window.devicePixelRatio || 1);
}

function drawBars(
  context: CanvasRenderingContext2D,
  width: number,
  height: number,
  frame: EqualizerVisualizerFrame,
  color: string,
  active: boolean,
  mirror: boolean,
) {
  const bands = frame.bands;
  const count = bands.length || 32;
  const gap = Math.max(2, width * 0.008);
  const barWidth = Math.max(2, (width - gap * (count + 1)) / count);
  const baseline = mirror ? height / 2 : height * 0.88;
  const maxHeight = mirror ? height * 0.42 : height * 0.72;
  const gradient = context.createLinearGradient(0, 0, width, 0);
  gradient.addColorStop(0, color);
  gradient.addColorStop(0.52, color);
  gradient.addColorStop(1, color);
  context.save();
  context.globalAlpha = active ? 0.8 : 0.22;
  context.fillStyle = gradient;
  for (let index = 0; index < count; index += 1) {
    const value = active ? bands[index] ?? 0 : 0.04;
    const eased = Math.pow(Math.max(0.025, value), 0.72);
    const x = gap + index * (barWidth + gap);
    const barHeight = Math.max(2, eased * maxHeight);
    if (mirror) {
      roundRect(context, x, baseline - barHeight, barWidth, barHeight * 2, barWidth / 2);
    } else {
      roundRect(context, x, baseline - barHeight, barWidth, barHeight, barWidth / 2);
    }
    context.fill();
  }
  context.restore();
}

function drawWaveform(
  context: CanvasRenderingContext2D,
  width: number,
  height: number,
  frame: EqualizerVisualizerFrame,
  color: string,
  active: boolean,
) {
  const waveform = active && frame.waveform.length > 1 ? frame.waveform : new Array<number>(64).fill(0);
  const centerY = height / 2;
  const amplitude = height * (active ? 0.34 : 0.08);
  context.save();
  context.lineWidth = Math.max(1.5, height * 0.05);
  context.lineCap = "round";
  context.lineJoin = "round";
  context.shadowColor = color;
  context.shadowBlur = height * 0.12;
  context.globalAlpha = active ? 0.82 : 0.26;
  context.strokeStyle = color;
  context.beginPath();
  waveform.forEach((sample, index) => {
    const x = (width * index) / (waveform.length - 1);
    const y = centerY - sample * amplitude;
    if (index === 0) {
      context.moveTo(x, y);
    } else {
      context.lineTo(x, y);
    }
  });
  context.stroke();
  context.restore();
}

function drawHeatmap(
  context: CanvasRenderingContext2D,
  width: number,
  height: number,
  frame: EqualizerVisualizerFrame,
  color: string,
  active: boolean,
  historyRef: React.MutableRefObject<number[][]>,
  lastSequenceRef: React.MutableRefObject<number>,
) {
  const columns = 44;
  if (frame.sequence !== lastSequenceRef.current) {
    lastSequenceRef.current = frame.sequence;
    historyRef.current = [
      normalizeBandsForHeatmap(frame.bands, active),
      ...historyRef.current,
    ].slice(0, columns);
  }
  const history = historyRef.current;
  const columnWidth = width / columns;
  const rowCount = 12;
  const rowHeight = height / rowCount;
  context.save();
  context.fillStyle = color;
  for (let column = 0; column < columns; column += 1) {
    const bands = history[column] ?? [];
    for (let row = 0; row < rowCount; row += 1) {
      const bandIndex = Math.floor((row / rowCount) * Math.max(1, bands.length - 1));
      const value = bands[bandIndex] ?? 0;
      if (value <= 0.01) {
        continue;
      }
      context.globalAlpha = 0.08 + value * 0.72;
      context.fillRect(
        width - (column + 1) * columnWidth,
        height - (row + 1) * rowHeight,
        Math.ceil(columnWidth) + 1,
        Math.ceil(rowHeight) + 1,
      );
    }
  }
  context.restore();
}

function normalizeBandsForHeatmap(bands: number[], active: boolean) {
  if (!active) {
    return new Array<number>(32).fill(0.04);
  }
  return bands.map((value) => Math.pow(Math.max(0, value), 0.78));
}

function sampleBands(bands: number[], position: number) {
  if (bands.length === 0) {
    return 0;
  }
  const scaled = Math.min(Math.max(position, 0), 1) * (bands.length - 1);
  const left = Math.floor(scaled);
  const right = Math.min(bands.length - 1, left + 1);
  const mix = scaled - left;
  return (bands[left] ?? 0) * (1 - mix) + (bands[right] ?? 0) * mix;
}

function roundRect(
  context: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
) {
  const safeRadius = Math.max(0, Math.min(radius, Math.abs(width) / 2, Math.abs(height) / 2));
  context.beginPath();
  context.moveTo(x + safeRadius, y);
  context.lineTo(x + width - safeRadius, y);
  context.quadraticCurveTo(x + width, y, x + width, y + safeRadius);
  context.lineTo(x + width, y + height - safeRadius);
  context.quadraticCurveTo(x + width, y + height, x + width - safeRadius, y + height);
  context.lineTo(x + safeRadius, y + height);
  context.quadraticCurveTo(x, y + height, x, y + height - safeRadius);
  context.lineTo(x, y + safeRadius);
  context.quadraticCurveTo(x, y, x + safeRadius, y);
  context.closePath();
}
