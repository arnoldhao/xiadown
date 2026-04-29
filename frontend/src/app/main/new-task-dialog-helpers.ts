
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

import { AUDIO_FILE_EXTENSIONS,CONNECTOR_TYPES,VIDEO_FILE_EXTENSIONS,formatCodecLabel,resolveConnectorTypeForDomain } from "@/app/main/helpers";
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
    const bestSize = currentBest.filesize ?? 0;
    const currentSize = current.filesize ?? 0;
    return currentSize > bestSize ? current : currentBest;
  });
  return best.id;
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

export function normalizeConnectorType(value?: string) {
  const normalized = (value ?? "")
    .trim()
    .toLowerCase()
    .replace(/^connector-/, "");
  return CONNECTOR_TYPES.has(normalized) ? normalized : "";
}

export function resolvePreparedConnectorType(
  prepared: PrepareYTDLPDownloadResponse | null,
) {
  return (
    normalizeConnectorType(prepared?.connectorId) ||
    resolveConnectorTypeForDomain(prepared?.domain)
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
