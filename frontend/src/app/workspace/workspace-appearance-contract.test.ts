import { describe, expect, test } from "bun:test";
import postcss, { type Declaration } from "postcss";

const FEATURE_STYLE_PATHS = [
  "./workspace.css",
  "./workspace-navigation.css",
  "./station-dock-editor.css",
] as const;

const FORBIDDEN_APPEARANCE_PROPERTY = /^(?:color|color-scheme|background(?:-.+)?|border(?:-.+)?|outline(?:-.+)?|box-shadow|text-shadow|filter|-webkit-backdrop-filter|backdrop-filter|font|font-.+|line-height|letter-spacing|text-align|text-transform|text-decoration(?:-.+)?|text-overflow|white-space|word-break|overflow-wrap|list-style(?:-.+)?|accent-color|caret-color|fill|stroke(?:-.+)?|mix-blend-mode|transition(?:-.+)?|animation(?:-.+)?|opacity|cursor|resize|forced-color-adjust|appearance|-webkit-appearance|user-select|-webkit-user-select|mask(?:-.+)?|-webkit-mask(?:-.+)?|clip-path|transform-origin)$/;
const FORBIDDEN_WORKSPACE_RECIPE = /^(?:--app-workspace-ambient-.+|--app-main-shell-aba-surface|--app-workspace-chrome-edge-radius|--app-glass-.+|--app-surface-state-shadow)$/;

function belongsToKeyframes(declaration: Declaration): boolean {
  for (
    let parent = declaration.parent;
    parent;
    parent = parent.parent
  ) {
    if (parent.type === "atrule" && /keyframes$/i.test(parent.name)) {
      return true;
    }
  }
  return false;
}

describe("Workspace Dream appearance boundary", () => {
  test("keeps feature CSS limited to shell, pane, scroll and responsive composition", async () => {
    for (const relativePath of FEATURE_STYLE_PATHS) {
      const source = await Bun.file(new URL(relativePath, import.meta.url)).text();
      const root = postcss.parse(source);
      const violations: string[] = [];

      root.walkDecls((declaration) => {
        if (belongsToKeyframes(declaration)) {
          return;
        }
        if (
          FORBIDDEN_APPEARANCE_PROPERTY.test(declaration.prop) ||
          FORBIDDEN_WORKSPACE_RECIPE.test(declaration.prop)
        ) {
          violations.push(
            `${relativePath}:${declaration.source?.start?.line ?? 0} ${declaration.prop}`,
          );
        }
        if (declaration.prop === "transform") {
          violations.push(
            `${relativePath}:${declaration.source?.start?.line ?? 0} visual transform`,
          );
        }
      });

      expect(violations).toEqual([]);
    }
  });

  test("registers Workspace material, selected, focus, typography and accessibility recipes in Dream", async () => {
    const [entry, appearance] = await Promise.all([
      Bun.file(
        new URL("../../shared/styles/dream.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/workspace.css", import.meta.url),
      ).text(),
    ]);

    expect(entry).toContain('@import "./dream/workspace.css";');
    expect(appearance).toContain("--app-workspace-ambient-base");
    expect(appearance).toContain(".app-workspace-primary-pane");
    expect(appearance).toContain(".app-workspace-account-profile:focus-visible");
    expect(appearance).toContain(".app-workspace-companion__title");
    expect(appearance).toContain(
      ".app-workspace-companion__header-material",
    );
    expect(appearance).toContain('[data-scroll-state="scrolled"]');
    expect(appearance).not.toContain(
      ".app-workspace-companion__footer-material",
    );
    expect(appearance).not.toContain("data-footer-state");
    expect(appearance).toContain("@media (forced-colors: active)");
    expect(appearance).toContain("var(--app-media-chrome-foreground)");
    expect(appearance).toContain("var(--app-status-tone-orphan)");
    expect(appearance).not.toMatch(/(?:rgb|hsl)\(\s*\d|#[0-9a-f]{3,8}\b/i);
  });

  test("uses one selected navigation surface and delegates generic editor controls", async () => {
    const [appearance, dockEditorSource, stationEditorSource] =
      await Promise.all([
      Bun.file(
        new URL("../../shared/styles/dream/workspace.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./StationDockEditor.tsx", import.meta.url)).text(),
      Bun.file(new URL("./StationEditor.tsx", import.meta.url)).text(),
    ]);

    expect(appearance).toMatch(
      /\.app-workspace-nav-button\[data-active="true"\][\s\S]*?\{[^}]*var\(--app-accent-on-solid[^}]*var\(--app-accent-solid/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-nav-button\[data-active="true"\]\s+\.app-workspace-nav-button__icon\s*\{[^}]*color:\s*inherit/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-nav-button\[data-active="true"\]\s+\.app-rss-workspace-sidebar__favicon\s*\{[^}]*background:\s*transparent[^}]*color:\s*inherit/s,
    );
    const nestedSurfaceViolations: string[] = [];
    postcss.parse(appearance).walkRules((rule) => {
      const ownsActiveDescendant = rule.selectors.some((selector) =>
        /\.app-workspace-nav-button\[data-active="true"\]\s+/.test(selector),
      );
      if (!ownsActiveDescendant) {
        return;
      }
      rule.walkDecls(/^(?:background(?:-color)?|box-shadow)$/, (declaration) => {
        if (!/^(?:transparent|none)$/.test(declaration.value.trim())) {
          nestedSurfaceViolations.push(
            `${rule.selector} -> ${declaration.prop}: ${declaration.value}`,
          );
        }
      });
    });
    expect(nestedSurfaceViolations).toEqual([]);
    expect(dockEditorSource).toContain("<DreamInlineSwitch");
    expect(dockEditorSource).toContain('size="compactIcon"');
    expect(dockEditorSource).toContain('variant="ghost"');
    expect(stationEditorSource).toContain("<Select");
  });
});
