import type {
  ListenLyricsCandidate,
  ListenLyricsData,
} from "@/app/main/listen/types";
import type { ListenLyricsCandidateTrack } from "@/app/main/listen/lyrics-api";

export const LISTEN_LYRICS_OFFSET_STEP_MS = 250;
export const LISTEN_LYRICS_OFFSET_LIMIT_MS = 5_000;

export type ListenLyricsSearchDraft = {
  title: string;
  artist: string;
  album: string;
};

export function resolveListenLyricsRenderTimeMs(
  currentTimeMs: number,
  offsetMs: number,
) {
  const current = Number.isFinite(currentTimeMs) ? currentTimeMs : 0;
  const offset = Number.isFinite(offsetMs) ? offsetMs : 0;
  return Math.max(0, current + offset);
}

export function normalizeListenLyricsWorkspaceOffset(value: number) {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(
    -LISTEN_LYRICS_OFFSET_LIMIT_MS,
    Math.min(LISTEN_LYRICS_OFFSET_LIMIT_MS, Math.round(value)),
  );
}

export function stepListenLyricsWorkspaceOffset(
  offsetMs: number,
  direction: "earlier" | "later",
) {
  return normalizeListenLyricsWorkspaceOffset(
    offsetMs +
      (direction === "earlier"
        ? LISTEN_LYRICS_OFFSET_STEP_MS
        : -LISTEN_LYRICS_OFFSET_STEP_MS),
  );
}

export function formatListenLyricsOffset(offsetMs: number) {
  const normalized = normalizeListenLyricsWorkspaceOffset(offsetMs);
  if (normalized === 0) {
    return "0.00 s";
  }
  const sign = normalized > 0 ? "+" : "−";
  return `${sign}${(Math.abs(normalized) / 1000).toFixed(2)} s`;
}

export function listenLyricsCandidateKey(
  candidate: Pick<ListenLyricsCandidate, "providerId" | "providerTrackId">,
) {
  return `${candidate.providerId.trim().toLowerCase()}:${candidate.providerTrackId.trim()}`;
}

export function listenLyricsCandidatePreviewKey(options: {
  track: ListenLyricsCandidateTrack;
  candidate: Pick<ListenLyricsCandidate, "providerId" | "providerTrackId">;
  language?: string;
  synced?: boolean;
}) {
  return [
    normalizeIdentity(options.track.lyricsId),
    normalizeIdentity(options.track.videoId),
    normalizeIdentity(options.track.title),
    normalizeIdentity(options.track.artist ?? options.track.channel),
    normalizeIdentity(options.track.album),
    normalizeIdentity(options.track.localPath),
    String(Math.round(Math.max(0, options.track.durationSeconds ?? 0))),
    listenLyricsCandidateKey(options.candidate),
    normalizeIdentity(options.language),
    options.synced === false ? "plain" : "synced",
  ].join("\u0000");
}

export function buildListenLyricsCandidateTrack(
  track: ListenLyricsCandidateTrack,
  draft: ListenLyricsSearchDraft,
): ListenLyricsCandidateTrack {
  return {
    lyricsId: track.lyricsId?.trim() || undefined,
    videoId: track.videoId?.trim() || undefined,
    title: draft.title.trim(),
    artist: draft.artist.trim(),
    album: draft.album.trim() || undefined,
    localPath: track.localPath?.trim() || undefined,
    durationSeconds:
      Number.isFinite(track.durationSeconds) && Number(track.durationSeconds) > 0
        ? Number(track.durationSeconds)
        : undefined,
  };
}

export function initialListenLyricsSearchDraft(
  track: ListenLyricsCandidateTrack,
): ListenLyricsSearchDraft {
  return {
    title: track.title.trim(),
    artist: String(track.artist ?? track.channel ?? "").trim(),
    album: String(track.album ?? "").trim(),
  };
}

export function formatListenLyricsCandidateDuration(seconds?: number) {
  if (!Number.isFinite(seconds) || Number(seconds) < 0) {
    return "";
  }
  const rounded = Math.round(Number(seconds));
  const minutes = Math.floor(rounded / 60);
  return `${minutes}:${String(rounded % 60).padStart(2, "0")}`;
}

export function createListenLyricsRequestGate() {
  let generation = 0;
  return {
    begin() {
      generation += 1;
      return generation;
    },
    invalidate() {
      generation += 1;
    },
    isCurrent(request: number) {
      return request === generation;
    },
  };
}

export function createListenLyricsPreviewCache() {
  const requests = new Map<string, Promise<ListenLyricsData>>();
  return {
    clear() {
      requests.clear();
    },
    load(key: string, loader: () => Promise<ListenLyricsData>) {
      const existing = requests.get(key);
      if (existing) {
        return existing;
      }
      const request = Promise.resolve().then(loader);
      requests.set(key, request);
      void request.catch(() => {
        if (requests.get(key) === request) {
          requests.delete(key);
        }
      });
      return request;
    },
    size() {
      return requests.size;
    },
  };
}

function normalizeIdentity(value: unknown) {
  return String(value ?? "")
    .normalize("NFKC")
    .trim()
    .toLocaleLowerCase()
    .replace(/\s+/g, " ");
}
