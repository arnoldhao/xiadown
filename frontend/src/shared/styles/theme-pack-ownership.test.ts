import { expect, test } from "bun:test";

test("theme pack visuals have one Dream CSS source of truth", async () => {
  const [
    entry,
    themePacks,
    registry,
    runtime,
    settings,
    welcome,
    appearance,
    foundation,
    tokens,
    lyrics,
  ] = await Promise.all([
    Bun.file(new URL("./dream.css", import.meta.url)).text(),
    Bun.file(new URL("./dream/theme-packs.css", import.meta.url)).text(),
    Bun.file(new URL("./xiadown-theme.ts", import.meta.url)).text(),
    Bun.file(new URL("./theme-runtime.ts", import.meta.url)).text(),
    Bun.file(
      new URL("../../app/settings/SettingsApp.tsx", import.meta.url),
    ).text(),
    Bun.file(
      new URL("../../app/main/WelcomeScreen.tsx", import.meta.url),
    ).text(),
    Bun.file(new URL("../../app/dev/AppearanceLab.tsx", import.meta.url)).text(),
    Bun.file(new URL("./dream/foundation.css", import.meta.url)).text(),
    Bun.file(new URL("./dream/tokens.css", import.meta.url)).text(),
    Bun.file(
      new URL("../../app/main/listen/lyrics-renderers.tsx", import.meta.url),
    ).text(),
  ]);

  expect(entry).toContain('@import "./dream/theme-packs.css";');
  expect(themePacks).toContain(
    ':root[data-xiadown-theme-pack="graphite"]',
  );
  expect(themePacks).toContain(
    ':root.dark[data-xiadown-theme-pack="nocturne"]',
  );
  expect(themePacks).toContain('[data-theme-pack-preview="citrus"]');
  expect(themePacks).toContain(
    ':root[data-xiadown-theme-pack][data-xiadown-accent-mode="color"]',
  );
  expect(themePacks).toContain("--app-user-accent-solid");

  expect(registry).not.toMatch(/#[\dA-F]{6}/i);
  expect(registry).not.toContain("functionalAccent");
  expect(registry).not.toContain("preview:");
  expect(runtime).toContain(
    "document.documentElement.dataset.xiadownThemePack = pack.id",
  );
  expect(runtime).not.toMatch(
    /style\.setProperty\(\s*["']--(?:background|foreground|card|popover|muted|accent|border|input|chart-)/,
  );
  expect(runtime).toContain('"--app-user-accent-solid"');
  expect(runtime).not.toContain("style.colorScheme");
  expect(runtime).not.toContain("system-ui");
  expect(runtime).not.toMatch(/\d+\s+\d+%\s+\d+%/);
  expect(runtime).toContain(
    'style.removeProperty("--app-font-size")',
  );
  expect(tokens).toContain("--app-font-system: system-ui");
  expect(tokens).toContain("--app-font-body: var(--app-font-system)");

  for (const source of [settings, welcome]) {
    expect(source).toContain("data-theme-pack-preview");
    expect(source).not.toContain("pack.preview");
  }
  expect(appearance).toContain(
    "applyXiaTheme({",
  );
  expect(appearance).toContain('aria-label="Accent source"');
  expect(appearance).toContain('aria-label="Font family"');
  expect(appearance).toContain('aria-label="Font size"');
  expect(appearance).not.toContain("root.style.setProperty");

  for (const source of [runtime, foundation]) {
    expect(source).not.toContain("--listen-hover-line");
  }
  expect(lyrics).not.toContain("--listen-lyrics-focus-line-progress");
});
