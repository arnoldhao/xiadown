import { describe, expect, test } from "bun:test";

import { normalizeListenLyricsSnapshot } from "@/app/main/listen/lyrics-api";

describe("listen lyrics api adapter", () => {
  test("normalizes lyrics snapshots from the lyrics service", () => {
    const normalized = normalizeListenLyricsSnapshot({
      videoId: "a-video",
      kind: "plain",
      source: "test",
      text: "hello",
      loading: false,
    });

    expect(normalized?.videoId).toBe("a-video");
    expect(normalized?.kind).toBe("plain");
    expect(normalized?.text).toBe("hello");
  });
});
