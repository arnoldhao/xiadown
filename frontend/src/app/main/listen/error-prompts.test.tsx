import { describe, expect, test } from "bun:test";

import type { ListenPageProps } from "./types";
import { resolveListenLibraryErrorPrompt } from "./error-prompts";

const text = {
  listen: {
    onlineAuthRequired: "Authentication required",
    openConnections: "Open connections",
    onlineAuthExpired: "Authentication expired",
    onlineNetworkUnavailable: "Network unavailable",
    onlineRegionUnavailable: "Region unavailable",
    onlineTransientUnavailable: "Temporarily unavailable",
    refresh: "Refresh",
    onlineServiceUnavailable: "Service unavailable",
  },
} as ListenPageProps["text"];

describe("Listen library error prompts", () => {
  test("routes missing or expired cookies to the connections prompt", () => {
    expect(resolveListenLibraryErrorPrompt("youtube_cookies_missing", text)).toMatchObject({
      message: "Authentication required",
      actionLabel: "Open connections",
      action: "connections",
    });
    expect(resolveListenLibraryErrorPrompt("youtube_auth_expired", text)).toMatchObject({
      message: "Authentication expired",
      actionLabel: "Open connections",
      action: "connections",
    });
  });

  test("routes network failures to the retry prompt", () => {
    expect(
      resolveListenLibraryErrorPrompt("youtube_network_unavailable", text),
    ).toMatchObject({
      message: "Network unavailable",
      actionLabel: "Refresh",
      action: "refresh",
    });
    expect(resolveListenLibraryErrorPrompt("youtube_timeout", text)).toMatchObject({
      message: "Network unavailable",
      actionLabel: "Refresh",
      action: "refresh",
    });
    expect(
      resolveListenLibraryErrorPrompt("youtube_tls_unavailable", text),
    ).toMatchObject({
      message: "Network unavailable",
      actionLabel: "Refresh",
      action: "refresh",
    });
  });

  test("keeps region errors non-retryable and transient errors actionable", () => {
    const region = resolveListenLibraryErrorPrompt(
      "youtube_region_unavailable",
      text,
      true,
    );
    expect(region).toEqual({ message: "Region unavailable" });

    expect(
      resolveListenLibraryErrorPrompt(
        "youtube_transient_unavailable",
        text,
      ),
    ).toMatchObject({
      message: "Temporarily unavailable",
      actionLabel: "Refresh",
      action: "refresh",
    });
  });

  test("only offers Refresh for retryable unknown errors", () => {
    expect(resolveListenLibraryErrorPrompt("unexpected", text)).toEqual({
      message: "Service unavailable",
    });
    expect(
      resolveListenLibraryErrorPrompt("unexpected", text, true),
    ).toMatchObject({
      message: "Service unavailable",
      actionLabel: "Refresh",
      action: "refresh",
    });
  });

  test("preserves the backend retryable contract through the library page", async () => {
    const [listenSource, pageSource] = await Promise.all([
      Bun.file(new URL("../Listen.tsx", import.meta.url)).text(),
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
    ]);

    expect(listenSource).toContain("getListenErrorRetryable");
    expect(listenSource).toContain(
      "setLibraryErrorRetryable(getListenErrorRetryable(error))",
    );
    expect(listenSource).toContain("libraryErrorRetryable,");
    expect(pageSource).toContain(
      "libraryErrorCode,\n    props.text,\n    libraryErrorRetryable,",
    );
    expect(pageSource).toContain(
      'libraryErrorPrompt.action === "refresh"\n                ? reloadLibrary\n                : undefined',
    );
  });
});
