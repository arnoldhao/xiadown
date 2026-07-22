export const RSS_IMAGE_SESSION_CACHE_LIMIT = 512;
export const RSS_IMAGE_FAILURE_TTL_MS = 30_000;
export const RSS_SCROLL_SESSION_CACHE_LIMIT = 64;
export const RSS_SELECTION_SESSION_CACHE_LIMIT = 64;

/**
 * A small process-local LRU for presentation state. RSS data itself belongs in
 * TanStack Query/the backend; this cache is only for inexpensive UI state that
 * should survive a React unmount during the current app session.
 */
export class BoundedSessionLRU<Key, Value> {
  private readonly values = new Map<Key, Value>();

  constructor(readonly capacity: number) {
    if (!Number.isInteger(capacity) || capacity < 1) {
      throw new RangeError("capacity must be a positive integer");
    }
  }

  get size() {
    return this.values.size;
  }

  get(key: Key): Value | undefined {
    if (!this.values.has(key)) {
      return undefined;
    }
    const value = this.values.get(key) as Value;
    this.values.delete(key);
    this.values.set(key, value);
    return value;
  }

  set(key: Key, value: Value) {
    this.values.delete(key);
    this.values.set(key, value);
    while (this.values.size > this.capacity) {
      const oldest = this.values.keys().next();
      if (oldest.done) {
        break;
      }
      this.values.delete(oldest.value);
    }
  }

  delete(key: Key) {
    return this.values.delete(key);
  }

  clear() {
    this.values.clear();
  }
}

export type RSSImageSessionState =
  | { status: "loaded" }
  | { status: "failed"; retryAt: number };

export class RSSImageSessionCache {
  private readonly values: BoundedSessionLRU<string, RSSImageSessionState>;

  constructor(
    capacity = RSS_IMAGE_SESSION_CACHE_LIMIT,
    readonly failureTTL = RSS_IMAGE_FAILURE_TTL_MS,
  ) {
    if (!Number.isFinite(failureTTL) || failureTTL < 0) {
      throw new RangeError("failureTTL must be a non-negative number");
    }
    this.values = new BoundedSessionLRU(capacity);
  }

  get size() {
    return this.values.size;
  }

  get(url: string, now = Date.now()): RSSImageSessionState | undefined {
    const state = this.values.get(url);
    if (state?.status === "failed" && state.retryAt <= now) {
      this.values.delete(url);
      return undefined;
    }
    return state;
  }

  markLoaded(url: string) {
    this.values.set(url, { status: "loaded" });
  }

  markFailed(url: string, now = Date.now()) {
    this.values.set(url, {
      status: "failed",
      retryAt: now + this.failureTTL,
    });
  }

  clear() {
    this.values.clear();
  }
}

const imageSessionCache = new RSSImageSessionCache();
const scrollSessionCache = new BoundedSessionLRU<string, number>(
  RSS_SCROLL_SESSION_CACHE_LIMIT,
);
const selectionSessionCache = new BoundedSessionLRU<string, string>(
  RSS_SELECTION_SESSION_CACHE_LIMIT,
);

export function getRSSImageSessionState(url: string, now = Date.now()) {
  return imageSessionCache.get(url, now);
}

export function markRSSImageLoaded(url: string) {
  imageSessionCache.markLoaded(url);
}

export function markRSSImageFailed(url: string, now = Date.now()) {
  imageSessionCache.markFailed(url, now);
}

export interface RSSScrollCacheKeyParts {
  routeId: string;
  presentation?: string;
  subscriptionId?: string | null;
  filter?: string;
}

/** JSON tuples avoid collisions between route/filter values containing delimiters. */
export function buildRSSScrollCacheKey({
  routeId,
  presentation = "",
  subscriptionId = "",
  filter = "",
}: RSSScrollCacheKeyParts) {
  return JSON.stringify([
    "rss-scroll-v1",
    routeId.trim(),
    subscriptionId?.trim() ?? "",
    presentation.trim(),
    filter.trim(),
  ]);
}

export function readRSSScrollOffset(key: string) {
  return scrollSessionCache.get(key) ?? 0;
}

export function writeRSSScrollOffset(key: string, offset: number) {
  if (!Number.isFinite(offset)) {
    return;
  }
  scrollSessionCache.set(key, Math.max(0, offset));
}

export function readRSSSelectedEntryID(key: string) {
  return selectionSessionCache.get(key) ?? "";
}

export function writeRSSSelectedEntryID(key: string, entryID: string) {
  const normalized = entryID.trim();
  if (!normalized) {
    selectionSessionCache.delete(key);
    return;
  }
  selectionSessionCache.set(key, normalized);
}
