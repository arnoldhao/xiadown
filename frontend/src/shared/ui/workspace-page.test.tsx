import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  assertWorkspacePageContract,
  defineWorkspacePageContract,
  isWorkspacePageFooterCovered,
  isWorkspacePageHeaderScrolled,
  resolveWorkspacePageFooterLayer,
  resolveWorkspacePageHeaderLayer,
  WORKSPACE_PAGE_FOOTER_SCROLL_THRESHOLD,
  WORKSPACE_PAGE_HEADER_SCROLL_THRESHOLD,
  WorkspacePage,
  WorkspacePageContent,
  WorkspacePageFooter,
  WorkspacePageTopBar,
  type WorkspacePageContract,
} from "./workspace-page";

const browseContract = defineWorkspacePageContract({
  presentation: "primary",
  recipe: "browse",
  routeLabel: "New releases",
  topBar: "drag",
  heading: "display",
  contentLayout: "shelves",
  footer: "none",
  scroll: "content",
  density: "comfortable",
  immersion: "standard",
});

describe("WorkspacePage", () => {
  test("publishes the complete page contract as stable data attributes", () => {
    const markup = renderToStaticMarkup(
      <WorkspacePage contract={browseContract}>
        <WorkspacePageTopBar reserveWindowControls />
        <WorkspacePageContent>
          <p>Albums</p>
        </WorkspacePageContent>
      </WorkspacePage>,
    );

    expect(markup).toContain('data-page-presentation="primary"');
    expect(markup).toContain('data-page-recipe="browse"');
    expect(markup).toContain('data-page-topbar="drag"');
    expect(markup).toContain('data-page-heading="display"');
    expect(markup).toContain('data-page-content-layout="shelves"');
    expect(markup).toContain('data-page-footer="none"');
    expect(markup).toContain('data-page-footer-layer="absent"');
    expect(markup).not.toContain("data-page-footer-state");
    expect(markup).toContain('data-page-scroll="content"');
    expect(markup).toContain('data-page-density="comfortable"');
    expect(markup).toContain('data-page-immersion="standard"');
    expect(markup).toContain('data-page-header-layer="layered"');
    expect(markup).toContain('data-page-header-state="top"');
    expect(markup).toContain('data-scroll-owner="true"');
    expect(markup).toContain('data-glass-role="header"');
    expect(markup).toContain('data-surface-role="chrome"');
  });

  test("layers only ordinary Primary pages with the shared content scroller", () => {
    expect(resolveWorkspacePageHeaderLayer(browseContract)).toBe("layered");
    expect(
      resolveWorkspacePageHeaderLayer({
        ...browseContract,
        recipe: "search",
        topBar: "search",
      }),
    ).toBe("flow");
    expect(
      resolveWorkspacePageHeaderLayer({
        ...browseContract,
        scroll: "panes",
        contentLayout: "split",
      }),
    ).toBe("flow");
    expect(
      resolveWorkspacePageHeaderLayer({
        ...browseContract,
        immersion: "edge-to-edge",
        contentLayout: "canvas",
      }),
    ).toBe("flow");
    expect(
      resolveWorkspacePageHeaderLayer({
        ...browseContract,
        presentation: "companion",
      }),
    ).toBe("flow");
    expect(
      resolveWorkspacePageHeaderLayer({
        ...browseContract,
        topBar: "host-owned",
      }),
    ).toBe("absent");
  });

  test("layers true Footers independently from special Search Headers", () => {
    const paginationContract = defineWorkspacePageContract({
      ...browseContract,
      recipe: "collection",
      footer: "pagination",
    });

    expect(resolveWorkspacePageFooterLayer(paginationContract)).toBe(
      "layered",
    );
    expect(
      resolveWorkspacePageFooterLayer({
        ...paginationContract,
        recipe: "search",
        topBar: "search",
      }),
    ).toBe("layered");
    expect(
      resolveWorkspacePageFooterLayer({
        ...paginationContract,
        scroll: "panes",
        contentLayout: "split",
      }),
    ).toBe("flow");
    expect(
      resolveWorkspacePageFooterLayer({
        ...paginationContract,
        footer: "overlay",
      }),
    ).toBe("flow");
    expect(
      resolveWorkspacePageFooterLayer({
        ...paginationContract,
        presentation: "companion",
        footer: "host-owned",
      }),
    ).toBe("absent");
    expect(resolveWorkspacePageFooterLayer(browseContract)).toBe("absent");
  });

  test("changes Header material only after the shared scroll threshold", () => {
    expect(WORKSPACE_PAGE_HEADER_SCROLL_THRESHOLD).toBe(8);
    expect(isWorkspacePageHeaderScrolled(Number.NaN)).toBeFalse();
    expect(isWorkspacePageHeaderScrolled(-1)).toBeFalse();
    expect(isWorkspacePageHeaderScrolled(8)).toBeFalse();
    expect(isWorkspacePageHeaderScrolled(8.01)).toBeTrue();
  });

  test("shows Footer material only while content remains beneath it", () => {
    expect(WORKSPACE_PAGE_FOOTER_SCROLL_THRESHOLD).toBe(8);
    expect(isWorkspacePageFooterCovered(0, Number.NaN, 100)).toBeFalse();
    expect(isWorkspacePageFooterCovered(0, 100, 100)).toBeFalse();
    expect(isWorkspacePageFooterCovered(92, 200, 100)).toBeFalse();
    expect(isWorkspacePageFooterCovered(91.99, 200, 100)).toBeTrue();
    expect(isWorkspacePageFooterCovered(-20, 200, 100)).toBeTrue();
    expect(isWorkspacePageFooterCovered(0, 160, 100, 52)).toBeFalse();
    expect(isWorkspacePageFooterCovered(0, 161, 100, 52)).toBeTrue();
    expect(
      isWorkspacePageFooterCovered(0, 200, 100, Number.NaN),
    ).toBeFalse();
  });

  test("re-synchronizes restored content offsets before the next paint", async () => {
    const source = await Bun.file(
      new URL("./workspace-page.tsx", import.meta.url),
    ).text();

    expect(source).toContain("const owner = contentScrollOwnerRef.current");
    expect(source).toContain("owner?.scrollTop");
    expect(source).toContain("useIsomorphicLayoutEffect(() =>");
    expect(source).toContain("window.requestAnimationFrame(sync)");
    expect(source).toContain("new ResizeObserver(sync)");
    expect(source).toContain("new MutationObserver(sync)");
    expect(source).toContain("setContentScrollOwnerNode(");
    expect(source).toContain("readWorkspacePageFooterEndInset");
    expect(source).toContain("setContentScrollOwner(");
  });

  test("renders one visible h1 inside content for display and hero headings", () => {
    const displayMarkup = renderToStaticMarkup(
      <WorkspacePage contract={browseContract}>
        <WorkspacePageContent headingDescription="Fresh this week">
          <p>Albums</p>
        </WorkspacePageContent>
      </WorkspacePage>,
    );
    const heroMarkup = renderToStaticMarkup(
      <WorkspacePage contract={{ ...browseContract, heading: "hero" }}>
        <WorkspacePageContent>
          <p>Album</p>
        </WorkspacePageContent>
      </WorkspacePage>,
    );

    for (const markup of [displayMarkup, heroMarkup]) {
      expect(markup.match(/<h1/g)).toHaveLength(1);
      expect(markup).toContain(">New releases</h1>");
      expect(markup).not.toContain('class="app-visually-hidden"');
      const headingId = markup.match(/<h1 id="([^"]+)"/)?.[1];
      expect(headingId).toBeTruthy();
      expect(markup).toContain(`aria-labelledby="${headingId}"`);
    }
    expect(displayMarkup).toContain("Fresh this week");
    expect(heroMarkup).toContain('data-page-heading="hero"');
  });

  test("switches collection pages to one assistive h1 without a duplicate title", () => {
    const contract = defineWorkspacePageContract({
      ...browseContract,
      recipe: "collection",
      routeLabel: "Library",
      topBar: "actions",
      heading: "assistive",
      contentLayout: "card-grid",
      footer: "pagination",
      density: "regular",
    });
    const markup = renderToStaticMarkup(
      <WorkspacePage contract={contract}>
        <WorkspacePageTopBar actionsLabel="Library actions">
          <button type="button">Refresh</button>
        </WorkspacePageTopBar>
        <WorkspacePageContent>
          <p>Items</p>
        </WorkspacePageContent>
        <WorkspacePageFooter>Pages</WorkspacePageFooter>
      </WorkspacePage>,
    );

    expect(markup.match(/<h1/g)).toHaveLength(1);
    expect(markup).toContain('<h1 id="workspace-page-heading-');
    expect(markup).toContain('class="app-visually-hidden">Library</h1>');
    expect(markup).not.toContain("app-workspace-page__heading-title");
    expect(markup).toContain('role="group"');
    expect(markup).toContain('aria-label="Library actions"');
    expect(markup).toContain("app-workspace-primary-header__actions");
    expect(markup).toContain("wails-no-drag");
    expect(markup).toContain('data-page-footer="pagination"');
    expect(markup).toContain('data-page-footer-layer="layered"');
    expect(markup).toContain('data-page-footer-state="end"');
    expect(markup).toContain('data-glass-role="footer"');
    expect(markup).toContain("app-workspace-page__footer-material");
    expect(markup).toContain("<footer");
  });

  test("owns native dragging, caption reservation, and absent chrome", () => {
    const withChrome = renderToStaticMarkup(
      <WorkspacePage contract={browseContract}>
        <WorkspacePageTopBar reserveWindowControls>
          <button type="button">Back</button>
        </WorkspacePageTopBar>
        <WorkspacePageContent />
      </WorkspacePage>,
    );
    const hostOwned = defineWorkspacePageContract({
      ...browseContract,
      presentation: "companion",
      topBar: "host-owned",
      heading: "host-owned",
      footer: "host-owned",
      scroll: "host",
      immersion: "edge-to-edge",
    });
    const hostOwnedMarkup = renderToStaticMarkup(
      <WorkspacePage contract={hostOwned}>
        <WorkspacePageTopBar>Ignored</WorkspacePageTopBar>
        <WorkspacePageContent>Lyrics</WorkspacePageContent>
        <WorkspacePageFooter>Ignored</WorkspacePageFooter>
      </WorkspacePage>,
    );

    expect(withChrome).toContain("app-workspace-primary-header wails-drag");
    expect(withChrome).toContain(
      "app-workspace-page__topbar-drag-region wails-drag",
    );
    expect(withChrome).toContain('data-window-controls="true"');
    expect(withChrome).toContain(
      "app-workspace-primary-header__safe-area",
    );
    expect(hostOwnedMarkup).toContain('aria-label="New releases"');
    expect(hostOwnedMarkup).not.toContain("<h1");
    expect(hostOwnedMarkup).not.toContain("<header");
    expect(hostOwnedMarkup).not.toContain("<footer");
  });

  test("requires named custom exceptions and a usable route label", () => {
    expect(() =>
      assertWorkspacePageContract({
        ...browseContract,
        routeLabel: "   ",
      }),
    ).toThrow("non-empty routeLabel");

    expect(() =>
      assertWorkspacePageContract({
        ...browseContract,
        recipe: "custom",
      } as WorkspacePageContract),
    ).toThrow("requires customContractId");

    const contract = defineWorkspacePageContract({
      ...browseContract,
      contentLayout: "custom",
      customContractId: "youtube-watch-stage",
    });
    const markup = renderToStaticMarkup(
      <WorkspacePage contract={contract}>
        <WorkspacePageContent />
      </WorkspacePage>,
    );
    expect(markup).toContain(
      'data-page-custom-contract="youtube-watch-stage"',
    );
  });

  test("owns layered Header and Footer chrome in shared CSS", async () => {
    const [css, glass, tokens] = await Promise.all([
      Bun.file(
        new URL("../styles/dream/layout-contract.css", import.meta.url),
      ).text(),
      Bun.file(new URL("../styles/dream/glass.css", import.meta.url)).text(),
      Bun.file(new URL("../styles/dream/tokens.css", import.meta.url)).text(),
    ]);

    expect(css).toMatch(
      /\.app-workspace-page\s*\{[^}]*container:\s*workspace-page \/ inline-size[^}]*display:\s*grid[^}]*grid-template-rows:\s*auto minmax\(0, 1fr\) auto/s,
    );
    expect(tokens).toContain(
      "--app-page-gutter: clamp(1.25rem, 3cqw, 2.25rem);",
    );
    expect(tokens).toContain(
      "--app-workspace-page-display-heading-size: clamp(1.75rem, 2.2cqw, 2.25rem);",
    );
    expect(tokens).toContain(
      "--app-workspace-page-hero-heading-size: clamp(2rem, 3cqw, 2.75rem);",
    );
    expect(css).toMatch(
      /\.app-workspace-page__topbar\s*\{[^}]*grid-area:\s*topbar/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__topbar-drag-region\s*\{[^}]*min-width:\s*var\(--app-workspace-page-drag-region-min-width\)[^}]*flex:\s*1 1 auto/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__topbar-actions\s*\{[^}]*min-width:\s*0[^}]*flex:\s*0 1 auto[^}]*overflow-x:\s*auto[^}]*overflow-y:\s*hidden/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__topbar-actions::\-webkit-scrollbar\s*\{[^}]*display:\s*none/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__content\s*\{[^}]*grid-area:\s*content/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__footer\s*\{[^}]*grid-area:\s*footer/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__content\[data-page-scroll="content"\]\s*\{[^}]*overflow-y:\s*auto/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__footer\s*\{[^}]*min-height:\s*var\(--app-workspace-page-footer-min-height\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__footer\s*\{[^}]*border:\s*0/s,
    );
    expect(css).not.toMatch(
      /\.app-workspace-page__footer\s*\{[^}]*border-block-start:/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page\[data-page-footer="overlay"\][\s\S]*?padding-block-end:\s*calc\(/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page\[data-page-footer-layer="layered"\]\s*\{[^}]*grid-template-rows:\s*auto minmax\(0, 1fr\)/s,
    );
    expect(css).toMatch(
      /\[data-page-footer-layer="layered"\][\s\S]*?> \.app-workspace-page__footer\s*\{[^}]*position:\s*absolute[^}]*inset:\s*auto 0 0/s,
    );
    expect(css).toMatch(
      /\[data-page-footer-layer="layered"\][\s\S]*?> \.app-workspace-page__content\[data-page-scroll="content"\],[\s\S]*?padding-block-end:\s*calc\([^}]*--app-workspace-page-footer-min-height/s,
    );
    expect(css).toMatch(
      /\[data-page-footer-layer="layered"\][\s\S]*?> \.app-workspace-page__content\[data-page-scroll="content"\]::\-webkit-scrollbar-track\s*\{[^}]*margin-block-end:\s*var\(--app-workspace-page-footer-min-height\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page\[data-page-header-layer="layered"\]\s*\{[^}]*grid-template-rows:\s*minmax\(0, 1fr\) auto/s,
    );
    expect(css).toMatch(
      /\[data-page-header-layer="layered"\][\s\S]*?> \.app-workspace-page__topbar\s*\{[^}]*position:\s*absolute[^}]*inset:\s*0 0 auto/s,
    );
    expect(css).toMatch(
      /\[data-page-header-layer="layered"\][\s\S]*?> \.app-workspace-page__content\[data-page-scroll="content"\]\s*\{[^}]*padding-block-start:\s*calc\([^}]*--app-workspace-page-topbar-height/s,
    );
    expect(css).toMatch(
      /\[data-page-header-layer="layered"\][\s\S]*?> \.app-workspace-page__content\[data-page-scroll="content"\]::\-webkit-scrollbar-track\s*\{[^}]*margin-block-start:\s*var\(--app-workspace-page-topbar-height\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-page__topbar-material\s*\{[^}]*--app-glass-filter:\s*none[^}]*inset:\s*0[^}]*opacity:\s*0[^}]*-webkit-mask-image:\s*none[^}]*mask-image:\s*none/s,
    );
    expect(css).toContain(
      '.app-glass-surface[data-material="regular"][data-glass-role="header"].app-workspace-page__topbar-material',
    );
    const scrolledHeaderMaterialRule = css.match(
      /\.app-workspace-page\[data-page-header-layer="layered"\]\[data-page-header-state="scrolled"\][\s\S]*?\{([^}]*)\}/,
    )?.[1];
    expect(scrolledHeaderMaterialRule).toContain(
      "--app-glass-filter: var(--app-glass-chrome-filter)",
    );
    expect(scrolledHeaderMaterialRule).toContain("opacity: 1");
    expect(css).toContain(
      '.app-glass-surface[data-material="regular"][data-glass-role="footer"].app-workspace-page__footer-material',
    );
    expect(css).toMatch(
      /\.app-workspace-page__footer-material\s*\{[^}]*inset:\s*0[^}]*border:\s*0[^}]*opacity:\s*0[^}]*-webkit-mask-image:\s*none[^}]*mask-image:\s*none/s,
    );
    expect(css).toMatch(
      /\[data-page-footer-state="content"\][\s\S]*?> \.app-workspace-page__footer-material\s*\{[^}]*opacity:\s*1/s,
    );
    expect(css).toContain("@media (forced-colors: active)");
    expect(css).toMatch(
      /@media \(forced-colors: active\)[\s\S]*?\.app-glass-surface\[data-material="regular"\]\[data-glass-role="header"\]\.app-workspace-page__topbar-material\s*\{[^}]*--app-glass-filter:\s*none/s,
    );
    const contrastHeaderStart = css.indexOf(
      ':where(\n    :root[data-xiadown-surface-style="contrast"],',
    );
    const contrastHeaderEnd = css.indexOf(
      "@media (forced-colors: active)",
      contrastHeaderStart,
    );
    expect(contrastHeaderStart).toBeGreaterThanOrEqual(0);
    expect(contrastHeaderEnd).toBeGreaterThan(contrastHeaderStart);
    const contrastHeaderRule = css.slice(
      contrastHeaderStart,
      contrastHeaderEnd,
    );
    expect(contrastHeaderRule).toContain(
      ".app-workspace-page__topbar-material",
    );
    expect(contrastHeaderRule).toContain("--app-glass-filter: none");
    expect(css).toMatch(
      /@media \(forced-colors: active\)[\s\S]*?\.app-glass-surface\[data-material="regular"\]\[data-glass-role="footer"\]\.app-workspace-page__footer-material\s*\{[^}]*--app-glass-filter:\s*none/s,
    );
    expect(glass).toContain('[data-glass-role="header"]');
    expect(glass).toContain('[data-glass-role="footer"]');
    expect(css).not.toContain("--app-workspace-page-header-fade-size");
    expect(css).not.toContain("--app-workspace-page-footer-fade-size");
    expect(tokens).not.toContain("--app-workspace-page-header-fade-size");
    expect(tokens).not.toContain("--app-workspace-page-footer-fade-size");
  });
});
