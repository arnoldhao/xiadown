import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueries,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query";
import { Call } from "@wailsio/runtime";
import { useEffect } from "react";

import { normalizeDiscoveryRequest } from "./discovery-utils";
import {
  RSSLatestEntryStateMutationCoordinator,
  type RSSStateMutationTicket,
} from "./entry-state-mutation-coordinator";
import {
  applyRSSOptimisticEntryMutation,
  applyRSSOptimisticSubscriptionUpdate,
  commitRSSAddedSubscription,
  commitRSSUpdatedSubscription,
  createRSSMarkAllReadCacheCallbacks,
  invalidateFailedRSSEntryMutation,
  invalidateRSSEntryCachesForSubscriptions,
  invalidateRSSEntryDetailsForSubscriptions,
  reconcileRSSEntryCollectionSnapshots,
  reconcileRSSEntryStateCaches,
  resetRSSBackfillCaches,
  rollbackRSSOptimisticSubscriptionUpdate,
  rssEntryDetailQueryKey,
  RSS_ENTRIES_QUERY_ROOT,
  RSS_ENTRY_QUERY_ROOT,
  RSS_SUBSCRIPTIONS_QUERY_KEY,
} from "./query-cache";
import {
  RSS_ENTRIES_QUERY_POLICY,
  RSS_ENTRIES_REFETCH_INTERVAL_MS,
  RSS_ENTRY_DETAIL_QUERY_POLICY,
  RSS_SEARCH_QUERY_POLICY,
  RSS_SUBSCRIPTIONS_QUERY_POLICY,
  RSS_SUBSCRIPTIONS_REFETCH_INTERVAL_MS,
  rssEntriesRefetchInterval,
  rssDiscoveryQueryPolicy,
} from "./query-policy";
import { createRSSMutationID, rssFieldRevision } from "./state-utils";
import { isRSSBilibiliVideoStatusForSession } from "./video-transport";
import type {
  RSSAddSubscriptionRequest,
  RSSBackfillHistoryRequest,
  RSSBackfillHistoryResult,
  RSSChangePage,
  RSSDiscoveryResult,
  RSSDiscoveryRoute,
  RSSEntry,
  RSSEntryPage,
  RSSEntryState,
  RSSListChangesRequest,
  RSSListDiscoveryRequest,
  RSSListEntriesRequest,
  RSSMarkAllReadRequest,
  RSSMarkAllReadResult,
  RSSPreviewResult,
  RSSPreviewSubscriptionRequest,
  RSSRefreshRequest,
  RSSRefreshResult,
  RSSSetEntryReadRequest,
  RSSSetEntryStateRequest,
  RSSSaveEntryImageRequest,
  RSSSaveEntryImageResult,
  RSSSubscription,
  RSSSubscriptionRequest,
  RSSUpdateSubscriptionRequest,
  RSSCategory,
  RSSCollection,
  RSSCollectionItems,
  RSSCreateCategoryRequest,
  RSSCreateCollectionRequest,
  RSSReorderRequest,
  RSSReorderSubscriptionsRequest,
  RSSUpdateCategoryRequest,
  RSSUpdateCollectionItemsRequest,
  RSSUpdateCollectionRequest,
} from "@/app/rss/types";

export const RSS_HANDLER_SERVICE =
  "xiadown/internal/presentation/wails.RSSHandler";
export const RSS_VIDEO_PLAYER_HANDLER_SERVICE =
  "xiadown/internal/presentation/wails.RSSVideoPlayerHandler";
export const RSS_SITE_PLAYER_HANDLER_SERVICE =
  "xiadown/internal/presentation/wails.RSSSitePlayerHandler";

export interface RSSBilibiliPlaybackDescriptor {
  platform: "bilibili";
  adapter: "video" | "bangumi";
  platformVideoId: string;
  /** Canonical full-site playback page used by the native App Session webview. */
  playerUrl: string;
  authenticated: boolean;
  sessionId: string;
}

export interface RSSBilibiliPrepareRequest {
  requestId: number;
  platformVideoId: string;
  startSeconds?: number;
  volume?: number;
  muted?: boolean;
}

export interface RSSBilibiliPlayerOption {
  id: string;
  label: string;
}

export interface RSSBilibiliPlayerControls {
  playPause: boolean;
  seek: boolean;
  volume: boolean;
  playbackRate: boolean;
  fullscreen: boolean;
  captions: boolean;
  quality: boolean;
  danmaku: boolean;
}

export interface RSSBilibiliPlayerStatus {
  provider: "bilibili";
  sessionId: string;
  available: boolean;
  platformVideoId: string;
  state: string;
  title?: string;
  publisher?: string;
  publishedAt?: string;
  viewCount?: number;
  likeCount?: number;
  currentTime: number;
  duration: number;
  bufferedTime: number;
  volume: number;
  muted: boolean;
  playbackRate: number;
  fullscreen: boolean;
  danmakuEnabled: boolean;
  controls: RSSBilibiliPlayerControls;
  captionOptions: RSSBilibiliPlayerOption[];
  qualityOptions: RSSBilibiliPlayerOption[];
  playbackRateOptions: RSSBilibiliPlayerOption[];
  selections: {
    playbackRateId?: string;
    captionId?: string;
    qualityId?: string;
  };
  errorMessage?: string;
}

