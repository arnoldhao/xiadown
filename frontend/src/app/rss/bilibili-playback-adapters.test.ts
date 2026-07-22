import { describe, expect, test } from "bun:test";

import {
  canonicalRSSBilibiliIdentityKey,
  normalizedBilibiliBangumiID,
  normalizedBilibiliVideoID,
  resolveRSSBilibiliPlaybackIdentity,
  RSS_BILIBILI_BANGUMI_ADAPTER,
  RSS_BILIBILI_VIDEO_ADAPTER,
} from "./bilibili-playback-adapters";

describe("RSS Bilibili playback adapters", () => {
  test("keeps ordinary video and Bangumi as explicit adapters", () => {
    expect(RSS_BILIBILI_VIDEO_ADAPTER.key).toBe("video");
    expect(RSS_BILIBILI_BANGUMI_ADAPTER.key).toBe("bangumi");
  });

  test("resolves ordinary BV and av identities to full video pages", () => {
    expect(resolveRSSBilibiliPlaybackIdentity({
      platformVideoId: "bv1xx411c7mD",
    })).toEqual({
      adapter: "video",
      platformVideoId: "BV1xx411c7mD",
      playbackUrl: "https://www.bilibili.com/video/BV1xx411c7mD/",
    });
    expect(resolveRSSBilibiliPlaybackIdentity({
      platformVideoId: "AV000170001",
    })).toEqual({
      adapter: "video",
      platformVideoId: "av170001",
      playbackUrl: "https://www.bilibili.com/video/av170001/",
    });
  });

  test("resolves ep and ss identities to canonical Bangumi pages", () => {
    expect(resolveRSSBilibiliPlaybackIdentity({
      platformVideoId: "EP003854807",
    })).toEqual({
      adapter: "bangumi",
      platformVideoId: "ep3854807",
      playbackUrl: "https://www.bilibili.com/bangumi/play/ep3854807",
    });
    expect(resolveRSSBilibiliPlaybackIdentity({
      url: "https://www.bilibili.com/bangumi/play/ss28747?from=search",
    })).toEqual({
      adapter: "bangumi",
      platformVideoId: "ss28747",
      playbackUrl: "https://www.bilibili.com/bangumi/play/ss28747",
    });
  });

  test("preserves the legacy ordinary-video normalizer boundary", () => {
    const bangumi = {
      platformVideoId: "ep3854807",
      url: "https://www.bilibili.com/bangumi/play/ep3854807",
    };
    expect(normalizedBilibiliVideoID(bangumi)).toBe("");
    expect(normalizedBilibiliBangumiID(bangumi)).toBe("ep3854807");
  });

  test("scopes canonical keys by adapter as well as platform id", () => {
    expect(canonicalRSSBilibiliIdentityKey({ platformVideoId: "ep123" }))
      .toBe("bangumi:ep123");
    expect(canonicalRSSBilibiliIdentityKey({ platformVideoId: "av123" }))
      .toBe("video:av123");
  });
});
