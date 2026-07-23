import { describe, expect, test } from "bun:test";

import {
  buildListenLyricsCandidateTrack,
  createListenLyricsPreviewCache,
  createListenLyricsRequestGate,
  formatListenLyricsOffset,
  listenLyricsCandidatePreviewKey,
  normalizeListenLyricsWorkspaceOffset,
  resolveListenLyricsRenderTimeMs,
  stepListenLyricsWorkspaceOffset,
} from "@/app/main/listen/lyrics-workspace-state";

describe("listen lyrics workspace state", () => {
  test("uses positive offsets to show lyrics earlier", () => {
    expect(resolveListenLyricsRenderTimeMs(2_000, 250)).toBe(2_250);
    expect(resolveListenLyricsRenderTimeMs(100, -250)).toBe(0);
    expect(resolveListenLyricsRenderTimeMs(2_000, -250)).toBe(1_750);
    expect(normalizeListenLyricsWorkspaceOffset(8_000)).toBe(5_000);
    expect(formatListenLyricsOffset(-250)).toBe("−0.25 s");
    expect(stepListenLyricsWorkspaceOffset(0, "earlier")).toBe(250);
    expect(stepListenLyricsWorkspaceOffset(0, "later")).toBe(-250);
    expect(stepListenLyricsWorkspaceOffset(5_000, "earlier")).toBe(5_000);
  });

  test("builds normalized editable candidate searches and stable preview keys", () => {
    const track = buildListenLyricsCandidateTrack(
      {
        lyricsId: " local:track ",
        videoId: " video ",
        title: "Original",
        localPath: " /music/song.flac ",
        durationSeconds: 212.6,
      },
      { title: " Song ", artist: " Artist ", album: " Album " },
    );
    expect(track).toEqual({
      lyricsId: "local:track",
      videoId: "video",
      title: "Song",
      artist: "Artist",
      album: "Album",
      localPath: "/music/song.flac",
      durationSeconds: 212.6,
    });
    expect(
      listenLyricsCandidatePreviewKey({
        track,
        candidate: { providerId: "LRCLIB", providerTrackId: "42" },
        language: "zh-CN",
      }),
    ).toContain("lrclib:42");
  });

  test("deduplicates candidate previews and lets failed previews retry", async () => {
    const cache = createListenLyricsPreviewCache();
    let calls = 0;
    const lyrics = {
      videoId: "one",
      kind: "plain" as const,
      source: "test",
      text: "hello",
      lines: [],
    };
    const first = cache.load("same", async () => {
      calls += 1;
      return lyrics;
    });
    const second = cache.load("same", async () => {
      calls += 1;
      return lyrics;
    });
    expect(first).toBe(second);
    expect(await second).toBe(lyrics);
    expect(calls).toBe(1);

    await expect(
      cache.load("failed", async () => {
        throw new Error("failed");
      }),
    ).rejects.toThrow("failed");
    await Promise.resolve();
    expect(cache.size()).toBe(1);
  });

  test("invalidates stale async request generations", () => {
    const gate = createListenLyricsRequestGate();
    const first = gate.begin();
    const second = gate.begin();
    expect(gate.isCurrent(first)).toBe(false);
    expect(gate.isCurrent(second)).toBe(true);
    gate.invalidate();
    expect(gate.isCurrent(second)).toBe(false);
  });
});
