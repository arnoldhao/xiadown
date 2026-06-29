
import {
getXiaText,
resolveLibraryCoverURL,
} from "@/features/xiadown/shared";
import {
COMPLETED_DEFAULT_COVER_IMAGE_URLS,
DEFAULT_COVER_IMAGE_URL,
type CompletedDefaultCoverImageKey,
} from "@/shared/assets/default-cover";
import type {
LibraryDTO,
LibraryMediaInfoDTO,
OperationListItemDTO
} from "@/shared/contracts/library";
import type { ProxySettings,Settings } from "@/shared/contracts/settings";
import { getLanguage } from "@/shared/i18n";
import { formatBytes } from "@/shared/utils/formatBytes";
import {
buildAssetPreviewURL,
extractExtensionFromPath
} from "@/shared/utils/resourceHelpers";
import {
FileArchive,
FileBraces,
FileCode,
FileText,
FileType,
FileVideo,
ImageIcon,
Languages,
Link2,
Music2,
} from "lucide-react";

import { COMPLETED_PREVIEW_SUPPORT_CACHE } from "@/app/main/main-constants";
import type { CompletedDeleteConfirmation,CompletedFileEntry,CompletedFileType,CompletedPreviewGroupKind,CompletedTaskFileTypeSummary,CompletedViewMode } from "@/app/main/types";

export const AUDIO_FILE_EXTENSIONS = new Set([
  "aac",
  "aiff",
  "alac",
  "ape",
  "flac",
  "m4a",
  "mp3",
  "oga",
  "ogg",
  "opus",
  "wav",
  "wma",
]);
export const VIDEO_FILE_EXTENSIONS = new Set([
  "avi",
  "flv",
  "m2ts",
  "m4v",
  "mkv",
  "mov",
  "mp4",
  "mpeg",
  "mpg",
  "mts",
  "ogv",
  "ts",
  "webm",
  "wmv",
]);
export const IMAGE_FILE_EXTENSIONS = new Set([
  "apng",
  "avif",
  "gif",
  "heic",
  "jpeg",
  "jpg",
  "png",
  "svg",
  "webp",
]);
export const SUBTITLE_FILE_EXTENSIONS = new Set([
  "ass",
  "dfxp",
  "fcpxml",
  "itt",
  "lrc",
  "sbv",
  "srt",
  "ssa",
  "ttml",
  "vtt",
]);
export const MANIFEST_FILE_EXTENSIONS = new Set([
  "f4m",
  "ism",
  "m3u8",
  "mpd",
]);
export const DOCUMENT_FILE_EXTENSIONS = new Set([
  "doc",
  "docx",
  "pdf",
  "ppt",
  "pptx",
  "xls",
  "xlsx",
]);
export const FONT_FILE_EXTENSIONS = new Set([
  "eot",
  "otf",
  "ttf",
  "woff",
  "woff2",
]);
export const API_FILE_EXTENSIONS = new Set([
  "json",
]);
export const ARCHIVE_FILE_EXTENSIONS = new Set([
  "7z",
  "dmg",
  "exe",
  "pkg",
  "rar",
  "zip",
]);
export const COMPLETED_TASK_FILE_TYPE_LIMIT = 4;
export const COMPLETED_FILE_TYPE_ORDER: CompletedFileType[] = [
  "video",
  "audio",
  "subtitle",
  "image",
  "manifest",
  "document",
  "font",
  "archive",
  "api",
  "other",
];
export const COMPLETED_TEXT_PREVIEW_MAX_BYTES = 512 * 1024;
export const COMPLETED_IMAGE_PREVIEW_MAX_BYTES = 32 * 1024 * 1024;
export const SITE_KEYS = new Set([
  "youtube",
  "bilibili",
  "tiktok",
  "china_private",
  "instagram",
  "x",
  "facebook",
  "vimeo",
  "twitch",
  "niconico",
]);

export function normalizeProxy(settings?: Settings | null): ProxySettings {
  return (
    settings?.proxy ?? {
      mode: "system",
      scheme: "http",
      host: "",
      port: 0,
      username: "",
      password: "",
      noProxy: [],
      timeoutSeconds: 30,
      testedAt: "",
      testSuccess: false,
      testMessage: "",
    }
  );
}

export function resolveStatusTone(status?: string) {
  switch ((status ?? "").trim().toLowerCase()) {
    case "succeeded":
    case "installed":
      return "bg-emerald-500/15 text-emerald-700 dark:text-emerald-200";
    case "running":
    case "queued":
      return "bg-sky-500/15 text-sky-700 dark:text-sky-200";
    case "failed":
    case "invalid":
      return "bg-rose-500/15 text-rose-700 dark:text-rose-200";
    case "canceled":
      return "bg-amber-500/15 text-amber-700 dark:text-amber-200";
    default:
      return "bg-muted text-muted-foreground";
  }
}

export function formatRelativeTime(value?: string) {
  if (!value) {
    return "";
  }
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed)) {
    return value;
  }
  const delta = parsed - Date.now();
  const absDelta = Math.abs(delta);
  const locale = getLanguage();
  const rtf =
    typeof Intl !== "undefined" &&
    typeof Intl.RelativeTimeFormat !== "undefined"
      ? new Intl.RelativeTimeFormat(locale, { numeric: "auto", style: "short" })
      : null;

  const units: Array<{ unit: Intl.RelativeTimeFormatUnit; ms: number }> = [
    { unit: "year", ms: 365 * 24 * 60 * 60 * 1000 },
    { unit: "month", ms: 30 * 24 * 60 * 60 * 1000 },
    { unit: "week", ms: 7 * 24 * 60 * 60 * 1000 },
    { unit: "day", ms: 24 * 60 * 60 * 1000 },
    { unit: "hour", ms: 60 * 60 * 1000 },
    { unit: "minute", ms: 60 * 1000 },
    { unit: "second", ms: 1000 },
  ];

  const match =
    units.find((item) => absDelta >= item.ms) ?? units[units.length - 1];
  const amount = Math.round(delta / match.ms);
  if (rtf) {
    return rtf.format(amount, match.unit);
  }
  return value;
}

