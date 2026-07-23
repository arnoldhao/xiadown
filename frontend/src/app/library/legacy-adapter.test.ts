import { describe, expect, test } from "bun:test";

import type {
  LibraryDTO,
  LibraryFileDTO,
  OperationListItemDTO,
} from "@/shared/contracts/library";

import {
  adaptAvailableLegacyImageFiles,
  adaptLegacyLibraryFiles,
  adaptLegacyLibraryTasks,
  classifyLegacyLibraryFile,
} from "./legacy-adapter";

function file(
  id: string,
  name: string,
  kind = "file",
  overrides: Partial<LibraryFileDTO> = {},
): LibraryFileDTO {
  return {
    id,
    libraryId: "library-one",
    kind,
    name,
    storage: { mode: "local_path", localPath: `/Library/${name}` },
    origin: { kind: "import" },
    lineage: {},
    metadata: {},
    state: { status: "available", deleted: false, archived: false },
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-02-01T00:00:00Z",
    ...overrides,
  };
}

function library(files: LibraryFileDTO[]): LibraryDTO {
  return {
    version: "current",
    id: "library-one",
    name: "My Library",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-02-01T00:00:00Z",
    createdBy: { source: "test" },
    files,
    records: {
      history: [],
      workspaceStates: [],
      fileEvents: [],
    },
  };
}

