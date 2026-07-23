import { describe, expect, test } from "bun:test";

import type {
  PlaybackCapabilities,
  PlaybackProvider,
  PlaybackSessionSnapshot,
} from "@/shared/playback";
import type { ListenNowPlayingStatus } from "@/app/main/listen/types";
import {
  projectCoordinatorPlaybackStatus,
  recoverYouTubeWorkspacePlayback,
  resolveGlobalPlaybackCommandRoute,
  resolveListenFallbackPlaybackCommand,
  shouldPresentMusicWorkspaceTransport,
  shouldShowWorkspacePlaybackActivity,
} from "./workspace-playback";

const capabilities: PlaybackCapabilities = {
  available: true,
  mediaKinds: ["audio"],
  playPause: true,
  stop: true,
  seek: true,
  previous: false,
  next: false,
  volume: true,
  queue: false,
  shuffle: false,
  repeat: false,
  lyrics: false,
  video: false,
  like: false,
  dislike: false,
  captions: false,
  audioTracks: false,
  quality: false,
  fullscreen: true,
};

function makeSession(
  provider: PlaybackProvider,
  options: {
    focus?: PlaybackSessionSnapshot["focus"];
    id?: string;
    title?: string;
    artist?: string;
    artworkUrl?: string;
    live?: boolean;
    position?: number;
    duration?: number;
    volume?: number;
    muted?: boolean;
  } = {},
): PlaybackSessionSnapshot {
  const mediaID = options.id ?? "current-media";
  return {
    id: `session:${mediaID}`,
    focus: options.focus ?? "persistent",
    state: "playing",
    item: {
      id: mediaID,
      kind: provider === "youtube" ? "video" : "audio",
      source: { provider, id: mediaID, live: options.live },
      title: options.title ?? "Current title",
      artist: options.artist ?? "Current artist",
      artworkUrl: options.artworkUrl,
      duration: options.duration,
    },
    capabilities,
    position: options.position ?? 12,
    duration: options.duration ?? 120,
    volume: options.volume ?? 0.4,
    muted: options.muted ?? false,
    queue: [],
    currentIndex: 0,
    shuffleEnabled: false,
    repeatMode: "off",
  };
}

function makeListenStatus(
  overrides: Partial<ListenNowPlayingStatus> = {},
): ListenNowPlayingStatus {
  return {
    state: "playing",
    live: false,
    mediaId: "current-media",
    title: "Current title",
    subtitle: "Current artist",
    artworkURL: "https://catalog.example/current.jpg",
    artworkCandidates: ["https://catalog.example/fallback.jpg"],
    playbackSource: "youtube_music",
    playbackSourceLabel: "YouTube Music",
    mode: "muse",
    canControl: true,
    canPrevious: true,
    canNext: true,
    progress: { currentTime: 99, duration: 999, bufferedTime: 500 },
    muted: true,
    volume: 0.9,
    sourceURL: "https://music.youtube.com/watch?v=current-media",
    favoriteActive: true,
    canFavorite: true,
    ...overrides,
  };
}

