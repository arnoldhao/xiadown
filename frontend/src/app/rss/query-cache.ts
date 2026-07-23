import type {
  InfiniteData,
  Query,
  QueryClient,
  QueryKey,
} from "@tanstack/react-query";

import type {
  RSSBackfillHistoryRequest,
  RSSBackfillHistoryResult,
  RSSEntry,
  RSSEntryPage,
  RSSEntryState,
  RSSEntryStateField,
  RSSListEntriesRequest,
  RSSMarkAllReadRequest,
  RSSSetEntryStateRequest,
  RSSSubscription,
  RSSUpdateSubscriptionRequest,
} from "./types";
import { applyRSSStateToEntry } from "./workspace-utils";

export const RSS_SUBSCRIPTIONS_QUERY_KEY = ["rss", "subscriptions"] as const;
export const RSS_ENTRIES_QUERY_ROOT = ["rss", "entries"] as const;
export const RSS_ENTRY_QUERY_ROOT = ["rss", "entry"] as const;

export function rssEntryDetailQueryKey(id: string, contentRevision = 0) {
  return [
    ...RSS_ENTRY_QUERY_ROOT,
    id.trim(),
    Math.max(0, Math.trunc(contentRevision)),
  ] as const;
}

type RSSEntryCollectionData = RSSEntryPage | InfiniteData<RSSEntryPage>;

interface RSSOptimisticEntryMutation {
  id: string;
  field: RSSEntryStateField;
  read?: boolean;
  starred?: boolean;
  articleProgress?: RSSSetEntryStateRequest["articleProgress"];
  videoProgressSeconds?: number;
  videoDurationSeconds?: number;
  expectedRevision?: number;
  mutationId?: string;
}

/**
 * The service refreshes feeds outside React Query. Unread-count polling can
 * therefore observe a newer SQLite snapshot while an active collection still
 * renders older infinite-query pages. Refetch only those collection/search
 * caches with at least one page predating the count snapshot.
 */
export async function reconcileRSSEntryCollectionSnapshots(
  queryClient: QueryClient,
  snapshot: number,
) {
  const normalizedSnapshot = nonNegativeSnapshot(snapshot);
  if (normalizedSnapshot <= 0) return;
  await queryClient.invalidateQueries({
    queryKey: RSS_ENTRIES_QUERY_ROOT,
    predicate: (query) => {
      if (query.queryKey[2] !== "infinite") return false;
      const data = query.state.data as InfiniteData<RSSEntryPage> | undefined;
      const pages = data?.pages;
      if (!pages?.length) return false;
      return pages.some(
        (page) => nonNegativeSnapshot(page.snapshot) < normalizedSnapshot,
      );
    },
    refetchType: "active",
  });
}

/**
 * Commits a newly-created subscription without allowing an older empty list
 * request to land after the mutation and make that empty result look fresh.
 * Existing single-subscription pages are unrelated and stay warm.
 */
export async function commitRSSAddedSubscription(
  queryClient: QueryClient,
  subscription: RSSSubscription,
) {
  const matches = (query: Query) =>
    rssEntryQueryIncludesSubscriptions(query.queryKey, new Set([subscription.id]));
  await Promise.all([
    queryClient.cancelQueries({ queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY }),
    queryClient.cancelQueries({
      queryKey: RSS_ENTRIES_QUERY_ROOT,
      predicate: matches,
    }),
  ]);

  queryClient.setQueryData<RSSSubscription[]>(
    RSS_SUBSCRIPTIONS_QUERY_KEY,
    (current) => upsertRSSSubscription(current, subscription),
  );

  await Promise.all([
    queryClient.invalidateQueries({
      queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
      refetchType: "active",
    }),
    queryClient.invalidateQueries({
      queryKey: RSS_ENTRIES_QUERY_ROOT,
      predicate: matches,
      refetchType: "active",
    }),
  ]);
}

export interface RSSOptimisticSubscriptionUpdate {
  previous?: RSSSubscription;
  optimistic?: RSSSubscription;
}

