// Lightweight color helpers for dynamic theme color application.

function clamp(value: number, min = 0, max = 255) {
  return Math.min(Math.max(value, min), max);
}

function clampPercent(value: number, min = 0, max = 100) {
  return Math.min(Math.max(value, min), max);
}

export type HslColor = {
  h: number;
  s: number;
  l: number;
};

export const WCAG_CONTRAST = {
  smallText: 4.5,
  nonText: 3,
} as const;

export type FunctionalAccentTokens = {
  brand: string;
  text: string;
  solid: string;
  onSolid: string;
  ring: string;
};

function parseHexToRgb(hex: string | undefined) {
  if (!hex) return null;
  const normalized = hex.trim();
  const match = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(normalized);
  if (!match) return null;
  return {
    r: parseInt(match[1], 16) / 255,
    g: parseInt(match[2], 16) / 255,
    b: parseInt(match[3], 16) / 255,
  };
}

export function hexToHslColor(hex: string | undefined): HslColor | null {
  const rgb = parseHexToRgb(hex);
  if (!rgb) return null;

  const { r, g, b } = rgb;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  let h = 0;
  let s = 0;
  const l = (max + min) / 2;

  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    switch (max) {
      case r:
        h = (g - b) / d + (g < b ? 6 : 0);
        break;
      case g:
        h = (b - r) / d + 2;
        break;
      case b:
        h = (r - g) / d + 4;
        break;
    }
    h /= 6;
  }

  return {
    h: Math.round(h * 360),
    s: Math.round(s * 100),
    l: Math.round(l * 100),
  };
}

export function toHslToken(color: HslColor): string {
  return `${Math.round(color.h)} ${Math.round(color.s)}% ${Math.round(color.l)}%`;
}

function hslColorToRgb(color: HslColor) {
  const h = (((color.h % 360) + 360) % 360) / 360;
  const s = clampPercent(color.s) / 100;
  const l = clampPercent(color.l) / 100;

  if (s === 0) {
    const value = Math.round(l * 255);
    return { r: value, g: value, b: value };
  }

  const hueToRgb = (p: number, q: number, t: number) => {
    let next = t;
    if (next < 0) next += 1;
    if (next > 1) next -= 1;
    if (next < 1 / 6) return p + (q - p) * 6 * next;
    if (next < 1 / 2) return q;
    if (next < 2 / 3) return p + (q - p) * (2 / 3 - next) * 6;
    return p;
  };

  const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
  const p = 2 * l - q;

  return {
    r: Math.round(hueToRgb(p, q, h + 1 / 3) * 255),
    g: Math.round(hueToRgb(p, q, h) * 255),
    b: Math.round(hueToRgb(p, q, h - 1 / 3) * 255),
  };
}

export function parseHslToken(token: string | undefined): HslColor | null {
  if (!token) return null;
  const match = /^\s*(-?(?:\d+(?:\.\d+)?|\.\d+))\s+(-?(?:\d+(?:\.\d+)?|\.\d+))%\s+(-?(?:\d+(?:\.\d+)?|\.\d+))%\s*$/.exec(
    token,
  );
  if (!match) return null;
  return {
    h: Number(match[1]),
    s: clampPercent(Number(match[2])),
    l: clampPercent(Number(match[3])),
  };
}

export function relativeLuminance(rgb: { r: number; g: number; b: number }) {
  const channel = (value: number) => {
    const normalized = clamp(value) / 255;
    return normalized <= 0.04045
      ? normalized / 12.92
      : Math.pow((normalized + 0.055) / 1.055, 2.4);
  };

  return (
    channel(rgb.r) * 0.2126 +
    channel(rgb.g) * 0.7152 +
    channel(rgb.b) * 0.0722
  );
}

export function contrastRatio(a: number, b: number) {
  const lighter = Math.max(a, b);
  const darker = Math.min(a, b);
  return (lighter + 0.05) / (darker + 0.05);
}

export function contrastRatioForHslColors(a: HslColor, b: HslColor) {
  return contrastRatio(
    relativeLuminance(hslColorToRgb(a)),
    relativeLuminance(hslColorToRgb(b)),
  );
}

const lightForeground: HslColor = { h: 0, s: 0, l: 100 };
const darkForeground: HslColor = { h: 0, s: 0, l: 7 };

function foregroundForHslColor(color: HslColor) {
  const lightContrast = contrastRatioForHslColors(lightForeground, color);
  const darkContrast = contrastRatioForHslColors(darkForeground, color);
  return darkContrast >= lightContrast
    ? { color: darkForeground, hex: "#111111", contrast: darkContrast }
    : { color: lightForeground, hex: "#ffffff", contrast: lightContrast };
}

export function pickAccessibleForegroundForHslColor(color: HslColor) {
  return foregroundForHslColor(color).hex;
}

function normalizeContrastSurfaces(
  surfaces: HslColor | readonly HslColor[],
): readonly HslColor[] {
  return Array.isArray(surfaces) ? surfaces : [surfaces as HslColor];
}

