import * as React from "react";

import {
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
import { cn } from "@/lib/utils";
import {
  isEqualizerArtworkVisualizerMode,
  isEqualizerSpectrumVisualizerMode,
  type EqualizerVisualizerFrame,
  type EqualizerVisualizerMode,
} from "@/shared/contracts/equalizer";

type VisualizerVariant = "artwork" | "inline";
const MAX_ARTWORK_PULSE_EVENTS = 7;
const ARTWORK_PULSE_CLEANUP_GRACE_SECONDS = 0.16;
const NEON_PULSE_INNER_SHADOW_BLUR_MAX_RATIO = 0.032;
const NEON_PULSE_OUTER_SHADOW_BLUR_MAX_RATIO = 0.026;
type RenderedArtworkPulseEvent = ArtworkPulseEvent & {
  createdAtMs: number;
};

export function ListenArtworkVisualizer(props: {
  mode: EqualizerVisualizerMode;
  frame: EqualizerVisualizerFrame;
  active: boolean;
  visible: boolean;
}) {
  const active = props.active && props.frame.running;
  const level = active ? props.frame.level : 0;
  const effectEnergy =
    (props.mode === "halo" || props.mode === "neonPulse" || props.mode === "pondRipple") && active
      ? resolveArtworkEnergy(props.frame)
      : level;
  const reducedMotion = usePrefersReducedMotion();
  const pulseActive =
    active &&
    props.visible &&
    !reducedMotion &&
    (props.mode === "neonPulse" || props.mode === "pondRipple");
  const audioClock = useVisualizerAudioClock(
    props.frame.analysisTimeSeconds,
    props.frame.receivedAtMs,
    props.frame.frameTimeOffsetSeconds,
  );
  const pulseEvents = useArtworkPulseEvents(
    props.mode,
    pulseActive,
    active ? effectEnergy : 0,
    active ? props.frame.bands : [],
    props.frame.analysisTimeSeconds,
    audioClock.resolveNow,
  );
  const style = {
    "--listen-artwork-visualizer-effect-energy": effectEnergy.toFixed(3),
    "--listen-artwork-visualizer-halo-accent-alpha": formatVisualizerAlpha(0.14 + effectEnergy * 0.24),
    "--listen-artwork-visualizer-halo-opacity": formatVisualizerAlpha(0.34 + effectEnergy * 0.52),
    "--listen-artwork-visualizer-halo-secondary-alpha": formatVisualizerAlpha(0.05 + effectEnergy * 0.16),
    "--listen-artwork-visualizer-halo-tertiary-alpha": formatVisualizerAlpha(0.04 + effectEnergy * 0.13),
    "--listen-artwork-visualizer-level": level.toFixed(3),
    "--listen-artwork-visualizer-rim-opacity": formatVisualizerAlpha(0.12 + effectEnergy * 0.34),
    "--listen-artwork-visualizer-rim-shadow-alpha": formatVisualizerAlpha(0.08 + effectEnergy * 0.24),
  } as React.CSSProperties;
  if (!isEqualizerArtworkVisualizerMode(props.mode) || !props.visible) {
    return null;
  }
  return (
    <div
      className="listen-artwork-visualizer pointer-events-none absolute inset-0 z-[2] rounded-[2.85rem] text-[hsl(var(--primary))] transition-[transform] duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)]"
      data-active={active ? "true" : "false"}
      data-mode={props.mode}
      style={style}
      aria-hidden="true"
    >
      <span className="listen-artwork-visualizer-halo absolute inset-8 rounded-[2rem]" />
      <span className="listen-artwork-visualizer-rim absolute inset-8 rounded-[2rem]" />
      {props.mode === "neonPulse" || props.mode === "pondRipple" ? (
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

function resolveArtworkEnergy(frame: EqualizerVisualizerFrame) {
  const bands = frame.bands.filter(Number.isFinite);
  if (bands.length === 0) {
    return clampVisualizerUnit(frame.level);
  }
  const peak = bands.reduce((current, value) => Math.max(current, value), 0);
  const average = bands.reduce((sum, value) => sum + value, 0) / bands.length;
  const midStart = Math.floor(bands.length * 0.22);
  const midEnd = Math.max(midStart + 1, Math.ceil(bands.length * 0.72));
  const mids = bands.slice(midStart, midEnd);
  const midAverage = mids.reduce((sum, value) => sum + value, 0) / Math.max(1, mids.length);
  const energy = Math.max(frame.level * 1.1, peak * 0.72, average * 1.9, midAverage * 1.55);
  return Math.min(1, Math.pow(Math.max(0, energy), 0.72));
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
    lastPulseTimeSeconds: -Number.POSITIVE_INFINITY,
    mode: "" as EqualizerVisualizerMode | "",
    nextId: 1,
    previousBands: [] as number[],
    previousEnergy: 0,
    tone: 0,
  });

  React.useEffect(() => {
    const state = pulseRef.current;
    if (state.mode !== mode) {
      state.energyBaseline = 0;
      state.energyReference = 0;
      state.lastPulseTimeSeconds = -Number.POSITIVE_INFINITY;
      state.mode = mode;
      state.previousBands = [];
      state.previousEnergy = 0;
      setEvents((current) => (current.length > 0 ? [] : current));
    }
    if (mode !== "neonPulse" && mode !== "pondRipple") {
      state.energyBaseline = 0;
      state.energyReference = 0;
      state.previousBands = [];
      state.previousEnergy = 0;
      setEvents((current) => (current.length > 0 ? [] : current));
      return;
    }
    if (!active) {
      state.energyBaseline = 0;
      state.energyReference = 0;
      state.lastPulseTimeSeconds = -Number.POSITIVE_INFINITY;
      state.previousBands = [];
      state.previousEnergy = 0;
      setEvents((current) => (current.length > 0 ? [] : current));
      return;
    }

    const clampedEnergy = clampVisualizerUnit(energy);
    const bandSnapshot = resolveArtworkPulseBandSnapshot(bands);
    const spectralFlux = resolveArtworkPulseSpectralFlux(bandSnapshot, state.previousBands);
    const visualizerTimeSeconds = resolveVisualizerTimeSeconds();
    const candidate = resolveArtworkPulseTimingDecision({
      analysisTimeSeconds,
      energy: clampedEnergy,
      energyBaseline: state.energyBaseline,
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
    const eventEnergy = Math.max(0.16, candidate.energy);
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
    setEvents((current) => [...current, event].slice(-MAX_ARTWORK_PULSE_EVENTS));
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
        ARTWORK_PULSE_CLEANUP_GRACE_SECONDS -
        visualizerTimeSeconds,
    );
  }
  const wallElapsedSeconds = Math.max(0, (nowMs - event.createdAtMs) / 1000);
  return Math.max(0, event.durationSeconds + ARTWORK_PULSE_CLEANUP_GRACE_SECONDS - wallElapsedSeconds);
}

function ListenArtworkPulseCanvas(props: {
  mode: "neonPulse" | "pondRipple";
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
        if (modeRef.current === "neonPulse") {
          const cornerRadius = resolveArtworkFrameCanvasRadius(canvas, rect, width, height);
          drawNeonPulseEvents(context, width, height, events, visualizerTimeSeconds, nowMs, palette, cornerRadius);
        } else {
          drawPondRippleEvents(context, width, height, events, visualizerTimeSeconds, nowMs, palette);
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
  width: number,
  height: number,
  events: RenderedArtworkPulseEvent[],
  visualizerTimeSeconds: number,
  nowMs: number,
  palette: CanvasPalette,
  cornerRadius: number,
) {
  const base = Math.min(width, height);
  const centerX = width / 2;
  const centerY = height / 2;
  const baseSize = base * 0.62;
  const fadeOutProgress = 0.82;
  context.save();
  context.lineJoin = "round";
  for (const event of events) {
    const progress = resolveArtworkPulseProgressForEvent(event, visualizerTimeSeconds, nowMs);
    if (progress <= 0 || progress >= 1) {
      continue;
    }
    const expansion = easeOutCubic(progress);
    const fadeProgress = clampVisualizerUnit(progress / fadeOutProgress);
    const fade = Math.pow(1 - fadeProgress, 1.48);
    const energy = clampVisualizerUnit(event.energy);
    const scale = 0.985 + (resolveArtworkPulseTargetScale(energy) - 0.985) * expansion;
    const size = baseSize * scale;
    const x = centerX - size / 2;
    const y = centerY - size / 2;
    const radius = Math.max(0, cornerRadius);
    const alpha = (0.24 + energy * 0.46) * fade;
    if (alpha <= 0.01) {
      continue;
    }
    context.globalAlpha = Math.min(0.82, alpha);
    context.strokeStyle = canvasTone(palette, event.tone);
    context.shadowColor = canvasTone(palette, event.tone);
    context.lineWidth = Math.max(1, base * (0.0032 + energy * 0.0022) * (0.62 + fade * 0.38));
    context.shadowBlur = Math.min(
      base * (0.014 + energy * 0.024) * (0.30 + fade * 0.70),
      base * NEON_PULSE_INNER_SHADOW_BLUR_MAX_RATIO,
    );
    roundRect(context, x, y, size, size, radius);
    context.stroke();

    context.globalAlpha = Math.min(0.42, alpha * (0.38 + fade * 0.20));
    const glowOffset = base * 0.006;
    const glowX = x - glowOffset;
    const glowY = y - glowOffset;
    const glowSize = size + glowOffset * 2;
    context.shadowBlur = Math.min(
      base * (0.030 + energy * 0.035) * (0.28 + fade * 0.72),
      base * NEON_PULSE_OUTER_SHADOW_BLUR_MAX_RATIO,
    );
    roundRect(context, glowX, glowY, glowSize, glowSize, radius);
    context.stroke();
  }
  context.restore();
}

function resolveArtworkFrameCanvasRadius(
  canvas: HTMLCanvasElement,
  rect: DOMRect,
  width: number,
  height: number,
) {
  const fallback = Math.min(width, height) * 0.082;
  const shell = canvas.closest(".listen-artwork-shell");
  const frame = shell?.querySelector(".listen-artwork-frame");
  if (!(frame instanceof HTMLElement)) {
    return fallback;
  }
  const frameStyle = window.getComputedStyle(frame);
  const cssRadius = parseCssPixelValue(frameStyle.borderTopLeftRadius);
  if (cssRadius <= 0) {
    return fallback;
  }
  const pixelRatio = rect.width > 0 && rect.height > 0
    ? Math.min(width / rect.width, height / rect.height)
    : window.devicePixelRatio || 1;
  return cssRadius * pixelRatio;
}

function parseCssPixelValue(value: string) {
  const match = value.match(/-?\d*\.?\d+/);
  return match ? Math.max(0, Number.parseFloat(match[0])) : 0;
}

function drawPondRippleEvents(
  context: CanvasRenderingContext2D,
  width: number,
  height: number,
  events: RenderedArtworkPulseEvent[],
  visualizerTimeSeconds: number,
  nowMs: number,
  palette: CanvasPalette,
) {
  const base = Math.min(width, height);
  const centerX = width / 2;
  const centerY = height / 2;
  const baseRadius = base * 0.305;
  context.save();
  context.lineCap = "round";
  context.lineJoin = "round";
  for (const event of events) {
    const progress = resolveArtworkPulseProgressForEvent(event, visualizerTimeSeconds, nowMs);
    if (progress <= 0 || progress >= 1) {
      continue;
    }
    const elapsedSeconds = resolveArtworkPulseElapsedSecondsForEvent(event, visualizerTimeSeconds, nowMs);
    const expansion = easeOutCubic(elapsedSeconds / artworkPulseAttackSeconds());
    const energy = clampVisualizerUnit(event.energy);
    const targetScale = resolveArtworkPulseTargetScale(energy);
    const radius = baseRadius * (0.92 + (targetScale - 0.92) * expansion);
    const alpha = (0.12 + energy * 0.32) * Math.pow(1 - progress, 1.46) * (0.70 + expansion * 0.30);
    if (alpha <= 0.01) {
      continue;
    }
    const tone = canvasTone(palette, event.tone);
    context.strokeStyle = tone;
    context.shadowColor = canvasTone(palette, event.tone);
    context.shadowBlur = base * 0.010;
    context.lineWidth = Math.max(1, base * 0.0038);
    context.globalAlpha = Math.min(0.64, alpha);
    drawIrregularRipplePath(context, centerX, centerY, radius, event.id, 1);
    context.stroke();

    context.globalAlpha = Math.min(0.28, alpha * 0.58);
    context.lineWidth = Math.max(1, base * 0.0026);
    drawIrregularRipplePath(context, centerX, centerY, radius * 0.82, event.id, -1);
    context.stroke();
  }
  context.restore();
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

function clampVisualizerUnit(value: number) {
  return Number.isFinite(value) ? Math.min(1, Math.max(0, value)) : 0;
}

function formatVisualizerAlpha(value: number) {
  return clampVisualizerUnit(value).toFixed(3);
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
