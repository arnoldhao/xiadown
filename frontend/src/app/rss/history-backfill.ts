import type {
  RSSBackfillHistoryRequest,
  RSSBackfillHistoryResult,
  RSSEntryPage,
  RSSListEntriesRequest,
} from "./types";

export const RSS_HISTORY_BACKFILL_PAGE_BUDGET = 12;
export const RSS_PENDING_HYDRATION_REFETCH_INTERVAL_MS = 2_000;
export const RSS_PENDING_HYDRATION_REFETCH_WINDOW_MS = 2 * 60_000;

export interface RSSHistoryCollectionMetric {
  total: number;
  visibleEntries: number;
}

export interface RSSHistorySentinelReadiness {
  enabled: boolean;
  busy: boolean;
  continuation: string;
}

export interface RSSHistorySentinelGate {
  setVisible: (visible: boolean) => void;
  tryAcquire: (readiness: RSSHistorySentinelReadiness) => boolean;
}

/**
 * Converts an entry-list filter into the backend's bounded history scope.
 * Read/search/star filters describe local state, not a remote feed history,
 * and must never cause an upstream backfill.
 */
export function rssBackfillRequestForEntries(
  request: RSSListEntriesRequest,
): RSSBackfillHistoryRequest | null {
  if (request.query?.trim() || request.unreadOnly || request.starredOnly) {
    return null;
  }
  const subscriptionId = request.subscriptionId?.trim();
  if (subscriptionId) {
    return { subscriptionId };
  }
  if (request.kind) {
    return { kind: request.kind };
  }
  return {};
}

export function rssBackfillRequestKey(
  request: RSSBackfillHistoryRequest | null,
) {
  if (request === null) return "disabled";
  if (request.subscriptionId) return `subscription:${request.subscriptionId}`;
  if (request.kind) return `kind:${request.kind}`;
  return "all";
}

export function rssBackfillChangeCount(result: RSSBackfillHistoryResult) {
  return Math.max(0, result.created) + Math.max(0, result.updated);
}

/**
 * Tracks what the active filtered query can actually render. A kind-scoped
 * backend backfill may report aggregate changes from another entry kind, so
 * backend counters alone are not sufficient evidence that this collection
 * made progress.
 */
export function rssHistoryCollectionMetric(
  pages: readonly RSSEntryPage[] | undefined,
): RSSHistoryCollectionMetric {
  const visibleEntries = new Set<string>();
  let total = 0;
  for (const page of pages ?? []) {
    total = Math.max(total, nonNegativeInteger(page.total));
    for (const entry of page.items) visibleEntries.add(entry.id);
  }
  return { total, visibleEntries: visibleEntries.size };
}

export function rssHistoryCollectionGrew(
  before: RSSHistoryCollectionMetric,
  after: RSSHistoryCollectionMetric,
) {
  return after.total > before.total || after.visibleEntries > before.visibleEntries;
}

export function rssHistorySessionShouldStop(
  result: RSSBackfillHistoryResult,
  before: RSSHistoryCollectionMetric,
  after: RSSHistoryCollectionMetric,
  attemptedPages: number,
) {
  return (
    attemptedPages >= RSS_HISTORY_BACKFILL_PAGE_BUDGET ||
    rssBackfillShouldStop(result) ||
    !rssHistoryCollectionGrew(before, after)
  );
}

export function rssSubscriptionHistoryReady(
  subscriptionId: string | null | undefined,
  lastSuccessAt: string | undefined,
) {
  return !subscriptionId?.trim() || Boolean(lastSuccessAt?.trim());
}

export function rssShouldFastPollPendingSubscription({
  enabled,
  subscriptionId,
  lastSuccessAt,
  visibleEntries,
  now,
  deadline,
}: {
  enabled: boolean;
  subscriptionId: string | null | undefined;
  lastSuccessAt: string | undefined;
  visibleEntries: number;
  now: number;
  deadline: number;
}) {
  return (
    enabled &&
    !rssSubscriptionHistoryReady(subscriptionId, lastSuccessAt) &&
    visibleEntries === 0 &&
    now < deadline
  );
}

/**
 * Acquires each settled pagination generation once while the sentinel is
 * visible. When an async request settles, a new continuation can therefore be
 * acquired even if IntersectionObserver does not emit another edge event.
 */
export function createRSSHistorySentinelGate(): RSSHistorySentinelGate {
  let visible = false;
  const acquired = new Set<string>();
  return {
    setVisible(nextVisible) {
      visible = nextVisible;
    },
    tryAcquire({ enabled, busy, continuation }) {
      const normalized = continuation.trim();
      if (
        !visible ||
        !enabled ||
        busy ||
        !normalized ||
        acquired.has(normalized)
      ) {
        return false;
      }
      acquired.add(normalized);
      return true;
    },
  };
}

/**
 * A successful call with no inserts/updates is terminal for automatic
 * pagination even if a malformed source reports hasMore. This is the loop
 * breaker for a permanently visible IntersectionObserver sentinel.
 */
export function rssBackfillShouldStop(result: RSSBackfillHistoryResult) {
  return !result.hasMore || rssBackfillChangeCount(result) === 0;
}

export function rssBackfillFailureMessage(result: RSSBackfillHistoryResult) {
  if (result.failed <= 0 || rssBackfillChangeCount(result) > 0) return "";
  const messages = result.sources
    .map((source) => source.error?.trim() || "")
    .filter(Boolean);
  return messages[0] || "RSS history backfill failed";
}

function nonNegativeInteger(value: number) {
  return Number.isFinite(value) ? Math.max(0, Math.trunc(value)) : 0;
}
