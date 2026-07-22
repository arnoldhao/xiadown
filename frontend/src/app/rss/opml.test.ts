import { describe, expect, test } from "bun:test";

import type { RSSSubscription } from "./types";
import {
  exportRSSSubscriptionsToOPML,
  parseRSSSubscriptionsFromOPML,
  RSS_OPML_MAX_SOURCE_BYTES,
  RSS_OPML_MAX_SUBSCRIPTIONS,
} from "./opml";

const subscription: RSSSubscription = {
  id: "one",
  workspaceId: "rss-default",
  feedUrl: "https://example.com/feed.xml?one=1&two=2",
  siteUrl: "https://example.com/",
  title: "Example & updates",
  viewType: "article",
  enabled: true,
  unreadCount: 0,
  createdAt: "2026-07-13T00:00:00Z",
  updatedAt: "2026-07-13T00:00:00Z",
  revision: 1,
};

describe("RSS OPML portability", () => {
  test("round-trips title, URL and XiaDown view type with valid XML escaping", () => {
    const source = exportRSSSubscriptionsToOPML(
      [subscription],
      new Date("2026-07-13T00:00:00Z"),
    );
    expect(source).toContain("Example &amp; updates");
    expect(source).toContain("one=1&amp;two=2");
    expect(source).toContain("<title>XiaDown RSS subscriptions</title>");
    expect(parseRSSSubscriptionsFromOPML(source)).toEqual([{
      url: subscription.feedUrl,
      title: subscription.title,
      viewType: "article",
    }]);
  });

  test("supports nested standard outlines, feed URLs and duplicate removal", () => {
    expect(parseRSSSubscriptionsFromOPML(`<?xml version="1.0"?><opml version="2.0"><body>
      <outline text="Folder">
        <outline type="rss" text="Feed" xmlUrl="feed://example.com/rss"/>
        <outline type="rss" text="Duplicate" xmlUrl="https://example.com/rss"/>
        <outline type="rss" text="Bili" xmlUrl="rsshub://bilibili/ranking/0/"/>
      </outline>
    </body></opml>`)).toEqual([
      { url: "https://example.com/rss", title: "Feed" },
      { url: "rsshub://bilibili/ranking/0", title: "Bili" },
    ]);
  });

  test("rejects malformed XML and ignores unsafe schemes", () => {
    expect(() => parseRSSSubscriptionsFromOPML("<opml><body>")).toThrow();
    expect(parseRSSSubscriptionsFromOPML(
      '<opml><body><outline xmlUrl="file:///etc/passwd"/></body></opml>',
    )).toEqual([]);
  });

  test("rejects oversized sources and unbounded feed lists", () => {
    expect(() => parseRSSSubscriptionsFromOPML(
      "x".repeat(RSS_OPML_MAX_SOURCE_BYTES + 1),
    )).toThrow("invalid_opml");
    expect(() => parseRSSSubscriptionsFromOPML(
      "\u754c".repeat(Math.floor(RSS_OPML_MAX_SOURCE_BYTES / 2)),
    )).toThrow("invalid_opml");
    const outlines = Array.from(
      { length: RSS_OPML_MAX_SUBSCRIPTIONS + 1 },
      (_, index) => `<outline xmlUrl="https://example.com/${index}.xml"/>`,
    ).join("");
    expect(() => parseRSSSubscriptionsFromOPML(
      `<opml><body>${outlines}</body></opml>`,
    )).toThrow("invalid_opml");
  });
});
