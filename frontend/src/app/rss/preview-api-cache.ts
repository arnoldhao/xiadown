import { previewRSSSubscription } from "./api";
import { RSSPreviewCache } from "./preview-cache";
import type { RSSPreviewSubscriptionRequest } from "./types";

// Keep transport wiring outside the reusable cache primitive. Tests can load
// and exercise RSSPreviewCache without initializing Wails, while the product
// owns exactly one preview/lease cache for the renderer session.
const rssPreviewCache = new RSSPreviewCache(previewRSSSubscription);

export function cachedPreviewRSSSubscription(
  request: RSSPreviewSubscriptionRequest,
  options?: { force?: boolean },
) {
  return rssPreviewCache.get(request, options);
}

export function deleteCachedRSSPreview(request: RSSPreviewSubscriptionRequest) {
  rssPreviewCache.delete(request);
}
