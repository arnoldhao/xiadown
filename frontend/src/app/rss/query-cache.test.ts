import { describe, expect, test } from "bun:test";
import {
  InfiniteQueryObserver,
  QueryClient,
  QueryObserver,
  type InfiniteData,
} from "@tanstack/react-query";

import {
  applyRSSOptimisticEntryMutation,
  applyRSSOptimisticSubscriptionUpdate,
  commitRSSAddedSubscription,
  commitRSSUpdatedSubscription,
  createRSSMarkAllReadCacheCallbacks,
  reconcileRSSEntryStateCaches,
  reconcileRSSEntryCollectionSnapshots,
  resetRSSBackfillCaches,
  rollbackRSSOptimisticSubscriptionUpdate,
  rssEntryDetailQueryKey,
  RSS_ENTRIES_QUERY_ROOT,
  RSS_ENTRY_QUERY_ROOT,
  RSS_SUBSCRIPTIONS_QUERY_KEY,
} from "./query-cache";
import {
  RSS_DISCOVERY_GC_TIME_MS,
  RSS_DISCOVERY_STALE_TIME_MS,
  RSS_ENTRIES_GC_TIME_MS,
  RSS_ENTRIES_QUERY_POLICY,
  RSS_ENTRIES_REFETCH_INTERVAL_MS,
  RSS_ENTRIES_STALE_TIME_MS,
  RSS_ENTRY_DETAIL_GC_TIME_MS,
  RSS_ENTRY_DETAIL_QUERY_POLICY,
  RSS_ENTRY_DETAIL_STALE_TIME_MS,
  RSS_SEARCH_QUERY_POLICY,
  RSS_SUBSCRIPTIONS_GC_TIME_MS,
  RSS_SUBSCRIPTIONS_REFETCH_INTERVAL_MS,
  RSS_SUBSCRIPTIONS_STALE_TIME_MS,
  rssEntriesRefetchInterval,
  rssDiscoveryQueryPolicy,
} from "./query-policy";
import type {
  RSSBackfillHistoryResult,
  RSSEntry,
  RSSEntryPage,
  RSSEntryState,
  RSSSubscription,
} from "./types";

describe("RSS add-subscription query cache", () => {
  test("uses low-cost local polling without coupling to discovery refresh", () => {
    expect(RSS_SUBSCRIPTIONS_REFETCH_INTERVAL_MS).toBe(30_000);
    expect(RSS_ENTRIES_REFETCH_INTERVAL_MS).toBe(60_000);
    expect(RSS_SUBSCRIPTIONS_STALE_TIME_MS).toBe(30 * 60_000);
    expect(RSS_SUBSCRIPTIONS_GC_TIME_MS).toBe(2 * 60 * 60_000);
    expect(RSS_ENTRIES_STALE_TIME_MS).toBe(10 * 60_000);
    expect(RSS_ENTRIES_GC_TIME_MS).toBe(60 * 60_000);
    expect(RSS_ENTRY_DETAIL_STALE_TIME_MS).toBe(30 * 60_000);
    expect(RSS_ENTRY_DETAIL_GC_TIME_MS).toBe(2 * 60 * 60_000);
    expect(RSS_DISCOVERY_STALE_TIME_MS).toBe(24 * 60 * 60_000);
    expect(RSS_DISCOVERY_GC_TIME_MS).toBe(48 * 60 * 60_000);
    expect(rssDiscoveryQueryPolicy({ query: "bilibili" }).staleTime).toBe(
      10 * 60_000,
    );
    expect(rssDiscoveryQueryPolicy({}).staleTime).toBe(24 * 60 * 60_000);
    expect(rssDiscoveryQueryPolicy({ query: "bilibili" }).refetchOnMount).toBe(true);
    expect(rssDiscoveryQueryPolicy({}).refetchOnMount).toBe(true);
  });

  test("cancels late empty responses before committing and invalidating the new feed", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const lateSubscriptions = deferred<RSSSubscription[]>();
    const lateEntries = deferred<RSSEntryPage>();
    const entriesKey = [
      ...RSS_ENTRIES_QUERY_ROOT,
      "infinite",
      { limit: 80 },
    ] as const;

    const staleSubscriptionRequest = queryClient
      .fetchQuery({
        queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
        queryFn: () => lateSubscriptions.promise,
      })
      .catch(() => undefined);
    const staleEntryRequest = queryClient
      .fetchInfiniteQuery({
        queryKey: entriesKey,
        queryFn: () => lateEntries.promise,
        initialPageParam: 0,
        getNextPageParam: () => undefined,
      })
      .catch(() => undefined);
    const subscription = rssSubscription("new-feed");

    await commitRSSAddedSubscription(queryClient, subscription);
    lateSubscriptions.resolve([]);
    lateEntries.resolve({ items: [], total: 0 });
    await Promise.all([staleSubscriptionRequest, staleEntryRequest]);
    await Promise.resolve();

    expect(queryClient.getQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY)).toEqual([
      subscription,
    ]);
    expect(queryClient.getQueryState(entriesKey)?.isInvalidated).toBe(true);

    const freshEntry = rssEntry("entry-after-add", subscription.id);
    const fresh = await queryClient.fetchInfiniteQuery({
      queryKey: entriesKey,
      queryFn: async () => ({ items: [freshEntry], total: 1 }),
      initialPageParam: 0,
      getNextPageParam: () => undefined,
    });
    expect(fresh.pages[0]?.items).toEqual([freshEntry]);
    expect(queryClient.getQueryState(entriesKey)?.isInvalidated).toBe(false);

    queryClient.clear();
  });

  test("immediately refetches active collection lists and unread counts", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const subscription = rssSubscription("fresh-feed");
    const keys = [
      [...RSS_ENTRIES_QUERY_ROOT, { unreadOnly: true, limit: 1 }],
      [
        ...RSS_ENTRIES_QUERY_ROOT,
        { kind: "article", unreadOnly: true, limit: 1 },
      ],
      [...RSS_ENTRIES_QUERY_ROOT, "infinite", { limit: 80 }],
    ] as const;
    const refetches = [0, 0, 0];
    const unsubscribe = keys.map((queryKey, index) => {
      queryClient.setQueryData(queryKey, { items: [], total: 0 });
      const observer = new QueryObserver(queryClient, {
        queryKey,
        queryFn: async () => {
          refetches[index] += 1;
          return {
            items: [rssEntry(`fresh-entry-${index}`, subscription.id)],
            total: 1,
          };
        },
        staleTime: Infinity,
      });
      return observer.subscribe(() => undefined);
    });

    await commitRSSAddedSubscription(queryClient, subscription);

    expect(refetches).toEqual([1, 1, 1]);
    for (const queryKey of keys) {
      expect(queryClient.getQueryData<RSSEntryPage>(queryKey)?.total).toBe(1);
      expect(queryClient.getQueryState(queryKey)?.isInvalidated).toBe(false);
    }

    unsubscribe.forEach((stop) => stop());
    queryClient.clear();
  });
});

