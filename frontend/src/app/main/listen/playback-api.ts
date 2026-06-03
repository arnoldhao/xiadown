import { LISTEN_NATIVE_PLAYER_SERVICE,LISTEN_PLAYBACK_SNAPSHOT_EVENT } from "@/app/main/listen/catalog";
import type {
  ListenNativePlayerEvent,
  ListenObservedPlaybackAudioQuality,
  ListenOnlineItem,
  ListenOnlineQueueState,
  ListenPlayMode,
  ListenRemotePlaybackState,
  ListenTrackArtist,
} from "@/app/main/listen/types";

export type ListenPlaybackRepeatMode = "off" | "all" | "one";

export type ListenPlaybackTrack = {
  id: string;
  videoId: string;
  title: string;
  artist: string;
  artists?: ListenTrackArtist[];
  artistBrowseId?: string;
  artistSource?: string;
  durationLabel?: string;
  durationSeconds?: number;
  thumbnailUrl?: string;
  musicVideoType?: string;
  hasVideo?: boolean;
  videoAvailabilityKnown?: boolean;
  likeStatus?: string;
  inLibrary?: boolean;
};

export type ListenPlaybackSnapshot = {
  version: number;
  state: ListenRemotePlaybackState;
  currentTrack?: ListenPlaybackTrack;
  progress: number;
  duration: number;
  volume: number;
  volumeBeforeMute?: number;
  muted: boolean;
  shuffleEnabled: boolean;
  repeatMode: ListenPlaybackRepeatMode;
  queue: ListenPlaybackTrack[];
  queueKind: "none" | "radio" | "playlist" | "mix";
  queueTitle: string;
  currentIndex: number;
  pendingPlayVideoId?: string;
  showMiniPlayer: boolean;
  canUndoQueue: boolean;
  canRedoQueue: boolean;
  canAutoloadPending: boolean;
  currentTimeMs?: number;
  observedPlaybackAudioQuality?: ListenObservedPlaybackAudioQuality | "";
};

export function normalizeListenPlaybackSnapshot(value: unknown): ListenPlaybackSnapshot | null {
  const snapshot = ((value as { data?: unknown })?.data ?? value) as Partial<ListenPlaybackSnapshot> | null;
  if (!snapshot || typeof snapshot !== "object") {
    return null;
  }
  return {
    version: Math.max(0, Number(snapshot.version ?? 0)),
    state: snapshot.state ?? "idle",
    currentTrack: snapshot.currentTrack,
    progress: Math.max(0, Number(snapshot.progress ?? 0)),
    duration: Math.max(0, Number(snapshot.duration ?? 0)),
    volume: Number.isFinite(Number(snapshot.volume)) ? Number(snapshot.volume) : 1,
    volumeBeforeMute: Number.isFinite(Number(snapshot.volumeBeforeMute))
      ? Number(snapshot.volumeBeforeMute)
      : undefined,
    muted: snapshot.muted === true,
    shuffleEnabled: snapshot.shuffleEnabled === true,
    repeatMode: snapshot.repeatMode ?? "off",
    queue: Array.isArray(snapshot.queue) ? snapshot.queue : [],
    queueKind: snapshot.queueKind ?? "none",
    queueTitle: snapshot.queueTitle ?? "",
    currentIndex: Number.isFinite(Number(snapshot.currentIndex))
      ? Number(snapshot.currentIndex)
      : 0,
    pendingPlayVideoId: snapshot.pendingPlayVideoId,
    showMiniPlayer: snapshot.showMiniPlayer === true,
    canUndoQueue: snapshot.canUndoQueue === true,
    canRedoQueue: snapshot.canRedoQueue === true,
    canAutoloadPending: snapshot.canAutoloadPending === true,
    currentTimeMs: Math.max(0, Number(snapshot.currentTimeMs ?? 0)),
    observedPlaybackAudioQuality: normalizeObservedPlaybackAudioQuality(snapshot.observedPlaybackAudioQuality),
  };
}

function normalizeObservedPlaybackAudioQuality(value?: string): ListenObservedPlaybackAudioQuality | "" {
  switch (value) {
    case "AUDIO_QUALITY_LOW":
      return "AUDIO_QUALITY_LOW";
    case "AUDIO_QUALITY_MEDIUM":
      return "AUDIO_QUALITY_MEDIUM";
    case "AUDIO_QUALITY_HIGH":
      return "AUDIO_QUALITY_HIGH";
    default:
      return "";
  }
}