/**
 * Projects subscription settings into the singleton list before the bridge
 * request finishes. In particular, an explicit view type is also the resolved
 * presentation type; waiting for the next feed refresh to derive that value
 * makes the old layout linger after the user has changed it.
 */
export async function applyRSSOptimisticSubscriptionUpdate(
  queryClient: QueryClient,
  request: RSSUpdateSubscriptionRequest,
): Promise<RSSOptimisticSubscriptionUpdate> {
  const id = request.id.trim();
  if (!id) return {};
  await queryClient.cancelQueries({ queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY });
  const previous = queryClient
    .getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)
    ?.find((subscription) => subscription.id === id);
  if (!previous) return {};
  const optimistic = projectRSSSubscriptionUpdate(previous, request);
  queryClient.setQueryData<RSSSubscription[]>(
    RSS_SUBSCRIPTIONS_QUERY_KEY,
    (current) => upsertRSSSubscription(current, optimistic),
  );
  return { previous, optimistic };
}

/** Commits the bridge result while repairing its intentionally stale derived view. */
export async function commitRSSUpdatedSubscription(
  queryClient: QueryClient,
  subscription: RSSSubscription,
  request: RSSUpdateSubscriptionRequest,
) {
  const current = queryClient
    .getQueryData<RSSSubscription[]>(RSS_SUBSCRIPTIONS_QUERY_KEY)
    ?.find((item) => item.id === subscription.id);
  const reconciled = reconcileRSSSubscriptionUpdateResult(
    current,
    subscription,
    request,
  );
  queryClient.setQueryData<RSSSubscription[]>(
    RSS_SUBSCRIPTIONS_QUERY_KEY,
    (items) => upsertRSSSubscription(items, reconciled),
  );
  await Promise.all([
    queryClient.invalidateQueries({
      queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
      refetchType: "active",
    }),
    invalidateRSSEntryCachesForSubscriptions(
      queryClient,
      new Set([subscription.id]),
    ),
  ]);
}

/** Restores only fields owned by the failed mutation, then asks for authority. */
export async function rollbackRSSOptimisticSubscriptionUpdate(
  queryClient: QueryClient,
  request: RSSUpdateSubscriptionRequest,
  context: RSSOptimisticSubscriptionUpdate | undefined,
) {
  if (context?.previous && context.optimistic) {
    const { previous, optimistic } = context;
    queryClient.setQueryData<RSSSubscription[]>(
      RSS_SUBSCRIPTIONS_QUERY_KEY,
      (current) => (current ?? []).map((item) =>
        item.id === previous.id
          ? restoreRSSSubscriptionUpdateFields(
              item,
              previous,
              optimistic,
              request,
            )
          : item),
    );
  }
  await queryClient.invalidateQueries({
    queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
    refetchType: "active",
  });
}

/** Applies a provisional state immediately while the SQLite mutation runs. */
export async function applyRSSOptimisticEntryMutation(
  queryClient: QueryClient,
  request: RSSOptimisticEntryMutation,
) {
  const cachedEntry = findCachedRSSEntry(queryClient, request.id, request.field);
  if (!cachedEntry) return undefined;
  await Promise.all([
    queryClient.cancelQueries({
      queryKey: RSS_ENTRIES_QUERY_ROOT,
      predicate: (query) =>
        rssEntryCollectionContains(query.state.data, request.id) ||
        rssDerivedEntryQueryMatches(query, request.field, cachedEntry),
    }),
    queryClient.cancelQueries({
      queryKey: [...RSS_ENTRY_QUERY_ROOT, request.id],
      // Cancelling a first hydration leaves an enabled observer in
      // pending/idle and React Query will not automatically restart it. A
      // populated detail can still be cancelled safely because its cached
      // body remains available for the optimistic state patch.
      predicate: (query) => query.state.data !== undefined,
    }),
    ...(request.field === "read"
      ? [queryClient.cancelQueries({ queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY })]
      : []),
  ]);
  const optimisticState = optimisticRSSEntryState(cachedEntry, request);
  patchRSSEntryStateCaches(queryClient, optimisticState, request.field);
  return optimisticState;
}

