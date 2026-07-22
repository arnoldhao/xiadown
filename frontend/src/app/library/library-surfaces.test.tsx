import { describe, expect, mock, test } from "bun:test";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";

import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";
import { t } from "@/shared/i18n";
import { createLibraryWorkspaceLabels, type LibraryWorkspaceItem } from "./types";

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByID: () => Promise.resolve(undefined),
    ByName: () => Promise.resolve(undefined),
  },
  Create: {
    Any: (value: unknown) => value,
    Array: (create: (value: unknown) => unknown) => (values: unknown[]) => values.map(create),
    Nullable: (create: (value: unknown) => unknown) => (value: unknown) => value == null ? value : create(value),
  },
  Events: {
    On: () => () => {},
    Types: { Common: { WindowFullscreen: "window-fullscreen", WindowUnFullscreen: "window-unfullscreen" } },
  },
  Window: { Fullscreen: () => Promise.resolve(), UnFullscreen: () => Promise.resolve() },
}));

const {
  LibraryDeleteConfirmationActions,
  LibraryPreviewCompanion,
  LibraryPreviewCompanionFooter,
} = await import("./LibraryPreviewCompanion");
const {
  LibraryWorkspacePage,
  buildLibraryDeletionPlan,
  executeLibraryDeletionPlan,
  executeLibraryItemDeletionBatch,
  isLibraryContextMenuKey,
  resolveLibraryKeyboardContextMenuPoint,
} = await import("./LibraryWorkspacePage");

function item(
  id: string,
  category: LibraryWorkspaceItem["category"],
  overrides: Partial<LibraryWorkspaceItem> = {},
): LibraryWorkspaceItem {
  const coverURL = category === "video"
    ? COMPLETED_DEFAULT_COVER_IMAGE_URLS.video
    : category === "audio"
      ? COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio
      : category === "book"
        ? COMPLETED_DEFAULT_COVER_IMAGE_URLS.document
        : category === "image"
          ? COMPLETED_DEFAULT_COVER_IMAGE_URLS.image
          : COMPLETED_DEFAULT_COVER_IMAGE_URLS.other;
  return {
    id,
    source: category === "task" ? "task" : "file",
    libraryId: "catalog-one",
    libraryName: "Home Library",
    title: id,
    subtitle: category,
    category,
    status: "available",
    format: "TEST",
    createdAt: "2026-01-01T10:00:00Z",
    updatedAt: "2026-02-01T10:00:00Z",
    path: `/Library/${id}`,
    coverURL,
    rootId: id,
    searchText: `${id} ${category}`.toLowerCase(),
    ...overrides,
  };
}

