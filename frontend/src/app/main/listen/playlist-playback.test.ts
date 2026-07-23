import { describe, expect, test } from "bun:test";

import {
  isListenPlaylistPlaybackDisabled,
  listenPlaylistPlaybackRemainder,
  resolveListenPlaylistQueueAction,
  startListenPlaylistPlayback,
  startListenPlaylistQueueAction,
} from "@/app/main/listen/playlist-playback";
import type { ListenPlaybackSnapshot } from "@/app/main/listen/playback-api";
import type { ListenOnlineItem } from "@/app/main/listen/types";

function track(id: string): ListenOnlineItem {
  return {
    id,
    group: "playlist",
    videoId: `video-${id}`,
    title: `Track ${id}`,
    channel: "Artist",
    description: "",
    durationLabel: "3:00",
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
}

function playbackSnapshot(
  overrides: Partial<ListenPlaybackSnapshot> = {},
): ListenPlaybackSnapshot {
  return {
    version: 1,
    queueIdentity: "server:queue",
    state: "paused",
    progress: 0,
    duration: 0,
    volume: 1,
    muted: false,
    shuffleEnabled: false,
    repeatMode: "off",
    queue: [],
    queueKind: "playlist",
    queueTitle: "Queue",
    currentIndex: 0,
    showMiniPlayer: true,
    canUndoQueue: false,
    canRedoQueue: false,
    canAutoloadPending: false,
    ...overrides,
  };
}

describe("listen playlist playback startup", () => {
  test("keeps visible tracks playable while continuation pages append", () => {
    expect(
      isListenPlaylistPlaybackDisabled({
        loading: false,
        appending: true,
        itemCount: 2,
      }),
    ).toBe(false);
    expect(
      isListenPlaylistPlaybackDisabled({
        loading: true,
        appending: false,
        itemCount: 0,
      }),
    ).toBe(true);
  });

  test("starts the visible page before complete pagination resolves", async () => {
    const pagination = deferred<ListenOnlineItem[]>();
    const playback = deferred<boolean>();
    const calls: string[] = [];
    const initialItems = [track("one"), track("two")];

    const command = startListenPlaylistPlayback({
      initialItems,
      hasContinuation: true,
      playInitial: (items) => {
        calls.push(`play:${items.map((item) => item.id).join(",")}`);
        return {
          epoch: 7,
          queueIdentity: "client:queue-73",
          completed: playback.promise,
        };
      },
      loadComplete: () => {
        calls.push("paginate");
        return pagination.promise;
      },
      isCurrent: (epoch) => epoch === 7,
      appendRemaining: (items, expectedQueueIdentity) => {
        calls.push(
          `append:${items.map((item) => item.id).join(",")}:${expectedQueueIdentity}`,
        );
      },
    });

    expect(command?.epoch).toBe(7);
    expect(calls).toEqual(["play:one,two", "paginate"]);

    playback.resolve(true);
    pagination.resolve([...initialItems, track("three")]);
    await Promise.all([playback.promise, pagination.promise]);
    await Promise.resolve();

    expect(calls).toEqual([
      "play:one,two",
      "paginate",
      "append:three:client:queue-73",
    ]);
  });

  test("does not append a stale playlist after another playback wins", async () => {
    const initialItems = [track("one")];
    const appended: ListenOnlineItem[][] = [];

    startListenPlaylistPlayback({
      initialItems,
      hasContinuation: true,
      playInitial: () => ({
        epoch: 3,
        queueIdentity: "client:stale",
        completed: Promise.resolve(true),
      }),
      loadComplete: () => Promise.resolve([...initialItems, track("two")]),
      isCurrent: () => false,
      appendRemaining: (items) => appended.push(items),
    });
    await Promise.resolve();
    await Promise.resolve();

    expect(appended).toEqual([]);
  });

  test("never launches an unbound continuation append", async () => {
    const initialItems = [track("one")];
    const appended: ListenOnlineItem[][] = [];

    startListenPlaylistPlayback({
      initialItems,
      hasContinuation: true,
      playInitial: () => ({
        epoch: 4,
        queueIdentity: "",
        completed: Promise.resolve(true),
      }),
      loadComplete: () => Promise.resolve([...initialItems, track("two")]),
      isCurrent: () => true,
      appendRemaining: (items) => appended.push(items),
    });
    await Promise.resolve();
    await Promise.resolve();

    expect(appended).toEqual([]);
  });

  test("keeps repeated videos when their playlist row ids differ", () => {
    const first = track("first");
    const repeated = { ...track("repeated"), videoId: first.videoId };

    expect(
      listenPlaylistPlaybackRemainder([first], [first, repeated]),
    ).toEqual([repeated]);
  });

  test("keeps explicit duplicate rows when adding a playlist to an active queue", () => {
    const repeated = track("one");
    const items = [repeated, repeated];
    const action = resolveListenPlaylistQueueAction({
      items,
      hasActiveQueue: true,
      placement: "end",
    });

    expect(action?.kind).toBe("append");
    expect(action?.items).toEqual(items);
    expect(action?.items).not.toBe(items);
  });

  test("maps play-next and an empty player to their concrete queue operations", () => {
    expect(
      resolveListenPlaylistQueueAction({
        items: [track("one")],
        hasActiveQueue: true,
        placement: "next",
      })?.kind,
    ).toBe("insert-next");
    expect(
      resolveListenPlaylistQueueAction({
        items: [track("one")],
        hasActiveQueue: false,
        placement: "end",
      })?.kind,
    ).toBe("start");
    expect(
      resolveListenPlaylistQueueAction({
        items: [],
        hasActiveQueue: true,
        placement: "end",
      }),
    ).toBeNull();
  });

  test("queues visible rows before a continuation request resolves", async () => {
    const pagination = deferred<ListenOnlineItem[]>();
    const initialResult = deferred<ListenPlaybackSnapshot | null>();
    const calls: string[] = [];
    const initialItems = [track("one"), track("two")];

    const command = startListenPlaylistQueueAction({
      initialItems,
      hasContinuation: true,
      enqueueInitial: (items) => {
        calls.push(`initial:${items.map((item) => item.id).join(",")}`);
        return { epoch: 11, result: initialResult.promise };
      },
      loadComplete: () => {
        calls.push("paginate");
        return pagination.promise;
      },
      isCurrent: (epoch) => epoch === 11,
      enqueueRemaining: (items, snapshot) => {
        calls.push(
          `remaining:${items.map((item) => item.id).join(",")}:${snapshot.queueIdentity}`,
        );
      },
    });

    expect(command?.epoch).toBe(11);
    expect(calls).toEqual(["initial:one,two", "paginate"]);

    initialResult.resolve(playbackSnapshot({ queueIdentity: "server:active" }));
    pagination.resolve([...initialItems, track("three")]);
    await Promise.all([initialResult.promise, pagination.promise]);
    await Promise.resolve();

    expect(calls).toEqual([
      "initial:one,two",
      "paginate",
      "remaining:three:server:active",
    ]);
  });

  test("does not append continuation rows after a newer queue action wins", async () => {
    const remaining: ListenOnlineItem[][] = [];
    startListenPlaylistQueueAction({
      initialItems: [track("one")],
      hasContinuation: true,
      enqueueInitial: () => ({
        epoch: 12,
        result: Promise.resolve(playbackSnapshot()),
      }),
      loadComplete: () => Promise.resolve([track("one"), track("two")]),
      isCurrent: () => false,
      enqueueRemaining: (items) => remaining.push(items),
    });
    await Promise.resolve();
    await Promise.resolve();

    expect(remaining).toEqual([]);
  });
});
