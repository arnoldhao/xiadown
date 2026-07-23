import { describe, expect, test } from "bun:test";

import {
  reconcileRSSBulkSelection,
  runRSSBulkAction,
  setRSSVisibleSelection,
  toggleRSSBulkSelection,
} from "./rss-bulk-actions";

describe("RSS subscription bulk actions", () => {
  test("scopes select-all and clear-all to the visible result set", () => {
    const initial = new Set(["hidden", "visible-a"]);
    const selected = setRSSVisibleSelection(
      initial,
      ["visible-a", "visible-b"],
      true,
    );
    expect([...selected]).toEqual(["visible-a", "visible-b"]);
    expect([
      ...setRSSVisibleSelection(selected, ["visible-a", "visible-b"], false),
    ]).toEqual([]);
    expect([...toggleRSSBulkSelection(selected, "visible-a")]).toEqual([
      "visible-b",
    ]);
  });

  test("drops stale IDs after subscriptions change", () => {
    expect([
      ...reconcileRSSBulkSelection(
        new Set(["feed-a", "deleted", " feed-b "]),
        ["feed-b", "feed-a"],
      ),
    ]).toEqual(["feed-a", "feed-b"]);
  });

  test("bounds concurrency, de-duplicates IDs, and preserves result order", async () => {
    let active = 0;
    let maxActive = 0;
    const result = await runRSSBulkAction(
      ["a", "b", "a", "bad", "c", ""],
      async (id) => {
        active += 1;
        maxActive = Math.max(maxActive, active);
        await Promise.resolve();
        active -= 1;
        if (id === "bad") throw new Error("failed");
      },
      2,
    );

    expect(maxActive).toBeLessThanOrEqual(2);
    expect(result.requested).toBe(4);
    expect(result.succeededIDs).toEqual(["a", "b", "c"]);
    expect(result.failures.map((failure) => failure.id)).toEqual(["bad"]);
  });
});
