/** The product currently exposes one Focus composition. */
export const LISTEN_LYRICS_FOCUS_STYLES = ["prism"] as const;

/**
 * Former persisted values remain part of the internal input contract so an
 * existing preference can be read safely and collapsed onto the single Focus
 * composition.
 */
const LISTEN_LYRICS_LEGACY_FOCUS_STYLES = [
  "prism",
  "splice",
  "facet",
  "pendulum",
] as const;

export type ListenLyricsFocusStyle =
  (typeof LISTEN_LYRICS_LEGACY_FOCUS_STYLES)[number];

export const DEFAULT_LISTEN_LYRICS_FOCUS_STYLE: ListenLyricsFocusStyle =
  "prism";

export type ListenLyricsFocusLayoutDensity =
  | "sparse"
  | "balanced"
  | "dense";

export type ListenLyricsFocusWordLayout = {
  index: number;
  text: string;
  direction: -1 | 1;
  phase: number;
  liftEm: number;
  depth: number;
};

export type ListenLyricsFocusLayout = {
  style: ListenLyricsFocusStyle;
  density: ListenLyricsFocusLayoutDensity;
  words: ListenLyricsFocusWordLayout[];
};

export type ListenLyricsFocusWordState = "pending" | "active" | "passed";

export type ListenLyricsFocusWordMotion = {
  prismOffsetEm: number;
  spliceShiftEm: number;
  facetAngleDeg: number;
  facetDepthEm: number;
  facetScale: number;
  pendulumAngleDeg: number;
  pendulumYEm: number;
  scale: number;
};

type ListenLyricsFocusLayoutUnit = {
  text: string;
};

/**
 * Every former stored style now resolves to the one supported Focus layout.
 */
export function normalizeListenLyricsFocusStyle(
  _value: unknown,
): ListenLyricsFocusStyle {
  return DEFAULT_LISTEN_LYRICS_FOCUS_STYLE;
}

export function isListenLyricsFocusStyle(
  value: unknown,
): value is ListenLyricsFocusStyle {
  return (
    typeof value === "string" &&
    (LISTEN_LYRICS_FOCUS_STYLES as readonly string[]).includes(value)
  );
}

/**
 * Produces deterministic, source-order metadata shared by the four effects.
 * Playback time is deliberately absent: timing may animate a word in its own
 * slot, but it can never reorder or reflow the sentence.
 */
export function buildListenLyricsFocusLayout(
  units: readonly ListenLyricsFocusLayoutUnit[],
  style: ListenLyricsFocusStyle,
): ListenLyricsFocusLayout {
  const normalizedStyle = normalizeListenLyricsFocusStyle(style);
  const density = resolveListenLyricsFocusLayoutDensity(units);
  return {
    style: normalizedStyle,
    density,
    words: units.map((unit, index) => {
      const text = String(unit.text ?? "");
      const seed = hashListenLyricsFocusUnit(`${index}\u0000${text}`);
      const direction = (seed & 1) === 0 ? -1 : 1;
      const phase = roundListenLyricsFocusLayoutValue(
        (((seed >>> 1) % 2001) - 1000) / 1000,
      );
      const liftEm = roundListenLyricsFocusLayoutValue(
        0.08 + ((seed >>> 12) % 101) / 1000,
      );
      const depth = roundListenLyricsFocusLayoutValue(
        0.82 + ((seed >>> 7) % 19) / 100,
      );
      return {
        index,
        text,
        direction,
        phase,
        liftEm,
        depth,
      };
    }),
  };
}

/** Maps the lyric clock to transform-only motion values; no layout reads. */
export function resolveListenLyricsFocusWordMotion(
  layout: ListenLyricsFocusWordLayout,
  state: ListenLyricsFocusWordState,
  progress: number,
): ListenLyricsFocusWordMotion {
  const unitProgress = clampListenLyricsFocusUnit(progress);
  const eased = 1 - Math.pow(1 - unitProgress, 3);
  const active = state === "active";
  const pending = state === "pending";
  const direction = layout.direction;
  const prismAmplitude = pending
    ? 0.016 + Math.abs(layout.phase) * 0.008
    : active
      ? 0.018 + Math.sin(unitProgress * Math.PI) * 0.052
      : 0.006;
  const spliceAmplitude = pending
    ? 0.115 + Math.abs(layout.phase) * 0.035
    : active
      ? (1 - eased) * 0.14
      : 0;
  const facetAngle = pending
    ? direction * (3.2 + layout.depth * 0.8)
    : active
      ? direction *
        ((1 - eased) * 4 - Math.sin(unitProgress * Math.PI) * 0.8)
      : 0;
  const facetDepth = pending
    ? 0.14 * layout.depth
    : active
      ? 0.14 * layout.depth * (1 - eased) - 0.05 * eased
      : -0.05;
  const facetScale = pending
    ? 1.1
    : active
      ? 1.1 - eased * 0.13
      : 0.97;
  const pendulumAngle = active
    ? direction *
      Math.sin(unitProgress * Math.PI * 4) *
      6 *
      Math.exp(-2.15 * unitProgress)
    : pending
      ? direction * (0.7 + Math.abs(layout.phase) * 0.8)
      : 0;
  const pendulumY = pending
    ? layout.liftEm
    : active
      ? layout.liftEm * (1 - eased) +
        Math.abs(Math.sin(unitProgress * Math.PI * 3)) *
          0.045 *
          (1 - unitProgress)
      : 0;
  const scale = pending
    ? 0.97
    : active
      ? 1 + Math.sin(unitProgress * Math.PI) * 0.055
      : 1;
  return {
    prismOffsetEm: roundListenLyricsFocusLayoutValue(
      direction * prismAmplitude,
    ),
    spliceShiftEm: roundListenLyricsFocusLayoutValue(
      direction * spliceAmplitude,
    ),
    facetAngleDeg: roundListenLyricsFocusLayoutValue(facetAngle),
    facetDepthEm: roundListenLyricsFocusLayoutValue(facetDepth),
    facetScale: roundListenLyricsFocusLayoutValue(facetScale),
    pendulumAngleDeg: roundListenLyricsFocusLayoutValue(pendulumAngle),
    pendulumYEm: roundListenLyricsFocusLayoutValue(pendulumY),
    scale: roundListenLyricsFocusLayoutValue(scale),
  };
}

function resolveListenLyricsFocusLayoutDensity(
  units: readonly ListenLyricsFocusLayoutUnit[],
): ListenLyricsFocusLayoutDensity {
  const graphemeCount = units.reduce(
    (total, unit) =>
      total + countListenLyricsFocusGraphemes(String(unit.text ?? "")),
    0,
  );
  if (units.length <= 4 && graphemeCount <= 18) {
    return "sparse";
  }
  if (units.length <= 10 && graphemeCount <= 48) {
    return "balanced";
  }
  return "dense";
}

function countListenLyricsFocusGraphemes(text: string) {
  return Array.from(text.normalize("NFC").replace(/\s+/gu, "")).length;
}

function hashListenLyricsFocusUnit(value: string) {
  let hash = 2166136261;
  for (const character of value) {
    hash ^= character.codePointAt(0) ?? 0;
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

function roundListenLyricsFocusLayoutValue(value: number) {
  return Math.round(value * 1000) / 1000;
}

function clampListenLyricsFocusUnit(value: number) {
  return Math.min(1, Math.max(0, Number.isFinite(value) ? value : 0));
}
