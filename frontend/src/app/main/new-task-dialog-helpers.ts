
import {
getXiaText
} from "@/features/xiadown/shared";
import type {
PrepareYTDLPDownloadResponse,
TranscodePreset,
YTDLPFormatOption,
YTDLPSubtitleOption
} from "@/shared/contracts/library";
import {
getPathBaseName,
stripPathExtension
} from "@/shared/utils/resourceHelpers";
import { formatBytes } from "@/shared/utils/formatBytes";

import { AUDIO_FILE_EXTENSIONS,SITE_KEYS,VIDEO_FILE_EXTENSIONS,formatCodecLabel,resolveSiteKeyForDomain } from "@/app/main/helpers";
import type { SelectOption,SourceMediaType } from "@/app/main/types";

export function resolveFormatMediaType(
  format: YTDLPFormatOption | null | undefined,
): SourceMediaType {
  return format?.hasVideo ? "video" : "audio";
}

export function pickDefaultFormat(formats: YTDLPFormatOption[]) {
  if (!formats || formats.length === 0) {
    return null;
  }
  const videoFormats = formats.filter((format) => format.hasVideo);
  if (videoFormats.length > 0) {
    return videoFormats.reduce((best, current) => {
      const bestHeight = best.height ?? 0;
      const currentHeight = current.height ?? 0;
      if (currentHeight !== bestHeight) {
        return currentHeight > bestHeight ? current : best;
      }
      const bestSize = best.filesize ?? 0;
      const currentSize = current.filesize ?? 0;
      return currentSize > bestSize ? current : best;
    });
  }
  const audioFormats = formats.filter((format) => format.hasAudio);
  if (audioFormats.length > 0) {
    return audioFormats.reduce((best, current) => {
      const bestSize = best.filesize ?? 0;
      const currentSize = current.filesize ?? 0;
      return currentSize > bestSize ? current : best;
    });
  }
  return formats[0];
}

export function selectAudioFormatId(formats: YTDLPFormatOption[]) {
  const audioFormats = (formats ?? []).filter(
    (format) => format.hasAudio && !format.hasVideo,
  );
  if (audioFormats.length === 0) {
    return "";
  }
  const best = audioFormats.reduce((currentBest, current) => {
    const bestBitrate = audioFormatBitrateScore(currentBest);
    const currentBitrate = audioFormatBitrateScore(current);
    if (currentBitrate !== bestBitrate) {
      return currentBitrate > bestBitrate ? current : currentBest;
    }
    const bestSize = currentBest.filesize ?? 0;
    const currentSize = current.filesize ?? 0;
    return currentSize > bestSize ? current : currentBest;
  });
  return best.id;
}

function audioFormatBitrateScore(format: YTDLPFormatOption) {
  for (const value of [format.abr, format.tbr]) {
    if (value && Number.isFinite(value) && value > 0) {
      return value;
    }
  }
  return 0;
}

function formatKbps(value?: number) {
  if (!value || !Number.isFinite(value) || value <= 0) {
    return "";
  }
  return `${Math.round(value)}k`;
}

export function formatAudioTrackLabel(format: YTDLPFormatOption) {
  const parts: string[] = [];
  const language = format.language?.trim();
  if (language) {
    parts.push(language);
  }
  const note = format.formatNote?.trim();
  if (note && !parts.some((part) => part.toLowerCase() === note.toLowerCase())) {
    parts.push(note);
  }
  const bitrate = formatKbps(audioFormatBitrateScore(format));
  if (bitrate && !parts.some((part) => part.includes(bitrate))) {
    parts.push(bitrate);
  }
  if (format.audioChannels && format.audioChannels > 0) {
    parts.push(`${format.audioChannels}ch`);
  }
  if (format.ext) {
    parts.push(format.ext);
  }
  const codec = format.acodec ? formatCodecLabel(format.acodec) : "";
  if (codec) {
    parts.push(codec);
  }
  const size = format.filesize && format.filesize > 0 ? formatBytes(format.filesize) : "";
  if (size && size !== "-") {
    parts.push(size);
  }
  if (parts.length > 0) {
    return parts.join(" · ");
  }
  return format.label || format.id;
}

export function formatSubtitleLabel(subtitle: YTDLPSubtitleOption) {
  return [
    subtitle.name || subtitle.language,
    subtitle.isAuto ? "auto" : "",
    subtitle.ext,
  ]
    .filter(Boolean)
    .join(" · ");
}

export function inferMediaTypeFromPath(path: string): SourceMediaType | null {
  const extension = path.split(".").pop()?.trim().toLowerCase() ?? "";
  if (!extension) {
    return null;
  }
  if (AUDIO_FILE_EXTENSIONS.has(extension)) {
    return "audio";
  }
  if (VIDEO_FILE_EXTENSIONS.has(extension)) {
    return "video";
  }
  return null;
}

