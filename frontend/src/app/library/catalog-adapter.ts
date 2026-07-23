import type { CatalogItem } from "@/shared/contracts/catalog";
import type { LibraryFileDTO } from "@/shared/contracts/library";
import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";
import {
  buildAssetPreviewURL,
  extractExtensionFromPath,
} from "@/shared/utils/resourceHelpers";

import type { LibraryOtherGroup, LibraryWorkspaceItem } from "./types";

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

export function adaptCatalogItems(
  items: readonly CatalogItem[],
  previewSources: CatalogItemPreviewSources = {},
): LibraryWorkspaceItem[] {
  return items.map((item) => {
    const group = item.category === "other" ? otherGroup(item.status, item.kind) : undefined;
    const fallbackCoverURL = defaultCover(item.category, group);
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
      format: item.format ?? "",
      sizeBytes: item.sizeBytes,
      durationMs: item.durationMs,
      createdAt: item.createdAt,
      updatedAt: item.updatedAt,
      path: "",
      coverURL: catalogCover(item, previewSources) || fallbackCoverURL,
      fallbackCoverURL,
      rootId: item.id,
      searchText: `${item.title} ${item.description ?? ""} ${item.category} ${item.kind ?? ""} ${item.status}`.toLocaleLowerCase(),
      catalogItem: item,
    };
  });
}

function catalogCover(
  item: CatalogItem,
  sources: CatalogItemPreviewSources,
) {
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
    item.status !== "missing" &&
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

function otherGroup(status: CatalogItem["status"], kind?: string): LibraryOtherGroup {
  if (status === "missing") return "missing";
  if (status === "needs_review") return "needs-review";
  const normalized = (kind ?? "").trim().toLocaleLowerCase();
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
