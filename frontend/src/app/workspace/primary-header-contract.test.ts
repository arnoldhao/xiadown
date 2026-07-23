import { describe, expect, test } from "bun:test";

describe("workspace primary header contract", () => {
  test("uses one caption safe-area token and leading-action layout", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/layout-contract.css", import.meta.url),
    ).text();

    expect(css).toContain(".app-workspace-primary-header {");
    expect(css).toContain(
      "--app-workspace-primary-header-window-controls-safe-area: 0px;",
    );
    expect(css).toMatch(
      /\.app-workspace-primary-header\s*\{[^}]*position:\s*relative[^}]*z-index:\s*var\(--app-layer-floating-controls\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-primary-header\[data-window-controls="true"\]\s*\{[^}]*--app-workspace-primary-header-window-controls-safe-area:\s*var\(--app-windows-caption-control-width\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-primary-header__actions\s*\{[^}]*justify-content:\s*flex-start/s,
    );
    expect(css).toMatch(
      /\.app-workspace-primary-header__safe-area\s*\{[^}]*flex:\s*0 0 var\(--app-workspace-primary-header-window-controls-safe-area\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__topbar-drag-region\s*\{[^}]*min-width:\s*var\(--app-workspace-page-drag-region-min-width\)[^}]*flex:\s*1 1 auto/s,
    );
  });

  test("propagates the live Primary caption owner through every workspace branch", async () => {
    const mainSource = await Bun.file(
      new URL("../main/MainApp.tsx", import.meta.url),
    ).text();
    const propagations = mainSource.match(
      /reserveWindowControls=\{primaryWindowsChromeVisible\}/g,
    );

    expect(mainSource).toContain(
      "isWindows && !companion.open && !playerFullscreen",
    );
    expect(propagations?.length).toBeGreaterThanOrEqual(7);
    expect(mainSource).toContain("<RunningPage");
    expect(mainSource).toContain("<LibraryWorkspacePage");
    expect(mainSource).toContain("<AppSessionsSection");
    expect(mainSource).toContain("<PetsGalleryPage");
    expect(mainSource).toContain("<RSSWorkspacePage");
    expect(mainSource).toContain("<YouTubeWorkspacePage");
    expect(mainSource).toContain("<ListenPage");
  });

  test("gives every migrated station one shared page anatomy and an explicit heading policy", async () => {
    const [primitive, ...sources] = await Promise.all([
      Bun.file(
        new URL("../../shared/ui/workspace-page.tsx", import.meta.url),
      ).text(),
      Bun.file(new URL("../library/LibraryWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("../main/listen/PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("../youtube/YouTubeWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("../rss/RSSWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("../rss/RSSAddSubscriptionPage.tsx", import.meta.url)).text(),
    ]);

    for (const source of sources) {
      expect(source).toContain("<WorkspacePage");
      expect(source).toContain("<WorkspacePageContent");
      expect(source).not.toContain("app-station-search-header__title");
    }

    expect(primitive).toContain("<WorkspacePageHeading");
    expect(primitive).toContain('contract.heading === "assistive"');
    expect(primitive).toContain(
      'className={cn("app-visually-hidden", titleClassName)}',
    );
    expect(primitive).toContain("app-workspace-page__heading-title");

    expect(sources[0]).toContain('recipe: searchRoute ? "search" : "collection"');
    expect(sources[0]).toContain('heading: "assistive"');
    expect(sources[1]).toContain('recipe: "browse"');
    expect(sources[1]).toContain('heading: "display"');
    expect(sources[2]).toContain('recipe: "browse"');
    expect(sources[2]).toContain('heading: "display"');
    expect(sources[3]).toContain('recipe: "feed"');
    expect(sources[3]).toContain('heading: "assistive"');
    expect(sources[3]).toContain("collectionToolbarOwnsTrailingEdge");
    expect(sources[4]).toContain('recipe: isSearch ? "search" : "collection"');
    expect(sources[4]).toContain('heading: "assistive"');
  });

  test("classifies every Library-only primary branch instead of cloning station chrome", async () => {
    const [running, pets, appSessions] = await Promise.all([
      Bun.file(new URL("../main/RunningPage.tsx", import.meta.url)).text(),
      Bun.file(new URL("../pets-gallery/PetsGalleryPage.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../features/settings/app-sessions/index.tsx", import.meta.url),
      ).text(),
    ]);

    for (const source of [running, pets, appSessions]) {
      expect(source).toContain("defineWorkspacePageContract");
      expect(source).toContain("<WorkspacePage");
      expect(source).toContain("<WorkspacePageTopBar");
      expect(source).toContain("<WorkspacePageContent");
      expect(source).toContain("reserveWindowControls");
    }
    expect(running).toContain('recipe: "operational"');
    expect(appSessions).toContain('recipe: "operational"');
    expect(pets).toContain('recipe: "browse"');
    expect(pets).toContain('heading: "display"');
    expect(pets).toContain('contentLayout: "card-grid"');
    expect(pets).toContain('recipe: "detail"');
    expect(pets).not.toContain('customContractId: "pet-gallery-primary"');
    expect(pets).not.toContain("System.IsWindows()");
    expect(appSessions).not.toMatch(/app-sessions-detail-header[^\n]*border-b/);
    expect(appSessions).not.toContain("app-sessions-detail-icon");
    expect(appSessions).toContain('heading: "assistive"');
  });

  test("classifies special route roots while presentation hosts retain their own chrome", async () => {
    const [sniff, settings, appearanceGuide] = await Promise.all([
      Bun.file(new URL("../sniff-desk/SniffDeskPage.tsx", import.meta.url)).text(),
      Bun.file(new URL("../settings/SettingsApp.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/ui/APPEARANCE_CONTRACT.md", import.meta.url),
      ).text(),
    ]);

    for (const source of [sniff, settings]) {
      expect(source).toContain("defineWorkspacePageContract");
      expect(source).toContain("<WorkspacePage");
      expect(source).toContain("<WorkspacePageContent");
    }
    expect(sniff).toContain('customContractId: "sniff-desk-primary"');
    expect(sniff).toContain('topBar: "drag"');
    expect(settings).toContain('presentation: "standalone-window"');
    expect(settings).toContain('topBar: "host-owned"');
    expect(appearanceGuide).toContain("Companion content, portaled");
    expect(appearanceGuide).toContain("embedded fullscreen state");
  });
});
