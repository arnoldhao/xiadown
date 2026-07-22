import { resolveUnknownErrorMessage } from "@/app/main/helpers";

import type {
  RSSAddSubscriptionRequest,
  RSSPreviewResult,
  RSSSubscription,
  RSSUpdateSubscriptionRequest,
  RSSViewType,
} from "./types";

export interface RSSSubscriptionSettingsDraft {
  title: string;
  viewType: RSSViewType;
  enabled: boolean;
  categoryId?: string;
}

/** Build a true patch so an edit cannot overwrite fields updated in parallel. */
export function buildRSSSubscriptionUpdateRequest(
  subscription: RSSSubscription,
  draft: RSSSubscriptionSettingsDraft,
): RSSUpdateSubscriptionRequest {
  const title = draft.title.trim();
  return {
    id: subscription.id,
    ...(title !== subscription.title ? { title } : {}),
    ...(draft.viewType !== subscription.viewType
      ? { viewType: draft.viewType }
      : {}),
    ...(draft.enabled !== subscription.enabled
      ? { enabled: draft.enabled }
      : {}),
    ...(draft.categoryId !== undefined &&
        draft.categoryId.trim() !== (subscription.categoryId || "")
      ? { categoryId: draft.categoryId.trim() }
      : {}),
  };
}

export function buildRSSAddSubscriptionRequest(
  url: string,
  viewType: RSSViewType,
  preview: RSSPreviewResult | null,
  title?: string,
): RSSAddSubscriptionRequest {
  const customTitle = title?.trim();
  if (!preview) return {
    url,
    viewType,
    ...(customTitle ? { title: customTitle } : {}),
    allowPending: true,
  };
  return {
    url,
    viewType,
    title: customTitle || preview.subscription.title,
    previewToken: preview.previewToken,
    allowPending: true,
  };
}

/**
 * Wails may append a structured RuntimeError payload to its transport message.
 * Keep that payload, URLs, and nested causes out of the UI while retaining the
 * one bounded detail that is reliably useful to a reader: an HTTP status code.
 */
export function rssPreviewErrorText(error: unknown, fallback: string) {
  const normalized = resolveUnknownErrorMessage(error, "").trim();
  const status = normalized.match(
    /\bHTTP(?:\s+(?:status|status code))?\s*[:=]?\s*(\d{3})\b/i,
  )?.[1];
  return status ? `HTTP ${status} · ${fallback}` : fallback;
}
