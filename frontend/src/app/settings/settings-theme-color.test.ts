import { describe, expect, test } from "bun:test";

import {
  previewFontStack,
  resolveAccentColor,
  resolveThemeColorPreview,
  resolveThemeColorSelection,
  SYSTEM_THEME_COLOR,
} from "./settings-helpers";

describe("settings dynamic accent values", () => {
  test("routes invalid stored values to the system/Dream fallback", () => {
    expect(resolveAccentColor("not-a-color")).toBe("");
    expect(resolveThemeColorSelection("not-a-color")).toBe(
      SYSTEM_THEME_COLOR,
    );
    expect(resolveThemeColorPreview(SYSTEM_THEME_COLOR)).toBe("");
  });

  test("preserves valid custom and system colors", () => {
    expect(resolveThemeColorSelection("#4f46e5")).toBe("#4f46e5");
    expect(resolveThemeColorPreview("#4f46e5", "#0f766e")).toBe("#4f46e5");
    expect(resolveThemeColorPreview(SYSTEM_THEME_COLOR, "#0f766e")).toBe(
      "#0f766e",
    );
  });

  test("uses Dream's system stack token for custom font fallbacks", () => {
    expect(previewFontStack("Georgia")).toBe(
      '"Georgia", var(--app-font-system)',
    );
    expect(previewFontStack(" ")).toBeUndefined();
  });
});
