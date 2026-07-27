import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { getXiaText } from "@/features/xiadown/shared";

import {
  LIBRARY_WORKSPACE_ROUTE_IDS,
  LibraryWorkspaceSidebar,
  type LibraryWorkspaceSidebarCatalog,
} from "./LibraryWorkspaceSidebar";

const routeLabels: Record<keyof typeof LIBRARY_WORKSPACE_ROUTE_IDS, string> = {
  search: "Search",
  running: "Running",
  ended: "Ended",
  appSessions: "App Sessions",
  all: "All",
  video: "Video",
  audio: "Audio",
  books: "Books",
  images: "Images",
  others: "Others",
  petGallery: "Pet Gallery",
};

const catalog: LibraryWorkspaceSidebarCatalog = {
  sidebarAriaLabel: "Library navigation",
  sections: {
    library: { label: "Library" },
    more: { label: "More" },
  },
  routes: Object.fromEntries(
    Object.entries(routeLabels).map(([route, label]) => [route, { label }]),
  ) as LibraryWorkspaceSidebarCatalog["routes"],
};

describe("LibraryWorkspaceSidebar", () => {
  test("renders the product route order in the shared wide-sidebar chrome", () => {
    const markup = renderToStaticMarkup(
      <LibraryWorkspaceSidebar
        activeRouteId="all"
        catalog={catalog}
        onNavigate={() => undefined}
      />,
    );

    expect(markup).toContain("app-library-workspace-sidebar");
    expect(markup).toContain('aria-label="Library navigation"');
    expect(markup).toContain('aria-current="page"');

    const labels = [
      "Search",
      "Running",
      "Ended",
      "All",
      "Video",
      "Audio",
      "Books",
      "Images",
      "Others",
      "App Sessions",
      "Pet Gallery",
    ];
    labels.reduce((previousIndex, label) => {
      const index = markup.indexOf(label);
      expect(index).toBeGreaterThan(previousIndex);
      return index;
    }, -1);
    expect(markup.indexOf('data-section="library"')).toBeGreaterThan(
      markup.indexOf('data-section="primary"'),
    );
    expect(markup.indexOf('data-section="more"')).toBeGreaterThan(
      markup.indexOf('data-section="library"'),
    );
  });

  test("does not expose Open Library as a sidebar route", () => {
    expect("openLibrary" in LIBRARY_WORKSPACE_ROUTE_IDS).toBe(false);
    expect(LIBRARY_WORKSPACE_ROUTE_IDS.all).toBe("all");
  });

  test("keeps the English product route labels exact", () => {
    const text = getXiaText("en");
    expect([
      text.workspace.libraryStation,
      text.workspace.more,
      text.workspace.all,
      text.views.ended,
      text.workspace.video,
      text.workspace.audio,
      text.workspace.books,
      text.workspace.images,
      text.workspace.others,
    ]).toEqual([
      "Library",
      "More",
      "All",
      "Ended",
      "Video",
      "Audio",
      "Books",
      "Images",
      "Others",
    ]);
  });
});
