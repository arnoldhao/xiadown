import { describe, expect, test } from "bun:test";

function section(source: string, start: string, end: string) {
  const startIndex = source.indexOf(start);
  const endIndex = source.indexOf(end, startIndex + start.length);
  expect(startIndex).toBeGreaterThanOrEqual(0);
  expect(endIndex).toBeGreaterThan(startIndex);
  return source.slice(startIndex, endIndex);
}

describe("canonical status surface material", () => {
  test("shares the floating player frost through the semantic status role", async () => {
    const [tokens, glass] = await Promise.all([
      Bun.file(new URL("./dream/tokens.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/glass.css", import.meta.url)).text(),
    ]);

    for (const token of [
      "--app-surface-status-fill:",
      "--app-surface-status-filter:",
      "--app-surface-status-line:",
      "--app-surface-status-shadow:",
      "--app-surface-status-specular:",
      "--app-surface-status-specular-opacity:",
      "--app-surface-status-artwork-opacity:",
      "--app-surface-status-artwork-filter:",
      "--app-surface-status-artwork-veil:",
    ]) {
      expect(tokens).toContain(token);
    }
    expect(tokens).toContain(
      "--app-surface-status-fill: var(--app-glass-regular-surface)",
    );
    expect(tokens).toContain(
      "--app-surface-status-filter: var(--app-glass-regular-filter)",
    );
    expect(tokens).toContain(
      "--app-surface-status-shadow: var(--app-glass-regular-shadow)",
    );
    expect(tokens).toContain(
      "--app-surface-status-specular: var(--app-glass-specular)",
    );
    expect(tokens).toContain("--app-surface-status-artwork-opacity: 0.12");
    expect(tokens).toContain(
      "blur(18px) saturate(0.82) contrast(0.86)",
    );

    const statusRule = glass.match(
      /\.app-glass-surface\[data-surface-role="status"\]\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    expect(statusRule).toContain(
      "--app-glass-fill: var(--app-surface-status-fill)",
    );
    expect(statusRule).toContain(
      "--app-glass-filter: var(--app-surface-status-filter)",
    );
    expect(statusRule).toContain(
      "--app-glass-local-specular: var(--app-surface-status-specular)",
    );
    expect(statusRule).toContain("--app-surface-status-specular-opacity");
    expect(statusRule).not.toContain("--app-glass-regular-filter");
  });

  test("inherits dark player glass while keeping Contrast and accessibility fallbacks centralized", async () => {
    const tokens = await Bun.file(
      new URL("./dream/tokens.css", import.meta.url),
    ).text();
    const glass = await Bun.file(
      new URL("./dream/glass.css", import.meta.url),
    ).text();

    const dark = section(
      tokens,
      ":root.dark {",
      "/* Surface Style remaps",
    );
    expect(dark).toContain("--app-glass-regular-surface: color-mix(");
    expect(dark).toContain("--app-glass-specular-opacity: 0.58");
    expect(dark).not.toContain("--app-surface-status-fill:");

    const contrast = section(
      tokens,
      "/* Surface Style remaps",
      ':root[data-reduce-transparency="true"]',
    );
    expect(contrast).toContain(
      ':root[data-xiadown-surface-style="contrast"]',
    );
    expect(contrast).toContain("--app-surface-status-fill: hsl(var(--card))");
    expect(contrast).toContain("--app-surface-status-filter: none");
    expect(contrast).toContain("--app-surface-status-specular-opacity: 0");
    expect(glass).toMatch(
      /data-xiadown-surface-style="contrast"[\s\S]*?data-surface-role="status"[\s\S]*?--app-glass-filter:\s*none/,
    );

    const explicitReduction = section(
      tokens,
      ':root[data-reduce-transparency="true"]',
      "@supports not (color: color-mix",
    );
    const preferredReduction = section(
      tokens,
      "@media (prefers-reduced-transparency: reduce)",
      "@media (prefers-contrast: more)",
    );
    const increasedContrast = section(
      tokens,
      "@media (prefers-contrast: more)",
      "@supports not ((backdrop-filter:",
    );
    const unsupportedBackdrop = section(
      tokens,
      "@supports not ((backdrop-filter:",
      "@media (forced-colors: active)",
    );
    const overlappingPreferences = section(
      tokens,
      "/* When accessibility preferences overlap",
      "@supports not ((backdrop-filter:",
    );
    const forcedColors = section(
      tokens,
      "@media (forced-colors: active)",
      ':root[data-motion="expressive"]',
    );

    for (const fallback of [
      explicitReduction,
      preferredReduction,
      unsupportedBackdrop,
    ]) {
      expect(fallback).toContain(
        "--app-surface-status-fill: var(--app-glass-solid-surface)",
      );
      expect(fallback).toContain("--app-surface-status-filter: none");
      expect(fallback).toContain("--app-surface-status-specular-opacity: 0");
      expect(fallback).toContain("--app-surface-status-artwork-opacity: 0");
      expect(fallback).toContain("--app-surface-status-artwork-filter: none");
      expect(fallback).toContain("--app-surface-status-artwork-veil: none");
    }
    expect(increasedContrast).toContain(
      "--app-surface-status-fill: hsl(var(--popover) / 0.95)",
    );
    expect(increasedContrast).toContain(
      "--app-surface-status-line: hsl(var(--foreground) / 0.34)",
    );
    expect(overlappingPreferences).toContain(
      ':root[data-reduce-transparency="true"]',
    );
    expect(overlappingPreferences).toContain(
      "@media (prefers-reduced-transparency: reduce)",
    );
    expect(
      overlappingPreferences.match(/--app-surface-status-filter: none/g),
    ).toHaveLength(2);
    expect(forcedColors).toContain("--app-surface-status-fill: Canvas");
    expect(forcedColors).toContain("--app-surface-status-line: CanvasText");
    expect(forcedColors).toContain("--app-surface-status-filter: none");
  });

  test("Player, Running, and Sniff consume the shared role without recipes", async () => {
    const [source, css] = await Promise.all([
      Bun.file(
        new URL("../../app/main/WorkspaceActivitySurfaces.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("./dream/activity.css", import.meta.url),
      ).text(),
    ]);

    expect(source.match(/surfaceRole="status"/g)).toHaveLength(1);
    expect(source).toContain("app-workspace-player-wide");
    expect(source).toContain("app-workspace-operation-wide");
    expect(source).toContain("app-workspace-sniff-wide");
    expect(source).not.toMatch(/\bmaterial=/);
    expect(source).not.toContain("data-material=");
    expect(css).not.toContain('[data-surface-role="status"]');
    expect(css).not.toContain("backdrop-filter");
    expect(css).toContain(
      "filter: var(--app-surface-status-artwork-filter)",
    );
    expect(css).toContain(
      "opacity: var(--app-surface-status-artwork-opacity)",
    );
    expect(css).toContain(
      "background: var(--app-surface-status-artwork-veil)",
    );
    expect(css).toMatch(
      /\.app-workspace-status-card\[data-artwork="true"\]::after\s*\{[^}]*background:\s*var\(--app-surface-status-artwork-veil\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-status-card\s*>\s*\.app-workspace-status-card__artwork-backdrop\s*\{[^}]*opacity:\s*var\(--app-surface-status-artwork-opacity\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-status-card__artwork-backdrop\s*>\s*:is\(img, svg\)\s*\{[^}]*object-fit:\s*cover[^}]*filter:\s*var\(--app-surface-status-artwork-filter\)/s,
    );
    expect(css).not.toMatch(/filter:\s*blur/);
  });
});
