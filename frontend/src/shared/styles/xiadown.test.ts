import { describe, expect, test } from "bun:test";

import { resolveXiaMainSidebarSurface } from "./xiadown";

describe("XiaDown style helpers", () => {
  test("resolves the sidebar surface from the window-wide style", () => {
    expect(resolveXiaMainSidebarSurface("citrus", "contrast", "dream")).toBe(
      "app-main-sidebar-surface app-main-sidebar-surface--contrast",
    );
    const glassSurface = resolveXiaMainSidebarSurface(
      "citrus",
      "glass",
      "dream",
    );
    expect(glassSurface).toBe(
      "app-main-sidebar-surface app-main-sidebar-surface--glass",
    );
    expect(resolveXiaMainSidebarSurface("pixel", "glass", "dream")).toBe(
      glassSurface,
    );
  });

  test("keeps visual recipes out of the TypeScript style vocabulary", async () => {
    const [source, anatomy] = await Promise.all([
      Bun.file(new URL("./xiadown.ts", import.meta.url)).text(),
      Bun.file(new URL("./dream/anatomy.css", import.meta.url)).text(),
    ]);

    for (const prohibited of [
      /React\.CSSProperties/,
      /backgroundImage/,
      /(?:Webkit)?maskImage/,
      /(?:bg|border|ring|rounded|shadow|text|w|h)-\[/,
      /\b(?:bg|border|shadow|text)-(?:background|foreground|muted|primary|secondary|destructive)\b/,
    ]) {
      expect(source).not.toMatch(prohibited);
    }
    expect(source).not.toContain("resolvePetCardLighting");
    expect(source).not.toContain("PET_DISPLAY_GLOW_STYLE");
    expect(source).not.toContain("RUNNING_PET_GLOW_STYLE");
    for (const selector of [
      ".app-main-sidebar-action-icon",
      ".app-menu-content-fit",
      ".app-sidebar-dropdown-content",
      ".app-settings-list-card",
      ".app-settings-row-content",
      ".app-pet-gallery-context-menu-content",
      ".app-completed-preview-volume-range",
    ]) {
      expect(anatomy).toContain(selector);
    }
  });

  test("keeps shared sidebar and context menus sized to localized content", async () => {
    const [anatomy, mainApp, libraryWorkspace] = await Promise.all([
      Bun.file(new URL("./dream/anatomy.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../app/main/MainApp.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../app/library/LibraryWorkspacePage.tsx", import.meta.url),
      ).text(),
    ]);
    const menuBaseIndex = anatomy.indexOf(".app-menu-content-base {");
    const contentFitIndex = anatomy.indexOf(
      ".app-menu-content-base.app-sidebar-dropdown-content {",
    );

    expect(menuBaseIndex).toBeGreaterThan(-1);
    expect(contentFitIndex).toBeGreaterThan(menuBaseIndex);
    expect(
      anatomy.slice(contentFitIndex, anatomy.indexOf("}", contentFitIndex) + 1),
    ).toMatch(
      /width:\s*max-content;[\s\S]*min-width:\s*fit-content;[\s\S]*max-width:/,
    );
    expect(mainApp).toMatch(
      /const workspaceNewAction[\s\S]*?<DropdownMenuContent[\s\S]*?className=\{SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME\}/,
    );
    expect(libraryWorkspace).toMatch(
      /<DropdownMenuContent[\s\S]*?SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME[\s\S]*?"app-library-context-menu"/,
    );
    expect(libraryWorkspace).toContain(
      'className="app-menu-content-fit app-library-actions-menu"',
    );
  });

  test("uses readable nocturne accents from the Dream theme pack", async () => {
    const themePacks = await Bun.file(
      new URL("./dream/theme-packs.css", import.meta.url),
    ).text();
    expect(themePacks).toMatch(
      /:root\[data-xiadown-theme-pack="nocturne"\][\s\S]*?--app-theme-pack-functional-accent:\s*#0F172A;/,
    );
    expect(themePacks).toMatch(
      /:root\.dark\[data-xiadown-theme-pack="nocturne"\][\s\S]*?--app-theme-pack-functional-accent:\s*#F3B549;/,
    );
  });

  test("defines semantic Liquid Glass materials with accessible fallbacks", async () => {
    const tokens = await Bun.file(
      new URL("./dream/tokens.css", import.meta.url),
    ).text();

    for (const token of [
      "--app-glass-regular-surface:",
      "--app-glass-regular-line:",
      "--app-glass-regular-shadow:",
      "--app-glass-regular-filter:",
      "--app-glass-panel-surface:",
      "--app-glass-panel-line:",
      "--app-glass-panel-shadow:",
      "--app-glass-panel-filter:",
      "--app-glass-clear-surface:",
      "--app-glass-clear-line:",
      "--app-glass-clear-shadow:",
      "--app-glass-clear-filter:",
      "--app-glass-chrome-surface:",
      "--app-glass-chrome-dense-surface:",
      "--app-glass-chrome-line:",
      "--app-glass-chrome-shadow:",
      "--app-glass-chrome-embedded-shadow:",
      "--app-glass-chrome-filter:",
      "--app-glass-chrome-dense-filter:",
      "--app-glass-chrome-tint-alpha:",
      "--app-glass-chrome-tint-soft-alpha:",
      "--app-glass-chrome-specular-opacity:",
      "--app-glass-interactive-fill:",
      "--app-glass-solid-surface:",
    ]) {
      expect(tokens).toContain(token);
    }
    expect(tokens).toContain(
      "--dream-glass-surface: var(--app-glass-regular-surface)",
    );
    expect(tokens).toContain(
      "--dream-menu-surface: var(--app-glass-panel-surface)",
    );
    expect(tokens).not.toContain("--dream-glass-filter: blur(");
    expect(tokens).not.toContain("brightness(");
    expect(tokens).toContain("--app-glass-regular-surface: color-mix(");
    expect(tokens).toContain("--app-glass-panel-surface: color-mix(");
    expect(tokens).toContain(
      "@supports not (color: color-mix(in srgb, white, black))",
    );
    expect(tokens).toContain("@media (prefers-reduced-transparency: reduce)");
    expect(tokens).toContain("[data-reduce-transparency=\"true\"]");
    expect(tokens).toContain(
      "@supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px)))",
    );
    expect(tokens).toContain("@media (forced-colors: active)");
    expect(tokens).toContain("--app-glass-panel-surface: Canvas");
    expect(tokens).toContain("--app-glass-chrome-surface: Canvas");
    expect(tokens).toContain("--app-glass-solid-surface: hsl(var(--popover));");
    expect(tokens).not.toMatch(
      /--app-glass-solid-surface:\s*hsl\(var\(--popover\)\s*\/\s*0\./,
    );
  });

  test("centralizes the optical recipe and keeps component anatomy material-free", async () => {
    const [glass, components, completed] = await Promise.all([
      Bun.file(new URL("./dream/glass.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/components.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/completed.css", import.meta.url)).text(),
    ]);

    for (const contract of [
      ".app-glass-surface {",
      '[data-material="panel"]',
      '[data-material="clear"]',
      '[data-material="solid"]',
      '[data-elevation="modal"]',
      '[data-shape="control"]',
      '[data-shape="capsule"]',
      '[data-tint="artwork"]',
      '[data-glass-role="header"]',
      '[data-glass-role="sidebar"]',
      '[data-glass-role="companion"]',
      ".app-glass-group {",
    ]) {
      expect(glass).toContain(contract);
    }
    expect(glass).toContain("backdrop-filter: var(--app-glass-filter)");
    expect(glass).toContain(".app-glass-surface::before");
    expect(glass).toContain(".app-glass-surface .app-glass-surface");
    expect(glass).toContain(
      '.app-glass-surface[data-material="regular"][data-glass-role][data-elevation="embedded"]',
    );
    expect(glass).not.toContain(
      '.app-glass-surface[data-material="panel"][data-glass-role',
    );
    expect(components).toContain('[data-surface-style="contrast"]');
    expect(components).toContain(':root[data-xiadown-theme-pack="pixel"]');
    expect(components).not.toContain('[data-sidebar-style="contrast"]');
    expect(components).not.toContain('[data-sidebar-style="pixel"]');

    const dialogAnatomy = components.match(
      /\.app-dialog-content \{([\s\S]*?)\n  \}/,
    )?.[1] ?? "";
    const messageAnatomy = components.match(
      /\.app-message-surface \{([\s\S]*?)\n  \}/,
    )?.[1] ?? "";
    for (const anatomy of [dialogAnatomy, messageAnatomy]) {
      expect(anatomy).not.toContain("--app-glass-");
      expect(anatomy).not.toContain("backdrop-filter");
      expect(anatomy).not.toContain("box-shadow");
    }
    expect(completed).not.toMatch(
      /\.app-menu-content[\s\S]{0,240}backdrop-filter:/,
    );
  });

  test("migrates transport and status surfaces to semantic contracts", async () => {
    const [transport, transportCss, youtube, activity] =
      await Promise.all([
        Bun.file(
          new URL("../../app/main/WorkspaceTransportBar.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("./dream/transport.css", import.meta.url),
        ).text(),
        Bun.file(
          new URL(
            "../../app/youtube/YouTubeWorkspaceTransportBar.tsx",
            import.meta.url,
          ),
        ).text(),
        Bun.file(
          new URL("../../app/main/WorkspaceActivitySurfaces.tsx", import.meta.url),
        ).text(),
      ]);

    expect(transport).toContain("<GlassGroup");
    expect(transport).toContain('surfaceRole="control"');
    expect(transport).not.toContain('material="regular"');
    expect(transport).toContain('shape="capsule"');
    expect(youtube).toContain("<GlassGroup");
    expect(youtube).toContain('surfaceRole="control"');
    expect(youtube).not.toContain('material="regular"');
    expect(youtube).toContain('elevation="floating"');
    expect(youtube).toContain('shape="card"');
    expect(transportCss).toContain("var(--app-control-bar-height)");
    expect(transportCss).toContain("var(--app-layer-floating-controls)");
    expect(transportCss).not.toContain("--app-glass-");
    expect(transportCss).not.toContain("backdrop-filter");
    expect(activity).toContain('tint="neutral"');
    for (const tone of ["idle", "busy", "success", "error", "orphan"]) {
      expect(activity).toContain(`"${tone}"`);
    }
  });

  test("keeps the semantic glass contract singular and feature recipes material-free", async () => {
    const [entry, glass, ...migratedCss] = await Promise.all([
      Bun.file(new URL("./dream.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/glass.css", import.meta.url)).text(),
      Bun.file(
        new URL("./dream/transport.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("./dream/activity.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL(
          "../../app/workspace/workspace-navigation.css",
          import.meta.url,
        ),
      ).text(),
      Bun.file(
        new URL(
          "../../app/workspace/station-dock-editor.css",
          import.meta.url,
        ),
      ).text(),
    ]);

    expect(entry.indexOf('./dream/tokens.css')).toBeLessThan(
      entry.indexOf('./dream/glass.css'),
    );
    expect(entry.indexOf('./dream/glass.css')).toBeLessThan(
      entry.indexOf('./dream/components.css'),
    );
    expect(glass).toContain(".app-glass-surface[data-material]");
    expect(glass).not.toContain("@layer components");
    for (const css of migratedCss) {
      expect(css).not.toMatch(
        /--app-glass-(?:regular|panel)-(?:surface|filter|shadow)/,
      );
      expect(css).not.toMatch(/backdrop-filter:\s*(?:var|blur|saturate)/);
    }
  });

  test("governs legacy dream surfaces without bespoke backdrop blur recipes", async () => {
    const [components, shell, workflows, pets, completed, motion] =
      await Promise.all(
        [
          "components.css",
          "shell.css",
          "workflows.css",
          "pets.css",
          "completed.css",
          "motion.css",
        ].map((file) =>
          Bun.file(new URL(`./dream/${file}`, import.meta.url)).text(),
        ),
      );

    for (const css of [
      components,
      shell,
      workflows,
      pets,
      completed,
      motion,
    ]) {
      expect(css).not.toMatch(/backdrop-filter\s*:\s*[^;\n]*\bblur\(/);
    }

    expect(components).toContain(
      "backdrop-filter: var(--app-glass-panel-filter)",
    );
    expect(components).toContain(
      "backdrop-filter: var(--app-glass-clear-filter)",
    );
    expect(shell).toContain(
      "backdrop-filter: var(--app-glass-regular-filter)",
    );
    expect(workflows).toContain(
      "backdrop-filter: var(--app-glass-clear-filter)",
    );
    expect(completed).toContain(
      "backdrop-filter: var(--app-glass-regular-filter)",
    );
    expect(motion).toContain(
      "backdrop-filter: var(--app-glass-regular-filter)",
    );
  });

  test("keeps pointer focus chrome-free without removing material shadows", async () => {
    const [globalCss, glass] = await Promise.all([
      Bun.file(new URL("./dream/foundation.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/glass.css", import.meta.url)).text(),
    ]);

    expect(globalCss).toContain('-webkit-tap-highlight-color: transparent');
    expect(globalCss).toContain(':root[data-input-modality="pointer"]');
    expect(globalCss).toContain('--tw-ring-shadow: 0 0 #0000 !important');
    expect(globalCss).toContain('outline: none !important');
    expect(globalCss).toContain('@media (forced-colors: active)');
    expect(globalCss).toContain('outline: 2px solid Highlight !important');
    expect(glass).toContain(
      'box-shadow: var(--app-glass-shadow), var(--app-surface-state-shadow)',
    );
  });
});
