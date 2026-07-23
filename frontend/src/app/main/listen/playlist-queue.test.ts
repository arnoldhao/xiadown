import { describe, expect, test } from "bun:test";

import {
  fetchCompleteListenPlaylistQueue,
  fetchListenPlaylistPage,
  fetchListenPlaylistQueue,
} from "@/app/main/listen/api";
import type { ListenOnlineItem } from "@/app/main/listen/types";

function playlistTrack(videoId: string, title: string) {
  return {
    id: videoId,
    group: "playlist",
    videoId,
    title,
    channel: "Artist",
    description: "Album",
    durationLabel: "3:00",
  };
}

describe("listen playlist queue pagination", () => {
  test("keeps structured album metadata separate from its description", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          author: "Album Artist",
          authorBrowseId: "UCalbumartist",
          trackCountLabel: "10 songs",
          durationLabel: "42 minutes",
          description: "A story about 10 songs and 42 minutes.",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      )) as typeof fetch;

    try {
      const page = await fetchListenPlaylistPage(
        "http://127.0.0.1:34115",
        "MPREalbum123",
        new AbortController().signal,
      );

      expect(page.authorBrowseId).toBe("UCalbumartist");
      expect(page.trackCountLabel).toBe("10 songs");
      expect(page.durationLabel).toBe("42 minutes");
      expect(page.description).toBe("A story about 10 songs and 42 minutes.");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("keeps the mix queue helper bounded to its first response", async () => {
    const originalFetch = globalThis.fetch;
    let fetchCalls = 0;
    globalThis.fetch = (async () => {
      fetchCalls += 1;
      return new Response(
        JSON.stringify({
          items: [playlistTrack("MIXQUEUE001", "Mix Track")],
          continuation: "dynamic-radio-cursor",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    try {
      const items = await fetchListenPlaylistQueue(
        "http://127.0.0.1:34115",
        "RDMIX123",
        new AbortController().signal,
        "zh-CN",
      );

      expect(fetchCalls).toBe(1);
      expect(items.map((item) => item.videoId)).toEqual(["MIXQUEUE001"]);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("collects continuation pages from the visible static playlist", async () => {
    const originalFetch = globalThis.fetch;
    const requestedContinuations: string[] = [];
    globalThis.fetch = (async (input) => {
      const url = new URL(String(input));
      const continuation = url.searchParams.get("continuation") ?? "";
      requestedContinuations.push(continuation);

      const payload =
        continuation === "cursor-1"
          ? {
              items: [
                playlistTrack("QUEUEVID001", "One"),
                playlistTrack("QUEUEVID002", "Two"),
              ],
              continuation: "cursor-2",
            }
          : {
              items: [playlistTrack("QUEUEVID003", "Three")],
              continuation: "cursor-2",
            };

      return new Response(JSON.stringify(payload), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }) as typeof fetch;

    try {
      const result = await fetchCompleteListenPlaylistQueue(
        "http://127.0.0.1:34115",
        "VLPL123",
        new AbortController().signal,
        {
          continuation: "cursor-1",
          initialItems: [
            playlistTrack("QUEUEVID001", "One") as ListenOnlineItem,
          ],
          language: "zh-CN",
        },
      );

      expect(requestedContinuations).toEqual(["cursor-1", "cursor-2"]);
      expect(result.items.map((item) => item.videoId)).toEqual([
        "QUEUEVID001",
        "QUEUEVID002",
        "QUEUEVID003",
      ]);
      expect(result.continuation).toBe("");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("caps a static playlist with endlessly unique continuations", async () => {
    const originalFetch = globalThis.fetch;
    let fetchCalls = 0;
    globalThis.fetch = (async () => {
      fetchCalls += 1;
      return new Response(
        JSON.stringify({
          items: [
            playlistTrack(
              `QUEUE${String(fetchCalls).padStart(6, "0")}`,
              `Track ${fetchCalls}`,
            ),
          ],
          continuation: `cursor-${fetchCalls}`,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    try {
      const result = await fetchCompleteListenPlaylistQueue(
        "http://127.0.0.1:34115",
        "VLPL123",
        new AbortController().signal,
        {
          continuation: "cursor-0",
          initialItems: [],
          language: "zh-CN",
        },
      );

      expect(fetchCalls).toBe(128);
      expect(result.items).toHaveLength(128);
      expect(result.continuation).toBe("cursor-128");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
