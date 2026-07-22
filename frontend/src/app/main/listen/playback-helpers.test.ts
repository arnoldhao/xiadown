import { describe, expect, test } from "bun:test";

import { fetchListenLyricsCached,forgetListenLyricsCache,forgetListenLyricsCacheVariants,hasTrustedListenOnlineArtist,isListenLiveEventForSession,listenArtistCountFromLabelParts,loadListenLyricsCached,readListenLyricsCache,resolveListenLyricsCurrentState,resolveListenPlaybackActivity,resolveListenRadioFullscreenVideoDecision,resolveListenTrackVideoAvailability,resolveTrustedListenOnlineArtistLabel,splitListenArtistLabel } from "@/app/main/listen/playback-helpers";
import type { ListenOnlineItem } from "@/app/main/listen/types";

function item(overrides: Partial<ListenOnlineItem>): ListenOnlineItem {
  return {
    id: "track",
    group: "playlist",
    videoId: "TESTVID007G",
    title: "Track",
    channel: "Artist",
    description: "",
    durationLabel: "",
    ...overrides,
  };
}

describe("listen live player event identity", () => {
  test("rejects YouTube and stale stream sessions before Hush reconciliation", () => {
    expect(
      isListenLiveEventForSession(
        { provider: "stream", sessionId: "radio-2" },
        "stream",
        "radio-2",
      ),
    ).toBe(true);
    expect(
      isListenLiveEventForSession(
        { provider: "youtube", sessionId: "radio-2" },
        "stream",
        "radio-2",
      ),
    ).toBe(false);
    expect(
      isListenLiveEventForSession(
        { provider: "stream", sessionId: "old-radio" },
        "stream",
        "radio-2",
      ),
    ).toBe(false);
  });
});

describe("listen playback activity projection", () => {
  test("keeps an initial load pausable while showing loading", () => {
    expect(resolveListenPlaybackActivity("loading")).toEqual({
      transportActive: true,
      timelineRunning: false,
      loading: true,
    });
  });

  test("keeps buffering active while freezing lyrics and showing loading", () => {
    expect(resolveListenPlaybackActivity("buffering")).toEqual({
      transportActive: true,
      timelineRunning: false,
      loading: true,
    });
    expect(resolveListenPlaybackActivity("playing")).toEqual({
      transportActive: true,
      timelineRunning: true,
      loading: false,
    });
  });
});

describe("listen playback video availability", () => {
  test("treats audio endpoint musicVideoType as confirmed no video", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({ musicVideoType: "MUSIC_VIDEO_TYPE_ATV" }),
        false,
      ),
    ).toBe("unavailable");
  });

  test("does not treat user generated endpoint type as confirmed video", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({ musicVideoType: "MUSIC_VIDEO_TYPE_UGC" }),
        false,
      ),
    ).toBe("checking");
  });

  test("ignores stale non-ATV unavailable metadata", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({
          musicVideoType: "MUSIC_VIDEO_TYPE_UGC",
          hasVideo: false,
          videoAvailabilityKnown: true,
        }),
        false,
      ),
    ).toBe("checking");
  });

  test("treats a non-video thumbnail as unavailable once artwork is known", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({
          musicVideoType: "MUSIC_VIDEO_TYPE_UGC",
          thumbnailUrl: "https://lh3.googleusercontent.com/art=w544-h544",
        }),
        false,
      ),
    ).toBe("unavailable");
  });

  test("infers video from YouTube video thumbnail", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({
          musicVideoType: "MUSIC_VIDEO_TYPE_PODCAST_EPISODE",
          thumbnailUrl: "https://i.ytimg.com/vi/TESTVID007G/hq720.jpg",
        }),
        false,
      ),
    ).toBe("available");
  });

  test("keeps authoritative unavailable metadata unavailable", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({
          hasVideo: false,
          videoAvailabilityKnown: true,
          musicVideoType: "MUSIC_VIDEO_TYPE_ATV",
        }),
        false,
      ),
    ).toBe("unavailable");
  });
});

