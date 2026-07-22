import { describe, expect, test } from "bun:test";

import { getXiaText } from "@/features/xiadown/shared";
import { resolveListenLyricsErrorPresentation } from "@/app/main/listen/lyrics-errors";

function lyricsError(code: string, retryable = true) {
  return Object.assign(
    new Error(
      'Get "https://lrclib.net/api/get-cached?artist_name=Private": Bad Gateway',
    ),
    { code, retryable },
  );
}

describe("listen lyrics public error presentation", () => {
  const text = getXiaText("en");

  test.each([
    ["lyrics_provider_unavailable", text.listen.lyricsErrorProviderUnavailable],
    ["lyrics_provider_transient", text.listen.lyricsErrorProviderUnavailable],
    ["lyrics_rate_limited", text.listen.lyricsErrorRateLimited],
    ["lyrics_timeout", text.listen.lyricsErrorTimeout],
    [
      "lyrics_network_unavailable",
      text.listen.lyricsErrorNetworkUnavailable,
    ],
  ])("maps %s to a localized actionable message", (code, message) => {
    const presentation = resolveListenLyricsErrorPresentation(
      text,
      lyricsError(code),
    );

    expect(presentation.message).toBe(message);
    expect(presentation.retryable).toBe(true);
    expect(presentation.message).not.toContain("https://");
    expect(presentation.message).not.toContain("Bad Gateway");
  });

  test("normalizes unknown provider details instead of exposing them", () => {
    expect(
      resolveListenLyricsErrorPresentation(
        text,
        new Error("private provider payload"),
      ),
    ).toEqual({
      code: "lyrics_unavailable",
      message: text.listen.lyricsErrorUnavailable,
      retryable: true,
    });
  });

  test("uses a lyrics-specific message for an authentication failure", () => {
    expect(
      resolveListenLyricsErrorPresentation(
        text,
        lyricsError("lyrics_auth_required", false),
      ),
    ).toEqual({
      code: "lyrics_auth_required",
      message: text.listen.lyricsErrorAuthRequired,
      retryable: false,
    });
  });
});
