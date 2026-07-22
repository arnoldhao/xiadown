import { LISTEN_LYRICS_SERVICE } from "@/app/main/listen/catalog";
import { buildListenLyricsChineseSearchTrackVariants } from "@/app/main/listen/lyrics-chinese";
import { loadListenLyricsCached } from "@/app/main/listen/playback-helpers";
import type {
  ListenLyricsCandidate,
  ListenLyricsData,
  ListenLyricsKind,
  ListenLyricTimingQuality,
  ListenLyricWord,
  ListenOnlineItem,
} from "@/app/main/listen/types";
import { listenPlaybackTrackFromOnlineItem } from "@/app/main/listen/playback-api";

export type ListenLyricsSnapshot = ListenLyricsData & {
  loading?: boolean;
  error?: string;
  errorCode?: string;
  retryable?: boolean;
  activeProvider?: string;
};

export type ListenLyricsCandidateTrack = {
  lyricsId?: string;
  videoId?: string;
  title: string;
  artist?: string;
  channel?: string;
  album?: string;
  localPath?: string;
  durationSeconds?: number;
};

export function resolveListenLyricsOnlineArtist(
  track: Pick<ListenOnlineItem, "channel" | "artists">,
) {
  const seen = new Set<string>();
  const linkedArtists = (track.artists ?? [])
    .map((artist) => artist.name.trim())
    .filter((artist) => {
      const key = artist.toLocaleLowerCase();
      if (!artist || seen.has(key)) {
        return false;
      }
      seen.add(key);
      return true;
    });
  return linkedArtists.join(", ") || track.channel.trim();
}

export function normalizeListenLyricsSnapshot(
  value: unknown,
): ListenLyricsSnapshot | null {
  const payload = ((value as { data?: unknown })?.data ?? value) as
    | Partial<ListenLyricsSnapshot>
    | null;
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const lines = Array.isArray(payload.lines) ? payload.lines : [];
  return {
    videoId: String(payload.videoId ?? "").trim(),
    kind: normalizeListenLyricsKind(payload.kind),
    source: String(payload.source ?? "").trim(),
    providerId: String(payload.providerId ?? "").trim() || undefined,
    providerTrackId: String(payload.providerTrackId ?? "").trim() || undefined,
    attribution: String(payload.attribution ?? "").trim() || undefined,
    timingQuality: normalizeListenLyricTimingQuality(payload.timingQuality),
    confidence: normalizeListenLyricConfidence(payload.confidence),
    text: String(payload.text ?? ""),
    lines: lines.map((line) => ({
      startMs: Math.max(0, Number(line.startMs ?? 0)),
      durationMs: Math.max(0, Number(line.durationMs ?? 0)),
      endEstimated: line.endEstimated === true || undefined,
      text: String(line.text ?? ""),
      translationText: line.translationText?.trim() || undefined,
      romanizedText: line.romanizedText?.trim() || undefined,
      romanizedKind:
        line.romanizedKind === "romanized" || line.romanizedKind === "pinyin"
          ? line.romanizedKind
          : undefined,
      alternateTexts: Array.isArray(line.alternateTexts)
        ? line.alternateTexts
            .map((alternate) => ({
              role: String(alternate.role ?? "").trim(),
              language: String(alternate.language ?? "").trim() || undefined,
              text: String(alternate.text ?? "").trim(),
            }))
            .filter((alternate) => alternate.role && alternate.text)
        : undefined,
      words: normalizeListenLyricWords(line.words),
    })),
    loading: payload.loading === true,
    error: String(payload.error ?? "").trim(),
    errorCode: String(payload.errorCode ?? "").trim() || undefined,
    retryable: payload.retryable === true || undefined,
    activeProvider: String(payload.activeProvider ?? "").trim() || undefined,
  };
}

function normalizeListenLyricWords(value: unknown): ListenLyricWord[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  return value.map((word) => {
    const candidate = word as Partial<ListenLyricWord>;
    return {
      startMs: Math.max(0, Number(candidate.startMs ?? 0)),
      endMs:
        candidate.endMs === undefined
          ? undefined
          : Math.max(0, Number(candidate.endMs ?? 0)),
      text: String(candidate.text ?? ""),
      endsWithSpace:
        typeof candidate.endsWithSpace === "boolean"
          ? candidate.endsWithSpace
          : undefined,
      syllables: normalizeListenLyricWords(candidate.syllables),
    };
  });
}

function normalizeListenLyricTimingQuality(
  value: unknown,
): ListenLyricTimingQuality | undefined {
  return value === "plain" ||
    value === "line" ||
    value === "word" ||
    value === "syllable" ||
    value === "estimated"
    ? value
    : undefined;
}

function normalizeListenLyricConfidence(value: unknown): number | undefined {
  const confidence = Number(value);
  return Number.isFinite(confidence)
    ? Math.min(100, Math.max(0, Math.round(confidence)))
    : undefined;
}