/**
 * Replaces an optimistic projection with the authoritative repository state.
 * Only membership-derived queries are invalidated; ordinary collection and
 * detail caches remain warm.
 */
export async function reconcileRSSEntryStateCaches(
  queryClient: QueryClient,
  state: RSSEntryState,
  field: RSSEntryStateField,
) {
  const cachedEntry = findCachedRSSEntry(queryClient, state.entryId, field);
  reapplyRSSStateAfterPendingDetailHydration(queryClient, state);
  patchRSSEntryStateCaches(queryClient, state, field);
  const nextEntry = findCachedRSSEntry(queryClient, state.entryId, field) ?? cachedEntry;

  const tasks: Promise<unknown>[] = [];
  if (field === "read" || field === "starred") {
    tasks.push(queryClient.invalidateQueries({
      queryKey: RSS_ENTRIES_QUERY_ROOT,
      predicate: (query) => rssDerivedEntryQueryMatches(query, field, nextEntry),
      refetchType: "active",
    }));
  }
  if (field === "read" && !cachedEntry) {
    // Without a cached entry the subscription id and previous read state are
    // unknown, so the singleton subscription projection is the narrowest
    // authoritative fallback.
    tasks.push(queryClient.invalidateQueries({
      queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
      refetchType: "active",
    }));
  }
  if (field === "read") {
    tasks.push(
      queryClient.invalidateQueries({ queryKey: ["rss", "categories"] }),
      queryClient.invalidateQueries({ queryKey: ["rss", "collections"] }),
    );
  }
  await Promise.all(tasks);
}

/**
 * A first detail hydration cannot be cancelled for an optimistic mutation:
 * doing so leaves its active observer pending/idle without a replacement
 * request. If that older request completes after the repository commit, its
 * stale state projection would otherwise replace the authoritative state.
 * Reapply the committed snapshot after the body lands; field revisions keep
 * this callback from rolling back any still-newer mutation.
 */
function reapplyRSSStateAfterPendingDetailHydration(
  queryClient: QueryClient,
  state: RSSEntryState,
) {
  for (const query of queryClient.getQueryCache().findAll({
    queryKey: [...RSS_ENTRY_QUERY_ROOT, state.entryId],
  })) {
    const hydration = query.state.data === undefined ? query.promise : undefined;
    if (!hydration) continue;
    void hydration.then(
      () => {
        queryClient.setQueryData<RSSEntry>(
          query.queryKey,
          (current) => current ? applyRSSStateToEntry(current, state) : current,
        );
      },
      () => undefined,
    );
  }
}

/** Recovers only the optimistic entry and membership projections on failure. */
export async function invalidateFailedRSSEntryMutation(
  queryClient: QueryClient,
  entryID: string,
  field: RSSEntryStateField,
) {
  const cachedEntry = findCachedRSSEntry(queryClient, entryID, field);
  await Promise.all([
    queryClient.invalidateQueries({
      queryKey: RSS_ENTRIES_QUERY_ROOT,
      predicate: (query) =>
        rssEntryCollectionContains(query.state.data, entryID) ||
        rssDerivedEntryQueryMatches(query, field, cachedEntry),
      refetchType: "active",
    }),
    queryClient.invalidateQueries({
      queryKey: [...RSS_ENTRY_QUERY_ROOT, entryID],
      refetchType: "active",
    }),
    ...(field === "read"
      ? [queryClient.invalidateQueries({
          queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
          refetchType: "active" as const,
        })]
      : []),
  ]);
}

/**
 * Offset pages touched by history insertion must restart at page zero. Limit
 * that reset to aggregates and the subscriptions reported as changed by the
 * backend; unrelated subscriptions keep their pages and scroll history.
 */
