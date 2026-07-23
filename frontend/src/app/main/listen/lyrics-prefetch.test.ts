import { describe, expect, test } from "bun:test";

import { canPrefetchListenTrackLyrics } from "@/app/main/listen/lyrics-prefetch";

describe("listen lyrics prefetch", () => {
  test("prefetches only an enabled identified track", () => {
    expect(
      canPrefetchListenTrackLyrics(true, {
        videoId: "TESTVID007G",
        title: "Track",
      }),
    ).toBe(true);
    expect(
      canPrefetchListenTrackLyrics(false, {
        videoId: "TESTVID007G",
        title: "Track",
      }),
    ).toBe(false);
    expect(
      canPrefetchListenTrackLyrics(true, { videoId: "", title: "Track" }),
    ).toBe(false);
  });
});
