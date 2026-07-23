import {
  EMPTY_PLAYBACK_SNAPSHOT,
  type PlaybackCapabilities,
  type PlaybackFocus,
  type PlaybackMediaItem,
  type PlaybackMediaKind,
  type PlaybackProvider,
  type PlaybackSessionSnapshot,
  type PlaybackSnapshot,
  type PlaybackState,
} from "@/shared/playback/types";

const PLAYBACK_STATES = new Set<PlaybackState>([
  "idle",
  "loading",
  "playing",
  "paused",
  "buffering",
  "ended",
  "error",
]);

const PLAYBACK_PROVIDERS = new Set<PlaybackProvider>([
  "youtube_music",
  "youtube",
  "local",
  "stream",
]);

function record(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object"
    ? (value as Record<string, unknown>)
    : null;
}

function finite(value: unknown, fallback = 0) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function bool(value: unknown) {
  return value === true;
}

function string(value: unknown) {
  return typeof value === "string" ? value : "";
}

function normalizeMediaKind(value: unknown): PlaybackMediaKind {
  return value === "video" ? "video" : "audio";
}

function normalizeMediaItem(value: unknown): PlaybackMediaItem {
  const item = record(value) ?? {};
  const source = record(item.source) ?? {};
  const providerValue = string(source.provider) as PlaybackProvider;
  const provider = PLAYBACK_PROVIDERS.has(providerValue)
    ? providerValue
    : "local";
  const id = string(item.id) || string(source.id) || string(source.uri);
  const metadataRecord = record(item.metadata);
  const metadata = metadataRecord
    ? Object.fromEntries(
        Object.entries(metadataRecord).flatMap(([key, entry]) =>
          typeof entry === "string" ? [[key, entry]] : [],
        ),
      )
    : undefined;
  return {
    id,
    kind: normalizeMediaKind(item.kind),
    source: {
      provider,
      id: string(source.id) || undefined,
      uri: string(source.uri) || undefined,
      live: bool(source.live) || undefined,
    },
    title: string(item.title) || id,
    artist: string(item.artist) || undefined,
    artists: Array.isArray(item.artists)
      ? item.artists.filter((artist): artist is string => typeof artist === "string")
      : undefined,
    artworkUrl: string(item.artworkUrl) || undefined,
    canonicalUrl: string(item.canonicalUrl) || undefined,
    duration: Math.max(0, finite(item.duration)),
    metadata,
  };
}

function normalizeCapabilities(value: unknown): PlaybackCapabilities {
  const capabilities = record(value) ?? {};
  return {
    available: bool(capabilities.available),
    unsupportedReason: string(capabilities.unsupportedReason) || undefined,
    mediaKinds: Array.isArray(capabilities.mediaKinds)
      ? capabilities.mediaKinds.map(normalizeMediaKind)
      : [],
    playPause: bool(capabilities.playPause),
    stop: bool(capabilities.stop),
    seek: bool(capabilities.seek),
    previous: bool(capabilities.previous),
    next: bool(capabilities.next),
    volume: bool(capabilities.volume),
    queue: bool(capabilities.queue),
    shuffle: bool(capabilities.shuffle),
    repeat: bool(capabilities.repeat),
    lyrics: bool(capabilities.lyrics),
    video: bool(capabilities.video),
    like: bool(capabilities.like),
    dislike: bool(capabilities.dislike),
    captions: bool(capabilities.captions),
    audioTracks: bool(capabilities.audioTracks),
    quality: bool(capabilities.quality),
    fullscreen: bool(capabilities.fullscreen),
  };
}

function normalizeSession(value: unknown): PlaybackSessionSnapshot | null {
  const session = record(value);
  if (!session) {
    return null;
  }
  const stateValue = string(session.state) as PlaybackState;
  const focus: PlaybackFocus =
    session.focus === "transient_preview"
      ? "transient_preview"
      : "persistent";
  return {
    id: string(session.id),
    focus,
    state: PLAYBACK_STATES.has(stateValue) ? stateValue : "idle",
    errorMessage: string(session.errorMessage) || undefined,
    item: normalizeMediaItem(session.item),
    capabilities: normalizeCapabilities(session.capabilities),
    position: Math.max(0, finite(session.position)),
    duration: Math.max(0, finite(session.duration)),
    volume: Math.max(0, Math.min(1, finite(session.volume, 1))),
    muted: bool(session.muted),
    queue: Array.isArray(session.queue)
      ? session.queue.map(normalizeMediaItem)
      : [],
    currentIndex: Math.max(0, Math.trunc(finite(session.currentIndex))),
    shuffleEnabled: bool(session.shuffleEnabled),
    repeatMode:
      session.repeatMode === "all" || session.repeatMode === "one"
        ? session.repeatMode
        : "off",
  };
}

export function normalizePlaybackSnapshot(value: unknown): PlaybackSnapshot {
  const payload = record(value);
  const raw = record(payload?.data) ?? payload;
  if (!raw) {
    return EMPTY_PLAYBACK_SNAPSHOT;
  }
  return {
    version: Math.max(0, Math.trunc(finite(raw.version))),
    audibleSessionId: string(raw.audibleSessionId) || undefined,
    active: normalizeSession(raw.active),
    suspendedPersistent: normalizeSession(raw.suspendedPersistent),
  };
}

export function playbackSessionByID(
  snapshot: PlaybackSnapshot,
  sessionID: string,
) {
  if (snapshot.active?.id === sessionID) {
    return snapshot.active;
  }
  if (snapshot.suspendedPersistent?.id === sessionID) {
    return snapshot.suspendedPersistent;
  }
  return null;
}
