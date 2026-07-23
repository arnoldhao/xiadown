import { describe, expect, test } from "bun:test";
import postcss from "postcss";

import {
  COMPANION_PANEL_WIDTH,
  PRIMARY_PANE_DEFAULT_MIN_WIDTH,
  WORKSPACE_SIDEBAR_WIDTH,
} from "./layout";

async function readWorkspaceAppearance() {
  return Bun.file(
    new URL("../../shared/styles/dream/workspace.css", import.meta.url),
  ).text();
}

describe("workspace CSS contract", () => {
  test("makes the fixed-width Primary pane the responsive query context", async () => {
    const css = await Bun.file(
      new URL("./workspace.css", import.meta.url),
    ).text();

    expect(css).toMatch(
      /\.app-workspace-primary-pane\s*\{[^}]*container:\s*workspace-primary \/ inline-size/s,
    );
  });

  test("keeps the companion fixed and closed panels out of layout", async () => {
    const css = await Bun.file(
      new URL("./workspace.css", import.meta.url),
    ).text();

    expect(COMPANION_PANEL_WIDTH).toBe(390);
    expect(css).toContain(
      `var(--app-workspace-companion-width, ${COMPANION_PANEL_WIDTH}px)`,
    );
    expect(css).toMatch(
      /\.app-workspace-companion\s*\{[^}]*--app-workspace-companion-gutter:\s*1\.25rem;[^}]*--app-workspace-companion-reading-gutter:\s*1\.75rem;/s,
    );
    expect(css).toContain(".app-workspace-companion[hidden]");
    expect(css).toContain("display: none");
  });

  test("layers canonical glass title chrome over every Companion surface", async () => {
    const [css, appearance] = await Promise.all([
      Bun.file(new URL("./workspace.css", import.meta.url)).text(),
      readWorkspaceAppearance(),
    ]);
    const regionSurfaceRule = appearance.match(
      /\.app-workspace-companion__header,\s*\.app-workspace-companion__content,\s*\.app-workspace-companion__footer\s*\{([^}]*)\}/s,
    );
    const portaledMusicSurfaceRule = appearance.match(
      /\.app-workspace-companion:is\([\s\S]*?\):not\(\.app-workspace-companion--player-fullscreen\)\s*\.listen-content-surface\[data-player-presentation="companion"\]\s*\{([^}]*)\}/,
    );
    const appearanceRoot = postcss.parse(appearance);
    const matchingDeclarations = (...selectorFragments: string[]) => {
      const matches: Array<Record<string, string>> = [];
      appearanceRoot.walkRules((rule) => {
        if (
          !selectorFragments.every((fragment) =>
            rule.selector.includes(fragment),
          )
        ) {
          return;
        }
        const declarations: Record<string, string> = {};
        rule.walkDecls((declaration) => {
          declarations[declaration.prop] = declaration.value;
        });
        matches.push(declarations);
      });
      return matches;
    };

    expect(css).toMatch(
      /\.app-workspace-companion__content\s*\{[^}]*min-height:\s*0[^}]*flex:\s*1 1 auto[^}]*overflow:\s*hidden/s,
    );
    expect(css).toMatch(
      /\.app-workspace-companion__footer\s*\{[^}]*width:\s*100%[^}]*min-width:\s*0[^}]*box-sizing:\s*border-box[^}]*flex:\s*0 0 auto/s,
    );
    expect(
      css.match(/\.app-workspace-companion__content\s*\{([^}]*)\}/s)?.[1],
    ).not.toContain("padding");
    expect(css).toContain(".app-workspace-companion__header-material");
    expect(`${css}\n${appearance}`).not.toContain(
      ".app-workspace-companion__footer-material",
    );
    expect(css).toMatch(
      /\.app-workspace-companion\s*\{[^}]*--app-workspace-companion-footer-inset:\s*60px/s,
    );
    expect(css).toMatch(
      /\.app-workspace-companion\[data-scroll-chrome="active"\]\[data-has-footer="true"\]:not\([\s\S]*?\)\s*\.app-workspace-companion__content\s*\[data-companion-scroll-owner\]\s*\{[^}]*scroll-padding-block-end:\s*var\(--app-workspace-companion-footer-inset\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-companion\[data-scroll-chrome="active"\]\[data-has-footer="true"\]:not\([\s\S]*?\)\s*\.app-workspace-companion__content\s*\[data-companion-scroll-owner\]::after\s*\{[^}]*height:\s*var\(--app-workspace-companion-footer-inset\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-companion\[data-scroll-chrome="active"\]:not\([\s\S]*?\)\s*> \.app-workspace-companion__footer\s*\{[^}]*position:\s*absolute[^}]*z-index:\s*2[^}]*inset:\s*auto 0 0[^}]*min-height:\s*var\(--app-workspace-companion-footer-inset\)/s,
    );
    expect(regionSurfaceRule?.[1]).toContain("background: transparent");
    expect(regionSurfaceRule?.[1]).toContain("border: 0");
    expect(portaledMusicSurfaceRule?.[1]).toContain("background: transparent");
    expect(appearance).toMatch(
      /\.app-main-shell\[data-surface-style="glass"\][\s\S]*?\.app-workspace-companion\[data-glass-host="true"\],[\s\S]*?\.app-workspace-companion\[data-presentation="overlay"\]\[data-glass-host="true"\]\s*\{[^}]*background:\s*transparent[^}]*border-left:\s*0/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-companion\[data-presentation="docked"\][\s\S]*?> \.app-workspace-chrome-material\[data-glass-role="companion"\]\s*\{[^}]*border-color:\s*var\(--app-workspace-divider-color\)[^}]*border-width:\s*0 0 0 var\(--app-workspace-divider-width\)[^}]*border-radius:\s*var\(--app-workspace-chrome-edge-radius\)/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-companion\[data-presentation="overlay"\]\s*\{[^}]*border-radius:\s*var\(--app-radius-panel\)\s*var\(--app-radius-none\)\s*var\(--app-radius-none\)\s*var\(--app-radius-panel\)[^}]*box-shadow:\s*-18px 0 44px/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-companion\.app-workspace-companion--player-fullscreen\s*\{[^}]*border-radius:\s*var\(--app-radius-none\)[^}]*box-shadow:\s*none/s,
    );
    expect(css).toContain(
      ".app-workspace-companion--player-fullscreen\n  > .app-workspace-chrome-material",
    );
    expect(css).toMatch(
      /\.app-workspace-companion--player-fullscreen\s*> \.app-workspace-chrome-material,[\s\S]*?\.app-workspace-companion__header-material\s*\{[^}]*display:\s*none/s,
    );
    expect(appearance).not.toContain("--youtube-up-next-companion-surface");
    expect(appearance).not.toMatch(
      /\.app-workspace-companion__(?:header|footer)\s*\{[^}]*background:\s*hsl\(var\(--background\)\s*\/\s*0\.82\)/s,
    );
    expect(css).toContain(
      '.app-workspace-companion[data-platform="windows"]:not(',
    );
    expect(css).toContain("padding: 0 0 0 12px");
    expect(css).not.toMatch(
      /\[data-destination="queue"\][\s\S]*?\.app-workspace-companion__header\s*\{[^}]*pointer-events:\s*none/,
    );
    expect(appearance).toContain('[data-glass-role="header"]');
    expect(appearance).not.toContain('[data-glass-role="footer"]');
    const baseMaterial = matchingDeclarations(
      "app-workspace-companion__header-material",
    ).find(
      (declarations) =>
        declarations.opacity === "0" &&
        declarations["--app-glass-filter"] === "none",
    );
    expect(baseMaterial).toBeDefined();
    expect(baseMaterial?.border).toBe("0");
    expect(baseMaterial?.["border-radius"]).toBe("var(--app-radius-none)");
    expect(baseMaterial?.["box-shadow"]).toBe("none");
    const scrolledHeader = matchingDeclarations(
      'data-scroll-state="scrolled"',
      "app-workspace-companion__header-material",
    ).find((declarations) => declarations.opacity === "1");
    expect(scrolledHeader?.["--app-glass-filter"]).toBe(
      "var(--app-glass-chrome-filter)",
    );
    const contrastMaterial = matchingDeclarations(
      'data-surface-style="contrast"',
      "app-workspace-companion__header-material",
    ).find(
      (declarations) =>
        declarations["--app-glass-fill"] === "var(--app-surface-canvas)",
    );
    expect(contrastMaterial?.["--app-glass-filter"]).toBe("none");
    const reducedTransparencyMaterial = matchingDeclarations(
      "app-workspace-companion__header-material",
    ).find(
      (declarations) =>
        declarations["--app-glass-fill"] ===
        "var(--app-glass-solid-surface)",
    );
    expect(reducedTransparencyMaterial?.["--app-glass-filter"]).toBe("none");
    expect(appearance).toContain("prefers-reduced-transparency: reduce");
    expect(`${css}\n${appearance}`).not.toContain(
      ".app-workspace-companion__header::after",
    );
    expect(`${css}\n${appearance}`).not.toContain('data-footer-state="');
    expect(appearance).toMatch(
      /@media \(forced-colors:\s*active\)[\s\S]*?app-workspace-companion__header-material[\s\S]*?--app-glass-filter:\s*none/s,
    );
    expect(`${css}\n${appearance}`).not.toContain(
      "--app-workspace-page-header-fade-size",
    );
  });

  test("paints one ambient field and one dense Primary veil in Glass", async () => {
    const [css, appearance, tokens] = await Promise.all([
      Bun.file(new URL("./workspace.css", import.meta.url)).text(),
      readWorkspaceAppearance(),
      Bun.file(
        new URL("../../shared/styles/dream/tokens.css", import.meta.url),
      ).text(),
    ]);

    expect(css).toMatch(
      /\.app-workspace-ambient-canvas\s*\{[^}]*position:\s*absolute[^}]*inset:\s*0[^}]*pointer-events:\s*none/s,
    );
    expect(css).toMatch(
      /\.app-workspace-ambient-canvas__artwork\s*\{[^}]*object-fit:\s*cover/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-ambient-canvas\s*\{[^}]*--app-workspace-ambient-base:[^}]*--app-main-shell-aba-surface[^}]*--sidebar-background[^}]*--primary[^}]*--app-workspace-ambient-base/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-ambient-canvas__artwork\s*\{[^}]*opacity:\s*0\.18[^}]*filter:\s*blur\(72px\) saturate\(1\.18\)/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-stage\s*\{[^}]*background:\s*transparent/s,
    );
    const glassPrimaryRule = appearance.match(
      /\.app-main-shell\[data-surface-style="glass"\]\s*\.app-workspace-primary-pane\s*\{([^}]*)\}/s,
    );

    expect(appearance).toMatch(
      /\.app-main-shell\[data-surface-style="glass"\]\s*\{[^}]*--app-main-shell-aba-surface:\s*transparent/s,
    );
    expect(tokens).toMatch(
      /:root\[data-xiadown-surface-style="glass"\],[\s\S]*?:where\(\[data-surface-style="glass"\]\)\s*\{[^}]*--app-workspace-primary-surface:[^}]*var\(--app-workspace-primary-glass-surface\)/s,
    );
    expect(tokens).toContain("--app-workspace-primary-glass-opacity: 0.88;");
    expect(glassPrimaryRule?.[1]).toContain("var(--app-workspace-primary-surface)");
    expect(glassPrimaryRule?.[1]).toContain("box-shadow: none");
    expect(glassPrimaryRule?.[1]).toContain("backdrop-filter: none");
    expect(tokens).toMatch(
      /:root\.dark\s*\{[^}]*--app-workspace-primary-glass-opacity:\s*0\.90/s,
    );
    expect(appearance).toMatch(
      /\.app-main-shell\[data-surface-style="glass"\][\s\S]*?\.app-workspace-primary-pane[\s\S]*?:is\([\s\S]*?\.app-main-view-viewport,[\s\S]*?\.app-main-page,[\s\S]*?\.app-workspace-primary-subpane,[\s\S]*?\.app-main-list-pane,[\s\S]*?\.app-main-detail-pane,[\s\S]*?\.app-library-page,[\s\S]*?\.rss-workspace-page,[\s\S]*?\.youtube-workspace-page[\s\S]*?\)\s*\{[^}]*background:\s*var\(--app-workspace-primary-subpane-surface\)/s,
    );
    expect(appearance).not.toMatch(
      /\.app-workspace-primary-pane[\s\S]{0,500}:is\([^)]*(?:rss-discovery-dialog|app-glass-surface|player-fullscreen)/s,
    );
    expect(appearance).toContain('[data-listen-native-video-underlay="true"]');
    expect(`${css}\n${appearance}`).toContain('[data-reduce-transparency="true"]');
    expect(`${css}\n${appearance}`).toContain("prefers-reduced-transparency: reduce");
  });

  test("keeps Glass ambient paint outside the native video aperture", async () => {
    const [workspaceAppearance, listenLayoutCss, listenAppearanceCss] = await Promise.all([
      readWorkspaceAppearance(),
      Bun.file(
        new URL("../main/listen/listen.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
    ]);
    const underlayGeometryTokens = listenLayoutCss.match(
      /:root\[data-listen-native-video-underlay="true"\]\s*\{([\s\S]*?)\n\s*\}/,
    )?.[1];
    const underlayAppearanceTokens = listenAppearanceCss.match(
      /:root\[data-listen-native-video-underlay="true"\]\s*\{([\s\S]*?)\n\s*\}/,
    )?.[1];
    const ambientUnderlayRule = workspaceAppearance.match(
      /:root\[data-listen-native-video-underlay="true"\][\s\S]*?\.app-workspace-ambient-canvas\s*\{([^}]*)\}/,
    )?.[1];
    const underlayCanvasRule = listenAppearanceCss.match(
      /:root\[data-listen-native-video-underlay="true"\] #root::before\s*\{([^}]*)\}/,
    )?.[1];
    const reactLayerRule = listenLayoutCss.match(
      /:root\[data-listen-native-video-underlay="true"\] #root > \*\s*\{([^}]*)\}/,
    )?.[1];

    expect(underlayAppearanceTokens).toContain("--listen-native-video-surface:");
    expect(underlayAppearanceTokens).toContain("hsl(var(--dream-shell-top) / 0.28)");
    expect(underlayAppearanceTokens).not.toContain("hsl(var(--background))");
    expect(underlayGeometryTokens).toContain("--listen-native-video-outside-mask:");
    expect(ambientUnderlayRule).not.toContain("display: none");
    expect(ambientUnderlayRule).toContain(
      "-webkit-mask: var(--listen-native-video-outside-mask)",
    );
    expect(ambientUnderlayRule).toContain(
      "mask: var(--listen-native-video-outside-mask)",
    );
    expect(underlayCanvasRule).toContain(
      "background: var(--listen-native-video-surface)",
    );
    expect(underlayCanvasRule).toContain(
      "mask: var(--listen-native-video-outside-mask)",
    );
    expect(reactLayerRule).toContain("z-index: 1");
  });

  test("delegates persistent Glass sampling to the native window material only", async () => {
    const css = await readWorkspaceAppearance();
    const nativePersistentRule = css.match(
      /\.app-main-shell\[data-surface-style="glass"\]\[data-window-material="native"\][\s\S]*?\.app-workspace-sidebar[\s\S]*?> \.app-workspace-chrome-material\[data-glass-role="sidebar"\],[\s\S]*?\.app-workspace-companion\[data-presentation="docked"\][\s\S]*?> \.app-workspace-chrome-material\[data-glass-role="companion"\]\s*\{([^}]*)\}/,
    );

    expect(css).toMatch(
      /\.app-main-shell\[data-surface-style="glass"\]\[data-window-material="native"\]\s*\{[^}]*background:\s*transparent/s,
    );
    expect(css).toMatch(
      /\.app-main-shell\[data-surface-style="glass"\]\[data-window-material="native"\][\s\S]*?\.app-workspace-ambient-canvas\s*\{[^}]*--app-workspace-ambient-base:\s*transparent/s,
    );
    expect(nativePersistentRule?.[1]).toContain("backdrop-filter: none");
    expect(nativePersistentRule?.[1]).not.toContain("background: none");
    expect(nativePersistentRule?.[0]).not.toContain(
      'data-presentation="overlay"',
    );
  });

  test("uses one opaque canvas for every Contrast pane and keeps solid fallbacks", async () => {
    const css = await readWorkspaceAppearance();

    expect(css).toMatch(
      /\.app-main-shell\[data-surface-style="contrast"\]\s*\.app-workspace-shell\s*\{[^}]*background:\s*var\(--app-surface-canvas\)/s,
    );
    expect(css).toMatch(
      /\.app-main-shell\[data-surface-style="contrast"\]\s*\.app-workspace-sidebar\s*\{[^}]*background:\s*var\(--app-surface-canvas\)[^}]*border-right:[^}]*var\(--app-workspace-divider-width\) solid[^}]*var\(--app-workspace-divider-color\)/s,
    );
    expect(css).toMatch(
      /\.app-main-shell\[data-surface-style="contrast"\][\s\S]*?\.app-workspace-companion\[data-presentation="docked"\]\s*\{[^}]*background:\s*var\(--app-surface-canvas\)[^}]*border-left:[^}]*var\(--app-workspace-divider-width\) solid[^}]*var\(--app-workspace-divider-color\)/s,
    );
    expect(css).toMatch(
      /\.app-main-shell\[data-surface-style="contrast"\]\s*\.app-workspace-primary-pane\s*\{[^}]*background:\s*var\(--app-workspace-primary-surface\)/s,
    );
    expect(css).toMatch(
      /\.app-main-shell\[data-surface-style="contrast"\]\s*\{[^}]*--app-main-shell-aba-surface:\s*var\(--app-surface-canvas\)/s,
    );
    expect(css).toMatch(
      /\.app-main-shell\[data-surface-style="contrast"\][\s\S]*?\.app-workspace-primary-pane[\s\S]*?:is\([\s\S]*?\.app-main-view-viewport,[\s\S]*?\.app-library-primary-surface,[\s\S]*?\.rss-workspace-page,[\s\S]*?\.youtube-workspace-page[\s\S]*?\)\s*\{[^}]*background:\s*var\(--app-workspace-primary-subpane-surface\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-companion\[data-presentation="overlay"\]\[data-glass-host="true"\]\s*\{[^}]*background:\s*transparent/s,
    );
    expect(css).toMatch(
      /\[data-window-material="solid"\][\s\S]*?\.app-main-shell\[data-surface-style="glass"\][\s\S]*?--app-glass-chrome-surface:\s*hsl\(var\(--sidebar-background\)\)[\s\S]*?--app-glass-chrome-dense-surface:\s*hsl\(var\(--card\)\)/s,
    );
    expect(css).toMatch(
      /\[data-window-material="solid"\][\s\S]*?\.app-workspace-primary-pane[\s\S]*?background:\s*hsl\(var\(--background\)\)/s,
    );
  });

  test("provides the standalone sidebar width fallback without rail styles", async () => {
    const css = await Bun.file(
      new URL("./workspace.css", import.meta.url),
    ).text();

    expect(WORKSPACE_SIDEBAR_WIDTH).toBe(224);
    expect(PRIMARY_PANE_DEFAULT_MIN_WIDTH).toBe(800);
    expect(css).toContain(
      `var(--app-workspace-navigation-width, ${WORKSPACE_SIDEBAR_WIDTH}px)`,
    );
    expect(css).not.toContain("app-workspace-rail");
  });

  test("stops background workspace regions from painting under fullscreen players", async () => {
    const css = await Bun.file(
      new URL("./workspace.css", import.meta.url),
    ).text();

    expect(css).toContain('[data-player-fullscreen="true"]');
    expect(css).toContain(".app-workspace-primary-pane");
    expect(css).toContain("visibility: hidden");
  });

  test("makes avatar, identity, and disclosure one full-width menu trigger", async () => {
    const [css, appearance, workflows] = await Promise.all([
      Bun.file(new URL("./workspace.css", import.meta.url)).text(),
      readWorkspaceAppearance(),
      Bun.file(
        new URL("../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    expect(css).toMatch(
      /\.app-workspace-account-profile\s*\{[^}]*width:\s*100%[^}]*height:\s*44px[^}]*grid-template-columns:\s*40px minmax\(0, 1fr\) auto/s,
    );
    expect(css).toMatch(
      /\.app-workspace-account-profile__avatar\s*\{[^}]*width:\s*40px[^}]*height:\s*40px[^}]*pointer-events:\s*none/s,
    );
    expect(css).toMatch(
      /\.app-workspace-account-profile__avatar-media\s*\{[^}]*inset:\s*2px[^}]*pointer-events:\s*none/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-account-profile\[aria-expanded="true"\]\s*\{[^}]*background:\s*var\(--app-glass-interactive-fill-pressed\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-account-menu\.app-menu-content\s*\{[^}]*width:\s*var\(--radix-dropdown-menu-trigger-width\)[^}]*min-width:\s*var\(--radix-dropdown-menu-trigger-width\)[^}]*max-width:\s*var\(--radix-dropdown-menu-trigger-width\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-account-menu__access-row\.app-menu-item\s*\{[^}]*box-sizing:\s*border-box[^}]*display:\s*flex[^}]*width:\s*100%[^}]*height:\s*var\(--app-menu-action-height\)[^}]*min-height:\s*var\(--app-menu-action-height\)[^}]*align-items:\s*center[^}]*justify-content:\s*space-between[^}]*padding:\s*0 0\.625rem/s,
    );
    expect(appearance).toMatch(
      /\.app-workspace-account-menu__access-name\s*\{[^}]*color:\s*inherit[^}]*font-size:\s*0\.8125rem[^}]*font-weight:\s*500/s,
    );
    const checkedAccessRules = appearance.match(
      /\.app-workspace-account-menu__access-row\.app-menu-item\[data-state="checked"\][\s\S]*?\.app-workspace-account-menu__access-name\s*\{/,
    )?.[0] ?? "";
    expect(checkedAccessRules).not.toMatch(/\bcolor\s*:/);
    expect(css).toMatch(
      /\.app-workspace-account-menu__access-switch\.app-dream-inline-switch\s*\{[^}]*flex:\s*0 0 auto[^}]*pointer-events:\s*none/s,
    );
    expect(css).toMatch(
      /\.app-workspace-account-menu__mobile-action\.app-menu-item\s*\{[^}]*width:\s*28px[^}]*min-height:\s*28px[^}]*place-items:\s*center/s,
    );
    expect(`${css}\n${appearance}`).not.toContain(
      "app-workspace-account-menu__access-indicator",
    );
    expect(`${css}\n${appearance}`).not.toContain(
      "app-workspace-account-menu__access-status",
    );
    expect(css).toMatch(
      /\.app-workspace-account-menu__quick-actions\s*\{[^}]*display:\s*flex/s,
    );
    expect(css).toMatch(
      /\.app-workspace-account-menu__quick-action\.app-menu-item\s*\{[^}]*height:\s*var\(--app-menu-action-height\)[^}]*min-height:\s*var\(--app-menu-action-height\)/s,
    );
    expect(workflows).toMatch(
      /@media \(forced-colors:\s*active\)[\s\S]*?\.app-dream-inline-switch\s*\{[^}]*border:\s*1px solid CanvasText[^}]*background:\s*Canvas[^}]*box-shadow:\s*none/s,
    );
    expect(appearance).not.toMatch(
      /@media \(forced-colors:\s*active\)[\s\S]*?\.app-workspace-account-menu__access-switch\.app-dream-inline-switch/,
    );
    expect(`${css}\n${appearance}`).not.toContain(
      "app-workspace-account-profile__avatar-overlay",
    );
    expect(`${css}\n${appearance}`).not.toContain("data-home-visible");
    expect(appearance).toContain("prefers-reduced-motion: reduce");
  });

  test("does not divide workspace navigation from its bottom regions", async () => {
    const css = await Bun.file(
      new URL("./workspace.css", import.meta.url),
    ).text();
    const bottomRule = css.match(/\.app-workspace-sidebar__bottom\s*\{([^}]*)\}/s);

    expect(bottomRule).not.toBeNull();
    expect(bottomRule?.[1]).not.toContain("border");
  });

  test("uses pointer-safe, inset keyboard focus on the account trigger", async () => {
    const css = await readWorkspaceAppearance();

    expect(css).toMatch(
      /:root:not\(\[data-input-modality="pointer"\]\)[^{]*\.app-workspace-account-profile:focus-visible\s*\{[^}]*outline-offset:\s*-2px/s,
    );
  });

  test("uses the shared shape and global layer scales", async () => {
    const [css, appearance] = await Promise.all([
      Bun.file(new URL("./workspace.css", import.meta.url)).text(),
      readWorkspaceAppearance(),
    ]);

    expect(css).toContain("z-index: var(--app-layer-floating-controls)");
    expect(css).toContain("z-index: var(--app-layer-window-controls)");
    expect(appearance).toContain("border-radius: var(--app-radius-capsule)");
    expect(css).not.toMatch(/z-index:\s*(?:[5-9]|[1-9]\d+)\s*;/);
    expect(appearance).not.toMatch(/border-radius:\s*(?:\d|\.\d)/);
  });
});
