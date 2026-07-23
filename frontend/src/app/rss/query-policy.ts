/**
 * RSS list calls read the local Wails SQLite store. Polling these lightweight
 * projections keeps the UI in step with the backend's independent feed
 * refresher without polling the remote discovery catalog.
 */
export const RSS_SUBSCRIPTIONS_REFETCH_INTERVAL_MS = 30_000;
export const RSS_ENTRIES_REFETCH_INTERVAL_MS = 60_000;

export function rssEntriesRefetchInterval(
  request: { query?: string },
  enabled: boolean,
) {
  return enabled && !request.query?.trim()
    ? RSS_ENTRIES_REFETCH_INTERVAL_MS
    : false;
}

export const RSS_SUBSCRIPTIONS_STALE_TIME_MS = 30 * 60_000;
export const RSS_SUBSCRIPTIONS_GC_TIME_MS = 2 * 60 * 60_000;

export const RSS_ENTRIES_STALE_TIME_MS = 10 * 60_000;
export const RSS_ENTRIES_GC_TIME_MS = 60 * 60_000;

export const RSS_SEARCH_STALE_TIME_MS = 10 * 60_000;
export const RSS_SEARCH_GC_TIME_MS = 60 * 60_000;

export const RSS_ENTRY_DETAIL_STALE_TIME_MS = 30 * 60_000;
export const RSS_ENTRY_DETAIL_GC_TIME_MS = 2 * 60 * 60_000;

export const RSS_DISCOVERY_STALE_TIME_MS = 24 * 60 * 60_000;
export const RSS_DISCOVERY_GC_TIME_MS = 48 * 60 * 60_000;

/**
 * Cached RSS projections are refreshed by polling, mutations, or a newer
 * SQLite snapshot. Merely returning to a route must not turn elapsed wall time
 * into a second request. Explicit invalidation remains authoritative.
 */
export function refetchInvalidatedRSSQueryOnMount(query: {
  state: { isInvalidated: boolean };
}) {
  return query.state.isInvalidated ? "always" as const : false;
}

export const RSS_SUBSCRIPTIONS_QUERY_POLICY = {
  staleTime: RSS_SUBSCRIPTIONS_STALE_TIME_MS,
  gcTime: RSS_SUBSCRIPTIONS_GC_TIME_MS,
  refetchOnMount: refetchInvalidatedRSSQueryOnMount,
  refetchOnReconnect: false,
  refetchOnWindowFocus: false,
} as const;

export const RSS_ENTRIES_QUERY_POLICY = {
  staleTime: RSS_ENTRIES_STALE_TIME_MS,
  gcTime: RSS_ENTRIES_GC_TIME_MS,
  refetchOnMount: refetchInvalidatedRSSQueryOnMount,
  refetchOnReconnect: false,
  refetchOnWindowFocus: false,
} as const;

export const RSS_SEARCH_QUERY_POLICY = {
  staleTime: RSS_SEARCH_STALE_TIME_MS,
  gcTime: RSS_SEARCH_GC_TIME_MS,
  // Search has no background poller. A fresh result survives route changes;
  // after its TTL it revalidates when the user returns.
  refetchOnMount: true,
  refetchOnReconnect: false,
  refetchOnWindowFocus: false,
} as const;

export const RSS_ENTRY_DETAIL_QUERY_POLICY = {
  staleTime: RSS_ENTRY_DETAIL_STALE_TIME_MS,
  gcTime: RSS_ENTRY_DETAIL_GC_TIME_MS,
  refetchOnMount: refetchInvalidatedRSSQueryOnMount,
  refetchOnReconnect: false,
  refetchOnWindowFocus: false,
} as const;

export const RSS_DISCOVERY_QUERY_POLICY = {
  staleTime: RSS_DISCOVERY_STALE_TIME_MS,
  gcTime: RSS_DISCOVERY_GC_TIME_MS,
  // Browse data is remote and otherwise static in the renderer, so its
  // 24-hour TTL must remain an actual revalidation boundary.
  refetchOnMount: true,
  refetchOnReconnect: false,
  refetchOnWindowFocus: false,
} as const;

export function rssDiscoveryQueryPolicy(request: { query?: string }) {
  return request.query?.trim()
    ? RSS_SEARCH_QUERY_POLICY
    : RSS_DISCOVERY_QUERY_POLICY;
}
