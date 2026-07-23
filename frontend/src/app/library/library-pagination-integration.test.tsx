import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";
import type { LibraryWorkspaceItem } from "./types";

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

mock.module("@/shared/ui/tooltip", () => ({
  TOOLTIP_ALIGNS: ["start", "center", "end"],
  TOOLTIP_SIDES: ["top", "bottom", "left", "right"],
  Tooltip: ({ children }: { children?: unknown }) => children,
  TooltipContent: () => null,
  TooltipProvider: ({ children }: { children?: unknown }) => children,
  TooltipTrigger: ({ children }: { children?: unknown }) => children,
}));

const {
  LibraryWorkspacePage,
  resolveControlledLibrarySearchExpanded,
} = await import("./LibraryWorkspacePage");

function libraryItem(
  index: number,
  overrides: Partial<LibraryWorkspaceItem> = {},
): LibraryWorkspaceItem {
  const title = `Video ${String(index).padStart(2, "0")}`;
  return {
    id: `video-${index}`,
    source: "file",
    libraryId: "library",
    libraryName: "Library",
    title,
    subtitle: "Video",
    category: "video",
    status: "active",
    format: "mp4",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: `2026-01-${String(Math.min(index, 28)).padStart(2, "0")}T00:00:00Z`,
    path: "",
    coverURL: COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
    rootId: `video-${index}`,
    searchText: title.toLocaleLowerCase(),
    ...overrides,
  };
}

describe("LibraryWorkspacePage pagination integration", () => {
  test("paginates a complete local result set instead of rendering every card", () => {
    const items = Array.from({ length: 50 }, (_, index) => libraryItem(index + 1));
    const markup = renderToStaticMarkup(
      <LibraryWorkspacePage route="video" items={items} />,
    );

    expect(markup.match(/data-item-id=/g)?.length).toBe(48);
    expect(markup).toContain("1–48 · 50 items");
    expect(markup).toContain("Page 1/2");
  });

  test("preserves the items and order selected by an already paged backend response", () => {
    const items = [
      libraryItem(50, {
        title: "Zulu audio",
        category: "audio",
        searchText: "zulu audio",
      }),
      libraryItem(49, {
        title: "Alpha video",
        searchText: "alpha video",
      }),
    ];
    const markup = renderToStaticMarkup(
      <LibraryWorkspacePage
        route="video"
        items={items}
        query="not present"
        sort="name"
        pagination={{
          page: 2,
          pageSize: 48,
          total: 100,
          itemsArePage: true,
          onPageChange: () => {},
          onPageSizeChange: () => {},
        }}
      />,
    );

    expect(markup.match(/data-item-id=/g)?.length).toBe(2);
    expect(markup.indexOf('data-item-id="video-50"')).toBeLessThan(
      markup.indexOf('data-item-id="video-49"'),
    );
    expect(markup).toContain("49–96 · 100 items");
    expect(markup).toContain('aria-current="page"');
  });

  test("derives controlled search expansion from both route and query updates", () => {
    expect(resolveControlledLibrarySearchExpanded("search", "")).toBe(true);
    expect(resolveControlledLibrarySearchExpanded("all", "library item")).toBe(true);
    expect(resolveControlledLibrarySearchExpanded("all", "")).toBe(false);
  });
});
