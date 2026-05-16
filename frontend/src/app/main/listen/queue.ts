import { dedupeOnlineItems } from "@/app/main/listen/storage";
import type { ListenOnlineItem,ListenOnlineQueueState,ListenPlayMode } from "@/app/main/listen/types";

export type ListenQueueEditSnapshot = {
  queue: ListenOnlineQueueState;
  selectedId: string;
};

export type ListenQueueEditResult = ListenQueueEditSnapshot & {
  changed: boolean;
};

export type ListenQueueAdvanceReason = "manual" | "ended";

export type ListenQueueNextAction =
  | { type: "select"; index: number }
  | { type: "replay" }
  | { type: "stop" };

export type ListenQueuePreviousAction =
  | { type: "select"; index: number; forwardStack: number[] }
  | { type: "seek-start"; forwardStack: number[] }
  | { type: "restart"; forwardStack: number[] }
  | { type: "none"; forwardStack: number[] };

export function limitListenQueueHistory(
  snapshots: ListenQueueEditSnapshot[],
  limit = 20,
) {
  return snapshots.slice(Math.max(0, snapshots.length - Math.max(1, limit)));
}

export function resolveListenQueueNextAction(options: {
  length: number;
  currentIndex: number;
  playMode: ListenPlayMode;
  reason: ListenQueueAdvanceReason;
}): ListenQueueNextAction {
  const length = Math.max(0, Math.floor(options.length));
  if (length <= 0) {
    return { type: "stop" };
  }
  const currentIndex = clampListenQueueIndex(options.currentIndex, length);
  if (options.reason === "ended" && options.playMode === "repeat") {
    return { type: "replay" };
  }
  if (options.playMode === "shuffle") {
    return { type: "select", index: resolveListenQueueRandomIndex(length, currentIndex) };
  }
  if (currentIndex < length - 1) {
    return { type: "select", index: currentIndex + 1 };
  }
  return { type: "stop" };
}

export function resolveListenQueuePreviousAction(options: {
  length: number;
  currentIndex: number;
  currentTime: number;
  forwardStack: number[];
}): ListenQueuePreviousAction {
  const length = Math.max(0, Math.floor(options.length));
  const forwardStack = options.forwardStack.filter(
    (index) => Number.isInteger(index) && index >= 0 && index < length,
  );
  if (length <= 0) {
    return { type: "none", forwardStack: [] };
  }
  if (options.currentTime > 3) {
    return { type: "seek-start", forwardStack };
  }
  const priorIndex = forwardStack[forwardStack.length - 1];
  if (priorIndex !== undefined) {
    return {
      type: "select",
      index: priorIndex,
      forwardStack: forwardStack.slice(0, -1),
    };
  }
  const currentIndex = clampListenQueueIndex(options.currentIndex, length);
  if (currentIndex > 0) {
    return { type: "select", index: currentIndex - 1, forwardStack };
  }
  return { type: "restart", forwardStack };
}

export function pushListenForwardSkipIndex(
  forwardStack: number[],
  fromIndex: number,
  toIndex: number,
  length: number,
) {
  if (
    length <= 0 ||
    fromIndex === toIndex ||
    fromIndex < 0 ||
    fromIndex >= length ||
    toIndex < 0 ||
    toIndex >= length
  ) {
    return forwardStack;
  }
  return [...forwardStack, fromIndex].slice(-20);
}

export function clearListenOnlineQueueKeepingSelected(
  queue: ListenOnlineQueueState,
  selectedId: string,
): ListenQueueEditResult {
  const selected =
    queue.items.find((item) => item.id === selectedId) ?? queue.items[0] ?? null;
  if (!selected) {
    return {
      queue: { kind: "none", title: "", items: [] },
      selectedId: "",
      changed: queue.kind !== "none" || queue.items.length > 0 || selectedId !== "",
    };
  }
  if (queue.items.length === 1 && queue.items[0]?.id === selected.id) {
    return { queue, selectedId: selected.id, changed: false };
  }
  if (queue.kind === "radio") {
    return {
      queue: {
        ...queue,
        items: [selected],
        seedVideoId: selected.videoId,
      },
      selectedId: selected.id,
      changed: true,
    };
  }
  return {
    queue: {
      ...queue,
      items: [selected],
    },
    selectedId: selected.id,
    changed: true,
  };
}