function meetsContrastAgainstSurfaces(
  color: HslColor,
  surfaces: readonly HslColor[],
  minimum: number,
) {
  return surfaces.every(
    (surface) => contrastRatioForHslColors(color, surface) >= minimum,
  );
}

function closestColorByLightness(
  base: HslColor,
  predicate: (candidate: HslColor) => boolean,
  preferLighter: boolean,
) {
  const baseLightness = Math.round(clampPercent(base.l));
  const directions = preferLighter ? [1, -1] : [-1, 1];

  for (let distance = 0; distance <= 100; distance += 1) {
    for (const direction of directions) {
      const lightness = baseLightness + distance * direction;
      if (lightness < 0 || lightness > 100) continue;
      const candidate = { ...base, l: lightness };
      if (predicate(candidate)) return candidate;
      if (distance === 0) break;
    }
  }

  return { ...base, l: baseLightness };
}

function preferLighterAgainst(surfaces: readonly HslColor[]) {
  const averageLuminance =
    surfaces.reduce(
      (sum, surface) => sum + relativeLuminance(hslColorToRgb(surface)),
      0,
    ) / surfaces.length;
  return averageLuminance < 0.5;
}

/**
 * Derives distinct functional roles from a brand color. The generated text role
 * meets WCAG AA for normal text, the ring meets the non-text threshold, and the
 * solid role remains distinguishable from its surrounding surfaces while also
 * providing readable button labels.
 */
export function deriveFunctionalAccentTokens(
  hex: string | undefined,
  surfaces: HslColor | readonly HslColor[],
): FunctionalAccentTokens | null {
  const base = hexToHslColor(hex);
  if (!base) return null;

  const normalizedSurfaces = normalizeContrastSurfaces(surfaces);
  if (normalizedSurfaces.length === 0) return null;
  const preferLighter = preferLighterAgainst(normalizedSurfaces);

  const text = closestColorByLightness(
    base,
    (candidate) =>
      meetsContrastAgainstSurfaces(
        candidate,
        normalizedSurfaces,
        WCAG_CONTRAST.smallText,
      ),
    preferLighter,
  );
  const ring = closestColorByLightness(
    base,
    (candidate) =>
      meetsContrastAgainstSurfaces(
        candidate,
        normalizedSurfaces,
        WCAG_CONTRAST.nonText,
      ),
    preferLighter,
  );
  const solid = closestColorByLightness(
    base,
    (candidate) =>
      meetsContrastAgainstSurfaces(
        candidate,
        normalizedSurfaces,
        WCAG_CONTRAST.nonText,
      ) && foregroundForHslColor(candidate).contrast >= WCAG_CONTRAST.smallText,
    preferLighter,
  );
  const onSolid = foregroundForHslColor(solid).color;

  return {
    brand: toHslToken(base),
    text: toHslToken(text),
    solid: toHslToken(solid),
    onSolid: toHslToken(onSolid),
    ring: toHslToken(ring),
  };
}

export function hexToHsl(hex: string | undefined): string | null {
  const color = hexToHslColor(hex);
  return color ? toHslToken(color) : null;
}

export function deriveAccentTokens(hex: string | undefined, isDark: boolean) {
  const base = hexToHslColor(hex);
  if (!base) return null;

  const accent = {
    h: base.h,
    s: clampPercent(base.s * (isDark ? 0.46 : 0.58), isDark ? 18 : 24, isDark ? 62 : 78),
    l: isDark ? 22 : 95,
  };
  const accentForeground = {
    h: base.h,
    s: clampPercent(base.s * (isDark ? 0.7 : 0.9), isDark ? 24 : 32, 96),
    l: isDark ? 92 : 28,
  };
  const secondary = {
    h: base.h,
    s: clampPercent(
      base.s * (isDark ? 0.22 : 0.28),
      isDark ? 8 : 12,
      isDark ? 32 : 40,
    ),
    l: isDark ? 17 : 94,
  };
  const secondaryForeground = {
    h: base.h,
    s: clampPercent(base.s * (isDark ? 0.42 : 0.5), isDark ? 16 : 20, 72),
    l: isDark ? 92 : 22,
  };
  const sidebarAccent = {
    h: base.h,
    s: clampPercent(base.s * (isDark ? 0.36 : 0.5), isDark ? 14 : 18, isDark ? 48 : 64),
    l: isDark ? 18 : 90,
  };
  const sidebarAccentForeground = {
    h: base.h,
    s: clampPercent(base.s * (isDark ? 0.74 : 0.94), isDark ? 24 : 32, 96),
    l: isDark ? 92 : 26,
  };

  return {
    accent: toHslToken(accent),
    accentForeground: toHslToken(accentForeground),
    secondary: toHslToken(secondary),
    secondaryForeground: toHslToken(secondaryForeground),
    sidebarAccent: toHslToken(sidebarAccent),
    sidebarAccentForeground: toHslToken(sidebarAccentForeground),
  };
}

export function pickAccessibleForeground(hex: string | undefined): string | null {
  const color = hexToHslColor(hex);
  return color ? pickAccessibleForegroundForHslColor(color) : null;
}
