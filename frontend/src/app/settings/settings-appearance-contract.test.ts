import { expect, test } from "bun:test";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = fileURLToPath(new URL("../../../", import.meta.url));

const read = (relativePath: string) =>
  Bun.file(new URL(relativePath, import.meta.url)).text();

const strictScopes = [
  "src/app/settings/**/*.{ts,tsx}",
  "src/features/settings/**/*.{ts,tsx}",
  "src/shared/browser-source/**/*.{ts,tsx}",
  "src/app/main/file-relink/**/*.{ts,tsx}",
  "src/app/sniff-desk/**/*.{ts,tsx}",
  "src/shared/message/MessageHost.tsx",
  "src/app/main/{NewTaskDialog,RunningPage,MainApp,dependency-repair-card,WorkspaceActivitySurfaces}.tsx",
] as const;

const strictAppearanceUtility = /^(?:[a-z0-9_\-[\]=]+:)*(?:bg-[a-z0-9_[\]./%-]+|text-(?!ellipsis$|clip$)[a-z0-9_[\]./%-]+|font-[a-z0-9_[\]./%-]+|leading-[a-z0-9_[\]./%-]+|tracking-[a-z0-9_[\]./%-]+|border(?:-[a-z0-9_[\]./%-]+)?|divide-[a-z0-9_[\]./%-]+|from-[a-z0-9_[\]./%-]+|via-[a-z0-9_[\]./%-]+|to-[a-z0-9_[\]./%-]+|rounded(?:-[a-z0-9_[\]./%-]+)?|shadow(?:-[a-z0-9_[\]./%-]+)?|drop-shadow(?:-[a-z0-9_[\]./%-]+)?|ring(?:-[a-z0-9_[\]./%-]+)?|opacity-[a-z0-9_[\]./%-]+|outline-[a-z0-9_[\]./%-]+|fill-[a-z0-9_[\]./%-]+|stroke-[a-z0-9_[\]./%-]+|decoration-[a-z0-9_[\]./%-]+|accent-[a-z0-9_[\]./%-]+|caret-[a-z0-9_[\]./%-]+|placeholder-[a-z0-9_[\]./%-]+|cursor-[a-z0-9_[\]./%-]+|(?:backdrop-)?blur-[a-z0-9_[\]./%-]+|transition(?:-[a-z0-9_[\]./%-]+)?|duration-[a-z0-9_[\]./%-]+|ease-[a-z0-9_[\]./%-]+|delay-[a-z0-9_[\]./%-]+|animate-[a-z0-9_[\]./%-]+|filter|appearance-none|italic|not-italic|uppercase|lowercase|capitalize|normal-case|antialiased|subpixel-antialiased|tabular-nums)$/i;

function quotedTokens(source: string) {
  const tokens: string[] = [];
  for (const match of source.matchAll(/"([^"\n]*)"|'([^'\n]*)'|`([^`\n]*)`/g)) {
    const value = match[1] ?? match[2] ?? match[3] ?? "";
    tokens.push(...value.split(/\s+/).filter(Boolean));
  }
  return tokens;
}

