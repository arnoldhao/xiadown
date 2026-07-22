export const RSS_PREVIEW_NOTICE_DISMISSED_STORAGE_KEY =
  "xiadown:rss:preview-notice-dismissed:v1";

export function readRSSPreviewNoticeDismissed() {
  if (typeof window === "undefined") return false;
  try {
    return window.localStorage.getItem(
      RSS_PREVIEW_NOTICE_DISMISSED_STORAGE_KEY,
    ) === "true";
  } catch {
    return false;
  }
}

export function dismissRSSPreviewNotice() {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(
      RSS_PREVIEW_NOTICE_DISMISSED_STORAGE_KEY,
      "true",
    );
  } catch {
    // Storage can be unavailable in a locked-down WebView. The notice remains
    // session-dismissible and will be offered again on a later RSS visit.
  }
}