export async function resetRSSBackfillCaches(
  queryClient: QueryClient,
  result: RSSBackfillHistoryResult,
  request: RSSBackfillHistoryRequest = {},
) {
  if (result.created <= 0 && result.updated <= 0) return;
  const changedIDs = new Set(
    result.sources
      .filter((source) => source.created > 0 || source.updated > 0)
      .map((source) => source.subscriptionId.trim())
      .filter(Boolean),
  );
  if (changedIDs.size === 0) {
    for (const source of result.sources) {
      const id = source.subscriptionId.trim();
      if (id) changedIDs.add(id);
    }
  }
  const requestedID = request.subscriptionId?.trim();
  if (changedIDs.size === 0 && requestedID) changedIDs.add(requestedID);

  await Promise.all([
    queryClient.resetQueries({
      queryKey: RSS_ENTRIES_QUERY_ROOT,
      predicate: (query) =>
        rssEntryQueryIncludesSubscriptions(query.queryKey, changedIDs),
    }),
    queryClient.invalidateQueries({
      queryKey: RSS_ENTRY_QUERY_ROOT,
      predicate: (query) => {
        const entry = query.state.data as RSSEntry | undefined;
        return Boolean(entry && changedIDs.has(entry.subscriptionId));
      },
      refetchType: "active",
    }),
    queryClient.invalidateQueries({
      queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
      refetchType: "active",
    }),
  ]);
}

/** Invalidates aggregate and matching subscription collections, not siblings. */
export async function invalidateRSSEntryCachesForSubscriptions(
  queryClient: QueryClient,
  subscriptionIDs?: ReadonlySet<string>,
) {
  await queryClient.invalidateQueries({
    queryKey: RSS_ENTRIES_QUERY_ROOT,
    predicate: subscriptionIDs
      ? (query) => rssEntryQueryIncludesSubscriptions(query.queryKey, subscriptionIDs)
      : undefined,
    refetchType: "active",
  });
}

export async function invalidateRSSEntryDetailsForSubscriptions(
  queryClient: QueryClient,
  subscriptionIDs?: ReadonlySet<string>,
) {
  await queryClient.invalidateQueries({
    queryKey: RSS_ENTRY_QUERY_ROOT,
    predicate: subscriptionIDs
      ? (query) => {
          const entry = query.state.data as RSSEntry | undefined;
          return Boolean(entry && subscriptionIDs.has(entry.subscriptionId));
        }
      : undefined,
    refetchType: "active",
  });
}

/** Reconciles only collections and details overlapped by a bulk-read scope. */
export async function invalidateRSSReadStateCaches(
  queryClient: QueryClient,
  request?: RSSMarkAllReadRequest,
) {
  const aggregateScope = Boolean(
    request?.categoryId || request?.collectionId,
  );
  await Promise.all([
    queryClient.invalidateQueries({
      queryKey: RSS_SUBSCRIPTIONS_QUERY_KEY,
      refetchType: "active",
    }),
    queryClient.invalidateQueries({
      queryKey: RSS_ENTRIES_QUERY_ROOT,
      predicate: request && !aggregateScope
        ? (query) => rssEntryQueryOverlapsMarkAllRead(query.queryKey, request)
        : undefined,
      refetchType: "active",
    }),
    queryClient.invalidateQueries({
      queryKey: RSS_ENTRY_QUERY_ROOT,
      predicate: request && !aggregateScope
        ? (query) => {
            const entry = query.state.data as RSSEntry | undefined;
            return Boolean(entry && rssEntryMatchesMarkAllRead(entry, request));
          }
        : undefined,
      refetchType: "active",
    }),
    queryClient.invalidateQueries({ queryKey: ["rss", "categories"] }),
    queryClient.invalidateQueries({ queryKey: ["rss", "collections"] }),
  ]);
}

export function createRSSMarkAllReadCacheCallbacks(queryClient: QueryClient) {
  return {
    onSettled: (
      _data?: unknown,
      _error?: unknown,
      request?: RSSMarkAllReadRequest,
    ) => invalidateRSSReadStateCaches(queryClient, request),
  };
}

export function upsertRSSSubscription(
  current: RSSSubscription[] | undefined,
  next: RSSSubscription,
) {
  return [...(current ?? []).filter((item) => item.id !== next.id), next].sort(
    (left, right) => {
      const category = (left.categoryId ?? "\uffff").localeCompare(
        right.categoryId ?? "\uffff",
      );
      if (category !== 0) return category;
      const position = (left.sortOrder ?? 0) - (right.sortOrder ?? 0);
      if (position !== 0) return position;
      return left.title.localeCompare(right.title, undefined, {
        sensitivity: "base",
      });
    },
  );
}

