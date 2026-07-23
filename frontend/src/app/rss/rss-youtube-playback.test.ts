import { describe, expect, test } from "bun:test";

import type { RSSEntry } from "./types";
import {
  rssEntryToYouTubeVideo,
  shouldAcceptRSSYouTubeProgress,
} from "./RSSYouTubePlayback";

function entry(overrides: Partial<RSSEntry> = {}): RSSEntry {
  return {
    id: "rss-entry-1",
    subscriptionId: "rss-subscription-1",
    externalId: "video-1",
    title: "A subscribed video",
    kind: "video",
    imageUrls: [],
    media: [],
    platform: "youtube",
    platformVideoId: "AbCdEfGhI12",
    url: "https://www.youtube.com/watch?v=AbCdEfGhI12",
    stateRevision: 0,
    revision: 1,
    createdAt: "2026-07-13T00:00:00Z",
    modifiedAt: "2026-07-13T00:00:00Z",
    ...overrides,
  };
}

describe("RSS YouTube playback adapter", () => {
  test("converts a YouTube RSS entry into the shared station player contract", () => {
    expect(rssEntryToYouTubeVideo(entry())).toEqual({
      itemKind: "video",
      videoId: "AbCdEfGhI12",
      title: "A subscribed video",
      channel: undefined,
      thumbnailUrl: undefined,
      webUrl: "https://www.youtube.com/watch?v=AbCdEfGhI12",
    });
  });

  test("does not route non-YouTube or malformed identifiers into the native player", () => {
    expect(rssEntryToYouTubeVideo(entry({
      platform: "bilibili",
      platformVideoId: "BV1xx411c7mD",
      url: "https://www.bilibili.com/video/BV1xx411c7mD",
    }))).toBeNull();
    expect(rssEntryToYouTubeVideo(entry({
      platformVideoId: "too-short",
      url: "https://www.youtube.com/watch?v=too-short",
    }))).toBeNull();
  });

  test("does not overwrite a saved resume point with the transient zero status", () => {
    const barrier = {
      sessionId: "youtube-session-1",
      resumeAt: 180,
      expiresAt: 20_000,
    };
    expect(shouldAcceptRSSYouTubeProgress(0, barrier, 10_000)).toBeFalse();
    expect(shouldAcceptRSSYouTubeProgress(179, barrier, 10_000)).toBeTrue();
    expect(shouldAcceptRSSYouTubeProgress(12, barrier, 20_000)).toBeTrue();
  });

  test("waits for accept and resume before exposing the shared native controls", async () => {
    const source = await Bun.file(
      new URL("./RSSYouTubePlayback.tsx", import.meta.url),
    ).text();
    expect(source.indexOf("await acceptYouTubeWorkspacePlay(requestID)")).toBeLessThan(
      source.indexOf("setControlsReady(true)"),
    );
    expect(source.indexOf("await seekYouTubeWorkspaceVideo(descriptor.sessionId, resumeAt)")).toBeLessThan(
      source.indexOf("setControlsReady(true)"),
    );
    expect(source).toContain("if (!accepted || !descriptorSessionID)");
    expect(source).toContain("cancelYouTubeWorkspacePlay(requestID)");
  });

  test("keeps RSS posters on the controlled loopback resource surface", async () => {
    const source = await Bun.file(
      new URL("./RSSYouTubePlayback.tsx", import.meta.url),
    ).text();
    expect(source).toContain("allowRemotePosterCandidates={false}");
    expect(source).toContain("controlledRSSResourceURL(status.thumbnailUrl)");
    expect(source).toContain("controlledRSSResourceURL(entry.thumbnailUrl)");
  });
});
