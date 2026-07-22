import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import type { CatalogItem } from "@/shared/contracts/catalog";
import type {
  DeletedLibraryItemDTO,
  LibraryFileDTO,
  LibraryHistoryRecordDTO,
} from "@/shared/contracts/library";

const callByName = mock((_: string, __?: unknown): Promise<unknown> => Promise.resolve(undefined));

mock.module("@wailsio/runtime", () => ({
  Call: { ByID: () => Promise.resolve(undefined), ByName: callByName },
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
  LibraryDeletedCompanionView,
  libraryDeletedCompanionItemKey,
  mergeDeletedLibraryItems,
} = await import("./LibraryDeletedCompanion");
const { LibraryWorkspacePage } = await import("./LibraryWorkspacePage");
const { listCompleteDeletedLibraryItems } = await import("@/shared/query/library");

function deletedFile(id = "same-id"): DeletedLibraryItemDTO {
  const file: LibraryFileDTO = {
    id,
    libraryId: "library-1",
    kind: "image",
    name: "old-photo.png",
    storage: { mode: "local_path", localPath: "/Volumes/Archive/a/very/long/path/old-photo.png" },
    origin: { kind: "download" },
    lineage: {},
    metadata: { title: "Old Photo" },
    media: { format: "PNG", sizeBytes: 2048 },
    state: { status: "deleted", deleted: true, archived: false },
    createdAt: "2026-06-01T08:00:00Z",
    updatedAt: "2026-07-19T10:00:00Z",
  };
  return {
    id,
    kind: "file",
    source: "legacy_file",
    libraryId: "library-1",
    title: "Old Photo",
    category: "image",
    status: "deleted",
    deletedAt: "2026-07-19T10:00:00Z",
    canRestore: true,
    detail: { file },
  };
}

function deletedTask(id = "same-id"): DeletedLibraryItemDTO {
  const taskHistory: LibraryHistoryRecordDTO = {
    recordId: "event-delete-1",
    libraryId: "library-1",
    category: "operation_event",
    action: "operation_deleted",
    displayName: "A deliberately long deleted task title that must wrap inside Companion",
    status: "deleted",
    source: { kind: "desktop", actor: "desktop-library" },
    refs: { operationId: id, fileIds: ["file-1", "file-2"] },
    metrics: { fileCount: 2 },
    occurredAt: "2026-07-20T10:00:00Z",
    createdAt: "2026-07-20T10:00:00Z",
  };
  return {
    id,
    kind: "task",
    source: "operation_history",
    libraryId: "library-1",
    title: taskHistory.displayName,
    category: "download",
    status: "deleted",
    deletedAt: taskHistory.occurredAt,
    canRestore: false,
    detail: { taskHistory },
  };
}

function catalogItem(overrides: Partial<CatalogItem> = {}): CatalogItem {
  return {
    id: "catalog-1",
    catalogId: "catalog-default",
    category: "image",
    status: "trashed",
    title: "Catalog Photo",
    sortTitle: "Catalog Photo",
    revision: 7,
    trashedAt: "2026-07-18T10:00:00Z",
    createdAt: "2026-06-01T10:00:00Z",
    updatedAt: "2026-07-18T10:00:00Z",
    ...overrides,
  };
}

describe("Library Deleted companion", () => {
  test("uses kind-qualified identities and keeps same-id task and file independently selectable", () => {
    expect(libraryDeletedCompanionItemKey(deletedTask())).toBe("task:same-id");
    expect(libraryDeletedCompanionItemKey(deletedFile())).toBe("file:same-id");

    const items = mergeDeletedLibraryItems([deletedTask(), deletedFile()], []);
    expect(items.map(libraryDeletedCompanionItemKey).sort()).toEqual([
      "file:same-id",
      "task:same-id",
    ]);
  });

  test("deduplicates the Catalog projection of a deleted legacy file", () => {
    const items = mergeDeletedLibraryItems(
      [deletedFile("file-1")],
      [
        catalogItem({ id: "duplicate", primaryFileId: "file-1" }),
        catalogItem({ id: "catalog-only", primaryFileId: "file-2", title: "Catalog only" }),
      ],
    );

    expect(items.map(libraryDeletedCompanionItemKey)).toEqual([
      "file:file-1",
      "catalog_item:catalog-only",
    ]);

    expect(mergeDeletedLibraryItems(
      [],
      [catalogItem({ id: "purged-projection", primaryFileId: "  purged-file  " })],
      new Set(["purged-file"]),
    )).toEqual([]);
  });

  test("fails closed when any archive source fails instead of exposing partial actions", () => {
    const markup = renderToStaticMarkup(
      <LibraryDeletedCompanionView
        items={mergeDeletedLibraryItems([deletedFile("partial-file")], [])}
        total={1}
        loadError
        initialSelectedKey="file:partial-file"
        labels={{ loadFailed: "LOAD_FAILED", retry: "RETRY" }}
      />,
    );

    expect(markup).toContain('role="alert"');
    expect(markup).toContain("LOAD_FAILED");
    expect(markup).toContain("RETRY");
    expect(markup).not.toContain("Restore item");
    expect(markup).not.toContain("Clean up permanently");
  });

  test("renders task audit details without presenting an invalid restore action", () => {
    const task = deletedTask();
    const markup = renderToStaticMarkup(
      <LibraryDeletedCompanionView
        items={mergeDeletedLibraryItems([task], [])}
        total={1}
        initialSelectedKey="task:same-id"
      />,
    );

    expect(markup).toContain('data-view="detail"');
    expect(markup).toContain("Audit event");
    expect(markup).not.toContain("operation_deleted");
    expect(markup).toContain("A deliberately long deleted task title");
    expect(markup).toContain("This item cannot be restored");
    expect(markup).not.toContain("Clean up permanently");
  });

  test("renders a unified list for tasks, files, and Catalog trash", () => {
    const items = mergeDeletedLibraryItems(
      [deletedTask("task-1"), deletedFile("file-1")],
      [catalogItem()],
    );
    const markup = renderToStaticMarkup(
      <LibraryDeletedCompanionView items={items} total={items.length} />,
    );

    expect(markup).toContain('data-companion-scroll-owner="library-deleted"');
    expect(markup).toContain('data-kind="task"');
    expect(markup).toContain('data-kind="file"');
    expect(markup).toContain('data-kind="catalog_item"');
    expect(markup).toContain("Showing 3 of 3");
  });

  test("keeps all Deleted geometry inside the fixed-width Companion contract", async () => {
    const dream = await Bun.file(
      new URL("../../shared/styles/dream/library.css", import.meta.url),
    ).text();
    expect(dream).toMatch(
      /\.app-library-deleted\s*\{[^}]*width:\s*100%[^}]*min-width:\s*0[^}]*overflow-x:\s*clip/s,
    );
    expect(dream).toMatch(
      /\.app-library-deleted__item\s*\{[^}]*grid-template-columns:\s*2\.55rem minmax\(0, 1fr\) 0\.9rem/s,
    );
    expect(dream).toMatch(
      /\.app-library-deleted__fact dd\s*\{[^}]*overflow-wrap:\s*anywhere/s,
    );
    expect(dream).toMatch(
      /\.app-library-deleted__detail-actions\s*\{[^}]*flex-wrap:\s*wrap/s,
    );
  });

  test("keeps the Deleted title action inside the Library sidebar group", () => {
    const searchMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage
        route="search"
        items={[]}
        query=""
        onOpenDeletedItems={() => {}}
      />,
    );
    const libraryMarkup = renderToStaticMarkup(
      <LibraryWorkspacePage
        route="all"
        items={[]}
        onOpenDeletedItems={() => {}}
      />,
    );

    expect(searchMarkup).not.toContain("app-library-deleted-items-action");
    expect(libraryMarkup).toContain("app-library-deleted-items-action");
    expect(libraryMarkup).toContain('aria-label="Deleted items"');
    expect(libraryMarkup).not.toMatch(/app-library-deleted-items-action[^>]*disabled/);
  });

  test("opens Deleted as a Library workspace companion instead of a route-bound selection", async () => {
    const [mainSource, runningSource] = await Promise.all([
      Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
      Bun.file(new URL("../main/RunningPage.tsx", import.meta.url)).text(),
    ]);
    expect(mainSource).toContain('id: "library-deleted"');
    expect(mainSource).toMatch(
      /id: "library-deleted",[\s\S]*?kind: "workspace",[\s\S]*?workspaceId: APP_WORKSPACE_IDS\.library/,
    );
    expect(mainSource).toContain("onOpenDeletedItems={openLibraryDeletedItems}");
    expect(mainSource).toContain('<LibraryDeletedCompanion />');

    const runningBranch = mainSource.slice(
      mainSource.indexOf("<RunningPage"),
      mainSource.indexOf("/>", mainSource.indexOf("<RunningPage")) + 2,
    );
    expect(runningBranch).not.toContain("deletedItemsLabel");
    expect(runningBranch).not.toContain("onOpenDeletedItems");
    expect(runningSource).not.toContain("app-library-deleted-items-action");
    expect(runningSource).not.toContain("onOpenDeletedItems");
  });

  test("collects every backend Deleted page instead of truncating the companion", async () => {
    const requests: Array<{ limit?: number; offset?: number }> = [];
    callByName.mockImplementation(async (name, request) => {
      expect(name).toEndWith("LibraryHandler.ListDeletedLibraryItems");
      const typed = (request ?? {}) as { limit?: number; offset?: number };
      requests.push(typed);
      const item = typed.offset === 0
        ? deletedTask("page-task")
        : deletedFile("page-file");
      return {
        items: [item],
        total: 2,
        limit: typed.limit ?? 500,
        offset: typed.offset ?? 0,
      };
    });
    try {
      const response = await listCompleteDeletedLibraryItems();
      expect(response.items.map(libraryDeletedCompanionItemKey)).toEqual([
        "task:page-task",
        "file:page-file",
      ]);
      expect(requests.map((request) => request.offset)).toEqual([0, 1]);
      expect(requests.every((request) => request.limit === 500)).toBeTrue();
    } finally {
      callByName.mockImplementation(() => Promise.resolve(undefined));
    }
  });
});
