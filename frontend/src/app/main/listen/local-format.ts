import { AUDIO_MIME_BY_EXTENSION } from "@/app/main/listen/catalog";
import type { ListenLocalItem } from "@/app/main/listen/types";
import { extractExtensionFromPath } from "@/shared/utils/resourceHelpers";

export type ListenLocalPlaybackUnsupportedReason =
  | ""
  | "unsupported-container"
  | "unsupported-codec"
  | "unknown-format"
  | "webview-rejected";

export type ListenLocalPlaybackCapability = {
  supported: boolean;
  unsupportedReason: ListenLocalPlaybackUnsupportedReason;
  mimeType: string;
};

export type ListenLocalCanPlayType = (
  mimeType: string,
) => "" | "maybe" | "probably";

type ListenLocalPlaybackDescriptor = {
  path?: string;
  format?: string;
  audioCodec?: string;
};

type ListenLocalPlaybackCapabilityOptions = {
  canPlayType?: ListenLocalCanPlayType | null;
};

const HARD_UNSUPPORTED_EXTENSIONS = new Set(["ape", "wma"]);
const HARD_UNSUPPORTED_CODECS = new Set([
  "ape",
  "wmav1",
  "wmav2",
  "wmapro",
  "wmalossless",
  "wmavoice",
]);
const KNOWN_PLAYABLE_CODECS = new Set([
  "aac",
  "alac",
  "flac",
  "mp3",
  "opus",
  "vorbis",
]);
const DETERMINISTIC_FALLBACK_EXTENSIONS = new Set([
  "aac",
  "flac",
  "m4a",
  "m4b",
  "mp3",
  "mp4",
  "mpga",
  "oga",
  "ogg",
  "opus",
  "wav",
  "wave",
  "weba",
  "webm",
]);
let runtimeAudioElement: HTMLAudioElement | null = null;
const runtimeCanPlayTypeCache = new Map<string, "" | "maybe" | "probably">();
const cachedRuntimeCanPlayType: ListenLocalCanPlayType = (mimeType) => {
  const cached = runtimeCanPlayTypeCache.get(mimeType);
  if (cached !== undefined) {
    return cached;
  }
  const result = runtimeAudioElement?.canPlayType(mimeType) ?? "";
  runtimeCanPlayTypeCache.set(mimeType, result);
  return result;
};

/**
 * Resolve the capability of the actual desktop playback engine.
 *
 * XiaDown's native local transport is a hidden HTMLMediaElement hosted by the
 * same WebView engine as the main window. Static rules reject formats known to
 * be unreliable (APE/WMA), while canPlayType decides platform/version-specific
 * containers such as Ogg, WebM, AIFF and CAF at runtime. Tests and non-DOM
 * callers use a conservative deterministic fallback.
 */
export function resolveListenLocalPlaybackCapability(
  descriptor: ListenLocalPlaybackDescriptor,
  options: ListenLocalPlaybackCapabilityOptions = {},
): ListenLocalPlaybackCapability {
  const extension = extractExtensionFromPath(descriptor.path ?? "").toLowerCase();
  const codec = normalizeAudioCodec(descriptor.audioCodec);

  if (HARD_UNSUPPORTED_EXTENSIONS.has(extension)) {
    return unsupported("unsupported-container");
  }
  if (HARD_UNSUPPORTED_CODECS.has(codec)) {
    return unsupported("unsupported-codec");
  }
  if (codec && !isKnownPlayableCodec(codec)) {
    return unsupported("unsupported-codec");
  }

  const candidates = buildPlaybackMIMECandidates({
    extension,
    format: descriptor.format ?? "",
    codec,
  });
  if (candidates.length === 0) {
    return unsupported("unknown-format");
  }

  const canPlayType = Object.prototype.hasOwnProperty.call(options, "canPlayType")
    ? options.canPlayType ?? null
    : runtimeCanPlayType();
  if (canPlayType) {
    for (const mimeType of candidates) {
      try {
        if (canPlayType(mimeType) !== "") {
          return supported(mimeType);
        }
      } catch {
        // A broken capability probe is equivalent to the WebView rejecting it.
      }
    }
    return {
      ...unsupported("webview-rejected"),
      mimeType: candidates[0] ?? "",
    };
  }

  if (
    DETERMINISTIC_FALLBACK_EXTENSIONS.has(extension) ||
    isKnownPlayableCodec(codec)
  ) {
    return supported(candidates[0] ?? "");
  }
  return {
    ...unsupported("webview-rejected"),
    mimeType: candidates[0] ?? "",
  };
}

export function resolveListenLocalPlayableQueue(
  tracks: readonly ListenLocalItem[],
): ListenLocalItem[] {
  return tracks.filter((track) => track.playbackSupported);
}

