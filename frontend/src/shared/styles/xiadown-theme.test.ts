import { describe, expect, test } from "bun:test";

import type { Settings } from "@/shared/contracts/settings";

import {
  XIA_THEME_PACKS,
  mergeXiaAppearanceConfig,
  readXiaAppearance,
} from "./xiadown-theme";

function settingsWithAppearance(
  appearance: Record<string, unknown>,
): Settings {
  return {
    appearanceConfig: { appearance },
  } as unknown as Settings;
}

describe("XiaDown surface style settings", () => {
  test("reads the new surface style and defaults to glass", () => {
    expect(readXiaAppearance(null).surfaceStyle).toBe("glass");
    expect(
      readXiaAppearance(
        settingsWithAppearance({ surfaceStyle: "contrast" }),
      ).surfaceStyle,
    ).toBe("contrast");
  });

  test.each([
    ["glass", "glass"],
    ["contrast", "contrast"],
    ["pixel", "contrast"],
  ] as const)(
    "migrates legacy sidebar style %s to %s",
    (sidebarStyle, expected) => {
      expect(
        readXiaAppearance(
          settingsWithAppearance({ sidebarStyle }),
        ).surfaceStyle,
      ).toBe(expected);
    },
  );

  test("prefers a valid new value and can recover from an invalid one", () => {
    expect(
      readXiaAppearance(
        settingsWithAppearance({
          surfaceStyle: "glass",
          sidebarStyle: "contrast",
        }),
      ).surfaceStyle,
    ).toBe("glass");
    expect(
      readXiaAppearance(
        settingsWithAppearance({
          surfaceStyle: "unsupported",
          sidebarStyle: "pixel",
        }),
      ).surfaceStyle,
    ).toBe("contrast");
  });

  test("persists surfaceStyle without dropping unrelated appearance data", () => {
    const settings = settingsWithAppearance({
      themePackId: "teal",
      sidebarStyle: "pixel",
      accentMode: "color",
      futurePreference: "preserved",
    });

    const merged = mergeXiaAppearanceConfig(settings, {
      surfaceStyle: "glass",
    });
    const appearance = merged.appearance as Record<string, unknown>;

    expect(appearance).toEqual({
      themePackId: "teal",
      accentMode: "color",
      futurePreference: "preserved",
      surfaceStyle: "glass",
    });
    expect(appearance).not.toHaveProperty("sidebarStyle");
  });

  test("keeps Pixel as a theme pack instead of a surface option", () => {
    expect(XIA_THEME_PACKS.some((pack) => pack.id === "pixel")).toBe(true);
  });
});