describe("library primary surfaces", () => {
  test("renders each primary route from normalized items without CompletedPage", async () => {
    const items = [
      item("Feature Film", "video"),
      item("Studio Album", "audio"),
      item("Field Notes", "book"),
      item("Portrait", "image"),
      item("Download Task", "task"),
    ];
    const videoMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage route="video" items={items} />,
    );
    const taskMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage route="tasks" items={items} />,
    );
    const source = await Bun.file(new URL("./LibraryWorkspacePage.tsx", import.meta.url)).text();

    expect(videoMarkup).toContain('data-library-route="video"');
    expect(videoMarkup).toMatch(/<h1[^>]*class="app-visually-hidden"[^>]*>Video<\/h1>/);
    expect(videoMarkup).toContain('data-page-recipe="collection"');
    expect(videoMarkup).toContain('data-page-heading="assistive"');
    expect(videoMarkup).not.toContain("app-library-toolbar__title");
    expect(videoMarkup).toContain("Feature Film");
    expect(videoMarkup).not.toContain("Studio Album");
    expect(taskMarkup).toContain("Download Task");
    expect(taskMarkup).not.toContain("Feature Film");
    expect(source).not.toContain("CompletedPage");
  });

  test("keeps Others as a sidebar-colored group column plus one independent primary content column", async () => {
    const markup = renderToStaticMarkup(
      <LibraryWorkspacePage
        route="others"
        initialOtherGroup="font"
        items={[
          item("Inter", "other", { otherGroup: "font" }),
          item("Bundle", "other", { otherGroup: "archive" }),
        ]}
      />,
    );
    const [layoutCss, appearanceCss] = await Promise.all([
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/library.css", import.meta.url),
      ).text(),
    ]);

    expect(markup).toContain("app-library-content--others");
    expect(markup).toContain(
      "app-library-other-pane app-main-list-pane app-main-sidebar",
    );
    expect(markup).toContain("app-workspace-primary-subpane--leading");
    expect(markup).toContain("app-library-other-groups");
    expect(markup).toContain("app-library-other-groups__label");
    expect(markup).toMatch(
      /app-library-other-groups__label[^>]*><svg[\s\S]*?<span>Fonts<\/span>/,
    );
    expect(markup).toContain(
      "app-library-primary-surface app-library-primary-surface--other app-main-detail-pane",
    );
    const headerIndex = markup.indexOf("app-library-page__header");
    const splitContentIndex = markup.indexOf("app-library-content--others");
    const listPaneEnd = markup.indexOf("</aside>");
    const detailContentIndex = markup.indexOf("app-library-other-detail");
    expect(headerIndex).toBeGreaterThan(0);
    expect(splitContentIndex).toBeGreaterThan(headerIndex);
    expect(listPaneEnd).toBeGreaterThan(splitContentIndex);
    expect(detailContentIndex).toBeGreaterThan(listPaneEnd);
    expect(markup).toContain('data-page-content-layout="split"');
    expect(markup).toContain('data-page-scroll="panes"');
    expect(markup).toContain('data-page-footer="none"');
    expect(markup).not.toContain("app-library-page__footer");
    expect(markup).toMatch(/<h1[^>]*class="app-visually-hidden"[^>]*>Others<\/h1>/);
    expect(markup).toContain('aria-current="page"');
    expect(markup).toContain("Inter");
    expect(markup).not.toContain('data-item-id="Bundle"');
    expect(markup).toContain("Needs review");
    expect(markup).toContain("Missing");
    expect(layoutCss).toMatch(
      /\.app-library-page \.app-library-other-pane\.app-main-list-pane\s*\{[^}]*position:\s*relative[^}]*overflow:\s*hidden/s,
    );
    expect(appearanceCss).toMatch(
      /\.app-library-page \.app-library-other-pane\.app-main-list-pane\s*\{[^}]*color:\s*hsl\(var\(--sidebar-foreground\)\)/s,
    );
    expect(layoutCss).toMatch(
      /\.app-library-page \.app-library-other-pane\.app-main-list-pane\s*\{[^}]*width:\s*clamp\(15rem, 24cqw, 20rem\)[^}]*flex:\s*0 0 clamp\(15rem, 24cqw, 20rem\)/s,
    );
    expect(layoutCss).not.toContain("clamp(15rem, 24vw, 20rem)");
    expect(layoutCss).toMatch(/\.app-library-other-groups\s*\{[^}]*overflow-y:\s*auto/s);
    expect(layoutCss).toMatch(/\.app-library-primary\s*\{[^}]*overflow:\s*auto/s);
    expect(layoutCss).not.toContain(
      "border-right: 1px solid hsl(var(--sidebar-border))",
    );
  });

  test("keeps Search below an accessible drag rail before station result tools", async () => {
    const searchItems = Array.from({ length: 49 }, (_, index) =>
      item(`Film ${index + 1}`, "video"),
    );
    const landingMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage route="search" items={searchItems} />,
    );
    const resultsMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage
        route="search"
        initialQuery="film"
        items={searchItems}
      />,
    );
    const [libraryCss, layoutContract] = await Promise.all([
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/layout-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(landingMarkup).toContain('data-search-state="landing"');
    expect(landingMarkup).toContain("app-library-toolbar--search app-station-search-header");
    expect(landingMarkup).toMatch(/<h1[^>]*class="app-visually-hidden"[^>]*>Search<\/h1>/);
    expect(landingMarkup).toContain('data-page-recipe="search"');
    expect(landingMarkup).toContain('data-page-topbar="search"');
    expect(landingMarkup).not.toContain("app-station-search-header__title");
    expect(landingMarkup).toContain(
      "app-dream-workspace-search app-dream-search-control app-dream-control-shell app-station-search-content-search",
    );
    expect(landingMarkup).toContain('type="search"');
    expect(landingMarkup).not.toContain('data-item-id="A Film"');
    expect(landingMarkup).not.toContain('aria-label="Sort"');
    expect(resultsMarkup).toContain('data-search-state="results"');
    expect(resultsMarkup).toContain('data-item-id="Film 1"');
    expect(resultsMarkup).toContain('data-page-header-layer="flow"');
    expect(resultsMarkup).toContain('data-page-footer="pagination"');
    expect(resultsMarkup).toContain('data-page-footer-layer="layered"');
    expect(resultsMarkup).toContain('data-glass-role="footer"');
    expect(resultsMarkup).toContain("app-library-search-results-toolbar");
    expect(resultsMarkup).toContain('aria-label="Sort"');
    expect(resultsMarkup).toContain('aria-label="Grid"');
    expect(resultsMarkup).toContain('aria-label="List"');
    const headerStart = resultsMarkup.indexOf("app-library-toolbar--search");
    const headerEnd = resultsMarkup.indexOf("</header>", headerStart);
    const searchControl = resultsMarkup.indexOf(
      "app-dream-workspace-search",
      headerEnd,
    );
    const resultTools = resultsMarkup.indexOf(
      "app-library-search-results-toolbar",
      searchControl,
    );
    expect(searchControl).toBeGreaterThan(headerEnd);
    expect(resultTools).toBeGreaterThan(searchControl);
    expect(resultsMarkup.slice(headerStart, headerEnd)).not.toContain('type="search"');
    expect(resultsMarkup.slice(headerStart, headerEnd)).not.toContain('aria-label="Sort"');
    expect(libraryCss).not.toContain(".app-library-toolbar--search {");
    expect(libraryCss).not.toContain(".app-library-search-page__control");
    expect(layoutContract).toContain(".app-station-search-content-search {");
  });

  test("uses one continuous header-content-footer surface and reserves native Windows controls", async () => {
    const items = Array.from({ length: 49 }, (_, index) =>
      item(`Film ${index + 1}`, "video"),
    );
    const markup = renderToStaticMarkup(
      <LibraryWorkspacePage
        route="all"
        items={items}
        reserveWindowControls
      />,
    );
    const [css, layoutContract] = await Promise.all([
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/layout-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(markup).not.toContain("app-library-header");
    expect(markup).toContain("app-workspace-page app-library-page");
    expect(markup).toContain("app-library-page__header");
    expect(markup).toContain("app-library-page__content");
    expect(markup).toContain("app-library-page__footer");
    expect(markup).toContain("app-workspace-page__topbar-actions");
    expect(markup).toContain("app-workspace-primary-header__actions");
    expect(markup).toContain('data-window-controls="true"');
    expect(markup).not.toContain("app-library-toolbar__windows-actions");
    expect(markup).toMatch(/<h1[^>]*class="app-visually-hidden"[^>]*>All files<\/h1>/);
    expect(markup).toContain('data-search-expanded="false"');
    expect(markup).toContain("app-library-search-toggle");
    expect(markup).not.toContain('type="search"');
    expect(markup).toMatch(
      /app-library-page__header[\s\S]*app-library-page__content[\s\S]*app-library-page__footer/,
    );
    const actionsIndex = markup.indexOf("app-workspace-page__topbar-actions");
    expect(actionsIndex).toBeGreaterThan(0);
    expect(markup).not.toContain("app-library-toolbar__title");
    expect(markup).toContain('data-page-footer="pagination"');
    expect(markup).toContain('data-page-footer-layer="layered"');
    expect(markup).toContain('data-page-footer-state="end"');
    expect(markup).toContain('data-glass-role="footer"');
    expect(markup).toContain("app-workspace-page__footer-material");
    expect(markup.match(/<footer/g)).toHaveLength(1);
    expect(markup).toContain("app-workspace-page__topbar-safe-area");
    expect(css).not.toMatch(/\.app-library-toolbar\s*\{[^}]*position:\s*sticky/s);
    expect(css).toMatch(/\.app-library-primary\s*\{[^}]*overflow:\s*auto/s);
    expect(layoutContract).toMatch(/\.app-workspace-page\s*\{[^}]*grid-template-rows:\s*auto minmax\(0, 1fr\) auto/s);
    expect(layoutContract).toContain(".app-workspace-page__topbar-safe-area");
  });

  test("uses canonical ghost actions and retains secondary operations in More", async () => {
    const collapsedMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage route="all" items={[]} />,
    );
    const expandedMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage route="all" initialQuery="film" items={[]} />,
    );
    const [source, css] = await Promise.all([
      Bun.file(new URL("./LibraryWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./library.css", import.meta.url)).text(),
    ]);

    const moreIndex = collapsedMarkup.indexOf('aria-label="More"');
    const searchIndex = collapsedMarkup.indexOf('aria-label="Search"');
    expect(collapsedMarkup).toContain('aria-label="Sort"');
    expect(collapsedMarkup).toContain('aria-label="Grid"');
    expect(collapsedMarkup).toContain('aria-label="List"');
    expect(collapsedMarkup).toContain('aria-label="More"');
    expect(collapsedMarkup).toContain('aria-label="Search"');
    expect(collapsedMarkup).toContain('aria-pressed="true"');
    expect(collapsedMarkup).toContain('data-variant="ghost"');
    expect(collapsedMarkup).toContain('data-page-footer="none"');
    expect(collapsedMarkup).toContain('data-page-footer-layer="absent"');
    expect(collapsedMarkup).not.toContain("app-library-page__footer");
    expect(searchIndex).toBeGreaterThan(moreIndex);
    expect(expandedMarkup).toContain('data-search-expanded="true"');
    expect(expandedMarkup).toContain('type="search"');
    expect(source).toContain("setSearchExpanded(nextQuery.length > 0)");
    expect(source).toContain("WorkspacePrimaryHeaderAction");
    expect(source).toContain("WorkspacePrimaryHeaderActionGroup");
    expect(source.match(/<WorkspacePrimaryHeaderMenuContent/g)).toHaveLength(2);
    expect(source).toContain(
      'className="app-menu-content-fit app-library-actions-menu"',
    );
    expect(source).not.toMatch(/DropdownMenuContent align="(?:start|end)"/);
    expect(source).toContain("<DropdownMenuItem");
    expect(source).toContain('setManagementSection("summary")');
    expect(source).toContain('setManagementSection("data")');
    expect(css).toContain("container: app-library-primary / inline-size");
    expect(css).not.toContain(".app-library-action--health");
    expect(css).not.toContain(".app-library-action--manage");
    expect(css).not.toContain(".app-library-actions-menu.app-menu-content");
    expect(css).not.toMatch(/app-library-view-switch[^}]*display:\s*none/s);
  });

  test("shows explicit loading and retry states instead of a truncated Tasks list", () => {
    const hiddenTask = item("Partial Task", "task");
    const loading = renderToStaticMarkup(
      <LibraryWorkspacePage route="tasks" items={[hiddenTask]} loading />,
    );
    const failed = renderToStaticMarkup(
      <LibraryWorkspacePage route="tasks" items={[hiddenTask]} loadError />,
    );
    expect(loading).toContain('role="status"');
    expect(loading).toContain("Loading");
    expect(loading).not.toContain('data-item-id="Partial Task"');
    expect(failed).toContain('role="alert"');
    expect(failed).toContain("Try again");
  });

  test("keeps keyboard focus visible for Library controls that suppress native outlines", async () => {
    const css = await Bun.file(
      new URL("../../shared/styles/dream/library.css", import.meta.url),
    ).text();
    expect(css).toContain('.app-library-search:focus-within');
    expect(css).not.toContain('.app-library-sort:focus-visible');
    expect(css).toContain('.app-catalog-roots__add input:focus-visible');
    expect(css).toMatch(/focus-within[^}]*outline:\s*2px solid/s);
    expect(css).toMatch(/app-catalog-roots__add input:focus-visible[^}]*outline:\s*2px solid/s);
  });

  test("uses the shared compact menu treatment for Library sorting", async () => {
    const [source, layoutCss, appearanceCss] = await Promise.all([
      Bun.file(new URL("./LibraryWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/library.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("<DropdownMenuRadioGroup");
    expect(source).toContain('className="app-library-sort-menu__item"');
    expect(source).not.toContain('<select\n                  value={sort}');
    expect(layoutCss).toMatch(/\.app-library-sort-menu__item\.app-menu-item\s*\{[^}]*min-height:\s*1\.75rem/s);
    expect(appearanceCss).toMatch(/\.app-library-sort-menu__item\.app-menu-item\s*\{[^}]*font-size:\s*0\.75rem/s);
  });

  test("keeps Catalog trash lifecycle separate from physical legacy deletion", () => {
    const catalog = item("Catalog card", "video", {
      catalogItem: {
        id: "catalog-item-opaque",
        catalogId: "catalog-one",
        category: "video",
        status: "active",
        title: "Catalog card",
        sortTitle: "Catalog card",
        revision: 7,
        createdAt: "2026-01-01T10:00:00Z",
        updatedAt: "2026-02-01T10:00:00Z",
      },
      // A hydrated Catalog card may eventually carry asset data. Catalog must
      // still win over the legacy physical-file branch.
      file: { id: "must-not-delete-as-file" } as LibraryWorkspaceItem["file"],
    });
    const legacyFile = item("Legacy file", "video", {
      file: { id: "legacy-physical-file" } as LibraryWorkspaceItem["file"],
    });
    const task = item("Legacy task", "task", {
      operation: {
        operationId: "legacy-operation",
        libraryId: "catalog-one",
        name: "Legacy task",
        kind: "download",
        status: "completed",
        correlation: {},
        metrics: { fileCount: 1 },
        createdAt: "2026-01-01T10:00:00Z",
      },
    });

    expect(buildLibraryDeletionPlan([catalog, legacyFile, task])).toEqual({
      operationIds: ["legacy-operation"],
      catalogItems: [{ id: "catalog-item-opaque", expectedRevision: 7 }],
      fileIds: ["legacy-physical-file"],
    });
  });

  test("trashes Catalog items sequentially between legacy batch operations", async () => {
    const calls: string[] = [];
    let releaseFirstCatalog!: () => void;
    const firstCatalog = new Promise<void>((resolve) => {
      releaseFirstCatalog = resolve;
    });
    const execution = executeLibraryDeletionPlan({
      operationIds: ["operation-one"],
      catalogItems: [
        { id: "catalog-one", expectedRevision: 1 },
        { id: "catalog-two", expectedRevision: 2 },
      ],
      fileIds: ["legacy-file-one"],
    }, {
      deleteOperations: async (request) => {
        calls.push(`operations:${request.operationIds.join(",")}`);
      },
      trashCatalogItem: async (request) => {
        calls.push(`catalog:${request.id}:${request.expectedRevision}:${request.actorId}`);
        if (request.id === "catalog-one") await firstCatalog;
      },
      deleteFiles: async (request) => {
        calls.push(`files:${request.fileIds.join(",")}`);
      },
    });

    await Promise.resolve();
    await Promise.resolve();
    expect(calls).toEqual([
      "operations:operation-one",
      "catalog:catalog-one:1:desktop-library",
    ]);
    releaseFirstCatalog();
    await execution;
    expect(calls).toEqual([
      "operations:operation-one",
      "catalog:catalog-one:1:desktop-library",
      "catalog:catalog-two:2:desktop-library",
      "files:legacy-file-one",
    ]);
  });

  test("retries only Catalog cards that failed the previous batch", async () => {
    const first = item("First card", "video", {
      catalogItem: {
        id: "catalog-first",
        catalogId: "catalog-one",
        category: "video",
        status: "active",
        title: "First card",
        sortTitle: "First card",
        revision: 4,
        createdAt: "2026-01-01T10:00:00Z",
        updatedAt: "2026-02-01T10:00:00Z",
      },
    });
    const second = item("Second card", "video", {
      catalogItem: {
        id: "catalog-second",
        catalogId: "catalog-one",
        category: "video",
        status: "active",
        title: "Second card",
        sortTitle: "Second card",
        revision: 9,
        createdAt: "2026-01-01T10:00:00Z",
        updatedAt: "2026-02-01T10:00:00Z",
      },
    });
    const calls: string[] = [];
    let failSecond = true;
    const mutations = {
      deleteOperations: async () => {},
      trashCatalogItem: async (request: { id: string; expectedRevision: number; actorId: string }) => {
        calls.push(`${request.id}:${request.expectedRevision}:${request.actorId}`);
        if (request.id === "catalog-second" && failSecond) {
          throw new Error("revision conflict");
        }
      },
      deleteFiles: async () => {},
    };

    const initialResult = await executeLibraryItemDeletionBatch(
      [first, second],
      mutations,
    );
    expect(initialResult.deletedItemIds).toEqual(["First card"]);
    expect(initialResult.failures.map((failure) => failure.item.id)).toEqual([
      "Second card",
    ]);

    failSecond = false;
    const retryResult = await executeLibraryItemDeletionBatch(
      initialResult.failures.map((failure) => failure.item),
      mutations,
    );
    expect(retryResult).toEqual({
      deletedItemIds: ["Second card"],
      failures: [],
    });
    expect(calls).toEqual([
      "catalog-first:4:desktop-library",
      "catalog-second:9:desktop-library",
      "catalog-second:9:desktop-library",
    ]);
  });

  test("registers accessible context and current-page batch interactions", async () => {
    const catalog = item("Context target", "video", {
      catalogItem: {
        id: "catalog-context-target",
        catalogId: "catalog-one",
        category: "video",
        status: "active",
        title: "Context target",
        sortTitle: "Context target",
        revision: 3,
        createdAt: "2026-01-01T10:00:00Z",
        updatedAt: "2026-02-01T10:00:00Z",
      },
    });
    const markup = renderToStaticMarkup(
      <LibraryWorkspacePage
        route="all"
        items={[catalog]}
        onItemClick={() => {}}
      />,
    );
    const searchMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage
        route="search"
        initialQuery="context"
        items={[catalog]}
        onItemClick={() => {}}
      />,
    );
    const [source, layoutCss, appearanceCss, mainSource] = await Promise.all([
      Bun.file(new URL("./LibraryWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/library.css", import.meta.url)).text(),
      Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
    ]);

    expect(markup).toContain('aria-label="Select"');
    expect(markup).toContain('data-selection-mode="false"');
    expect(markup).toContain('data-batch-selected="false"');
    expect(searchMarkup).toContain("app-library-search-results-toolbar");
    expect(searchMarkup).toContain('aria-label="Select"');
    expect(source).toContain("onContextMenu={(event) => {");
    expect(source).toContain('modal={false}');
    expect(source).toContain("onCloseAutoFocus={(event) => {");
    expect(source).toContain("SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME");
    expect(source).toContain('tone="destructive"');
    expect(layoutCss).toContain(".app-library-item__selection-indicator");
    expect(appearanceCss).toContain('[data-batch-selected="true"]');
    expect(appearanceCss).toMatch(
      /\.app-library-item\[data-category="task"\]:is\([\s\S]*?\)\s*\.app-library-task-folder\s*\{[^}]*outline:/,
    );
    expect(appearanceCss).not.toMatch(
      /data-category="task"\]\[data-selected="true"\][\s\S]{0,100}\.app-library-task-folder__front-cover\s*\{[^}]*outline:/,
    );
    expect(mainSource).toMatch(
      /const workspaceNewAction[\s\S]*?<DropdownMenuContent[\s\S]*?className=\{SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME\}/,
    );
    expect(isLibraryContextMenuKey("ContextMenu")).toBe(true);
    expect(isLibraryContextMenuKey("F10", true)).toBe(true);
    expect(isLibraryContextMenuKey("F10", false)).toBe(false);
    expect(resolveLibraryKeyboardContextMenuPoint({
      left: 10,
      bottom: 40,
      width: 20,
    })).toEqual({ x: 20, y: 40 });
  });
});

