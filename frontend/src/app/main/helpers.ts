
import {
getXiaText,
resolveLibraryCoverURL,
} from "@/features/xiadown/shared";
import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
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
FileVideo,
ImageIcon,
Languages,
Link2,
Music2,
} from "lucide-react";

import { COMPLETED_PREVIEW_SUPPORT_CACHE } from "@/app/main/main-constants";
import type { CompletedDeleteConfirmation,CompletedFileEntry,CompletedFileType,CompletedPreviewGroupKind,CompletedViewMode } from "@/app/main/types";

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
export const CONNECTOR_TYPES = new Set([
  "youtube",
  "bilibili",
  "tiktok",
  "douyin",
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
  if (normalizedKind === "video") {
    return "video" as const;
  }
  if (normalizedKind === "audio") {
    return "audio" as const;
  }
  if (normalizedKind === "subtitle") {
    return "subtitle" as const;
  }
  if (normalizedKind === "image" || normalizedKind === "thumbnail") {
    return "image" as const;
  }

  const extension = (extractExtensionFromPath(file.path) || file.format || "")
    .trim()
    .toLowerCase();
  if (AUDIO_FILE_EXTENSIONS.has(extension)) {
    return "audio" as const;
  }
  if (VIDEO_FILE_EXTENSIONS.has(extension)) {
    return "video" as const;
  }
  if (IMAGE_FILE_EXTENSIONS.has(extension)) {
    return "image" as const;
  }
  return "other" as const;
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
    "id" | "kind" | "path" | "format" | "previewURL" | "media"
  >,
) {
  const previewKind = resolveCompletedPreviewKind(file);
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
  file: Pick<CompletedFileEntry, "kind" | "path" | "format">,
): CompletedFileType {
  const previewKind = resolveCompletedPreviewKind(file);
  if (
    previewKind === "video" ||
    previewKind === "audio" ||
    previewKind === "image" ||
    previewKind === "subtitle"
  ) {
    return previewKind;
  }
  const normalizedKind = (file.kind ?? "").trim().toLowerCase();
  if (normalizedKind === "subtitle") {
    return "subtitle";
  }
  return "other";
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
    default:
      return text.completed.typeOther;
  }
}

export function resolveCompletedPreviewGroupKind(
  file: Pick<CompletedFileEntry, "kind" | "path" | "format">,
): CompletedPreviewGroupKind {
  const type = resolveCompletedFileType(file);
  if (type === "video" || type === "audio") {
    return "media";
  }
  if (type === "subtitle" || type === "image") {
    return type;
  }
  return "other";
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
  const isChinese = getLanguage() === "zh-CN";
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
  const isChinese = getLanguage() === "zh-CN";
  return isChinese
    ? `${text.completed.perPage}${pageSize}${text.completed.itemUnit}`
    : `${text.completed.perPage} ${pageSize} ${text.completed.itemUnit}`;
}

export function resolveCompletedPageLabel(
  page: number,
  pageCount: number,
  text: ReturnType<typeof getXiaText>,
) {
  const isChinese = getLanguage() === "zh-CN";
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
  return (file.format || file.media?.format || text.common.unknown)
    .trim()
    .toUpperCase();
}

export function resolveCompletedSubtitleOriginalFormat(
  file: CompletedFileEntry,
  text: ReturnType<typeof getXiaText>,
) {
  return (
    file.media?.codec ||
    file.media?.format ||
    file.format ||
    text.common.unknown
  )
    .trim()
    .toUpperCase();
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

export function resolveCompletedFileDetailFooterMeta(
  file: CompletedFileEntry,
  text: ReturnType<typeof getXiaText>,
) {
  const previewKind = resolveCompletedPreviewKind(file);

  switch (previewKind) {
    case "video":
      return [
        file.media?.videoCodec ? formatCodecLabel(file.media.videoCodec) : "",
        formatCompletedBitrate(
          file.media?.videoBitrateKbps ?? file.media?.bitrateKbps,
        ),
        formatCompletedBitrate(file.media?.audioBitrateKbps),
        file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "",
      ].filter(Boolean);
    case "audio":
      return [
        formatCompletedBitrate(
          file.media?.audioBitrateKbps ?? file.media?.bitrateKbps,
        ),
        file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "",
      ].filter(Boolean);
    case "image":
      return [
        formatCompletedResolution(file.media?.width, file.media?.height),
        formatCompletedDpi(file.media?.dpi),
        file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "",
      ].filter(Boolean);
    case "subtitle":
      return [
        resolveCompletedSubtitleOriginalFormat(file, text),
        formatCompletedCueCount(file.media?.cueCount, text),
        file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "",
      ].filter(Boolean);
    default:
      return [
        resolveCompletedCodecSummary(file.media),
        file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "",
      ].filter(Boolean);
  }
}

export function resolveCompletedFileFooterTooltipLabels(
  previewKind: CompletedFileType,
  text: ReturnType<typeof getXiaText>,
) {
  switch (previewKind) {
    case "video":
      return [
        text.completed.codec,
        text.completed.videoBitrate,
        text.completed.audioBitrate,
        text.completed.fileSize,
      ];
    case "audio":
      return [text.completed.bitrate, text.completed.fileSize];
    case "image":
      return [
        text.completed.resolution,
        text.completed.dpi,
        text.completed.fileSize,
      ];
    case "subtitle":
      return [
        text.completed.originalFormat,
        text.completed.lineCount,
        text.completed.fileSize,
      ];
    default:
      return [text.completed.info, text.completed.fileSize];
  }
}

export function resolveCompletedPreviewGroupLabel(
  kind: CompletedPreviewGroupKind,
  text: ReturnType<typeof getXiaText>,
) {
  switch (kind) {
    case "media":
      return text.completed.videoCount;
    case "subtitle":
      return text.completed.typeSubtitle;
    case "image":
      return text.completed.typeImage;
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
    default:
      return Link2;
  }
}

export function resolveCompletedPreviewGroupIcon(kind: CompletedPreviewGroupKind) {
  switch (kind) {
    case "media":
      return FileVideo;
    case "subtitle":
      return Languages;
    case "image":
      return ImageIcon;
    default:
      return Link2;
  }
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
  const operationKeys = [file.latestOperationId, file.origin.operationId]
    .map((value) => value?.trim() ?? "")
    .filter(Boolean);
  const rootKeys = [file.lineage.rootFileId, file.id]
    .map((value) => value?.trim() ?? "")
    .filter(Boolean);

  return (
    operationKeys.map((key) => coverLookup.byOperationId.get(key)).find(Boolean) ||
    rootKeys.map((key) => coverLookup.byRootFileId.get(key)).find(Boolean) ||
    resolveLibraryCoverURL(baseURL, library) ||
    DEFAULT_COVER_IMAGE_URL
  );
}

export function resolveCompletedOperationCoverURL(
  baseURL: string,
  operation: OperationListItemDTO,
  library: LibraryDTO | null,
) {
  const operationId = operation.operationId.trim();
  const outputCoverURL =
    library && operationId
      ? buildCompletedCoverLookup(baseURL, library).byOperationId.get(
          operationId,
        )
      : "";

  return (
    outputCoverURL ||
    (library ? resolveLibraryCoverURL(baseURL, library) : "") ||
    DEFAULT_COVER_IMAGE_URL
  );
}

export function formatCodecLabel(codec?: string) {
  switch ((codec ?? "").trim().toLowerCase()) {
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

export function resolveConnectorTypeForDomain(domain?: string) {
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
      return "douyin";
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

export function resolveUnknownErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  if (typeof error === "string" && error.trim()) {
    return error;
  }
  return fallback;
}