describe("RSS update-subscription query cache", () => {
  test("applies an explicit view type immediately and repairs a stale bridge result", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const current = {
      ...rssSubscription("view-type-feed"),
      viewType: "auto" as const,
      resolvedViewType: "article" as const,
    };
    queryClient.setQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY, [current]);

    const request = { id: current.id, viewType: "video" as const };
    await applyRSSOptimisticSubscriptionUpdate(queryClient, request);

    expect(
      queryClient.getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)?.[0],
    ).toMatchObject({ viewType: "video", resolvedViewType: "video" });

    await commitRSSUpdatedSubscription(
      queryClient,
      {
        ...current,
        viewType: "video",
        // Keep the cache resilient if an older or delayed bridge result still
        // carries the derived value from before this mutation.
        resolvedViewType: "article",
        revision: 2,
      },
      request,
    );

    expect(
      queryClient.getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)?.[0],
    ).toMatchObject({ viewType: "video", resolvedViewType: "video", revision: 2 });
    queryClient.clear();
  });

  test("switches auto to a neutral presentation until the list query derives it", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const current = {
      ...rssSubscription("auto-feed"),
      viewType: "video" as const,
      resolvedViewType: "video" as const,
    };
    queryClient.setQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY, [current]);
    const request = { id: current.id, viewType: "auto" as const };

    await applyRSSOptimisticSubscriptionUpdate(queryClient, request);
    await commitRSSUpdatedSubscription(
      queryClient,
      {
        ...current,
        viewType: "auto",
        resolvedViewType: "video",
        revision: 2,
      },
      request,
    );

    expect(
      queryClient.getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)?.[0],
    ).toMatchObject({ viewType: "auto", resolvedViewType: "auto" });
    queryClient.clear();
  });

  test("rolls back only the failed setting", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const current = {
      ...rssSubscription("rollback-feed"),
      resolvedViewType: "article" as const,
    };
    queryClient.setQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY, [current]);
    const request = { id: current.id, viewType: "image" as const };
    const context = await applyRSSOptimisticSubscriptionUpdate(
      queryClient,
      request,
    );

    await rollbackRSSOptimisticSubscriptionUpdate(
      queryClient,
      request,
      context,
    );

    expect(
      queryClient.getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)?.[0],
    ).toMatchObject({ viewType: "auto", resolvedViewType: "article" });
    queryClient.clear();
  });

  test("preserves fields changed in parallel while rolling back a partial patch", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const current = {
      ...rssSubscription("parallel-update-feed"),
      title: "Original title",
      enabled: true,
      resolvedViewType: "article" as const,
    };
    queryClient.setQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY, [current]);
    const request = { id: current.id, title: "Local title" };
    const context = await applyRSSOptimisticSubscriptionUpdate(
      queryClient,
      request,
    );

    queryClient.setQueryData<RSSSubscription[]>(
      RSS_SUBSCRIPTIONS_QUERY_KEY,
      (items) => (items ?? []).map((item) =>
        item.id === current.id ? { ...item, enabled: false } : item),
    );
    await rollbackRSSOptimisticSubscriptionUpdate(
      queryClient,
      request,
      context,
    );

    expect(
      queryClient.getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)?.[0],
    ).toMatchObject({
      title: "Original title",
      enabled: false,
      viewType: "auto",
      resolvedViewType: "article",
    });
    queryClient.clear();
  });

  test("applies and independently rolls back category placement", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const current = {
      ...rssSubscription("placed-feed"),
      categoryId: "category-old",
      sortOrder: 4,
    };
    queryClient.setQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY, [current]);
    const request = {
      id: current.id,
      categoryId: "category-new",
      sortOrder: 1,
    };
    const context = await applyRSSOptimisticSubscriptionUpdate(
      queryClient,
      request,
    );

    expect(
      queryClient.getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)?.[0],
    ).toMatchObject({ categoryId: "category-new", sortOrder: 1 });

    queryClient.setQueryData<RSSSubscription[]>(
      RSS_SUBSCRIPTIONS_QUERY_KEY,
      (items) => (items ?? []).map((item) => ({ ...item, enabled: false })),
    );
    await rollbackRSSOptimisticSubscriptionUpdate(
      queryClient,
      request,
      context,
    );

    expect(
      queryClient.getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)?.[0],
    ).toMatchObject({
      categoryId: "category-old",
      sortOrder: 4,
      enabled: false,
    });
    queryClient.clear();
  });
});

