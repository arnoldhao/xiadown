export type BrowserScanReasonTranslationKey =
  | "browserSource.reasonNoAuthCookies"
  | "browserSource.reasonSourceUnavailable"
  | "browserSource.reasonBrowserCookieAccessRequired"
  | "browserSource.reasonProtectedCookiesUnsupported";

export function browserScanReasonTranslationKey(
  value?: string,
): BrowserScanReasonTranslationKey | null {
  switch (value?.trim().toLowerCase()) {
    case "no_auth_cookies":
      return "browserSource.reasonNoAuthCookies";
    case "source_unavailable":
      return "browserSource.reasonSourceUnavailable";
    case "browser_cookie_access_required":
      return "browserSource.reasonBrowserCookieAccessRequired";
    case "protected_cookies_unsupported":
      return "browserSource.reasonProtectedCookiesUnsupported";
    default:
      return null;
  }
}

export function isBrowserScanSourceNotice(value?: string) {
  const normalized = value?.trim().toLowerCase();
  return (
    normalized === "browser_cookie_access_required" ||
    normalized === "protected_cookies_unsupported"
  );
}
