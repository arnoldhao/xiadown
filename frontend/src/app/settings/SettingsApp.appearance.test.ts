import { expect, test } from "bun:test";

test("offers Glass and Contrast as window-wide surface styles", async () => {
  const source = await Bun.file(
    new URL("./SettingsApp.tsx", import.meta.url),
  ).text();
  const start = source.indexOf(
    "<SettingsCompactRow label={text.settings.surfaceStyle}>",
  );
  const end = source.indexOf("<SettingsCompactSeparator />", start);
  const surfaceStyleRow = source.slice(start, end);

  expect(start).toBeGreaterThan(-1);
  expect(end).toBeGreaterThan(start);
  expect(surfaceStyleRow).toContain('{ value: "glass"');
  expect(surfaceStyleRow).toContain('{ value: "contrast"');
  expect(surfaceStyleRow).not.toContain('{ value: "pixel"');
  expect(surfaceStyleRow).toContain(
    "saveAppearancePatch({ surfaceStyle: option.value })",
  );
});

test("publishes the selected style and resolved material on the settings root", async () => {
  const source = await Bun.file(
    new URL("./SettingsApp.tsx", import.meta.url),
  ).text();

  expect(source).toContain(
    "useWindowMaterialMode(appearanceDraft.surfaceStyle)",
  );
  expect(source).toContain(
    "data-surface-style={appearanceDraft.surfaceStyle}",
  );
  expect(source).toContain("data-window-material={windowMaterial}");
});

test("routes selected option appearance through the shared Button tone", async () => {
  const [source, buttonContract] = await Promise.all([
    Bun.file(new URL("./SettingsApp.tsx", import.meta.url)).text(),
    Bun.file(
      new URL("../../shared/styles/dream/button-contract.css", import.meta.url),
    ).text(),
  ]);

  expect(source).not.toContain("activeSegmentStyle");
  expect(source).toContain("<TabButton");
  expect(source).toContain("tone={proxyDraft.mode === option.value ? \"accent\" : \"neutral\"}");
  expect(source).toContain('tone={active ? "accent" : "neutral"}');
  expect(source).toContain(
    'tone={appearanceDraft.surfaceStyle === option.value ? "accent" : "neutral"}',
  );
  expect(buttonContract).toContain(
    '[data-variant="outline"][data-tone="accent"][data-size]',
  );
});

test("maps Settings and dependency health through canonical primitives", async () => {
  const [settingsSource, dependencySource, helpersSource, buttonContract] = await Promise.all([
    Bun.file(new URL("./SettingsApp.tsx", import.meta.url)).text(),
    Bun.file(
      new URL("../main/dependency-repair-card.tsx", import.meta.url),
    ).text(),
    Bun.file(new URL("./settings-helpers.tsx", import.meta.url)).text(),
    Bun.file(
      new URL("../../shared/styles/dream/button-contract.css", import.meta.url),
    ).text(),
  ]);

  expect(settingsSource).toContain("<StatusBadge");
  expect(settingsSource).toContain('tone="warning"');
  expect(settingsSource).toContain("tone={latestUpdateBadgeTone}");
  expect(settingsSource).not.toContain("app-settings-status-badge");
  expect(settingsSource).not.toContain("app-dream-status-badge-");

  expect(dependencySource).toContain("<StatusBadge");
  expect(dependencySource).toContain("tone={resolveDependencyTone(status)}");
  expect(dependencySource).not.toContain("app-dependency-status-badge-");

  expect(helpersSource).toContain("<StatusBadge");
  expect(helpersSource).toContain("<Progress");
  expect(helpersSource).toContain('tone={props.active ? "accent" : "neutral"}');
  expect(helpersSource).not.toMatch(/border-(?:emerald|rose|amber)-/);
  expect(helpersSource).not.toContain("<button");
  expect(buttonContract).toContain(".app-settings-tab-button");
});