describe("RSS bulk-read query cache", () => {
  test("settled callback reconciles caches after a partially committed failure", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const activeEntriesKey = [
      ...RSS_ENTRIES_QUERY_ROOT,
      { unreadOnly: true, limit: 1 },
    ] as const;
    const activeEntryKey = [...RSS_ENTRY_QUERY_ROOT, "entry-1"] as const;
    const inactiveEntryKey = [...RSS_ENTRY_QUERY_ROOT, "entry-2"] as const;
    const refetches = { subscriptions: 0, entries: 0, entry: 0 };

    queryClient.setQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY, [
      rssSubscription("partially-read-feed"),
    ]);
    queryClient.setQueryData(activeEntriesKey, { items: [], total: 12 });
    queryClient.setQueryData(activeEntryKey, { id: "entry-1", readAt: undefined });
    queryClient.setQueryData(inactiveEntryKey, { id: "entry-2", readAt: undefined });

    const observers = [
      new QueryObserver(queryClient, {
        queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
        queryFn: async () => {
          refetches.subscriptions += 1;
          return [rssSubscription("partially-read-feed")];
        },
        staleTime: Infinity,
      }),
      new QueryObserver(queryClient, {
        queryKey: activeEntriesKey,
        queryFn: async () => {
          refetches.entries += 1;
          return { items: [], total: 0 };
        },
        staleTime: Infinity,
      }),
      new QueryObserver(queryClient, {
        queryKey: activeEntryKey,
        queryFn: async () => {
          refetches.entry += 1;
          return { id: "entry-1", readAt: "2026-07-14T08:00:00Z" };
        },
        staleTime: Infinity,
      }),
    ];
    const unsubscribe = observers.map((observer) =>
      observer.subscribe(() => undefined),
    );

    // `onSettled` runs for both success and error. In particular, a rejected
    // later batch must not hide rows already committed by earlier batches.
    await createRSSMarkAllReadCacheCallbacks(queryClient).onSettled();

    expect(refetches).toEqual({ subscriptions: 1, entries: 1, entry: 1 });
    expect(queryClient.getQueryData<RSSEntryPage>(activeEntriesKey)?.total).toBe(0);
    expect(queryClient.getQueryData<{ readAt?: string }>(activeEntryKey)?.readAt).toBe(
      "2026-07-14T08:00:00Z",
    );
    expect(queryClient.getQueryState(inactiveEntryKey)?.isInvalidated).toBe(true);

    unsubscribe.forEach((stop) => stop());
    queryClient.clear();
  });
});