export function subscribeListenPlaybackSnapshots(
  handler: (snapshot: ListenPlaybackSnapshot) => void,
) {
  let active = true;
  let unsubscribe = () => {};
  void import("@wailsio/runtime")
    .then(({ Events }) => {
      if (!active) {
        return;
      }
      unsubscribe = Events.On(LISTEN_PLAYBACK_SNAPSHOT_EVENT, (event: unknown) => {
        const snapshot = normalizeListenPlaybackSnapshot(event);
        if (snapshot) {
          handler(snapshot);
        }
      });
    })
    .catch(() => {});
  return () => {
    active = false;
    unsubscribe();
  };
}

async function callListenPlayback(name: string, payload?: unknown) {
  const { Call } = await import("@wailsio/runtime");
  if (payload === undefined) {
    return Call.ByName(`${LISTEN_NATIVE_PLAYER_SERVICE}.${name}`);
  }
  return Call.ByName(`${LISTEN_NATIVE_PLAYER_SERVICE}.${name}`, payload);
}

async function callListenPlaybackSnapshotByName(
  name: string,
  payload?: unknown,
) {
  const snapshot = await callListenPlayback(name, payload);
  const normalized = normalizeListenPlaybackSnapshot(snapshot);
  if (!normalized) {
    throw new Error("Invalid listen playback snapshot");
  }
  return normalized;
}

export function listenPlaybackTrackFromOnlineItem(
  item: ListenOnlineItem,
): ListenPlaybackTrack {
  return {
    id: item.id || item.videoId,
    videoId: item.videoId,
    title: item.title,
    artist: item.channel,
    artists: item.artists,
    artistBrowseId: item.artistBrowseId,
    artistSource: item.artistSource || (item.artistBrowseId ? "api-linked" : undefined),
    durationLabel: item.durationLabel,
    durationSeconds: item.durationSeconds,
    thumbnailUrl: item.thumbnailUrl,
    musicVideoType: item.musicVideoType,
    hasVideo: item.hasVideo,
    videoAvailabilityKnown: item.videoAvailabilityKnown,
  };
}

export function listenOnlineItemFromPlaybackTrack(
  track: ListenPlaybackTrack,
  httpBaseURL = "",
): ListenOnlineItem {
  return {
    id: track.id || `ytmusic-native-${track.videoId}`,
    group: "playlist",
    videoId: track.videoId,
    title: track.title || track.videoId,
    channel: track.artist || "YouTube Music",
    artists: track.artists,
    artistBrowseId: track.artistBrowseId,
    artistSource: track.artistSource,
    description: "",
    durationLabel: track.durationLabel || "",
    durationSeconds: track.durationSeconds,
    thumbnailUrl: track.thumbnailUrl,
    musicVideoType: track.musicVideoType,
    hasVideo: track.hasVideo,
    videoAvailabilityKnown: track.videoAvailabilityKnown,
  };
}

export function listenQueueStateFromPlaybackSnapshot(
  snapshot: ListenPlaybackSnapshot,
  httpBaseURL = "",
): ListenOnlineQueueState {
  const items = snapshot.queue.map((track) =>
    listenOnlineItemFromPlaybackTrack(track, httpBaseURL),
  );
  if (items.length === 0 || snapshot.queueKind === "none") {
    return { kind: "none", title: "", items: [] };
  }
  if (snapshot.queueKind === "radio") {
    const currentItem = items[Math.max(0, snapshot.currentIndex)] ?? items[0];
    return {
      kind: "radio",
      title: snapshot.queueTitle,
      items,
      seedVideoId: currentItem?.videoId ?? "",
    };
  }
  return {
    kind: "playlist",
    title: snapshot.queueTitle,
    items,
    playlistId: "",
  };
}

export function listenSelectedIDFromPlaybackSnapshot(
  snapshot: ListenPlaybackSnapshot,
) {
  return snapshot.queue[Math.max(0, snapshot.currentIndex)]?.id ?? "";
}

export function listenPlayModeFromPlaybackSnapshot(
  snapshot: ListenPlaybackSnapshot,
): ListenPlayMode {
  if (snapshot.shuffleEnabled) {
    return "shuffle";
  }
  if (snapshot.repeatMode === "one") {
    return "repeat";
  }
  return "order";
}

export function listenRepeatModeFromPlayMode(
  mode: ListenPlayMode,
): ListenPlaybackRepeatMode {
  return mode === "repeat" ? "one" : "off";
}

export async function callListenPlaybackSnapshot() {
  return callListenPlaybackSnapshotByName("PlaybackSnapshot");
}