describe("library single-item companion", () => {
  test("previews an open task folder, status and complete actionable file list", () => {
    const task = item("Active Download", "task", {
      status: "running",
      path: "https://example.com/video",
      sizeBytes: 4096,
      durationMs: 65_000,
      taskPreviewItems: [
        {
          id: "operation-1:cover",
          kind: "thumbnail",
          previewURL: "http://127.0.0.1:43127/assets/task-cover.webp",
        },
        { id: "operation-1:video", kind: "video", label: "MP4" },
      ],
      taskPreviewTotalCount: 3,
      taskFiles: [
        {
          fileId: "video-file",
          previewItemId: "file:video-file",
          title: "video.mp4",
          kind: "video",
          category: "video",
          status: "available",
          format: "MP4",
          canView: true,
          file: {
            id: "video-file",
            libraryId: "catalog-one",
            kind: "video",
            name: "video.mp4",
            displayName: "video.mp4",
            storage: { mode: "local_path", localPath: "/Library/video.mp4" },
            origin: { kind: "download", operationId: "operation-1" },
            lineage: {},
            metadata: {},
            media: { format: "mp4", sizeBytes: 4096 },
            state: { status: "available", deleted: false, archived: false },
            createdAt: "2026-01-01T10:00:00Z",
            updatedAt: "2026-01-01T10:02:00Z",
          },
        },
        {
          fileId: "missing-file",
          previewItemId: "file:missing-file",
          title: "missing.vtt",
          kind: "subtitle",
          category: "other",
          status: "missing",
          format: "VTT",
          canView: false,
        },
      ],
      operation: {
        operationId: "operation-1",
        libraryId: "catalog-one",
        name: "Active Download",
        kind: "download",
        status: "running",
        correlation: {},
        domain: "example.com",
        request: { url: "https://example.com/video" },
        progress: {
          percent: 42,
          stage: "i18n:library.status.succeeded",
          speed: "2 MB/s",
        },
        metrics: { fileCount: 3, totalSizeBytes: 4096, durationMs: 65_000 },
        createdAt: "2026-01-01T10:00:00Z",
        startedAt: "2026-01-01T10:01:00Z",
      },
    });
    const client = new QueryClient();
    const markup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={task} onOpenItem={() => {}} />
      </QueryClientProvider>,
    );
    expect(markup).toContain('data-preview-kind="task"');
    expect(markup).toContain('data-task-folder-artwork="true"');
    expect(markup).toContain('class="app-library-task-folder app-library-preview__task-folder-artwork"');
    expect(markup).toContain('data-view="grid"');
    expect(markup).toContain('data-presentation="companion-open"');
    expect(markup).toContain('data-total-count="3"');
    expect(markup).toContain("http://127.0.0.1:43127/assets/task-cover.webp");
    expect(markup).not.toContain('class="app-library-preview__media" data-preview-kind="task"');
    expect(markup).toContain('class="app-library-preview__title-marquee"');
    expect(markup).toContain("Progress");
    expect(markup).toContain("42% · Completed · 2 MB/s");
    expect(markup).not.toContain("i18n:");
    expect(markup).toContain(
      'class="app-library-preview__progress-card app-dialog-list-card"',
    );
    expect(markup).toContain('<progress max="100" value="42"');
    expect(markup).toContain("Outputs");
    expect(markup).toContain("video.mp4");
    expect(markup).toContain("missing.vtt");
    expect(markup).toContain('aria-label="Rename: video.mp4"');
    expect(markup).not.toContain('aria-label="Rename: missing.vtt"');
    expect(markup).toContain("View</button>");
    expect(markup).toContain('class="app-library-preview__task-file-actions"');
    expect(markup).toContain("app-library-preview__task-file-delete");
    expect(markup).toContain('data-tone="destructive"');
    const runningDeleteStart = markup.indexOf("app-library-preview__task-file-delete");
    const runningDeleteEnd = markup.indexOf("</button>", runningDeleteStart);
    expect(markup.slice(runningDeleteStart, runningDeleteEnd)).toContain('disabled=""');
    expect(markup).toContain('disabled="" type="button"><svg');
    expect(markup).not.toContain("https://example.com/video");

    const succeededTask = {
      ...task,
      status: "succeeded",
      operation: { ...task.operation!, status: "succeeded" },
    };
    const succeededMarkup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={succeededTask} onOpenItem={() => {}} />
      </QueryClientProvider>,
    );
    const succeededDeleteStart = succeededMarkup.indexOf("app-library-preview__task-file-delete");
    const succeededDeleteEnd = succeededMarkup.indexOf("</button>", succeededDeleteStart);
    expect(succeededMarkup.slice(succeededDeleteStart, succeededDeleteEnd)).not.toContain('disabled=""');
  });

  test("projects task-specific info as a direct card without preview-only actions", () => {
    const client = new QueryClient();
    const task = item("Finished Download", "task", {
      source: "task",
      format: "DOWNLOAD",
      status: "completed",
      operation: {
        operationId: "operation-info",
        libraryId: "catalog-one",
        name: "Finished Download",
        kind: "download",
        status: "completed",
        correlation: {},
        domain: "example.com",
        platform: "example",
        uploader: "Uploader",
        request: { url: "https://example.com/watch/1" },
        metrics: { fileCount: 2, totalSizeBytes: 4096 },
        createdAt: "2026-01-01T10:00:00Z",
        startedAt: "2026-01-01T10:01:00Z",
        finishedAt: "2026-01-01T10:02:00Z",
      },
    });
    const markup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={task} initialTab="info" />
      </QueryClientProvider>,
    );

    expect(markup).toContain("<dt>Type</dt>");
    expect(markup).toContain("<dt>Download URL</dt>");
    expect(markup).toContain("https://example.com/watch/1");
    expect(markup).toContain("app-library-preview__copy-value");
    expect(markup).toContain('aria-label="Copy Download URL"');
    expect(markup).not.toContain("<dt>Format</dt>");
    expect(markup).not.toContain("DOWNLOAD");
    expect(markup).toContain(
      'data-library-preview-section="info"><dl class="app-library-preview__info app-dialog-list-card app-dialog-list-card-content"',
    );
    expect(markup).not.toContain("app-library-preview__info-panel");
    expect(markup).not.toContain("app-library-preview__delete-button");
    expect(markup).not.toContain("Delete</button>");

    const previewMarkup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={task} />
      </QueryClientProvider>,
    );
    const fileList = previewMarkup.indexOf("app-library-preview__task-files");
    const deleteAction = previewMarkup.indexOf("app-library-preview__delete-button");
    expect(deleteAction).toBeGreaterThan(fileList);
    expect(previewMarkup).toContain("Delete</button>");
  });

  test("renders a second-step task delete choice for cascading output files", () => {
    const markup = renderToStaticMarkup(
      <LibraryDeleteConfirmationActions
        cascadeFiles={false}
        cascadeFilesLabel="Also delete file"
        labels={createLibraryWorkspaceLabels((key) => t(key, "en"), "en")}
        onCancel={() => {}}
        onCascadeFilesChange={() => {}}
        onConfirm={() => {}}
        pending={false}
        showCascadeFiles
      />,
    );

    expect(markup).toContain('class="app-library-preview__cascade-files"');
    expect(markup).toContain('type="checkbox"');
    expect(markup).toContain("Also delete file");
    expect(markup).toContain("Cancel</button>");
    expect(markup).toContain("Delete</button>");
  });

  test("places a confirmed delete action after ordinary file classification facts", () => {
    const client = new QueryClient();
    const selected = item("Photo", "image", {
      source: "file",
      format: "PNG",
      path: "/Library/photo.png",
      file: {
        id: "photo-file",
        libraryId: "catalog-one",
        kind: "image",
        name: "photo.png",
        fileName: "photo.png",
        storage: { mode: "local_path", localPath: "/Library/photo.png" },
        origin: { kind: "download", operationId: "photo-operation" },
        lineage: {},
        metadata: {},
        media: { format: "png", sizeBytes: 2048 },
        state: { status: "available", deleted: false, archived: false },
        createdAt: "2026-01-01T10:00:00Z",
        updatedAt: "2026-02-01T10:00:00Z",
      },
    });
    const markup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={selected} httpBaseURL="http://127.0.0.1:43127" />
      </QueryClientProvider>,
    );

    const statusFact = markup.indexOf("<dt>Status</dt>");
    const deleteAction = markup.indexOf("app-library-preview__delete-button");
    expect(markup).toContain('data-media-kind="image"');
    expect(markup).toContain("<dt>Category</dt>");
    expect(statusFact).toBeGreaterThan(0);
    expect(deleteAction).toBeGreaterThan(statusFact);
    expect(markup).not.toContain("<dt>Format</dt>");
    expect(markup).toContain("Delete</button>");

    const infoMarkup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion
          item={selected}
          httpBaseURL="http://127.0.0.1:43127"
          initialTab="info"
        />
      </QueryClientProvider>,
    );
    expect(infoMarkup).toContain(
      'data-library-preview-section="info"><dl class="app-library-preview__info app-dialog-list-card app-dialog-list-card-content"',
    );
    expect(infoMarkup).not.toContain("app-library-preview__info-panel");
    expect(infoMarkup).not.toContain("app-library-preview__delete-button");
    expect(infoMarkup).toContain("app-library-preview__location-value");
    expect(infoMarkup).toContain('aria-label="Open Directory"');
    expect(infoMarkup).not.toContain("app-library-preview__copy-value");
  });

  test("shows the legacy item timeline on Activity instead of an empty state", () => {
    const selected = item("Archive Book", "book");
    const markup = renderToStaticMarkup(
      <LibraryPreviewCompanion item={selected} initialTab="activity" />,
    );

    expect(markup).toContain('data-preview-tab="activity"');
    expect(markup).toContain("app-library-preview__timeline");
    expect(markup).toContain("Created");
    expect(markup).toContain("Updated");
    expect(markup).not.toContain("No activity yet");
  });

  test("renders exactly one selected item with four focused tabs", () => {
    const selected = item("Selected Book", "book");
    const markup = renderToStaticMarkup(
      <LibraryPreviewCompanion item={selected} items={[selected]} />,
    );

    expect(markup).toContain('data-library-preview="Selected Book"');
    expect(markup).toContain("Selected Book");
    expect(markup).toContain('aria-label="Preview"');
    expect(markup).toContain('aria-label="Info"');
    expect(markup).toContain('aria-label="Versions"');
    expect(markup).toContain('aria-label="Activity"');
    expect(markup).not.toContain('aria-label="Tags"');
    expect(markup).not.toContain('aria-label="Relations"');
    expect(markup).not.toContain('aria-label="Metadata"');
    expect(markup).toContain("app-dream-segment-switch");
    expect(markup).toContain('data-count="4"');
    expect(markup).toContain("app-dream-segment-switch-indicator");
    expect(markup.match(/aria-selected="true"/g)).toHaveLength(1);
    expect(markup.match(/tabindex="0"/g)).toHaveLength(1);
    expect(markup.match(/tabindex="-1"/g)).toHaveLength(3);
    expect(markup.match(/data-preview-tab=/g)).toHaveLength(1);
    expect(markup).toContain(
      'data-companion-scroll-owner="library-preview"',
    );
    expect(markup).not.toContain("app-library-preview__header");
    expect(markup).toMatch(
      /app-library-preview__body[\s\S]*app-library-preview__footer[\s\S]*app-library-preview__tabs/,
    );
  });

  test("moves preview navigation into the shell footer when externally composed", async () => {
    const selected = item("Selected Book", "book");
    const client = new QueryClient();
    const contentMarkup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion
          item={selected}
          activeTab="info"
          onActiveTabChange={() => {}}
          tabsPlacement="external"
        />
      </QueryClientProvider>,
    );
    const footerMarkup = renderToStaticMarkup(
      <LibraryPreviewCompanionFooter
        item={selected}
        activeTab="info"
        onActiveTabChange={() => {}}
      />,
    );
    const [mainSource, workspaceCSS] = await Promise.all([
      Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/workspace.css", import.meta.url),
      ).text(),
    ]);

    expect(contentMarkup).toContain('data-preview-tab="info"');
    expect(contentMarkup).not.toContain("app-library-preview__tabs");
    expect(footerMarkup).toContain("app-library-preview__tabs");
    expect(footerMarkup).toContain('data-active="true"');
    for (const tab of ["preview", "info", "versions", "activity"]) {
      expect(footerMarkup).toContain(
        `aria-controls="library-preview-panel-Selected%20Book-${tab}"`,
      );
      expect(contentMarkup).toContain(
        `id="library-preview-panel-Selected%20Book-${tab}"`,
      );
    }
    expect(footerMarkup).toContain('id="library-preview-tab-Selected%20Book-info"');
    expect(contentMarkup).toContain('id="library-preview-panel-Selected%20Book-info"');
    expect(contentMarkup).toContain('aria-labelledby="library-preview-tab-Selected%20Book-info"');
    expect(mainSource).toContain("<LibraryPreviewCompanionFooter");
    expect(mainSource).toContain('tabsPlacement="external"');
    expect(workspaceCSS).toMatch(
      /\.app-main-shell\[data-surface-style="glass"\][\s\S]*?\.app-workspace-companion\[data-glass-host="true"\],[\s\S]*?\.app-workspace-companion\[data-presentation="overlay"\]\[data-glass-host="true"\]\s*\{[^}]*background:\s*transparent/s,
    );
  });

  test("falls back to Preview when a stale removed tab reaches the companion at runtime", () => {
    const selected = item("Selected Book", "book");
    const markup = renderToStaticMarkup(
      <LibraryPreviewCompanion
        item={selected}
        activeTab={"metadata" as never}
        onActiveTabChange={() => {}}
      />,
    );

    expect(markup).toContain('data-preview-tab="preview"');
    expect(markup).toContain('aria-label="Preview"');
    expect(markup).not.toContain('data-library-preview-section="metadata"');
  });

  test("keeps Dream segment keyboard behavior enabled for every tablist", async () => {
    const [previewSource, switchSource, rovingTabsSource] = await Promise.all([
      Bun.file(new URL("./LibraryPreviewCompanion.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../shared/ui/dream-segment-switch.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../shared/ui/roving-tabs.ts", import.meta.url)).text(),
    ]);

    expect(previewSource).toContain("tooltips={false}");
    expect(switchSource).toContain("useRovingTabs");
    expect(rovingTabsSource).toContain("resolveRovingTabDestination");
    expect(switchSource).toContain("tabIndex={index === tabs.focusableIndex ? 0 : -1}");
    expect(switchSource).toContain("onKeyDown={(event) => tabs.onKeyDown(event, index)}");
  });

  test("renders an explicit empty companion state", () => {
    const markup = renderToStaticMarkup(<LibraryPreviewCompanion item={null} />);
    expect(markup).toContain('data-library-preview="empty"');
    expect(markup).toContain("Select an item");
  });

  test("opens professional data management from the shared toolbar", async () => {
    const markup = renderToStaticMarkup(<LibraryWorkspacePage route="all" items={[]} />);
    const [pageSource, dialogSource] = await Promise.all([
      Bun.file(new URL("./LibraryWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./CatalogManagementDialog.tsx", import.meta.url)).text(),
    ]);

    expect(markup).toContain('aria-label="More"');
    expect(pageSource).toContain("{labels.health}");
    expect(pageSource).toContain("{labels.manage}");
    expect(pageSource).toContain('setManagementSection("summary")');
    expect(pageSource).toContain('setManagementSection("data")');
    expect(dialogSource).toContain('t("xiadown.libraryData.managementTab")');
    expect(dialogSource).toContain("label: dataSectionLabel");
    expect(dialogSource).toContain("formatBytes(overview.data.totalSizeBytes)");
    expect(dialogSource).toContain("labels.librarySize");
    expect(dialogSource).not.toContain("useCatalogMigrationAudit");
    expect(dialogSource).not.toContain('value: "audit"');
    expect(dialogSource).toContain("? labels.library");
    expect(dialogSource).toContain("labels.statusLabel(overview.data.catalog.status)");
    expect(dialogSource).toContain("categoryLabel={labels.operationKindLabel}");
    expect(dialogSource).toContain("queryClient.invalidateQueries({ queryKey: catalogKeys.all })");
    expect(dialogSource).toContain('queryClient.invalidateQueries({ queryKey: ["library"] })');
    expect(dialogSource).toMatch(
      /className=\{cn\(\s*"app-catalog-management",\s*section === "data"\s*&&\s*"app-catalog-management[^"\n]+"\s*,?\s*\)\}/s,
    );
    expect(dialogSource).toContain("<DialogScrollArea");
    expect(dialogSource).toContain("</DialogScrollArea>");
    expect(dialogSource).toContain("<DialogFooter");
    expect(dialogSource.indexOf("<DialogFooter")).toBeGreaterThan(
      dialogSource.indexOf("</DialogScrollArea>"),
    );
    expect(dialogSource).toMatch(
      /<LibraryDataManagement(?=[^>]*\bembedded\b)(?=[^>]*labels=\{dataLabels\})[^>]*\/>/s,
    );
  });
});
