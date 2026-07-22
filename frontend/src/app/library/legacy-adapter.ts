import {
  COMPLETED_DEFAULT_COVER_IMAGE_URLS,
} from "@/shared/assets/default-cover";
import type {
  LibraryDTO,
  LibraryFileDTO,
  OperationListItemDTO,
} from "@/shared/contracts/library";
import {
  buildAssetPreviewURL,
  extractExtensionFromPath,
  getPathBaseName,
  stripPathExtension,
} from "@/shared/utils/resourceHelpers";

import type {
  LibraryItemCategory,
  LibraryOtherGroup,
  LibraryTaskFileItem,
  LibraryTaskPreviewItem,
  LibraryWorkspaceItem,
} from "./types";
import { isBrowserImagePreviewPath } from "./catalog-adapter";

const VIDEO_EXTENSIONS = new Set([
  "3gp", "avi", "flv", "m2ts", "m4v", "mkv", "mov", "mp4", "mpeg",
  "mpg", "mts", "ogv", "ts", "webm", "wmv",
]);
const AUDIO_EXTENSIONS = new Set([
  "aac", "aiff", "alac", "ape", "flac", "m4a", "mp3", "oga", "ogg",
  "opus", "wav", "wma",
]);
const BOOK_EXTENSIONS = new Set([
  "azw", "azw3", "cb7", "cbr", "cbz", "djvu", "epub", "fb2", "ibooks",
  "mobi", "pdf",
]);
const IMAGE_EXTENSIONS = new Set([
  "avif", "bmp", "gif", "heic", "heif", "ico", "jpeg", "jpg", "png",
  "raw", "svg", "tif", "tiff", "webp",
]);
const DOCUMENT_EXTENSIONS = new Set([
  "csv", "doc", "docx", "key", "md", "numbers", "odp", "ods", "odt",
  "pages", "ppt", "pptx", "rtf", "tex", "txt", "xls", "xlsx",
]);
const FONT_EXTENSIONS = new Set(["eot", "otf", "ttc", "ttf", "woff", "woff2"]);
const ARCHIVE_EXTENSIONS = new Set([
  "7z", "bz2", "cab", "dmg", "gz", "iso", "rar", "tar", "tgz", "xz", "zip",
]);
const SUBTITLE_EXTENSIONS = new Set([
  "ass", "dfxp", "itt", "lrc", "sami", "smi", "srt", "ssa", "sub", "ttml", "vtt",
]);
const MANIFEST_EXTENSIONS = new Set(["m3u", "m3u8", "mpd", "pls"]);
const API_EXTENSIONS = new Set(["json", "jsonl", "toml", "xml", "yaml", "yml"]);

function fileName(file: LibraryFileDTO) {
  const path = file.storage.localPath?.trim() ?? "";
  return (
    file.displayName?.trim() ||
    file.displayLabel?.trim() ||
    file.fileName?.trim() ||
    file.metadata.title?.trim() ||
    file.name?.trim() ||
    getPathBaseName(path) ||
    file.id
  );
}

function fileExtension(file: LibraryFileDTO) {
  return extractExtensionFromPath(
    file.storage.localPath || file.fileName || file.name,
  );
}

function hasAny(value: string, candidates: Set<string>) {
  return candidates.has(value.trim().toLowerCase());
}

function isMissingFile(file: LibraryFileDTO) {
  const state = file.state.status.trim().toLowerCase();
  return (
    state === "missing" ||
    state === "unavailable" ||
    state === "offline" ||
    /\b(?:missing|not found|no such file)\b/i.test(file.state.lastError ?? "")
  );
}

function needsReview(file: LibraryFileDTO) {
  const status = file.state.status.trim().toLowerCase();
  return status === "needs_review" || status === "needs-review" || status === "review";
}

/**
 * A legacy file tombstone is never a browseable Library resource. Keep this
 * decision at the adapter boundary so loading fallbacks, Preview candidates,
 * related artwork, and the normal grid cannot accidentally resurrect it.
 */
function isSoftDeletedFile(file: LibraryFileDTO) {
  const status = file.state.status.trim().toLowerCase();
  return file.state.deleted || status === "deleted" || status === "trashed";
}

