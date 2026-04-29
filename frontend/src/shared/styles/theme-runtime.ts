import { System } from "@wailsio/runtime";
import type * as React from "react";

import {
  deriveAccentTokens,
  hexToHsl,
  hexToHslColor,
  pickAccessibleForeground,
  pickAccessibleForegroundForHslColor,
  toHslToken,
} from "@/lib/color";
import type { Settings } from "@/shared/contracts/settings";
import {
  readXiaAppearance,
  resolveThemePack,
} from "@/shared/styles/xiadown-theme";

function isWailsRuntimeReady() {
  return typeof window !== "undefined" && typeof (window as any)._wails?.dispatchWailsEvent === "function";
}

function applyTheme(effectiveAppearance: string | undefined) {
  if (effectiveAppearance === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
}

function clearColorScheme() {
  delete document.documentElement.dataset.colorScheme;
}

function systemFontStack() {
  return [
    "system-ui",
    "-apple-system",
    "BlinkMacSystemFont",
    '"Segoe UI"',
    "Roboto",
    '"Noto Sans"',
    "Ubuntu",
    "Cantarell",
    '"Helvetica Neue"',
    "Arial",
    '"Apple Color Emoji"',
    '"Segoe UI Emoji"',
    '"Segoe UI Symbol"',
    "sans-serif",
  ].join(", ");
}

function quoteFontFamily(value: string) {
  const escaped = value.replace(/\\/g, "\\\\").replace(/\"/g, '\\"');
  return `"${escaped}"`;
}

function buildFontStack(fontFamily: string | undefined) {
  const trimmed = (fontFamily ?? "").trim();
  if (!trimmed) {
    return systemFontStack();
  }
  return `${quoteFontFamily(trimmed)}, ${systemFontStack()}`;
}

function applyFont(fontFamily: string | undefined) {
  const stack = buildFontStack(fontFamily);
  document.documentElement.style.setProperty("--app-font-body", stack);
  document.documentElement.style.setProperty("--app-font-display", stack);
}

function applyFontSize(fontSize: number | undefined) {
  const safeSize = fontSize && fontSize > 0 ? fontSize : 15;
  document.documentElement.style.setProperty("--app-font-size", `${safeSize}px`);
}

function resolveThemeColor(themeColor: string | undefined, systemThemeColor: string | undefined) {
  const trimmed = (themeColor ?? "").trim();
  if (trimmed.toLowerCase() === "system") {
    return (systemThemeColor ?? "").trim();
  }
  return trimmed;
}

function clampPercent(value: number, min = 0, max = 100) {
  return Math.min(Math.max(value, min), max);
}

function deriveDreamFMTrayControlTokens(color: string | undefined, isDark: boolean) {
  const base = hexToHslColor(color);
  if (!base) {
    return null;
  }

  const surface = isDark
    ? {
        h: base.h,
        s: clampPercent(base.s * 0.62, 28, 70),
        l: clampPercent(base.l + 5, 38, 58),
      }
    : {
        h: base.h,
        s: clampPercent(base.s * 0.68, 30, 72),
        l: clampPercent(base.l + 11, 66, 78),
      };
  const foreground = pickAccessibleForegroundForHslColor(surface);

  return {
    line: toHslToken(base),
    surface: toHslToken(surface),
    foreground: hexToHsl(foreground) ?? "0 0% 100%",
  };
}

function resolveIsDarkAppearance(effectiveAppearance?: string) {
  if (effectiveAppearance) {
    return effectiveAppearance === "dark";
  }
  return document.documentElement.classList.contains("dark");
}

function applyPrimaryColor(
  color: string | undefined,
  systemThemeColor?: string,
  effectiveAppearance?: string,
) {
  const resolved = resolveThemeColor(color, systemThemeColor);
  const hsl = hexToHsl(resolved);
  const fgHex = pickAccessibleForeground(resolved);
  const fgHsl = hexToHsl(fgHex ?? undefined);
  const trayTokens = deriveDreamFMTrayControlTokens(
    resolved,
    resolveIsDarkAppearance(effectiveAppearance),
  );

  if (!hsl || !fgHsl || !trayTokens) {
    return;
  }

  document.documentElement.style.setProperty("--primary", hsl);
  document.documentElement.style.setProperty("--primary-foreground", fgHsl);
  document.documentElement.style.setProperty("--dreamfm-hover-line", trayTokens.line);
  document.documentElement.style.setProperty("--tray-control-color", trayTokens.surface);
  document.documentElement.style.setProperty("--tray-control-foreground", trayTokens.foreground);
  document.documentElement.style.setProperty("--ring", hsl);
  document.documentElement.style.setProperty("--sidebar-primary", hsl);
  document.documentElement.style.setProperty("--sidebar-primary-foreground", fgHsl);
  document.documentElement.style.setProperty("--sidebar-ring", hsl);
}

function applyThemeColor(
  themeColor: string | undefined,
  systemThemeColor: string | undefined,
  effectiveAppearance: string | undefined,
  accentMode: string | undefined,
) {
  if (accentMode !== "color") {
    return;
  }

  const color = resolveThemeColor(themeColor, systemThemeColor);
  const accentTokens = deriveAccentTokens(color, effectiveAppearance === "dark");

  if (!accentTokens) {
    return;
  }

  applyPrimaryColor(color, undefined, effectiveAppearance);
  document.documentElement.style.setProperty("--accent", accentTokens.accent);
  document.documentElement.style.setProperty("--accent-foreground", accentTokens.accentForeground);
  document.documentElement.style.setProperty("--sidebar-accent", accentTokens.sidebarAccent);
  document.documentElement.style.setProperty("--sidebar-accent-foreground", accentTokens.sidebarAccentForeground);
}

function applyThemePack(themePackId: string | undefined, effectiveAppearance: string | undefined) {
  const pack = resolveThemePack(themePackId);
  const variant = effectiveAppearance === "dark" ? pack.dark : pack.light;
  Object.entries(variant).forEach(([key, value]) => {
    document.documentElement.style.setProperty(`--${key}`, value);
  });
  document.documentElement.dataset.xiadownThemePack = pack.id;
}

function detectPlatform() {
  try {
    if (isWailsRuntimeReady()) {
      if (System.IsWindows()) {
        return "windows";
      }
      if (System.IsMac()) {
        return "macos";
      }
      if (System.IsLinux()) {
        return "linux";
      }
    }
  } catch {
    // Runtime platform detection falls back to the browser UA below.
  }

  const platform = typeof navigator === "undefined" ? "" : `${navigator.platform} ${navigator.userAgent}`.toLowerCase();
  if (platform.includes("win")) {
    return "windows";
  }
  if (platform.includes("mac")) {
    return "macos";
  }
  if (platform.includes("linux")) {
    return "linux";
  }
  return "unknown";
}

export function applyPlatformChrome() {
  document.documentElement.dataset.platform = detectPlatform();
}

export function applyXiaTheme(settings: Settings) {
  const appearance = readXiaAppearance(settings);
  applyTheme(settings.effectiveAppearance);
  applyThemePack(appearance.themePackId, settings.effectiveAppearance);
  applyPrimaryColor(
    resolveThemePack(appearance.themePackId).preview.accent,
    undefined,
    settings.effectiveAppearance,
  );
  clearColorScheme();
  applyFont(settings.fontFamily);
  applyFontSize(settings.fontSize);
  applyThemeColor(
    settings.themeColor,
    settings.systemThemeColor,
    settings.effectiveAppearance,
    appearance.accentMode,
  );
}

export function applyXiaAppearanceChange(
  effectiveAppearance: string | undefined,
  settings?: Settings | null,
) {
  applyTheme(effectiveAppearance);
  if (!settings) {
    return;
  }
  const appearance = readXiaAppearance(settings);
  applyThemePack(appearance.themePackId, effectiveAppearance);
  applyPrimaryColor(
    resolveThemePack(appearance.themePackId).preview.accent,
    undefined,
    effectiveAppearance,
  );
  applyThemeColor(
    settings.themeColor,
    settings.systemThemeColor,
    effectiveAppearance,
    appearance.accentMode,
  );
}

function resolveTrayThemeColor(settings?: Settings | null) {
  const appearance = readXiaAppearance(settings);
  const packColor = resolveThemePack(appearance.themePackId).preview.accent;
  const themeColor = (settings?.themeColor ?? "").trim();
  const configuredColor =
    appearance.accentMode === "color"
      ? themeColor.toLowerCase() === "system"
        ? (settings?.systemThemeColor ?? "").trim()
        : themeColor
      : "";
  return hexToHsl(configuredColor) ? configuredColor : packColor;
}

export function createDreamFMTrayControlStyle(settings?: Settings | null) {
  const color = resolveTrayThemeColor(settings);
  const trayTokens = deriveDreamFMTrayControlTokens(
    color,
    settings?.effectiveAppearance === "dark",
  );
  return {
    "--dreamfm-hover-line": trayTokens?.line ?? hexToHsl(color) ?? "22 90% 52%",
    "--tray-control-color": trayTokens?.surface ?? hexToHsl(color) ?? "22 90% 52%",
    "--tray-control-foreground":
      trayTokens?.foreground ??
      hexToHsl(pickAccessibleForeground(color) ?? "#ffffff") ??
      "0 0% 100%",
  } as React.CSSProperties;
}
