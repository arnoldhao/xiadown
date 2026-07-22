import { describe, expect, test } from "bun:test";

import type { CatalogItem } from "@/shared/contracts/catalog";
import type { LibraryFileDTO } from "@/shared/contracts/library";
import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";

import {
  adaptCatalogItems,
  buildCatalogVideoThumbnailURL,
  isBrowserImagePreviewPath,
} from "./catalog-adapter";

const baseItem: CatalogItem = {
  id: "item-1",
  catalogId: "catalog-1",
  category: "video",
  status: "active",
  title: "A film",
  sortTitle: "A film",
  revision: 1,
  createdAt: "2026-07-13T00:00:00Z",
  updatedAt: "2026-07-13T00:00:00Z",
};

function file(id: string, name: string, kind: string): LibraryFileDTO {
  return {
    id,
    libraryId: "legacy-1",
    kind,
    name,
    storage: { mode: "local_path", localPath: `/Library/${name}` },
    origin: { kind: "download" },
    lineage: {},
    metadata: {},
    state: { status: "available", deleted: false, archived: false },
    createdAt: "2026-07-13T00:00:00Z",
    updatedAt: "2026-07-13T01:00:00Z",
  };
}

describe("Catalog list artwork", () => {
  test("uses the opaque lazy thumbnail endpoint for a video without artwork", () => {
    const [adapted] = adaptCatalogItems(
      [{ ...baseItem, id: "video one", primaryFileId: "video-file" }],
      { httpBaseURL: "http://127.0.0.1:43127/_xiadown/token/" },
    );

    expect(adapted?.coverURL).toBe(
      "http://127.0.0.1:43127/_xiadown/token/api/library/video-thumbnail/video%20one?v=2026-07-13T00%3A00%3A00Z",
    );
  });

  test("waits for a declared artwork file instead of decoding the video during query startup", () => {
    const [adapted] = adaptCatalogItems(
      [{
        ...baseItem,
        primaryFileId: "video-file",
        artworkFileId: "cover-file",
      }],
      {
        filesById: new Map(),
        httpBaseURL: "http://127.0.0.1:43127/_xiadown/token",
      },
    );

    expect(adapted?.coverURL).toBe(COMPLETED_DEFAULT_COVER_IMAGE_URLS.video);
    expect(adapted?.coverURL).not.toContain("/api/library/video-thumbnail/");
  });

  test("does not offer video generation for unavailable or non-video items", () => {
    for (const item of [
      { ...baseItem, status: "missing" as const, primaryFileId: "video-file" },
      { ...baseItem, category: "audio" as const, primaryFileId: "audio-file" },
    ]) {
      const [adapted] = adaptCatalogItems([item], {
        httpBaseURL: "http://127.0.0.1:43127/_xiadown/token",
      });
      expect(adapted?.coverURL).toBe(
        item.category === "audio"
          ? COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio
          : COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
      );
    }
    expect(buildCatalogVideoThumbnailURL("", "item-1")).toBe("");
    expect(buildCatalogVideoThumbnailURL("http://127.0.0.1", " ")).toBe("");
  });

  test("uses an artwork asset for video, audio and books without matching titles", () => {
    const artwork = file("cover-file", "cover.webp", "thumbnail");
    const filesById = new Map([[artwork.id, artwork]]);

    for (const category of ["video", "audio", "book"] as const) {
      const [adapted] = adaptCatalogItems(
        [{ ...baseItem, category, artworkFileId: artwork.id }],
        { filesById, httpBaseURL: "http://127.0.0.1:43127" },
      );
      expect(adapted?.coverURL).toContain("/api/library/asset/cover.webp?");
      expect(adapted?.coverURL).toContain("path=%2FLibrary%2Fcover.webp");
    }
  });

  test("uses an image original only when no bounded artwork exists", () => {
    const original = file("image-file", "portrait.avif", "image");
    const filesById = new Map([[original.id, original]]);
    const [adapted] = adaptCatalogItems(
      [{ ...baseItem, category: "image", primaryFileId: original.id }],
      { filesById, httpBaseURL: "http://127.0.0.1:43127" },
    );

    expect(adapted?.coverURL).toContain("portrait.avif");
    expect(adapted?.coverURL).toContain("path=%2FLibrary%2Fportrait.avif");
  });

  test("does not load deleted or unavailable originals as a grid preview", () => {
    const deleted = {
      ...file("image-file", "private.png", "image"),
      state: { status: "deleted", deleted: true, archived: false },
    } satisfies LibraryFileDTO;
    const [adapted] = adaptCatalogItems(
      [{ ...baseItem, category: "image", primaryFileId: deleted.id }],
      {
        filesById: new Map([[deleted.id, deleted]]),
        httpBaseURL: "http://127.0.0.1:43127",
      },
    );

    expect(adapted?.coverURL).toBe(COMPLETED_DEFAULT_COVER_IMAGE_URLS.image);
  });

  test("keeps cross-platform unsupported image originals behind the fallback", () => {
    const original = file("image-file", "camera.heic", "image");
    const [adapted] = adaptCatalogItems(
      [{ ...baseItem, category: "image", primaryFileId: original.id }],
      {
        filesById: new Map([[original.id, original]]),
        httpBaseURL: "http://127.0.0.1:43127",
      },
    );

    expect(adapted?.coverURL).toBe(COMPLETED_DEFAULT_COVER_IMAGE_URLS.image);
  });

  test("applies the same cross-platform decoder policy to artwork and companion covers", () => {
    const artwork = file("cover-file", "cover.heic", "thumbnail");
    const [adapted] = adaptCatalogItems(
      [{ ...baseItem, artworkFileId: artwork.id }],
      {
        filesById: new Map([[artwork.id, artwork]]),
        httpBaseURL: "http://127.0.0.1:43127",
      },
    );

    expect(adapted?.coverURL).toBe(COMPLETED_DEFAULT_COVER_IMAGE_URLS.video);
    expect(isBrowserImagePreviewPath("cover.webp")).toBe(true);
    expect(isBrowserImagePreviewPath("cover.heic")).toBe(false);
    expect(isBrowserImagePreviewPath("page.tiff")).toBe(false);
    expect(isBrowserImagePreviewPath("camera.raw")).toBe(false);
  });

  test("falls back when Catalog records a physical-file error", () => {
    const artwork = {
      ...file("cover-file", "cover.webp", "thumbnail"),
      state: {
        status: "available",
        deleted: false,
        archived: false,
        lastError: "checksum mismatch",
      },
    } satisfies LibraryFileDTO;
    const [adapted] = adaptCatalogItems(
      [{ ...baseItem, artworkFileId: artwork.id }],
      {
        filesById: new Map([[artwork.id, artwork]]),
        httpBaseURL: "http://127.0.0.1:43127",
      },
    );

    expect(adapted?.coverURL).toBe(COMPLETED_DEFAULT_COVER_IMAGE_URLS.video);
  });
});
