import type {
  RSSPreviewResult,
  RSSPreviewSubscriptionRequest,
} from "./types";
import { canonicalizeRSSHubInput } from "./discovery-utils";

export const RSS_PREVIEW_SUCCESS_TTL_MS = 4 * 60_000;
export const RSS_PREVIEW_FAILURE_TTL_MS = 60_000;
// Match the backend's bounded preview-lease pool so the renderer never assumes
// a token is reusable after the service had to evict it for a newer preview.
export const RSS_PREVIEW_CACHE_MAX_ENTRIES = 16;

type PreviewLoader = (
  request: RSSPreviewSubscriptionRequest,
) => Promise<RSSPreviewResult>;

type PreviewCacheValue =
  | { expiresAt: number; kind: "success"; value: RSSPreviewResult }
  | { expiresAt: number; kind: "failure"; value: unknown };

interface RSSPreviewCacheOptions {
  failureTTL?: number;
  maxEntries?: number;
  now?: () => number;
  successTTL?: number;
}

/**
 * A small session cache for feed previews. The backend preview lease lives for
 * five minutes, so the success TTL deliberately expires first. In-flight
 * requests share one promise and explicit retry bypasses the negative cache.
 */
export class RSSPreviewCache {
  private readonly cache = new Map<string, PreviewCacheValue>();
  private readonly inFlight = new Map<string, Promise<RSSPreviewResult>>();
  private readonly failureTTL: number;
  private readonly maxEntries: number;
  private readonly now: () => number;
  private readonly successTTL: number;

  constructor(
    private readonly load: PreviewLoader,
    options: RSSPreviewCacheOptions = {},
  ) {
    this.failureTTL = options.failureTTL ?? RSS_PREVIEW_FAILURE_TTL_MS;
    this.maxEntries = Math.max(1, options.maxEntries ?? RSS_PREVIEW_CACHE_MAX_ENTRIES);
    this.now = options.now ?? Date.now;
    this.successTTL = options.successTTL ?? RSS_PREVIEW_SUCCESS_TTL_MS;
  }

  get(
    request: RSSPreviewSubscriptionRequest,
    options: { force?: boolean } = {},
  ): Promise<RSSPreviewResult> {
    const key = rssPreviewCacheKey(request);
    if (options.force) {
      this.cache.delete(key);
    } else {
      const cached = this.read(key);
      if (cached?.kind === "success") return Promise.resolve(cached.value);
      if (cached?.kind === "failure") return Promise.reject(cached.value);
    }

    const active = this.inFlight.get(key);
    if (active) return active;

    const pending = this.load(request)
      .then((result) => {
        this.write(key, {
          expiresAt: this.now() + this.successTTL,
          kind: "success",
          value: result,
        });
        return result;
      })
      .catch((error: unknown) => {
        this.write(key, {
          expiresAt: this.now() + this.failureTTL,
          kind: "failure",
          value: error,
        });
        throw error;
      })
      .finally(() => {
        if (this.inFlight.get(key) === pending) this.inFlight.delete(key);
      });
    this.inFlight.set(key, pending);
    return pending;
  }

  delete(request: RSSPreviewSubscriptionRequest) {
    this.cache.delete(rssPreviewCacheKey(request));
  }

  clear() {
    this.cache.clear();
  }

  private read(key: string) {
    const value = this.cache.get(key);
    if (!value) return undefined;
    if (value.expiresAt <= this.now()) {
      this.cache.delete(key);
      return undefined;
    }
    this.cache.delete(key);
    this.cache.set(key, value);
    return value;
  }

  private write(key: string, value: PreviewCacheValue) {
    this.cache.delete(key);
    this.cache.set(key, value);
    while (this.cache.size > this.maxEntries) {
      const oldest = this.cache.keys().next().value;
      if (oldest === undefined) break;
      this.cache.delete(oldest);
    }
  }
}

export function rssPreviewCacheKey(request: RSSPreviewSubscriptionRequest) {
  return `${request.viewType ?? "auto"}\u0000${canonicalizeRSSHubInput(request.url)}`;
}