describe("RSS collection snapshot reconciliation", () => {
  test("refetches an active All page when count polling observes newer video data", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const allKey = [
      ...RSS_ENTRIES_QUERY_ROOT,
      "infinite",
      { limit: 80 },
    ] as const;
    const article = rssEntry("entry-article", "subscription-mixed");
    const video = {
      ...rssEntry("entry-video", "subscription-mixed"),
      kind: "video" as const,
      title: "New background video",
    };
    queryClient.setQueryData<InfiniteData<RSSEntryPage>>(allKey, {
      pages: [{ items: [article], total: 1, snapshot: 10 }],
      pageParams: [0],
    });
    let refetches = 0;
    const observer = new InfiniteQueryObserver(queryClient, {
      queryKey: allKey,
      queryFn: async () => {
        refetches += 1;
        return { items: [video, article], total: 2, snapshot: 11 };
      },
      initialPageParam: 0,
      getNextPageParam: () => undefined,
      staleTime: Infinity,
    });
    const unsubscribe = observer.subscribe(() => undefined);

    await reconcileRSSEntryCollectionSnapshots(queryClient, 11);

    const refreshed = queryClient.getQueryData<InfiniteData<RSSEntryPage>>(allKey);
    expect(refetches).toBe(1);
    expect(refreshed?.pages[0]?.total).toBe(2);
    expect(refreshed?.pages[0]?.snapshot).toBe(11);
    expect(refreshed?.pages[0]?.items.map((item) => item.kind)).toEqual([
      "video",
      "article",
    ]);

    await reconcileRSSEntryCollectionSnapshots(queryClient, 11);
    expect(refetches).toBe(1);

    unsubscribe();
    queryClient.clear();
  });

});

describe("RSS route cache policy", () => {
  test("does not poll search caches and versions hydrated details by content revision", () => {
    expect(rssEntriesRefetchInterval({ query: "video" }, true)).toBeFalse();
    expect(rssEntriesRefetchInterval({ query: "   " }, true)).toBe(
      RSS_ENTRIES_REFETCH_INTERVAL_MS,
    );
    expect(rssEntriesRefetchInterval({}, false)).toBeFalse();
    expect(rssEntryDetailQueryKey(" entry-1 ", 4)).toEqual([
      "rss",
      "entry",
      "entry-1",
      4,
    ]);
    expect(rssEntryDetailQueryKey("entry-1", 5)).not.toEqual(
      rssEntryDetailQueryKey("entry-1", 4),
    );
  });

  test("hydrates a new detail query when a refreshed list advances content revision", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const oldKey = rssEntryDetailQueryKey("entry-revision", 1);
    const nextKey = rssEntryDetailQueryKey("entry-revision", 2);
    queryClient.setQueryData(oldKey, {
      ...rssEntry("entry-revision", "subscription-a"),
      title: "Old body",
      revision: 1,
    });
    let calls = 0;
    const observer = new QueryObserver(queryClient, {
      queryKey: nextKey,
      queryFn: async () => {
        calls += 1;
        return {
          ...rssEntry("entry-revision", "subscription-a"),
          title: "New body",
          revision: 2,
        };
      },
      ...RSS_ENTRY_DETAIL_QUERY_POLICY,
    });
    const unsubscribe = observer.subscribe(() => undefined);
    await waitFor(
      () => queryClient.getQueryData<RSSEntry>(nextKey)?.revision === 2,
    );

    expect(calls).toBe(1);
    expect(queryClient.getQueryData<RSSEntry>(oldKey)?.title).toBe("Old body");
    expect(queryClient.getQueryData<RSSEntry>(nextKey)?.title).toBe("New body");
    unsubscribe();
    queryClient.clear();
  });

  test("reuses A after A to B to A and only refetches an explicitly invalidated mount", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const keyA = [...RSS_ENTRIES_QUERY_ROOT, "infinite", { limit: 80 }] as const;
    const keyB = [
      ...RSS_ENTRIES_QUERY_ROOT,
      "infinite",
      { kind: "article", limit: 80 },
    ] as const;
    const calls = { a: 0, b: 0 };
    const mount = (queryKey: typeof keyA | typeof keyB) => {
      const observer = new QueryObserver(queryClient, {
        queryKey,
        queryFn: async () => {
          if (queryKey === keyA) calls.a += 1;
          else calls.b += 1;
          return { items: [], total: 0, snapshot: 1 };
        },
        ...RSS_ENTRIES_QUERY_POLICY,
      });
      return { observer, unsubscribe: observer.subscribe(() => undefined) };
    };

    let mounted = mount(keyA);
    await waitFor(() => calls.a === 1);
    mounted.unsubscribe();
    mounted = mount(keyB);
    await waitFor(() => calls.b === 1);
    mounted.unsubscribe();
    mounted = mount(keyA);
    await flushTasks();
    expect(calls).toEqual({ a: 1, b: 1 });
    mounted.unsubscribe();

    await queryClient.invalidateQueries({ queryKey: keyA, refetchType: "none" });
    mounted = mount(keyA);
    await waitFor(() => calls.a === 2);
    expect(calls).toEqual({ a: 2, b: 1 });

    mounted.unsubscribe();
    queryClient.clear();
  });

  test("keeps fresh search results but revalidates an expired search on return", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const freshKey = ["rss", "search", "fresh"] as const;
    const expiredKey = ["rss", "search", "expired"] as const;
    let freshCalls = 0;
    let expiredCalls = 0;
    queryClient.setQueryData(freshKey, "cached", { updatedAt: Date.now() });
    queryClient.setQueryData(expiredKey, "cached", {
      updatedAt: Date.now() - RSS_SEARCH_QUERY_POLICY.staleTime - 1,
    });

    const fresh = new QueryObserver(queryClient, {
      queryKey: freshKey,
      queryFn: async () => { freshCalls += 1; return "fresh"; },
      ...RSS_SEARCH_QUERY_POLICY,
    });
    const expired = new QueryObserver(queryClient, {
      queryKey: expiredKey,
      queryFn: async () => { expiredCalls += 1; return "refreshed"; },
      ...RSS_SEARCH_QUERY_POLICY,
    });
    const unsubscribeFresh = fresh.subscribe(() => undefined);
    const unsubscribeExpired = expired.subscribe(() => undefined);
    await waitFor(() => expiredCalls === 1);

    expect(freshCalls).toBe(0);
    expect(expiredCalls).toBe(1);
    unsubscribeFresh();
    unsubscribeExpired();
    queryClient.clear();
  });
});

