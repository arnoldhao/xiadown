import {
  deriveAccentTokens,
  deriveFunctionalAccentTokens,
  hexToHsl,
  hexToHslColor,
  parseHslToken,
  pickAccessibleForegroundForHslColor,
  toHslToken,
  type FunctionalAccentTokens,
  type HslColor,
} from "@/lib/color";
import type { Settings } from "@/shared/contracts/settings";
import {
  readXiaAppearance,
  resolveThemePack,
} from "@/shared/styles/xiadown-theme";
import { readWailsRuntimeOS } from "@/shared/styles/window-material";

const LEGACY_THEME_INLINE_VARIABLES = [
  "--background",
  "--foreground",
  "--card",
  "--card-foreground",
  "--popover",
  "--popover-foreground",
  "--primary",
  "--primary-foreground",
  "--secondary",
  "--secondary-foreground",
  "--muted",
  "--muted-foreground",
  "--accent",
  "--accent-foreground",
  "--border",
  "--input",
  "--ring",
  "--sidebar-background",
  "--sidebar-foreground",
  "--sidebar-primary",
  "--sidebar-primary-foreground",
  "--sidebar-accent",
  "--sidebar-accent-foreground",
  "--sidebar-border",
  "--sidebar-ring",
  "--chart-1",
  "--chart-2",
  "--chart-3",
  "--chart-4",
  "--chart-5",
  "--app-accent-brand",
  "--app-accent-text",
  "--app-accent-solid",
  "--app-accent-on-solid",
  "--app-accent-surface",
  "--app-accent-ring",
  "--tray-control-color",
  "--tray-control-foreground",
] as const;

const USER_ACCENT_VARIABLES = [
  "--app-user-accent-brand",
  "--app-user-accent-text",
  "--app-user-accent-solid",
  "--app-user-accent-on-solid",
  "--app-user-accent-surface",
  "--app-user-accent-surface-foreground",
  "--app-user-accent-ring",
  "--app-user-secondary",
  "--app-user-secondary-foreground",
  "--app-user-sidebar-accent",
  "--app-user-sidebar-accent-foreground",
  "--app-user-tray-control-color",
  "--app-user-tray-control-foreground",
] as const;

const STARTUP_THEME_STORAGE_KEY = "xiadown:startup-theme";

