import { describe, expect, test } from "bun:test";
import postcss from "postcss";
import type { ComponentProps } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import {
  GLASS_ELEVATIONS,
  GLASS_SHAPES,
  GLASS_TINTS,
} from "@/shared/ui/glass-surface";
import {
  XIA_GLASS_MATERIALS,
  XIA_SURFACE_ROLES,
} from "@/shared/ui/surface-contract";

import {
  AppearanceLab,
  PrismGlassFixture,
  RegressionContractSpecimen,
  SurfaceRoleMatrix,
} from "./AppearanceLab";

const noop = () => undefined;

const VISUAL_PROPERTY_PATTERN = /^(?:accent-color|animation(?:-.+)?|backdrop-filter|background(?:-.+)?|border(?:-.+)?|box-shadow|caret-color|clip-path|color|fill|filter|font(?:-.+)?|mask(?:-.+)?|mix-blend-mode|opacity|outline(?:-.+)?|stroke|text-decoration(?:-.+)?|text-shadow|transition(?:-.+)?|-webkit-backdrop-filter)$/;

function renderFixture(
  overrides: Partial<ComponentProps<typeof PrismGlassFixture>> = {},
) {
  return renderToStaticMarkup(
    <PrismGlassFixture
      appearance="light"
      companionOpen
      nativeVideoPreview="off"
      onCompanionOpenChange={noop}
      onNativeVideoPreviewChange={noop}
      onPlatformChange={noop}
      onPresentationChange={noop}
      onReduceTransparencyChange={noop}
      onSurfaceStyleChange={noop}
      platform="macos"
      presentation="docked"
      reduceTransparency={false}
      surfaceStyle="glass"
      {...overrides}
    />,
  );
}

