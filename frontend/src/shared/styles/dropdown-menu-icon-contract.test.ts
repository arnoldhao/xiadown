import { describe, expect, test } from "bun:test";

describe("dropdown menu icon contract", () => {
  test("normalizes artwork without resizing semantic indicators", async () => {
    const [css, primitive, mainApp] = await Promise.all([
      Bun.file(new URL("./dream/components.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../components/ui/dropdown-menu.tsx", import.meta.url),
      ).text(),
      Bun.file(new URL("../../app/main/MainApp.tsx", import.meta.url)).text(),
    ]);

    const artworkRule = css.match(
      /:is\(\.app-dream-menu-item, \.app-menu-item\)\s*>\s*svg:not\(\[data-menu-indicator="true"\]\)\s*\{([^}]*)\}/s,
    )?.[1];
    expect(artworkRule).toBeDefined();
    expect(artworkRule).toContain("width: 1rem");
    expect(artworkRule).toContain("height: 1rem");
    expect(artworkRule).toContain("flex: 0 0 1rem");
    expect(artworkRule).toContain("stroke-width: 1.75");

    expect(primitive.match(/data-menu-indicator="true"/g)).toHaveLength(2);
    expect(mainApp).toMatch(
      /<Check\s+className="h-3\.5 w-3\.5 shrink-0"\s+data-menu-indicator="true"\s*\/>/s,
    );
  });
});