function operation(
  overrides: Partial<OperationListItemDTO> = {},
): OperationListItemDTO {
  return {
    operationId: "operation-one",
    libraryId: "library-one",
    libraryName: "My Library",
    name: "Download a film",
    kind: "download",
    status: "succeeded",
    correlation: {},
    metrics: { fileCount: 1, totalSizeBytes: 1024 },
    createdAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("legacy library workspace adapter", () => {
  test("classifies the five primary experiences and professional other groups", () => {
    expect(classifyLegacyLibraryFile(file("video", "movie.mkv"))).toEqual({ category: "video" });
    expect(classifyLegacyLibraryFile(file("audio", "album.flac"))).toEqual({ category: "audio" });
    expect(classifyLegacyLibraryFile(file("book", "novel.epub"))).toEqual({ category: "book" });
    expect(classifyLegacyLibraryFile(file("image", "photo.heic"))).toEqual({ category: "image" });
    expect(classifyLegacyLibraryFile(file("font", "display.woff2"))).toEqual({
      category: "other",
      otherGroup: "font",
    });
    expect(classifyLegacyLibraryFile(file("subtitle", "movie.vtt"))).toEqual({
      category: "other",
      otherGroup: "subtitle",
    });
  });

  test("surfaces missing and needs-review states before extension classification", () => {
    expect(classifyLegacyLibraryFile(file("missing", "movie.mp4", "video", {
      state: { status: "missing", deleted: false, archived: false },
    }))).toEqual({ category: "other", otherGroup: "missing" });
    expect(classifyLegacyLibraryFile(file("review", "track.mp3", "audio", {
      state: { status: "needs_review", deleted: false, archived: false },
    }))).toEqual({ category: "other", otherGroup: "needs-review" });
  });

  test("uses related thumbnails while preserving source DTOs and stable IDs", () => {
    const video = file("video", "movie.mp4", "video");
    const thumbnail = file("art", "movie.webp", "thumbnail", {
      lineage: { rootFileId: "video" },
    });
    const items = adaptLegacyLibraryFiles(
      [library([video, thumbnail])],
      "http://127.0.0.1:34115",
    );
    const adaptedVideo = items.find((item) => item.id === "file:video");

    expect(adaptedVideo?.file).toBe(video);
    expect(adaptedVideo?.rootId).toBe("video");
    expect(adaptedVideo?.coverURL).toContain("movie.webp");
    expect(adaptedVideo?.coverURL).toContain("%2FLibrary%2Fmovie.webp");
  });

  test("keeps available artwork outputs visible as image files without duplicating Catalog images", () => {
    const artwork = file("task-artwork", "cover.webp", "thumbnail", {
      origin: { kind: "download", operationId: "operation-one" },
    });
    const standalone = file("catalog-image", "photo.png", "image");
    const deletedCatalogImages = Array.from({ length: 7 }, (_, index) =>
      file(`deleted-catalog-image-${index + 1}`, `old-${index + 1}.jpg`, "thumbnail", {
        state: { status: "deleted", deleted: true, archived: false },
      }));
    const failed = file("failed-artwork", "broken.jpg", "thumbnail", {
      state: {
        status: "available",
        deleted: false,
        archived: false,
        lastError: "decode failed",
      },
    });

    const images = adaptAvailableLegacyImageFiles(
      [library([artwork, standalone, ...deletedCatalogImages, failed])],
      "http://127.0.0.1:34115",
      new Set([standalone.id]),
    );

    expect(images.map((item) => item.id)).toEqual(["file:task-artwork"]);
    expect(images[0]?.coverURL).toContain("cover.webp");
    // Normal Library Catalog queries exclude trashed logical items. The
    // supplemental projection must still contribute the folded active Task
    // image without resurrecting any of those deleted file records.
    expect(images).toHaveLength(1);
  });

  test("hides a soft-deleted intermediate download when an active transcode replaced it", () => {
    const source = file("source-webm", "episode.webm", "video", {
      origin: { kind: "download", operationId: "download-and-transcode" },
      state: { status: "deleted", deleted: true, archived: false },
    });
    const transcode = file("output-mp4", "episode.mp4", "transcode", {
      origin: { kind: "transcode", operationId: "download-and-transcode" },
      lineage: { rootFileId: source.id },
      media: { format: "mp4", videoCodec: "h264" },
    });

    const items = adaptLegacyLibraryFiles([library([source, transcode])]);

    expect(items.map((item) => item.id)).toEqual(["file:output-mp4"]);
    expect(items[0]?.status).toBe("available");
    expect(items[0]?.format).toBe("MP4");
  });

  test("never projects a soft-deleted source into the normal Library", () => {
    const source = file("source-webm", "episode.webm", "video", {
      origin: { kind: "download", operationId: "download-operation" },
      state: { status: "deleted", deleted: true, archived: false },
    });
    const transcode = file("output-mp4", "episode.mp4", "transcode", {
      origin: { kind: "transcode", operationId: "standalone-transcode" },
      lineage: { rootFileId: source.id },
      media: { format: "mp4", videoCodec: "h264" },
    });

    const items = adaptLegacyLibraryFiles([library([source, transcode])]);

    expect(items.map((item) => item.id)).toEqual(["file:output-mp4"]);
  });

  test("does not use a soft-deleted thumbnail as active Library artwork", () => {
    const video = file("video", "movie.mp4", "video");
    const deletedThumbnail = file("deleted-art", "old-cover.webp", "thumbnail", {
      lineage: { rootFileId: video.id },
      state: { status: "deleted", deleted: true, archived: false },
    });

    const [item] = adaptLegacyLibraryFiles(
      [library([video, deletedThumbnail])],
      "http://127.0.0.1:34115",
    );

    expect(item?.id).toBe("file:video");
    expect(item?.coverURL).not.toContain("old-cover.webp");
  });

  test("adapts operations as task items without folding them into files", () => {
    const sourceOperation = operation();
    const sourceLibrary = library([]);
    const [item] = adaptLegacyLibraryTasks([sourceOperation], "", [sourceLibrary]);

    expect(item).toMatchObject({
      id: "task:operation-one",
      category: "task",
      source: "task",
      operation: sourceOperation,
      library: sourceLibrary,
      taskPreviewItems: [],
      taskPreviewTotalCount: 0,
    });
  });

  test("projects a single output as a type page without changing the task cover", () => {
    const video = file("video-output", "film.mp4", "video", {
      origin: { kind: "download", operationId: "operation-one" },
      media: { format: "mp4", videoCodec: "h264" },
    });
    const sourceOperation = operation({
      thumbnailPreviewPath: "/Library/task-cover.webp",
      outputFiles: [{
        fileId: video.id,
        kind: "video",
        format: "mp4",
        isPrimary: true,
      }],
    });
    const [item] = adaptLegacyLibraryTasks(
      [sourceOperation],
      "http://127.0.0.1:34115",
      [library([video])],
    );

    expect(item?.coverURL).toContain("task-cover.webp");
    expect(item?.taskPreviewItems).toEqual([{
      id: "video-output",
      kind: "video",
      label: "MP4",
    }]);
  });

  test("puts unique artwork before stable primary type pages and bounds slots", () => {
    const primary = file("primary-video", "film.mp4", "video");
    const firstArtwork = file("artwork-one", "cover.webp", "thumbnail");
    const alternateArtwork = file("artwork-two", "cover-alt.webp", "thumbnail", {
      storage: { mode: "local_path", localPath: "/Library/cover.webp" },
    });
    const audio = file("audio-output", "soundtrack.flac", "audio");
    const sourceOperation = operation({
      metrics: { fileCount: 4 },
      outputFiles: [
        { fileId: firstArtwork.id, kind: "thumbnail", format: "webp" },
        { fileId: alternateArtwork.id, kind: "thumbnail", format: "webp" },
        { fileId: audio.id, kind: "audio", format: "flac" },
        { fileId: primary.id, kind: "video", format: "mp4", isPrimary: true },
      ],
    });
    const [item] = adaptLegacyLibraryTasks(
      [sourceOperation],
      "http://127.0.0.1:34115",
      [library([primary, firstArtwork, alternateArtwork, audio])],
    );

    expect(item?.taskPreviewItems).toHaveLength(3);
    expect(item?.taskPreviewTotalCount).toBe(4);
    expect(item?.taskPreviewItems?.map((preview) => preview.id)).toEqual([
      "artwork-one",
      "primary-video",
      "artwork-two",
    ]);
    expect(item?.taskPreviewItems?.[0]?.previewURL).toContain("cover.webp");
    expect(item?.taskPreviewItems?.[2]).toEqual({
      id: "artwork-two",
      kind: "thumbnail",
      label: "WEBP",
    });
  });

  test("deduplicates artwork before moving all real image previews ahead of defaults", () => {
    const primaryType = file("primary-type", "film.mp4", "video");
    const firstArtwork = file("artwork-a", "a.webp", "thumbnail");
    const duplicateArtwork = file("artwork-a-duplicate", "a-alt.webp", "thumbnail", {
      storage: { mode: "local_path", localPath: "/Library/a.webp" },
    });
    const secondArtwork = file("artwork-b", "b.webp", "thumbnail");
    const [item] = adaptLegacyLibraryTasks(
      [operation({
        outputFiles: [
          { fileId: primaryType.id, kind: "video", format: "mp4", isPrimary: true },
          { fileId: firstArtwork.id, kind: "thumbnail", format: "webp" },
          { fileId: duplicateArtwork.id, kind: "thumbnail", format: "webp" },
          { fileId: secondArtwork.id, kind: "thumbnail", format: "webp" },
        ],
      })],
      "http://127.0.0.1:34115",
      [library([primaryType, firstArtwork, duplicateArtwork, secondArtwork])],
    );

    expect(item?.taskPreviewItems?.map((preview) => preview.id)).toEqual([
      "artwork-a",
      "artwork-b",
      "primary-type",
    ]);
    expect(item?.taskPreviewItems?.slice(0, 2).every((preview) => preview.previewURL)).toBe(true);
    expect(item?.taskPreviewTotalCount).toBe(4);
  });

  test("filters deleted and unavailable files while retaining a safe missing-record type page", () => {
    const missing = file("missing-output", "gone.mp4", "video", {
      state: { status: "missing", deleted: false, archived: false },
    });
    const failed = file("failed-output", "broken.webp", "thumbnail", {
      state: {
        status: "available",
        deleted: false,
        archived: false,
        lastError: "decode failed",
      },
    });
    const sourceOperation = operation({
      outputFiles: [
        { fileId: "deleted-output", kind: "video", deleted: true },
        { fileId: missing.id, kind: "video", format: "mp4" },
        { fileId: failed.id, kind: "thumbnail", format: "webp" },
        { fileId: "not-yet-indexed", kind: "subtitle", format: "vtt" },
      ],
    });
    const [item] = adaptLegacyLibraryTasks(
      [sourceOperation],
      "http://127.0.0.1:34115",
      [library([missing, failed])],
    );

    expect(item?.taskPreviewItems).toEqual([{
      id: "not-yet-indexed",
      kind: "subtitle",
      label: "VTT",
    }]);
    expect(item?.taskPreviewTotalCount).toBe(1);
    expect(item?.taskFiles?.map((entry) => ({
      id: entry.fileId,
      category: entry.category,
      status: entry.status,
      canView: entry.canView,
    }))).toEqual([
      { id: "deleted-output", category: "video", status: "deleted", canView: false },
      { id: "missing-output", category: "video", status: "missing", canView: false },
      { id: "failed-output", category: "image", status: "error", canView: false },
      { id: "not-yet-indexed", category: "other", status: "missing", canView: false },
    ]);
  });

  test("keeps complete task file metadata separately from the bounded artwork projection", () => {
    const audio = file("audio-output", "soundtrack.flac", "audio", {
      origin: { kind: "download", operationId: "operation-one" },
      media: { format: "flac", audioCodec: "flac" },
    });
    const [item] = adaptLegacyLibraryTasks(
      [operation({ outputFiles: [{ fileId: audio.id, kind: "audio", format: "flac" }] })],
      "http://127.0.0.1:34115",
      [library([audio])],
    );

    expect(item?.taskFiles).toHaveLength(1);
    expect(item?.taskFiles?.[0]).toMatchObject({
      fileId: "audio-output",
      previewItemId: "file:audio-output",
      title: "soundtrack.flac",
      kind: "audio",
      category: "audio",
      status: "available",
      format: "FLAC",
      canView: true,
      file: audio,
    });
  });

  test("keeps a detached output out of both the task file list and folder artwork", () => {
    const detached = file("detached-output", "removed.png", "image", {
      origin: { kind: "download", operationId: "operation-one" },
    });
    const kept = file("kept-output", "kept.mp4", "video", {
      origin: { kind: "download", operationId: "operation-one" },
    });
    const [item] = adaptLegacyLibraryTasks(
      [operation({
        outputFiles: [
          { fileId: detached.id, kind: "image", format: "png" },
          { fileId: kept.id, kind: "video", format: "mp4" },
        ],
        detachedOutputFileIds: [detached.id],
      })],
      "http://127.0.0.1:34115",
      [library([detached, kept])],
    );

    expect(item?.taskFiles?.map((entry) => entry.fileId)).toEqual([kept.id]);
    expect(item?.taskPreviewItems?.map((entry) => entry.id)).toEqual([kept.id]);
    expect(item?.taskPreviewTotalCount).toBe(1);
  });

  test("discovers an available operation file when an older output list is incomplete", () => {
    const discovered = file("discovered-artwork", "poster.png", "image", {
      origin: { kind: "download", operationId: "operation-one" },
    });
    const unsupported = file("unsupported-artwork", "poster.heic", "image", {
      latestOperationId: "operation-one",
    });
    const [item] = adaptLegacyLibraryTasks(
      [operation({ outputFiles: [] })],
      "http://127.0.0.1:34115",
      [library([discovered, unsupported])],
    );

    expect(item?.taskPreviewItems?.[0]?.previewURL).toContain("poster.png");
    expect(item?.taskPreviewItems?.[1]).toEqual({
      id: "unsupported-artwork",
      kind: "image",
      label: "HEIC",
    });
  });

  test("counts discovered files independently from stale operation metrics", () => {
    const discovered = ["one", "two", "three", "four"].map((id) =>
      file(id, `${id}.mp4`, "video", {
        origin: { kind: "download", operationId: "operation-one" },
      }));
    const [item] = adaptLegacyLibraryTasks(
      [operation({ metrics: { fileCount: 1 }, outputFiles: [] })],
      "http://127.0.0.1:34115",
      [library(discovered)],
    );

    expect(item?.taskPreviewItems).toHaveLength(3);
    expect(item?.taskPreviewTotalCount).toBe(4);
  });

  test("does not count a deleted output in the effective preview total", () => {
    const valid = ["one", "two", "three"].map((id) =>
      file(id, `${id}.mp4`, "video"));
    const [item] = adaptLegacyLibraryTasks(
      [operation({
        metrics: { fileCount: 4 },
        outputFiles: [
          { fileId: "deleted", kind: "video", deleted: true },
          ...valid.map((output) => ({ fileId: output.id, kind: output.kind })),
        ],
      })],
      "http://127.0.0.1:34115",
      [library(valid)],
    );

    expect(item?.taskPreviewItems).toHaveLength(3);
    expect(item?.taskPreviewTotalCount).toBe(3);
  });
});