describe("Xia Prism Appearance Lab fixture", () => {
  test("loads the development-only Lab in its own application chunk", async () => {
    const [
      appSource,
      mainSource,
      providersSource,
      settingsQuerySource,
      themeRuntimeSource,
      windowControlsSource,
    ] =
      await Promise.all([
        Bun.file(new URL("../../App.tsx", import.meta.url)).text(),
        Bun.file(new URL("../../main.tsx", import.meta.url)).text(),
        Bun.file(
          new URL("../providers/AppProviders.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../shared/query/settings.ts", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../shared/styles/theme-runtime.ts", import.meta.url),
        ).text(),
        Bun.file(
          new URL(
            "../../components/layout/WindowControls.tsx",
            import.meta.url,
          ),
        ).text(),
      ]);

    expect(appSource).not.toContain('import("./app/dev/AppearanceLab")');
    expect(appSource).not.toContain(
      'import { AppearanceLab } from "./app/dev/AppearanceLab"',
    );
    expect(mainSource).toContain(
      'const { AppearanceLab } = await import("./app/dev/AppearanceLab")',
    );
    expect(mainSource).toContain('import("./app/dev/AppearanceLab")');
    expect(mainSource).toContain("if (!import.meta.env.DEV)");
    expect(mainSource.indexOf("if (isAppearanceLabWindow())")).toBeLessThan(
      mainSource.indexOf('import("./app/providers/AppProviders")'),
    );
    expect(mainSource).not.toContain(
      'import { AppProviders } from "./app/providers/AppProviders"',
    );
    expect(mainSource).not.toContain('import App from "./App"');
    expect(mainSource).toContain("loadWindowApplication(windowType)");
    expect(mainSource).toContain('import("./app/main")');
    expect(mainSource).toContain('import("./app/settings")');
    expect(appSource).not.toContain('import("./app/main")');
    expect(appSource).not.toContain('import("./app/settings")');
    expect(providersSource.match(/if \(!runtimeEnabled\)/g)).toHaveLength(1);
    expect(providersSource.match(/if \(!telemetryEnabled\)/g)).toHaveLength(1);
    expect(appSource).toContain('useSettings(\n    windowType !== "appearance-lab"');
    expect(settingsQuerySource).toContain("export function useSettings(enabled = true)");
    expect(settingsQuerySource).toContain("enabled,");
    expect(themeRuntimeSource).not.toContain('from "@wailsio/runtime"');
    expect(windowControlsSource).not.toContain('from "@wailsio/runtime"');
    expect(windowControlsSource).toContain('import("@wailsio/runtime")');
  });

  test("composes the real three-pane workspace with one sampler per chrome pane", () => {
    const markup = renderFixture();

    expect(markup).toContain("appearance-lab__prism-window");
    expect(markup).toContain("app-workspace-shell");
    expect(markup).toContain("app-workspace-sidebar");
    expect(markup).toContain("app-workspace-primary-pane");
    expect(markup).toContain("app-workspace-companion");
    expect(markup).toContain("app-workspace-ambient-canvas");
    expect(markup).toContain(
      'data-appearance-fixture="native-video-shell-isolation"',
    );
    expect(markup).toContain('data-surface-style="glass"');
    expect(markup).toContain('aria-label="Surface style"');
    expect(markup).toContain('aria-label="Native video playback"');
    expect(markup).toContain('data-native-video-preview="off"');
    expect(
      markup.match(/data-appearance-native-video-state=/g),
    ).toHaveLength(3);
    expect(markup).toContain('data-appearance-native-video-state="youtube"');
    expect(markup).toContain('data-appearance-native-video-state="rss"');
    expect(markup.match(/data-glass-role="sidebar"/g)).toHaveLength(1);
    expect(markup.match(/data-glass-role="companion"/g)).toHaveLength(1);
    expect(markup.match(/data-glass-role="header"/g)).toHaveLength(1);
    expect(markup).not.toContain('data-glass-role="footer"');
    expect(markup).toContain('data-companion-scroll-owner="queue"');
    expect(markup.match(/appearance-lab__prism-queue-row/g)).toHaveLength(12);
    expect(markup).toContain('data-scroll-state="top"');
    expect(markup).toContain('data-has-footer="true"');
    expect(markup).toContain("app-workspace-companion__footer");
    expect(markup).not.toContain("app-workspace-companion__footer-material");
    expect(markup).not.toContain("data-footer-state");
    expect(markup).not.toContain("appearance-lab__prism-material");
  });

  test("renders production playback anatomy instead of a lookalike specimen", () => {
    const markup = renderToStaticMarkup(<AppearanceLab />);
    const playerButtons = (markup.match(/<button[^>]*>/g) ?? []).filter(
      (tag) =>
        tag.includes("listen-player-icon-button") ||
        tag.includes("listen-primary-play-button"),
    );

    expect(markup).toContain('data-appearance-fixture="playback-surfaces"');
    expect(markup).toContain("app-workspace-transport");
    expect(markup).toContain("app-workspace-transport__left");
    expect(markup).toContain("app-workspace-transport__center");
    expect(markup).toContain("app-workspace-transport__timeline");
    expect(markup).toContain("app-workspace-transport__more");
    expect(markup).toContain("app-workspace-transport__right-actions");
    expect(markup).toContain("app-workspace-transport__artwork-open");
    expect(markup).toContain('data-shape="square"');
    expect(markup).toContain('data-transport-emphasis="step"');
    expect(markup).toContain('data-transport-emphasis="primary"');
    expect(markup).toContain("listen-primary-play-button--medium");
    expect(markup).toContain('data-transport-size="small"');
    expect(markup).toContain('data-transport-size="normal"');
    expect(markup).toContain("listen-workspace-queue-row__artwork");
    expect(markup).toContain('data-footer-region="leading"');
    expect(markup).toContain('data-footer-region="dynamic"');
    expect(markup).toContain('data-footer-region="trailing"');
    expect(markup).toContain('data-listen-lyrics-renderer="focus"');
    expect(markup).not.toContain("listen-lyrics-focus__keywords");
    expect(markup).not.toContain("listen-lyrics-focus__word-effect");
    expect(markup).not.toContain("listen-lyrics-focus__romanization-effect");
    expect(playerButtons.length).toBeGreaterThan(0);
    expect(playerButtons.every((tag) => tag.includes('data-shape="circle"'))).toBe(true);
  });

  test("keeps every reported regression visible through production contracts", () => {
    const markup = renderToStaticMarkup(<RegressionContractSpecimen />);
    const fullscreenFooters = markup.match(
      /<footer[^>]*listen-workspace-fullscreen-player__footer[^>]*>/g,
    ) ?? [];
    const allButtons = markup.match(/<button[^>]*>/g) ?? [];
    const fullscreenButtons = allButtons.filter(
      (tag) => tag.includes("listen-player-icon-button"),
    );
    const fullscreenPrimaryButtons = allButtons.filter(
      (tag) => tag.includes("listen-primary-play-button"),
    );
    const activeFooterButtons = allButtons.filter(
      (tag) =>
        tag.includes("listen-player-footer-icon-button") &&
        tag.includes('data-active="true"'),
    );

    expect(markup).toContain('data-appearance-fixture="regression-contracts"');
    expect(markup.match(/app-settings-tab-button/g)).toHaveLength(3);
    for (const action of [
      "running-new-egg",
      "sniff-start",
      "theme-choice",
      "session-verify",
      "session-sign-out",
    ]) {
      expect(markup).toContain(`data-appearance-action="${action}"`);
    }
    expect(markup.match(/data-equalizer-control-card=/g)).toHaveLength(2);
    expect(markup).toContain('data-equalizer-control-card="preset"');
    expect(markup).toContain('data-equalizer-control-card="preamp"');
    expect(markup).toContain("app-equalizer-preset-card-content");
    expect(markup).toContain("app-equalizer-preamp-card-content");
    expect(markup).toContain('data-library-device="appearance-device"');
    expect(markup).toContain('data-appearance-fixture="library-device-details"');
    expect(markup).toContain('data-library-device-details="appearance-device"');
    expect(markup).toContain('data-library-device-details-presentation="true"');
    expect(markup).toContain('data-task-folder-artwork="true"');
    expect(markup.match(/data-companion-width-contract="390px"/g)).toHaveLength(8);
    expect(markup).toContain('data-appearance-library-companion="task"');
    expect(markup).toContain('data-appearance-library-companion="task-versions"');
    expect(markup).toContain('data-appearance-library-companion="task-activity"');
    expect(markup).toContain('data-appearance-library-companion="deleted"');
    expect(markup).toContain('data-companion-scroll-owner="library-deleted"');
    expect(markup).toContain('data-appearance-library-companion="video"');
    expect(markup).toContain(
      'data-appearance-library-companion="image-ultrawide"',
    );
    expect(markup).toContain(
      'data-appearance-library-companion="image-landscape-long-title"',
    );
    expect(markup).toContain(
      'data-appearance-library-companion="footer-tabs"',
    );
    expect(markup).toContain('class="app-library-preview__tabs-frame"');
    expect(markup).toContain('class="app-dream-segment-switch app-library-preview__tabs"');
    expect(markup).toContain('data-count="4"');
    expect(markup.match(/class="app-library-preview__body"/g)?.length ?? 0)
      .toBeGreaterThanOrEqual(3);
    expect(markup).toContain('data-presentation="companion-open"');
    expect(markup).toContain('data-placement="inside"');
    expect(markup).toContain('data-placement="outside"');
    expect(markup).toContain('data-preview-kind="thumbnail" data-preview-source="asset"');
    expect(markup).toContain('data-preview-kind="audio" data-preview-source="fallback"');
    expect(markup).toContain('href="/dreamcreator.png"');
    expect(markup).not.toContain('href="xiadown-library-default:audio"');
    expect(markup).toContain('data-preview-kind="video"');
    expect(markup).toContain('data-appearance-library-ipod="video"');
    expect(markup).toContain('data-media-kind="video"');
    expect(markup).toContain(
      'data-appearance-library-ipod="image-ultrawide"',
    );
    expect(markup).toContain('data-source-dimensions="2400x500"');
    expect(markup).toContain('data-dialog-regression="16:9-long-title"');
    expect(markup).toContain('data-source-dimensions="1280x720"');
    expect(markup).toContain('data-media-kind="image"');
    expect(markup).toContain("Ultrawide image · 2400 × 500");
    expect(markup).toContain(
      "Lofi Music Project-this is what pure nostalgia sounds like ｜ emotional support lo-fi-medium-IwhR61zEFPo",
    );
    expect(markup).toContain("width=&#x27;2400&#x27;");
    expect(markup).toContain("height=&#x27;500&#x27;");
    expect(markup).toMatch(
      /data-appearance-library-ipod="image-ultrawide"[\s\S]*?<button[^>]*aria-label="Preview"[^>]*data-position="top"/,
    );
    expect(markup).toContain("app-library-ipod__screen");
    expect(markup).toContain("app-library-ipod__wheel");
    expect(markup).toMatch(
      /listen-workspace-fullscreen-backdrop__artwork[\s\S]*?<img src="\/dreamcreator\.png"/,
    );
    expect(markup).toContain('data-appearance-fixture="undivided-dialog"');
    expect(markup).toContain("app-dialog-header");
    expect(markup).toContain("app-dialog-list-card");
    expect(markup).toContain("app-dialog-footer");
    expect(markup).toContain('data-appearance-fixture="youtube-rss-watch"');
    expect(markup.match(/data-appearance-watch-root="youtube"/g)).toHaveLength(1);
    expect(markup.match(/data-appearance-watch-root="rss"/g)).toHaveLength(1);
    expect(markup).toContain(
      "youtube-workspace-page app-workspace-primary-subpane appearance-lab__watch-root-sample",
    );
    expect(markup).toContain(
      "rss-workspace-page app-dream-window app-workspace-primary-subpane appearance-lab__watch-root-sample",
    );
    expect(markup).toContain("youtube-workspace-watch-page");
    expect(markup).toContain("youtube-workspace-watch-header");
    expect(markup).toContain("youtube-workspace-watch-video-region");
    expect(markup).toContain("youtube-workspace-watch-subscribe");
    expect(markup).toContain("youtube-uploader-subscribe");
    const subscriptionButtons = allButtons.filter((tag) =>
      tag.includes("youtube-subscription-icon-button"),
    );
    expect(subscriptionButtons).toHaveLength(2);
    expect(
      subscriptionButtons.every(
        (tag) =>
          tag.includes('data-variant="ghost"') &&
          tag.includes('data-size="compactIcon"') &&
          tag.includes('data-shape="circle"'),
      ),
    ).toBe(true);
    expect(markup.match(/data-workspace-fullscreen="true"/g)).toHaveLength(4);
    expect(
      markup.match(/data-appearance-fullscreen-transport="true"/g),
    ).toHaveLength(4);
    expect(
      markup.match(/class="listen-workspace-fullscreen-backdrop"/g),
    ).toHaveLength(4);
    expect(markup.match(/data-fullscreen-media-mode="cover"/g)).toHaveLength(2);
    expect(markup.match(/data-fullscreen-media-mode="lyrics"/g)).toHaveLength(1);
    expect(markup.match(/data-fullscreen-media-mode="video"/g)).toHaveLength(1);
    expect(fullscreenFooters).toHaveLength(4);
    expect(fullscreenPrimaryButtons).toHaveLength(4);
    expect(
      fullscreenPrimaryButtons.some((tag) => tag.includes('aria-label="Play"')),
    ).toBe(true);
    expect(
      fullscreenPrimaryButtons.some((tag) => tag.includes('aria-label="Pause"')),
    ).toBe(true);
    expect(
      fullscreenPrimaryButtons.every((tag) => tag.includes('data-shape="circle"')),
    ).toBe(true);
    expect(activeFooterButtons).toHaveLength(3);
    expect(
      activeFooterButtons.every((tag) => tag.includes('data-shape="circle"')),
    ).toBe(true);
    expect(fullscreenButtons.length).toBeGreaterThan(0);
    expect(
      fullscreenButtons.every((tag) => tag.includes('data-shape="circle"')),
    ).toBe(true);
  });

  test("exposes overlay, Windows, and reduced-transparency QA states", () => {
    const markup = renderFixture({
      appearance: "dark",
      platform: "windows",
      presentation: "overlay",
      reduceTransparency: true,
      surfaceStyle: "contrast",
    });
    const companionMaterial = markup.match(
      /<div[^>]*class="[^"]*app-workspace-chrome-material[^"]*"[^>]*data-glass-role="companion"[^>]*>/,
    )?.[0];

    expect(markup).toContain('data-appearance="dark"');
    expect(markup).toContain('data-platform="windows"');
    expect(markup).toContain('data-presentation="overlay"');
    expect(markup).toContain('data-reduce-transparency="true"');
    expect(markup).toContain('data-surface-style="contrast"');
    expect(markup).toContain("Windows · Acrylic");
    expect(markup).toContain('data-window-controls-platform="windows"');
    expect(markup).not.toContain("Windows · Mica");
    expect(markup).toContain('aria-label="Companion presentation"');
    expect(markup).toContain('aria-label="Platform preview"');
    expect(markup).toContain('data-window-controls-platform="windows"');
    expect(markup).toContain("app-window-control-glyph--minimize");
    expect(companionMaterial).toContain('data-material="panel"');
    expect(companionMaterial).toContain('data-elevation="floating"');
  });

  test("shows every canonical product role against the active surface style", () => {
    const markup = renderToStaticMarkup(
      <SurfaceRoleMatrix surfaceStyle="contrast" />,
    );

    expect(markup).toContain("Surface contract");
    expect(markup).toContain("The Settings window canvas");
    expect(markup).toContain('data-surface-style="contrast"');
    expect(
      markup.match(/class="app-glass-surface appearance-lab__role-swatch"/g),
    ).toHaveLength(XIA_SURFACE_ROLES.length);
    for (const role of XIA_SURFACE_ROLES) {
      expect(markup).toContain(`data-surface-role="${role}"`);
    }
    for (const material of XIA_GLASS_MATERIALS) {
      expect(markup).toContain(`data-material="${material}"`);
      expect(markup).toContain(
        `data-appearance-material-specimen="${material}"`,
      );
    }
    for (const elevation of GLASS_ELEVATIONS) {
      expect(markup).toContain(`data-elevation="${elevation}"`);
    }
    for (const shape of GLASS_SHAPES) {
      expect(markup).toContain(`data-shape="${shape}"`);
    }
    for (const tint of GLASS_TINTS) {
      expect(markup).toContain(`data-tint="${tint}"`);
    }
    for (const token of [
      "--app-surface-canvas",
      "--app-surface-status-*",
      "--app-surface-overlay-*",
      "--app-surface-card-*",
      "--app-surface-inset-fill",
      "--app-surface-control-fill",
    ]) {
      expect(markup).toContain(token);
    }
  });

  test("keeps the fixture CSS structural and lets shared glass own sampling", async () => {
    const [css, source, primitiveSource, catalogSource] = await Promise.all([
      Bun.file(new URL("./appearance-lab.css", import.meta.url)).text(),
      Bun.file(new URL("./AppearanceLab.tsx", import.meta.url)).text(),
      Bun.file(new URL("./PrimitiveFixtureGallery.tsx", import.meta.url)).text(),
      Bun.file(new URL("./DreamStyleCatalog.tsx", import.meta.url)).text(),
    ]);

    const productionPaint: string[] = [];
    postcss.parse(css).walkRules((rule) => {
      if (!/\.app-[a-z0-9_-]+/i.test(rule.selector)) {
        return;
      }
      rule.walkDecls((declaration) => {
        if (VISUAL_PROPERTY_PATTERN.test(declaration.prop)) {
          productionPaint.push(
            `${rule.selector}: ${declaration.prop}: ${declaration.value}`,
          );
        }
      });
    });

    expect(productionPaint).toEqual([]);
    expect(css).not.toContain(
      ".appearance-lab__prism-window .app-workspace-ambient-canvas",
    );
    expect(css).not.toContain(".app-workspace-chrome-material");
    expect(css).not.toContain("appearance-lab__prism-companion-host");
    expect(css).not.toContain("appearance-lab__prism-sidebar-host");
    expect(css).not.toContain("appearance-lab__prism-traffic-lights");
    expect(css).not.toContain("appearance-lab__prism-windows-controls");
    expect(css).not.toContain("appearance-lab__rail-");
    expect(css).not.toContain(
      '.appearance-lab__role-swatch[data-surface-role="status"]',
    );
    expect(css).toMatch(
      /\.appearance-lab__role-swatch\s*\{[^}]*display:\s*flex[^}]*padding:\s*var\(--app-space-3\)[^}]*\}/s,
    );
    expect(css).not.toMatch(
      /\.appearance-lab__role-swatch\s*\{[^}]*(?:background|border|box-shadow):/s,
    );
    expect(css).not.toContain("backdrop-filter");
    expect(source.match(/\bmaterial="(?:regular|panel|clear)"/g)).toHaveLength(3);
    expect(source.match(/surfaceRole="status"/g)).toHaveLength(3);
    expect(source).toContain('surfaceRole="overlay"');
    expect(source).toContain('surfaceRole="chrome"');
    expect(source).toContain('surfaceRole="control"');
    expect(source).toContain("<WorkspaceSearchControl");
    expect(source).toContain("<StatusBadge");
    expect(source).toContain("<PrimitiveFixtureGallery");
    expect(source).toContain("<DreamStyleCatalog");
    expect(source).toContain(
      'className="app-main-shell appearance-lab__prism-window"',
    );
    expect(source).toContain("<WindowControls");
    expect(source).toContain("platform={platform}");
    expect(source).toContain("runtimeEnabled={false}");
    expect(source).not.toContain("appearance-lab__prism-companion-host");
    expect(source).not.toContain("appearance-lab__prism-sidebar-host");
    expect(source).not.toContain("appearance-lab__prism-traffic-lights");
    expect(source).not.toContain("<AppRail");
    expect(source).not.toContain("<StationDock");
    expect(source).toContain("applyAppearanceLabPlatform(document.documentElement, prismPlatform)");
    expect(source).toContain("applyAppearanceLabNativeVideoPreview(");
    expect(source).toContain("document.documentElement,\n        nativeVideoPreview");
    expect(source).toContain('"windowMaterial",\n        "native"');
    expect(source).toContain('root.dataset.reduceTransparency = "true"');
    expect(source).toContain("delete root.dataset.reduceTransparency");
    expect(source).toContain("<span>06</span>");
    expect(primitiveSource).toContain("<span>07</span>");
    expect(catalogSource).toContain("<span>08</span>");
  });

  test("catalogs the shared YouTube and RSS page-root surface contract", async () => {
    const [youtubeCSS, rssCSS, layoutCSS, source] = await Promise.all([
      Bun.file(
        new URL("../../shared/styles/dream/youtube.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/rss.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/layout-contract.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./AppearanceLab.tsx", import.meta.url)).text(),
    ]);
    const sharedPageSurface =
      "background: var(--app-workspace-primary-subpane-surface)";

    expect(
      youtubeCSS.match(/\.youtube-workspace-page\s*\{([^}]*)\}/s)?.[1],
    ).toContain(sharedPageSurface);
    expect(
      rssCSS.match(/\.rss-workspace-page\s*\{([^}]*)\}/s)?.[1],
    ).toContain(sharedPageSurface);
    expect(
      layoutCSS.match(
        /\.app-workspace-primary-subpane\.app-dream-window\s*\{([^}]*)\}/s,
      )?.[1],
    ).toContain(sharedPageSurface);
    expect(source).toContain('data-appearance-watch-root="youtube"');
    expect(source).toContain('data-appearance-watch-root="rss"');
    expect(source).toContain("<DreamStyleCatalog");
  });

  test("keeps native playback paint out of feature CSS and the Sidebar", async () => {
    const [youtubeCSS, rssCSS] = await Promise.all([
      Bun.file(
        new URL("../../shared/styles/dream/youtube.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/rss.css", import.meta.url),
      ).text(),
    ]);
    const featureSurfaceDeclarations: string[] = [];
    const youtubeActiveTransparencySelectors: string[] = [];

    for (const [moduleName, css] of [
      ["youtube", youtubeCSS],
      ["rss", rssCSS],
    ] as const) {
      postcss.parse(css).walkDecls("--listen-native-video-surface", (decl) => {
        featureSurfaceDeclarations.push(`${moduleName}: ${decl.value}`);
      });
    }

    postcss.parse(youtubeCSS).walkRules((rule) => {
      if (!rule.selector.includes('data-youtube-workspace-video-active="true"')) {
        return;
      }
      const clearsPlaybackAncestry = rule.nodes.some(
        (node) =>
          node.type === "decl" &&
          node.prop === "background" &&
          node.value.includes("transparent"),
      );
      if (clearsPlaybackAncestry) {
        youtubeActiveTransparencySelectors.push(rule.selector);
      }
    });

    expect(featureSurfaceDeclarations).toEqual([]);
    expect(youtubeActiveTransparencySelectors).toHaveLength(1);
    expect(youtubeActiveTransparencySelectors[0]).not.toContain(
      ".app-workspace-sidebar",
    );
  });
});
