import { describe, expect, test } from "bun:test";

import {
  clearListenLocalQueueKeepingSelected,
  moveListenLocalQueueItem,
  normalizeListenLocalQueueIds,
  pruneListenLocalQueueIds,
  removeListenLocalQueueItem,
  shouldClearListenLocalSelection,
} from "@/app/main/listen/local-queue";

const items = [{ id: "a" }, { id: "b" }, { id: "c" }];

describe("local playback queue", () => {
  test("normalizes and prunes restored IDs without changing the default queue sentinel", () => {
    expect(normalizeListenLocalQueueIds([" a ", "a", "", "b"])).toEqual([
      "a",
      "b",
    ]);
    expect(pruneListenLocalQueueIds(null, new Set(["a"]))).toBeNull();
    expect(
      pruneListenLocalQueueIds(["missing", "b", "a"], new Set(["a", "b"])),
    ).toEqual(["b", "a"]);
  });

  test("does not erase a persisted selection while the local index is unavailable", () => {
    expect(
      shouldClearListenLocalSelection({
        selectedId: "persisted",
        loading: false,
        error: "listen local failed: HTTP 500",
        playableIds: new Set(),
      }),
    ).toBe(false);
    expect(
      shouldClearListenLocalSelection({
        selectedId: "stale",
        loading: false,
        error: "",
        playableIds: new Set(["available"]),
      }),
    ).toBe(true);
  });

  test("clear keeps the selected playing track", () => {
    expect(clearListenLocalQueueKeepingSelected(items, "b")).toEqual({
      queueIds: ["b"],
      selectedId: "b",
      changed: true,
    });
  });

  test("removing the selected track selects its nearest successor", () => {
    expect(removeListenLocalQueueItem(items, "b", "b")).toEqual({
      queueIds: ["a", "c"],
      selectedId: "c",
      changed: true,
    });
    expect(removeListenLocalQueueItem([{ id: "a" }], "a", "a")).toEqual({
      queueIds: [],
      selectedId: "",
      changed: true,
    });
  });

  test("moves an item while retaining the playing selection", () => {
    expect(moveListenLocalQueueItem(items, "c", "b", -1)).toEqual({
      queueIds: ["b", "a", "c"],
      selectedId: "c",
      changed: true,
    });
    expect(moveListenLocalQueueItem(items, "a", "a", -1).changed).toBe(false);
  });
});