export function classifyLegacyLibraryFile(file: LibraryFileDTO): {
  category: Exclude<LibraryItemCategory, "task">;
  otherGroup?: LibraryOtherGroup;
} {
  if (isMissingFile(file)) {
    return { category: "other", otherGroup: "missing" };
  }
  if (needsReview(file)) {
    return { category: "other", otherGroup: "needs-review" };
  }

  const extension = fileExtension(file);
  const kind = file.kind.trim().toLowerCase();
  const format = file.media?.format?.trim().toLowerCase() ?? "";
  const matches = (values: Set<string>) =>
    hasAny(extension, values) || hasAny(format, values);

  if (kind === "video" || (kind === "transcode" && Boolean(file.media?.videoCodec)) || matches(VIDEO_EXTENSIONS)) {
    return { category: "video" };
  }
  if (kind === "audio" || (kind === "transcode" && Boolean(file.media?.audioCodec)) || matches(AUDIO_EXTENSIONS)) {
    return { category: "audio" };
  }
  if (kind === "book" || kind === "ebook" || matches(BOOK_EXTENSIONS)) {
    return { category: "book" };
  }
  if (kind === "image" || kind === "thumbnail" || matches(IMAGE_EXTENSIONS)) {
    return { category: "image" };
  }
  if (kind === "subtitle" || matches(SUBTITLE_EXTENSIONS)) {
    return { category: "other", otherGroup: "subtitle" };
  }
  if (kind === "manifest" || matches(MANIFEST_EXTENSIONS)) {
    return { category: "other", otherGroup: "manifest" };
  }
  if (kind === "font" || matches(FONT_EXTENSIONS)) {
    return { category: "other", otherGroup: "font" };
  }
  if (kind === "archive" || matches(ARCHIVE_EXTENSIONS)) {
    return { category: "other", otherGroup: "archive" };
  }
  if (kind === "api" || matches(API_EXTENSIONS)) {
    return { category: "other", otherGroup: "api" };
  }
  if (kind === "document" || matches(DOCUMENT_EXTENSIONS)) {
    return { category: "other", otherGroup: "document" };
  }
  return { category: "other", otherGroup: "unknown" };
}

function defaultCover(category: LibraryItemCategory, group?: LibraryOtherGroup) {
  switch (category) {
    case "video":
      return COMPLETED_DEFAULT_COVER_IMAGE_URLS.video;
    case "audio":
      return COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio;
    case "book":
      return COMPLETED_DEFAULT_COVER_IMAGE_URLS.document;
    case "image":
      return COMPLETED_DEFAULT_COVER_IMAGE_URLS.image;
    case "task":
      return COMPLETED_DEFAULT_COVER_IMAGE_URLS.mixed;
    case "other":
      if (group === "font") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.font;
      if (group === "archive") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.archive;
      if (group === "subtitle") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.subtitle;
      if (group === "manifest") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.manifest;
      if (group === "api") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.api;
      if (group === "document") return COMPLETED_DEFAULT_COVER_IMAGE_URLS.document;
      return COMPLETED_DEFAULT_COVER_IMAGE_URLS.other;
  }
}

function fileRootId(file: LibraryFileDTO) {
  return file.lineage.rootFileId?.trim() || file.id;
}

function buildThumbnailByRoot(libraries: readonly LibraryDTO[], httpBaseURL: string) {
  const result = new Map<string, string>();
  libraries.forEach((library) => {
    library.files.forEach((file) => {
      if (isSoftDeletedFile(file)) return;
      const classification = classifyLegacyLibraryFile(file);
      if (file.kind.trim().toLowerCase() !== "thumbnail" || classification.category !== "image") {
        return;
      }
      const path = file.storage.localPath?.trim() ?? "";
      const rootId = file.lineage.rootFileId?.trim() ?? "";
      const previewURL = buildAssetPreviewURL(httpBaseURL, path, file.updatedAt);
      if (rootId && previewURL) result.set(`${library.id}:${rootId}`, previewURL);
    });
  });
  return result;
}

const TASK_PREVIEW_ITEM_LIMIT = 3;
const TASK_PREVIEW_UNAVAILABLE_STATES = new Set([
  "deleted",
  "error",
  "missing",
  "offline",
  "unavailable",
]);

function isUnavailableTaskPreviewFile(file: LibraryFileDTO) {
  return (
    file.state.deleted ||
    Boolean(file.state.lastError?.trim()) ||
    TASK_PREVIEW_UNAVAILABLE_STATES.has(file.state.status.trim().toLowerCase())
  );
}

function taskPreviewLabel(
  kind: string,
  format: string | undefined,
  file: LibraryFileDTO | undefined,
) {
  return (
    format?.trim() ||
    file?.media?.format?.trim() ||
    (file ? fileExtension(file) : "") ||
    kind
  ).toUpperCase();
}

