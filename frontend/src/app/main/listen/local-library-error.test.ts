import { describe, expect, test } from "bun:test";

import { preserveListenLocalTracksAfterLoadFailure } from "@/app/main/listen/local-library";

describe("local track index failure state", () => {
  test("keeps the last successful tracks and exposes a retryable error", () => {
    const tracks = [{ id: "existing" }];
    const failure = preserveListenLocalTracksAfterLoadFailure(
      tracks,
      new Error("listen local failed: HTTP 503"),
    );

    expect(failure.tracks).toBe(tracks);
    expect(failure.error).toBe("listen local failed: HTTP 503");
  });

  test("normalizes non-error failures without dropping the existing index", () => {
    const tracks = [{ id: "existing" }];
    const failure = preserveListenLocalTracksAfterLoadFailure(tracks, null);

    expect(failure.tracks).toBe(tracks);
    expect(failure.error).toBe("listen local unavailable");
  });
});
