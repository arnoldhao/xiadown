import { describe, expect, test } from "bun:test";

import {
  clearListenOnlineQueueKeepingSelected,
  createListenQueueTrack,
  moveListenOnlineQueueItem,
  pushListenForwardSkipIndex,
  removeListenOnlineQueueItem,
  resolveListenQueueNextAction,
  resolveListenQueuePreviousAction,
} from "@/app/main/listen/queue";
import type { ListenOnlineQueueState } from "@/app/main/listen/types";

function radioQueue(): ListenOnlineQueueState {
  return {
    kind: "radio",
    title: "Radio",
    seedVideoId: "a-video",
    items: [
      createListenQueueTrack("a", "a-video"),
      createListenQueueTrack("b", "b-video"),
      createListenQueueTrack("c", "c-video"),
    ],
  };
}

describe("listen queue edits", () => {
  test("clear keeps the selected item and radio seed aligned", () => {
    const result = clearListenOnlineQueueKeepingSelected(radioQueue(), "b");

    expect(result.changed).toBe(true);
    expect(result.selectedId).toBe("b");
    expect(result.queue.items.map((item) => item.id)).toEqual(["b"]);
    expect(result.queue.kind === "radio" ? result.queue.seedVideoId : "").toBe(
      "b-video",
    );
  });

  test("remove advances selection and preserves a queued radio seed when possible", () => {
    const result = removeListenOnlineQueueItem(radioQueue(), "b", "b");

    expect(result.selectedId).toBe("c");
    expect(result.queue.items.map((item) => item.id)).toEqual(["a", "c"]);
    expect(result.queue.kind === "radio" ? result.queue.seedVideoId : "").toBe(
      "a-video",
    );
  });

  test("move reorders without changing selection", () => {
    const result = moveListenOnlineQueueItem(radioQueue(), "b", "c", -1);

    expect(result.selectedId).toBe("b");
    expect(result.queue.items.map((item) => item.id)).toEqual(["a", "c", "b"]);
  });

  test("manual next advances in repeat mode instead of replaying", () => {
    expect(
      resolveListenQueueNextAction({
        length: 3,
        currentIndex: 1,
        playMode: "repeat",
        reason: "manual",
      }),
    ).toEqual({ type: "select", index: 2 });
  });

  test("natural ended replays in repeat mode", () => {
    expect(
      resolveListenQueueNextAction({
        length: 3,
        currentIndex: 1,
        playMode: "repeat",
        reason: "ended",
      }),
    ).toEqual({ type: "replay" });
  });

  test("order mode stops at queue end", () => {
    expect(
      resolveListenQueueNextAction({
        length: 3,
        currentIndex: 2,
        playMode: "order",
        reason: "ended",
      }),
    ).toEqual({ type: "stop" });
  });

  test("previous seeks to start before walking the forward skip stack", () => {
    expect(
      resolveListenQueuePreviousAction({
        length: 3,
        currentIndex: 1,
        currentTime: 12,
        forwardStack: [0],
      }),
    ).toEqual({ type: "seek-start", forwardStack: [0] });
  });

  test("previous consumes forward skip stack", () => {
    expect(
      resolveListenQueuePreviousAction({
        length: 3,
        currentIndex: 2,
        currentTime: 0,
        forwardStack: [0, 1],
      }),
    ).toEqual({ type: "select", index: 1, forwardStack: [0] });
  });

  test("forward skip stack records the index being left", () => {
    expect(pushListenForwardSkipIndex([], 0, 1, 3)).toEqual([0]);
    expect(pushListenForwardSkipIndex([0], 1, 1, 3)).toEqual([0]);
  });
});