function classifyTaskOutput(
  kind: string,
  format: string | undefined,
): Exclude<LibraryItemCategory, "task"> {
  const normalizedKind = kind.trim().toLowerCase();
  const normalizedFormat = (format ?? "").trim().toLowerCase().replace(/^\./, "");
  const combined = `${normalizedKind} ${normalizedFormat}`;
  if (/\b(?:video|movie|film)\b/.test(combined) || VIDEO_EXTENSIONS.has(normalizedFormat)) {
    return "video";
  }
  if (/\b(?:audio|music|song)\b/.test(combined) || AUDIO_EXTENSIONS.has(normalizedFormat)) {
    return "audio";
  }
  if (/\b(?:image|artwork|thumbnail|cover|photo)\b/.test(combined) || IMAGE_EXTENSIONS.has(normalizedFormat)) {
    return "image";
  }
  if (/\b(?:book|ebook)\b/.test(combined) || BOOK_EXTENSIONS.has(normalizedFormat)) {
    return "book";
  }
  return "other";
}

function taskFileStatus(file: LibraryFileDTO | undefined, explicitlyDeleted: boolean) {
  if (explicitlyDeleted || file?.state.deleted) return "deleted";
  if (!file) return "missing";
  if (file.state.lastError?.trim()) return "error";
  return file.state.status.trim() || "available";
}

function buildTaskFiles(
  operation: OperationListItemDTO,
  library: LibraryDTO | undefined,
  httpBaseURL: string,
): readonly LibraryTaskFileItem[] {
  const files = library?.files ?? [];
  const filesById = new Map(files.map((file) => [file.id, file] as const));
  const detachedFileIds = new Set(operation.detachedOutputFileIds ?? []);
  const seenIds = new Set<string>();
  const result: LibraryTaskFileItem[] = [];

  const append = (input: {
    fileId: string;
    kind: string;
    format?: string;
    explicitlyDeleted?: boolean;
    file?: LibraryFileDTO;
  }) => {
    const fileId = input.fileId.trim();
    if (!fileId || seenIds.has(fileId) || detachedFileIds.has(fileId)) return;
    seenIds.add(fileId);

    const file = input.file;
    const kind = (input.kind.trim() || file?.kind.trim() || "file").toLowerCase();
    const category = classifyTaskOutput(
      kind,
      input.format || file?.media?.format || (file ? fileExtension(file) : ""),
    );
    const status = taskFileStatus(file, Boolean(input.explicitlyDeleted));
    const path = file?.storage.localPath?.trim() ?? "";
    const previewURL = category === "image" && path && isBrowserImagePreviewPath(path)
      ? buildAssetPreviewURL(httpBaseURL, path, file?.updatedAt)
      : "";
    const canView = Boolean(
      file &&
      path &&
      !input.explicitlyDeleted &&
      !isUnavailableTaskPreviewFile(file),
    );

    result.push({
      fileId,
      previewItemId: `file:${fileId}`,
      title: file ? fileName(file) : fileId,
      kind,
      category,
      status,
      format: taskPreviewLabel(kind, input.format, file),
      canView,
      ...(previewURL ? { previewURL } : {}),
      ...(file ? { file } : {}),
    });
  };

  (operation.outputFiles ?? []).forEach((output) => {
    append({
      fileId: output.fileId,
      kind: output.kind,
      format: output.format,
      explicitlyDeleted: output.deleted,
      file: filesById.get(output.fileId),
    });
  });

  // Older operations may not have a complete outputFiles snapshot. Keep the
  // Library records in their stable order and append only records not already
  // described by the operation.
  files.forEach((file) => {
    if (
      file.origin.operationId !== operation.operationId &&
      file.latestOperationId !== operation.operationId
    ) return;
    append({
      fileId: file.id,
      kind: file.kind,
      format: file.media?.format,
      file,
    });
  });

  return result;
}

