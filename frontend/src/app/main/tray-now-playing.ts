import type {
  ListenMode,
  ListenNowPlayingState,
  ListenNowPlayingStatus,
  ListenPlaybackSource,
  ListenTrackArtist,
} from "@/app/main/listen/types";

const LISTEN_STATES: ListenNowPlayingState[] = [
  "idle",
  "loading",
  "playing",
  "paused",
  "error",
];
const LISTEN_MODES: ListenMode[] = ["linger", "muse", "hush"];
const PLAYBACK_SOURCES: ListenPlaybackSource[] = [
  "youtube_music",
  "radio",
  "local",
  "youtube",
  "library_preview",
  "unknown",
];

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object"
    ? (value as Record<string, unknown>)
    : null;
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value : "";
}

function finiteNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function optionalFiniteNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value)
    ? value
    : undefined;
}

function optionalBoolean(value: unknown) {
  return typeof value === "boolean" ? value : undefined;
}

function normalizeStringList(value: unknown) {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const result = value
    .map(stringValue)
    .map((item) => item.trim())
    .filter(Boolean);
  return result.length > 0 ? result : undefined;
}

function normalizeArtists(value: unknown): ListenTrackArtist[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const artists = value.flatMap((item) => {
    const record = asRecord(item);
    const name = stringValue(record?.name).trim();
    if (!name) {
      return [];
    }
    return [{
      name,
      browseId: stringValue(record?.browseId) || undefined,
      thumbnailUrl: stringValue(record?.thumbnailUrl) || undefined,
    }];
  });
  return artists.length > 0 ? artists : undefined;
}

export function normalizeTrayNowPlayingStatus(
  value: unknown,
): ListenNowPlayingStatus | null {
  const record = asRecord(value);
  if (!record) {
    return null;
  }

  const state = stringValue(record.state) as ListenNowPlayingState;
  if (!LISTEN_STATES.includes(state)) {
    return null;
  }

  const mode = stringValue(record.mode) as ListenMode;
  const source = stringValue(record.playbackSource) as ListenPlaybackSource;
  const progress = asRecord(record.progress);

  return {
    state,
    live: optionalBoolean(record.live),
    mediaId: stringValue(record.mediaId) || undefined,
    title: stringValue(record.title),
    subtitle: stringValue(record.subtitle),
    artists: normalizeArtists(record.artists),
    artworkURL: stringValue(record.artworkURL),
    artworkCandidates: normalizeStringList(record.artworkCandidates),
    playbackSource: PLAYBACK_SOURCES.includes(source) ? source : undefined,
    playbackSourceLabel: stringValue(record.playbackSourceLabel) || undefined,
    mode: LISTEN_MODES.includes(mode) ? mode : "linger",
    canControl: record.canControl === true,
    canPrevious: optionalBoolean(record.canPrevious),
    canNext: optionalBoolean(record.canNext),
    progress: {
      currentTime: finiteNumber(progress?.currentTime),
      duration: finiteNumber(progress?.duration),
      bufferedTime: finiteNumber(progress?.bufferedTime),
    },
    muted: optionalBoolean(record.muted),
    volume: optionalFiniteNumber(record.volume),
    sourceURL: stringValue(record.sourceURL) || undefined,
    favoriteActive: optionalBoolean(record.favoriteActive),
    canFavorite: optionalBoolean(record.canFavorite),
  };
}
