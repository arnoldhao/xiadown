import { describe, expect, test } from "bun:test";

import { normalizeTrayNowPlayingStatus } from "./tray-now-playing";

describe("tray now-playing payload", () => {
  test("preserves global provider capabilities and artwork fallbacks", () => {
    const status = normalizeTrayNowPlayingStatus({
      state: "playing",
      live: false,
      mediaId: "youtube-video",
      title: "Video title",
      subtitle: "Channel",
      artists: [{ name: "Channel", browseId: "channel-id" }],
      artworkURL: "https://img.example/main.jpg",
      artworkCandidates: [
        "https://img.example/fallback.jpg",
        42,
        "",
      ],
      playbackSource: "youtube",
      playbackSourceLabel: "YouTube",
      mode: "muse",
      canControl: true,
      canPrevious: false,
      canNext: true,
      progress: { currentTime: 12, duration: 120, bufferedTime: 40 },
      muted: false,
      volume: 0.6,
      sourceURL: "https://youtube.com/watch?v=youtube-video",
    });

    expect(status).toEqual({
      state: "playing",
      live: false,
      mediaId: "youtube-video",
      title: "Video title",
      subtitle: "Channel",
      artists: [{
        name: "Channel",
        browseId: "channel-id",
        thumbnailUrl: undefined,
      }],
      artworkURL: "https://img.example/main.jpg",
      artworkCandidates: ["https://img.example/fallback.jpg"],
      playbackSource: "youtube",
      playbackSourceLabel: "YouTube",
      mode: "muse",
      canControl: true,
      canPrevious: false,
      canNext: true,
      progress: { currentTime: 12, duration: 120, bufferedTime: 40 },
      muted: false,
      volume: 0.6,
      sourceURL: "https://youtube.com/watch?v=youtube-video",
      favoriteActive: undefined,
      canFavorite: undefined,
    });
  });

  test("rejects invalid payloads and unsupported playback sources", () => {
    expect(normalizeTrayNowPlayingStatus(null)).toBeNull();
    expect(normalizeTrayNowPlayingStatus({ state: "stopped" })).toBeNull();
    expect(
      normalizeTrayNowPlayingStatus({
        state: "paused",
        mode: "muse",
        playbackSource: "future-provider",
        progress: {},
      })?.playbackSource,
    ).toBeUndefined();
  });
});