export function normalizeListenLyricsCandidates(
  value: unknown,
): ListenLyricsCandidate[] {
  const payload = ((value as { data?: unknown })?.data ?? value) as unknown;
  if (!Array.isArray(payload)) {
    return [];
  }
  return payload
    .map((candidate) => normalizeListenLyricsCandidate(candidate))
    .filter((candidate): candidate is ListenLyricsCandidate => Boolean(candidate));
}

function normalizeListenLyricsCandidate(
  value: unknown,
): ListenLyricsCandidate | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const candidate = value as Partial<ListenLyricsCandidate>;
  const providerId = String(candidate.providerId ?? "").trim().toLowerCase();
  const providerTrackId = String(candidate.providerTrackId ?? "").trim();
  if (!providerId || !providerTrackId) {
    return null;
  }
  return {
    providerId,
    providerTrackId,
    title: String(candidate.title ?? "").trim(),
    artist: String(candidate.artist ?? "").trim(),
    album: String(candidate.album ?? "").trim() || undefined,
    durationSeconds: normalizeOptionalListenLyricsNumber(
      candidate.durationSeconds,
    ),
    instrumental: candidate.instrumental === true || undefined,
    hasSynced: candidate.hasSynced === true || undefined,
    hasPlain: candidate.hasPlain === true || undefined,
    timingQuality: normalizeListenLyricTimingQuality(candidate.timingQuality),
    attribution: String(candidate.attribution ?? "").trim() || undefined,
    confidence: normalizeListenLyricScore(candidate.confidence),
    titleScore: normalizeListenLyricScore(candidate.titleScore),
    artistScore: normalizeListenLyricScore(candidate.artistScore),
    albumScore: normalizeListenLyricScore(candidate.albumScore),
    durationScore: normalizeListenLyricScore(candidate.durationScore),
    durationDiff: normalizeOptionalListenLyricsNumber(candidate.durationDiff),
    accepted: candidate.accepted === true,
    rejection: String(candidate.rejection ?? "").trim() || undefined,
  };
}

function normalizeListenLyricScore(value: unknown) {
  return normalizeListenLyricConfidence(value) ?? 0;
}

function normalizeOptionalListenLyricsNumber(value: unknown) {
  const number = Number(value);
  return Number.isFinite(number) && number >= 0 ? number : undefined;
}

function normalizeListenLyricsKind(
  value: ListenLyricsKind | string | undefined,
): ListenLyricsKind {
  return value === "synced" || value === "plain" || value === "unavailable"
    ? value
    : "unavailable";
}

async function callListenLyrics(name: string, payload?: unknown) {
  const { Call } = await import("@wailsio/runtime");
  if (payload === undefined) {
    return Call.ByName(`${LISTEN_LYRICS_SERVICE}.${name}`);
  }
  return Call.ByName(`${LISTEN_LYRICS_SERVICE}.${name}`, payload);
}

export async function callListenLyricsCandidates(options: {
  track: ListenLyricsCandidateTrack;
  language?: string;
  synced?: boolean;
}) {
  const candidates = await callListenLyrics("SearchCandidates", {
    track: {
      lyricsId: options.track.lyricsId ?? "",
      videoId: options.track.videoId ?? "",
      title: options.track.title,
      artist: options.track.artist ?? options.track.channel ?? "",
      album: options.track.album ?? "",
      localPath: options.track.localPath ?? "",
      durationSeconds: Math.max(0, options.track.durationSeconds ?? 0),
    },
    plainOnly: options.synced === false,
    language: options.language ?? "",
    searchVariants: listenLyricsSearchVariantPayloads(
      options.track,
      options.language,
    ),
  });
  return normalizeListenLyricsCandidates(candidates);
}

export async function callListenLyricsCandidate(options: {
  track: ListenLyricsCandidateTrack;
  candidate: Pick<ListenLyricsCandidate, "providerId" | "providerTrackId">;
  language?: string;
  synced?: boolean;
}) {
  const lyrics = await callListenLyrics("TrackCandidate", {
    track: {
      lyricsId: options.track.lyricsId ?? "",
      videoId: options.track.videoId ?? "",
      title: options.track.title,
      artist: options.track.artist ?? options.track.channel ?? "",
      album: options.track.album ?? "",
      localPath: options.track.localPath ?? "",
      durationSeconds: Math.max(0, options.track.durationSeconds ?? 0),
    },
    providerId: options.candidate.providerId,
    providerTrackId: options.candidate.providerTrackId,
    plainOnly: options.synced === false,
    language: options.language ?? "",
  });
  const normalized = normalizeListenLyricsSnapshot(lyrics);
  if (!normalized) {
    throw new Error("Invalid listen lyrics candidate");
  }
  return normalized;
}