describe("RSS entry-state cache precision", () => {
  test("automatic read marking does not cancel first detail hydration", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const entry = {
      ...rssEntry("entry-first-hydration", "subscription-video"),
      kind: "video" as const,
    };
    const listKey = infiniteEntriesKey({ kind: "video", limit: 80 });
    const detailKey = rssEntryDetailQueryKey(entry.id, entry.revision);
    const hydration = deferred<RSSEntry>();
    let aborted = false;
    queryClient.setQueryData(listKey, infinitePage(entry));
    const observer = new QueryObserver(queryClient, {
      queryKey: detailKey,
      queryFn: ({ signal }) => {
        signal.addEventListener("abort", () => { aborted = true; });
        return hydration.promise;
      },
      ...RSS_ENTRY_DETAIL_QUERY_POLICY,
    });
    const unsubscribe = observer.subscribe(() => undefined);
    await waitFor(
      () => queryClient.getQueryState(detailKey)?.fetchStatus === "fetching",
    );

    await applyRSSOptimisticEntryMutation(queryClient, {
      id: entry.id,
      field: "read",
      read: true,
      expectedRevision: 0,
      mutationId: "automatic-read",
    });

    expect(aborted).toBeFalse();
    expect(queryClient.getQueryState(detailKey)?.fetchStatus).toBe("fetching");
    hydration.resolve({ ...entry, contentHtml: "<p>Hydrated video detail</p>" });
    await waitFor(
      () => queryClient.getQueryData<RSSEntry>(detailKey)?.contentHtml !== undefined,
    );
    expect(queryClient.getQueryData<RSSEntry>(detailKey)?.contentHtml).toBe(
      "<p>Hydrated video detail</p>",
    );

    unsubscribe();
    queryClient.clear();
  });

  test("reapplies committed read state after an older first hydration lands", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const entry = {
      ...rssEntry("entry-stale-hydration", "subscription-video"),
      kind: "video" as const,
    };
    const listKey = infiniteEntriesKey({ kind: "video", limit: 80 });
    const detailKey = rssEntryDetailQueryKey(entry.id, entry.revision);
    const hydration = deferred<RSSEntry>();
    queryClient.setQueryData(listKey, infinitePage(entry));
    const observer = new QueryObserver(queryClient, {
      queryKey: detailKey,
      queryFn: () => hydration.promise,
      ...RSS_ENTRY_DETAIL_QUERY_POLICY,
    });
    const unsubscribe = observer.subscribe(() => undefined);
    await waitFor(
      () => queryClient.getQueryState(detailKey)?.fetchStatus === "fetching",
    );

    await applyRSSOptimisticEntryMutation(queryClient, {
      id: entry.id,
      field: "read",
      read: true,
      expectedRevision: 0,
      mutationId: "automatic-read",
    });
    await reconcileRSSEntryStateCaches(
      queryClient,
      rssState(entry.id, {
        read: true,
        readAt: "2026-07-14T09:00:00Z",
        readRevision: 1,
      }),
      "read",
    );

    hydration.resolve({
      ...entry,
      contentHtml: "<p>Older unread hydration</p>",
      readAt: undefined,
      fieldRevisions: {
        read: 0,
        starred: 0,
        articleProgress: 0,
        videoProgressSeconds: 0,
      },
    });
    await waitFor(
      () => queryClient.getQueryData<RSSEntry>(detailKey)?.contentHtml !== undefined,
    );

    const detail = queryClient.getQueryData<RSSEntry>(detailKey);
    expect(detail?.contentHtml).toBe("<p>Older unread hydration</p>");
    expect(detail?.readAt).toBe("2026-07-14T09:00:00Z");
    expect(detail?.fieldRevisions?.read).toBe(1);

    unsubscribe();
    queryClient.clear();
  });

  test("still cancels an in-flight detail refetch when cached body exists", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const entry = rssEntry("entry-cached-detail", "subscription-a");
    const detailKey = rssEntryDetailQueryKey(entry.id, entry.revision);
    const refetch = deferred<RSSEntry>();
    let aborted = false;
    queryClient.setQueryData(detailKey, {
      ...entry,
      contentHtml: "<p>Cached body</p>",
    });
    const observer = new QueryObserver(queryClient, {
      queryKey: detailKey,
      queryFn: ({ signal }) => {
        signal.addEventListener("abort", () => { aborted = true; });
        return refetch.promise;
      },
      ...RSS_ENTRY_DETAIL_QUERY_POLICY,
    });
    const unsubscribe = observer.subscribe(() => undefined);
    void observer.refetch();
    await waitFor(
      () => queryClient.getQueryState(detailKey)?.fetchStatus === "fetching",
    );

    await applyRSSOptimisticEntryMutation(queryClient, {
      id: entry.id,
      field: "read",
      read: true,
      expectedRevision: 0,
      mutationId: "cached-read",
    });

    expect(aborted).toBeTrue();
    expect(queryClient.getQueryState(detailKey)?.fetchStatus).toBe("idle");
    expect(queryClient.getQueryData<RSSEntry>(detailKey)?.contentHtml).toBe(
      "<p>Cached body</p>",
    );
    expect(queryClient.getQueryData<RSSEntry>(detailKey)?.readAt).toBeTruthy();

    unsubscribe();
    queryClient.clear();
  });

  test("optimistically patches list, detail and unread count without invalidating ordinary or unrelated lists", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const entry = rssEntry("entry-read", "subscription-a");
    const allKey = infiniteEntriesKey({ limit: 80 });
    const unreadKey = infiniteEntriesKey({ unreadOnly: true, limit: 80 });
    const articlesUnreadKey = infiniteEntriesKey({
      kind: "article",
      unreadOnly: true,
      limit: 80,
    });
    const videosUnreadKey = infiniteEntriesKey({
      kind: "video",
      unreadOnly: true,
      limit: 80,
    });
    const starredUnreadKey = infiniteEntriesKey({
      starredOnly: true,
      unreadOnly: true,
      limit: 80,
    });
    const unrelatedKey = infiniteEntriesKey({
      subscriptionId: "subscription-b",
      limit: 80,
    });
    const detailKey = rssEntryDetailQueryKey(entry.id, entry.revision);
    const page = infinitePage(entry);

    queryClient.setQueryData(allKey, page);
    queryClient.setQueryData(unreadKey, page);
    queryClient.setQueryData(articlesUnreadKey, page);
    queryClient.setQueryData(videosUnreadKey, infinitePage());
    queryClient.setQueryData(starredUnreadKey, infinitePage());
    queryClient.setQueryData(unrelatedKey, infinitePage(rssEntry("entry-b", "subscription-b")));
    queryClient.setQueryData(detailKey, entry);
    queryClient.setQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY, [
      rssSubscription("subscription-a"),
      rssSubscription("subscription-b"),
    ]);

    await applyRSSOptimisticEntryMutation(queryClient, {
      id: entry.id,
      field: "read",
      read: true,
      expectedRevision: 0,
      mutationId: "read-1",
    });

    expect(firstInfiniteEntry(queryClient, allKey)?.readAt).toBeTruthy();
    expect(queryClient.getQueryData<InfiniteData<RSSEntryPage>>(unreadKey)?.pages[0]?.items).toEqual([]);
    expect(queryClient.getQueryData<RSSEntry>(detailKey)?.readAt).toBeTruthy();
    expect(queryClient.getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)?.[0]?.unreadCount).toBe(0);

    await reconcileRSSEntryStateCaches(
      queryClient,
      rssState(entry.id, { read: true, readAt: "2026-07-14T09:00:00Z", readRevision: 1 }),
      "read",
    );

    expect(queryClient.getQueryState(allKey)?.isInvalidated).toBe(false);
    expect(queryClient.getQueryState(detailKey)?.isInvalidated).toBe(false);
    expect(queryClient.getQueryState(unrelatedKey)?.isInvalidated).toBe(false);
    expect(queryClient.getQueryState(unreadKey)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(articlesUnreadKey)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(videosUnreadKey)?.isInvalidated).toBe(false);
    expect(queryClient.getQueryState(starredUnreadKey)?.isInvalidated).toBe(false);
    expect(queryClient.getQueryState(RSS_SUBSCRIPTIONS_QUERY_KEY)?.isInvalidated).toBe(false);

    queryClient.clear();
  });

  test("optimistically patches starred and progress state across list and detail caches", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const entry = rssEntry("entry-state", "subscription-a");
    const listKey = infiniteEntriesKey({ limit: 80 });
    const starredKey = infiniteEntriesKey({ starredOnly: true, limit: 80 });
    const detailKey = rssEntryDetailQueryKey(entry.id, entry.revision);
    queryClient.setQueryData(listKey, infinitePage(entry));
    queryClient.setQueryData(starredKey, infinitePage());
    queryClient.setQueryData(detailKey, entry);

    await applyRSSOptimisticEntryMutation(queryClient, {
      id: entry.id,
      field: "starred",
      starred: true,
      expectedRevision: 0,
      mutationId: "star-1",
    });
    expect(firstInfiniteEntry(queryClient, listKey)?.starredAt).toBeTruthy();
    expect(queryClient.getQueryData<RSSEntry>(detailKey)?.starredAt).toBeTruthy();
    await reconcileRSSEntryStateCaches(
      queryClient,
      rssState(entry.id, { starred: true, starredAt: "2026-07-14T09:00:00Z", starredRevision: 1 }),
      "starred",
    );
    expect(queryClient.getQueryState(starredKey)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(false);

    await applyRSSOptimisticEntryMutation(queryClient, {
      id: entry.id,
      field: "articleProgress",
      articleProgress: { fraction: 0.6, contentRevision: 1 },
      expectedRevision: 0,
      mutationId: "progress-1",
    });
    expect(firstInfiniteEntry(queryClient, listKey)?.articleProgress?.fraction).toBe(0.6);
    expect(queryClient.getQueryData<RSSEntry>(detailKey)?.articleProgress?.fraction).toBe(0.6);
    expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(false);

    queryClient.clear();
  });
});