export function removeListenOnlineQueueItem(
  queue: ListenOnlineQueueState,
  selectedId: string,
  itemId: string,
): ListenQueueEditResult {
  const removeIndex = queue.items.findIndex((item) => item.id === itemId);
  if (removeIndex < 0) {
    return { queue, selectedId, changed: false };
  }
  const items = queue.items.filter((item) => item.id !== itemId);
  if (items.length === 0) {
    return {
      queue: { kind: "none", title: "", items: [] },
      selectedId: "",
      changed: true,
    };
  }
  const nextSelectedId =
    selectedId === itemId
      ? items[Math.min(removeIndex, items.length - 1)]?.id ?? ""
      : selectedId;
  if (queue.kind === "radio") {
    const seedStillQueued = items.some(
      (item) => item.videoId === queue.seedVideoId,
    );
    return {
      queue: {
        ...queue,
        items,
        seedVideoId: seedStillQueued
          ? queue.seedVideoId
          : items[0]?.videoId ?? queue.seedVideoId,
      },
      selectedId: nextSelectedId,
      changed: true,
    };
  }
  return {
    queue: { ...queue, items },
    selectedId: nextSelectedId,
    changed: true,
  };
}

export function moveListenOnlineQueueItem(
  queue: ListenOnlineQueueState,
  selectedId: string,
  itemId: string,
  direction: -1 | 1,
): ListenQueueEditResult {
  const fromIndex = queue.items.findIndex((item) => item.id === itemId);
  if (fromIndex < 0) {
    return { queue, selectedId, changed: false };
  }
  const toIndex = fromIndex + direction;
  if (toIndex < 0 || toIndex >= queue.items.length) {
    return { queue, selectedId, changed: false };
  }
  const items = [...queue.items];
  [items[fromIndex], items[toIndex]] = [items[toIndex], items[fromIndex]];
  const nextQueue = normalizeListenOnlineQueueSeed({
    ...queue,
    items: dedupeOnlineItems(items),
  });
  return {
    queue: nextQueue,
    selectedId,
    changed: true,
  };
}

export function normalizeListenOnlineQueueSeed(
  queue: ListenOnlineQueueState,
): ListenOnlineQueueState {
  if (queue.kind !== "radio") {
    return queue;
  }
  const seedStillQueued = queue.items.some(
    (item) => item.videoId === queue.seedVideoId,
  );
  if (seedStillQueued) {
    return queue;
  }
  return {
    ...queue,
    seedVideoId: queue.items[0]?.videoId ?? queue.seedVideoId,
  };
}

export function queueSnapshotChanged(
  left: ListenQueueEditSnapshot,
  right: ListenQueueEditSnapshot,
) {
  return (
    left.selectedId !== right.selectedId ||
    left.queue.kind !== right.queue.kind ||
    left.queue.title !== right.queue.title ||
    queueSourceKey(left.queue) !== queueSourceKey(right.queue) ||
    left.queue.items.length !== right.queue.items.length ||
    left.queue.items.some((item, index) => item.id !== right.queue.items[index]?.id)
  );
}

function queueSourceKey(queue: ListenOnlineQueueState) {
  if (queue.kind === "playlist") {
    return queue.playlistId;
  }
  if (queue.kind === "radio") {
    return queue.seedVideoId;
  }
  return "";
}

function clampListenQueueIndex(index: number, length: number) {
  if (length <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(length - 1, Math.floor(index)));
}

function resolveListenQueueRandomIndex(length: number, currentIndex: number) {
  if (length <= 1) {
    return 0;
  }
  const randomIndex = Math.floor(Math.random() * (length - 1));
  return randomIndex >= currentIndex ? randomIndex + 1 : randomIndex;
}

export function createListenQueueTrack(id: string, videoId = id): ListenOnlineItem {
  return {
    id,
    group: "playlist",
    videoId,
    title: id,
    channel: "",
    description: "",
    durationLabel: "",
  };
}
