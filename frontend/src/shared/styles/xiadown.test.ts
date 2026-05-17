import { describe, expect, test } from "bun:test";

import { resolveXiaMainSidebarSurface } from "./xiadown";
import { resolveThemePackAccentColor } from "./xiadown-theme";

describe("XiaDown style helpers", () => {
  test("keeps explicit sidebar style active inside the dream shell", () => {
    expect(resolveXiaMainSidebarSurface("citrus", "pixel", "dream")).toContain(
      "rounded-none",
    );
    expect(resolveXiaMainSidebarSurface("citrus", "contrast", "dream")).toContain(
      "shadow-[inset_-1px_0_0_hsl(var(--sidebar-border))]",
    );
    expect(resolveXiaMainSidebarSurface("citrus", "glass", "dream")).toContain(
      "backdrop-blur-2xl",
    );
  });

  test("uses a readable nocturne accent in dark appearance", () => {
    expect(resolveThemePackAccentColor("nocturne", "light")).toBe("#0F172A");
    expect(resolveThemePackAccentColor("nocturne", "dark")).toBe("#F3B549");
  });
});