export function formatLocalDateTime(value?: string) {
  const trimmed = (value ?? "").trim();
  if (!trimmed) {
    return "";
  }
  const parsed = Date.parse(trimmed);
  if (!Number.isFinite(parsed)) {
    return trimmed;
  }
  const date = new Date(parsed);
  if (
    typeof Intl !== "undefined" &&
    typeof Intl.DateTimeFormat !== "undefined"
  ) {
    return new Intl.DateTimeFormat(getLanguage(), {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(date);
  }
  return date.toLocaleString();
}

export function normalizeDependencyVersion(version?: string, dependencyName?: string) {
  let value = (version ?? "").trim();
  if (!value) {
    return "";
  }
  value = value.replace(/^v/i, "");
  if ((dependencyName ?? "").trim().toLowerCase() === "ffmpeg") {
    value = value.replace(/^n-/i, "").replace(/-tessus$/i, "");
  }
  return value;
}

export function formatVersionBadge(version?: string) {
  const value = (version ?? "").trim();
  if (!value) {
    return "";
  }
  return value.toLowerCase().startsWith("v") ? value : `v${value}`;
}

export function resolveOperationUpdatedAt(operation: OperationListItemDTO) {
  return (
    operation.progress?.updatedAt ||
    operation.finishedAt ||
    operation.startedAt ||
    operation.createdAt ||
    ""
  );
}

export function resolveOperationKindLabel(
  text: ReturnType<typeof getXiaText>,
  kind?: string,
) {
  switch ((kind ?? "").trim().toLowerCase()) {
    case "download":
      return text.running.downloadBadge;
    case "transcode":
      return text.running.transcodeBadge;
    default:
      return kind || text.common.unknown;
  }
}

export function resolveCompletedStatusLabel(
  text: ReturnType<typeof getXiaText>,
  status?: string,
) {
  switch ((status ?? "").trim().toLowerCase()) {
    case "succeeded":
      return text.completed.succeeded;
    case "failed":
      return text.completed.failed;
    case "canceled":
      return text.completed.canceled;
    default:
      return status || text.common.unknown;
  }
}

export function resolveCompletedPreviewKind(
  file: Pick<CompletedFileEntry, "kind" | "path" | "format">,
) {
  const normalizedKind = (file.kind ?? "").trim().toLowerCase();
  const extension = (extractExtensionFromPath(file.path) || file.format || "")
    .trim()
    .toLowerCase();

  if (normalizedKind === "subtitle" || SUBTITLE_FILE_EXTENSIONS.has(extension)) {
    return "subtitle" as const;
  }
  if (
    normalizedKind === "image" ||
    normalizedKind === "thumbnail" ||
    IMAGE_FILE_EXTENSIONS.has(extension)
  ) {
    return "image" as const;
  }
  if (normalizedKind === "audio" || AUDIO_FILE_EXTENSIONS.has(extension)) {
    return "audio" as const;
  }
  if (
    normalizedKind === "video" ||
    normalizedKind === "transcode" ||
    VIDEO_FILE_EXTENSIONS.has(extension)
  ) {
    return "video" as const;
  }
  return "other" as const;
}

export function resolveCompletedPreviewMaxBytes(
  file: Pick<CompletedFileEntry, "kind" | "path" | "format">,
) {
  const previewKind = resolveCompletedPreviewKind(file);
  if (previewKind === "subtitle") {
    return COMPLETED_TEXT_PREVIEW_MAX_BYTES;
  }
  if (previewKind === "image") {
    return COMPLETED_IMAGE_PREVIEW_MAX_BYTES;
  }
  return 0;
}

export function isCompletedPreviewTooLarge(
  file: Pick<CompletedFileEntry, "kind" | "path" | "format" | "sizeBytes">,
) {
  const maxBytes = resolveCompletedPreviewMaxBytes(file);
  return maxBytes > 0 && file.sizeBytes > maxBytes;
}

export function normalizeCompletedMediaToken(value?: string) {
  return (value ?? "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "");
}

export function resolveCompletedProbeMime(
  kind: "video" | "audio",
  format?: string,
  path?: string,
) {
  const normalized = normalizeCompletedMediaToken(
    format || extractExtensionFromPath(path || ""),
  );
  if (kind === "video") {
    switch (normalized) {
      case "mp4":
      case "m4v":
        return "video/mp4";
      case "mov":
        return "video/quicktime";
      case "webm":
        return "video/webm";
      case "ogv":
      case "ogg":
        return "video/ogg";
      case "avi":
        return "video/x-msvideo";
      case "mkv":
      case "matroska":
        return "video/x-matroska";
      case "mpeg":
      case "mpg":
        return "video/mpeg";
      case "3gp":
        return "video/3gpp";
      default:
        return "";
    }
  }

  switch (normalized) {
    case "mp3":
      return "audio/mpeg";
    case "m4a":
    case "mp4a":
      return "audio/mp4";
    case "aac":
      return "audio/aac";
    case "webm":
      return "audio/webm";
    case "ogg":
    case "oga":
      return "audio/ogg";
    case "flac":
      return "audio/flac";
    case "wav":
      return "audio/wav";
    default:
      return "";
  }
}

export function resolveCompletedCodecProbeCandidates(
  mediaKind: "video" | "audio",
  codec?: string,
) {
  const normalized = normalizeCompletedMediaToken(codec);
  if (!normalized) {
    return [];
  }

  if (mediaKind === "video") {
    switch (normalized) {
      case "h264":
      case "avc":
      case "avc1":
        return ["avc1.42E01E", "avc1", "h264"];
      case "h265":
      case "hevc":
      case "hev1":
      case "hvc1":
        return ["hvc1.1.6.L93.B0", "hev1.1.6.L93.B0", "hvc1", "hevc", "h265"];
      case "av1":
      case "av01":
        return ["av01.0.05M.08", "av1"];
      case "vp9":
      case "vp09":
        return ["vp09.00.10.08", "vp9"];
      case "vp8":
        return ["vp8"];
      case "mpeg4":
      case "mp4v":
        return ["mp4v.20.8", "mpeg4"];
      case "theora":
        return ["theora"];
      default:
        return [normalized];
    }
  }

  switch (normalized) {
    case "aac":
    case "mp4a":
      return ["mp4a.40.2", "aac"];
    case "opus":
      return ["opus"];
    case "mp3":
      return ["mp3"];
    case "vorbis":
      return ["vorbis"];
    case "flac":
      return ["flac"];
    case "pcm":
    case "wav":
      return ["pcm"];
    default:
      return [normalized];
  }
}

export function buildCompletedProbeTypes(
  file: Pick<CompletedFileEntry, "kind" | "path" | "format" | "media">,
  previewKind: "video" | "audio",
) {
  const mime = resolveCompletedProbeMime(
    previewKind,
    file.media?.format || file.format,
    file.path,
  );
  if (!mime) {
    return [];
  }

  const videoCodecs =
    previewKind === "video"
      ? resolveCompletedCodecProbeCandidates(
          "video",
          file.media?.videoCodec || file.media?.codec,
        )
      : [];
  const audioCodecs = resolveCompletedCodecProbeCandidates(
    "audio",
    file.media?.audioCodec ||
      (previewKind === "audio" ? file.media?.codec : ""),
  );
  const probeTypes = new Set<string>([mime]);

  if (previewKind === "video") {
    if (videoCodecs.length > 0 && audioCodecs.length > 0) {
      for (const videoCodec of videoCodecs) {
        for (const audioCodec of audioCodecs) {
          probeTypes.add(`${mime}; codecs="${videoCodec}, ${audioCodec}"`);
        }
      }
    } else if (videoCodecs.length > 0) {
      for (const videoCodec of videoCodecs) {
        probeTypes.add(`${mime}; codecs="${videoCodec}"`);
      }
    }
  } else if (audioCodecs.length > 0) {
    for (const audioCodec of audioCodecs) {
      probeTypes.add(`${mime}; codecs="${audioCodec}"`);
    }
  }

  return [...probeTypes];
}

export function canPreviewCompletedFile(
  file: Pick<
    CompletedFileEntry,
    "id" | "kind" | "path" | "format" | "previewURL" | "media" | "sizeBytes"
  >,
) {
  const previewKind = resolveCompletedPreviewKind(file);
  if (previewKind === "subtitle" || previewKind === "image") {
    return Boolean(file.previewURL) && !isCompletedPreviewTooLarge(file);
  }
  if (previewKind === "other") {
    return false;
  }
  if (previewKind !== "video" && previewKind !== "audio") {
    return true;
  }
  if (!file.previewURL) {
    return false;
  }

  if (typeof document === "undefined") {
    return true;
  }

  const cacheKey = [
    previewKind,
    file.previewURL,
    file.media?.format || file.format || "",
    file.media?.videoCodec || "",
    file.media?.audioCodec || "",
    file.media?.codec || "",
  ].join("|");
  const cached = COMPLETED_PREVIEW_SUPPORT_CACHE.get(cacheKey);
  if (typeof cached === "boolean") {
    return cached;
  }

  const probeTypes = buildCompletedProbeTypes(file, previewKind);
  if (probeTypes.length === 0) {
    COMPLETED_PREVIEW_SUPPORT_CACHE.set(cacheKey, true);
    return true;
  }

  const mediaElement = document.createElement(
    previewKind === "audio" ? "audio" : "video",
  );
  const supported = probeTypes.some((type) => {
    try {
      return mediaElement.canPlayType(type).trim() !== "";
    } catch {
      return false;
    }
  });
  COMPLETED_PREVIEW_SUPPORT_CACHE.set(cacheKey, supported);
  return supported;
}

export function resolveCompletedFileType(
  file: Pick<CompletedFileEntry, "kind" | "path" | "format"> & {
    media?: LibraryMediaInfoDTO | null;
  },
): CompletedFileType {
  switch (resolveCompletedCoverFileKind(file)) {
    case "video":
      return "video";
    case "audio":
      return "audio";
    case "subtitle":
      return "subtitle";
    case "image":
      return "image";
    case "manifest":
    case "live":
      return "manifest";
    case "api":
      return "api";
    case "document":
      return "document";
    case "font":
      return "font";
    case "archive":
      return "archive";
    default:
      return "other";
  }
}

export function resolveCompletedFileTypeLabel(
  type: CompletedFileType,
  text: ReturnType<typeof getXiaText>,
) {
  switch (type) {
    case "video":
      return text.completed.typeVideo;
    case "audio":
      return text.completed.typeAudio;
    case "subtitle":
      return text.completed.typeSubtitle;
    case "image":
      return text.completed.typeImage;
    case "manifest":
      return text.completed.typeManifest;
    case "api":
      return text.completed.typeApi;
    case "document":
      return text.completed.typeDocument;
    case "font":
      return text.completed.typeFont;
    case "archive":
      return text.completed.typeArchive;
    default:
      return text.completed.typeOther;
  }
}

export function resolveCompletedPreviewGroupKind(
  file: Pick<CompletedFileEntry, "kind" | "path" | "format">,
): CompletedPreviewGroupKind {
  return resolveCompletedFileType(file);
}

export function resolveCompletedTaskFileTypeSummaries(
  files: Array<
    Pick<CompletedFileEntry, "kind" | "path" | "format"> & {
      media?: LibraryMediaInfoDTO | null;
    }
  >,
  limit = COMPLETED_TASK_FILE_TYPE_LIMIT,
): CompletedTaskFileTypeSummary[] {
  const counts = new Map<CompletedFileType, number>();
  files.forEach((file) => {
    const type = resolveCompletedFileType(file);
    counts.set(type, (counts.get(type) ?? 0) + 1);
  });
  const maxItems = Math.max(0, limit);
  return COMPLETED_FILE_TYPE_ORDER.map((type) => ({
    type,
    count: counts.get(type) ?? 0,
  }))
    .filter((item) => item.count > 0)
    .slice(0, maxItems);
}

export type CompletedCoverFileKind =
  | "video"
  | "audio"
  | "subtitle"
  | "image"
  | "live"
  | "manifest"
  | "api"
  | "document"
  | "font"
  | "archive"
  | "other";

type CompletedCoverFileLike = Pick<
  CompletedFileEntry,
  "kind" | "path" | "format"
> & {
  media?: LibraryMediaInfoDTO | null;
  previewURL?: string;
};

const COMPLETED_DEFAULT_COVER_NON_MEDIA_ORDER: CompletedCoverFileKind[] = [
  "live",
  "manifest",
  "document",
  "font",
  "archive",
  "api",
  "other",
];

function completedFileExtension(
  file: Pick<CompletedFileEntry, "path" | "format">,
) {
  return (extractExtensionFromPath(file.path) || file.format || "")
    .trim()
    .toLowerCase();
}

function completedCoverURLIsLegacyDefault(value?: string) {
  return (value ?? "").trim() === DEFAULT_COVER_IMAGE_URL;
}

export function isCompletedLibraryFileUnavailable(
  file?: Pick<LibraryDTO["files"][number], "state"> | null,
) {
  return (file?.state.lastError ?? "").trim() === "missing_local_file";
}

export function resolveCompletedCoverFileKind(
  file: CompletedCoverFileLike,
): CompletedCoverFileKind {
  const normalizedKind = (file.kind ?? "").trim().toLowerCase();
  const extension = completedFileExtension(file);
  const hasVideoSignals = Boolean(
    file.media?.videoCodec ||
      file.media?.width ||
      file.media?.height ||
      file.media?.frameRate,
  );
  const hasAudioSignals = Boolean(
    file.media?.audioCodec || file.media?.channels,
  );

  if (
    normalizedKind === "image" ||
    normalizedKind === "thumbnail" ||
    IMAGE_FILE_EXTENSIONS.has(extension)
  ) {
    return "image";
  }
  if (normalizedKind === "subtitle" || SUBTITLE_FILE_EXTENSIONS.has(extension)) {
    return "subtitle";
  }
  if (MANIFEST_FILE_EXTENSIONS.has(extension)) {
    return "manifest";
  }
  if (DOCUMENT_FILE_EXTENSIONS.has(extension)) {
    return "document";
  }
  if (FONT_FILE_EXTENSIONS.has(extension)) {
    return "font";
  }
  if (API_FILE_EXTENSIONS.has(extension)) {
    return "api";
  }
  if (ARCHIVE_FILE_EXTENSIONS.has(extension)) {
    return "archive";
  }
  if (
    normalizedKind === "audio" ||
    AUDIO_FILE_EXTENSIONS.has(extension) ||
    (normalizedKind === "transcode" && hasAudioSignals && !hasVideoSignals)
  ) {
    return "audio";
  }
  if (
    normalizedKind === "video" ||
    normalizedKind === "transcode" ||
    VIDEO_FILE_EXTENSIONS.has(extension)
  ) {
    return "video";
  }

  switch (normalizedKind) {
    case "live":
      return "live";
    case "manifest":
      return "manifest";
    case "api":
      return "api";
    case "document":
      return "document";
    case "font":
      return "font";
    case "archive":
      return "archive";
  }

  return "other";
}

export function resolveCompletedDefaultCoverImageKey(
  files: CompletedCoverFileLike[],
): CompletedDefaultCoverImageKey {
  const kinds = new Set(files.map(resolveCompletedCoverFileKind));
  if (kinds.size === 0) {
    return "other";
  }
  if (kinds.has("image")) {
    return "image";
  }

  const hasVideo = kinds.has("video");
  const hasAudio = kinds.has("audio");
  const hasSubtitle = kinds.has("subtitle");
  if (hasVideo && hasAudio && hasSubtitle) {
    return "mediaSubtitle";
  }
  if (hasVideo && hasAudio) {
    return "media";
  }
  if (hasVideo && hasSubtitle) {
    return "videoSubtitle";
  }
  if (hasAudio && hasSubtitle) {
    return "audioSubtitle";
  }
  if (hasVideo) {
    return "video";
  }
  if (hasAudio) {
    return "audio";
  }
  if (hasSubtitle) {
    return "subtitle";
  }

  const nonMediaKinds = COMPLETED_DEFAULT_COVER_NON_MEDIA_ORDER.filter((kind) =>
    kinds.has(kind),
  );
  if (nonMediaKinds.length === 1) {
    return nonMediaKinds[0];
  }
  return nonMediaKinds.length > 1 ? "mixed" : "other";
}

export function resolveCompletedDefaultCoverImageURL(
  files: CompletedCoverFileLike[],
) {
  return COMPLETED_DEFAULT_COVER_IMAGE_URLS[
    resolveCompletedDefaultCoverImageKey(files)
  ];
}

export function resolveCompletedRepresentativeImageCoverURL(
  files: CompletedCoverFileLike[],
) {
  for (const file of files) {
    if (resolveCompletedCoverFileKind(file) !== "image") {
      continue;
    }
    const previewURL = file.previewURL?.trim() ?? "";
    if (previewURL) {
      return previewURL;
    }
  }
  return "";
}

export function resolveCompletedTaskCoverURL(
  files: CompletedCoverFileLike[],
  preferredCoverURL?: string,
) {
  const preferred = preferredCoverURL?.trim() ?? "";
  if (preferred && !completedCoverURLIsLegacyDefault(preferred)) {
    return preferred;
  }
  return (
    resolveCompletedRepresentativeImageCoverURL(files) ||
    resolveCompletedDefaultCoverImageURL(files)
  );
}

export function resolveCompletedFileCoverURL(
  file: CompletedCoverFileLike,
  preferredCoverURL?: string,
) {
  const imageCoverURL = resolveCompletedRepresentativeImageCoverURL([file]);
  if (imageCoverURL) {
    return imageCoverURL;
  }
  const preferred = preferredCoverURL?.trim() ?? "";
  if (preferred && !completedCoverURLIsLegacyDefault(preferred)) {
    return preferred;
  }
  return resolveCompletedDefaultCoverImageURL([file]);
}

export function resolveCompletedSelectionSummary(
  count: number,
  text: ReturnType<typeof getXiaText>,
) {
  const label = text.completed.selectionSummary;
  const unit = text.completed.selectionUnit;
  const compact = /[\u4e00-\u9fff]/.test(`${label}${unit}`);
  return compact
    ? `${label}${count}${unit}`
    : `${label} ${count} ${unit}`.trim();
}

export function formatCompletedDeleteMessage(
  template: string,
  values: { name?: string; count?: number },
) {
  return template
    .replace("{name}", values.name ?? "")
    .replace("{count}", String(values.count ?? 0));
}

export function resolveCompletedDeleteDialogTitle(
  target: CompletedDeleteConfirmation,
  text: ReturnType<typeof getXiaText>,
) {
  if (target.kind === "tasks") {
    return target.count > 1
      ? text.completed.deleteTasksTitle
      : text.completed.deleteTaskTitle;
  }
  return target.count > 1
    ? text.completed.deleteFilesTitle
    : text.completed.deleteFileTitle;
}

export function resolveCompletedDeleteDialogMessage(
  target: CompletedDeleteConfirmation,
  text: ReturnType<typeof getXiaText>,
) {
  if (target.kind === "tasks") {
    return formatCompletedDeleteMessage(
      target.count > 1
        ? text.completed.deleteTasksMessage
        : text.completed.deleteTaskMessage,
      { name: target.label, count: target.count },
    );
  }
  return formatCompletedDeleteMessage(
    target.count > 1
      ? text.completed.deleteFilesMessage
      : text.completed.deleteFileMessage,
    { name: target.label, count: target.count },
  );
}

export function resolveCompletedTotalLabel(
  count: number,
  viewMode: CompletedViewMode,
  text: ReturnType<typeof getXiaText>,
) {
  const isChinese = getLanguage().startsWith("zh");
  const unit =
    viewMode === "tasks"
      ? text.completed.taskCountLabel
      : text.completed.fileCountLabel;
  return isChinese
    ? `${text.completed.total} ${count} ${unit}`
    : `${text.completed.total} ${count} ${unit}`;
}

export function resolveCompletedPerPageLabel(
  pageSize: number,
  text: ReturnType<typeof getXiaText>,
) {
  const isChinese = getLanguage().startsWith("zh");
  return isChinese
    ? `${text.completed.perPage}${pageSize}${text.completed.itemUnit}`
    : `${text.completed.perPage} ${pageSize} ${text.completed.itemUnit}`;
}

export function resolveCompletedPageLabel(
  page: number,
  pageCount: number,
  text: ReturnType<typeof getXiaText>,
) {
  const isChinese = getLanguage().startsWith("zh");
  return isChinese
    ? `${text.completed.page}${page}/${pageCount}${text.completed.pageSuffix}`
    : `${text.completed.page} ${page}/${pageCount}`;
}

export function resolveCompletedTaskSourceLabel(
  operation: Pick<OperationListItemDTO, "domain" | "platform" | "kind">,
) {
  const fallback =
    operation.kind === "transcode" ? "local" : operation.platform?.trim() || "";
  return (operation.domain?.trim() || fallback).toUpperCase();
}

export function formatCompletedTranscodedFromLabel(
  text: ReturnType<typeof getXiaText>,
  sourceName?: string,
) {
  const name = sourceName?.trim() ?? "";
  return name ? `${text.completed.transcodedFrom} ${name}` : "";
}

export function formatCompletedDuration(durationMs?: number) {
  if (!durationMs || durationMs <= 0) {
    return "";
  }
  const totalSeconds = Math.max(0, Math.round(durationMs / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

export function formatCompletedResolution(width?: number, height?: number) {
  if (!width || !height) {
    return "";
  }
  return `${width}x${height}`;
}

export function formatCompletedBitrate(bitrateKbps?: number) {
  if (!bitrateKbps || bitrateKbps <= 0) {
    return "";
  }
  if (bitrateKbps >= 1000) {
    const mbps = bitrateKbps / 1000;
    return `${Number.isInteger(mbps) ? mbps.toFixed(0) : mbps.toFixed(1).replace(/\.0$/, "")} Mbps`;
  }
  return `${Math.round(bitrateKbps)} Kbps`;
}

export function formatCompletedFrameRate(frameRate?: number) {
  if (!frameRate || frameRate <= 0) {
    return "";
  }
  const formatted = Number.isInteger(frameRate)
    ? frameRate.toFixed(0)
    : frameRate
        .toFixed(2)
        .replace(/\.00$/, "")
        .replace(/(\.\d)0$/, "$1");
  return `${formatted} fps`;
}

export function formatCompletedChannels(channels?: number) {
  if (!channels || channels <= 0) {
    return "";
  }
  return `${channels} ch`;
}

export function formatCompletedCueCount(
  cueCount: number | undefined,
  text: ReturnType<typeof getXiaText>,
) {
  if (!cueCount || cueCount <= 0) {
    return "";
  }
  const compact = /[\u4e00-\u9fff]/.test(text.completed.lineUnit);
  return compact
    ? `${cueCount}${text.completed.lineUnit}`
    : `${cueCount} ${text.completed.lineUnit}`;
}

export function formatCompletedDpi(dpi?: number) {
  if (!dpi || dpi <= 0) {
    return "";
  }
  return `${Math.round(dpi)} dpi`;
}

export function resolveCompletedCodecSummary(media?: LibraryMediaInfoDTO | null) {
  const videoCodec = media?.videoCodec
    ? formatCodecLabel(media.videoCodec)
    : "";
  const audioCodec = media?.audioCodec
    ? formatCodecLabel(media.audioCodec)
    : "";
  const singleCodec = media?.codec ? formatCodecLabel(media.codec) : "";
  if (videoCodec && audioCodec) {
    return `${videoCodec} / ${audioCodec}`;
  }
  return videoCodec || audioCodec || singleCodec;
}

export function resolveCompletedFileFormatLabel(
  file: CompletedFileEntry,
  text: ReturnType<typeof getXiaText>,
) {
  const format = (file.format || file.media?.format || "").trim();
  return format ? format.toUpperCase() : text.common.unknown;
}

export function resolveCompletedSubtitleOriginalFormat(
  file: CompletedFileEntry,
  text: ReturnType<typeof getXiaText>,
) {
  const format = (file.media?.codec || file.media?.format || file.format || "").trim();
  return format ? format.toUpperCase() : text.common.unknown;
}

export function resolveCompletedFileDetailInfo(
  file: CompletedFileEntry,
  text: ReturnType<typeof getXiaText>,
) {
  const previewKind = resolveCompletedPreviewKind(file);
  const sizeLabel = file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "";

  switch (previewKind) {
    case "video":
      return [
        formatCompletedResolution(file.media?.width, file.media?.height),
        formatCompletedFrameRate(file.media?.frameRate),
        formatCompletedDuration(file.media?.durationMs),
      ].filter(Boolean);
    case "audio":
      return [
        formatCompletedChannels(file.media?.channels),
        formatCompletedDuration(file.media?.durationMs),
      ].filter(Boolean);
    case "image":
      return [
        formatCompletedResolution(file.media?.width, file.media?.height),
        formatCompletedDpi(file.media?.dpi),
      ].filter(Boolean);
    case "subtitle":
      return [
        resolveCompletedSubtitleOriginalFormat(file, text),
        formatCompletedCueCount(file.media?.cueCount, text),
      ].filter(Boolean);
    default:
      return [sizeLabel].filter(Boolean);
  }
}

export function resolveCompletedFileDetailFooterItems(
  file: CompletedFileEntry,
  text: ReturnType<typeof getXiaText>,
) {
  const previewKind = resolveCompletedPreviewKind(file);
  const compactItems = (items: Array<{ label: string; value: string }>) =>
    items.filter((item) => item.value.trim().length > 0);
  const fileFormat = resolveCompletedFileFormatLabel(file, text);
  const codecSummary =
    resolveCompletedCodecSummary(file.media) || text.common.unknown;
  const fileSize = file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "";

  switch (previewKind) {
    case "video":
      return compactItems([
        { label: text.completed.fileFormat, value: fileFormat },
        { label: text.completed.fileSize, value: fileSize },
        { label: text.completed.codec, value: codecSummary },
        {
          label: text.completed.videoBitrate,
          value: formatCompletedBitrate(
            file.media?.videoBitrateKbps ?? file.media?.bitrateKbps,
          ),
        },
        {
          label: text.completed.audioBitrate,
          value: formatCompletedBitrate(file.media?.audioBitrateKbps),
        },
      ]);
    case "audio":
      return compactItems([
        { label: text.completed.fileFormat, value: fileFormat },
        { label: text.completed.fileSize, value: fileSize },
        { label: text.completed.codec, value: codecSummary },
        {
          label: text.completed.bitrate,
          value: formatCompletedBitrate(
            file.media?.audioBitrateKbps ?? file.media?.bitrateKbps,
          ),
        },
      ]);
    case "image":
      return compactItems([
        {
          label: text.completed.resolution,
          value: formatCompletedResolution(file.media?.width, file.media?.height),
        },
        { label: text.completed.fileSize, value: fileSize },
        { label: text.completed.dpi, value: formatCompletedDpi(file.media?.dpi) },
      ]);
    case "subtitle":
      return compactItems([
        {
          label: text.completed.originalFormat,
          value: resolveCompletedSubtitleOriginalFormat(file, text),
        },
        { label: text.completed.fileSize, value: fileSize },
        {
          label: text.completed.lineCount,
          value: formatCompletedCueCount(file.media?.cueCount, text),
        },
      ]);
    default:
      return compactItems([
        { label: text.completed.fileFormat, value: fileFormat },
        { label: text.completed.fileSize, value: fileSize },
      ]);
  }
}

export function resolveCompletedPreviewGroupLabel(
  kind: CompletedPreviewGroupKind,
  text: ReturnType<typeof getXiaText>,
) {
  switch (kind) {
    case "video":
      return text.completed.typeVideo;
    case "audio":
      return text.completed.typeAudio;
    case "subtitle":
      return text.completed.typeSubtitle;
    case "image":
      return text.completed.typeImage;
    case "manifest":
      return text.completed.typeManifest;
    case "api":
      return text.completed.typeApi;
    case "document":
      return text.completed.typeDocument;
    case "font":
      return text.completed.typeFont;
    case "archive":
      return text.completed.typeArchive;
    default:
      return text.completed.typeOther;
  }
}

export function resolveCompletedPreviewTabIcon(kind: CompletedFileType) {
  switch (kind) {
    case "video":
      return FileVideo;
    case "audio":
      return Music2;
    case "subtitle":
      return Languages;
    case "image":
      return ImageIcon;
    case "manifest":
      return FileCode;
    case "api":
      return FileBraces;
    case "document":
      return FileText;
    case "font":
      return FileType;
    case "archive":
      return FileArchive;
    default:
      return Link2;
  }
}

export function resolveCompletedPreviewGroupIcon(kind: CompletedPreviewGroupKind) {
  return resolveCompletedPreviewTabIcon(kind);
}

export function resolveCompletedFileIcon(file: CompletedFileEntry) {
  return resolveCompletedPreviewTabIcon(resolveCompletedFileType(file));
}

export function resolveCompletedImagePreviewURL(file: CompletedFileEntry) {
  if (resolveCompletedPreviewKind(file) !== "image") {
    return "";
  }
  return file.previewURL || DEFAULT_COVER_IMAGE_URL;
}

export function firstCompletedText(...values: Array<string | undefined | null>) {
  for (const value of values) {
    const trimmed = value?.trim() ?? "";
    if (trimmed) {
      return trimmed;
    }
  }
  return "";
}

export function isCompletedThumbnailLibraryFile(
  file?: LibraryDTO["files"][number] | null,
) {
  return Boolean(
    file &&
      !file.state.deleted &&
      !isCompletedLibraryFileUnavailable(file) &&
      (file.kind ?? "").trim().toLowerCase() === "thumbnail" &&
      file.storage.localPath?.trim(),
  );
}

export function buildCompletedCoverLookup(baseURL: string, library: LibraryDTO) {
  const byOperationId = new Map<string, string>();
  const byRootFileId = new Map<string, string>();

  (library.files ?? []).forEach((file) => {
    if (!isCompletedThumbnailLibraryFile(file)) {
      return;
    }
    const localPath = file.storage.localPath?.trim() ?? "";
    const coverURL = buildAssetPreviewURL(baseURL, localPath);
    if (!coverURL) {
      return;
    }

    [file.latestOperationId, file.origin.operationId]
      .map((value) => value?.trim() ?? "")
      .filter(Boolean)
      .forEach((key) => {
        if (!byOperationId.has(key)) {
          byOperationId.set(key, coverURL);
        }
      });

    [file.lineage.rootFileId, file.id]
      .map((value) => value?.trim() ?? "")
      .filter(Boolean)
      .forEach((key) => {
        if (!byRootFileId.has(key)) {
          byRootFileId.set(key, coverURL);
        }
      });
  });

  return { byOperationId, byRootFileId };
}

export function resolveCompletedLibraryFileCoverURL(
  baseURL: string,
  library: LibraryDTO,
  file: LibraryDTO["files"][number],
  coverLookup: ReturnType<typeof buildCompletedCoverLookup>,
) {
  return (
    resolveCompletedLibraryFileExplicitCoverURL(
      baseURL,
      library,
      file,
      coverLookup,
    ) ||
    resolveLibraryCoverURL(baseURL, library) ||
    DEFAULT_COVER_IMAGE_URL
  );
}

export function resolveCompletedLibraryFileExplicitCoverURL(
  _baseURL: string,
  _library: LibraryDTO,
  file: LibraryDTO["files"][number],
  coverLookup: ReturnType<typeof buildCompletedCoverLookup>,
) {
  const operationKeys = [file.latestOperationId, file.origin.operationId]
    .map((value) => value?.trim() ?? "")
    .filter(Boolean);
  const rootKeys = [file.lineage.rootFileId, file.id]
    .map((value) => value?.trim() ?? "")
    .filter(Boolean);

  return (
    operationKeys.map((key) => coverLookup.byOperationId.get(key)).find(Boolean) ||
    rootKeys.map((key) => coverLookup.byRootFileId.get(key)).find(Boolean) ||
    ""
  );
}

export function resolveCompletedOperationCoverURL(
  baseURL: string,
  operation: OperationListItemDTO,
  library: LibraryDTO | null,
) {
  return (
    resolveCompletedOperationExplicitCoverURL(baseURL, operation, library) ||
    (library ? resolveLibraryCoverURL(baseURL, library) : "") ||
    DEFAULT_COVER_IMAGE_URL
  );
}

export function resolveCompletedOperationExplicitCoverURL(
  baseURL: string,
  operation: OperationListItemDTO,
  library: LibraryDTO | null,
) {
  const operationId = operation.operationId.trim();
  return library && operationId
    ? buildCompletedCoverLookup(baseURL, library).byOperationId.get(
        operationId,
      ) || ""
    : "";
}

export function formatCodecLabel(codec?: string) {
  const normalized = (codec ?? "").trim().toLowerCase();
  if (normalized.includes("mp4a")) {
    return "AAC";
  }
  switch (normalized) {
    case "h264":
      return "H.264";
    case "h265":
      return "H.265";
    case "vp9":
      return "VP9";
    case "aac":
      return "AAC";
    case "mp3":
      return "MP3";
    case "opus":
      return "Opus";
    case "flac":
      return "FLAC";
    case "pcm":
      return "PCM";
    case "text":
      return "Copy";
    default:
      return (codec ?? "").trim().toUpperCase();
  }
}

export function resolveSiteKeyForDomain(domain?: string) {
  const normalized = (domain ?? "").trim().toLowerCase();
  switch (normalized) {
    case "youtube.com":
    case "youtu.be":
    case "youtube-nocookie.com":
      return "youtube";
    case "bilibili.com":
    case "b23.tv":
      return "bilibili";
    case "tiktok.com":
    case "tiktokv.com":
    case "vm.tiktok.com":
      return "tiktok";
    case "douyin.com":
    case "iesdouyin.com":
    case "xiaohongshu.com":
    case "rednote.com":
    case "xhs.cn":
    case "xhslink.com":
    case "xhslink.cn":
    case "xhsurl.com":
    case "rl.ink":
      return "china_private";
    case "instagram.com":
      return "instagram";
    case "x.com":
    case "twitter.com":
      return "x";
    case "facebook.com":
    case "fb.watch":
      return "facebook";
    case "vimeo.com":
    case "player.vimeo.com":
      return "vimeo";
    case "twitch.tv":
    case "clips.twitch.tv":
      return "twitch";
    case "nicovideo.jp":
    case "nico.ms":
    case "nicovideo.cdn.nimg.jp":
      return "niconico";
    default:
      return "";
  }
}

type ParsedAppErrorMessage = {
  code: string;
  message: string;
  structured: boolean;
};

function isErrorRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

function errorStringField(
  record: Record<string, unknown>,
  keys: string[],
): string {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return "";
}

function parseJSONErrorRecord(message: string): Record<string, unknown> | null {
  const trimmed = message.trim();
  const candidates = [trimmed];
  const objectStart = trimmed.indexOf("{");
  const objectEnd = trimmed.lastIndexOf("}");
  if (objectStart >= 0 && objectEnd > objectStart) {
    candidates.push(trimmed.slice(objectStart, objectEnd + 1));
  }

  for (const candidate of candidates) {
    if (!candidate.startsWith("{") || !candidate.endsWith("}")) {
      continue;
    }
    try {
      const parsed = JSON.parse(candidate) as unknown;
      if (isErrorRecord(parsed)) {
        return parsed;
      }
    } catch {
      // Try the next JSON-looking candidate.
    }
  }
  return null;
}

function errorRecordField(
  record: Record<string, unknown>,
  keys: string[],
): Record<string, unknown> | null {
  for (const key of keys) {
    const value = record[key];
    if (isErrorRecord(value)) {
      return value;
    }
  }
  return null;
}

function parseAppErrorRecord(
  record: Record<string, unknown>,
): ParsedAppErrorMessage {
  const code = errorStringField(record, [
    "code",
    "errorCode",
    "Code",
    "ErrorCode",
  ]);
  const message = errorStringField(record, [
    "message",
    "errorMessage",
    "detail",
    "error",
    "Message",
    "ErrorMessage",
    "Detail",
    "Error",
  ]);
  const parsedMessage = message ? parseAppErrorMessageParts(message) : null;
  const nestedRecord = errorRecordField(record, ["error", "cause", "err"]);
  const nested = nestedRecord ? parseAppErrorRecord(nestedRecord) : null;
  return {
    code: code || parsedMessage?.code || nested?.code || "",
    message: parsedMessage?.message || message || nested?.message || "",
    structured: true,
  };
}

function parseAppErrorMessageParts(message: string): ParsedAppErrorMessage {
  const trimmed = message.trim();
  const jsonRecord = parseJSONErrorRecord(trimmed);
  if (jsonRecord) {
    return parseAppErrorRecord(jsonRecord);
  }
  const match = trimmed.match(/\[([a-z0-9_.-]+)]\s*(.*)$/i);
  if (!match) {
    return { code: "", message: trimmed, structured: false };
  }
  return {
    code: match[1]?.trim() ?? "",
    message: match[2]?.trim() ?? "",
    structured: true,
  };
}

function extractAppErrorMessage(error: unknown): ParsedAppErrorMessage {
  if (error instanceof Error) {
    const parsed = parseAppErrorMessageParts(error.message);
    if (parsed.structured) {
      return parsed;
    }
    const cause = (error as Error & { cause?: unknown }).cause;
    const parsedCause = cause === undefined ? null : extractAppErrorMessage(cause);
    if (parsedCause && (parsedCause.code || parsedCause.message)) {
      return {
        code: parsedCause.code || parsed.code,
        message: parsedCause.message || parsed.message,
        structured: parsedCause.structured,
      };
    }
    return parsed;
  }
  if (typeof error === "string") {
    return parseAppErrorMessageParts(error);
  }
  if (isErrorRecord(error)) {
    return parseAppErrorRecord(error);
  }
  return { code: "", message: "", structured: false };
}

export function resolveUnknownErrorMessage(error: unknown, fallback: string) {
  const parsed = extractAppErrorMessage(error);
  return parsed.message || fallback;
}

export function parseAppErrorMessage(message: string) {
  const parsed = parseAppErrorMessageParts(message);
  return { code: parsed.code, message: parsed.message };
}

export function getAppErrorCode(error: unknown) {
  return extractAppErrorMessage(error).code;
}
