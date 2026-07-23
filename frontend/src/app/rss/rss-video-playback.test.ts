import { describe, expect, test } from "bun:test";

import type {
  RSSBilibiliPlaybackDescriptor,
  RSSBilibiliPlayerStatus,
} from "./api";
import { RSSBilibiliPrepareLifecycle } from "./bilibili-prepare-lifecycle";
import {
  createBilibiliTransportPlayback,
  createRSSWebVideoTransportPlayback,
  isRSSBilibiliNativeReady,
  isRSSBilibiliVideoStatusForSession,
} from "./video-transport";

const descriptor: RSSBilibiliPlaybackDescriptor = {
  platform: "bilibili",
  adapter: "video",
  platformVideoId: "BV1xx411c7mD",
  playerUrl: "https://www.bilibili.com/video/BV1xx411c7mD/",
  authenticated: true,
  sessionId: "bilibili-session-1",
};

const status: RSSBilibiliPlayerStatus = {
  provider: "bilibili",
  sessionId: descriptor.sessionId,
  available: true,
  platformVideoId: descriptor.platformVideoId,
  state: "playing",
  currentTime: 25,
  duration: 100,
  bufferedTime: 60,
  volume: 0.8,
  muted: false,
  playbackRate: 1.25,
  fullscreen: false,
  controls: {
    playPause: true,
    seek: true,
    volume: true,
    playbackRate: true,
    fullscreen: true,
    captions: true,
    quality: true,
    danmaku: true,
  },
  captionOptions: [{ id: "zh-CN", label: "Chinese" }],
  qualityOptions: [{ id: "80", label: "1080P" }],
  playbackRateOptions: [
    { id: "1", label: "1×" },
    { id: "1.25", label: "1.25×" },
  ],
  selections: {
    playbackRateId: "1.25",
    captionId: "zh-CN",
    qualityId: "80",
  },
  danmakuEnabled: true,
};

