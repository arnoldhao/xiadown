import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  GLASS_ELEVATIONS,
  GLASS_SHAPES,
  GLASS_TINTS,
  GlassGroup,
  GlassSurface,
} from "./glass-surface";

describe("GlassSurface", () => {
  test("publishes every reusable appearance axis", () => {
    expect(GLASS_ELEVATIONS).toEqual(["embedded", "floating", "modal"]);
    expect(GLASS_SHAPES).toEqual(["control", "card", "panel", "capsule"]);
    expect(GLASS_TINTS).toEqual(["neutral", "accent", "artwork"]);
  });

  test("emits the semantic material contract", () => {
    const markup = renderToStaticMarkup(
      <GlassSurface
        elevation="modal"
        focusRing
        interactive
        material="panel"
        shape="panel"
        tint="accent"
      >
        content
      </GlassSurface>,
    );

    expect(markup).toContain('class="app-glass-surface"');
    expect(markup).toContain('data-material="panel"');
    expect(markup).toContain('data-elevation="modal"');
    expect(markup).toContain('data-shape="panel"');
    expect(markup).toContain('data-tint="accent"');
    expect(markup).toContain('data-interactive="true"');
    expect(markup).toContain('data-focus-ring="true"');
  });

  test("groups related controls on one regular glass sample", () => {
    const markup = renderToStaticMarkup(
      <GlassGroup aria-label="Playback controls">
        <button type="button">Previous</button>
        <button type="button">Play</button>
        <button type="button">Next</button>
      </GlassGroup>,
    );

    expect(markup).toContain("app-glass-surface app-glass-group");
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-elevation="floating"');
    expect(markup).toContain('data-shape="capsule"');
  });

  test("resolves semantic roles through the canonical material preset", () => {
    const markup = renderToStaticMarkup(
      <>
        <GlassSurface material="regular" surfaceRole="overlay">
          overlay
        </GlassSurface>
        <GlassSurface surfaceRole="status">status</GlassSurface>
      </>,
    );

    expect(markup).toContain('data-surface-role="overlay"');
    expect(markup).toContain('data-surface-role="status"');
    expect(markup.match(/data-material="panel"/g)).toHaveLength(1);
    expect(markup.match(/data-material="regular"/g)).toHaveLength(1);
  });

  test("passes persistent chrome roles through the regular material API", () => {
    const markup = renderToStaticMarkup(
      <GlassSurface
        data-glass-role="sidebar"
        elevation="embedded"
        material="regular"
      >
        chrome
      </GlassSurface>,
    );

    expect(markup).toContain('data-glass-role="sidebar"');
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-elevation="embedded"');
  });

  test("keeps material, shape, interaction, and accessibility in one recipe", async () => {
    const [tokens, glass] = await Promise.all([
      Bun.file(
        new URL("../styles/dream/tokens.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../styles/dream/glass.css", import.meta.url),
      ).text(),
    ]);

    for (const role of [
      "--app-radius-control",
      "--app-radius-card",
      "--app-radius-panel",
      "--app-radius-capsule",
      "--app-glass-rim-highlight",
      "--app-glass-rim-lowlight",
      "--app-glass-specular",
      "--app-glass-chrome-surface",
      "--app-glass-chrome-dense-surface",
      "--app-glass-chrome-filter",
      "--app-glass-interactive-fill-hover",
    ]) {
      expect(tokens).toContain(role);
    }

    expect(tokens).toContain("@media (prefers-contrast: more)");
    expect(glass).toContain('.app-glass-surface[data-material="clear"]');
    expect(glass).toContain('.app-glass-surface[data-material="solid"]');
    expect(glass).toContain(
      '.app-glass-surface[data-material="regular"][data-glass-role="header"]',
    );
    expect(glass).toContain(
      '.app-glass-surface[data-material="regular"][data-glass-role="sidebar"]',
    );
    expect(glass).toContain(
      '.app-glass-surface[data-material="regular"][data-glass-role="footer"]',
    );
    expect(glass).toContain(
      '.app-glass-surface[data-material="regular"][data-glass-role="companion"]',
    );
    const structuralRecipe = glass.match(
      /\.app-glass-surface\[data-material="regular"\]\[data-glass-role="sidebar"\],[\s\S]*?\{([\s\S]*?)\n  \}/,
    )?.[1] ?? "";
    expect(structuralRecipe).toContain(
      "--app-glass-filter: var(--app-glass-chrome-filter)",
    );
    expect(structuralRecipe).toContain(
      "--app-glass-chrome-tint-alpha",
    );
    expect(structuralRecipe).not.toContain("backdrop-filter:");
    expect(glass).toContain(
      "--app-glass-filter: var(--app-glass-chrome-dense-filter)",
    );
    expect(glass).not.toContain(
      '.app-glass-surface[data-material="panel"][data-glass-role',
    );
    expect(glass).not.toContain("inset 0 1px 0 var(--app-glass-rim-highlight)");
    expect(glass).toContain("inset 0 -1px 0 var(--app-glass-rim-lowlight)");
    expect(glass).toContain(
      "--app-surface-inner-radius: calc(var(--app-glass-surface-radius) - 1px)",
    );
    expect(glass).toContain(".app-glass-group > :is(button");
    expect(glass).toContain('--app-surface-state-shadow: 0 0 transparent');
    expect(glass).toContain(
      'box-shadow: var(--app-glass-shadow), var(--app-surface-state-shadow)',
    );
    expect(glass).toContain(':root:not([data-input-modality="pointer"])');
    expect(glass).toContain("outline-offset: -2px");
    expect(glass).toContain("outline-color: Highlight");
    const opticalRecipe = glass.match(
      /\.app-glass-surface\[data-material\] \{([\s\S]*?)\n  \}/,
    )?.[1] ?? "";
    expect(glass).toMatch(/\.app-glass-surface \{[\s\S]*?position: relative;/);
    expect(glass).toMatch(
      /\.app-glass-surface::before\s*\{[^}]*inset:\s*0;[^}]*border-radius:\s*0;/s,
    );
    expect(opticalRecipe).not.toContain("position:");
    expect(glass).not.toMatch(
      /\.app-glass-group\s*>[^{}]+\{[^}]*backdrop-filter/s,
    );
  });
});
