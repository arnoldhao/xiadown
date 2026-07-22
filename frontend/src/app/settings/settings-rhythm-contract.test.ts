import { describe, expect, test } from "bun:test";

const read = (relativePath: string) =>
  Bun.file(new URL(relativePath, import.meta.url)).text();

function cssRule(source: string, selector: string) {
  const start = source.indexOf(selector);
  expect(start).toBeGreaterThan(-1);
  const end = source.indexOf("}", start);
  expect(end).toBeGreaterThan(start);
  return source.slice(start, end + 1);
}

describe("Settings and comfortable action rhythm", () => {
  test("keeps the established 76 by 52 Settings tab geometry", async () => {
    const [tokens, workflows] = await Promise.all([
      read("../../shared/styles/dream/tokens.css"),
      read("../../shared/styles/dream/workflows.css"),
    ]);
    const tabRule = cssRule(workflows, ".app-settings-tab-button {");

    expect(tokens).toContain("--app-settings-tab-inline-size: 4.75rem;");
    expect(tokens).toContain("--app-settings-tab-block-size: 3.25rem;");
    expect(tabRule).toContain(
      "--app-button-inline-size: var(--app-settings-tab-inline-size)",
    );
    expect(tabRule).toContain(
      "--app-button-block-size: var(--app-settings-tab-block-size)",
    );
    expect(tabRule).toContain("--app-button-gap: 0");
    expect(tabRule).toContain("display: grid");
    expect(tabRule).toContain("place-items: center");
  });

  test("compact Settings cards override generic CardContent padding", async () => {
    const [dreamRoot, anatomy, settings] = await Promise.all([
      read("../../shared/styles/dream.css"),
      read("../../shared/styles/dream/anatomy.css"),
      read("../../shared/styles/dream/settings.css"),
    ]);
    const compactContentRule = cssRule(
      settings,
      ".app-settings-list-card-content.app-settings-list-card-content-compact {",
    );
    const themeContentRule = cssRule(
      settings,
      ".app-settings-list-card-content.app-settings-list-card-content-compact.app-settings-theme-pack-card-content {",
    );

    expect(dreamRoot.indexOf('@import "./dream/anatomy.css"')).toBeLessThan(
      dreamRoot.indexOf('@import "./dream/settings.css"'),
    );
    expect(compactContentRule).toContain("padding: 0");
    expect(themeContentRule).toContain("padding: var(--app-space-3)");
    expect(anatomy).toMatch(
      /\.app-settings-list-card\s*\{[^}]*border:\s*0;[^}]*border-radius:\s*var\(--dream-radius-xl\)[^}]*background:\s*var\(--dream-surface-card\)/s,
    );
  });

  test("restores comfortable actions through one Dream size vocabulary", async () => {
    const [
      tokens,
      workflows,
      settings,
      settingsApp,
      sessions,
      running,
      sniff,
    ] = await Promise.all([
      read("../../shared/styles/dream/tokens.css"),
      read("../../shared/styles/dream/workflows.css"),
      read("../../shared/styles/dream/settings.css"),
      read("./SettingsApp.tsx"),
      read("../../features/settings/app-sessions/index.tsx"),
      read("../main/RunningPage.tsx"),
      read("../sniff-desk/SniffDeskPage.tsx"),
    ]);

    expect(tokens).toContain("--app-button-block-size-comfortable: 2.5rem;");
    expect(tokens).toContain("--app-button-block-size-choice: 2.75rem;");
    expect(workflows).toMatch(
      /\.app-running-new-download-button[^{}]*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-comfortable\)/s,
    );
    expect(workflows).toMatch(
      /\.app-running-action-button[^{}]*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-comfortable\)/s,
    );
    expect(settings).toMatch(
      /\.app-settings-theme-pack-button\s*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-choice\)/s,
    );
    expect(settings).toMatch(
      /\.app-settings-option-button\s*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-comfortable\)/s,
    );
    expect(settings).toMatch(
      /\.app-sessions-account-action\s*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-comfortable\)/s,
    );

    expect(settingsApp).toContain(
      'contentClassName="app-settings-theme-pack-card-content"',
    );
    expect(settingsApp).toContain("app-settings-theme-pack-button");
    expect(sessions.match(/app-sessions-account-action/g)).toHaveLength(4);
    expect(running).toContain('? "app-running-new-download-button"');
    expect(sniff).toContain(
      'className="app-sniff-desk-start-button app-running-new-download-button"',
    );
  });
});
