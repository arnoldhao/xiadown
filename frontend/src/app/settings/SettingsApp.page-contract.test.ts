import { describe, expect, test } from "bun:test";

const settingsAppURL = new URL("./SettingsApp.tsx", import.meta.url);
const shellCSSURL = new URL(
  "../../shared/styles/dream/shell.css",
  import.meta.url,
);
const layoutCSSURL = new URL(
  "../../shared/styles/dream/layout-contract.css",
  import.meta.url,
);

describe("Settings standalone page contract", () => {
  test("publishes the Settings recipe without replacing its host-owned header", async () => {
    const source = await Bun.file(settingsAppURL).text();

    expect(source).toContain('presentation: "standalone-window"');
    expect(source).toContain('recipe: "settings"');
    expect(source).toContain("routeLabel: activeTabLabel");
    expect(source).toContain('topBar: "host-owned"');
    expect(source).toContain('heading: "assistive"');
    expect(source).toContain('contentLayout: "form"');
    expect(source).toContain('footer: "none"');
    expect(source).toContain('scroll: "content"');
    expect(source).toContain("<WorkspacePage");
    expect(source).toContain("<WorkspacePageContent");
    expect(source).toContain("app-settings-host-header");
  });

  test("uses the active tab label as the sole route heading", async () => {
    const source = await Bun.file(settingsAppURL).text();

    expect(source).toContain(
      "visibleTabs.find((tab) => tab.id === activeTab)?.label",
    );
    expect(source).not.toMatch(/<h1\b/);
    expect(source.match(/<WorkspacePageContent\b/g)).toHaveLength(1);
  });

  test("assigns the host header and sole content scroller to separate grid areas", async () => {
    const source = await Bun.file(settingsAppURL).text();
    const [shellCSS, layoutCSS] = await Promise.all([
      Bun.file(shellCSSURL).text(),
      Bun.file(layoutCSSURL).text(),
    ]);

    expect(source).not.toContain("flex h-screen flex-col");
    expect(source).not.toContain("flex-1 overflow-auto");
    expect(shellCSS).toContain(
      ".app-settings-window > .app-settings-host-header",
    );
    expect(shellCSS).toContain(
      ".app-settings-window > .app-settings-page-content",
    );
    expect(layoutCSS).toContain(
      "grid-template-rows: auto minmax(0, 1fr) auto",
    );
    expect(shellCSS).not.toContain("grid-template-rows:");
    expect(shellCSS).not.toContain("grid-template-areas:");
    expect(shellCSS).toContain("padding: var(--app-space-5)");
  });

  test("uses the validated system handler for web links and one fixed mail action", async () => {
    const source = await Bun.file(settingsAppURL).text();

    expect(source).toContain('import { openExternalURL, useFontFamilies } from "@/shared/query/system";');
    expect(source).toContain('const ABOUT_CONTACT_EMAIL_URL = "mailto:xunruhao@gmail.com";');
    expect(source).toContain("Browser.OpenURL(ABOUT_CONTACT_EMAIL_URL)");
    expect(source.match(/Browser\.OpenURL\(/g)).toHaveLength(1);
    expect(source).not.toContain('openExternalURL("mailto:');
    expect(source).toContain('openExternalURL("https://xiadown.app/")');
  });

  test("separates official web links from contact channels and includes Xiaohongshu", async () => {
    const source = await Bun.file(settingsAppURL).text();
    const webStart = source.indexOf(
      "<SettingsCompactRow label={text.about.web}>",
    );
    const contactStart = source.indexOf(
      "<SettingsCompactRow label={text.about.contact}>",
      webStart,
    );
    const feedbackStart = source.indexOf(
      "<SettingsCompactRow label={text.about.feedback}>",
      contactStart,
    );
    const webRow = source.slice(webStart, contactStart);
    const contactRow = source.slice(contactStart, feedbackStart);

    expect(webStart).toBeGreaterThan(-1);
    expect(contactStart).toBeGreaterThan(webStart);
    expect(feedbackStart).toBeGreaterThan(contactStart);
    expect(webRow).toContain('openExternalURL("https://xiadown.app/")');
    expect(webRow).toContain(
      'openExternalURL("https://github.com/arnoldhao/xiadown")',
    );
    expect(contactRow).toContain("openAboutContactEmail");
    expect(contactRow).toContain('openExternalURL("https://x.com/ArnoldHaoCA")');
    expect(contactRow).toContain(
      "openExternalURL(ABOUT_XIAOHONGSHU_URL)",
    );
    expect(contactRow).toContain(
      '<SiteBrandIcon siteKey="xiaohongshu" className="h-4 w-4" />',
    );
    expect(source).toContain(
      '"https://www.xiaohongshu.com/user/profile/64dccf7d000000000100577e"',
    );
    expect(contactRow).not.toContain("https://xiadown.app/");
    expect(contactRow).not.toContain("https://github.com/arnoldhao/xiadown");
  });

  test("moves data and browser profile management into General sheets", async () => {
    const [source, sheets] = await Promise.all([
      Bun.file(settingsAppURL).text(),
      Bun.file(new URL("./SettingsDataSheets.tsx", import.meta.url)).text(),
    ]);

    expect(source).toContain("<DataManagementSheet");
    expect(source).toContain("<BrowserProfilesSheet");
    expect(source).toContain('t("dataManagement.settingsRow")');
    expect(source).not.toContain("handleRefreshBrowserCandidates");
    expect(source).not.toContain("selectedSniffBrowser");
    expect(sheets).toContain("useDataManagementSnapshot");
    expect(sheets).toContain("useCleanDataManagement");
    expect(sheets).not.toContain("createSniffProfile");
    expect(sheets).toContain("renameSniffProfile");
    expect(sheets).toContain("deleteSniffProfile");
    expect(sheets).toContain("profiles.map((profile)");
    expect(sheets).toContain('t("dataManagement.obsolete")');
    expect(sheets).not.toContain(
      ["dataManagement", "createProfile"].join("."),
    );
  });
});