describe("radio fullscreen video default", () => {
  const input = {
    presentation: "fullscreen" as const,
    workspaceFullscreen: true,
    active: true,
    enabled: true,
    live: true,
    trackKey: "radio:liveVideo01",
    attemptedTrackKey: "",
    hasVideo: true,
    nativeVideoAvailable: true,
    queueOpen: false,
    mediaMode: "cover" as const,
  };

  test("opens video once on the first fullscreen entry", () => {
    expect(resolveListenRadioFullscreenVideoDecision(input)).toBe("open");
  });

  test("does not force video open again after the user closes it", () => {
    expect(
      resolveListenRadioFullscreenVideoDecision({
        ...input,
        attemptedTrackKey: input.trackKey,
      }),
    ).toBe("keep");
  });

  test("allows the next radio track to make its own fullscreen attempt", () => {
    expect(
      resolveListenRadioFullscreenVideoDecision({
        ...input,
        attemptedTrackKey: "radio:previousLiveVideo",
      }),
    ).toBe("open");
  });

  test("falls back from video when the native surface is unavailable", () => {
    expect(
      resolveListenRadioFullscreenVideoDecision({
        ...input,
        attemptedTrackKey: input.trackKey,
        mediaMode: "video",
        nativeVideoAvailable: false,
      }),
    ).toBe("fallback");
  });
});

describe("listen playback artist provenance", () => {
  test("does not trust plain API text recommendation labels as artists", () => {
    const track = item({ channel: "Made for", artistSource: "api-text" });

    expect(hasTrustedListenOnlineArtist(track)).toBe(false);
    expect(resolveTrustedListenOnlineArtistLabel(track)).toBe("");
  });

  test("trusts linked API artists", () => {
    const track = item({
      channel: "Resolved Artist",
      artistBrowseId: "UCresolved",
      artistSource: "api-linked",
    });

    expect(hasTrustedListenOnlineArtist(track)).toBe(true);
    expect(resolveTrustedListenOnlineArtistLabel(track)).toBe("Resolved Artist");
  });

  test("trusts linked multi-artist labels", () => {
    const track = item({
      channel: "Artist A, Artist B",
      artistBrowseId: "UCfirst",
      artistSource: "api-linked-multiple",
    });

    expect(hasTrustedListenOnlineArtist(track)).toBe(true);
    expect(resolveTrustedListenOnlineArtistLabel(track)).toBe("Artist A, Artist B");
  });

  test("trusts artists resolved by backend track metadata", () => {
    const track = item({
      channel: "Accusefive",
      artistSource: "api-metadata",
    });

    expect(hasTrustedListenOnlineArtist(track)).toBe(true);
    expect(resolveTrustedListenOnlineArtistLabel(track)).toBe("Accusefive");
  });

  test("splits multi-artist labels without losing separators", () => {
    const parts = splitListenArtistLabel("Artist A、Artist B feat. Artist C");

    expect(parts).toEqual([
      { kind: "artist", text: "Artist A" },
      { kind: "separator", text: "、" },
      { kind: "artist", text: "Artist B" },
      { kind: "separator", text: " feat. " },
      { kind: "artist", text: "Artist C" },
    ]);
    expect(listenArtistCountFromLabelParts(parts)).toBe(3);
  });
});

