import { describe, expect, test } from "bun:test";

import {
  auditCanonicalSurfaceTokens,
  auditContrastCanvasContract,
  auditDreamAppearanceContracts,
  auditFeatureCssAppearanceBoundary,
  auditFeatureMaterialFilters,
  auditForeignProviderAppearanceBoundary,
  auditInlineBackdropBlurSource,
  auditInlineStaticStyleSource,
  auditRSSDocumentStyleContract,
  auditStaticCssStringSource,
  auditStaticElementStyleAssignmentSource,
  auditStyleEntrypointBoundary,
  auditTailwindCompositionConfigSource,
  auditTailwindAppearanceUtilitiesSource,
  auditOverlayRoleConsumers,
  auditPrimaryHeaderContract,
  auditProductSurfaceEntries,
  auditRoleOwnedFeatureSource,
  auditSharedWindowCanvasContract,
  auditSurfaceContractSource,
  auditWorkspaceContentControlContracts,
  auditWorkspacePageLayoutOwnership,
  auditWorkspacePaneAppearanceContract,
  auditWorkspaceStructuralOverrides,
  auditWorkspaceStatusRole,
} from "./audit-appearance.mjs";

const foreignProviderBridgeSources = new Map([
  [
    "internal/presentation/wails/listen_player_handler.go",
    "video { object-fit: contain; background: #000; }",
  ],
  [
    "internal/presentation/wails/listen_live_player_handler.go",
    "body { visibility: hidden; background: #000; }",
  ],
  [
    "internal/presentation/wails/rss_video_transport_bridge.go",
    "video { position: fixed; pointer-events: none; }",
  ],
]);

const canonicalTokenSource = `
  :root {
    --app-surface-window-canvas: Canvas;
    --app-surface-window-glass-wash: Canvas;
    --app-surface-canvas: Canvas;
    --app-surface-status-fill: Canvas;
    --app-surface-status-filter: none;
    --app-surface-status-line: CanvasText;
    --app-surface-status-shadow: none;
    --app-surface-status-specular: none;
    --app-surface-status-specular-opacity: 0;
    --app-surface-status-artwork-opacity: 0;
    --app-surface-status-artwork-filter: none;
    --app-surface-status-artwork-veil: none;
    --app-surface-overlay-fill: Canvas;
    --app-surface-overlay-line: CanvasText;
    --app-surface-overlay-shadow: none;
    --app-surface-card-fill: Canvas;
    --app-surface-card-line: CanvasText;
    --app-surface-inset-fill: Canvas;
    --app-surface-control-fill: Canvas;
  }
`;

const canonicalContractSource = `
  export const XIA_SURFACE_ROLES = [
    "canvas",
    "chrome",
    "content",
    "status",
    "overlay",
    "card",
    "inset",
    "control",
  ] as const;
  export type XiaSurfaceRole = (typeof XIA_SURFACE_ROLES)[number];

  export const XIA_SURFACE_ROLE_PRESETS = {
    canvas: { material: "clear" },
    chrome: { material: "regular" },
    content: { material: "regular" },
    status: { material: "regular" },
    overlay: { material: "panel" },
    card: { material: "regular" },
    inset: { material: "solid" },
    control: { material: "regular" },
  };

  export type XiaSurfaceAttributes = {};
  export function getXiaSurfaceAttributes(role) {
    return {
      "data-surface-role": role,
      "data-material": XIA_SURFACE_ROLE_PRESETS[role].material,
    };
  }
`;