describe("RSS history cache scope", () => {
  test("resets aggregate and changed subscription pages while preserving unrelated subscription pages", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { gcTime: Infinity, retry: false } },
    });
    const aggregateKey = infiniteEntriesKey({ limit: 80 });
    const subscriptionAKey = infiniteEntriesKey({ subscriptionId: "subscription-a", limit: 80 });
    const subscriptionBKey = infiniteEntriesKey({ subscriptionId: "subscription-b", limit: 80 });
    const detailAKey = [...RSS_ENTRY_QUERY_ROOT, "entry-a"] as const;
    const detailBKey = [...RSS_ENTRY_QUERY_ROOT, "entry-b"] as const;
    queryClient.setQueryData(aggregateKey, infinitePage(rssEntry("entry-all", "subscription-a")));
    queryClient.setQueryData(subscriptionAKey, infinitePage(rssEntry("entry-a", "subscription-a")));
    queryClient.setQueryData(subscriptionBKey, infinitePage(rssEntry("entry-b", "subscription-b")));
    queryClient.setQueryData(detailAKey, rssEntry("entry-a", "subscription-a"));
    queryClient.setQueryData(detailBKey, rssEntry("entry-b", "subscription-b"));
    queryClient.setQueryData(RSS_SUBSCRIPTIONS_QUERY_KEY, [
      rssSubscription("subscription-a"),
      rssSubscription("subscription-b"),
    ]);

    await resetRSSBackfillCaches(queryClient, backfillResult([
      historySource("subscription-a", 1),
      historySource("subscription-b", 0),
    ]));

    expect(queryClient.getQueryData(aggregateKey)).toBeUndefined();
    expect(queryClient.getQueryData(subscriptionAKey)).toBeUndefined();
    expect(queryClient.getQueryData<InfiniteData<RSSEntryPage>>(subscriptionBKey)?.pages).toHaveLength(1);
    expect(queryClient.getQueryState(detailAKey)?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(detailBKey)?.isInvalidated).toBe(false);

    queryClient.clear();
  });
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((accept) => {
    resolve = accept;
  });
  return { promise, resolve };
}

