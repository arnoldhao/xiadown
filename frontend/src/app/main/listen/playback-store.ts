import * as React from "react";

import {
  callListenPlaybackSnapshot,
  listenOnlineItemFromPlaybackTrack,
  listenPlayModeFromPlaybackSnapshot,
  listenQueueStateFromPlaybackSnapshot,
  listenSelectedIDFromPlaybackSnapshot,
  subscribeListenPlaybackSnapshots,
  type ListenPlaybackSnapshot,
} from "@/app/main/listen/playback-api";
import { clampVolume } from "@/app/main/listen/local-library";
import type {
  ListenOnlineItem,
  ListenOnlineQueueState,
  ListenObservedPlaybackAudioQuality,
  ListenPlayMode,
  ListenPlaybackProgressState,
  ListenRemotePlaybackState,
} from "@/app/main/listen/types";

export type ListenPlaybackProjection = {
  snapshot: ListenPlaybackSnapshot | null;
  hydrated: boolean;
  queueState: ListenOnlineQueueState;
  queueItems: ListenOnlineItem[];
  currentItem: ListenOnlineItem | null;
  selectedId: string;
  state: ListenRemotePlaybackState;
  playing: boolean;
  armed: boolean;
  progress: ListenPlaybackProgressState & { videoId: string };
  playMode: ListenPlayMode;
  canUndoQueue: boolean;
  canRedoQueue: boolean;
  volume: number;
  muted: boolean;
  volumeBeforeMute: number;
  observedPlaybackAudioQuality: ListenObservedPlaybackAudioQuality | "";
};

type UseListenPlaybackStoreOptions = {
  httpBaseURL?: string;
  shouldAcceptSnapshot?: (snapshot: ListenPlaybackSnapshot) => boolean;
  shouldActivateSnapshot?: (snapshot: ListenPlaybackSnapshot) => boolean;
  onActivate?: () => void;
};

const emptyQueueState: ListenOnlineQueueState = {
  kind: "none",
  title: "",
  items: [],
};

function emptyProjection(hydrated = false): ListenPlaybackProjection {
  return {
    snapshot: null,
    hydrated,
    queueState: emptyQueueState,
    queueItems: [],
    currentItem: null,
    selectedId: "",
    state: "idle",
    playing: false,
    armed: false,
    progress: { videoId: "", currentTime: 0, duration: 0, bufferedTime: 0 },
    playMode: "order",
    canUndoQueue: false,
    canRedoQueue: false,
    volume: 1,
    muted: false,
    volumeBeforeMute: 1,
    observedPlaybackAudioQuality: "",
  };
}

export function deriveListenPlaybackProjection(
  snapshot: ListenPlaybackSnapshot | null,
  hydrated = true,
  httpBaseURL = "",
): ListenPlaybackProjection {
  if (!snapshot) {
    return emptyProjection(hydrated);
  }
  const queueState = listenQueueStateFromPlaybackSnapshot(snapshot, httpBaseURL);
  const currentTrack = snapshot.currentTrack ?? snapshot.queue[snapshot.currentIndex];
  const currentItem = currentTrack
    ? listenOnlineItemFromPlaybackTrack(currentTrack, httpBaseURL)
    : null;
  const selectedId =
    listenSelectedIDFromPlaybackSnapshot(snapshot) || currentItem?.id || "";
  const nextVolume = clampVolume(snapshot.volume);
  const volumeBeforeMute = clampVolume(snapshot.volumeBeforeMute ?? nextVolume);
  return {
    snapshot,
    hydrated,
    queueState,
    queueItems: queueState.items,
    currentItem,
    selectedId,
    state: snapshot.state,
    playing: snapshot.state === "playing" || snapshot.state === "buffering",
    armed: Boolean(currentTrack),
    progress: currentTrack?.videoId
      ? {
          videoId: currentTrack.videoId,
          currentTime: Math.max(0, snapshot.progress || 0),
          duration: Math.max(0, snapshot.duration || 0),
          bufferedTime: 0,
        }
      : { videoId: "", currentTime: 0, duration: 0, bufferedTime: 0 },
    playMode: listenPlayModeFromPlaybackSnapshot(snapshot),
    canUndoQueue: snapshot.canUndoQueue,
    canRedoQueue: snapshot.canRedoQueue,
    volume: nextVolume,
    muted: snapshot.muted || nextVolume <= 0,
    volumeBeforeMute,
    observedPlaybackAudioQuality: snapshot.observedPlaybackAudioQuality ?? "",
  };
}

