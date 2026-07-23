import { describe, expect, test } from "bun:test";

import { browserScanReasonTranslationKey } from "./browserScanReason";

describe("browser scan reason presentation", () => {
  test("maps supported backend codes to localized messages", () => {
    expect(browserScanReasonTranslationKey("no_auth_cookies")).toBe(
      "browserSource.reasonNoAuthCookies",
    );
    expect(browserScanReasonTranslationKey("source_unavailable")).toBe(
      "browserSource.reasonSourceUnavailable",
    );
    expect(browserScanReasonTranslationKey("browser_cookie_access_required")).toBe(
      "browserSource.reasonBrowserCookieAccessRequired",
    );
    expect(browserScanReasonTranslationKey("protected_cookies_unsupported")).toBe(
      "browserSource.reasonProtectedCookiesUnsupported",
    );
  });

  test("never exposes unknown backend reason codes", () => {
    expect(browserScanReasonTranslationKey("private_backend_detail")).toBeNull();
    expect(browserScanReasonTranslationKey(" ")).toBeNull();
    expect(browserScanReasonTranslationKey()).toBeNull();
  });
});
