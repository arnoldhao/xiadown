import { describe, expect, test } from "bun:test";

import {
  controlledRSSEntryImageResource,
  controlledRSSResourceOrigin,
  controlledRSSResourceURL,
} from "./remote-resource";

const token = "a".repeat(64);
const base = `http://127.0.0.1:43127/_xiadown/${token}`;

describe("RSS controlled remote resources", () => {
  test("accepts only tokenized loopback entity slots", () => {
    expect(controlledRSSResourceURL(`${base}/api/rss/subscriptions/sub-1/icon`)).toBe(
      `${base}/api/rss/subscriptions/sub-1/icon`,
    );
    expect(controlledRSSResourceURL(`${base}/api/rss/discovery/categories/multimedia/icon`)).toBe(
      `${base}/api/rss/discovery/categories/multimedia/icon`,
    );
    expect(controlledRSSResourceURL(`${base}/api/rss/discovery/routes/rsshub:bilibili-ranking/icon`)).toBe(
      `${base}/api/rss/discovery/routes/rsshub:bilibili-ranking/icon`,
    );
    expect(controlledRSSResourceURL(`${base}/api/rss/entries/entry-1/resources/thumbnail`)).toBe(
      `${base}/api/rss/entries/entry-1/resources/thumbnail`,
    );
    expect(controlledRSSResourceURL(`${base}/api/rss/entries/entry-1/resources/image-3`)).toBe(
      `${base}/api/rss/entries/entry-1/resources/image-3`,
    );
    expect(controlledRSSResourceURL(`${base}/api/rss/entries/entry-1/resources/media-2-thumbnail`)).toBe(
      `${base}/api/rss/entries/entry-1/resources/media-2-thumbnail`,
    );
    expect(controlledRSSResourceURL(`${base}/api/rss/entries/entry-1/resources/image-63`)).toBe(
      `${base}/api/rss/entries/entry-1/resources/image-63`,
    );
    expect(controlledRSSResourceURL(`${base}/api/rss/entries/entry-1/resources/image-3?v=17`)).toBe(
      `${base}/api/rss/entries/entry-1/resources/image-3?v=17`,
    );
    expect(controlledRSSResourceURL(`${base}/api/rss/subscriptions/sub-1/icon?v=2`)).toBe(
      `${base}/api/rss/subscriptions/sub-1/icon?v=2`,
    );
  });

  test("rejects feed URLs, untokenized localhost and query-based relays", () => {
    expect(controlledRSSResourceURL("https://images.example/feed.jpg")).toBe("");
    expect(controlledRSSResourceURL("http://127.0.0.1:43127/api/rss/entries/e/resources/image-0")).toBe("");
    expect(controlledRSSResourceURL(`${base}/api/rss/resource?url=https://private.example`)).toBe("");
    expect(controlledRSSResourceURL(`${base}/api/rss/discovery/sources/bilibili/icon`)).toBe("");
    expect(controlledRSSResourceURL(`${base}/api/rss/discovery/routes/bilibili/resources/icon`)).toBe("");
    expect(controlledRSSResourceURL(`${base}/api/rss/entries/e/resources/media-0?token=leak`)).toBe("");
    expect(controlledRSSResourceURL(`${base}/api/rss/entries/e/resources/media-0?v=0`)).toBe("");
    expect(controlledRSSResourceURL(`${base}/api/rss/entries/e/resources/media-0?v=1&url=https://private.example`)).toBe("");
    for (const slot of ["image-00", "image-01", "image-64", "media-064", "media-64-thumbnail"]) {
      expect(controlledRSSResourceURL(`${base}/api/rss/entries/e/resources/${slot}`)).toBe("");
    }
  });

  test("derives only the controlled loopback CSP origin", () => {
    expect(controlledRSSResourceOrigin([
      "https://attacker.example/image.png",
      `${base}/api/rss/entries/e/resources/image-0`,
    ])).toBe("http://127.0.0.1:43127");
  });

  test("reduces reader images to the expected persisted entry slot", () => {
    expect(controlledRSSEntryImageResource(
      `${base}/api/rss/entries/entry-1/resources/image-3?v=17`,
      "entry-1",
    )).toEqual({ entryId: "entry-1", slot: "image-3" });
    expect(controlledRSSEntryImageResource(
      `${base}/api/rss/entries/entry-2/resources/image-3?v=17`,
      "entry-1",
    )).toBeNull();
    expect(controlledRSSEntryImageResource(
      "https://images.example/feed.jpg",
      "entry-1",
    )).toBeNull();
  });
});