describe("appearance Surface Contract audit", () => {
  test("keeps foreign provider media isolation outside Dream appearance", () => {
    expect(
      auditForeignProviderAppearanceBoundary(foreignProviderBridgeSources),
    ).toEqual([]);
    const invalid = new Map(foreignProviderBridgeSources);
    invalid.set(
      "internal/presentation/wails/listen_player_handler.go",
      ".app-dream-button { color: var(--app-accent-text); }",
    );
    expect(
      auditForeignProviderAppearanceBoundary(invalid).join("\n"),
    ).toContain(
      "foreign-provider transport CSS must not contain Dream appearance marker",
    );
  });

  test("keeps the Primary header recipe in Dream CSS with shared consumers", async () => {
    const layout = await Bun.file(
      new URL("../src/shared/styles/dream/layout-contract.css", import.meta.url),
    ).text();
    const paths = [
      "app/library/LibraryWorkspacePage.tsx",
      "app/main/RunningPage.tsx",
      "features/settings/app-sessions/index.tsx",
      "app/pets-gallery/PetsGalleryPage.tsx",
      "app/rss/RSSWorkspacePage.tsx",
      "app/rss/RSSAddSubscriptionPage.tsx",
      "app/youtube/YouTubeWorkspacePage.tsx",
      "app/main/listen/PageView.tsx",
      "app/sniff-desk/SniffDeskPage.tsx",
      "app/settings/SettingsApp.tsx",
    ];
    const sources = new Map(
      await Promise.all(
        paths.map(async (relative) => [
          relative,
          await Bun.file(new URL(`../src/${relative}`, import.meta.url)).text(),
        ] as const),
      ),
    );

    expect(auditPrimaryHeaderContract(layout, sources)).toEqual([]);
    expect(
      auditPrimaryHeaderContract(
        `${layout}\n.app-workspace-page__footer { border-block-start: 1px solid; }`,
        sources,
      ).join("\n"),
    ).toContain("page footers must use spacing without a content divider");
  });

  test("keeps Dream geometry, semantic status, primitive, and guide contracts together", async () => {
    const [tokens, statusCSS, statusBadge, guide] = await Promise.all([
      Bun.file(new URL("../src/shared/styles/dream/tokens.css", import.meta.url)).text(),
      Bun.file(new URL("../src/shared/styles/dream/status-contract.css", import.meta.url)).text(),
      Bun.file(new URL("../src/shared/ui/status-badge.tsx", import.meta.url)).text(),
      Bun.file(new URL("../src/shared/ui/APPEARANCE_CONTRACT.md", import.meta.url)).text(),
    ]);

    expect(
      auditDreamAppearanceContracts(tokens, statusCSS, statusBadge, guide),
    ).toEqual([]);
    expect(
      auditDreamAppearanceContracts(
        tokens.replace(
          "--app-selection-list-inset: var(--app-space-1);",
          "--app-selection-list-inset: var(--app-space-3);",
        ),
        statusCSS,
        statusBadge,
        guide,
      ).join("\n"),
    ).toContain("selection list inset must use the compact --app-space-1 geometry");
  });

  test("requires the complete functional token vocabulary", () => {
    expect(auditCanonicalSurfaceTokens(canonicalTokenSource)).toEqual([]);
    expect(
      auditCanonicalSurfaceTokens(
        canonicalTokenSource.replace("--app-surface-card-fill", "--legacy-card-fill"),
      ),
    ).toContain(
      "shared/styles/dream/tokens.css: missing canonical surface token --app-surface-card-fill",
    );
  });

  test("requires one typed preset map with status and overlay recipes", () => {
    expect(auditSurfaceContractSource(canonicalContractSource)).toEqual([]);
    expect(
      auditSurfaceContractSource(
        canonicalContractSource.replace(
          'overlay: { material: "panel" }',
          'overlay: { material: "regular" }',
        ),
      ).join("\n"),
    ).toContain("overlay must resolve through the canonical panel preset");
    expect(
      auditSurfaceContractSource(
        canonicalContractSource.replace('    "control",\n', ""),
      ).join("\n"),
    ).toContain("XIA_SURFACE_ROLES is missing control");
  });

  test("keeps every persistent Contrast pane on one canvas", () => {
    const valid = `
      :root {
        --app-workspace-primary-surface: var(--app-surface-canvas);
      }
      .app-main-shell[data-surface-style="contrast"] .app-workspace-sidebar {
        background: var(--app-surface-canvas);
      }
      .app-main-shell[data-surface-style="contrast"] .app-workspace-primary-pane {
        background: var(--app-workspace-primary-surface);
      }
      .app-main-shell[data-surface-style="contrast"]
        .app-workspace-companion[data-presentation="docked"] {
        background: var(--app-surface-canvas);
      }
    `;

    expect(auditContrastCanvasContract(valid, valid)).toEqual([]);
    expect(
      auditContrastCanvasContract(
        valid.replace(
          "background: var(--app-surface-canvas);",
          "background: hsl(var(--sidebar-background));",
        ),
        valid,
      ).join("\n"),
    ).toContain("Contrast Sidebar must consume");
  });

  test("governs pane opacity, dividers, Access geometry, and one navigation selection surface", async () => {
    const [tokens, layout, workspace, appearance, mainApp, guide] =
      await Promise.all([
        Bun.file(new URL("../src/shared/styles/dream/tokens.css", import.meta.url)).text(),
        Bun.file(new URL("../src/shared/styles/dream/layout-contract.css", import.meta.url)).text(),
        Bun.file(new URL("../src/app/workspace/workspace.css", import.meta.url)).text(),
        Bun.file(new URL("../src/shared/styles/dream/workspace.css", import.meta.url)).text(),
        Bun.file(new URL("../src/app/main/MainApp.tsx", import.meta.url)).text(),
        Bun.file(new URL("../src/shared/ui/APPEARANCE_CONTRACT.md", import.meta.url)).text(),
      ]);

    expect(
      auditWorkspacePaneAppearanceContract(
        tokens,
        layout,
        `${workspace}\n${appearance}`,
        appearance,
        mainApp,
        guide,
      ),
    ).toEqual([]);

    expect(
      auditWorkspacePaneAppearanceContract(
        tokens,
        layout,
        `${workspace}\n${appearance}`,
        appearance.replace("var(--app-accent-on-solid", "var(--sidebar-foreground"),
        mainApp,
        guide,
      ).join("\n"),
    ).toContain("one accent-solid surface and accent-on-solid foreground");

    expect(
      auditWorkspacePaneAppearanceContract(
        tokens,
        layout,
        `${workspace.replaceAll(
          "height: var(--app-menu-action-height);",
          "height: 28px;",
        )}\n${appearance}`,
        appearance,
        mainApp,
        guide,
      ).join("\n"),
    ).toContain("Access name/value row and account actions must share menu geometry");
  });

  test("rejects local Primary paint, duplicate dividers, and nested active navigation tiles", async () => {
    const paths = [
      "shared/styles/dream/layout-contract.css",
      "shared/styles/dream/shell.css",
      "shared/styles/dream/components.css",
      "shared/styles/dream/workspace.css",
      "app/workspace/workspace.css",
      "app/workspace/workspace-navigation.css",
      "app/library/library.css",
      "app/rss/rss-workspace.css",
      "app/main/listen/listen.css",
      "app/youtube/youtube-workspace.css",
    ];
    const sources = new Map(
      await Promise.all(
        paths.map(async (relative) => [
          relative,
          await Bun.file(new URL(`../src/${relative}`, import.meta.url)).text(),
        ] as const),
      ),
    );

    expect(auditWorkspaceStructuralOverrides(sources)).toEqual([]);
    expect(auditWorkspacePageLayoutOwnership(sources)).toEqual([]);

    const withPageGridOverride = new Map(sources);
    withPageGridOverride.set(
      "app/youtube/youtube-workspace.css",
      `${sources.get("app/youtube/youtube-workspace.css")}\n.youtube-workspace-page { display: flex; overflow: auto; }`,
    );
    expect(
      auditWorkspacePageLayoutOwnership(withPageGridOverride).join("\n"),
    ).toContain("must not override WorkspacePage-owned display");

    const withLocalPaint = new Map(sources);
    withLocalPaint.set(
      "app/library/library.css",
      `${sources.get("app/library/library.css")}\n.app-library-primary-surface { background: hsl(var(--background) / 0.72); }`,
    );
    expect(auditWorkspaceStructuralOverrides(withLocalPaint).join("\n")).toContain(
      "must not repaint the Primary host background",
    );

    const withDuplicateDivider = new Map(sources);
    withDuplicateDivider.set(
      "shared/styles/dream/shell.css",
      `${sources.get("shared/styles/dream/shell.css")}\n.app-main-list-pane { border-inline-end: 1px solid red; }`,
    );
    expect(
      auditWorkspaceStructuralOverrides(withDuplicateDivider).join("\n"),
    ).toContain("must not own an extra Primary divider");

    const withNestedTile = new Map(sources);
    withNestedTile.set(
      "app/workspace/workspace-navigation.css",
      `${sources.get("app/workspace/workspace-navigation.css")}\n.app-workspace-nav-button[data-active="true"] .app-workspace-nav-button__icon { background: red; }`,
    );
    expect(auditWorkspaceStructuralOverrides(withNestedTile).join("\n")).toContain(
      "selected navigation descendants must not add another active surface",
    );
  });

  test("aliases Settings and Main Contrast to the existing tinted window canvas", () => {
    const tokens = `
      :root {
        --app-surface-window-canvas:
          linear-gradient(
            hsl(var(--dream-shell-top)),
            hsl(var(--dream-shell-mid) / 0.54),
            hsl(var(--dream-shell-bottom) / 0.10)
          ),
          hsl(var(--dream-shell-mid) / 0.54);
        --app-main-shell-aba-surface: var(--app-surface-window-canvas);
      }
      :root[data-xiadown-surface-style="contrast"],
      :where([data-surface-style="contrast"]) {
        --app-surface-canvas: var(--app-surface-window-canvas);
      }
    `;
    const settings = `
      .app-settings-window.app-dream-window[data-surface-style="contrast"] {
        background: var(--app-surface-canvas);
      }
    `;

    expect(auditSharedWindowCanvasContract(tokens, settings)).toEqual([]);
    expect(
      auditSharedWindowCanvasContract(
        tokens.replace(
          "--app-main-shell-aba-surface: var(--app-surface-window-canvas)",
          "--app-main-shell-aba-surface: hsl(var(--background))",
        ),
        settings,
      ).join("\n"),
    ).toContain("legacy ABA surface must alias");
    expect(
      auditSharedWindowCanvasContract(
        tokens,
        settings.replace(
          "background: var(--app-surface-canvas)",
          "background: hsl(var(--background))",
        ),
      ).join("\n"),
    ).toContain("Settings Contrast must consume");
  });

  test("accepts canonical overlay consumers without exposing material choice", () => {
    const sources = new Map([
      [
        "shared/ui/dropdown-menu.tsx",
        'const attrs = getXiaSurfaceAttributes("overlay");',
      ],
      ["shared/ui/sheet.tsx", '<DialogContent surfaceRole="overlay" />'],
      ["shared/ui/dialog.tsx", '<BaseContent data-surface-role="overlay" />'],
    ]);

    expect(auditOverlayRoleConsumers(sources)).toEqual([]);
    sources.set("shared/ui/sheet.tsx", "<DialogContent />");
    expect(auditOverlayRoleConsumers(sources).join("\n")).toContain(
      "shared/ui/sheet.tsx: must consume the canonical overlay surface role",
    );
  });

  test("routes running, player and sniff variants through one status boundary", () => {
    const source = `
      function WorkspaceStatusSurface() {
        return <GlassSurface surfaceRole="status" />;
      }
      export function WideSniffActivity() {
        return <WorkspaceStatusSurface />;
      }
      export function SniffWorkspaceSessionActivity() {
        return <WorkspaceStatusSurface />;
      }
      export function WidePlaybackActivity() {
        return <WorkspaceStatusSurface />;
      }
      export function WideOperationActivity() {
        return <WorkspaceStatusSurface />;
      }
    `;

    expect(auditWorkspaceStatusRole(source)).toEqual([]);
    expect(
      auditWorkspaceStatusRole(
        source.replace(
          "export function WideSniffActivity() {\n        return <WorkspaceStatusSurface />;",
          "export function WideSniffActivity() {\n        return <GlassSurface />;",
        ),
      ).join("\n"),
    ).toContain("WideSniffActivity must render through WorkspaceStatusSurface");
  });

  test("keeps station Search and RSS row selection on shared content roles", () => {
    const sources = new Map([
      [
        "shared/ui/workspace-search-control.tsx",
        `
          export const WorkspaceSearchControl = () => (
            <form className="app-dream-workspace-search app-dream-search-control app-dream-control-shell app-station-search-content-search">
              <Search className="app-dream-workspace-search__icon" />
              <Input className="app-dream-workspace-search__input" />
              <button className="app-dream-workspace-search__clear" />
              <Button className="app-dream-workspace-search__submit" />
            </form>
          );
        `,
      ],
      ["app/rss/RSSAddSubscriptionPage.tsx", "<WorkspaceSearchControl />"],
      ["app/library/LibraryWorkspacePage.tsx", "<WorkspaceSearchControl />"],
      ["app/main/listen/PageView.tsx", "<WorkspaceSearchControl />"],
      ["app/youtube/YouTubeWorkspacePage.tsx", "<WorkspaceSearchControl />"],
      [
        "app/rss/RSSWorkspacePage.tsx",
        'className="rss-entry-list app-dream-selection-list" className="rss-entry-row app-dream-selection-item"',
      ],
    ]);
    const controls = `
      .app-dream-selection-list {
        padding-inline: var(--app-selection-list-inset);
        scroll-padding-inline: var(--app-selection-list-inset);
        scrollbar-gutter: stable;
      }
    `;

    expect(auditWorkspaceContentControlContracts(sources, controls)).toEqual([]);
    expect(
      auditWorkspaceContentControlContracts(
        sources,
        controls.replace("scrollbar-gutter: stable;", "scrollbar-gutter: stable both-edges;"),
      ).join("\n"),
    ).toContain("one stable scrollbar gutter");
    sources.set("app/youtube/YouTubeWorkspacePage.tsx", "<form />");
    expect(
      auditWorkspaceContentControlContracts(sources, controls).join("\n"),
    ).toContain("YouTubeWorkspacePage.tsx: Search route must render");
  });

  test("rejects feature-owned material selection and blur", () => {
    const findings = auditRoleOwnedFeatureSource(
      `
        const props = { material: "panel" };
        const style = { backdropFilter: "blur(24px)" };
      `,
      "feature.tsx",
    );

    expect(findings).toHaveLength(2);
    expect(findings.join("\n")).toContain("must declare a canonical role");
    expect(findings.join("\n")).toContain("must not define blur");
  });

  test("keeps every positive backdrop material filter in Dream CSS", () => {
    expect(
      auditFeatureMaterialFilters(
        ".feature { backdrop-filter: var(--app-glass-panel-filter); }",
        "app/feature.css",
      ).join("\n"),
    ).toContain("feature CSS must not choose a backdrop material filter");
    expect(
      auditFeatureMaterialFilters(
        ".artwork { backdrop-filter: saturate(0.92); }",
        "app/feature.css",
      ).join("\n"),
    ).toContain("feature CSS must not choose a backdrop material filter");
    expect(
      auditFeatureMaterialFilters(
        ".subpane { backdrop-filter: none; -webkit-backdrop-filter: none; }",
        "app/workspace/workspace.css",
      ),
    ).toEqual([]);
    expect(
      auditFeatureMaterialFilters(
        ".app-glass-surface { backdrop-filter: var(--app-glass-filter); }",
        "shared/styles/dream/glass.css",
      ),
    ).toEqual([]);
  });

  test("keeps feature stylesheets composition-only and Dream-inventoriable", () => {
    expect(
      auditFeatureCssAppearanceBoundary(
        ".layout { display: grid; grid-template-columns: 1fr 2fr; gap: 1rem; }",
        "app/feature.css",
      ),
    ).toEqual([]);
    expect(
      auditFeatureCssAppearanceBoundary(
        "@keyframes pulse { from { opacity: 0; } to { opacity: 1; } }",
        "app/feature.css",
      ),
    ).toEqual([]);

    const findings = auditFeatureCssAppearanceBoundary(
      ".card { background: red; border-radius: 1rem; --feature-surface: blue; }",
      "app/feature.css",
    );
    expect(findings).toHaveLength(3);
    expect(findings.join("\n")).toContain("appearance property background");
    expect(findings.join("\n")).toContain("appearance property border-radius");
    expect(findings.join("\n")).toContain("visual custom property --feature-surface");
    expect(
      auditFeatureCssAppearanceBoundary(
        ".card { background: red; }",
        "shared/styles/dream/feature.css",
      ),
    ).toEqual([]);
  });

  test("keeps index.css as an import and Tailwind assembly boundary", () => {
    const entrypoint = `
      @import "./shared/styles/dream.css";
      @import "vendor.css";
      @tailwind base;
      @tailwind components;
      @tailwind utilities;
    `;
    expect(auditStyleEntrypointBoundary(entrypoint)).toEqual([]);
    const findings = auditStyleEntrypointBoundary(
      `${entrypoint}\n@layer base { body { color: red; } }`,
    );
    expect(findings.join("\n")).toContain("must not define CSS rules");
    expect(findings.join("\n")).toContain("must not define color");
    expect(findings.join("\n")).toContain("allows only @import and @tailwind");
  });

  test("keeps Tailwind as a composition-only compiler", () => {
    const config = `
      export default {
        content: ["./src/**/*.{ts,tsx}"],
        theme: {},
        plugins: [],
      };
    `;
    expect(auditTailwindCompositionConfigSource(config)).toEqual([]);
    expect(
      auditTailwindCompositionConfigSource(
        config.replace("theme: {}", "theme: { extend: { colors: { brand: 'red' } } }"),
      ).join("\n"),
    ).toContain("Tailwind theme must stay empty");
    expect(
      auditTailwindCompositionConfigSource(
        config.replace("plugins: []", "plugins: [visualPlugin]"),
      ).join("\n"),
    ).toContain("Tailwind plugins must stay empty");
  });

  test("rejects static inline backdrop blur recipes across product TypeScript", () => {
    const findings = auditInlineBackdropBlurSource(
      `
        const style = {
          backdropFilter: "blur(9px) saturate(1.08)",
          WebkitBackdropFilter: 'blur(9px)',
        };
      `,
      "app/feature.tsx",
    );

    expect(findings).toHaveLength(2);
    expect(findings.join("\n")).toContain(
      "static inline backdrop blur recipes must be defined in Dream CSS",
    );
    expect(
      auditInlineBackdropBlurSource(
        `
          const style = {
            backdropFilter: "var(--app-glass-filter)",
            WebkitBackdropFilter: resolvedFilter,
          };
        `,
        "app/feature.ts",
      ),
    ).toEqual([]);
  });

  test("rejects static React style values while allowing data-driven values", () => {
    const findings = auditInlineStaticStyleSource(
      `
        const cardStyle: React.CSSProperties = {
          backgroundImage: "linear-gradient(red, blue)",
          borderRadius: 18,
          visibility: ready ? "visible" : "hidden",
          width: dynamicWidth,
        };
        const tintStyle: React.CSSProperties = {
          backgroundColor: \`${"${swatch}"}20\`,
        };
        const view = <div style={{ color: "white", left: x }} />;
      `,
      "app/feature.tsx",
    );

    expect(findings).toHaveLength(5);
    expect(findings.join("\n")).toContain("static backgroundImage");
    expect(findings.join("\n")).toContain("static borderRadius");
    expect(findings.join("\n")).toContain("static visibility");
    expect(findings.join("\n")).toContain(
      "encodes alpha in dynamic backgroundColor",
    );
    expect(findings.join("\n")).toContain("static color");
    expect(
      auditInlineStaticStyleSource(
        `
          const dynamicStyle: React.CSSProperties = {
            backgroundImage: \`url(\${artworkURL})\`,
            left: x,
            "--progress": \`\${progress}%\`,
          };
        `,
        "app/feature.ts",
      ),
    ).toEqual([]);
  });

  test("rejects visual Tailwind utilities only in actual class contexts", () => {
    const findings = auditTailwindAppearanceUtilitiesSource(
      `
        import classNames from "clsx";
        import { cva as variants } from "class-variance-authority";
        const prose = "bg-red-500 text-sm rounded-xl";
        const rowClassName = classNames(
          "flex min-w-0 gap-2 overflow-auto app-result-row text-copy",
          active && "sm:data-[state=open]:!bg-red-500",
          { "[&>svg]:stroke-current": active },
        );
        const tile = variants("grid rounded-xl font-medium", {
          variants: {
            intent: {
              neutral: "text-sm shadow-soft",
              danger: "dark:hover:text-red-500",
            },
          },
          compoundVariants: [
            { intent: "danger", className: "border ring-2 transition-colors" },
          ],
        });
        const view = (
          <div
            className={cn("h-8 w-8 p-2 [&_[data-slot=icon]]:fill-current", rowClassName)}
            title="cursor-pointer opacity-50"
          />
        );
      `,
      "app/feature.tsx",
    );

    expect(findings.map((finding) => finding.match(/utility ("[^"]+")/)?.[1])).toEqual([
      '"sm:data-[state=open]:!bg-red-500"',
      '"[&>svg]:stroke-current"',
      '"rounded-xl"',
      '"font-medium"',
      '"text-sm"',
      '"shadow-soft"',
      '"dark:hover:text-red-500"',
      '"border"',
      '"ring-2"',
      '"transition-colors"',
      '"[&_[data-slot=icon]]:fill-current"',
    ]);
    expect(findings.every((finding) => /^app\/feature\.tsx:\d+:/.test(finding))).toBe(true);
    expect(findings.join("\n")).not.toContain("text-copy");
    expect(findings.join("\n")).not.toContain("cursor-pointer");
    expect(findings.join("\n")).not.toContain("opacity-50");
  });

  test("handles arbitrary visual properties, important prefixes, templates, and class maps", () => {
    const findings = auditTailwindAppearanceUtilitiesSource(
      `
        const ICON_CLASSES = {
          idle: "size-4 app-icon",
          active: "hover:[background-color:red] !opacity-80",
        };
        const dynamicClassName = \`text-\${tone} app-tone-\${tone}\`;
        const view = <div contentClassName="md:[&>span]:font-semibold absolute inset-0" />;
      `,
      "app/feature.tsx",
    );

    expect(findings.join("\n")).toContain('"hover:[background-color:red]"');
    expect(findings.join("\n")).toContain('"!opacity-80"');
    expect(findings.join("\n")).toContain('"text-${…}"');
    expect(findings.join("\n")).toContain('"md:[&>span]:font-semibold"');
    expect(findings.join("\n")).not.toContain("size-4");
    expect(findings.join("\n")).not.toContain("absolute");
    expect(findings.join("\n")).not.toContain("inset-0");
  });

  test("rejects static element.style writes outside controlled theme interfaces", () => {
    const findings = auditStaticElementStyleAssignmentSource(
      `
        node.style.color = "red";
        node.style["opacity"] = 0.5;
        node.style.colorScheme = dark ? "dark" : "light";
        node.style.setProperty("--card-color", "red");
        node.style.width = measuredWidth;
      `,
      "app/feature.ts",
    );
    expect(findings).toHaveLength(4);
    expect(findings.join("\n")).toContain("element.style.color");
    expect(findings.join("\n")).toContain('element.style."opacity"');
    expect(findings.join("\n")).toContain("element.style.colorScheme");
    expect(findings.join("\n")).toContain('setProperty("--card-color", ...)');
    expect(findings.join("\n")).not.toContain("width");
    expect(
      auditStaticElementStyleAssignmentSource(
        'document.documentElement.style.colorScheme = dark ? "dark" : "light";',
        "shared/styles/theme-runtime.ts",
      ),
    ).toEqual([]);
    expect(
      auditStaticElementStyleAssignmentSource(
        'root.style.colorScheme = appearance;',
        "app/dev/AppearanceLab.tsx",
      ),
    ).toEqual([]);
    expect(
      auditStaticElementStyleAssignmentSource(
        'document.documentElement.style.background = "red";',
        "shared/styles/theme-runtime.ts",
      ).join("\n"),
    ).toContain("element.style.background");
  });

  test("rejects static CSS payload strings and governs the RSS raw module bridge", () => {
    expect(
      auditStaticCssStringSource(
        'const readerStyles = ["body{color:red}", ".card{border-radius:8px}"];',
        "app/feature.ts",
      ),
    ).toHaveLength(2);
    expect(
      auditStaticCssStringSource(
        'const document = `<style>:root{color-scheme:dark}</style>`;',
        "app/feature.ts",
      ).join("\n"),
    ).toContain("inline <style> payload");
    expect(
      auditStaticCssStringSource(
        'const document = `<style>${dreamStyles}</style>`;',
        "app/feature.ts",
      ),
    ).toEqual([]);

    const source = `
      import rssDocumentStyles from "../../shared/styles/dream/rss-documents.css?raw";
      const readerStyleProperties = [
        "--app-rss-reader-font-size:16px",
        "--app-rss-reader-line-height:1.75",
        "--app-rss-reader-paragraph-spacing:1.25em",
      ];
      const reader = \`<html class="app-rss-reader-document" style="\${readerStyleProperties}"><style>\${rssDocumentStyles}</style></html>\`;
      const print = \`<html class="app-rss-print-document"><style>\${rssDocumentStyles}</style></html>\`;
    `;
    const css = `
      html.app-rss-reader-document body {
        font-size: var(--app-rss-reader-font-size);
        line-height: var(--app-rss-reader-line-height);
      }
      html.app-rss-reader-document p {
        margin-bottom: var(--app-rss-reader-paragraph-spacing);
      }
      html.app-rss-print-document body { color: CanvasText; }
    `;
    expect(auditRSSDocumentStyleContract(source, css)).toEqual([]);
    expect(
      auditRSSDocumentStyleContract(
        source,
        `${css}\nbody { color: red; }`,
      ).join("\n"),
    ).toContain("selectors must stay scoped");
  });

  test("requires role-first Glass entries across product TSX", () => {
    expect(
      auditProductSurfaceEntries(
        '<GlassSurface surfaceRole="status" shape="card" />',
        "feature.tsx",
      ),
    ).toEqual([]);

    const findings = auditProductSurfaceEntries(
      '<GlassGroup material="regular"><span /></GlassGroup>',
      "feature.tsx",
    );
    expect(findings.join("\n")).toContain(
      "product code must declare surfaceRole instead of a literal material",
    );
    expect(findings.join("\n")).toContain(
      "GlassSurface and GlassGroup entries must declare surfaceRole",
    );
  });

  test("requires raw glass class entries to resolve a material", () => {
    const missingMaterial = auditProductSurfaceEntries(
      `
        <div
          className="app-glass-surface feature-status"
          data-surface-role="status"
          role="status"
        />
      `,
      "app/feature.tsx",
    );
    expect(missingMaterial.join("\n")).toContain(
      "raw app-glass-surface entries must resolve their material",
    );

    expect(
      auditProductSurfaceEntries(
        `
          <div
            className="app-glass-surface feature-status"
            {...getXiaSurfaceAttributes("status")}
          />
        `,
        "app/feature.tsx",
      ),
    ).toEqual([]);
    const explicitMaterial = auditProductSurfaceEntries(
      '<div className="app-glass-surface" data-material="regular" />',
      "app/feature.tsx",
    );
    expect(explicitMaterial.join("\n")).not.toContain(
      "raw app-glass-surface entries must resolve their material",
    );
    expect(explicitMaterial.join("\n")).toContain(
      "product code must declare surfaceRole instead of a literal material",
    );
  });

  test("allows only the named Appearance Lab material specimens", () => {
    const specimen = `
      <GlassSurface
        data-appearance-material-specimen="regular"
        material="regular"
      />
      <GlassSurface
        data-appearance-material-specimen="panel"
        material="panel"
      />
      <GlassSurface
        data-appearance-material-specimen="clear"
        material="clear"
      />
      <GlassSurface surfaceRole="control" />
    `;
    expect(
      auditProductSurfaceEntries(specimen, "app/dev/AppearanceLab.tsx"),
    ).toEqual([]);
    expect(
      auditProductSurfaceEntries(
        `${specimen}\n<GlassSurface material="regular" />`,
        "app/dev/AppearanceLab.tsx",
      ).join("\n"),
    ).toContain("product code must declare surfaceRole");
  });
});
