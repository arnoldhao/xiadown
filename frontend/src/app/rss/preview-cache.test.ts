import { describe, expect, test } from "bun:test";

import {
  RSSPreviewCache,
  rssPreviewCacheKey,
} from "./preview-cache";

import type { RSSPreviewResult } from "./types";

function preview(token: string): RSSPreviewResult {
  return {
    entries: [],
    previewToken: token,
    resolvedUrl: "https://example.com/feed",
    subscription: {
      description: "",
      enabled: true,
      feedUrl: "https://example.com/feed",
      id: "preview",
      revision: 1,
      siteUrl: "https://example.com",
      title: "Example",
      unreadCount: 0,
      updatedAt: "2026-07-14T00:00:00Z",
      viewType: "article",
      workspaceId: "rss-default",
    },
  };
}

describe("RSS preview session cache", () => {
  test("coalesces in-flight requests and reuses a fresh preview", async () => {
    let calls = 0;
    let resolve!: (value: RSSPreviewResult) => void;
    const pending = new Promise<RSSPreviewResult>((done) => { resolve = done; });
    const cache = new RSSPreviewCache(async () => {
      calls++;
      return pending;
    });
    const request = { url: "https://example.com/feed", viewType: "article" as const };

    const first = cache.get(request);
    const second = cache.get(request);
    expect(first).toBe(second);
    resolve(preview("one"));
    expect((await first).previewToken).toBe("one");
    expect((await cache.get(request)).previewToken).toBe("one");
    expect(calls).toBe(1);
  });

  test("expires success before the backend lease and lets retry bypass failures", async () => {
    let now = 1_000;
    let calls = 0;
    const cache = new RSSPreviewCache(async () => {
      calls++;
      if (calls === 1) throw new Error("offline");
      return preview(`token-${calls}`);
    }, { failureTTL: 50, now: () => now, successTTL: 100 });
    const request = { url: "https://example.com/feed", viewType: "auto" as const };

    await expect(cache.get(request)).rejects.toThrow("offline");
    await expect(cache.get(request)).rejects.toThrow("offline");
    expect(calls).toBe(1);
    expect((await cache.get(request, { force: true })).previewToken).toBe("token-2");
    now += 101;
    expect((await cache.get(request)).previewToken).toBe("token-3");
  });

  test("uses URL and view type in the key and evicts least-recently-used entries", async () => {
    let calls = 0;
    const cache = new RSSPreviewCache(async () => preview(String(++calls)), {
      maxEntries: 2,
    });
    const first = { url: " https://example.com/a ", viewType: "article" as const };
    const second = { url: "https://example.com/b", viewType: "article" as const };
    const third = { url: "https://example.com/c", viewType: "article" as const };

    await cache.get(first);
    await cache.get(second);
    await cache.get(first);
    await cache.get(third);
    await cache.get(second);
    expect(calls).toBe(4);
    expect(rssPreviewCacheKey(first)).toBe("article\u0000https://example.com/a");
    expect(rssPreviewCacheKey({ url: "feed://example.com/a", viewType: "article" })).toBe(
      rssPreviewCacheKey({ url: "https://example.com/a", viewType: "article" }),
    );
    expect(rssPreviewCacheKey({ ...first, viewType: "social" })).not.toBe(
      rssPreviewCacheKey(first),
    );
  });
});
