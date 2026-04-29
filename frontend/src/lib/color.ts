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

function relativeLuminance(rgb: { r: number; g: number; b: number }) {
  const channel = (value: number) => {
    const normalized = clamp(value) / 255;
    return normalized <= 0.03928
      ? normalized / 12.92
      : Math.pow((normalized + 0.055) / 1.055, 2.4);
  };

  return (
    channel(rgb.r) * 0.2126 +
    channel(rgb.g) * 0.7152 +
    channel(rgb.b) * 0.0722
  );
}

function contrastRatio(a: number, b: number) {
  const lighter = Math.max(a, b);
  const darker = Math.min(a, b);
  return (lighter + 0.05) / (darker + 0.05);
}

export function pickAccessibleForegroundForHslColor(color: HslColor) {
  const luminance = relativeLuminance(hslColorToRgb(color));
  const whiteContrast = contrastRatio(1, luminance);
  const blackContrast = contrastRatio(luminance, relativeLuminance({ r: 17, g: 17, b: 17 }));
  return blackContrast >= whiteContrast ? "#111111" : "#ffffff";
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
    sidebarAccent: toHslToken(sidebarAccent),
    sidebarAccentForeground: toHslToken(sidebarAccentForeground),
  };
}

export function pickAccessibleForeground(hex: string | undefined): string | null {
  if (!hex) return null;
  const match = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex.trim());
  if (!match) return null;
  const r = clamp(parseInt(match[1], 16));
  const g = clamp(parseInt(match[2], 16));
  const b = clamp(parseInt(match[3], 16));
  // YIQ contrast
  const yiq = (r * 299 + g * 587 + b * 114) / 1000;
  return yiq >= 150 ? "#111111" : "#ffffff";
}
