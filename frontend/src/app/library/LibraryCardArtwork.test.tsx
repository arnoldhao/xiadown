import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { LibraryCardArtwork } from "./LibraryCardArtwork";
import type { LibraryWorkspaceItem } from "./types";

function previewItem(kind: "pdf" | "log"): LibraryWorkspaceItem {
  return {
    id: `item-${kind}`,
    source: "file",
    libraryId: "catalog-1",
    libraryName: "",
    title: kind === "pdf" ? "Guide.pdf" : "download.log",
    subtitle: kind,
    category: kind === "pdf" ? "book" : "other",
    otherGroup: kind === "log" ? "document" : undefined,
    status: "active",
    format: kind,
    createdAt: "2026-07-26T00:00:00Z",
    updatedAt: "2026-07-26T01:00:00Z",
    path: "",
    coverURL: "",
    rootId: `item-${kind}`,
    searchText: kind,
    cardPreview: {
      kind,
      sourceURL: `http://127.0.0.1/card-preview/${kind}/item-${kind}`,
      cacheKey: `item-${kind}:v1`,
    },
  };
}

describe("LibraryCardArtwork", () => {
  test("keeps a semantic fallback without duplicating the format label", () => {
    const pdf = renderToStaticMarkup(
      <LibraryCardArtwork item={previewItem("pdf")} />,
    );
    const log = renderToStaticMarkup(
      <LibraryCardArtwork item={previewItem("log")} />,
    );

    expect(pdf).toContain('data-preview-kind="pdf"');
    expect(pdf).not.toContain("app-library-card-preview__badge");
    expect(log).toContain('data-preview-kind="log"');
    expect(log).not.toContain("app-library-card-preview__badge");
    expect(log).toContain('data-artwork-kind="document"');
  });

  test("contains bounded viewport, concurrency and persistent-cache controls", async () => {
    const source = await Bun.file(
      new URL("./LibraryCardArtwork.tsx", import.meta.url),
    ).text();

    expect(source).toContain("new IntersectionObserver");
    expect(source).toContain("rootMargin: CARD_PREVIEW_ROOT_MARGIN");
    expect(source).toContain("activeLogRequests < 2");
    expect(source).toContain("pdfQueueTail");
    expect(source).toContain("PDF_THUMBNAIL_DISK_LIMIT = 96");
    expect(source).toContain("disableAutoFetch: true");
    expect(source).toContain("rangeChunkSize: 64 * 1_024");
  });
});
