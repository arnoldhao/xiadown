import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  APP_BUTTON_SHAPES,
  APP_BUTTON_SIZES,
  APP_BUTTON_TONES,
  APP_BUTTON_VARIANTS,
  Button,
} from "./button";

describe("shared button appearance contract", () => {
  test("publishes every supported appearance axis for fixtures and audits", () => {
    expect(APP_BUTTON_VARIANTS).toEqual([
      "default",
      "destructive",
      "outline",
      "secondary",
      "ghost",
      "link",
      "sidebar",
      "glass",
    ]);
    expect(APP_BUTTON_TONES).toEqual([
      "neutral",
      "accent",
      "destructive",
      "success",
      "warning",
    ]);
    expect(APP_BUTTON_SIZES).toEqual([
      "default",
      "sm",
      "lg",
      "icon",
      "compact",
      "compactIcon",
    ]);
    expect(APP_BUTTON_SHAPES).toEqual([
      "control",
      "capsule",
      "circle",
      "square",
    ]);
  });

  test("maps intent to stable variant, tone, size, and shape attributes", () => {
    const markup = renderToStaticMarkup(
      <Button shape="control" size="compact" tone="warning" variant="outline">
        Retry
      </Button>,
    );

    expect(markup).toContain('data-app-button="true"');
    expect(markup).toContain('data-variant="outline"');
    expect(markup).toContain('data-tone="warning"');
    expect(markup).toContain('data-size="compact"');
    expect(markup).toContain('data-shape="control"');
  });

  test("makes a standalone glass button consume the shared material recipe", () => {
    const markup = renderToStaticMarkup(<Button variant="glass">Options</Button>);

    expect(markup).toContain("app-glass-surface");
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('data-elevation="floating"');
    expect(markup).toContain('data-shape="control"');
    expect(markup).toContain('data-tint="neutral"');
  });

  test("loads explicit prop styling after legacy utility inference", async () => {
    const [entry, anatomy, contract] = await Promise.all([
      Bun.file(new URL("../styles/dream.css", import.meta.url)).text(),
      Bun.file(new URL("../styles/dream/anatomy.css", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/button-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(entry.indexOf('./dream/completed.css')).toBeLessThan(
      entry.indexOf('./dream/button-contract.css'),
    );
    expect(contract).toContain("--app-accent-solid");
    expect(contract).toContain("--app-accent-on-solid");
    expect(contract).toContain(
      '[data-tone="success"][data-size]',
    );
    expect(contract).toContain("--app-status-surface-success");
    expect(contract).toContain(
      '[data-tone="warning"][data-size]',
    );
    expect(contract).toContain("--app-status-surface-orphan");
    expect(contract).toContain(".app-glass-group > .app-dream-button");
    const focusRule = contract.match(
      /:root:not\(\[data-input-modality="pointer"\]\)[^{]*:focus-visible\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    expect(focusRule).toContain("outline: 2px solid");
    expect(focusRule).toContain("outline-offset: -2px");
    expect(focusRule).not.toContain("box-shadow");
    for (const source of [anatomy, contract]) {
      expect(source).toContain("--app-button-inline-size");
      expect(source).toContain("--app-button-block-size");
      expect(source).toContain("--app-button-padding");
      expect(source).not.toContain("--app-button-width");
      expect(source).not.toContain("--app-button-height");
    }
    expect(contract).toContain("--app-button-border");
    expect(contract).toContain("width: var(--app-button-inline-size);");
    expect(contract).toContain(
      '[data-app-button][data-variant][data-shape="circle"]',
    );
    expect(contract).toContain('[data-app-button][data-shape="control"]');
    expect(contract).not.toContain(
      "width: var(--app-button-inline-size, auto)",
    );
    expect(anatomy).toContain("--app-button-icon-size");
  });
});