export interface RSSNativeVideoRect {
  x: number;
  y: number;
  width: number;
  height: number;
  centerX: number;
  centerY: number;
  viewportWidth: number;
  viewportHeight: number;
  radius: number;
  interactive: boolean;
  sequence: number;
}

export interface RSSSitePlaybackDescriptor {
  sessionId: string;
  url: string;
  siteKey?: string;
  credentialsLoaded: boolean;
}

export interface RSSSitePrepareRequest {
  requestId: number;
  url: string;
}

export function prepareRSSBilibiliVideo(request: RSSBilibiliPrepareRequest) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.Prepare`,
    request,
  ) as Promise<RSSBilibiliPlaybackDescriptor>;
}

function bilibiliPrepareTransactionRequest(requestId: number) {
  return { requestId };
}

export function acceptRSSBilibiliVideoPrepare(requestId: number) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.AcceptPrepare`,
    bilibiliPrepareTransactionRequest(requestId),
  ) as Promise<void>;
}

export function cancelRSSBilibiliVideoPrepare(requestId: number) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.CancelPrepare`,
    bilibiliPrepareTransactionRequest(requestId),
  ) as Promise<void>;
}

function bilibiliSessionRequest(sessionId: string) {
  return { sessionId: sessionId.trim() };
}

export function playRSSBilibiliVideo(sessionId: string) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.Play`,
    bilibiliSessionRequest(sessionId),
  ) as Promise<void>;
}

export function pauseRSSBilibiliVideo(sessionId: string) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.Pause`,
    bilibiliSessionRequest(sessionId),
  ) as Promise<void>;
}

export function seekRSSBilibiliVideo(sessionId: string, seconds: number) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.Seek`,
    { sessionId: sessionId.trim(), seconds },
  ) as Promise<void>;
}

export function setRSSBilibiliVideoVolume(
  sessionId: string,
  volume: number,
  muted: boolean,
) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.SetVolume`,
    { sessionId: sessionId.trim(), volume, muted },
  ) as Promise<void>;
}

export function setRSSBilibiliVideoPlaybackRate(
  sessionId: string,
  rate: number,
) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.SetPlaybackRate`,
    { sessionId: sessionId.trim(), rate },
  ) as Promise<void>;
}

function bilibiliSelectionRequest(sessionId: string, value: string) {
  return { sessionId: sessionId.trim(), value: value.trim() };
}

export function toggleRSSBilibiliVideoCaptions(sessionId: string) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.ToggleCaptions`,
    bilibiliSessionRequest(sessionId),
  ) as Promise<void>;
}

export function selectRSSBilibiliVideoCaption(sessionId: string, value: string) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.SelectCaption`,
    bilibiliSelectionRequest(sessionId, value),
  ) as Promise<void>;
}

export function selectRSSBilibiliVideoQuality(sessionId: string, value: string) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.SelectQuality`,
    bilibiliSelectionRequest(sessionId, value),
  ) as Promise<void>;
}

export function toggleRSSBilibiliVideoDanmaku(sessionId: string) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.ToggleDanmaku`,
    bilibiliSessionRequest(sessionId),
  ) as Promise<void>;
}

export function requestRSSBilibiliVideoFullscreen(sessionId: string) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.RequestFullscreen`,
    bilibiliSessionRequest(sessionId),
  ) as Promise<void>;
}

export function exitRSSBilibiliVideoFullscreen(sessionId: string) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.ExitFullscreen`,
    bilibiliSessionRequest(sessionId),
  ) as Promise<void>;
}

export function getRSSBilibiliVideoStatus() {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.Status`,
  ) as Promise<RSSBilibiliPlayerStatus>;
}

export function subscribeRSSBilibiliVideoStatus(
  sessionId: string,
  handler: (status: RSSBilibiliPlayerStatus) => void,
) {
  let active = true;
  let unsubscribe = () => {};
  void import("@wailsio/runtime")
    .then(({ Events }) => {
      if (!active) return;
      unsubscribe = Events.On("rss:bilibili-video-player", (event: unknown) => {
        const payload = ((event as { data?: unknown })?.data ?? event) as
          | RSSBilibiliPlayerStatus
          | undefined;
        if (isRSSBilibiliVideoStatusForSession(payload, sessionId)) {
          handler(payload);
        }
      });
    })
    .catch(() => {});
  return () => {
    active = false;
    unsubscribe();
  };
}

export { isRSSBilibiliVideoStatusForSession } from "./video-transport";

export function showRSSBilibiliVideo(rect: RSSNativeVideoRect) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.Show`,
    rect,
  ) as Promise<boolean>;
}