function applyTheme(effectiveAppearance: string | undefined) {
  const isDark = effectiveAppearance === "dark";
  if (isDark) {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
  if (effectiveAppearance === "light" || effectiveAppearance === "dark") {
    document.documentElement.dataset.startupTheme = effectiveAppearance;
    try {
      window.localStorage.setItem(
        STARTUP_THEME_STORAGE_KEY,
        effectiveAppearance,
      );
    } catch {
      // Storage may be unavailable in hardened or ephemeral WebViews. The
      // inline startup shell still falls back to the system media query.
    }
  }
}

function clearInlineVariables(names: readonly string[]) {
  for (const name of names) {
    document.documentElement.style.removeProperty(name);
  }
}

function clearLegacyColorSchemeSelection() {
  delete document.documentElement.dataset.colorScheme;
}

function quoteFontFamily(value: string) {
  const escaped = value.replace(/\\/g, "\\\\").replace(/\"/g, '\\"');
  return `"${escaped}"`;
}

function buildFontStack(fontFamily: string | undefined) {
  const trimmed = (fontFamily ?? "").trim();
  if (!trimmed) {
    return undefined;
  }
  return `${quoteFontFamily(trimmed)}, var(--app-font-system)`;
}

function applyFont(fontFamily: string | undefined) {
  const stack = buildFontStack(fontFamily);
  if (!stack) {
    document.documentElement.style.removeProperty("--app-font-body");
    document.documentElement.style.removeProperty("--app-font-display");
    return;
  }
  document.documentElement.style.setProperty("--app-font-body", stack);
  document.documentElement.style.setProperty("--app-font-display", stack);
}

function applyFontSize(fontSize: number | undefined) {
  if (!fontSize || !Number.isFinite(fontSize) || fontSize <= 0) {
    document.documentElement.style.removeProperty("--app-font-size");
    return;
  }
  document.documentElement.style.setProperty("--app-font-size", `${fontSize}px`);
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

function deriveListenTrayControlTokens(color: string | undefined, isDark: boolean) {
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
    surface: toHslToken(surface),
    foreground: hexToHsl(foreground)!,
  };
}

function resolveContrastSurfaces() {
  const computed = getComputedStyle(document.documentElement);
  const surfaces = [
    "--background",
    "--card",
    "--popover",
    "--sidebar-background",
  ]
    .map((name) => computed.getPropertyValue(name).trim())
    .map(parseHslToken)
    .filter((color): color is HslColor => color !== null);
  if (surfaces.length > 0) {
    return surfaces;
  }
  return [];
}

function applyFunctionalAccent(
  tokens: FunctionalAccentTokens,
  accentSurface?: string,
) {
  document.documentElement.style.setProperty("--app-user-accent-brand", tokens.brand);
  document.documentElement.style.setProperty("--app-user-accent-text", tokens.text);
  document.documentElement.style.setProperty("--app-user-accent-solid", tokens.solid);
  document.documentElement.style.setProperty("--app-user-accent-on-solid", tokens.onSolid);
  document.documentElement.style.setProperty("--app-user-accent-ring", tokens.ring);
  if (accentSurface) {
    document.documentElement.style.setProperty("--app-user-accent-surface", accentSurface);
  }
}

function applyPrimaryColor(
  color: string | undefined,
  systemThemeColor?: string,
  effectiveAppearance?: string,
  contrastSurfaces?: readonly HslColor[],
  accentSurface?: string,
) {
  const resolved = resolveThemeColor(color, systemThemeColor);
  if (!contrastSurfaces?.length) {
    return false;
  }
  const functionalTokens = deriveFunctionalAccentTokens(
    resolved,
    contrastSurfaces,
  );
  const trayTokens = deriveListenTrayControlTokens(
    resolved,
    effectiveAppearance
      ? effectiveAppearance === "dark"
      : document.documentElement.classList.contains("dark"),
  );

  if (!functionalTokens || !trayTokens) {
    return false;
  }

  applyFunctionalAccent(functionalTokens, accentSurface);
  document.documentElement.style.setProperty(
    "--app-user-tray-control-color",
    trayTokens.surface,
  );
  document.documentElement.style.setProperty(
    "--app-user-tray-control-foreground",
    trayTokens.foreground,
  );
  return true;
}

function applyThemeColor(
  themeColor: string | undefined,
  systemThemeColor: string | undefined,
  effectiveAppearance: string | undefined,
  accentMode: string | undefined,
  contrastSurfaces: readonly HslColor[],
) {
  if (accentMode !== "color") {
    return;
  }

  const color = resolveThemeColor(themeColor, systemThemeColor);
  const accentTokens = deriveAccentTokens(color, effectiveAppearance === "dark");

  if (!accentTokens) {
    document.documentElement.dataset.xiadownAccentMode = "theme";
    return;
  }

  if (!applyPrimaryColor(
    color,
    undefined,
    effectiveAppearance,
    contrastSurfaces,
    accentTokens.accent,
  )) {
    document.documentElement.dataset.xiadownAccentMode = "theme";
    return;
  }
  document.documentElement.style.setProperty("--app-user-secondary", accentTokens.secondary);
  document.documentElement.style.setProperty(
    "--app-user-secondary-foreground",
    accentTokens.secondaryForeground,
  );
  document.documentElement.style.setProperty(
    "--app-user-accent-surface-foreground",
    accentTokens.accentForeground,
  );
  document.documentElement.style.setProperty(
    "--app-user-sidebar-accent",
    accentTokens.sidebarAccent,
  );
  document.documentElement.style.setProperty(
    "--app-user-sidebar-accent-foreground",
    accentTokens.sidebarAccentForeground,
  );
}

function applyThemePack(themePackId: string | undefined) {
  const pack = resolveThemePack(themePackId);
  clearInlineVariables(LEGACY_THEME_INLINE_VARIABLES);
  document.documentElement.dataset.xiadownThemePack = pack.id;
}

function applyAppearanceAttributes(appearance: ReturnType<typeof readXiaAppearance>) {
  document.documentElement.dataset.xiadownSurfaceStyle = appearance.surfaceStyle;
  delete document.documentElement.dataset.xiadownSidebarStyle;
  document.documentElement.dataset.xiadownAccentMode = appearance.accentMode;
}

function detectPlatform() {
  const runtimeOS = readWailsRuntimeOS(
    typeof window === "undefined" ? undefined : window,
  );
  if (runtimeOS === "windows" || runtimeOS === "win32") {
    return "windows";
  }
  if (runtimeOS === "darwin" || runtimeOS === "macos") {
    return "macos";
  }
  if (runtimeOS === "linux") {
    return "linux";
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
  applyThemePack(appearance.themePackId);
  applyAppearanceAttributes(appearance);
  clearInlineVariables(USER_ACCENT_VARIABLES);
  clearLegacyColorSchemeSelection();
  applyFont(settings.fontFamily);
  applyFontSize(settings.fontSize);
  applyThemeColor(
    settings.themeColor,
    settings.systemThemeColor,
    settings.effectiveAppearance,
    appearance.accentMode,
    resolveContrastSurfaces(),
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
  applyThemePack(appearance.themePackId);
  applyAppearanceAttributes(appearance);
  clearInlineVariables(USER_ACCENT_VARIABLES);
  applyThemeColor(
    settings.themeColor,
    settings.systemThemeColor,
    effectiveAppearance,
    appearance.accentMode,
    resolveContrastSurfaces(),
  );
}