function projectRSSSubscriptionUpdate(
  current: RSSSubscription,
  request: RSSUpdateSubscriptionRequest,
): RSSSubscription {
  const title = request.title?.trim();
  const viewType = request.viewType;
  return {
    ...current,
    ...(title ? { title } : {}),
    ...(viewType
      ? {
          viewType,
          // Auto deliberately becomes neutral until the authoritative list
          // query recomputes the dominant entry type.
          resolvedViewType: viewType,
        }
      : {}),
    ...(request.enabled === undefined ? {} : { enabled: request.enabled }),
    ...(request.categoryId === undefined
      ? {}
      : { categoryId: request.categoryId.trim() || undefined }),
    ...(request.sortOrder === undefined
      ? {}
      : { sortOrder: Math.max(0, Math.trunc(request.sortOrder)) }),
  };
}

function reconcileRSSSubscriptionUpdateResult(
  current: RSSSubscription | undefined,
  result: RSSSubscription,
  request: RSSUpdateSubscriptionRequest,
): RSSSubscription {
  if (request.viewType) {
    return {
      ...result,
      viewType: request.viewType,
      resolvedViewType: request.viewType,
      ...(request.categoryId === undefined
        ? {}
        : { categoryId: request.categoryId.trim() || undefined }),
      ...(request.sortOrder === undefined
        ? {}
        : { sortOrder: Math.max(0, Math.trunc(request.sortOrder)) }),
    };
  }
  if (result.viewType !== "auto") {
    return { ...result, resolvedViewType: result.viewType };
  }
  return {
    ...result,
    resolvedViewType: current?.resolvedViewType ?? result.resolvedViewType,
  };
}

function restoreRSSSubscriptionUpdateFields(
  current: RSSSubscription,
  previous: RSSSubscription,
  optimistic: RSSSubscription,
  request: RSSUpdateSubscriptionRequest,
): RSSSubscription {
  let restored = current;
  if (request.title?.trim() && current.title === optimistic.title) {
    restored = { ...restored, title: previous.title };
  }
  if (
    request.viewType &&
    current.viewType === optimistic.viewType &&
    current.resolvedViewType === optimistic.resolvedViewType
  ) {
    restored = {
      ...restored,
      viewType: previous.viewType,
      resolvedViewType: previous.resolvedViewType,
    };
  }
  if (
    request.enabled !== undefined &&
    current.enabled === optimistic.enabled
  ) {
    restored = { ...restored, enabled: previous.enabled };
  }
  if (
    request.categoryId !== undefined &&
    current.categoryId === optimistic.categoryId
  ) {
    restored = { ...restored, categoryId: previous.categoryId };
  }
  if (
    request.sortOrder !== undefined &&
    current.sortOrder === optimistic.sortOrder
  ) {
    restored = { ...restored, sortOrder: previous.sortOrder };
  }
  return restored;
}

export function rssEntriesRequestFromQueryKey(
  queryKey: QueryKey,
): RSSListEntriesRequest | undefined {
  if (queryKey[0] !== "rss" || queryKey[1] !== "entries") return undefined;
  const candidate = queryKey[2] === "infinite" ? queryKey[3] : queryKey[2];
  return candidate && typeof candidate === "object" && !Array.isArray(candidate)
    ? candidate as RSSListEntriesRequest
    : undefined;
}