function buildTaskPreviewItems(
  operation: OperationListItemDTO,
  library: LibraryDTO | undefined,
  httpBaseURL: string,
): {
  items: readonly LibraryTaskPreviewItem[];
  totalCount: number;
} {
  const files = library?.files ?? [];
  const filesById = new Map(files.map((file) => [file.id, file] as const));
  const detachedFileIds = new Set(operation.detachedOutputFileIds ?? []);
  const explicitlyDeletedIds = new Set(
    (operation.outputFiles ?? [])
      .filter((output) => output.deleted)
      .map((output) => output.fileId),
  );
  const seenIds = new Set<string>();
  const candidates: Array<{
    item: LibraryTaskPreviewItem;
    artworkKey: string;
    isPrimary: boolean;
    order: number;
  }> = [];

  const appendCandidate = (input: {
    id: string;
    kind: string;
    format?: string;
    file?: LibraryFileDTO;
    isPrimary?: boolean;
    order: number;
  }) => {
    const id = input.id.trim();
    if (!id || seenIds.has(id) || explicitlyDeletedIds.has(id) || detachedFileIds.has(id)) return;
    if (input.file && isUnavailableTaskPreviewFile(input.file)) return;
    seenIds.add(id);

    const kind = (input.kind.trim() || input.file?.kind.trim() || "file").toLowerCase();
    const path = input.file?.storage.localPath?.trim() ?? "";
    const isImage = input.file ? (
      kind === "image" ||
      kind === "thumbnail" ||
      classifyLegacyLibraryFile(input.file).category === "image"
    ) : false;
    const previewURL = isImage && path && isBrowserImagePreviewPath(path)
      ? buildAssetPreviewURL(httpBaseURL, path, input.file?.updatedAt)
      : "";
    candidates.push({
      item: {
        id,
        kind,
        ...(previewURL ? { previewURL } : {}),
        label: taskPreviewLabel(kind, input.format, input.file),
      },
      artworkKey: previewURL ? path.replace(/\\/g, "/") : "",
      isPrimary: Boolean(input.isPrimary),
      order: input.order,
    });
  };

  (operation.outputFiles ?? []).forEach((output, index) => {
    if (output.deleted) return;
    appendCandidate({
      id: output.fileId,
      kind: output.kind,
      format: output.format,
      file: filesById.get(output.fileId),
      isPrimary: output.isPrimary,
      order: index,
    });
  });

  // Some older operations have incomplete outputFiles arrays. Keep their real
  // Library files visible without inventing image URLs or backend lookups.
  files.forEach((file, index) => {
    if (
      file.origin.operationId !== operation.operationId &&
      file.latestOperationId !== operation.operationId
    ) return;
    appendCandidate({
      id: file.id,
      kind: file.kind,
      format: file.media?.format,
      file,
      order: (operation.outputFiles?.length ?? 0) + index,
    });
  });

  candidates.sort((left, right) => {
    if (left.isPrimary !== right.isPrimary) return left.isPrimary ? -1 : 1;
    return left.order - right.order;
  });

  const seenArtwork = new Set<string>();
  const normalizedCandidates: typeof candidates = candidates.map((candidate) => {
    const { item, artworkKey } = candidate;
    if (!artworkKey || !item.previewURL) return candidate;
    if (seenArtwork.has(artworkKey)) {
      const { previewURL: _duplicatePreviewURL, ...typePage } = item;
      return { ...candidate, item: typePage };
    }
    seenArtwork.add(artworkKey);
    return candidate;
  });
  normalizedCandidates.sort((left, right) => {
    const leftHasArtwork = Boolean(left.item.previewURL);
    const rightHasArtwork = Boolean(right.item.previewURL);
    if (leftHasArtwork !== rightHasArtwork) return leftHasArtwork ? -1 : 1;
    if (left.isPrimary !== right.isPrimary) return left.isPrimary ? -1 : 1;
    return left.order - right.order;
  });
  const items = normalizedCandidates
    .slice(0, TASK_PREVIEW_ITEM_LIMIT)
    .map(({ item }) => item);
  return { items, totalCount: candidates.length };
}

export function adaptLegacyLibraryFiles(
  libraries: readonly LibraryDTO[],
  httpBaseURL = "",
): LibraryWorkspaceItem[] {
  const thumbnailByRoot = buildThumbnailByRoot(libraries, httpBaseURL);
  return libraries.flatMap((library) =>
    library.files
      .filter((file) => !isSoftDeletedFile(file))
      .map((file): LibraryWorkspaceItem => {
        const classification = classifyLegacyLibraryFile(file);
        const path = file.storage.localPath?.trim() ?? "";
        const rootId = fileRootId(file);
        const title = stripPathExtension(fileName(file));
        const extension = fileExtension(file);
        const imagePreview = classification.category === "image"
          ? buildAssetPreviewURL(httpBaseURL, path, file.updatedAt)
          : "";
        const fallbackCoverURL = defaultCover(classification.category, classification.otherGroup);
        const coverURL =
          imagePreview ||
          thumbnailByRoot.get(`${library.id}:${rootId}`) ||
          fallbackCoverURL;
        const author = file.metadata.author?.trim() ?? "";
        const subtitle = author || extension.toUpperCase() || file.kind;
        const status = file.state.deleted ? "trashed" : file.state.status || "available";
        return {
          id: `file:${file.id}`,
          source: "file",
          libraryId: library.id,
          libraryName: library.name,
          title,
          subtitle,
          category: classification.category,
          otherGroup: classification.otherGroup,
          status,
          format: (file.media?.format || extension || file.kind).toUpperCase(),
          sizeBytes: file.media?.sizeBytes,
          durationMs: file.media?.durationMs,
          createdAt: file.createdAt,
          updatedAt: file.updatedAt,
          path,
          coverURL,
          fallbackCoverURL,
          rootId,
          searchText: [title, subtitle, path, file.kind, file.state.status, library.name]
            .join(" ")
            .toLocaleLowerCase(),
          file,
          library,
        };
      }),
  );
}

