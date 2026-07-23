import { describe, expect, test } from "bun:test";

import type { RSSEntry } from "./types";
import {
  buildRSSVideoBatchDownloadTarget,
  buildRSSVideoBatchDownloadTargets,
  canonicalRSSVideoTarget,
  isSafePublicSitePageURL,
  normalizedBilibiliBangumiID,
  normalizedBilibiliVideoID,
  resolveRSSBilibiliPlaybackIdentity,
  resolveRSSVideoExperience,
  shouldUseRSSVideoCollectionPresentation,
  shouldUseRSSVideoLayoutPresentation,
  shouldUseRSSVideoPresentation,
  siteKeyForRSSVideo,
} from "./video-platform";

const base: RSSEntry = {
  id: "entry",
  subscriptionId: "subscription",
  externalId: "external",
  title: "Video",
  kind: "video",
  imageUrls: [],
  media: [],
  stateRevision: 0,
  revision: 1,
  createdAt: "2026-07-13T00:00:00Z",
  modifiedAt: "2026-07-13T00:00:00Z",
};
const controlledMediaURL = `http://127.0.0.1:43127/_xiadown/${"a".repeat(64)}/api/rss/entries/entry/resources/media-0`;

describe("RSS video platform adapters", () => {
  test("routes YouTube through the shared native station player", () => {
    expect(resolveRSSVideoExperience({
      ...base,
      platform: "youtube",
      platformVideoId: "AbCdEfGhI12",
      url: "https://youtube.com/watch?v=AbCdEfGhI12",
    }).mode).toBe("youtube-native");
  });

  test("uses the full Bilibili video page while App Session stays a credential provider", () => {
    expect(resolveRSSVideoExperience({
      ...base,
      platform: "bilibili",
      platformVideoId: "BV1xx411c7mD",
      playbackUrl: "https://www.bilibili.com/video/BV1xx411c7mD/",
      url: "https://www.bilibili.com/video/BV1xx411c7mD/",
    })).toMatchObject({
      mode: "bilibili-native",
      siteKey: "bilibili",
      appSessionPreferred: true,
      playbackUrl: "https://www.bilibili.com/video/BV1xx411c7mD/",
      targetUrl: "https://www.bilibili.com/video/BV1xx411c7mD/",
    });
  });

  test("canonicalizes numeric Bilibili ids to full-site pages, never embed destinations", () => {
    expect(normalizedBilibiliVideoID({
      ...base,
      platformVideoId: "AV000170001",
      url: "https://www.bilibili.com/video/av170001/",
    })).toBe("av170001");
    expect(resolveRSSVideoExperience({
      ...base,
      platform: "bilibili",
      platformVideoId: "170001",
      url: "https://www.bilibili.com/video/av170001/",
    })).toMatchObject({
      mode: "bilibili-native",
      playbackUrl: "https://www.bilibili.com/video/av170001/",
      targetUrl: "https://www.bilibili.com/video/av170001/",
      appSessionPreferred: true,
    });
  });

  test("recognizes session-backed social video hosts without duplicating UI mappings", () => {
    expect(siteKeyForRSSVideo("https://www.youtube.com/watch?v=AbCdEfGhI12")).toBe("youtube");
    expect(siteKeyForRSSVideo("https://www.tiktok.com/@creator/video/1")).toBe("tiktok");
    expect(siteKeyForRSSVideo("https://vm.tiktok.com/example")).toBe("tiktok");
    expect(siteKeyForRSSVideo("https://b23.tv/example")).toBe("bilibili");
    expect(siteKeyForRSSVideo("https://www.douyin.com/video/1")).toBe("douyin");
    expect(siteKeyForRSSVideo("https://www.xiaohongshu.com/explore/64dccf7d000000000100577e")).toBe("xiaohongshu");
    expect(siteKeyForRSSVideo("https://xhslink.com/a/example")).toBe("xiaohongshu");
    expect(siteKeyForRSSVideo("https://www.rednote.com/explore/1")).toBe("china_private");
    expect(siteKeyForRSSVideo("https://www.instagram.com/reel/example")).toBe("instagram");
    expect(siteKeyForRSSVideo("https://x.com/creator/status/1")).toBe("x");
    expect(siteKeyForRSSVideo("https://fb.watch/example")).toBe("facebook");
    expect(siteKeyForRSSVideo("https://player.vimeo.com/video/1")).toBe("vimeo");
    expect(siteKeyForRSSVideo("https://clips.twitch.tv/example")).toBe("twitch");
    expect(siteKeyForRSSVideo("https://nico.ms/sm1")).toBe("niconico");
    expect(siteKeyForRSSVideo("https://evilbilibili.com/video/1")).toBe("");
  });

  test("uses the actual site page while App Session remains native-only", () => {
    expect(resolveRSSVideoExperience({
      ...base,
      url: "https://www.tiktok.com/@creator/video/1",
      media: [{
        kind: "video",
        mimeType: "video/mp4",
        url: "https://cdn.example/video.mp4",
      }],
    })).toMatchObject({
      mode: "site",
      siteKey: "tiktok",
      playbackUrl: "https://www.tiktok.com/@creator/video/1",
      appSessionPreferred: true,
    });
  });

  test("routes Xiaohongshu video notes through its independent App Session", () => {
    expect(resolveRSSVideoExperience({
      ...base,
      platform: "china_private",
      url: "https://www.xiaohongshu.com/explore/64dccf7d000000000100577e",
    })).toMatchObject({
      mode: "site",
      siteKey: "xiaohongshu",
      playbackUrl: "https://www.xiaohongshu.com/explore/64dccf7d000000000100577e",
      appSessionPreferred: true,
    });
  });

  test("routes Bilibili Bangumi through its native adapter", () => {
    for (const episode of [
      "https://www.bilibili.com/bangumi/play/ep123456",
      "https://www.bilibili.com/bangumi/play/ss98765",
    ]) {
      expect(resolveRSSVideoExperience({
        ...base,
        platform: "bilibili",
        platformVideoId: episode.includes("/ep") ? "ep123456" : "ss98765",
        url: episode,
      })).toMatchObject({
        mode: "bilibili-native",
        siteKey: "bilibili",
        bilibiliAdapter: "bangumi",
        playbackUrl: episode,
        targetUrl: episode,
        appSessionPreferred: true,
      });
    }
  });

  test("keeps the two Bilibili adapters identity-specific", () => {
    const bangumi = {
      ...base,
      platform: "bilibili",
      platformVideoId: "EP003854807",
      url: "https://www.bilibili.com/bangumi/play/ep3854807?from=rss",
    };
    expect(normalizedBilibiliVideoID(bangumi)).toBe("");
    expect(normalizedBilibiliBangumiID(bangumi)).toBe("ep3854807");
    expect(resolveRSSBilibiliPlaybackIdentity(bangumi)).toMatchObject({
      adapter: "bangumi",
      platformVideoId: "ep3854807",
    });
    expect(canonicalRSSVideoTarget(bangumi)).toBe(
      "https://www.bilibili.com/bangumi/play/ep3854807",
    );
  });

  test("allows only registry-approved public embeds", () => {
    expect(resolveRSSVideoExperience({
      ...base,
      platform: "vimeo",
      playbackUrl: "https://player.vimeo.com/video/123",
      url: "https://vimeo.com/123",
    }).mode).toBe("embed");

    expect(resolveRSSVideoExperience({
      ...base,
      platform: "vimeo",
      playbackUrl: "https://tracker.example/video/123",
      url: "https://vimeo.com/123",
    })).toMatchObject({
      mode: "site",
      siteKey: "vimeo",
      playbackUrl: "https://vimeo.com/123",
    });

    for (const playbackUrl of [
      "https://user:secret@player.vimeo.com/video/123",
      "https://player.vimeo.com:8443/video/123",
      "https://player.vimeo.com/not-an-embed",
      "https://player.vimeo.com/video/0",
    ]) {
      expect(resolveRSSVideoExperience({
        ...base,
        platform: "vimeo",
        playbackUrl,
        url: "https://vimeo.com/123",
      })).toMatchObject({
        mode: "site",
        playbackUrl: "https://vimeo.com/123",
      });
    }
  });

  test("keeps unknown public pages in the article reader", () => {
    expect(resolveRSSVideoExperience({
      ...base,
      url: "https://video.example/watch/42",
    }).mode).toBe("unavailable");
    expect(resolveRSSVideoExperience({
      ...base,
      platform: "bilibili",
      url: "https://video.example/watch/platform-cannot-grant-cookies",
    }).mode).toBe("unavailable");
    expect(shouldUseRSSVideoPresentation({
      ...base,
      url: "https://video.example/watch/42",
    })).toBeFalse();
  });

  test("does not mistake ordinary pages on supported hosts for videos", () => {
    for (const url of [
      "https://www.bilibili.com/read/cv123456",
      "https://www.tiktok.com/@creator",
      "https://www.instagram.com/p/Post_123/",
      "https://www.facebook.com/example",
      "https://www.twitch.tv/example",
      "https://www.nicovideo.jp/user/123",
      "https://x.com/creator/status/123",
      "https://www.xiaohongshu.com/explore/abc123",
    ]) {
      const entry = { ...base, url };
      expect(resolveRSSVideoExperience(entry).mode).toBe("unavailable");
      expect(shouldUseRSSVideoPresentation(entry)).toBeFalse();
    }
  });

  test("retains public URL validation independently of video evidence", () => {
    for (const rawURL of [
      "http://video.example/watch/42",
      "https://localhost/watch/42",
      "https://127.0.0.1/watch/42",
      "https://[::1]/watch/42",
      "https://user:secret@video.example/watch/42",
      "https://video.example:8443/watch/42",
    ]) {
      expect(isSafePublicSitePageURL(rawURL)).toBeFalse();
    }
  });

  test("uses video presentation only for playable evidence", () => {
    expect(shouldUseRSSVideoPresentation({
      ...base,
      url: "https://www.bilibili.com/bangumi/play/ep123456",
    })).toBeTrue();
    expect(shouldUseRSSVideoPresentation({
      ...base,
      kind: "article",
      platform: "youtube",
      platformVideoId: "AbCdEfGhI12",
      url: "https://www.youtube.com/watch?v=AbCdEfGhI12",
    })).toBeFalse();
    expect(shouldUseRSSVideoPresentation({
      ...base,
      media: [{ kind: "video", mimeType: "video/mp4", url: controlledMediaURL }],
    })).toBeTrue();
  });

  test("keeps mixed feeds in article layout even when an entry has playable media", () => {
    const embeddedYouTube = {
      ...base,
      url: "https://example.com/articles/with-an-embed",
      platform: "youtube",
      platformVideoId: "AbCdEfGhI12",
      media: [{
        kind: "video",
        mimeType: "text/html",
        url: "https://www.youtube-nocookie.com/embed/AbCdEfGhI12",
      }],
    };
    expect(shouldUseRSSVideoLayoutPresentation("article", embeddedYouTube)).toBeFalse();
    expect(shouldUseRSSVideoLayoutPresentation("all", embeddedYouTube)).toBeFalse();
    expect(shouldUseRSSVideoLayoutPresentation("video", embeddedYouTube)).toBeTrue();
    expect(shouldUseRSSVideoLayoutPresentation("all", {
      ...base,
      url: "https://unsupported.example/watch/42",
    })).toBeFalse();
    expect(shouldUseRSSVideoLayoutPresentation("video", {
      ...base,
      url: "https://unsupported.example/watch/42",
    })).toBeFalse();
  });

  test("uses the article list when a video collection contains unplayable rows", () => {
    const playable = {
      ...base,
      url: "https://vimeo.com/123456",
    };
    const staleVideo = {
      ...base,
      id: "stale-video",
      url: "https://example.com/ordinary-article",
    };
    expect(shouldUseRSSVideoCollectionPresentation([])).toBeTrue();
    expect(shouldUseRSSVideoCollectionPresentation([playable])).toBeTrue();
    expect(
      shouldUseRSSVideoCollectionPresentation([playable, staleVideo]),
    ).toBeFalse();
  });

  test("normalizes canonical video targets instead of downloading an enclosing article", () => {
    const bilibili = {
      ...base,
      platform: "bilibili",
      platformVideoId: "BV1xx411c7mD",
      downloadTarget: "https://blog.example/post-with-video",
      url: "https://www.bilibili.com/video/BV1xx411c7mD/",
    };
    expect(normalizedBilibiliVideoID(bilibili)).toBe("BV1xx411c7mD");
    expect(canonicalRSSVideoTarget(bilibili)).toBe(
      "https://www.bilibili.com/video/BV1xx411c7mD/",
    );
  });

  test("does not treat unsafe or HTML media as a direct video", () => {
    expect(resolveRSSVideoExperience({
      ...base,
      playbackUrl: "javascript:alert(1)",
      media: [{ kind: "video", mimeType: "text/html", url: "https://unknown.example/embed" }],
    }).mode).toBe("unavailable");

    expect(resolveRSSVideoExperience({
      ...base,
      media: [{ kind: "video", mimeType: "video/mp4", url: "https://cdn.attacker.example/video.mp4" }],
    }).mode).toBe("unavailable");
  });

  test("plays direct enclosures only through the controlled loopback slot", () => {
    expect(resolveRSSVideoExperience({
      ...base,
      media: [{ kind: "video", mimeType: "video/mp4", url: controlledMediaURL }],
    })).toMatchObject({ mode: "direct", playbackUrl: controlledMediaURL });
  });

  test("builds a deduplicated multiline target for the existing batch task dialog", () => {
    const entries = [
      { ...base, id: "article", kind: "article" as const, url: "https://example.com/post" },
      { ...base, id: "unknown", downloadTarget: "https://video.example/unknown" },
      {
        ...base,
        id: "one",
        downloadTarget: "https://video.example/one",
        url: "https://vimeo.com/123456",
      },
      {
        ...base,
        id: "duplicate",
        downloadTarget: "https://video.example/one",
        url: "https://vimeo.com/123456",
      },
      {
        ...base,
        id: "two",
        mediaUrl: "https://cdn.example/two.mp4",
        url: "https://www.tiktok.com/@creator/video/2",
      },
    ];
    expect(buildRSSVideoBatchDownloadTarget(entries)).toBe(
      "https://video.example/one\nhttps://cdn.example/two.mp4",
    );
    expect(buildRSSVideoBatchDownloadTargets(entries)).toEqual([
      {
        entryId: "one",
        url: "https://video.example/one",
        source: "xiadown.rss",
        caller: "rss-entry:one",
      },
      {
        entryId: "two",
        url: "https://cdn.example/two.mp4",
        source: "xiadown.rss",
        caller: "rss-entry:two",
      },
    ]);
  });
});