describe("workspace playback ownership", () => {
  test("wires the Music floating transport to coordinator-owned status and commands", async () => {
    const source = await Bun.file(
      new URL("./MainApp.tsx", import.meta.url),
    ).text();

    expect(source).toContain("status={globalPlaybackStatus}");
    expect(source).toContain(
      "onCommand={sendMusicWorkspaceTransportCommand}",
    );
    expect(source).toMatch(
      /const sendMusicWorkspaceTransportCommand[\s\S]*?sendGlobalPlaybackCommand\(command\);/,
    );
  });

  test("shows Music transport only for persistent Music providers", () => {
    expect(shouldPresentMusicWorkspaceTransport(null)).toBe(true);
    for (const provider of ["youtube_music", "stream", "local"] as const) {
      expect(
        shouldPresentMusicWorkspaceTransport({
          focus: "persistent",
          item: { source: { provider } },
        }),
      ).toBe(true);
    }
    expect(
      shouldPresentMusicWorkspaceTransport({
        focus: "persistent",
        item: { source: { provider: "youtube" } },
      }),
    ).toBe(false);
    expect(
      shouldPresentMusicWorkspaceTransport({
        focus: "transient_preview",
        item: { source: { provider: "local" } },
      }),
    ).toBe(false);
  });

  test("hides YouTube playback only on its open Watch surface", () => {
    expect(shouldShowWorkspacePlaybackActivity("youtube_music", "music", "online")).toBe(false);
    expect(shouldShowWorkspacePlaybackActivity("radio", "music", "online")).toBe(false);
    expect(shouldShowWorkspacePlaybackActivity("local", "music", "online")).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("local", "music", "local")).toBe(false);
    expect(shouldShowWorkspacePlaybackActivity("youtube_music", "music", "local")).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("youtube", "youtube", "online")).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("youtube", "youtube", "online", true)).toBe(false);
    expect(shouldShowWorkspacePlaybackActivity("youtube", "youtube", "online", false)).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("youtube", "music", "online", true)).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("youtube", "music", "online")).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("youtube", "sniff", "online")).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("youtube", "default", "local")).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("radio", "youtube", "online")).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("radio", "sniff", "online")).toBe(true);
    expect(shouldShowWorkspacePlaybackActivity("local", "default", "local")).toBe(true);
  });

  test("does not contaminate a transient local preview with stale radio state", () => {
    const result = projectCoordinatorPlaybackStatus(
      makeSession("local", {
        focus: "transient_preview",
        id: "preview-file",
        title: "Preview file",
        artist: "Local artist",
        artworkUrl: "file:///preview-cover.jpg",
        position: 37,
        duration: 180,
        volume: 0.25,
      }),
      makeListenStatus({
        mediaId: "old-radio",
        playbackSource: "radio",
        live: true,
        artworkURL: "https://radio.example/old.jpg",
      }),
    );

    expect(result).toMatchObject({
      live: false,
      playbackSource: "library_preview",
      artworkURL: "file:///preview-cover.jpg",
      progress: { currentTime: 37, duration: 180 },
      volume: 0.25,
      muted: false,
    });
    expect(result?.playbackSourceLabel).toBeUndefined();
    expect(result?.favoriteActive).toBeUndefined();
  });

  test("merges Listen metadata only when provider and media identity match", () => {
    const session = makeSession("youtube_music", {
      artworkUrl: "https://coordinator.example/current.jpg",
      volume: 0.35,
    });
    const matching = projectCoordinatorPlaybackStatus(
      session,
      makeListenStatus({ canPrevious: false }),
    );

    expect(matching).toMatchObject({
      artworkURL: "https://catalog.example/current.jpg",
      artworkCandidates: [
        "https://catalog.example/current.jpg",
        "https://catalog.example/fallback.jpg",
        "https://coordinator.example/current.jpg",
      ],
      playbackSourceLabel: "YouTube Music",
      canPrevious: false,
      canNext: true,
      volume: 0.35,
      muted: false,
      favoriteActive: true,
    });

    const stale = projectCoordinatorPlaybackStatus(
      session,
      makeListenStatus({
        mediaId: "previous-media",
        sourceURL: "https://music.youtube.com/watch?v=previous-media",
      }),
    );
    expect(stale).toMatchObject({
      artworkURL: "https://coordinator.example/current.jpg",
      artworkCandidates: ["https://coordinator.example/current.jpg"],
      canPrevious: false,
      canNext: false,
      volume: 0.35,
      muted: false,
    });
    expect(stale?.playbackSourceLabel).toBeUndefined();
    expect(stale?.favoriteActive).toBeUndefined();
  });

  test("projects a live stream as the standard timeline with zero remaining", () => {
    const result = projectCoordinatorPlaybackStatus(
      makeSession("stream", { live: true, position: 42, duration: 900 }),
      makeListenStatus({
        playbackSource: "radio",
        live: true,
      }),
    );

    expect(result).toMatchObject({
      live: true,
      mode: "hush",
      progress: { currentTime: 42, duration: 42, bufferedTime: 42 },
    });
  });

  test("routes global commands to the active provider without losing idempotence", () => {
    expect(resolveGlobalPlaybackCommandRoute(null, "play")).toEqual({
      target: "listen",
      command: "play",
    });
    expect(
      resolveGlobalPlaybackCommandRoute(makeSession("youtube"), "previous"),
    ).toEqual({ target: "youtube-queue", command: "previous" });
    expect(
      resolveGlobalPlaybackCommandRoute(makeSession("local"), "next"),
    ).toEqual({ target: "listen", command: "next" });
    expect(
      resolveGlobalPlaybackCommandRoute(makeSession("stream"), "toggle"),
    ).toEqual({ target: "coordinator", command: "pause" });

    const pausedYouTube = {
      ...makeSession("youtube"),
      state: "paused" as const,
    };
    expect(resolveGlobalPlaybackCommandRoute(pausedYouTube, "play")).toEqual({
      target: "coordinator",
      command: "play",
    });
    expect(resolveGlobalPlaybackCommandRoute(pausedYouTube, "pause")).toEqual({
      target: "none",
    });
    const bufferingYouTube = {
      ...makeSession("youtube"),
      state: "buffering" as const,
    };
    expect(resolveGlobalPlaybackCommandRoute(bufferingYouTube, "toggle")).toEqual({
      target: "coordinator",
      command: "pause",
    });
    expect(resolveGlobalPlaybackCommandRoute(bufferingYouTube, "play")).toEqual({
      target: "none",
    });
    const loadingMusic = {
      ...makeSession("youtube_music"),
      state: "loading" as const,
    };
    expect(resolveGlobalPlaybackCommandRoute(loadingMusic, "toggle")).toEqual({
      target: "coordinator",
      command: "pause",
    });
    expect(resolveGlobalPlaybackCommandRoute(loadingMusic, "play")).toEqual({
      target: "none",
    });

    const transientWithoutPrevious = makeSession("local", {
      focus: "transient_preview",
    });
    expect(
      resolveGlobalPlaybackCommandRoute(transientWithoutPrevious, "previous"),
    ).toEqual({ target: "none" });
    expect(
      resolveGlobalPlaybackCommandRoute(
        {
          ...transientWithoutPrevious,
          capabilities: { ...capabilities, previous: true },
        },
        "previous",
      ),
    ).toEqual({ target: "coordinator", command: "previous" });
  });

  test("turns a pre-coordinator loading toggle into an explicit pause", () => {
    expect(
      resolveListenFallbackPlaybackCommand(
        { state: "loading" },
        "toggle",
      ),
    ).toBe("pause");
    expect(
      resolveListenFallbackPlaybackCommand({ state: "paused" }, "toggle"),
    ).toBe("toggle");
    expect(resolveListenFallbackPlaybackCommand(null, "toggle")).toBe(
      "toggle",
    );
  });

  test("recovers an active YouTube session after the renderer remounts", () => {
    const current = makeSession("youtube", {
      id: "AbCdEfGh123",
      title: "Recovered video",
      artist: "Recovered creator",
      artworkUrl: "https://example.test/recovered.jpg",
      position: 31,
      duration: 240,
      volume: 0.27,
      muted: true,
    });
    current.capabilities = {
      ...capabilities,
      mediaKinds: ["video"],
      next: true,
      like: true,
      captions: true,
      video: true,
      fullscreen: true,
    };
    current.item.canonicalUrl =
      "https://www.youtube.com/watch?v=AbCdEfGh123";
    current.item.metadata = {
      channelId: "UCabcdefghijklmnopqrstuv",
      viewCount: "4567",
      publishedLabel: "2 days ago",
    };

    expect(recoverYouTubeWorkspacePlayback(current)).toMatchObject({
      descriptor: {
        sessionId: "session:AbCdEfGh123",
        videoId: "AbCdEfGh123",
        title: "Recovered video",
        channelId: "UCabcdefghijklmnopqrstuv",
        viewCount: 4567,
        publishedLabel: "2 days ago",
      },
      status: {
        provider: "youtube",
        state: "playing",
        currentTime: 31,
        duration: 240,
        volume: 0.27,
        muted: true,
        controls: { like: true, captions: true },
      },
      queue: [
        {
          videoId: "AbCdEfGh123",
          channelId: "UCabcdefghijklmnopqrstuv",
          viewCount: 4567,
          publishedLabel: "2 days ago",
        },
      ],
      currentIndex: 0,
      volume: 0.27,
      muted: true,
      capabilities: { next: true, like: true, captions: true },
    });
    expect(recoverYouTubeWorkspacePlayback(makeSession("local"))).toBeNull();
  });
});