export function hideRSSBilibiliVideo(sequence: number) {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.Hide`,
    { sequence },
  ) as Promise<boolean>;
}

export function closeRSSBilibiliVideo(sessionId = "") {
  return Call.ByName(
    `${RSS_VIDEO_PLAYER_HANDLER_SERVICE}.Close`,
    bilibiliSessionRequest(sessionId),
  ) as Promise<void>;
}

export function prepareRSSSiteVideo(request: RSSSitePrepareRequest) {
  return Call.ByName(
    `${RSS_SITE_PLAYER_HANDLER_SERVICE}.Prepare`,
    request,
  ) as Promise<RSSSitePlaybackDescriptor>;
}

export function acceptRSSSiteVideoPrepare(requestId: number) {
  return Call.ByName(
    `${RSS_SITE_PLAYER_HANDLER_SERVICE}.AcceptPrepare`,
    { requestId },
  ) as Promise<void>;
}

export function cancelRSSSiteVideoPrepare(requestId: number) {
  return Call.ByName(
    `${RSS_SITE_PLAYER_HANDLER_SERVICE}.CancelPrepare`,
    { requestId },
  ) as Promise<void>;
}

export function showRSSSiteVideo(
  sessionId: string,
  rect: RSSNativeVideoRect,
) {
  return Call.ByName(
    `${RSS_SITE_PLAYER_HANDLER_SERVICE}.Show`,
    { sessionId: sessionId.trim(), rect: { ...rect, interactive: true } },
  ) as Promise<boolean>;
}

export function hideRSSSiteVideo(sessionId: string, sequence: number) {
  return Call.ByName(
    `${RSS_SITE_PLAYER_HANDLER_SERVICE}.Hide`,
    { sessionId: sessionId.trim(), sequence },
  ) as Promise<boolean>;
}

export function closeRSSSiteVideo(sessionId: string) {
  return Call.ByName(
    `${RSS_SITE_PLAYER_HANDLER_SERVICE}.Close`,
    { sessionId: sessionId.trim() },
  ) as Promise<void>;
}

export { RSS_SUBSCRIPTIONS_QUERY_KEY } from "./query-cache";

export const rssQueryKeys = {
  all: ["rss"] as const,
  subscriptions: RSS_SUBSCRIPTIONS_QUERY_KEY,
  categories: ["rss", "categories"] as const,
  collections: ["rss", "collections"] as const,
  collectionItemsRoot: () => ["rss", "collection-items"] as const,
  collectionItems: (id: string) =>
    ["rss", "collection-items", id.trim()] as const,
  entriesRoot: () => RSS_ENTRIES_QUERY_ROOT,
  entries: (request: RSSListEntriesRequest) =>
    ["rss", "entries", normalizeEntriesRequest(request)] as const,
  entriesInfinite: (request: RSSListEntriesRequest) =>
    ["rss", "entries", "infinite", normalizeEntriesRequest(request)] as const,
  entryRoot: () => RSS_ENTRY_QUERY_ROOT,
  entry: rssEntryDetailQueryKey,
  changesRoot: () => ["rss", "changes"] as const,
  changes: (request: RSSListChangesRequest) =>
    ["rss", "changes", normalizeChangesRequest(request)] as const,
  discoveryRoot: () => ["rss", "discovery"] as const,
  discovery: (request: RSSListDiscoveryRequest) =>
    ["rss", "discovery", normalizeDiscoveryRequest(request)] as const,
  discoveryInfinite: (request: RSSListDiscoveryRequest) =>
    ["rss", "discovery", "infinite", normalizeDiscoveryRequest(request)] as const,
};

export async function listRSSSubscriptions(): Promise<RSSSubscription[]> {
  const result = await callRSS<unknown>("ListSubscriptions");
  return Array.isArray(result) ? (result as RSSSubscription[]) : [];
}

export async function listRSSCategories(): Promise<RSSCategory[]> {
  const result = await callRSS<unknown>("ListCategories");
  return Array.isArray(result) ? (result as RSSCategory[]) : [];
}

export function createRSSCategory(
  request: RSSCreateCategoryRequest,
): Promise<RSSCategory> {
  return callRSS("CreateCategory", request);
}

export function updateRSSCategory(
  request: RSSUpdateCategoryRequest,
): Promise<RSSCategory> {
  return callRSS("UpdateCategory", request);
}

export function deleteRSSCategory(request: RSSSubscriptionRequest): Promise<void> {
  return callRSS("DeleteCategory", request);
}

export function reorderRSSCategories(
  request: RSSReorderRequest,
): Promise<RSSCategory[]> {
  return callRSS("ReorderCategories", request);
}

export function reorderRSSSubscriptions(
  request: RSSReorderSubscriptionsRequest,
): Promise<RSSSubscription[]> {
  return callRSS("ReorderSubscriptions", request);
}

export async function listRSSCollections(): Promise<RSSCollection[]> {
  const result = await callRSS<unknown>("ListCollections");
  return Array.isArray(result) ? (result as RSSCollection[]) : [];
}

export function createRSSCollection(
  request: RSSCreateCollectionRequest,
): Promise<RSSCollection> {
  return callRSS("CreateCollection", request);
}

export function updateRSSCollection(
  request: RSSUpdateCollectionRequest,
): Promise<RSSCollection> {
  return callRSS("UpdateCollection", request);
}

export function deleteRSSCollection(request: RSSSubscriptionRequest): Promise<void> {
  return callRSS("DeleteCollection", request);
}

export async function listRSSCollectionItems(id: string): Promise<RSSCollectionItems> {
  const result = await callRSS<RSSCollectionItems>("ListCollectionItems", {
    id: id.trim(),
  });
  return {
    ...result,
    itemIds: Array.isArray(result.itemIds) ? result.itemIds : [],
  };
}

export function replaceRSSCollectionItems(
  request: RSSUpdateCollectionItemsRequest,
): Promise<RSSCollection> {
  return callRSS("ReplaceCollectionItems", request);
}

export function addRSSCollectionItems(
  request: RSSUpdateCollectionItemsRequest,
): Promise<RSSCollection> {
  return callRSS("AddCollectionItems", request);
}

export function removeRSSCollectionItems(
  request: RSSUpdateCollectionItemsRequest,
): Promise<RSSCollection> {
  return callRSS("RemoveCollectionItems", request);
}

export async function previewRSSSubscription(
  request: RSSPreviewSubscriptionRequest,
): Promise<RSSPreviewResult> {
  const result = await callRSS<RSSPreviewResult>(
    "PreviewSubscription",
    request,
  );
  return {
    ...result,
    entries: Array.isArray(result.entries) ? result.entries : [],
  };
}

export function addRSSSubscription(
  request: RSSAddSubscriptionRequest,
): Promise<RSSSubscription> {
  return callRSS("AddSubscription", request);
}

export async function listRSSDiscovery(
  request: RSSListDiscoveryRequest,
): Promise<RSSDiscoveryResult> {
  const result = await callRSS<RSSDiscoveryResult>(
    "ListDiscovery",
    normalizeDiscoveryRequest(request),
  );
  return {
    ...result,
    categories: Array.isArray(result.categories) ? result.categories : [],
    routes: Array.isArray(result.routes)
      ? result.routes.map(normalizeDiscoveryRoute)
      : [],
  };
}

export function updateRSSSubscription(
  request: RSSUpdateSubscriptionRequest,
): Promise<RSSSubscription> {
  return callRSS("UpdateSubscription", request);
}

export function deleteRSSSubscription(
  request: RSSSubscriptionRequest,
): Promise<void> {
  return callRSS("DeleteSubscription", request);
}

export function refreshRSS(
  request: RSSRefreshRequest,
): Promise<RSSRefreshResult> {
  return callRSS("Refresh", request);
}

export async function backfillRSSHistory(
  request: RSSBackfillHistoryRequest,
): Promise<RSSBackfillHistoryResult> {
  const subscriptionId = request.subscriptionId?.trim();
  const result = await callRSS<RSSBackfillHistoryResult>("BackfillHistory", {
    ...(subscriptionId ? { subscriptionId } : {}),
    ...(request.kind ? { kind: request.kind } : {}),
  });
  return {
    ...result,
    sources: Array.isArray(result.sources) ? result.sources : [],
  };
}

export async function listRSSEntries(
  request: RSSListEntriesRequest,
): Promise<RSSEntryPage> {
  const result = await callRSS<RSSEntryPage>("ListEntries", request);
  return {
    ...result,
    items: Array.isArray(result.items) ? result.items : [],
    snapshot: normalizeNonNegativeInteger(result.snapshot) ?? 0,
  };
}

export function getRSSEntry(id: string): Promise<RSSEntry> {
  return callRSS("GetEntry", { id: id.trim() });
}

export function saveRSSEntryImage(
  request: RSSSaveEntryImageRequest,
): Promise<RSSSaveEntryImageResult> {
  return callRSS("SaveEntryImage", request);
}

export function setRSSEntryRead(
  request: RSSSetEntryReadRequest,
): Promise<RSSEntryState> {
  return callRSS("SetEntryRead", request);
}

export function markAllRSSRead(
  request: RSSMarkAllReadRequest,
): Promise<RSSMarkAllReadResult> {
  return callRSS("MarkAllRead", request);
}

export function setRSSEntryState(
  request: RSSSetEntryStateRequest,
): Promise<RSSEntryState> {
  const ticket = rssEntryStateMutationCoordinator.reserve(request);
  return rssEntryStateMutationCoordinator
    .execute(ticket, () => setRSSEntryStateWithConflictRecovery(request))
    .finally(() => rssEntryStateMutationCoordinator.retire(ticket));
}

export function listRSSChanges(
  request: RSSListChangesRequest,
): Promise<RSSChangePage> {
  return callRSS("ListChanges", request);
}

export function useRSSSubscriptions(enabled = true) {
  return useQuery({
    queryKey: rssQueryKeys.subscriptions,
    queryFn: listRSSSubscriptions,
    enabled,
    refetchInterval: enabled ? RSS_SUBSCRIPTIONS_REFETCH_INTERVAL_MS : false,
    ...RSS_SUBSCRIPTIONS_QUERY_POLICY,
  });
}

export function useRSSCategories(enabled = true) {
  return useQuery({
    queryKey: rssQueryKeys.categories,
    queryFn: listRSSCategories,
    enabled,
  });
}

export function useRSSCollections(enabled = true) {
  return useQuery({
    queryKey: rssQueryKeys.collections,
    queryFn: listRSSCollections,
    enabled,
  });
}

export function useRSSCollectionItems(id: string, enabled = true) {
  const normalizedID = id.trim();
  return useQuery({
    queryKey: rssQueryKeys.collectionItems(normalizedID),
    queryFn: () => listRSSCollectionItems(normalizedID),
    enabled: enabled && Boolean(normalizedID),
  });
}

export function useRSSDiscovery(
  request: RSSListDiscoveryRequest,
  enabled = true,
) {
  const normalizedRequest = normalizeDiscoveryRequest(request);
  return useQuery({
    queryKey: rssQueryKeys.discovery(normalizedRequest),
    queryFn: () => listRSSDiscovery(normalizedRequest),
    enabled,
    ...rssDiscoveryQueryPolicy(normalizedRequest),
  });
}

export function useRSSDiscoveryInfinite(
  request: RSSListDiscoveryRequest,
  enabled = true,
) {
  const normalizedRequest = normalizeDiscoveryRequest(request);
  const initialOffset = normalizedRequest.offset ?? 0;
  return useInfiniteQuery({
    queryKey: rssQueryKeys.discoveryInfinite({
      ...normalizedRequest,
      offset: initialOffset,
    }),
    queryFn: ({ pageParam }) =>
      listRSSDiscovery({ ...normalizedRequest, offset: pageParam }),
    initialPageParam: initialOffset,
    getNextPageParam: (lastPage) =>
      lastPage.hasMore
        ? lastPage.offset + Math.max(lastPage.routes.length, lastPage.limit)
        : undefined,
    enabled,
    ...rssDiscoveryQueryPolicy(normalizedRequest),
  });
}

export function useRSSRefreshDiscovery() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: RSSListDiscoveryRequest) =>
      listRSSDiscovery({ ...request, forceRefresh: true }),
    onSuccess: async (result, request) => {
      const normalizedRequest = normalizeDiscoveryRequest({
        ...request,
        forceRefresh: false,
      });
      const initialOffset = normalizedRequest.offset ?? 0;
      await queryClient.invalidateQueries({
        queryKey: rssQueryKeys.discoveryRoot(),
        refetchType: "none",
      });
      queryClient.setQueryData(
        rssQueryKeys.discovery(normalizedRequest),
        result,
      );
      queryClient.setQueryData<InfiniteData<RSSDiscoveryResult>>(
        rssQueryKeys.discoveryInfinite({
          ...normalizedRequest,
          offset: initialOffset,
        }),
        { pages: [result], pageParams: [initialOffset] },
      );
    },
  });
}

export function useRSSEntries(
  request: RSSListEntriesRequest,
  enabled = true,
) {
  const normalizedRequest = normalizeEntriesRequest(request);
  const queryPolicy = normalizedRequest.query
    ? RSS_SEARCH_QUERY_POLICY
    : RSS_ENTRIES_QUERY_POLICY;
  return useQuery({
    queryKey: rssQueryKeys.entries(normalizedRequest),
    queryFn: () => listRSSEntries(normalizedRequest),
    enabled,
    placeholderData: (previous) => previous,
    refetchInterval: rssEntriesRefetchInterval(normalizedRequest, enabled),
    ...queryPolicy,
  });
}

export function useRSSEntriesInfinite(
  request: RSSListEntriesRequest,
  enabled = true,
) {
  const normalizedRequest = normalizeEntriesRequest(request);
  const initialOffset = normalizedRequest.offset ?? 0;
  const queryPolicy = normalizedRequest.query
    ? RSS_SEARCH_QUERY_POLICY
    : RSS_ENTRIES_QUERY_POLICY;
  return useInfiniteQuery({
    queryKey: rssQueryKeys.entriesInfinite({
      ...normalizedRequest,
      offset: initialOffset,
    }),
    queryFn: ({ pageParam }) =>
      listRSSEntries({ ...normalizedRequest, offset: pageParam }),
    initialPageParam: initialOffset,
    getNextPageParam: (lastPage, _pages, lastPageParam) => {
      const nextOffset = normalizeNonNegativeInteger(lastPage.nextOffset);
      return nextOffset !== undefined && nextOffset > lastPageParam
        ? nextOffset
        : undefined;
    },
    enabled,
    refetchInterval: rssEntriesRefetchInterval(normalizedRequest, enabled),
    ...queryPolicy,
  });
}

export type RSSCollectionUnreadCounts = Record<
  "all" | "articles" | "social" | "images" | "videos" | "starred",
  number
>;

export function useRSSCollectionUnreadCounts(enabled = true) {
  const queryClient = useQueryClient();
  const result = useQueries({
    queries: [
      { key: "all", request: { unreadOnly: true, limit: 1 } },
      { key: "articles", request: { kind: "article", unreadOnly: true, limit: 1 } },
      { key: "social", request: { kind: "social", unreadOnly: true, limit: 1 } },
      { key: "images", request: { kind: "image", unreadOnly: true, limit: 1 } },
      { key: "videos", request: { kind: "video", unreadOnly: true, limit: 1 } },
      { key: "starred", request: { starredOnly: true, unreadOnly: true, limit: 1 } },
    ].map(({ key, request }) => {
      const normalizedRequest = normalizeEntriesRequest(request as RSSListEntriesRequest);
      return {
        queryKey: rssQueryKeys.entries(normalizedRequest),
        queryFn: () => listRSSEntries(normalizedRequest),
        enabled,
        refetchInterval: enabled ? RSS_ENTRIES_REFETCH_INTERVAL_MS : false,
        ...RSS_ENTRIES_QUERY_POLICY,
        meta: { collectionKey: key },
      };
    }),
    combine: (results) => ({
      data: {
        all: results[0]?.data?.total ?? 0,
        articles: results[1]?.data?.total ?? 0,
        social: results[2]?.data?.total ?? 0,
        images: results[3]?.data?.total ?? 0,
        videos: results[4]?.data?.total ?? 0,
        starred: results[5]?.data?.total ?? 0,
      } satisfies RSSCollectionUnreadCounts,
      snapshot: Math.max(
        0,
        ...results.map((result) => result.data?.snapshot ?? 0),
      ),
      isLoading: results.some((result) => result.isLoading),
      isError: results.some((result) => result.isError),
    }),
  });
  useEffect(() => {
    if (!enabled || result.isLoading || result.snapshot <= 0) return;
    void reconcileRSSEntryCollectionSnapshots(queryClient, result.snapshot);
  }, [enabled, queryClient, result.isLoading, result.snapshot]);
  return result;
}

/** Hydrates the full body only after an entry is selected for reading. */
export function useRSSEntry(
  id: string,
  contentRevision = 0,
  enabled = true,
) {
  const normalizedID = id.trim();
  return useQuery({
    queryKey: rssQueryKeys.entry(normalizedID, contentRevision),
    queryFn: () => getRSSEntry(normalizedID),
    enabled: enabled && Boolean(normalizedID),
    ...RSS_ENTRY_DETAIL_QUERY_POLICY,
  });
}

export function useRSSAddSubscription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: addRSSSubscription,
    onSuccess: (subscription) =>
      commitRSSAddedSubscription(queryClient, subscription),
  });
}

export function useRSSUpdateSubscription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateRSSSubscription,
    onMutate: (request) =>
      applyRSSOptimisticSubscriptionUpdate(queryClient, request),
    onSuccess: async (subscription, request) => {
      await commitRSSUpdatedSubscription(queryClient, subscription, request);
      if (request.categoryId !== undefined || request.sortOrder !== undefined) {
        await invalidateRSSOrganizationCaches(queryClient, {
          categories: true,
          entries: true,
        });
      }
    },
    onError: (_error, request, context) =>
      rollbackRSSOptimisticSubscriptionUpdate(queryClient, request, context),
  });
}

export function useRSSCreateCategory() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createRSSCategory,
    onSuccess: () => invalidateRSSOrganizationCaches(queryClient, {
      categories: true,
    }),
  });
}

export function useRSSUpdateCategory() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateRSSCategory,
    onSuccess: () => invalidateRSSOrganizationCaches(queryClient, {
      categories: true,
      subscriptions: true,
    }),
  });
}

export function useRSSDeleteCategory() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteRSSCategory,
    onSuccess: () => invalidateRSSOrganizationCaches(queryClient, {
      categories: true,
      subscriptions: true,
      entries: true,
    }),
  });
}

export function useRSSReorderCategories() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: reorderRSSCategories,
    onSuccess: (categories) => {
      queryClient.setQueryData(rssQueryKeys.categories, categories);
      return invalidateRSSOrganizationCaches(queryClient, {
        subscriptions: true,
      });
    },
  });
}

export function useRSSReorderSubscriptions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: reorderRSSSubscriptions,
    onSuccess: () => invalidateRSSOrganizationCaches(queryClient, {
      categories: true,
      subscriptions: true,
      entries: true,
    }),
  });
}

export function useRSSCreateCollection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createRSSCollection,
    onSuccess: () => invalidateRSSOrganizationCaches(queryClient, {
      collections: true,
    }),
  });
}

export function useRSSUpdateCollection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateRSSCollection,
    onSuccess: () => invalidateRSSOrganizationCaches(queryClient, {
      collections: true,
      entries: true,
    }),
  });
}

export function useRSSDeleteCollection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteRSSCollection,
    onSuccess: (_result, request) => invalidateRSSOrganizationCaches(queryClient, {
      collections: true,
      collectionItems: request.id,
      entries: true,
    }),
  });
}

export function useRSSReplaceCollectionItems() {
  return useRSSCollectionItemsMutation(replaceRSSCollectionItems);
}

export function useRSSAddCollectionItems() {
  return useRSSCollectionItemsMutation(addRSSCollectionItems);
}

export function useRSSRemoveCollectionItems() {
  return useRSSCollectionItemsMutation(removeRSSCollectionItems);
}

function useRSSCollectionItemsMutation(
  mutationFn: (request: RSSUpdateCollectionItemsRequest) => Promise<RSSCollection>,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: (_collection, request) => invalidateRSSOrganizationCaches(queryClient, {
      collections: true,
      collectionItems: request.id,
      entries: true,
    }),
  });
}

export function useRSSDeleteSubscription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteRSSSubscription,
    onSuccess: async (_, request) => {
      queryClient.setQueryData<RSSSubscription[]>(
        rssQueryKeys.subscriptions,
        (current) =>
          (current ?? []).filter(
            (subscription) => subscription.id !== request.id,
          ),
      );
      await invalidateRSSEntryCachesForSubscriptions(
        queryClient,
        new Set([request.id]),
      );
      await invalidateRSSOrganizationCaches(queryClient, {
        categories: true,
        collections: true,
        collectionItemsRoot: true,
      });
    },
  });
}

export function useRSSRefresh() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: refreshRSS,
    onSuccess: async (_, request) => {
      const id = request.id?.trim();
      const subscriptionIDs = id ? new Set([id]) : undefined;
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: rssQueryKeys.subscriptions,
        }),
        invalidateRSSEntryCachesForSubscriptions(queryClient, subscriptionIDs),
        invalidateRSSEntryDetailsForSubscriptions(queryClient, subscriptionIDs),
        invalidateRSSOrganizationCaches(queryClient, {
          categories: true,
          collections: true,
        }),
      ]);
    },
  });
}

export function useRSSBackfillHistory() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: backfillRSSHistory,
    onSuccess: async (result, request) => {
      await resetRSSBackfillCaches(queryClient, result, request);
      await invalidateRSSOrganizationCaches(queryClient, {
        categories: true,
        collections: true,
      });
    },
  });
}

export function useRSSSetEntryRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: setRSSEntryRead,
    onMutate: (request) => applyRSSOptimisticEntryMutation(queryClient, {
      ...request,
      field: "read",
    }),
    onSuccess: (state) =>
      reconcileRSSEntryStateCaches(queryClient, state, "read"),
    onError: (_error, request) =>
      invalidateFailedRSSEntryMutation(queryClient, request.id, "read"),
  });
}

export function useRSSMarkAllRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: markAllRSSRead,
    ...createRSSMarkAllReadCacheCallbacks(queryClient),
  });
}

export function useRSSSetEntryState() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: executeReservedRSSSetEntryState,
    onMutate: async (request) => {
      const ticket = reserveRSSSetEntryStateMutation(request);
      try {
        const optimisticState = await applyRSSOptimisticEntryMutation(
          queryClient,
          request,
        );
        return { optimisticState, ticket };
      } catch (error) {
        discardReservedRSSSetEntryStateMutation(request, ticket);
        rssEntryStateMutationCoordinator.cancel(ticket);
        rssEntryStateMutationCoordinator.retire(ticket);
        throw error;
      }
    },
    onSuccess: (state, request, context) => {
      if (!context || rssEntryStateMutationCoordinator.isLatest(context.ticket)) {
        return reconcileRSSEntryStateCaches(queryClient, state, request.field);
      }
    },
    onError: (_error, request, context) => {
      if (!context || rssEntryStateMutationCoordinator.isLatest(context.ticket)) {
        return invalidateFailedRSSEntryMutation(
          queryClient,
          request.id,
          request.field,
        );
      }
    },
    onSettled: (_state, _error, _request, context) => {
      if (context) rssEntryStateMutationCoordinator.retire(context.ticket);
    },
  });
}

const rssEntryStateMutationCoordinator =
  new RSSLatestEntryStateMutationCoordinator();
const reservedRSSSetEntryStateMutations = new Map<
  string,
  RSSStateMutationTicket[]
>();

function reserveRSSSetEntryStateMutation(request: RSSSetEntryStateRequest) {
  const ticket = rssEntryStateMutationCoordinator.reserve(request);
  const key = rssSetEntryStateReservationKey(request);
  const pending = reservedRSSSetEntryStateMutations.get(key) ?? [];
  pending.push(ticket);
  reservedRSSSetEntryStateMutations.set(key, pending);
  return ticket;
}

function takeReservedRSSSetEntryStateMutation(request: RSSSetEntryStateRequest) {
  const key = rssSetEntryStateReservationKey(request);
  const pending = reservedRSSSetEntryStateMutations.get(key);
  const ticket = pending?.shift();
  if (!pending?.length) reservedRSSSetEntryStateMutations.delete(key);
  return ticket;
}

function discardReservedRSSSetEntryStateMutation(
  request: RSSSetEntryStateRequest,
  ticket: RSSStateMutationTicket,
) {
  const key = rssSetEntryStateReservationKey(request);
  const pending = reservedRSSSetEntryStateMutations.get(key);
  if (!pending) return;
  const remaining = pending.filter((candidate) => candidate !== ticket);
  if (remaining.length) reservedRSSSetEntryStateMutations.set(key, remaining);
  else reservedRSSSetEntryStateMutations.delete(key);
}

function executeReservedRSSSetEntryState(request: RSSSetEntryStateRequest) {
  const ticket = takeReservedRSSSetEntryStateMutation(request) ??
    rssEntryStateMutationCoordinator.reserve(request);
  return rssEntryStateMutationCoordinator.execute(
    ticket,
    () => setRSSEntryStateWithConflictRecovery(request),
  );
}

function rssSetEntryStateReservationKey(request: RSSSetEntryStateRequest) {
  return [request.id.trim(), request.field, request.mutationId.trim()].join("\u0000");
}

async function callRSS<T>(method: string, payload?: unknown): Promise<T> {
  const result =
    payload === undefined
      ? await Call.ByName(`${RSS_HANDLER_SERVICE}.${method}`)
      : await Call.ByName(`${RSS_HANDLER_SERVICE}.${method}`, payload);
  return result as T;
}

async function setRSSEntryStateWithConflictRecovery(
  request: RSSSetEntryStateRequest,
) {
  try {
    return await callRSS<RSSEntryState>("SetEntryState", request);
  } catch (firstError) {
    // A lost bridge response may hide a committed write. Reusing the exact
    // mutation ID lets the repository return the idempotent result safely.
    try {
      return await callRSS<RSSEntryState>("SetEntryState", request);
    } catch (retryError) {
      if (!isRSSRevisionConflict(retryError)) throw firstError;
      const latest = await getRSSEntry(request.id);
      return callRSS<RSSEntryState>("SetEntryState", {
        ...request,
        expectedRevision: rssFieldRevision(latest, request.field),
        mutationId: createRSSMutationID(),
      });
    }
  }
}

function isRSSRevisionConflict(error: unknown) {
  const message = error instanceof Error ? error.message : String(error ?? "");
  return /revision conflict|rss_state_conflict/i.test(message);
}

function normalizeEntriesRequest(
  request: RSSListEntriesRequest,
): RSSListEntriesRequest {
  const subscriptionId = request.subscriptionId?.trim() || undefined;
  const collectionId = request.collectionId?.trim() || undefined;
  const categoryId = request.categoryId?.trim() || undefined;
  const query = request.query?.trim() || undefined;
  const limit = normalizeNonNegativeInteger(request.limit);
  const offset = normalizeNonNegativeInteger(request.offset);
  return {
    ...(subscriptionId ? { subscriptionId } : {}),
    ...(collectionId ? { collectionId } : {}),
    ...(categoryId ? { categoryId } : {}),
    ...(request.kind ? { kind: request.kind } : {}),
    ...(query ? { query } : {}),
    ...(request.unreadOnly ? { unreadOnly: true } : {}),
    ...(request.starredOnly ? { starredOnly: true } : {}),
    ...(limit !== undefined ? { limit } : {}),
    ...(offset !== undefined ? { offset } : {}),
  };
}

interface RSSOrganizationInvalidation {
  categories?: boolean;
  subscriptions?: boolean;
  collections?: boolean;
  collectionItems?: string;
  collectionItemsRoot?: boolean;
  entries?: boolean;
}

function invalidateRSSOrganizationCaches(
  queryClient: ReturnType<typeof useQueryClient>,
  targets: RSSOrganizationInvalidation,
) {
  const invalidations: Promise<unknown>[] = [];
  if (targets.categories) {
    invalidations.push(queryClient.invalidateQueries({
      queryKey: rssQueryKeys.categories,
    }));
  }
  if (targets.subscriptions) {
    invalidations.push(queryClient.invalidateQueries({
      queryKey: rssQueryKeys.subscriptions,
    }));
  }
  if (targets.collections) {
    invalidations.push(queryClient.invalidateQueries({
      queryKey: rssQueryKeys.collections,
    }));
  }
  if (targets.collectionItems) {
    invalidations.push(queryClient.invalidateQueries({
      queryKey: rssQueryKeys.collectionItems(targets.collectionItems),
    }));
  } else if (targets.collectionItemsRoot) {
    invalidations.push(queryClient.invalidateQueries({
      queryKey: rssQueryKeys.collectionItemsRoot(),
    }));
  }
  if (targets.entries) {
    invalidations.push(queryClient.invalidateQueries({
      queryKey: rssQueryKeys.entriesRoot(),
    }));
  }
  return Promise.all(invalidations);
}

function normalizeChangesRequest(
  request: RSSListChangesRequest,
): RSSListChangesRequest {
  const after = normalizeNonNegativeInteger(request.after);
  const limit = normalizeNonNegativeInteger(request.limit);
  return {
    ...(after !== undefined ? { after } : {}),
    ...(limit !== undefined ? { limit } : {}),
  };
}

function normalizeDiscoveryRoute(route: RSSDiscoveryRoute): RSSDiscoveryRoute {
  const parameters = Array.isArray(route.parameters)
    ? route.parameters.map((parameter) => ({
        ...parameter,
        defaultValue: typeof parameter.defaultValue === "string" ? parameter.defaultValue : null,
        options: Array.isArray(parameter.options) ? parameter.options : [],
      }))
    : [];
  const templateNeedsParameters = /(^|\/):[^/]+/.test(route.routePath || "") || (route.routePath || "").includes("*");
  return {
    ...route,
    parameters,
    needsParameters: route.needsParameters === true || parameters.length > 0 || templateNeedsParameters,
  };
}

function normalizeNonNegativeInteger(value: number | undefined) {
  if (value === undefined || !Number.isFinite(value)) {
    return undefined;
  }
  return Math.max(0, Math.trunc(value));
}