function runtimeCanPlayType(): ListenLocalCanPlayType | null {
  if (typeof document === "undefined") {
    return null;
  }
  runtimeAudioElement ??= document.createElement("audio");
  if (typeof runtimeAudioElement.canPlayType !== "function") {
    return null;
  }
  return cachedRuntimeCanPlayType;
}

function buildPlaybackMIMECandidates(input: {
  extension: string;
  format: string;
  codec: string;
}) {
  const result: string[] = [];
  const add = (value: string) => {
    const normalized = value.trim();
    if (normalized && !result.includes(normalized)) {
      result.push(normalized);
    }
  };

  const containerMIME =
    AUDIO_MIME_BY_EXTENSION[input.extension] ||
    mimeTypeFromProbeFormat(input.format, input.codec);
  const codecMIME = mimeTypeForCodec(input.codec, input.extension, containerMIME);
  if (input.codec && !codecMIME) {
    return result;
  }
  add(codecMIME);
  add(containerMIME);
  if (!containerMIME) {
    add(mimeTypeForCodec(input.codec, input.extension, ""));
  }
  return result;
}

function mimeTypeForCodec(codec: string, extension: string, containerMIME: string) {
  switch (codec) {
    case "aac":
      if (extension === "aac" || containerMIME === "audio/aac" || !containerMIME) {
        return "audio/aac";
      }
      if (containerMIME === "audio/mp4") {
        return 'audio/mp4; codecs="mp4a.40.2"';
      }
      if (containerMIME === "audio/3gpp") {
        return 'audio/3gpp; codecs="mp4a.40.2"';
      }
      if (containerMIME === "audio/x-caf") {
        return "audio/x-caf";
      }
      return "";
    case "alac":
      if (containerMIME === "audio/mp4") {
        return 'audio/mp4; codecs="alac"';
      }
      return containerMIME === "audio/x-caf" ? containerMIME : "";
    case "flac":
      if (!containerMIME || containerMIME === "audio/flac") {
        return "audio/flac";
      }
      if (containerMIME === "audio/ogg") {
        return 'audio/ogg; codecs="flac"';
      }
      return "";
    case "mp3":
      if (!containerMIME || containerMIME === "audio/mpeg") {
        return "audio/mpeg";
      }
      return containerMIME === "audio/mp4"
        ? 'audio/mp4; codecs="mp3"'
        : "";
    case "opus":
      if (containerMIME === "audio/webm") {
        return 'audio/webm; codecs="opus"';
      }
      if (containerMIME === "audio/mp4") {
        return 'audio/mp4; codecs="opus"';
      }
      return !containerMIME || containerMIME === "audio/ogg"
        ? 'audio/ogg; codecs="opus"'
        : "";
    case "vorbis":
      if (containerMIME === "audio/webm") {
        return 'audio/webm; codecs="vorbis"';
      }
      return !containerMIME || containerMIME === "audio/ogg"
        ? 'audio/ogg; codecs="vorbis"'
        : "";
    default:
      if (codec.startsWith("pcm_")) {
        switch (containerMIME) {
          case "audio/wav":
          case "audio/aiff":
          case "audio/x-caf":
            return containerMIME;
          default:
            return extension === "wav" || extension === "wave"
              ? "audio/wav"
              : "";
        }
      }
      return "";
  }
}

function mimeTypeFromProbeFormat(format: string, codec: string) {
  const tokens = new Set(
    format
      .toLowerCase()
      .split(/[^a-z0-9]+/)
      .filter(Boolean),
  );
  if (tokens.has("flac")) return "audio/flac";
  if (tokens.has("mp3") || tokens.has("mpeg")) return "audio/mpeg";
  if (tokens.has("wav")) return "audio/wav";
  if (tokens.has("aiff")) return "audio/aiff";
  if (tokens.has("caf")) return "audio/x-caf";
  if (tokens.has("ogg")) return "audio/ogg";
  if (tokens.has("webm")) return "audio/webm";
  if (tokens.has("mp4") || tokens.has("m4a") || tokens.has("mov")) {
    return "audio/mp4";
  }
  return mimeTypeForCodec(codec, "", "");
}

function normalizeAudioCodec(value?: string) {
  return (value ?? "")
    .trim()
    .toLowerCase()
    .split(/[\s,;/]+/, 1)[0] ?? "";
}

function isKnownPlayableCodec(codec: string) {
  return KNOWN_PLAYABLE_CODECS.has(codec) || codec.startsWith("pcm_");
}

function supported(mimeType: string): ListenLocalPlaybackCapability {
  return { supported: true, unsupportedReason: "", mimeType };
}

function unsupported(
  unsupportedReason: Exclude<ListenLocalPlaybackUnsupportedReason, "">,
): ListenLocalPlaybackCapability {
  return { supported: false, unsupportedReason, mimeType: "" };
}