export async function callListenTrackLyrics(options: {
  track: ListenOnlineItem;
  durationSeconds?: number;
  language?: string;
  synced?: boolean;
}) {
  const playbackTrack = listenPlaybackTrackFromOnlineItem(options.track);
  return callListenLyricsForTrack({
    track: {
      videoId: playbackTrack.videoId,
      title: playbackTrack.title,
      artist: playbackTrack.artist,
      durationSeconds: playbackTrack.durationSeconds,
    },
    durationSeconds: options.durationSeconds,
    language: options.language,
    synced: options.synced,
  });
}

export async function callListenLyricsForTrack(options: {
  track: ListenLyricsCandidateTrack;
  durationSeconds?: number;
  language?: string;
  synced?: boolean;
}) {
  const durationSeconds = Math.max(
    0,
    Number(options.durationSeconds ?? options.track.durationSeconds ?? 0),
  );
  const lyrics = await callListenLyrics("TrackLyrics", {
    track: {
      lyricsId: options.track.lyricsId ?? "",
      videoId: options.track.videoId ?? "",
      title: options.track.title,
      artist: options.track.artist ?? options.track.channel ?? "",
      album: options.track.album ?? "",
      localPath: options.track.localPath ?? "",
      durationSeconds,
    },
    lyricsId: options.track.lyricsId ?? "",
    videoId: options.track.videoId ?? "",
    title: options.track.title,
    artist: options.track.artist ?? options.track.channel ?? "",
    album: options.track.album ?? "",
    localPath: options.track.localPath ?? "",
    durationSeconds,
    plainOnly: options.synced === false,
    language: options.language ?? "",
    searchVariants: listenLyricsSearchVariantPayloads(
      options.track,
      options.language,
    ),
  });
  const normalized = normalizeListenLyricsSnapshot(lyrics);
  if (!normalized) {
    throw new Error("Invalid listen lyrics");
  }
  if (normalized.errorCode || normalized.error) {
    throw new ListenLyricsServiceError("Lyrics request failed", {
      code: normalized.errorCode || "lyrics_unavailable",
      retryable: normalized.retryable,
    });
  }
  return normalized;
}

export function listenLyricsSearchVariantPayloads(
  track: ListenLyricsCandidateTrack,
  language?: string,
) {
  return buildListenLyricsChineseSearchTrackVariants(track, language)
    .slice(1)
    .map((variant) => ({
      title: variant.title.trim(),
      artist: String(variant.artist ?? variant.channel ?? "").trim(),
    }));
}

class ListenLyricsServiceError extends Error {
  code: string;
  retryable: boolean;

  constructor(
    message: string,
    options: { code?: string; retryable?: boolean } = {},
  ) {
    super(message);
    this.name = "ListenLyricsServiceError";
    this.code = options.code?.trim() ?? "";
    this.retryable = options.retryable === true;
  }
}

export function callListenLyricsForTrackCached(options: {
  track: ListenLyricsCandidateTrack;
  cacheID?: string;
  durationSeconds?: number;
  language?: string;
  synced?: boolean;
}) {
  const durationSeconds = Math.max(
    0,
    Number(options.durationSeconds ?? options.track.durationSeconds ?? 0),
  );
  const cacheID =
    options.cacheID?.trim() ||
    options.track.lyricsId?.trim() ||
    options.track.videoId?.trim() ||
    `${options.track.title.trim()}\u0000${String(
      options.track.artist ?? options.track.channel ?? "",
    ).trim()}`;
  return loadListenLyricsCached({
    cacheID,
    language: options.language,
    synced: options.synced,
    requestKey: [
      "wails-generic",
      options.track.title.trim().toLowerCase(),
      String(options.track.artist ?? options.track.channel ?? "")
        .trim()
        .toLowerCase(),
      String(options.track.album ?? "").trim().toLowerCase(),
      String(options.track.localPath ?? "").trim(),
      String(Math.round(durationSeconds)),
    ].join("\u0000"),
    loader: () =>
      callListenLyricsForTrack({
        ...options,
        durationSeconds,
      }),
  });
}

export function callListenTrackLyricsCached(options: {
  track: ListenOnlineItem;
  durationSeconds?: number;
  language?: string;
  synced?: boolean;
}) {
  const durationSeconds = Math.max(
    0,
    Number(options.durationSeconds ?? options.track.durationSeconds ?? 0),
  );
  const artist = resolveListenLyricsOnlineArtist(options.track);
  const cacheID =
    options.track.videoId.trim() ||
    `${options.track.title.trim()}\u0000${artist}`;
  return callListenLyricsForTrackCached({
    track: {
      videoId: options.track.videoId,
      title: options.track.title,
      artist,
      durationSeconds: options.track.durationSeconds,
    },
    cacheID,
    durationSeconds,
    language: options.language,
    synced: options.synced,
  });
}
