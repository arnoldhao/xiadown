import { LISTEN_LYRICS_SERVICE } from "@/app/main/listen/catalog";
import type {
  ListenLyricsData,
  ListenLyricsKind,
  ListenOnlineItem,
} from "@/app/main/listen/types";
import { listenPlaybackTrackFromOnlineItem } from "@/app/main/listen/playback-api";

export type ListenLyricsSnapshot = ListenLyricsData & {
  loading?: boolean;
  error?: string;
  activeProvider?: string;
};

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
    text: String(payload.text ?? ""),
    lines: lines.map((line) => ({
      startMs: Math.max(0, Number(line.startMs ?? 0)),
      durationMs: Math.max(0, Number(line.durationMs ?? 0)),
      text: String(line.text ?? ""),
      romanizedText: line.romanizedText?.trim() || undefined,
      romanizedKind:
        line.romanizedKind === "romanized" || line.romanizedKind === "pinyin"
          ? line.romanizedKind
          : undefined,
      words: Array.isArray(line.words)
        ? line.words.map((word) => ({
            startMs: Math.max(0, Number(word.startMs ?? 0)),
            text: String(word.text ?? ""),
          }))
        : undefined,
    })),
    loading: payload.loading === true,
    error: String(payload.error ?? "").trim(),
    activeProvider: String(payload.activeProvider ?? "").trim() || undefined,
  };
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

export async function callListenTrackLyrics(options: {
  track: ListenOnlineItem;
  durationSeconds?: number;
  language?: string;
  synced?: boolean;
}) {
  const lyrics = await callListenLyrics("TrackLyrics", {
    track: listenPlaybackTrackFromOnlineItem(options.track),
    videoId: options.track.videoId,
    title: options.track.title,
    artist: options.track.channel,
    durationSeconds: Math.max(
      0,
      Number(options.durationSeconds ?? options.track.durationSeconds ?? 0),
    ),
    plainOnly: options.synced === false,
    language: options.language ?? "",
  });
  const normalized = normalizeListenLyricsSnapshot(lyrics);
  if (!normalized) {
    throw new Error("Invalid listen lyrics");
  }
  return normalized;
}