function patchRSSEntryStateCaches(
  queryClient: QueryClient,
  state: RSSEntryState,
  field: RSSEntryStateField,
) {
  const before = findCachedRSSEntry(queryClient, state.entryId, field);
  for (const query of queryClient.getQueryCache().findAll({
    queryKey: RSS_ENTRIES_QUERY_ROOT,
  })) {
    const current = query.state.data as RSSEntryCollectionData | undefined;
    if (!current) continue;
    const next = patchRSSEntryCollection(
      current,
      state,
      rssEntriesRequestFromQueryKey(query.queryKey),
    );
    if (next !== current) queryClient.setQueryData(query.queryKey, next);
  }
  queryClient.setQueriesData<RSSEntry>(
    { queryKey: [...RSS_ENTRY_QUERY_ROOT, state.entryId] },
    (current) => current ? applyRSSStateToEntry(current, state) : current,
  );

  const after = before ? applyRSSStateToEntry(before, state) : undefined;
  if (field === "read" && before && after && Boolean(before.readAt) !== Boolean(after.readAt)) {
    const delta = after.readAt ? -1 : 1;
    queryClient.setQueryData<RSSSubscription[]>(
      RSS_SUBSCRIPTIONS_QUERY_KEY,
      (current) => current?.map((subscription) =>
        subscription.id === before.subscriptionId
          ? { ...subscription, unreadCount: Math.max(0, subscription.unreadCount + delta) }
          : subscription,
      ),
    );
  }
}

function patchRSSEntryCollection(
  current: RSSEntryCollectionData,
  state: RSSEntryState,
  request?: RSSListEntriesRequest,
): RSSEntryCollectionData {
  if ("pages" in current) {
    let changed = false;
    const pages = current.pages.map((page) => {
      const next = patchRSSEntryPage(page, state, request);
      if (next !== page) changed = true;
      return next;
    });
    return changed ? { ...current, pages } : current;
  }
  return patchRSSEntryPage(current, state, request);
}

function patchRSSEntryPage(
  current: RSSEntryPage,
  state: RSSEntryState,
  request?: RSSListEntriesRequest,
) {
  const index = current.items.findIndex((entry) => entry.id === state.entryId);
  if (index < 0) return current;
  const patched = applyRSSStateToEntry(current.items[index], state);
  const remove = Boolean(
    (request?.unreadOnly && patched.readAt) ||
    (request?.starredOnly && !patched.starredAt),
  );
  const items = remove
    ? current.items.filter((entry) => entry.id !== state.entryId)
    : current.items.map((entry, itemIndex) => itemIndex === index ? patched : entry);
  return {
    ...current,
    items,
    ...(remove ? { total: Math.max(0, current.total - 1) } : {}),
  };
}

function findCachedRSSEntry(
  queryClient: QueryClient,
  entryID: string,
  field?: RSSEntryStateField,
) {
  const candidates: RSSEntry[] = [];
  for (const [, detail] of queryClient.getQueriesData<RSSEntry>({
    queryKey: [...RSS_ENTRY_QUERY_ROOT, entryID],
  })) {
    if (detail?.id === entryID) candidates.push(detail);
  }
  for (const query of queryClient.getQueryCache().findAll({
    queryKey: RSS_ENTRIES_QUERY_ROOT,
  })) {
    for (const entry of rssEntriesFromCollection(query.state.data)) {
      if (entry.id === entryID) candidates.push(entry);
    }
  }
  return candidates.sort((left, right) => {
    const fieldDifference = field
      ? entryFieldRevision(right, field) - entryFieldRevision(left, field)
      : 0;
    return fieldDifference ||
      right.stateRevision - left.stateRevision ||
      right.revision - left.revision;
  })[0];
}

function rssEntriesFromCollection(value: unknown) {
  if (!value || typeof value !== "object") return [] as RSSEntry[];
  if ("pages" in value) {
    const pages = (value as InfiniteData<RSSEntryPage>).pages;
    return pages.flatMap((page) => page.items ?? []);
  }
  return Array.isArray((value as RSSEntryPage).items)
    ? (value as RSSEntryPage).items
    : [];
}

function rssEntryCollectionContains(value: unknown, entryID: string) {
  return rssEntriesFromCollection(value).some((entry) => entry.id === entryID);
}

