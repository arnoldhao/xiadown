import { describe, expect, test } from "bun:test";

describe("equalizer appearance contract", () => {
  test("uses the shared Button tone for selected visualizer placement", async () => {
    const source = await Bun.file(new URL("./index.tsx", import.meta.url)).text();

    expect(source).not.toContain("VISUALIZER_ACTIVE_STYLE");
    expect(source).toContain(
      'tone={visualizerPlacement === placement ? "accent" : "neutral"}',
    );
  });

  test("maps equalizer health to the canonical StatusBadge", async () => {
    const source = await Bun.file(new URL("./index.tsx", import.meta.url)).text();

    expect(source).toContain("<DreamStatusBadge");
    expect(source).not.toContain("border-emerald-500");
    expect(source).not.toContain("border-amber-500");
    expect(source).not.toContain("bg-muted/40");
  });
});
