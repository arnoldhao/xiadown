import { describe, expect, test } from "bun:test";
import postcss, { type Declaration } from "postcss";

const FEATURE_STYLE_PATHS = [
  "./library.css",
  "./LibraryDataManagement.css",
] as const;

const FORBIDDEN_APPEARANCE_PROPERTY = /^(?:color|color-scheme|background(?:-.+)?|border(?:-.+)?|outline(?:-.+)?|box-shadow|text-shadow|filter|-webkit-backdrop-filter|backdrop-filter|font|font-.+|line-height|letter-spacing|text-align|text-transform|text-decoration(?:-.+)?|accent-color|caret-color|fill|stroke(?:-.+)?|mix-blend-mode|transition(?:-.+)?|opacity|cursor|resize|forced-color-adjust|appearance)$/;
const FORBIDDEN_LIBRARY_RECIPE = /^(?:--app-library-material-|--app-library-task-folder-paper)/;

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

describe("Library Dream appearance boundary", () => {
  test("keeps feature CSS limited to composition, responsive layout, and domain animation", async () => {
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
          FORBIDDEN_LIBRARY_RECIPE.test(declaration.prop)
        ) {
          violations.push(
            `${relativePath}:${declaration.source?.start?.line ?? 0} ${declaration.prop}`,
          );
        }
        if (
          declaration.prop === "transform" &&
          /:(?:hover|focus|focus-visible|focus-within|active)\b|\[data-(?:active|selected)/.test(
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

  test("registers Library material, focus, status, control, and accessibility recipes in Dream", async () => {
    const [entry, appearance] = await Promise.all([
      Bun.file(
        new URL("../../shared/styles/dream.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/library.css", import.meta.url),
      ).text(),
    ]);

    expect(entry).toContain('@import "./dream/library.css";');
    expect(appearance).toContain("--app-library-material-face");
    expect(appearance).toContain("--app-library-primary-surface:");
    expect(appearance).toContain(".app-library-task-folder__front-cover");
    expect(appearance).toContain(".app-library-search:focus-within");
    expect(appearance).toContain(".library-data-management__notice");
    expect(appearance).toContain("var(--app-status-surface-success)");
    expect(appearance).toContain("var(--app-status-tone-error)");
    expect(appearance).toContain("var(--app-status-surface-orphan)");
    expect(appearance).toContain("@media (forced-colors: active)");
    expect(appearance).not.toMatch(/hsl\((?:35|38|145)\b/);
  });

  test("retains Library composition and delegates compact statuses to the shared primitive", async () => {
    const [workspaceLayout, managementLayout, managementSource] =
      await Promise.all([
        Bun.file(new URL("./library.css", import.meta.url)).text(),
        Bun.file(
          new URL("./LibraryDataManagement.css", import.meta.url),
        ).text(),
        Bun.file(
          new URL("./LibraryDataManagement.tsx", import.meta.url),
        ).text(),
      ]);

    expect(workspaceLayout).toContain("container: app-library-primary / inline-size");
    expect(workspaceLayout).toContain("grid-template-columns:");
    expect(workspaceLayout).toContain("@keyframes app-library-preview-title-bounce");
    expect(managementLayout).toContain(
      "container: library-data-management / inline-size",
    );
    expect(managementLayout).toContain(
      "@container library-data-management (max-width: 40rem)",
    );
    expect(managementSource).toContain("<StatusBadge");
    expect(managementSource).toContain("resolveImportBatchStatusTone");
    expect(managementSource).toContain("resolveImportCandidateStatusTone");
  });
});