export function filterTranscodePresetsForMediaType(
  presets: TranscodePreset[],
  mediaType: SourceMediaType | null,
) {
  if (!mediaType) {
    return presets;
  }
  return presets.filter((preset) =>
    mediaType === "audio"
      ? preset.outputType === "audio"
      : preset.outputType !== "audio",
  );
}

export function pickDefaultTranscodePreset(
  presets: TranscodePreset[],
  mediaType: SourceMediaType | null,
) {
  return filterTranscodePresetsForMediaType(presets, mediaType)[0] ?? null;
}

export function uniqueOptions(options: SelectOption[]) {
  const seen = new Set<string>();
  return options.filter((option) => {
    if (!option.value || seen.has(option.value)) {
      return false;
    }
    seen.add(option.value);
    return true;
  });
}

export function resolveTranscodeScaleValue(preset: TranscodePreset) {
  return preset.scale?.trim() || "original";
}

export function buildTranscodeCodecKey(preset: TranscodePreset) {
  if (preset.outputType === "audio") {
    return [
      "audio",
      preset.audioCodec || "auto",
      preset.audioBitrateKbps || 0,
    ].join(":");
  }
  return [
    "video",
    preset.videoCodec || "h264",
    preset.audioCodec || "auto",
  ].join(":");
}

export function resolveTranscodeCodecLabel(preset: TranscodePreset) {
  if (preset.outputType === "audio") {
    const codec = formatCodecLabel(preset.audioCodec || preset.container);
    return preset.audioBitrateKbps
      ? `${codec} ${preset.audioBitrateKbps}k`
      : codec;
  }
  const videoCodec = formatCodecLabel(preset.videoCodec || "h264");
  const audioCodec = preset.audioCodec
    ? formatCodecLabel(preset.audioCodec)
    : "";
  return audioCodec ? `${videoCodec} / ${audioCodec}` : videoCodec;
}

export function resolveTranscodeScaleLabel(
  value: string,
  text: ReturnType<typeof getXiaText>,
) {
  switch (value.trim().toLowerCase()) {
    case "":
    case "original":
      return text.dialogs.scaleOriginal;
    case "custom":
      return text.dialogs.scaleCustom;
    default:
      return value;
  }
}

export function applyTranscodePresetSelection(
  preset: TranscodePreset | null | undefined,
  setters: {
    setScale: (value: string) => void;
    setContainer: (value: string) => void;
    setCodec: (value: string) => void;
  },
) {
  setters.setScale(preset ? resolveTranscodeScaleValue(preset) : "");
  setters.setContainer(preset?.container ?? "");
  setters.setCodec(preset ? buildTranscodeCodecKey(preset) : "");
}

export function normalizeSiteKey(value?: string) {
  const normalized = (value ?? "")
    .trim()
    .toLowerCase()
    .replace(/^site-app-session-/, "")
    .replace(/^connector-/, "");
  return SITE_KEYS.has(normalized) ? normalized : "";
}

export function resolvePreparedSiteKey(
  prepared: PrepareYTDLPDownloadResponse | null,
) {
  return (
    normalizeSiteKey(prepared?.appSessionId) ||
    resolveSiteKeyForDomain(prepared?.domain)
  );
}

export function splitFileNameForDisplay(path: string) {
  const fileName = getPathBaseName(path) || path;
  const dotIndex = fileName.lastIndexOf(".");
  if (dotIndex <= 0 || dotIndex >= fileName.length - 1) {
    return { stem: fileName, extension: "" };
  }
  return {
    stem: fileName.slice(0, dotIndex),
    extension: fileName.slice(dotIndex),
  };
}

export function resolveFileFormatLabel(path: string) {
  const extension = splitFileNameForDisplay(path)
    .extension.replace(/^\./, "")
    .trim()
    .toUpperCase();
  if (extension) {
    return extension.length > 5 ? extension.slice(0, 5) : extension;
  }
  const mediaType = inferMediaTypeFromPath(path);
  return mediaType === "audio"
    ? "AUDIO"
    : mediaType === "video"
      ? "VIDEO"
      : "FILE";
}

export function resolveOpenFileName(path: string) {
  const baseName = getPathBaseName(path);
  return stripPathExtension(baseName);
}

export type ResourceSniffStartResolution =
  | "attach"
  | "cancel"
  | "preserve";

export function resolveResourceSniffStartResolution(input: {
  requestVersion: number;
  currentVersion: number;
  dialogOpen: boolean;
  transferRequestVersion: number | null | undefined;
}): ResourceSniffStartResolution {
  if (input.transferRequestVersion === input.requestVersion) {
    return "preserve";
  }
  if (input.requestVersion !== input.currentVersion || !input.dialogOpen) {
    return "cancel";
  }
  return "attach";
}
