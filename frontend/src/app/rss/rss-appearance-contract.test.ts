import { describe, expect, test } from "bun:test";
import postcss from "postcss";

const featureStylePaths = [
  "./rss-workspace.css",
  "./rss-organization-manager.css",
] as const;

const forbiddenAppearanceProperty = /^(?:color|color-scheme|background(?:-.+)?|border(?:-.+)?|outline(?:-.+)?|box-shadow|text-shadow|filter|-webkit-backdrop-filter|backdrop-filter|font|font-.+|line-height|letter-spacing|text-align|text-transform|text-decoration(?:-.+)?|accent-color|caret-color|cursor|fill|stroke|mix-blend-mode|animation(?:-.+)?|transition(?:-.+)?|opacity)$/;

describe("RSS Dream appearance boundary", () => {
  test("keeps feature CSS limited to composition and domain geometry", async () => {
    for (const relativePath of featureStylePaths) {
      const source = await Bun.file(new URL(relativePath, import.meta.url)).text();
      const root = postcss.parse(source);
      const violations: string[] = [];

      root.walkDecls((declaration) => {
        if (
          forbiddenAppearanceProperty.test(declaration.prop) ||
          declaration.prop === "border-radius" ||
          ["--rss-edge", "--rss-muted", "--rss-card"].includes(
            declaration.prop,
          )
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

  test("imports and defines RSS palette, status, focus, media and motion in Dream", async () => {
    const [entry, appearance] = await Promise.all([
      Bun.file(
        new URL("../../shared/styles/dream.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/rss.css", import.meta.url),
      ).text(),
    ]);

    expect(entry).toContain('@import "./dream/rss.css";');
    expect(appearance).toContain(".rss-entry-row[aria-current=\"true\"]");
    expect(appearance).toContain(".rss-organization-manager__section");
    expect(appearance).toContain(".rss-entry-row:focus-visible");
    expect(appearance).toContain("@keyframes rss-discovery-shimmer");
    expect(appearance).toContain("var(--app-status-tone-error)");
    expect(appearance).toContain("var(--app-media-chrome-canvas)");
    expect(appearance).toContain("var(--app-media-chrome-foreground)");
    expect(appearance).toContain(".rss-reader-progress__segment");
    expect(appearance).toContain(".rss-organization-manager__empty");
    expect(appearance).not.toMatch(/#[0-9a-f]{3,8}\b/i);
    expect(appearance).not.toMatch(/\b(?:white|black)\b/i);
  });

  test("retains layout ownership and shared primitive boundaries", async () => {
    const [workspaceCSS, organizationCSS, workspace, organization, bilibili] =
      await Promise.all([
        Bun.file(new URL("./rss-workspace.css", import.meta.url)).text(),
        Bun.file(
          new URL("./rss-organization-manager.css", import.meta.url),
        ).text(),
        Bun.file(new URL("./RSSWorkspacePage.tsx", import.meta.url)).text(),
        Bun.file(
          new URL("./RSSOrganizationManager.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("./RSSBilibiliVideoSurface.tsx", import.meta.url),
        ).text(),
      ]);

    expect(workspaceCSS).toContain("container-type: inline-size");
    expect(workspaceCSS).toContain("grid-template-columns:");
    expect(workspaceCSS).toContain("overscroll-behavior: contain");
    expect(organizationCSS).toContain(
      "@container rss-organization-manager (max-width: 540px)",
    );
    expect(workspace).toContain("app-dream-status-message");
    expect(bilibili).toContain('surfaceRole="status"');
    for (const primitive of ["button", "dialog", "input", "select"]) {
      expect(organization).toContain(`@/shared/ui/${primitive}`);
    }
  });
});