export async function callListenPlaybackPlayQueue(options: {
  tracks: ListenOnlineItem[];
  startingAt: number;
  title: string;
  kind?: "radio" | "playlist" | "mix";
  playlistId?: string;
  startVideoId?: string;
}) {
  return callListenPlaybackSnapshotByName("PlayQueue", {
    tracks: options.tracks.map(listenPlaybackTrackFromOnlineItem),
    startingAt: options.startingAt,
    title: options.title,
    kind: options.kind ?? "playlist",
    playlistId: options.playlistId ?? "",
    startVideoId: options.startVideoId ?? "",
  });
}

export async function callListenPlaybackPlayTrack(
  item: ListenOnlineItem,
  options: { startSeconds?: number; forceReload?: boolean } = {},
) {
  return callListenPlaybackSnapshotByName("PlayTrack", {
    track: listenPlaybackTrackFromOnlineItem(item),
    startSeconds: Math.max(0, options.startSeconds ?? 0),
    forceReload: options.forceReload === true,
  });
}

export async function callListenPlaybackMergeTrackMetadata(item: ListenOnlineItem) {
  return callListenPlaybackSnapshotByName("MergeTrackMetadata", {
    track: listenPlaybackTrackFromOnlineItem(item),
  });
}

export async function callListenPlaybackObserveNativeEvent(
  event: ListenNativePlayerEvent,
) {
  return callListenPlaybackSnapshotByName("ObservePlayback", {
    observedVideoId: String(event.observedVideoId || event.videoId || ""),
    title: event.title ?? "",
    artist: event.artist ?? "",
    thumbnailUrl: event.thumbnailUrl ?? "",
    likeStatus: event.likeStatus ?? "",
    trackChanged: event.trackChanged === true,
    state: event.state ?? "idle",
    progress: Math.max(0, Number(event.currentTime || 0)),
    duration: Math.max(0, Number(event.duration || 0)),
    paused: event.paused === true,
    ended: event.ended === true,
  });
}

export async function callListenPlaybackNext() {
  return callListenPlaybackSnapshotByName("Next");
}

export async function callListenPlaybackPrevious() {
  return callListenPlaybackSnapshotByName("Previous");
}

export async function callListenPlaybackPlayPause() {
  return callListenPlaybackSnapshotByName("PlayPause");
}

export async function callListenPlaybackPause() {
  return callListenPlaybackSnapshotByName("PlaybackPause");
}

export async function callListenPlaybackResume() {
  return callListenPlaybackSnapshotByName("PlaybackResume");
}

export async function callListenPlaybackSeek(seconds: number) {
  return callListenPlaybackSnapshotByName("PlaybackSeek", {
    seconds: Math.max(0, seconds),
  });
}

export async function callListenPlaybackSetVolume(volume: number, muted: boolean) {
  return callListenPlaybackSnapshotByName("PlaybackSetVolume", {
    volume,
    muted,
  });
}

export async function callListenPlaybackSetRepeatMode(mode: ListenPlaybackRepeatMode) {
  return callListenPlaybackSnapshotByName("SetRepeatMode", {
    mode,
  });
}

export async function callListenPlaybackSetShuffle(enabled: boolean) {
  return callListenPlaybackSnapshotByName("SetShuffle", {
    enabled,
  });
}

export async function callListenPlaybackClearQueue() {
  return callListenPlaybackSnapshotByName("ClearQueue");
}

export async function callListenPlaybackClearQueueEntirely() {
  return callListenPlaybackSnapshotByName("ClearQueueEntirely");
}

export async function callListenPlaybackRemoveFromQueue(items: ListenOnlineItem[]) {
  return callListenPlaybackSnapshotByName("RemoveFromQueue", {
    trackIds: items.map((item) => item.id).filter(Boolean),
    videoIds: items.map((item) => item.videoId).filter(Boolean),
  });
}

export async function callListenPlaybackMoveQueueItem(
  sourceIndex: number,
  destination: number,
) {
  return callListenPlaybackSnapshotByName("MoveQueueItems", {
    source: [sourceIndex],
    destination,
  });
}

export async function callListenPlaybackInsertNextInQueue(items: ListenOnlineItem[]) {
  return callListenPlaybackSnapshotByName("InsertNextInQueue", {
    tracks: items.map(listenPlaybackTrackFromOnlineItem),
  });
}

export async function callListenPlaybackAppendToQueue(items: ListenOnlineItem[]) {
  return callListenPlaybackSnapshotByName("AppendToQueue", {
    tracks: items.map(listenPlaybackTrackFromOnlineItem),
  });
}

export async function callListenPlaybackUndoQueue() {
  return callListenPlaybackSnapshotByName("UndoQueue");
}

export async function callListenPlaybackRedoQueue() {
  return callListenPlaybackSnapshotByName("RedoQueue");
}
