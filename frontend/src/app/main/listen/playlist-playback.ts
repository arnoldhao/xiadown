import {
  callListenPlaybackAppendToQueue,
  callListenPlaybackInsertAfterQueueItem,
  callListenPlaybackPlayQueue,
  createListenPlaybackQueueIdentity,
  type ListenPlaybackSnapshot,
} from "@/app/main/listen/playback-api";
import { isMissingListenArtistLabel } from "@/app/main/listen/playback-helpers";
import type {
  ListenOnlineItem,
  ListenPlaylistItem,
} from "@/app/main/listen/types";

const LISTEN_UNKNOWN_ARTIST = "Unknown Artist";

export type ListenPlaylistPlaybackCommand = {
  epoch: number;
  queueIdentity: string;
  completed: Promise<boolean>;
};

export type ListenPlaylistQueueActionKind =
  | "start"
  | "insert-next"
  | "append";

export type ListenPlaylistQueueAction = {
  kind: ListenPlaylistQueueActionKind;
  items: ListenOnlineItem[];
};

export type ListenPlaylistQueueActionCommand = {
  epoch: number;
  result: Promise<ListenPlaybackSnapshot | null>;
};

type StartListenPlaylistPlaybackOptions = {
  initialItems: ListenOnlineItem[];
  hasContinuation: boolean;
  playInitial: (
    items: ListenOnlineItem[],
  ) => ListenPlaylistPlaybackCommand | null;
  loadComplete: () => Promise<ListenOnlineItem[]>;
  isCurrent: (epoch: number) => boolean;
  appendRemaining: (
    items: ListenOnlineItem[],
    expectedQueueIdentity: string,
  ) => void;
};

type ListenPlaylistPlaybackCommandRunner = (
  operation: () => Promise<ListenPlaybackSnapshot>,
  options?: {
    clearForwardStack?: boolean;
    loading?: boolean;
    syncVolume?: boolean;
  },
) => Pick<ListenPlaylistPlaybackCommand, "epoch" | "completed"> | null;

type StartListenPlaylistPlaybackFromIndexOptions = {
  initialItems: ListenOnlineItem[];
  startingAt: number;
  hasContinuation: boolean;
  title: string;
  language: string;
  playlistId: string;
  loadComplete: () => Promise<ListenOnlineItem[]>;
  isCurrent: (epoch: number) => boolean;
  runCommand: ListenPlaylistPlaybackCommandRunner;
};

type StartListenPlaylistQueueActionOptions = {
  initialItems: ListenOnlineItem[];
  hasContinuation: boolean;
  enqueueInitial: (
    items: ListenOnlineItem[],
  ) => ListenPlaylistQueueActionCommand | null;
  loadComplete: () => Promise<ListenOnlineItem[]>;
  isCurrent: (epoch: number) => boolean;
  enqueueRemaining: (
    items: ListenOnlineItem[],
    initialSnapshot: ListenPlaybackSnapshot,
  ) => void;
};

function cleanListenPlaylistPlaybackArtist(value: string) {
  let artist = value.trim();
  if (!artist) {
    return "";
  }
  if (artist === "Album") {
    return LISTEN_UNKNOWN_ARTIST;
  }
  if (artist.startsWith("Album, ")) {
    artist = artist.slice(7).trim();
  }
  if (artist.includes("Album,")) {
    const parts = artist.split(/,(.*)/s);
    if (parts[1]) {
      artist = parts[1].trim();
    }
  }
  if (isMissingListenArtistLabel(artist)) {
    return LISTEN_UNKNOWN_ARTIST;
  }
  return artist;
}

export function applyListenPlaylistPlaybackFallback(
  items: ListenOnlineItem[],
  fallbackArtist: string,
) {
  const cleanedFallback = cleanListenPlaylistPlaybackArtist(fallbackArtist);
  return items.map((item) => {
    let channel = item.channel.trim();
    if (channel === "Album") {
      channel = "";
    } else if (channel.startsWith("Album, ")) {
      channel = channel.slice(7).trim();
    }
    if (isMissingListenArtistLabel(channel)) {
      channel = "";
    }
    if (!channel && cleanedFallback) {
      channel = cleanedFallback;
    }
    return channel === item.channel ? item : { ...item, channel };
  });
}

export function resolveListenPlaylistPlaybackFallbackArtist(
  playlist: ListenPlaylistItem,
  detailAuthor: string,
) {
  const author = detailAuthor.trim();
  if (author && !isMissingListenArtistLabel(author)) {
    return author;
  }
  const channel = playlist.channel.trim();
  if (isMissingListenArtistLabel(channel)) {
    return playlist.description.trim();
  }
  const normalizedChannel = channel.toLowerCase();
  if (
    normalizedChannel === "album" ||
    normalizedChannel === "专辑" ||
    normalizedChannel === "專輯" ||
    normalizedChannel === "single" ||
    normalizedChannel === "单曲" ||
    normalizedChannel === "單曲" ||
    normalizedChannel === "ep"
  ) {
    return playlist.description.trim() || channel;
  }
  return channel;
}

export function isListenPlaylistPlaybackDisabled(options: {
  loading: boolean;
  appending: boolean;
  itemCount: number;
}) {
  // Continuation loading happens in the background and must not swallow a
  // user's play/retry click for tracks that are already visible.
  void options.appending;
  return options.loading || options.itemCount <= 0;
}

