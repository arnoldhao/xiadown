import {
  hasTrustedListenOnlineArtist,
  isMissingListenArtistLabel,
} from "@/app/main/listen/playback-helpers";
import type {
  ListenNativePlayerEvent,
  ListenOnlineItem,
  ListenRemotePlaybackState,
  ListenTrackArtist,
} from "@/app/main/listen/types";

const LISTEN_UNKNOWN_ARTIST = "Unknown Artist";

const LISTEN_REMOTE_PLAYBACK_STATES: ListenRemotePlaybackState[] = [
  "idle",
  "loading",
  "playing",
  "paused",
  "buffering",
  "ended",
  "error",
];

export function mergeListenNativeTrackItem(
  incoming: ListenOnlineItem,
  current: ListenOnlineItem,
): ListenOnlineItem {
  const videoId = current.videoId || incoming.videoId;
  const incomingTitle = incoming.title.trim();
  const currentTitle = current.title.trim();
  const incomingArtistTrusted = hasTrustedListenOnlineArtist(incoming);
  const currentArtistTrusted = hasTrustedListenOnlineArtist(current);
  const incomingVideoKnown = incoming.videoAvailabilityKnown === true;
  const currentVideoKnown = current.videoAvailabilityKnown === true;
  const incomingArtists = normalizeListenTrackArtists(incoming.artists);
  const currentArtists = normalizeListenTrackArtists(current.artists);
  return {
    ...current,
    videoId,
    title:
      incomingTitle && incomingTitle !== videoId
        ? incoming.title
        : currentTitle || videoId,
    channel: incomingArtistTrusted
      ? incoming.channel
      : currentArtistTrusted
        ? current.channel
        : LISTEN_UNKNOWN_ARTIST,
    artists: incomingArtists ?? currentArtists,
    artistBrowseId: incoming.artistBrowseId || current.artistBrowseId,
    artistSource: incomingArtistTrusted
      ? incoming.artistSource || (incoming.artistBrowseId ? "api-linked" : undefined)
      : currentArtistTrusted
        ? current.artistSource || (current.artistBrowseId ? "api-linked" : undefined)
        : incoming.artistSource || current.artistSource,
    description: incoming.description || current.description,
    durationLabel: incoming.durationLabel || current.durationLabel,
    playCountLabel: incoming.playCountLabel || current.playCountLabel,
    thumbnailUrl: incoming.thumbnailUrl || current.thumbnailUrl,
    musicVideoType: incoming.musicVideoType || current.musicVideoType,
    hasVideo: incomingVideoKnown
      ? incoming.hasVideo === true
      : incoming.hasVideo === true || current.hasVideo === true,
    videoAvailabilityKnown:
      incomingVideoKnown || currentVideoKnown ? true : undefined,
  };
}

export function normalizeListenTrackArtists(
  artists: ListenTrackArtist[] | undefined,
): ListenTrackArtist[] | undefined {
  if (!Array.isArray(artists) || artists.length === 0) {
    return undefined;
  }
  const result: ListenTrackArtist[] = [];
  const seen = new Set<string>();
  for (const artist of artists) {
    const name = artist.name.trim();
    const browseId = artist.browseId?.trim() ?? "";
    if (!name) {
      continue;
    }
    const key = browseId || name.toLocaleLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    result.push({
      name,
      browseId: browseId || undefined,
      thumbnailUrl: artist.thumbnailUrl?.trim() || undefined,
    });
  }
  return result.length > 0 ? result : undefined;
}

function stringFromNativeStatus(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function secondsFromNativeStatus(value: unknown) {
  return typeof value === "number" && Number.isFinite(value)
    ? Math.max(0, value)
    : 0;
}

function normalizeNativePlaybackState(
  value: unknown,
  fallback: ListenRemotePlaybackState,
): ListenRemotePlaybackState {
  const state = stringFromNativeStatus(value) as ListenRemotePlaybackState;
  return LISTEN_REMOTE_PLAYBACK_STATES.includes(state) ? state : fallback;
}

export function createNativeOnlineItem(params: {
  videoId: string;
  title?: string;
  artist?: string;
  thumbnailUrl?: string;
}): ListenOnlineItem {
  const videoId = params.videoId.trim();
  const title = params.title?.trim() || videoId;
  const rawArtist = params.artist?.trim() || "YouTube Music";
  const artist = isMissingListenArtistLabel(rawArtist)
    ? LISTEN_UNKNOWN_ARTIST
    : rawArtist;
  const thumbnailUrl = params.thumbnailUrl?.trim() || "";
  return {
    id: `ytmusic-native-${videoId}`,
    group: "playlist",
    videoId,
    title,
    channel: artist,
    artistSource: "observed",
    description: "",
    durationLabel: "",
    thumbnailUrl: thumbnailUrl,
  };
}

export function nativeStatusToPlayerEvent(
  value: unknown,
  source = "listen-youtube-music-player",
): ListenNativePlayerEvent | null {
  const record =
    value && typeof value === "object"
      ? (value as Record<string, unknown>)
      : null;
  if (!record || record.available !== true) {
    return null;
  }
  const videoId =
    stringFromNativeStatus(record.observedVideoId) ||
    stringFromNativeStatus(record.videoId);
  if (!videoId) {
    return null;
  }
  const state = normalizeNativePlaybackState(record.state, "paused");
  if (state === "idle") {
    return null;
  }
  return {
    source,
    provider:
      record.provider === "stream" ||
      record.provider === "youtube" ||
      record.provider === "youtube_music" ||
      record.provider === "local"
        ? record.provider
        : undefined,
    sessionId: stringFromNativeStatus(record.sessionId),
    type: "status",
    state,
    videoId,
    observedVideoId: videoId,
    requestedVideoId: stringFromNativeStatus(record.videoId),
    title: stringFromNativeStatus(record.title),
    artist: stringFromNativeStatus(record.artist),
    thumbnailUrl: stringFromNativeStatus(record.thumbnailUrl),
    likeStatus: stringFromNativeStatus(record.likeStatus),
    currentTime: secondsFromNativeStatus(record.currentTime),
    duration: secondsFromNativeStatus(record.duration),
    bufferedTime: secondsFromNativeStatus(record.bufferedTime),
    advertising: record.advertising === true,
    adLabel: stringFromNativeStatus(record.adLabel),
    errorCode: stringFromNativeStatus(record.errorCode),
    errorMessage: stringFromNativeStatus(record.errorMessage),
  };
}