function listenOnlineItemStableKey(item: ListenOnlineItem | null | undefined) {
  if (!item) {
    return "";
  }
  return [
    item.id,
    item.group,
    item.videoId,
    item.title,
    item.channel,
    (item.artists ?? [])
      .map((artist) =>
        [artist.name, artist.browseId ?? "", artist.thumbnailUrl ?? ""].join("\u0002"),
      )
      .join("\u0003"),
    item.artistBrowseId ?? "",
    item.durationLabel ?? "",
    String(item.durationSeconds ?? ""),
    item.thumbnailUrl ?? "",
    item.musicVideoType ?? "",
    item.hasVideo === true ? "1" : "0",
    item.videoAvailabilityKnown === true ? "1" : "0",
  ].join("\u0001");
}

function listenQueueStableKey(queueState: ListenOnlineQueueState) {
  const base = [queueState.kind, queueState.title ?? ""];
  if (queueState.kind === "radio") {
    base.push(queueState.seedVideoId ?? "");
  } else if (queueState.kind === "playlist") {
    base.push(queueState.playlistId ?? "");
  } else {
    base.push("");
  }
  return `${base.join("\u0001")}\u0002${queueState.items
    .map((item) => listenOnlineItemStableKey(item))
    .join("\u0002")}`;
}

export function stabilizeListenPlaybackProjection(
  previous: ListenPlaybackProjection,
  next: ListenPlaybackProjection,
): ListenPlaybackProjection {
  if (
    listenOnlineItemStableKey(previous.currentItem) ===
    listenOnlineItemStableKey(next.currentItem)
  ) {
    next = { ...next, currentItem: previous.currentItem };
  }
  if (listenQueueStableKey(previous.queueState) === listenQueueStableKey(next.queueState)) {
    next = {
      ...next,
      queueState: previous.queueState,
      queueItems: previous.queueItems,
    };
  }
  return next;
}

export function useListenPlaybackStore(options: UseListenPlaybackStoreOptions = {}) {
  const optionsRef = React.useRef(options);
  const versionRef = React.useRef(0);
  const [projection, setProjection] = React.useState<ListenPlaybackProjection>(
    () => emptyProjection(false),
  );

  React.useEffect(() => {
    optionsRef.current = options;
  }, [options]);

  React.useEffect(() => {
    setProjection((current) =>
      current.snapshot
        ? stabilizeListenPlaybackProjection(
            current,
            deriveListenPlaybackProjection(
              current.snapshot,
              current.hydrated,
              options.httpBaseURL ?? "",
            ),
          )
        : current,
    );
  }, [options.httpBaseURL]);

  const applySnapshot = React.useCallback(
    (snapshot: ListenPlaybackSnapshot, applyOptions: { activate?: boolean } = {}) => {
      const snapshotVersion = Math.max(0, Number(snapshot.version ?? 0));
      if (
        versionRef.current > 0 &&
        (snapshotVersion <= 0 || versionRef.current > snapshotVersion)
      ) {
        return;
      }
      const currentOptions = optionsRef.current;
      if (
        currentOptions.shouldAcceptSnapshot &&
        !currentOptions.shouldAcceptSnapshot(snapshot)
      ) {
        return;
      }
      if (snapshotVersion > versionRef.current) {
        versionRef.current = snapshotVersion;
      }
      const shouldActivate =
        applyOptions.activate ??
        currentOptions.shouldActivateSnapshot?.(snapshot) ??
        false;
      if (shouldActivate) {
        currentOptions.onActivate?.();
      }
      setProjection((current) =>
        stabilizeListenPlaybackProjection(
          current,
          deriveListenPlaybackProjection(
            snapshot,
            true,
            currentOptions.httpBaseURL ?? "",
          ),
        ),
      );
    },
    [],
  );
  const reset = React.useCallback(() => {
    versionRef.current = 0;
    setProjection(emptyProjection(true));
  }, []);

  React.useEffect(() => {
    let cancelled = false;
    void callListenPlaybackSnapshot()
      .then((snapshot) => {
        if (
          cancelled ||
          (!snapshot.currentTrack && snapshot.queue.length === 0)
        ) {
          return;
        }
        applySnapshot(snapshot, { activate: true });
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) {
          setProjection((current) => ({ ...current, hydrated: true }));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [applySnapshot]);

  React.useEffect(
    () => subscribeListenPlaybackSnapshots((snapshot) => applySnapshot(snapshot)),
    [applySnapshot],
  );

  return { projection, applySnapshot, reset };
}