function playlistPlaybackItemKey(item: ListenOnlineItem) {
  return `${item.id}\u0000${item.videoId}`;
}

export function listenPlaylistPlaybackRemainder(
  initialItems: readonly ListenOnlineItem[],
  completeItems: readonly ListenOnlineItem[],
) {
  const initialKeys = new Set(initialItems.map(playlistPlaybackItemKey));
  return completeItems.filter(
    (item) => !initialKeys.has(playlistPlaybackItemKey(item)),
  );
}

/**
 * Resolves a playlist queue click without comparing it to the active queue.
 * Explicit queue actions are allowed to add the same song more than once; the
 * playback service assigns stable, unique row IDs to those duplicate entries.
 */
export function resolveListenPlaylistQueueAction(options: {
  items: readonly ListenOnlineItem[];
  hasActiveQueue: boolean;
  placement: "next" | "end";
}): ListenPlaylistQueueAction | null {
  if (options.items.length === 0) {
    return null;
  }
  return {
    kind: options.hasActiveQueue
      ? options.placement === "next"
        ? "insert-next"
        : "append"
      : "start",
    items: [...options.items],
  };
}

/**
 * Starts with the tracks already visible on the playlist page, then lets the
 * caller place continuation tracks after pagination completes. Network paging
 * must never make an explicit queue click appear inert.
 */
export function startListenPlaylistQueueAction(
  options: StartListenPlaylistQueueActionOptions,
) {
  const command = options.enqueueInitial(options.initialItems);
  if (!command || !options.hasContinuation) {
    return command;
  }

  void Promise.all([command.result, options.loadComplete()])
    .then(([snapshot, completeItems]) => {
      if (!snapshot || !options.isCurrent(command.epoch)) {
        return;
      }
      const remainingItems = listenPlaylistPlaybackRemainder(
        options.initialItems,
        completeItems,
      );
      if (remainingItems.length > 0) {
        options.enqueueRemaining(remainingItems, snapshot);
      }
    })
    .catch(() => {});

  return command;
}

export async function placeListenPlaylistQueueContinuation(options: {
  kind: ListenPlaylistQueueActionKind;
  initialItemCount: number;
  remainingItems: ListenOnlineItem[];
  initialSnapshot: ListenPlaybackSnapshot;
}) {
  const expectedQueueIdentity = options.initialSnapshot.queueIdentity.trim();
  if (!expectedQueueIdentity) {
    return options.initialSnapshot;
  }
  if (options.kind !== "insert-next") {
    return callListenPlaybackAppendToQueue(options.remainingItems, {
      expectedQueueIdentity,
    });
  }
  const anchorIndex =
    options.initialSnapshot.currentIndex + options.initialItemCount;
  const anchorTrackID = options.initialSnapshot.queue[anchorIndex]?.id ?? "";
  if (!anchorTrackID) {
    return options.initialSnapshot;
  }
  return callListenPlaybackInsertAfterQueueItem(options.remainingItems, {
    anchorTrackId: anchorTrackID,
    expectedQueueIdentity,
  });
}

export function startListenPlaylistPlayback(
  options: StartListenPlaylistPlaybackOptions,
) {
  // This call is intentionally synchronous: the visible playlist page must
  // enter playback before any continuation network request can delay it.
  const command = options.playInitial(options.initialItems);
  if (!command || !options.hasContinuation) {
    return command;
  }

  void Promise.all([command.completed, options.loadComplete()])
    .then(([playbackStarted, completeItems]) => {
      const expectedQueueIdentity = command.queueIdentity.trim();
      if (
        !playbackStarted ||
        !expectedQueueIdentity ||
        !options.isCurrent(command.epoch)
      ) {
        return;
      }
      const remainingItems = listenPlaylistPlaybackRemainder(
        options.initialItems,
        completeItems,
      );
      if (remainingItems.length > 0) {
        options.appendRemaining(remainingItems, expectedQueueIdentity);
      }
    })
    .catch(() => {});

  return command;
}

export function startListenPlaylistPlaybackFromIndex(
  options: StartListenPlaylistPlaybackFromIndexOptions,
) {
  if (
    options.initialItems.length === 0 ||
    options.startingAt < 0 ||
    options.startingAt >= options.initialItems.length
  ) {
    return null;
  }
  const queueIdentity = createListenPlaybackQueueIdentity();
  return startListenPlaylistPlayback({
    initialItems: options.initialItems,
    hasContinuation: options.hasContinuation,
    playInitial: (items) => {
      const command = options.runCommand(
        () =>
          callListenPlaybackPlayQueue({
            tracks: items,
            startingAt: options.startingAt,
            title: options.title,
            language: options.language,
            kind: "playlist",
            playlistId: options.playlistId,
            queueIdentity,
          }),
        { loading: true },
      );
      return command ? { ...command, queueIdentity } : null;
    },
    loadComplete: options.loadComplete,
    isCurrent: options.isCurrent,
    appendRemaining: (items, expectedQueueIdentity) => {
      options.runCommand(
        () =>
          callListenPlaybackAppendToQueue(items, { expectedQueueIdentity }),
        { clearForwardStack: false, syncVolume: false },
      );
    },
  });
}