/**
 * Catalog browsing normally renders logical items, but an image file used as
 * another item's artwork is still a real Task output. Keep those available
 * files addressable from the Images route without duplicating image files that
 * already own a first-class Catalog item.
 */
export function adaptAvailableLegacyImageFiles(
  libraries: readonly LibraryDTO[],
  httpBaseURL = "",
  catalogImageFileIds: ReadonlySet<string> = new Set(),
): LibraryWorkspaceItem[] {
  return adaptLegacyLibraryFiles(libraries, httpBaseURL).filter((item) => {
    const file = item.file;
    if (!file || item.category !== "image" || catalogImageFileIds.has(file.id)) {
      return false;
    }
    const status = file.state.status.trim().toLowerCase();
    return (
      !file.state.deleted &&
      !file.state.lastError?.trim() &&
      !TASK_PREVIEW_UNAVAILABLE_STATES.has(status)
    );
  });
}

export function adaptLegacyLibraryTasks(
  operations: readonly OperationListItemDTO[],
  httpBaseURL = "",
  libraries: readonly LibraryDTO[] = [],
): LibraryWorkspaceItem[] {
  const librariesById = new Map(libraries.map((library) => [library.id, library] as const));
  return operations.map((operation): LibraryWorkspaceItem => {
    const title = operation.name.trim() || operation.operationId;
    const thumbnailPath = operation.thumbnailPreviewPath?.trim() ?? "";
    const fallbackCoverURL = defaultCover("task");
    const coverURL =
      buildAssetPreviewURL(httpBaseURL, thumbnailPath, operation.finishedAt || operation.createdAt) ||
      fallbackCoverURL;
    const updatedAt = operation.finishedAt || operation.startedAt || operation.createdAt;
    const subtitle = operation.uploader?.trim() || operation.domain?.trim() || operation.kind;
    const taskPreview = buildTaskPreviewItems(
      operation,
      librariesById.get(operation.libraryId),
      httpBaseURL,
    );
    const taskFiles = buildTaskFiles(
      operation,
      librariesById.get(operation.libraryId),
      httpBaseURL,
    );
    const library = librariesById.get(operation.libraryId);
    return {
      id: `task:${operation.operationId}`,
      source: "task",
      libraryId: operation.libraryId,
      libraryName: operation.libraryName?.trim() ?? "",
      title,
      subtitle,
      category: "task",
      status: operation.status,
      format: operation.kind.toUpperCase(),
      sizeBytes: operation.metrics.totalSizeBytes,
      durationMs: operation.metrics.durationMs,
      createdAt: operation.createdAt,
      updatedAt,
      path: operation.request?.inputPath?.trim() || operation.request?.url?.trim() || "",
      coverURL,
      fallbackCoverURL,
      taskPreviewItems: taskPreview.items,
      taskPreviewTotalCount: taskPreview.totalCount,
      taskFiles,
      rootId: operation.operationId,
      searchText: [
        title,
        subtitle,
        operation.kind,
        operation.status,
        operation.platform,
        operation.libraryName,
      ].filter(Boolean).join(" ").toLocaleLowerCase(),
      operation,
      library,
    };
  });
}

export function adaptLegacyLibraryWorkspace(
  libraries: readonly LibraryDTO[],
  operations: readonly OperationListItemDTO[],
  httpBaseURL = "",
) {
  return {
    files: adaptLegacyLibraryFiles(libraries, httpBaseURL),
    tasks: adaptLegacyLibraryTasks(operations, httpBaseURL, libraries),
  };
}
