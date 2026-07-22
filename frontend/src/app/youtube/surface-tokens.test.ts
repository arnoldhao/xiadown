import { describe, expect, test } from "bun:test";

function rule(css: string, selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return css.match(new RegExp(`(?:^|\\n)${escaped}\\s*\\{([^}]*)\\}`, "s"))?.[1] ?? "";
}

describe("YouTube workspace surface tokens", () => {
  test("keeps YouTube inside the shared Primary and divider contracts", async () => {
    const [appearanceCSS, workspaceAppearanceCSS, pageSource] = await Promise.all([
      Bun.file(new URL("../../shared/styles/dream/youtube.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/workspace.css", import.meta.url)).text(),
      Bun.file(new URL("./YouTubeWorkspacePage.tsx", import.meta.url)).text(),
    ]);

    expect(rule(workspaceAppearanceCSS, ".app-workspace-sidebar")).toContain(
      "background: hsl(var(--sidebar-background))",
    );
    expect(rule(workspaceAppearanceCSS, ".app-workspace-primary-pane")).toContain(
      "background: var(--app-workspace-primary-surface)",
    );
    expect(rule(workspaceAppearanceCSS, ".app-workspace-companion")).toContain(
      "background: hsl(var(--background))",
    );
    expect(rule(workspaceAppearanceCSS, ".app-workspace-companion")).toContain(
      "var(--app-workspace-divider-width) solid",
    );
    expect(rule(workspaceAppearanceCSS, ".app-workspace-companion")).toContain(
      "var(--app-workspace-divider-color)",
    );
    expect(rule(appearanceCSS, ".youtube-workspace-page")).toContain(
      "background: var(--app-workspace-primary-subpane-surface)",
    );
    expect(pageSource).toContain(
      'className="youtube-workspace-page app-workspace-primary-subpane relative"',
    );
    expect(rule(appearanceCSS, ".youtube-workspace-watch-page")).toContain(
      "background: var(--app-workspace-primary-subpane-surface)",
    );
    expect(rule(appearanceCSS, ".youtube-uploader-page")).toContain(
      "background: transparent",
    );
  });

  test("keeps Up Next transparent inside the shared companion glass", async () => {
    const [youtubeCSS, appearanceCSS, workspaceAppearanceCSS, companionSource] = await Promise.all([
      Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/youtube.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/workspace.css", import.meta.url)).text(),
      Bun.file(new URL("../workspace/CompanionPanel.tsx", import.meta.url)).text(),
    ]);

    expect(workspaceAppearanceCSS).toMatch(
      /\.app-main-shell\[data-surface-style="glass"\][\s\S]*?\.app-workspace-companion\[data-glass-host="true"\],[\s\S]*?\.app-workspace-companion\[data-presentation="overlay"\]\[data-glass-host="true"\]\s*\{[^}]*background:\s*transparent/s,
    );
    expect(workspaceAppearanceCSS).not.toContain("--youtube-up-next-companion-surface");
    expect(companionSource).toContain('data-glass-role="companion"');
    expect(rule(appearanceCSS, ".youtube-up-next-companion")).toContain(
      "background: transparent",
    );
    expect(rule(youtubeCSS, ".youtube-up-next-companion-list")).toContain(
      "padding-inline: var(--app-workspace-companion-gutter, 1.25rem)",
    );
  });

  test("keeps the native video hole while matching Browse outside it", async () => {
    const [css, appearanceCSS, workspaceAppearanceCSS] = await Promise.all([
      Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/youtube.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/workspace.css", import.meta.url)).text(),
    ]);
    const activeNativeRule = appearanceCSS.match(
      /:root\[data-listen-native-video-underlay="true"\]\[data-youtube-workspace-video-active="true"\]\s*:is\(([^)]*)\)\s*\{([^}]*)\}/s,
    );

    expect(css).not.toContain("--listen-native-video-surface:");
    expect(appearanceCSS).not.toContain("--listen-native-video-surface:");
    expect(workspaceAppearanceCSS).toMatch(
      /:root\[data-listen-native-video-underlay="true"\]:is\([\s\S]*?\[data-youtube-workspace-video-active="true"\],[\s\S]*?\[data-rss-bilibili-video-active="true"\],[\s\S]*?\[data-rss-site-video-active="true"\][\s\S]*?\)\s*\.app-workspace-primary-pane::before\s*\{[^}]*background:\s*var\(--app-workspace-primary-surface\)[^}]*mask:\s*var\(--listen-native-video-primary-outside-mask\)/s,
    );
    expect(activeNativeRule).not.toBeNull();
    expect(activeNativeRule?.[1]).toContain(".app-workspace-primary-pane");
    expect(activeNativeRule?.[1]).toContain(".youtube-workspace-video-surface");
    expect(activeNativeRule?.[1]).not.toContain(".app-workspace-sidebar");
    expect(activeNativeRule?.[1]).not.toContain(".app-workspace-companion");
    expect(activeNativeRule?.[2]).toContain("background: transparent !important");
    expect(css).not.toMatch(
      /\.app-workspace-companion:not\(\.app-workspace-companion--player-fullscreen\)\s*\{/,
    );
  });
});