function rssSubscription(id: string): RSSSubscription {
  return {
    id,
    workspaceId: "rss-default",
    feedUrl: `https://example.com/${id}.xml`,
    title: "New feed",
    viewType: "auto",
    enabled: true,
    unreadCount: 1,
    createdAt: "2026-07-14T00:00:00Z",
    updatedAt: "2026-07-14T00:00:00Z",
    revision: 1,
  };
}

function rssEntry(id: string, subscriptionId: string): RSSEntry {
  return {
    id,
    subscriptionId,
    externalId: id,
    title: "Entry after add",
    kind: "article",
    imageUrls: [],
    media: [],
    stateRevision: 0,
    revision: 1,
    createdAt: "2026-07-14T00:00:00Z",
    modifiedAt: "2026-07-14T00:00:00Z",
  };
}

function infiniteEntriesKey(request: Record<string, unknown>) {
  return [...RSS_ENTRIES_QUERY_ROOT, "infinite", request] as const;
}

function infinitePage(...entries: RSSEntry[]): InfiniteData<RSSEntryPage> {
  return {
    pages: [{ items: entries, total: entries.length, snapshot: 1 }],
    pageParams: [0],
  };
}

function firstInfiniteEntry(
  queryClient: QueryClient,
  queryKey: readonly unknown[],
) {
  return queryClient.getQueryData<InfiniteData<RSSEntryPage>>(queryKey)?.pages[0]
    ?.items[0];
}