test("settings and maintenance states use canonical semantic primitives", async () => {
  const [access, pairing, dataSheets, relink, helpers] = await Promise.all([
    read("./LibraryAccessSettingsCard.tsx"),
    read("./LibraryPairingSheet.tsx"),
    read("./SettingsDataSheets.tsx"),
    read("../main/file-relink/FileRelinkDialog.tsx"),
    read("../main/helpers.ts"),
  ]);

  for (const source of [access, pairing, dataSheets, relink]) {
    expect(source).toContain("<StatusBadge");
    expect(source).not.toMatch(/(?:bg|text|border)-(?:emerald|amber|rose|sky)-/);
  }
  expect(dataSheets).toContain('tone="destructive"');
  expect(relink).toContain("<Badge");
  expect(helpers).toContain('return "success"');
  expect(helpers).toContain('return "busy"');
  expect(helpers).not.toMatch(/return "(?:bg|text)-/);
});

test("shared source and settings appearance is declared in the Dream catalog", async () => {
  const [dreamRoot, dreamSettings, picker, equalizerControls, sessions] = await Promise.all([
    read("../../shared/styles/dream.css"),
    read("../../shared/styles/dream/settings.css"),
    read("../../shared/browser-source/BrowserSourcePicker.tsx"),
    read("../../features/settings/equalizer/EqualizerControlCards.tsx"),
    read("../../features/settings/app-sessions/index.tsx"),
  ]);

  expect(dreamRoot).toContain('@import "./dream/settings.css"');
  for (const selector of [
    ".app-browser-source-option",
    ".app-settings-summary-card",
    ".app-equalizer-curve-glow",
    ".app-sessions-account-info",
    ".app-file-relink-row",
  ]) {
    expect(dreamSettings).toContain(selector);
  }
  expect(dreamSettings).not.toContain(".app-library-pairing-status");
  expect(picker).toContain("data-selected={selected || undefined}");
  expect(equalizerControls).toContain('className="app-equalizer-curve-glow"');
  expect(sessions).toContain('className="app-sessions-account-info');
});

test("strict Settings and Main directories contain no Tailwind appearance or motion utilities", async () => {
  const violations: string[] = [];
  const scannedFiles = new Set<string>();
  for (const pattern of strictScopes) {
    const glob = new Bun.Glob(pattern);
    for await (const file of glob.scan({ cwd: frontendRoot, onlyFiles: true })) {
      if (/\.(?:test|spec)\.[^/]+$/.test(file)) continue;
      scannedFiles.add(file);
      const source = await Bun.file(join(frontendRoot, file)).text();
      for (const token of quotedTokens(source)) {
        if (strictAppearanceUtility.test(token)) {
          violations.push(`${file}: ${token}`);
        }
      }
    }
  }

  expect(violations).toEqual([]);
  expect([...scannedFiles]).toEqual(expect.arrayContaining([
    "src/app/settings/SettingsApp.tsx",
    "src/app/settings/index.ts",
    "src/features/settings/app-sessions/AppSessionImportSheet.tsx",
    "src/shared/browser-source/BrowserSourcePicker.tsx",
    "src/app/main/file-relink/FileRelinkDialog.tsx",
    "src/shared/message/MessageHost.tsx",
    "src/app/main/NewTaskDialog.tsx",
    "src/app/sniff-desk/SniffDeskPage.tsx",
  ]));
});

test("semantic motion classes replace Tailwind spin and pulse utilities", async () => {
  const motion = await read("../../shared/styles/dream/motion.css");
  expect(motion).toContain(".app-motion-spin");
  expect(motion).toContain(".app-motion-pulse");
  expect(motion).toContain("prefers-reduced-motion: reduce");
});

test("dynamic theme previews expose values through Dream custom-property interfaces", async () => {
  const [source, browserIcon, settings, workflows] = await Promise.all([
    read("./SettingsApp.tsx"),
    read("../../shared/browser-source/BrowserBrandIcon.tsx"),
    read("../../shared/styles/dream/settings.css"),
    read("../../shared/styles/dream/workflows.css"),
  ]);

  expect(source).not.toMatch(/style=\{\{[^}]*\bboxShadow\s*:/s);
  expect(source).not.toMatch(/style=\{\{[^}]*\bbackgroundColor\s*:/s);
  expect(source).not.toMatch(/style=\{\{[^}]*\bfontFamily\s*:/s);
  expect(source).toContain("--app-settings-swatch-active-color");
  expect(source).toContain("--app-settings-font-preview-family");
  expect(browserIcon).toContain("--app-browser-brand-color");
  expect(browserIcon).not.toContain("style={{ color:");
  expect(settings).toContain('.app-settings-swatch[data-active="true"]');
  expect(settings).toMatch(
    /\.app-settings-swatch\s*\{[^}]*transition:[^}]*transform/s,
  );
  expect(workflows).not.toContain(".app-settings-swatch");
  expect(settings).toContain(".app-settings-font-preview-option");
});