describe("listen lyrics request cache", () => {
  test("hides stale lyrics synchronously when the active track changes", () => {
    const previous = {
      loading: false,
      data: {
        videoId: "previous-track",
        kind: "plain" as const,
        source: "test",
        text: "Previous lyrics",
        lines: [],
      },
      error: "previous error",
    };

    expect(
      resolveListenLyricsCurrentState(previous, "previous-track", "next-track"),
    ).toEqual({ loading: true, data: null, error: "" });
    expect(
      resolveListenLyricsCurrentState(previous, "previous-track", "previous-track"),
    ).toBe(previous);
  });

  test("reuses plain lyrics and deduplicates concurrent requests", async () => {
    const originalFetch = globalThis.fetch;
    const lyricsID = "CACHEVID07G";
    forgetListenLyricsCache(lyricsID, "zh-CN", { synced: true });
    let fetchCalls = 0;
    let releaseFetch: (() => void) | null = null;
    globalThis.fetch = (() => {
      fetchCalls += 1;
      return new Promise<Response>((resolve) => {
        releaseFetch = () =>
          resolve(
            new Response(
              JSON.stringify({
                videoId: lyricsID,
                kind: "plain",
                source: "test",
                text: "cached lyrics",
                lines: [{ startMs: 0, durationMs: 0, text: "cached lyrics" }],
              }),
              { status: 200, headers: { "Content-Type": "application/json" } },
            ),
          );
      });
    }) as typeof fetch;

    try {
      const request = {
        videoId: lyricsID,
        title: "Cache Track",
        artist: "Cache Artist",
      };
      const first = fetchListenLyricsCached(
        "http://127.0.0.1:34115",
        request,
        200,
        "zh-CN",
        { synced: true },
      );
      const second = fetchListenLyricsCached(
        "http://127.0.0.1:34115",
        request,
        200,
        "zh-CN",
        { synced: true },
      );
      await Promise.resolve();
      expect(fetchCalls).toBe(1);
      releaseFetch?.();
      const [firstResult, secondResult] = await Promise.all([first, second]);
      expect(firstResult.text).toBe("cached lyrics");
      expect(secondResult.text).toBe("cached lyrics");

      const third = await fetchListenLyricsCached(
        "http://127.0.0.1:34115",
        request,
        200,
        "zh-CN",
        { synced: true },
      );
      expect(third.text).toBe("cached lyrics");
      expect(fetchCalls).toBe(1);
      expect(readListenLyricsCache(lyricsID, "zh-CN", { synced: true })?.kind)
        .toBe("plain");
    } finally {
      globalThis.fetch = originalFetch;
      forgetListenLyricsCache(lyricsID, "zh-CN", { synced: true });
    }
  });

  test("caches negative results through an injected loader", async () => {
    const lyricsID = "MISSING007G";
    forgetListenLyricsCache(lyricsID, "en", { synced: true });
    let loads = 0;
    const load = () => {
      loads += 1;
      return Promise.resolve({
        videoId: lyricsID,
        kind: "unavailable" as const,
        source: "",
        text: "",
        lines: [],
      });
    };

    try {
      const first = await loadListenLyricsCached({
        cacheID: lyricsID,
        language: "en",
        synced: true,
        requestKey: "wails-test",
        loader: load,
      });
      const second = await loadListenLyricsCached({
        cacheID: lyricsID,
        language: "en",
        synced: true,
        requestKey: "wails-test",
        loader: load,
      });
      expect(first.kind).toBe("unavailable");
      expect(second.kind).toBe("unavailable");
      expect(loads).toBe(1);
    } finally {
      forgetListenLyricsCache(lyricsID, "en", { synced: true });
    }
  });

  test("clears every language and sync variant for a lyrics version", async () => {
    const lyricsID = "LYRICS-CACHE-VARIANTS";
    forgetListenLyricsCacheVariants(lyricsID);
    const load = (text: string) => Promise.resolve({
      videoId: lyricsID,
      kind: "plain" as const,
      source: "test",
      text,
      lines: [],
    });

    try {
      await loadListenLyricsCached({
        cacheID: lyricsID,
        language: "en",
        synced: true,
        requestKey: "synced-en",
        loader: () => load("synced"),
      });
      await loadListenLyricsCached({
        cacheID: lyricsID,
        language: "zh-CN",
        synced: false,
        requestKey: "plain-zh",
        loader: () => load("plain"),
      });

      forgetListenLyricsCacheVariants(lyricsID);

      expect(readListenLyricsCache(lyricsID, "en", { synced: true })).toBeNull();
      expect(
        readListenLyricsCache(lyricsID, "zh-CN", { synced: false }),
      ).toBeNull();
    } finally {
      forgetListenLyricsCacheVariants(lyricsID);
    }
  });

  test("does not reuse or persist an in-flight request after invalidation", async () => {
    const lyricsID = "LYRICS-PENDING-INVALIDATION";
    forgetListenLyricsCacheVariants(lyricsID);
    let loads = 0;
    let releaseOld = () => {};

    try {
      const oldRequest = loadListenLyricsCached({
        cacheID: lyricsID,
        language: "en",
        synced: true,
        requestKey: "same-request",
        loader: () => {
          loads += 1;
          return new Promise((resolve) => {
            releaseOld = () => resolve({
              videoId: lyricsID,
              kind: "plain" as const,
              source: "automatic",
              text: "old",
              lines: [],
            });
          });
        },
      });
      await Promise.resolve();
      expect(loads).toBe(1);

      forgetListenLyricsCacheVariants(lyricsID);
      const fresh = await loadListenLyricsCached({
        cacheID: lyricsID,
        language: "en",
        synced: true,
        requestKey: "same-request",
        loader: () => {
          loads += 1;
          return Promise.resolve({
            videoId: lyricsID,
            kind: "plain" as const,
            source: "lrclib",
            text: "fresh",
            lines: [],
          });
        },
      });
      expect(loads).toBe(2);
      expect(fresh.text).toBe("fresh");

      releaseOld();
      expect((await oldRequest).text).toBe("old");
      expect(
        readListenLyricsCache(lyricsID, "en", { synced: true })?.text,
      ).toBe("fresh");
    } finally {
      releaseOld();
      forgetListenLyricsCacheVariants(lyricsID);
    }
  });
});
