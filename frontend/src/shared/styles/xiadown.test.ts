import { describe, expect, test } from "bun:test";

import { resolveXiaMainSidebarSurface } from "./xiadown";

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
});
