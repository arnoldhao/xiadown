import { describe, expect, test } from "bun:test";

import { resolveRSSBilibiliDisplayMetadata } from "./bilibili-page-metadata";

const entry = {
  author: "Feed publisher",
  publishedAt: "2026-07-12T10:00:00Z",
  sourceUpdatedAt: "2026-07-13T10:00:00Z",
  createdAt: "2026-07-14T10:00:00Z",
};

describe("RSS Bilibili display metadata", () => {
  test("prefers active canonical-page metadata", () => {
    expect(
      resolveRSSBilibiliDisplayMetadata(
        entry,
        { title: "Feed" },
        {
          sessionId: "session-current",
          platformVideoId: "BV1xx411c7mD",
          publisher: "Canonical publisher",
          publishedAt: "2026-07-11T08:30:00Z",
          viewCount: 1_250_400,
          likeCount: 48_200,
        },
        "bv1xx411c7mD",
      ),
    ).toEqual({
      publisher: "Canonical publisher",
      publishedAt: "2026-07-11T08:30:00Z",
      viewCount: 1_250_400,
      likeCount: 48_200,
    });
  });

  test("falls back to feed data and hides invalid counters", () => {
    expect(
      resolveRSSBilibiliDisplayMetadata(
        entry,
        { title: "Feed" },
        {
          sessionId: "session-current",
          platformVideoId: "BV1xx411c7mD",
          publisher: " ",
          publishedAt: "",
          viewCount: Number.NaN,
          likeCount: -1,
        },
        "BV1xx411c7mD",
      ),
    ).toEqual({
      publisher: "Feed publisher",
      publishedAt: "2026-07-12T10:00:00Z",
      viewCount: 0,
      likeCount: 0,
    });
  });

  test("uses source title when the feed has no author", () => {
    expect(
      resolveRSSBilibiliDisplayMetadata(
        { ...entry, author: undefined, publishedAt: undefined },
        { title: "Subscription publisher" },
        null,
        "BV1xx411c7mD",
      ),
    ).toMatchObject({
      publisher: "Subscription publisher",
      publishedAt: "2026-07-13T10:00:00Z",
    });
  });

  test("rejects live metadata from a stale video identity", () => {
    expect(
      resolveRSSBilibiliDisplayMetadata(
        entry,
        { title: "Feed" },
        {
          sessionId: "session-old",
          platformVideoId: "BV1old411111",
          publisher: "Wrong publisher",
          publishedAt: "2020-01-01T00:00:00Z",
          viewCount: 999,
          likeCount: 888,
        },
        "BV1xx411c7mD",
      ),
    ).toEqual({
      publisher: "Feed publisher",
      publishedAt: "2026-07-12T10:00:00Z",
      viewCount: 0,
      likeCount: 0,
    });
  });

  test("treats the case-sensitive BVID suffix as part of the video identity", () => {
    expect(
      resolveRSSBilibiliDisplayMetadata(
        entry,
        { title: "Feed" },
        {
          sessionId: "session-stale-case",
          platformVideoId: "BV1XX411C7MD",
          publisher: "Wrong publisher",
          publishedAt: "2020-01-01T00:00:00Z",
          viewCount: 999,
          likeCount: 888,
        },
        "BV1xx411c7mD",
      ),
    ).toEqual({
      publisher: "Feed publisher",
      publishedAt: "2026-07-12T10:00:00Z",
      viewCount: 0,
      likeCount: 0,
    });
  });

  test("matches canonical AV identities after removing numeric leading zeroes", () => {
    expect(
      resolveRSSBilibiliDisplayMetadata(
        entry,
        { title: "Feed" },
        {
          sessionId: "session-av",
          platformVideoId: "av170001",
          publisher: "Canonical AV publisher",
          publishedAt: "2026-07-11T08:30:00Z",
          viewCount: 1_250_400,
          likeCount: 48_200,
        },
        "AV000170001",
      ),
    ).toEqual({
      publisher: "Canonical AV publisher",
      publishedAt: "2026-07-11T08:30:00Z",
      viewCount: 1_250_400,
      likeCount: 48_200,
    });
  });

  test("matches canonical Bangumi identities without mixing ep and ss", () => {
    expect(
      resolveRSSBilibiliDisplayMetadata(
        entry,
        { title: "Feed" },
        {
          sessionId: "session-ep",
          platformVideoId: "EP003854807",
          publisher: "Bangumi publisher",
          viewCount: 12_345,
        },
        "ep3854807",
      ),
    ).toMatchObject({
      publisher: "Bangumi publisher",
      viewCount: 12_345,
    });
    expect(
      resolveRSSBilibiliDisplayMetadata(
        entry,
        { title: "Feed" },
        {
          sessionId: "session-season",
          platformVideoId: "ss3854807",
          publisher: "Wrong season",
          viewCount: 99,
        },
        "ep3854807",
      ),
    ).toMatchObject({
      publisher: "Feed publisher",
      viewCount: 0,
    });
  });

  test("uses the YouTube watch chrome and localized common actions", async () => {
    const [pageSource, playbackSource, apiSource, youtubeCSS] = await Promise.all([
      Bun.file(new URL("./RSSWorkspacePage.tsx", import.meta.url)).text(),
      Bun.file(new URL("./RSSBilibiliPlayback.tsx", import.meta.url)).text(),
      Bun.file(new URL("./api.ts", import.meta.url)).text(),
      Bun.file(new URL("../youtube/youtube-workspace.css", import.meta.url)).text(),
    ]);

    expect(pageSource).toContain(
      'className="youtube-workspace-watch-header rss-video-watch-header wails-drag"',
    );
    expect(pageSource).toContain('data-has-back={props.onBack ? "true" : "false"}');
    expect(pageSource).toContain('className="youtube-workspace-watch-byline"');
    expect(pageSource).toContain('className="youtube-workspace-watch-stats"');
    expect(pageSource).toContain("<RSSVideoMoreMenu");
    expect(pageSource).toMatch(
      /<RSSBilibiliPlayback[\s\S]{0,400}onMetadata=\{receiveBilibiliMetadata\}/,
    );
    expect(pageSource).not.toMatch(
      /<RSSYouTubePlayback[\s\S]{0,300}onMetadata=\{receiveBilibiliMetadata\}/,
    );
    for (const key of [
      "xiadown.youtube.published",
      "xiadown.youtube.views",
      "xiadown.youtube.likes",
      "xiadown.workspace.more",
      "xiadown.actions.download",
      "xiadown.rss.openInBrowser",
      "xiadown.rss.copyLink",
      "xiadown.rss.copyTitle",
      "xiadown.rss.share",
    ]) {
      expect(pageSource).toContain(`t("${key}")`);
    }
    expect(youtubeCSS).toContain(".youtube-workspace-watch-more-menu");
    const rssCSS = await Bun.file(
      new URL("./rss-workspace.css", import.meta.url),
    ).text();
    expect(rssCSS).toMatch(
      /\.rss-video-watch-header\[data-has-back="false"\] \{[^}]*grid-template-columns: minmax\(0, 1fr\);/,
    );
    expect(playbackSource).toContain("onMetadataRef.current?.(metadata)");
    expect(playbackSource).toContain("platformVideoId: next.platformVideoId.trim() || platformVideoId");
    expect(playbackSource).toContain("metadataSignatureRef.current !== metadataSignature");
    expect(playbackSource.indexOf("!isRSSBilibiliVideoStatusForSession(next, sessionID)")).toBeLessThan(
      playbackSource.indexOf("onMetadataRef.current?.(metadata)"),
    );
    for (const field of ["publisher?: string", "publishedAt?: string", "viewCount?: number", "likeCount?: number"]) {
      expect(apiSource).toContain(field);
    }
  });
});
