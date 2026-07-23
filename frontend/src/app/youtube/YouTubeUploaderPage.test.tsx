import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  YouTubeUploaderPage,
  YouTubeUploaderContent,
  type YouTubeUploaderPageLabels,
} from "@/app/youtube/YouTubeUploaderPage";
import type { YouTubeUploaderPageData } from "@/app/youtube/types";

const labels: YouTubeUploaderPageLabels = {
  back: "Back",
  subscribe: "Subscribe",
  unsubscribe: "Subscribed",
  videos: "Videos",
  loading: "Loading channel",
  empty: "No videos",
  error: "Channel unavailable",
  retry: "Retry",
  loadMore: "Load more",
  fallbackChannel: "YouTube",
  more: "More",
  description: "Description",
  close: "Close",
};

const page: YouTubeUploaderPageData = {
  channelId: "UCabcdefghijklmnopqrstuv",
  name: "Workspace Creator",
  handle: "@workspace",
  description: "A complete creator description.",
  avatarUrl: "https://yt3.example/avatar.jpg",
  bannerUrl: "https://yt3.example/banner.jpg",
  subscriberCount: 34900,
  subscriberLabel: "34.9K subscribers",
  videoCountLabel: "173 videos",
  isSubscribed: true,
  webUrl: "https://www.youtube.com/@workspace",
  videos: [{
    itemKind: "video",
    videoId: "LatestVid01",
    title: "A title that can occupy two stable lines in the channel grid",
    channel: "Workspace Creator",
    channelId: "UCabcdefghijklmnopqrstuv",
    thumbnailUrl: "https://i.ytimg.com/vi/LatestVid01/hqdefault.jpg",
    durationLabel: "3:42",
    viewCount: 1_250_000,
    publishedLabel: "2 days ago",
    webUrl: "https://www.youtube.com/watch?v=LatestVid01",
  }],
  continuation: "page-2",
};

describe("YouTubeUploaderPage", () => {
  test("keeps one route heading and a usable drag region while loading", () => {
    const markup = renderToStaticMarkup(
      <YouTubeUploaderPage
        channelId="UCabcdefghijklmnopqrstuv"
        locale="en"
        fallbackName="Workspace Creator"
        labels={labels}
        onBack={() => {}}
        onOpenVideo={() => {}}
      />,
    );

    expect(markup.match(/<h1/g)).toHaveLength(1);
    expect(markup).toContain('class="sr-only">Workspace Creator</h1>');
    expect(markup).toContain("youtube-uploader-page wails-drag");
    expect(markup).toContain("youtube-uploader-back wails-no-drag");
  });

  test("renders a YouTube-style channel hero and video grid", () => {
    const markup = renderToStaticMarkup(
      <YouTubeUploaderContent
        page={page}
        locale="en"
        labels={labels}
        onBack={() => {}}
        onOpenVideo={() => {}}
        onToggleSubscription={() => {}}
        onLoadMore={() => {}}
      />,
    );

    expect(markup).toContain('data-youtube-uploader="UCabcdefghijklmnopqrstuv"');
    expect(markup.match(/<h1/g)).toHaveLength(1);
    expect(markup).toContain("youtube-uploader-hero wails-drag");
    expect(markup).toContain("youtube-uploader-subscribe");
    const subscribeClass = markup.indexOf("youtube-uploader-subscribe");
    const subscribeButtonStart = markup.lastIndexOf("<button", subscribeClass);
    const subscribeButtonEnd = markup.indexOf("</button>", subscribeClass);
    const subscribeButton = markup.slice(
      subscribeButtonStart,
      subscribeButtonEnd,
    );
    expect(subscribeButton).toContain('data-variant="ghost"');
    expect(subscribeButton).toContain('data-size="compactIcon"');
    expect(subscribeButton).toContain('data-shape="circle"');
    expect(subscribeButton).not.toContain('data-variant="outline"');
    expect(subscribeButton).not.toContain('data-variant="default"');
    expect(subscribeButton).not.toContain('data-variant="secondary"');
    expect(markup).toContain("youtube-uploader-banner");
    expect(markup).toContain("https://yt3.example/banner.jpg");
    expect(markup).toContain("https://yt3.example/avatar.jpg");
    expect(markup).toContain("Workspace Creator");
    expect(markup).toContain("@workspace");
    expect(markup).toContain("34.9K subscribers");
    expect(markup).toContain("173 videos");
    expect(markup).toContain("A complete creator description.");
    expect(markup).toContain("A title that can occupy two stable lines");
    expect(markup).toContain("1.3M");
    expect(markup).toContain("2 days ago");
    expect(markup).toContain("3:42");
    expect(markup).toContain("Subscribed");
    expect(markup).toContain('aria-pressed="true"');
    expect(markup).not.toContain("<span>Subscribed</span>");
    expect(markup).toContain("More");
    expect(markup).toContain('data-listen-infinite-scroll-sentinel="true"');
    expect(markup).not.toContain(">Load more<");
  });

  test("uses resilient YouTube images and returns the channel queue", async () => {
    const source = await Bun.file(
      new URL("./YouTubeUploaderPage.tsx", import.meta.url),
    ).text();
    const css = await Bun.file(
      new URL("./youtube-uploader-page.css", import.meta.url),
    ).text();

    expect(source).toContain("<YouTubeImage");
    expect(source).not.toContain("<img");
    expect(source).toContain("onOpenVideo(video, page.videos)");
    expect(source).toContain("getYouTubeWorkspaceUploader(channelId");
    expect(source).toContain("setYouTubeWorkspaceChannelSubscription");
    expect(source).toContain("<ListenInfiniteScrollSentinel");
    expect(source).toContain("video.publishedLabel || \"\"");
    expect(source).toContain("loadingMoreRef.current");
    expect(source).not.toContain("youtube-uploader-load-more");
    expect(source).toContain('size="compactIcon"');
    expect(source).toContain('shape="circle"');
    expect(source).toContain('variant="glass"');
    expect(source).toContain("<DialogContent");
    expect(source).toContain('className="youtube-uploader-info-dialog"');
    expect(source).toContain("<DialogScrollArea");
    expect(source).toContain('title={page.description}');
    expect(css).toContain("height: 40px");
    expect(css).toContain("-webkit-line-clamp: 2");
    expect(css).toContain("text-overflow: ellipsis");
    expect(css).toContain(".youtube-workspace-uploader-scroll");
    expect(css).toMatch(
      /data-platform="windows"[\s\S]*?youtube-workspace-uploader-scroll[\s\S]*?padding-top:\s*0/,
    );
    expect(css).toContain(
      "top: calc(var(--app-windows-caption-button-height) + 14px);",
    );
  });

  test("opens uploader metadata inside the YouTube primary pane", async () => {
    const source = await Bun.file(
      new URL("./YouTubeWorkspacePage.tsx", import.meta.url),
    ).text();

    expect(source).toContain('uploaderTarget ? "uploader"');
    expect(source).toContain("<YouTubeUploaderPage");
    expect(source).toContain("onOpenUploader={uploaderChannelID ? openUploader : undefined}");
    expect(source).toContain(
      "returnToUploader: uploaderTarget",
    );
    expect(source).not.toContain("openExternalURL(uploaderURL)");
  });
});
