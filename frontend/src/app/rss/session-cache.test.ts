import { describe, expect, test } from "bun:test";

import {
  BoundedSessionLRU,
  buildRSSScrollCacheKey,
  readRSSSelectedEntryID,
  RSSImageSessionCache,
  writeRSSSelectedEntryID,
} from "./session-cache";

describe("BoundedSessionLRU", () => {
  test("evicts the least recently used value and bounds memory", () => {
    const cache = new BoundedSessionLRU<string, number>(2);
    cache.set("first", 1);
    cache.set("second", 2);
    expect(cache.get("first")).toBe(1);

    cache.set("third", 3);

    expect(cache.size).toBe(2);
    expect(cache.get("second")).toBeUndefined();
    expect(cache.get("first")).toBe(1);
    expect(cache.get("third")).toBe(3);
  });
});

describe("RSSImageSessionCache", () => {
  test("keeps successful URLs for the session", () => {
    const cache = new RSSImageSessionCache(2, 100);
    cache.markLoaded("https://example.test/cover.jpg");

    expect(cache.get("https://example.test/cover.jpg", 50)).toEqual({
      status: "loaded",
    });
    expect(cache.get("https://example.test/cover.jpg", 5_000)).toEqual({
      status: "loaded",
    });
  });

  test("uses a short negative TTL and permits a retry after expiry", () => {
    const cache = new RSSImageSessionCache(2, 100);
    cache.markFailed("https://example.test/missing.jpg", 1_000);

    expect(cache.get("https://example.test/missing.jpg", 1_099)).toEqual({
      status: "failed",
      retryAt: 1_100,
    });
    expect(cache.get("https://example.test/missing.jpg", 1_100)).toBeUndefined();
    expect(cache.size).toBe(0);
  });
});

describe("buildRSSScrollCacheKey", () => {
  test("is stable while isolating routes, subscriptions, and filters", () => {
    const base = {
      routeId: "articles",
      presentation: "article",
      subscriptionId: "feed:one",
      filter: "all",
    };

    expect(buildRSSScrollCacheKey(base)).toBe(buildRSSScrollCacheKey(base));
    expect(buildRSSScrollCacheKey(base)).not.toBe(
      buildRSSScrollCacheKey({ ...base, routeId: "all" }),
    );
    expect(buildRSSScrollCacheKey(base)).not.toBe(
      buildRSSScrollCacheKey({ ...base, subscriptionId: "feed:two" }),
    );
    expect(buildRSSScrollCacheKey(base)).not.toBe(
      buildRSSScrollCacheKey({ ...base, filter: "unread" }),
    );
  });

  test("does not collide when values contain delimiter-like text", () => {
    expect(
      buildRSSScrollCacheKey({ routeId: "a:b", subscriptionId: "c" }),
    ).not.toBe(
      buildRSSScrollCacheKey({ routeId: "a", subscriptionId: "b:c" }),
    );
  });
});

describe("RSS selected entry session cache", () => {
  test("restores selection per collection key and clears an empty selection", () => {
    const all = buildRSSScrollCacheKey({ routeId: "rss-all", presentation: "all" });
    const articles = buildRSSScrollCacheKey({ routeId: "rss-articles", presentation: "article" });

    writeRSSSelectedEntryID(all, "entry-all");
    writeRSSSelectedEntryID(articles, "entry-article");
    expect(readRSSSelectedEntryID(all)).toBe("entry-all");
    expect(readRSSSelectedEntryID(articles)).toBe("entry-article");

    writeRSSSelectedEntryID(all, "  ");
    expect(readRSSSelectedEntryID(all)).toBe("");
    expect(readRSSSelectedEntryID(articles)).toBe("entry-article");
  });
});
