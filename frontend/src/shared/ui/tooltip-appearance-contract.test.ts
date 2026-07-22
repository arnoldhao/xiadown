import { describe, expect, test } from "bun:test";

describe("Tooltip Dream appearance contract", () => {
  test("keeps portal geometry in React and the visual recipe in Dream", async () => {
    const [source, controls] = await Promise.all([
      Bun.file(new URL("./tooltip.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/controls.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("createPortal(");
    expect(source).toContain("left: position.left");
    expect(source).toContain("top: position.top");
    expect(source).not.toContain("transform:");
    expect(source).not.toMatch(/translate\(/);
    expect(source).toContain("useIsomorphicLayoutEffect");
    expect(source).toContain("app-dream-tooltip");
    expect(source).toContain("app-dream-tooltip__arrow");
    expect(source).not.toMatch(
      /(?:rounded-md|bg-foreground|text-background|shadow-(?:lg|black)|text-\[10px\]|font-medium)/,
    );
    expect(source).not.toMatch(
      /style=\{\{[^}]*(?:background|color|boxShadow|borderRadius|filter|font)/s,
    );

    expect(controls).toContain(".app-dream-tooltip {");
    expect(controls).toContain("background: var(--app-text-primary)");
    expect(controls).toContain("font-size: 0.625rem");
    expect(controls).toContain(".app-dream-tooltip__arrow {");
    expect(controls).toContain("@media (forced-colors: active)");

    const anatomy = await Bun.file(
      new URL("../styles/dream/anatomy.css", import.meta.url),
    ).text();
    expect(anatomy).toContain(
      '.app-dream-tooltip[data-side="top"][data-align="center"]',
    );
    expect(anatomy).toContain("transform: translate(-50%, -100%)");
  });
});