function rssState(
  entryId: string,
  options: {
    read?: boolean;
    readAt?: string;
    starred?: boolean;
    starredAt?: string;
    articleProgress?: RSSEntryState["articleProgress"];
    videoProgressSeconds?: number;
    videoDurationSeconds?: number;
    readRevision?: number;
    starredRevision?: number;
    articleProgressRevision?: number;
    videoProgressSecondsRevision?: number;
  } = {},
): RSSEntryState {
  const fieldRevisions = {
    read: options.readRevision ?? 0,
    starred: options.starredRevision ?? 0,
    articleProgress: options.articleProgressRevision ?? 0,
    videoProgressSeconds: options.videoProgressSecondsRevision ?? 0,
  };
  return {
    entryId,
    subjectId: "rss-default",
    read: Boolean(options.read),
    readAt: options.readAt,
    starred: Boolean(options.starred),
    starredAt: options.starredAt,
    articleProgress: options.articleProgress,
    videoProgressSeconds: options.videoProgressSeconds,
    videoDurationSeconds: options.videoDurationSeconds,
    videoCompleted: Boolean(
      options.videoDurationSeconds &&
        options.videoProgressSeconds !== undefined &&
        options.videoProgressSeconds >= options.videoDurationSeconds,
    ),
    fieldRevisions,
    revision: Math.max(...Object.values(fieldRevisions)),
    updatedAt: "2026-07-14T09:00:00Z",
  };
}

function historySource(
  subscriptionId: string,
  created: number,
): RSSBackfillHistoryResult["sources"][number] {
  return {
    subscriptionId,
    attempted: true,
    capability: "available",
    exhausted: false,
    noProgress: 0,
    created,
    updated: 0,
  };
}

function backfillResult(
  sources: RSSBackfillHistoryResult["sources"],
): RSSBackfillHistoryResult {
  const created = sources.reduce((total, source) => total + source.created, 0);
  const updated = sources.reduce((total, source) => total + source.updated, 0);
  return {
    subscriptions: sources.length,
    attempted: sources.filter((source) => source.attempted).length,
    supported: sources.filter((source) => source.capability === "available")
      .length,
    unsupported: sources.filter(
      (source) => source.capability === "unsupported",
    ).length,
    exhausted: sources.filter((source) => source.exhausted).length,
    created,
    updated,
    failed: sources.filter((source) => Boolean(source.error)).length,
    hasMore: sources.some((source) => !source.exhausted),
    sources,
  };
}

async function flushTasks() {
  await Promise.resolve();
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
}

async function waitFor(predicate: () => boolean) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (predicate()) return;
    await flushTasks();
  }
  throw new Error("Timed out waiting for the query state to settle");
}