function rssDerivedEntryQueryMatches(
  query: Query,
  field: RSSEntryStateField,
  entry?: RSSEntry,
) {
  const request = rssEntriesRequestFromQueryKey(query.queryKey);
  if (!request) return false;
  if (field === "read" && !request.unreadOnly) return false;
  if (field === "starred" && !request.starredOnly) return false;
  if (field !== "read" && field !== "starred") return false;
  if (!entry) return true;
  if (request.subscriptionId && request.subscriptionId !== entry.subscriptionId) return false;
  if (request.kind && request.kind !== entry.kind) return false;
  if (request.starredOnly && field === "read" && !entry.starredAt) return false;
  if (request.unreadOnly && field === "starred" && entry.readAt) return false;
  return true;
}

function rssEntryQueryIncludesSubscriptions(
  queryKey: QueryKey,
  subscriptionIDs: ReadonlySet<string>,
) {
  const request = rssEntriesRequestFromQueryKey(queryKey);
  if (!request) return false;
  const subscriptionID = request.subscriptionId?.trim();
  return !subscriptionID || subscriptionIDs.has(subscriptionID);
}

function rssEntryQueryOverlapsMarkAllRead(
  queryKey: QueryKey,
  request: RSSMarkAllReadRequest,
) {
  const cached = rssEntriesRequestFromQueryKey(queryKey);
  if (!cached) return false;
  const targetSubscription = request.subscriptionId?.trim();
  const cachedSubscription = cached.subscriptionId?.trim();
  if (targetSubscription && cachedSubscription && targetSubscription !== cachedSubscription) {
    return false;
  }
  if (request.kind && cached.kind && request.kind !== cached.kind) return false;
  return true;
}

function rssEntryMatchesMarkAllRead(entry: RSSEntry, request: RSSMarkAllReadRequest) {
  if (request.subscriptionId && request.subscriptionId !== entry.subscriptionId) return false;
  if (request.kind && request.kind !== entry.kind) return false;
  if (request.starredOnly && !entry.starredAt) return false;
  return !entry.readAt;
}

function optimisticRSSEntryState(
  entry: RSSEntry,
  request: RSSOptimisticEntryMutation,
): RSSEntryState {
  const now = new Date().toISOString();
  const fieldRevisions = {
    read: entryFieldRevision(entry, "read"),
    starred: entryFieldRevision(entry, "starred"),
    articleProgress: entryFieldRevision(entry, "articleProgress"),
    videoProgressSeconds: entryFieldRevision(entry, "videoProgressSeconds"),
  };
  fieldRevisions[request.field] = Math.max(
    fieldRevisions[request.field],
    Math.max(0, Math.trunc(request.expectedRevision ?? 0)),
  ) + 1;

  const read = request.field === "read" ? Boolean(request.read) : Boolean(entry.readAt);
  const starred = request.field === "starred"
    ? Boolean(request.starred)
    : Boolean(entry.starredAt);
  const videoProgressSeconds = request.field === "videoProgressSeconds"
    ? request.videoProgressSeconds
    : entry.videoProgressSeconds;
  const videoDurationSeconds = request.field === "videoProgressSeconds"
    ? request.videoDurationSeconds ?? entry.videoDurationSeconds
    : entry.videoDurationSeconds;
  return {
    entryId: entry.id,
    subjectId: "",
    read,
    readAt: read ? entry.readAt || now : undefined,
    starred,
    starredAt: starred ? entry.starredAt || now : undefined,
    articleProgress: request.field === "articleProgress"
      ? request.articleProgress
      : entry.articleProgress,
    videoProgressSeconds,
    videoDurationSeconds,
    videoCompleted: Boolean(
      videoDurationSeconds &&
      videoDurationSeconds > 0 &&
      videoProgressSeconds !== undefined &&
      videoProgressSeconds >= videoDurationSeconds,
    ),
    fieldRevisions,
    revision: Math.max(0, Math.trunc(entry.stateRevision)) + 1,
    updatedAt: now,
    mutationId: request.mutationId,
  };
}

function entryFieldRevision(entry: RSSEntry, field: RSSEntryStateField) {
  return Math.max(0, Math.trunc(entry.fieldRevisions?.[field] ?? 0));
}

function nonNegativeSnapshot(value: number | undefined) {
  return Number.isFinite(value) ? Math.max(0, Math.trunc(value ?? 0)) : 0;
}
