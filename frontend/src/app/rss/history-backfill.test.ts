import { describe, expect, test } from "bun:test";

import {
  createRSSHistorySentinelGate,
  RSS_HISTORY_BACKFILL_PAGE_BUDGET,
  RSS_PENDING_HYDRATION_REFETCH_INTERVAL_MS,
  RSS_PENDING_HYDRATION_REFETCH_WINDOW_MS,
  rssBackfillChangeCount,
  rssBackfillFailureMessage,
  rssBackfillRequestForEntries,
  rssBackfillRequestKey,
  rssBackfillShouldStop,
  rssHistoryCollectionMetric,
  rssHistorySessionShouldStop,
  rssShouldFastPollPendingSubscription,
  rssSubscriptionHistoryReady,
} from "./history-backfill";
import type { RSSBackfillHistoryResult, RSSEntry, RSSEntryPage } from "./types";

function result(
  overrides: Partial<RSSBackfillHistoryResult> = {},
): RSSBackfillHistoryResult {
  return {
    subscriptions: 1,
    attempted: 1,
    supported: 1,
    unsupported: 0,
    exhausted: 0,
    created: 0,
    updated: 0,
    failed: 0,
    hasMore: false,
    sources: [],
    ...overrides,
  };
}

function page(ids: string[], total = ids.length): RSSEntryPage {
  return {
    items: ids.map((id) => ({ id }) as RSSEntry),
    total,
  };
}

describe("RSS history backfill pagination", () => {
  test("scopes all, kind, and subscription collections", () => {
    expect(rssBackfillRequestForEntries({ limit: 80 })).toEqual({});
    expect(rssBackfillRequestForEntries({ kind: "video", limit: 80 })).toEqual({ kind: "video" });
    expect(rssBackfillRequestForEntries({ subscriptionId: " feed ", kind: "article" })).toEqual({
      subscriptionId: "feed",
    });
    expect(rssBackfillRequestKey({ subscriptionId: "feed" })).toBe("subscription:feed");
  });

  test("never backfills search, unread-only, or starred collections", () => {
    expect(rssBackfillRequestForEntries({ query: "news" })).toBeNull();
    expect(rssBackfillRequestForEntries({ unreadOnly: true })).toBeNull();
    expect(rssBackfillRequestForEntries({ starredOnly: true })).toBeNull();
  });

  test("continues only when the backend made usable progress", () => {
    expect(rssBackfillShouldStop(result({ created: 12, hasMore: true }))).toBeFalse();
    expect(rssBackfillChangeCount(result({ created: 2, updated: 3 }))).toBe(5);
    expect(rssBackfillShouldStop(result({ created: 12, hasMore: false }))).toBeTrue();
    expect(rssBackfillShouldStop(result({ hasMore: true }))).toBeTrue();
    expect(rssBackfillShouldStop(result({
      created: 1,
      hasMore: true,
      sources: [{
        subscriptionId: "feed",
        attempted: true,
        capability: "available",
        exhausted: false,
        noProgress: 1,
        created: 1,
        updated: 0,
      }],
    }))).toBeFalse();
  });

  test("surfaces a retryable aggregate failure without inventing an end label", () => {
    expect(rssBackfillFailureMessage(result({
      failed: 1,
      hasMore: true,
      sources: [{
        subscriptionId: "feed",
        attempted: true,
        capability: "available",
        exhausted: false,
        noProgress: 1,
        created: 0,
        updated: 0,
        error: "upstream timeout",
      }],
    }))).toBe("upstream timeout");
  });

  test("continues a visible sentinel once for every settled generation", () => {
    const gate = createRSSHistorySentinelGate();
    gate.setVisible(true);

    expect(gate.tryAcquire({
      enabled: true,
      busy: false,
      continuation: "all:0",
    })).toBeTrue();
    expect(gate.tryAcquire({
      enabled: true,
      busy: false,
      continuation: "all:0",
    })).toBeFalse();
    expect(gate.tryAcquire({
      enabled: true,
      busy: true,
      continuation: "all:1",
    })).toBeFalse();
    expect(gate.tryAcquire({
      enabled: true,
      busy: false,
      continuation: "all:1",
    })).toBeTrue();
  });

  test("stops aggregate backfill when the active filtered collection did not grow", () => {
    const before = rssHistoryCollectionMetric([page(["article-1"], 1)]);
    const unchanged = rssHistoryCollectionMetric([page(["article-1"], 1)]);
    const grew = rssHistoryCollectionMetric([page(["article-1", "article-2"], 2)]);
    const progress = result({ created: 4, hasMore: true });

    expect(rssHistorySessionShouldStop(progress, before, unchanged, 1)).toBeTrue();
    expect(rssHistorySessionShouldStop(progress, before, grew, 1)).toBeFalse();
    expect(rssHistorySessionShouldStop(
      progress,
      before,
      grew,
      RSS_HISTORY_BACKFILL_PAGE_BUDGET,
    )).toBeTrue();
  });

  test("holds history while a new subscription hydrates and bounds fast polling", () => {
    expect(rssSubscriptionHistoryReady(undefined, undefined)).toBeTrue();
    expect(rssSubscriptionHistoryReady("feed", undefined)).toBeFalse();
    expect(rssSubscriptionHistoryReady("feed", "2026-07-14T00:00:00Z")).toBeTrue();
    expect(RSS_PENDING_HYDRATION_REFETCH_INTERVAL_MS).toBe(2_000);
    expect(RSS_PENDING_HYDRATION_REFETCH_WINDOW_MS).toBe(120_000);
    expect(rssShouldFastPollPendingSubscription({
      enabled: true,
      subscriptionId: "feed",
      lastSuccessAt: undefined,
      visibleEntries: 0,
      now: 1_000,
      deadline: 2_000,
    })).toBeTrue();
    expect(rssShouldFastPollPendingSubscription({
      enabled: true,
      subscriptionId: "feed",
      lastSuccessAt: undefined,
      visibleEntries: 1,
      now: 1_000,
      deadline: 2_000,
    })).toBeFalse();
    expect(rssShouldFastPollPendingSubscription({
      enabled: true,
      subscriptionId: "feed",
      lastSuccessAt: undefined,
      visibleEntries: 0,
      now: 2_000,
      deadline: 2_000,
    })).toBeFalse();
  });
});
