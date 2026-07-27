import type { CatalogItem } from "@/shared/contracts/catalog";
import type { LibraryFileDTO } from "@/shared/contracts/library";
import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";
import {
  buildAssetPreviewURL,
  extractExtensionFromPath,
} from "@/shared/utils/resourceHelpers";

import type {
  LibraryCardPreview,
  LibraryOtherGroup,
  LibraryWorkspaceItem,
} from "./types";

export interface CatalogItemPreviewSources {
  filesById?: ReadonlyMap<string, LibraryFileDTO>;
  httpBaseURL?: string;
}

const BROWSER_IMAGE_EXTENSIONS = new Set([
  "avif", "bmp", "gif", "ico", "jpeg", "jpg", "png", "svg", "webp",
]);

export function isBrowserImagePreviewPath(path: string) {
  return BROWSER_IMAGE_EXTENSIONS.has(extractExtensionFromPath(path));
}

export function buildCatalogVideoThumbnailURL(
  baseURL: string,
  itemId: string,
  cacheKey?: string,
) {
  const normalizedBaseURL = baseURL.trim().replace(/\/+$/, "");
  const normalizedItemId = itemId.trim();
  if (!normalizedBaseURL || !normalizedItemId) return "";
  const normalizedCacheKey = cacheKey?.trim() ?? "";
  const suffix = normalizedCacheKey
    ? `?v=${encodeURIComponent(normalizedCacheKey)}`
    : "";
  return `${normalizedBaseURL}/api/library/video-thumbnail/${encodeURIComponent(normalizedItemId)}${suffix}`;
}

export function buildCatalogCardPreviewURL(
  baseURL: string,
  kind: LibraryCardPreview["kind"],
  itemId: string,
  cacheKey?: string,
) {
  const normalizedBaseURL = baseURL.trim().replace(/\/+$/, "");
  const normalizedItemId = itemId.trim();
  if (!normalizedBaseURL || !normalizedItemId || !["pdf", "log"].includes(kind)) {
    return "";
  }
  const normalizedCacheKey = cacheKey?.trim() ?? "";
  const suffix = normalizedCacheKey
    ? `?v=${encodeURIComponent(normalizedCacheKey)}`
    : "";
  return `${normalizedBaseURL}/api/library/card-preview/${kind}/${encodeURIComponent(normalizedItemId)}${suffix}`;
}

export function adaptCatalogItems(
  items: readonly CatalogItem[],
  previewSources: CatalogItemPreviewSources = {},
): LibraryWorkspaceItem[] {
  return items.map((item) => {
    const availability = item.availability ??
      (item.status === "missing" || item.status === "trashed" ? "missing" : "available");
    const group = item.category === "other"
      ? otherGroup(item.status, item.kind, item.format, item.title)
      : undefined;
    const fallbackCoverURL = defaultCover(item.category, group);
    const coverURL = catalogCover(item, previewSources);
    return {
      id: item.id,
      source: "file",
      libraryId: item.catalogId,
      // Catalog list records expose an opaque catalog id, not a localized
      // display name. Leave this empty so presentation surfaces can use their
      // locale-specific Library label instead of leaking an English sentinel.
      libraryName: "",
      title: item.title,
      subtitle: item.description || item.category,
      category: item.category,
      otherGroup: group,
      status: item.status,
      availability,
      format: item.format ?? "",
      sizeBytes: item.sizeBytes,
      durationMs: item.durationMs,
      createdAt: item.createdAt,
      updatedAt: item.updatedAt,
      path: "",
      coverURL: coverURL || fallbackCoverURL,
      fallbackCoverURL,
      cardPreview: coverURL ? undefined : catalogCardPreview(item, previewSources),
      rootId: item.id,
      searchText: `${item.title} ${item.description ?? ""} ${item.category} ${item.kind ?? ""} ${item.status} ${availability}`.toLocaleLowerCase(),
      catalogItem: item,
    };
  });
}

