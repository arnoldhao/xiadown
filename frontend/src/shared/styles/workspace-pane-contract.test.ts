import { describe, expect, test } from "bun:test";

describe("workspace pane surface contract", () => {
  test("defines one Primary canvas and one structural divider in Dream tokens", async () => {
    const tokens = await Bun.file(
      new URL("./dream/tokens.css", import.meta.url),
    ).text();

    expect(tokens).toContain("--app-workspace-divider-width: 1px;");
    expect(tokens).toContain(
      "--app-workspace-divider-color: var(--app-surface-separator);",
    );
    expect(tokens).toContain("--app-workspace-primary-glass-opacity: 0.88;");
    expect(tokens).toContain(
      "hsl(var(--background) / var(--app-workspace-primary-glass-opacity))",
    );
    expect(tokens).toContain(
      "--app-workspace-primary-subpane-surface: transparent;",
    );
    expect(tokens).toMatch(
      /:root\.dark\s*\{[^}]*--app-workspace-primary-glass-opacity:\s*0\.90/s,
    );
    expect(tokens).toMatch(
      /:root\[data-platform="windows"\]\[data-window-material="native"\]\s*\{[^}]*--app-workspace-primary-glass-opacity:\s*0\.68/s,
    );
    expect(tokens).toMatch(
      /:root\.dark\[data-platform="windows"\]\[data-window-material="native"\]\s*\{[^}]*--app-workspace-primary-glass-opacity:\s*0\.74/s,
    );
    expect(tokens).toMatch(
      /:root\[data-xiadown-theme-pack="pixel"\]\s*\{[^}]*--app-workspace-divider-width:\s*2px/s,
    );
  });

  test("keeps Primary subpanes transparent and gives only the leading pane a divider", async () => {
    const [layout, shell] = await Promise.all([
      Bun.file(new URL("./dream/layout-contract.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/shell.css", import.meta.url)).text(),
    ]);

    expect(layout).toMatch(
      /\.app-workspace-primary-subpane\s*\{[^}]*background:\s*var\(--app-workspace-primary-subpane-surface\)[^}]*box-shadow:\s*none[^}]*backdrop-filter:\s*none/s,
    );
    expect(layout).toMatch(
      /\.app-workspace-primary-subpane--leading\s*\{[^}]*border-inline-end:[^}]*var\(--app-workspace-divider-width\) solid[^}]*var\(--app-workspace-divider-color\)/s,
    );
    expect(shell).toMatch(
      /:is\(\.app-main-list-pane, \.app-main-detail-pane\)\s*\{[^}]*background:\s*var\(--app-workspace-primary-subpane-surface\)/s,
    );
    expect(shell).not.toMatch(
      /\.app-main-list-pane\s*\{[^}]*border-inline-end:/s,
    );
    expect(shell).not.toMatch(
      /\.app-main-list-pane\s*\{[^}]*background:\s*var\(--dream-page-surface\)/s,
    );
  });

  test("uses the same single divider between Sidebar, Primary, and docked Companion", async () => {
    const [workspaceLayout, workspaceAppearance, tokens] = await Promise.all([
      Bun.file(
        new URL("../../app/workspace/workspace.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./dream/workspace.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/tokens.css", import.meta.url)).text(),
    ]);

    expect(workspaceAppearance).toMatch(
      /\.app-workspace-primary-pane\s*\{[^}]*background:\s*var\(--app-workspace-primary-surface\)/s,
    );
    expect(tokens).toMatch(
      /:root\[data-xiadown-surface-style="glass"\],[\s\S]*?:where\(\[data-surface-style="glass"\]\)\s*\{[^}]*--app-workspace-primary-surface:[^}]*var\(--app-workspace-primary-glass-surface\)/s,
    );
    expect(workspaceAppearance).toMatch(
      /\.app-main-shell\[data-surface-style="glass"\]\s*\.app-workspace-primary-pane\s*\{[^}]*box-shadow:\s*none/s,
    );
    expect(workspaceAppearance).toMatch(
      /\.app-workspace-sidebar[\s\S]*?border-right:[^;]*var\(--app-workspace-divider-width\) solid[^;]*var\(--app-workspace-divider-color\)/s,
    );
    expect(workspaceAppearance).toMatch(
      /\.app-workspace-companion[\s\S]*?border-left:[^;]*var\(--app-workspace-divider-width\) solid[^;]*var\(--app-workspace-divider-color\)/s,
    );
    expect(workspaceAppearance).toMatch(
      /\.app-workspace-chrome-material\[data-glass-role\]\s*\{[^}]*--app-glass-shadow:\s*none[^}]*box-shadow:\s*none/s,
    );
    expect(workspaceLayout).not.toMatch(
      /\.app-workspace-primary-pane\s*\{[^}]*inset [^;]*0 0/s,
    );
  });

  test("repaints native station video holes only inside Primary", async () => {
    const [layout, appearance, underlay] = await Promise.all([
      Bun.file(new URL("./dream/layout-contract.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/workspace.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../app/main/listen/native-video-underlay.ts", import.meta.url),
      ).text(),
    ]);
    const nativeSelector =
      /:root\[data-listen-native-video-underlay="true"\]:is\([\s\S]*?data-youtube-workspace-video-active[\s\S]*?data-rss-bilibili-video-active[\s\S]*?data-rss-site-video-active[\s\S]*?\)[\s\S]*?\.app-workspace-primary-pane::before\s*\{([^}]*)\}/;
    const layoutRule = layout.match(nativeSelector)?.[1] ?? "";
    const appearanceRule = appearance.match(nativeSelector)?.[1] ?? "";

    expect(layout).toContain("--listen-native-video-primary-hole-x: 0px");
    expect(layout).toContain("--listen-native-video-primary-hole-r: 0px");
    expect(layout).toContain("--listen-native-video-primary-outside-mask:");
    expect(layoutRule).toContain("position: absolute");
    expect(layoutRule).toContain("z-index: -1");
    expect(layoutRule).toContain("pointer-events: none");
    expect(appearanceRule).toContain(
      "background: var(--app-workspace-primary-surface)",
    );
    expect(appearanceRule).toContain(
      "mask: var(--listen-native-video-primary-outside-mask)",
    );
    expect(`${layout}\n${appearance}`).not.toMatch(
      /data-(?:youtube-workspace|rss-[^\]]+)-video-active[\s\S]*?\.app-workspace-sidebar::before/,
    );
    expect(underlay).toContain(
      '"--listen-native-video-primary-hole-x"',
    );
    expect(underlay).toContain(
      'document.querySelectorAll<HTMLElement>(\n    ".app-workspace-primary-pane"',
    );
  });

  test("makes Library, RSS, App Sessions, and Music consume the shared subpane role", async () => {
    const [
      libraryPage,
      libraryLayout,
      libraryAppearance,
      rssPage,
      rssAppearance,
      sessions,
      music,
    ] =
      await Promise.all([
        Bun.file(
          new URL("../../app/library/LibraryWorkspacePage.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../app/library/library.css", import.meta.url),
        ).text(),
        Bun.file(new URL("./dream/library.css", import.meta.url)).text(),
        Bun.file(
          new URL("../../app/rss/RSSWorkspacePage.tsx", import.meta.url),
        ).text(),
        Bun.file(new URL("./dream/rss.css", import.meta.url)).text(),
        Bun.file(
          new URL("../../features/settings/app-sessions/index.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../app/main/listen/PageView.tsx", import.meta.url),
        ).text(),
      ]);

    for (const source of [libraryPage, rssPage, sessions, music]) {
      expect(source).toContain("app-workspace-primary-subpane");
    }
    for (const source of [libraryPage, rssPage, sessions, music]) {
      expect(source).toContain("app-workspace-primary-subpane--leading");
    }
    expect(libraryAppearance).toContain(
      "var(--app-workspace-primary-subpane-surface, transparent)",
    );
    expect(libraryLayout).not.toContain(
      "border-right: 1px solid hsl(var(--sidebar-border))",
    );
    expect(rssAppearance).toContain(
      "--rss-edge: var(--app-workspace-divider-color);",
    );
    expect(sessions).not.toContain("flex-col border-r");
    expect(music).not.toContain(
      "shadow-[inset_-1px_0_0_hsl(var(--background)/0.14)]",
    );
    expect(music).toContain("!props.playerPortalTarget &&");
    expect(music).toMatch(
      /props\.workspaceLayout[\s\S]*?\? "app-workspace-primary-subpane"[\s\S]*?: hushFullscreen/s,
    );
  });
});