describe("RSS video transport adapters", () => {
  test("shares the YouTube Primary material across the RSS title and video content", async () => {
    const [
      rssSource,
      youtubeSource,
      rssYouTubePlayback,
      rssBilibiliPlayback,
      rssSitePlayback,
      rssWebPlayback,
      workspaceCSS,
      rssDreamCSS,
      youtubeDreamCSS,
      layoutContractCSS,
      workspaceAppearanceCSS,
    ] =
      await Promise.all([
        Bun.file(new URL("./RSSWorkspacePage.tsx", import.meta.url)).text(),
        Bun.file(
          new URL("../youtube/YouTubeWorkspacePage.tsx", import.meta.url),
        ).text(),
        Bun.file(new URL("./RSSYouTubePlayback.tsx", import.meta.url)).text(),
        Bun.file(new URL("./RSSBilibiliPlayback.tsx", import.meta.url)).text(),
        Bun.file(new URL("./RSSSiteVideoPlayback.tsx", import.meta.url)).text(),
        Bun.file(new URL("./RSSWebVideoPlayback.tsx", import.meta.url)).text(),
        Bun.file(new URL("./rss-workspace.css", import.meta.url)).text(),
        Bun.file(
          new URL("../../shared/styles/dream/rss.css", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../shared/styles/dream/youtube.css", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../shared/styles/dream/layout-contract.css", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../shared/styles/dream/workspace.css", import.meta.url),
        ).text(),
      ]);
    const rssWatch = rssSource.slice(
      rssSource.indexOf("function RSSVideoDetail"),
      rssSource.indexOf("function RSSVideoMoreMenu"),
    );
    const youtubeWatch = youtubeSource.slice(
      youtubeSource.indexOf("export function YouTubePrimaryWatchPage"),
      youtubeSource.indexOf("function YouTubeWorkspacePlayer"),
    );

    for (const source of [rssWatch, youtubeWatch]) {
      expect(source).toContain("youtube-workspace-watch-page");
      expect(source).toContain("youtube-workspace-watch-header");
    }
    expect(rssSource).toContain(
      'focusedVideoPresentation\n            && "youtube-workspace-page app-workspace-primary-subpane"',
    );
    expect(rssWatch).toContain(
      'className="youtube-workspace-watch-page rss-video-watch-page"',
    );
    expect(rssWatch).not.toContain(
      'className="youtube-workspace-page youtube-workspace-watch-page',
    );
    for (const source of [
      rssYouTubePlayback,
      rssBilibiliPlayback,
      rssSitePlayback,
      rssWebPlayback,
    ]) {
      expect(source).toContain("youtube-workspace-watch-video-region");
    }
    expect(rssWatch).not.toContain("WorkspacePageTopBarMaterial");
    expect(rssWatch).not.toContain("data-page-scroll-material-state");
    expect(workspaceCSS).not.toContain(
      ".rss-video-watch-header > :not(.app-workspace-page__topbar-material)",
    );
    expect(workspaceCSS).not.toMatch(
      /\.rss-video-watch-header\s*\{[^}]*isolation:\s*isolate/s,
    );
    expect(youtubeDreamCSS).toMatch(
      /\.youtube-workspace-watch-page\s*\{[^}]*background:\s*var\(--app-workspace-primary-subpane-surface\);/s,
    );
    expect(youtubeDreamCSS).not.toContain("--listen-native-video-surface:");
    expect(rssDreamCSS).not.toContain("--listen-native-video-surface:");
    expect(workspaceAppearanceCSS).toMatch(
      /:root\[data-listen-native-video-underlay="true"\]:is\([\s\S]*?\[data-youtube-workspace-video-active="true"\],[\s\S]*?\[data-rss-bilibili-video-active="true"\],[\s\S]*?\[data-rss-site-video-active="true"\][\s\S]*?\)\s*\.app-workspace-primary-pane::before\s*\{[^}]*background:\s*var\(--app-workspace-primary-surface\)[^}]*mask:\s*var\(--listen-native-video-primary-outside-mask\)/s,
    );
    expect(layoutContractCSS).toMatch(
      /\.app-workspace-primary-subpane\.app-dream-window\s*\{[^}]*background:\s*var\(--app-workspace-primary-subpane-surface\)/s,
    );
    expect(rssDreamCSS).not.toMatch(
      /\.rss-video-watch-page\s*\{[^}]*background:/s,
    );
  });

  test("settles only the newest rapid Bilibili Prepare generation", () => {
    let requestId = 100;
    const lifecycle = new RSSBilibiliPrepareLifecycle(() => ++requestId);
    const older = lifecycle.begin();
    const newer = lifecycle.begin();

    expect(lifecycle.isCurrent(older)).toBeFalse();
    expect(lifecycle.isCurrent(newer)).toBeTrue();
    expect(lifecycle.settle(newer)).toEqual({ pending: true, current: true });
    expect(lifecycle.settle(older)).toEqual({ pending: true, current: false });
    expect(lifecycle.cancel(older)).toBeFalse();
  });

  test("cleanup cancels an unresolved Prepare exactly once", () => {
    const lifecycle = new RSSBilibiliPrepareLifecycle(() => 201);
    const pending = lifecycle.begin();

    expect(lifecycle.cancel(pending)).toBeTrue();
    expect(lifecycle.cancel(pending)).toBeFalse();
    expect(lifecycle.settle(pending)).toEqual({ pending: false, current: false });
  });

  test("does not expose Bilibili controls before AcceptPrepare commits", async () => {
    const source = await Bun.file(
      new URL("./RSSBilibiliPlayback.tsx", import.meta.url),
    ).text();
    expect(source.indexOf("await acceptRSSBilibiliVideoPrepare(prepareToken.requestId)")).toBeLessThan(
      source.indexOf("setDescriptor(nextDescriptor)"),
    );
    expect(source).toContain(
      "await cancelRSSBilibiliVideoPrepare(prepareToken.requestId).catch(() => {})",
    );
    expect(source).not.toContain(
      "void acceptRSSBilibiliVideoPrepare(prepareToken.requestId).catch",
    );
  });

  test("accepts only Bilibili status events for the active native session", () => {
    expect(isRSSBilibiliVideoStatusForSession(status, descriptor.sessionId)).toBeTrue();
    expect(isRSSBilibiliVideoStatusForSession(status, "stale-session")).toBeFalse();
    expect(isRSSBilibiliVideoStatusForSession(
      { ...status, provider: "bilibili", sessionId: "" },
      descriptor.sessionId,
    )).toBeFalse();
  });

  test("reveals the native Bilibili surface only after the current session reports media controls", () => {
    expect(isRSSBilibiliNativeReady(descriptor, status)).toBeTrue();
    expect(isRSSBilibiliNativeReady(null, status)).toBeFalse();
    expect(isRSSBilibiliNativeReady(
      descriptor,
      { ...status, sessionId: "stale-session" },
    )).toBeFalse();
    expect(isRSSBilibiliNativeReady(
      descriptor,
      { ...status, available: false },
    )).toBeFalse();
    expect(isRSSBilibiliNativeReady(
      descriptor,
      { ...status, controls: { ...status.controls, playPause: false } },
    )).toBeFalse();
  });

  test("maps native Bilibili media state into the shared YouTube-style transport", () => {
    const playback = createBilibiliTransportPlayback(
      descriptor,
      status,
      { title: "Subscribed Bilibili video", videoDurationSeconds: 90 },
    );

    expect(playback.status).toMatchObject({
      state: "playing",
      currentTime: 25,
      duration: 100,
      selections: { playbackRateId: "1.25" },
    });
    expect(playback.capabilities).toEqual({
      playPause: true,
      seek: true,
      fullscreen: true,
      captions: true,
      audioTrack: false,
      quality: true,
      danmaku: true,
      volume: true,
      playbackRate: true,
    });
    expect(playback.volume).toBe(0.8);
  });

  test("keeps a complete disabled transport model while Prepare is loading or stale", () => {
    const unavailableStatus: RSSBilibiliPlayerStatus = {
      ...status,
      sessionId: "",
      available: false,
      state: "error",
      duration: 0,
      controls: {
        playPause: false,
        seek: false,
        volume: false,
        playbackRate: false,
        fullscreen: false,
        captions: false,
        quality: false,
        danmaku: false,
      },
      playbackRateOptions: [],
    };
    const expectedCapabilities = {
      playPause: false,
      seek: false,
      fullscreen: false,
      captions: false,
      audioTrack: false,
      quality: false,
      danmaku: false,
      volume: false,
      playbackRate: false,
    };

    const loadingPlayback = createBilibiliTransportPlayback(
      null,
      unavailableStatus,
      { title: "Subscribed Bilibili video", videoDurationSeconds: 90 },
    );
    const stalePlayback = createBilibiliTransportPlayback(
      descriptor,
      { ...status, sessionId: "stale-session" },
      { title: "Subscribed Bilibili video", videoDurationSeconds: 90 },
    );

    expect(loadingPlayback.capabilities).toEqual(expectedCapabilities);
    expect(loadingPlayback.descriptor.durationSeconds).toBe(90);
    expect(stalePlayback.capabilities).toEqual(expectedCapabilities);
  });

  test("enables HTMLMediaElement controls for direct media", () => {
    const playback = createRSSWebVideoTransportPlayback({
      direct: true,
      title: "Direct video",
      state: "paused",
      currentTime: 20,
      duration: 120,
      volume: 0.5,
      muted: false,
      playbackRate: 1,
      fullscreenAvailable: true,
    });

    expect(playback.capabilities).toMatchObject({
      playPause: true,
      seek: true,
      volume: true,
      playbackRate: true,
      fullscreen: true,
    });
    expect(playback.status.playbackRateOptions).toHaveLength(6);
  });

  test("keeps cross-origin embeds on the same footer while disabling unsupported controls", () => {
    const playback = createRSSWebVideoTransportPlayback({
      direct: false,
      title: "Embedded video",
      state: "paused",
      currentTime: 0,
      duration: 0,
      volume: 1,
      muted: false,
      playbackRate: 1,
      fullscreenAvailable: false,
    });

    expect(playback.capabilities).toEqual({
      playPause: false,
      seek: false,
      fullscreen: false,
      captions: false,
      audioTrack: false,
      quality: false,
      danmaku: false,
      volume: false,
      playbackRate: false,
    });
    expect(playback.status.playbackRateOptions).toEqual([]);
  });

  test("routes every Bilibili footer command through the session-scoped Wails API", async () => {
    const [playbackSource, apiSource, webSource, contractSource] = await Promise.all([
      Bun.file(new URL("./RSSBilibiliPlayback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./api.ts", import.meta.url)).text(),
      Bun.file(new URL("./RSSWebVideoPlayback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./video-transport.ts", import.meta.url)).text(),
    ]);

    for (const command of [
      "playRSSBilibiliVideo(sessionID)",
      "pauseRSSBilibiliVideo(sessionID)",
      "seekRSSBilibiliVideo(sessionID, currentTime)",
      "setRSSBilibiliVideoVolume(sessionID",
      "setRSSBilibiliVideoPlaybackRate(sessionID, rate)",
      "toggleRSSBilibiliVideoCaptions(sessionID)",
      "selectRSSBilibiliVideoCaption(sessionID, value)",
      "selectRSSBilibiliVideoQuality(sessionID, value)",
      "toggleRSSBilibiliVideoDanmaku(sessionID)",
    ]) {
      expect(playbackSource).toContain(command);
    }
    expect(playbackSource).toContain("const operationSessionID = sessionID;");
    expect(playbackSource).toContain(
      "current.sessionId !== operationSessionID",
    );
    expect(playbackSource).toContain("!operationSessionID ||");
    expect(playbackSource).toContain("closeRSSBilibiliVideo(sessionID)");
    expect(playbackSource).toContain("cancelRSSBilibiliVideoPrepare(prepareToken.requestId)");
    expect(playbackSource).toContain("acceptRSSBilibiliVideoPrepare(prepareToken.requestId)");
    expect(playbackSource).not.toContain('closeRSSBilibiliVideo("")');
    expect(apiSource).toContain('Events.On("rss:bilibili-video-player"');
    for (const method of [
      ".ToggleCaptions`",
      ".SelectCaption`",
      ".SelectQuality`",
      ".ToggleDanmaku`",
    ]) {
      expect(apiSource).toContain(method);
    }
    expect(contractSource).toContain("status.sessionId?.trim() === expectedSessionID");
    expect(contractSource).toContain("isRSSBilibiliNativeReady");
    expect(playbackSource).toContain("const playback = createBilibiliTransportPlayback(");
    expect(playbackSource).toContain("<YouTubeWorkspaceTransportBar");
    expect(playbackSource).not.toContain("{playback ? (");
    expect(playbackSource).toContain(
      'data-native-ready={nativeReady ? "true" : "false"}',
    );
    expect(playbackSource).toContain('loading={status.state !== "error"}');
    expect(webSource).toContain("player.playbackRate = rate");
    expect(webSource).toContain("player.muted = !player.muted");
    expect(webSource).not.toContain("<video\n                  controls");
  });
});
