import { describe, expect, test } from "bun:test";
import postcss from "postcss";

const featureStylePaths = [
  "./youtube-workspace.css",
  "./youtube-uploader-page.css",
] as const;

const forbiddenAppearanceProperty = /^(?:color|color-scheme|background(?:-.+)?|border(?:-.+)?|outline(?:-.+)?|box-shadow|text-shadow|filter|-webkit-backdrop-filter|backdrop-filter|font|font-.+|line-height|letter-spacing|text-align|text-transform|text-decoration(?:-.+)?|accent-color|caret-color|cursor|fill|stroke|mix-blend-mode|animation(?:-.+)?|transition(?:-.+)?|opacity)$/;

describe("YouTube Dream appearance boundary", () => {
  test("keeps feature CSS limited to composition and domain geometry", async () => {
    for (const relativePath of featureStylePaths) {
      const source = await Bun.file(new URL(relativePath, import.meta.url)).text();
      const root = postcss.parse(source);
      const violations: string[] = [];

      root.walkDecls((declaration) => {
        if (
          forbiddenAppearanceProperty.test(declaration.prop) ||
          declaration.prop === "border-radius" ||
          declaration.prop === "content"
        ) {
          violations.push(
            `${relativePath}:${declaration.source?.start?.line ?? 0} ${declaration.prop}`,
          );
        }

        if (
          declaration.prop === "transform" &&
          /:(?:hover|focus|focus-visible|focus-within|active)\b/.test(
            declaration.parent?.selector ?? "",
          )
        ) {
          violations.push(
            `${relativePath}:${declaration.source?.start?.line ?? 0} interactive transform`,
          );
        }
      });

      expect(violations).toEqual([]);
    }
  });

  test("imports and defines YouTube media, status, focus and motion in Dream", async () => {
    const [entry, appearance, workspaceAppearance] = await Promise.all([
      Bun.file(
        new URL("../../shared/styles/dream.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/youtube.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/workspace.css", import.meta.url),
      ).text(),
    ]);

    expect(entry).toContain('@import "./dream/youtube.css";');
    expect(appearance).toContain(".youtube-workspace-watch-player-shell");
    expect(appearance).toContain(".youtube-uploader-info-dialog");
    expect(appearance).toContain(".youtube-workspace-video-card__open:focus-visible");
    expect(appearance).toContain("@keyframes youtube-workspace-shimmer");
    expect(appearance).toContain("var(--app-status-tone-error)");
    expect(appearance).toContain("var(--app-media-chrome-canvas)");
    expect(appearance).toContain("var(--app-media-chrome-foreground)");
    expect(appearance).toContain(".youtube-workspace-video-card__open:disabled");
    expect(appearance).toContain(".youtube-uploader-info-dialog-identity");
    expect(appearance).toContain("--app-youtube-brand: rgb(255 0 51);");
    expect(appearance).not.toContain("--listen-native-video-surface:");
    expect(workspaceAppearance).toMatch(
      /:root\[data-listen-native-video-underlay="true"\]:is\([\s\S]*?\[data-youtube-workspace-video-active="true"\],[\s\S]*?\[data-rss-bilibili-video-active="true"\],[\s\S]*?\[data-rss-site-video-active="true"\][\s\S]*?\)\s*\.app-workspace-primary-pane::before\s*\{[^}]*background:\s*var\(--app-workspace-primary-surface\)[^}]*mask:\s*var\(--listen-native-video-primary-outside-mask\)/s,
    );
    expect(appearance).not.toContain(".youtube-workspace-transport-button {");
    expect(appearance).not.toMatch(/^\.youtube-uploader-info-dialog\s*\{/m);
    expect(appearance).not.toMatch(/#[0-9a-f]{3,8}\b/i);
    expect(appearance).not.toMatch(/\b(?:white|black)\b/i);
    expect(appearance).not.toMatch(/rgb\((?:0 0 0|255 255 255)(?:\s*\/|\))/);
  });

  test("retains layout ownership and shared primitive boundaries", async () => {
    const [workspaceCSS, uploaderCSS, workspace, nativeVideo, uploader] =
      await Promise.all([
        Bun.file(new URL("./youtube-workspace.css", import.meta.url)).text(),
        Bun.file(new URL("./youtube-uploader-page.css", import.meta.url)).text(),
        Bun.file(new URL("./YouTubeWorkspacePage.tsx", import.meta.url)).text(),
        Bun.file(
          new URL("./YouTubeNativeVideoSurface.tsx", import.meta.url),
        ).text(),
        Bun.file(new URL("./YouTubeUploaderPage.tsx", import.meta.url)).text(),
      ]);

    expect(workspaceCSS).toContain("grid-template-columns:");
    expect(workspaceCSS).toContain("overscroll-behavior: contain");
    expect(workspaceCSS).toContain(
      "@container youtube-workspace (max-width: 760px)",
    );
    expect(uploaderCSS).toContain("grid-template-columns:");
    expect(uploaderCSS).toContain("overflow-y: auto");
    expect(workspace).toContain("app-dream-status-message");
    expect(workspace).toContain("<StatusBadge");
    expect(workspace).toContain('surfaceRole="status"');
    expect(nativeVideo).toContain('surfaceRole="status"');
    expect(workspace).toContain('@/shared/ui/button');
    expect(uploader).toContain('@/shared/ui/button');
    const transport = await Bun.file(
      new URL("./YouTubeWorkspaceTransportBar.tsx", import.meta.url),
    ).text();
    expect(transport).toContain('@/shared/ui/button');
    expect(transport).not.toContain("<button");
    expect(uploader).toContain('shape="capsule"');
    for (const primitive of ["button", "dialog"]) {
      expect(uploader).toContain(`@/shared/ui/${primitive}`);
    }
  });
});
