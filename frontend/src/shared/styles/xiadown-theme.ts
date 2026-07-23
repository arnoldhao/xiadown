import type { Settings } from "@/shared/contracts/settings";

export const XIA_THEME_PACK_IDS = [
  "graphite",
  "citrus",
  "pixel",
  "tape",
  "teal",
  "cloud",
  "damson",
  "fuchsia",
  "cobalt",
  "terracotta",
  "lavender",
  "nocturne",
] as const;

export type XiaThemePackId = (typeof XIA_THEME_PACK_IDS)[number];
export type XiaSurfaceStyle = "glass" | "contrast";
export type XiaAccentMode = "theme" | "color";

export type XiaAppearanceSettings = {
  themePackId: XiaThemePackId;
  surfaceStyle: XiaSurfaceStyle;
  accentMode: XiaAccentMode;
};

/**
 * Theme packs are semantic identities only. Their visual palette, preview and
 * functional accent definitions live in Dream's theme-packs.css module.
 */
export type XiaThemePack = {
  id: XiaThemePackId;
};

const defaultAppearance: XiaAppearanceSettings = {
  themePackId: "citrus",
  surfaceStyle: "glass",
  accentMode: "theme",
};

export const XIA_THEME_PACKS: readonly XiaThemePack[] =
  XIA_THEME_PACK_IDS.map((id) => ({ id }));
export const XIA_DEFAULT_THEME_PACK_ID = defaultAppearance.themePackId;

export function resolveThemePack(id?: string): XiaThemePack {
  const normalized = (id ?? "").trim().toLowerCase();
  return (
    XIA_THEME_PACKS.find((item) => item.id === normalized) ??
    XIA_THEME_PACKS.find(
      (item) => item.id === defaultAppearance.themePackId,
    ) ??
    XIA_THEME_PACKS[0]
  );
}

export function readXiaAppearance(
  settings?: Settings | null,
): XiaAppearanceSettings {
  const appearanceConfig =
    settings?.appearanceConfig &&
    typeof settings.appearanceConfig === "object" &&
    settings.appearanceConfig !== null &&
    "appearance" in settings.appearanceConfig
      ? (settings.appearanceConfig as Record<string, unknown>).appearance
      : undefined;
  const record =
    appearanceConfig &&
    typeof appearanceConfig === "object" &&
    !Array.isArray(appearanceConfig)
      ? (appearanceConfig as Record<string, unknown>)
      : {};

  const themePackId = resolveThemePack(
    asString(record.themePackId) || defaultAppearance.themePackId,
  ).id;
  const surfaceStyle =
    parseSurfaceStyle(asString(record.surfaceStyle)) ??
    parseSurfaceStyle(asString(record.sidebarStyle)) ??
    defaultAppearance.surfaceStyle;
  const accentMode = normalizeAccentMode(
    asString(record.accentMode) || defaultAppearance.accentMode,
  );

  return {
    themePackId,
    surfaceStyle,
    accentMode,
  };
}

export function mergeXiaAppearanceConfig(
  settings: Settings | null | undefined,
  patch: Partial<XiaAppearanceSettings>,
): Record<string, unknown> {
  const baseTools =
    settings?.appearanceConfig &&
    typeof settings.appearanceConfig === "object" &&
    settings.appearanceConfig !== null
      ? { ...settings.appearanceConfig }
      : {};
  const storedAppearance =
    baseTools.appearance &&
    typeof baseTools.appearance === "object" &&
    !Array.isArray(baseTools.appearance)
      ? { ...(baseTools.appearance as Record<string, unknown>) }
      : {};
  // Writing any appearance patch performs the legacy field migration while
  // preserving unrelated appearance preferences owned by other features.
  delete storedAppearance.sidebarStyle;
  const current = readXiaAppearance(settings);
  return {
    ...baseTools,
    appearance: {
      ...storedAppearance,
      ...current,
      ...patch,
    },
  };
}

function parseSurfaceStyle(value: string): XiaSurfaceStyle | undefined {
  switch (value) {
    case "glass":
      return "glass";
    case "contrast":
    case "pixel":
      return "contrast";
    default:
      return undefined;
  }
}

function normalizeAccentMode(value: string): XiaAccentMode {
  switch (value) {
    case "color":
      return "color";
    default:
      return "theme";
  }
}

function asString(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}