function catalogCardPreview(
  item: CatalogItem,
  sources: CatalogItemPreviewSources,
): LibraryCardPreview | undefined {
  const availability = item.availability ??
    (item.status === "missing" || item.status === "trashed" ? "missing" : "available");
  if (
    !item.primaryFileId?.trim() ||
    availability !== "available" ||
    item.status === "trashed"
  ) {
    return undefined;
  }
  const normalizedFormat = (item.format ?? "").trim().toLocaleLowerCase();
  const normalizedTitle = item.title.trim().toLocaleLowerCase();
  const kind: LibraryCardPreview["kind"] | undefined =
    item.category === "book" && (
      normalizedFormat === "pdf" ||
      normalizedFormat === "application/pdf" ||
      normalizedTitle.endsWith(".pdf")
    )
      ? "pdf"
      : (
          normalizedFormat === "log" ||
          normalizedFormat === "text/x-log" ||
          normalizedTitle.endsWith(".log")
        )
        ? "log"
        : undefined;
  if (!kind) return undefined;
  const sourceURL = buildCatalogCardPreviewURL(
    sources.httpBaseURL ?? "",
    kind,
    item.id,
    item.updatedAt,
  );
  if (!sourceURL) return undefined;
  return {
    kind,
    sourceURL,
    cacheKey: `${item.id}:${item.updatedAt}:${item.sizeBytes ?? ""}`,
  };
}

function catalogCover(
  item: CatalogItem,
  sources: CatalogItemPreviewSources,
) {
  const availability = item.availability ??
    (item.status === "missing" || item.status === "trashed" ? "missing" : "available");
  // Artwork is already a bounded cover/thumbnail produced by the download or
  // import pipeline, so it is always safer than decoding an original media
  // file. Images may use their original as the final real-preview candidate;
  // other formats wait until the selected companion requests full content.
  const fileIds = [
    item.artworkFileId,
    item.category === "image" ? item.primaryFileId : undefined,
  ];
  for (const fileId of fileIds) {
    if (!fileId) continue;
    const file = sources.filesById?.get(fileId);
    const path = file?.storage.localPath?.trim() ?? "";
    const state = file?.state.status.trim().toLocaleLowerCase() ?? "";
    if (
      !file ||
      file.state.deleted ||
      Boolean(file.state.lastError?.trim()) ||
      !path ||
      ["deleted", "missing", "offline", "error", "unavailable"].includes(state)
    ) continue;
    if (!isBrowserImagePreviewPath(path)) {
      continue;
    }
    const resolved = buildAssetPreviewURL(sources.httpBaseURL ?? "", path, file.updatedAt);
    if (resolved) return resolved;
  }
  // The tokenized Desktop endpoint accepts only this opaque item ID. Its
  // handler resolves the registered primary file, creates at most one bounded
  // JPEG at a time, and serves the persisted cache. Native image lazy-loading
  // therefore controls when off-screen videos pay the one-time decode cost.
  if (
    item.category === "video" &&
    !item.artworkFileId?.trim() &&
    Boolean(item.primaryFileId?.trim()) &&
    availability === "available" &&
    item.status !== "trashed"
  ) {
    return buildCatalogVideoThumbnailURL(
      sources.httpBaseURL ?? "",
      item.id,
      item.updatedAt,
    );
  }
  return "";
}

function otherGroup(
  status: CatalogItem["status"],
  kind?: string,
  format?: string,
  title?: string,
): LibraryOtherGroup {
  if (status === "missing") return "missing";
  if (status === "needs_review") return "needs-review";
  const normalized = (kind ?? "").trim().toLocaleLowerCase();
  const normalizedFormat = (format ?? "").trim().toLocaleLowerCase();
  const normalizedTitle = (title ?? "").trim().toLocaleLowerCase();
  if (
    normalizedFormat === "log" ||
    normalizedFormat === "text/x-log" ||
    normalizedTitle.endsWith(".log")
  ) {
    return "document";
  }
  if (["document", "font", "archive", "subtitle", "manifest", "api"].includes(normalized)) {
    return normalized as LibraryOtherGroup;
  }
  return "unknown";
}

function defaultCover(category: CatalogItem["category"], group?: LibraryOtherGroup) {
  switch (category) {
    case "video": return COMPLETED_DEFAULT_COVER_IMAGE_URLS.video;
    case "audio": return COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio;
    case "book": return COMPLETED_DEFAULT_COVER_IMAGE_URLS.document;
    case "image": return COMPLETED_DEFAULT_COVER_IMAGE_URLS.image;
    default:
      if (group === "font") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.font;
      if (group === "archive") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.archive;
      if (group === "subtitle") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.subtitle;
      if (group === "manifest") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.manifest;
      if (group === "api") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.api;
      if (group === "document") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.document;
      return COMPLETED_DEFAULT_COVER_IMAGE_URLS.other;
  }
}
