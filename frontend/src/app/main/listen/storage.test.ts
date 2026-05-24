import { describe, expect, test } from "bun:test";

import {
  buildListenImageCacheURL,
  buildListenHighQualityThumbnailURL,
  buildListenTrackThumbnailCandidates,
  buildYouTubePosterURL,
  sanitizeListenOnlineItems,
  updateListenProgressMap,
} from "@/app/main/listen/storage";

describe("listen playback storage helpers", () => {
  test("removes zero progress from persisted resume map", () => {
    expect(updateListenProgressMap({ "song-a": 12 }, "song-a", 0)).toEqual({});
  });

  test("unwraps stale listen image cache URLs back to their source", () => {
    const cached = "http://127.0.0.1:5678/api/listen/image?url=https%3A%2F%2Fexample.com%2Fart.jpg";

    expect(buildListenImageCacheURL("http://127.0.0.1:5678", cached)).toBe(
      "https://example.com/art.jpg",
    );
    expect(
      buildListenImageCacheURL(
        "http://127.0.0.1:5678",
        "/api/listen/image?url=https%3A%2F%2Fexample.com%2Fart.jpg",
      ),
    ).toBe("https://example.com/art.jpg");
  });

  test("leaves remote artwork URLs direct instead of wrapping them in localhost", () => {
    expect(
      buildListenImageCacheURL(
        "http://127.0.0.1:5678",
        "https://lh3.googleusercontent.com/art=w60-h60",
        { size: 320 },
      ),
    ).toBe("https://lh3.googleusercontent.com/art=w60-h60");
  });

  test("rewrites YouTube Music thumbnail sizes without stripping other parameters", () => {
    expect(
      buildListenHighQualityThumbnailURL(
        "https://lh3.googleusercontent.com/art=w60-h60=s120",
      ),
    ).toBe("https://lh3.googleusercontent.com/art=w226-h226=s120");
  });

  test("adds public YouTube thumbnail fallback for track artwork", () => {
    expect(buildYouTubePosterURL("TESTVID007G")).toBe(
      "https://i.ytimg.com/vi/TESTVID007G/hqdefault.jpg",
    );
    expect(
      buildListenTrackThumbnailCandidates("http://127.0.0.1:5678", {
        videoId: "TESTVID007G",
        thumbnailUrl: "",
      }),
    ).toEqual(["https://i.ytimg.com/vi/TESTVID007G/hqdefault.jpg"]);
  });

  test("keeps audio endpoint video-unavailable cache entries", () => {
    const items = sanitizeListenOnlineItems([
      {
        id: "track",
        group: "playlist",
        videoId: "TESTVID007G",
        title: "Track",
        channel: "Artist",
        description: "",
        durationLabel: "",
        musicVideoType: "MUSIC_VIDEO_TYPE_ATV",
        hasVideo: false,
        videoAvailabilityKnown: true,
      },
    ]);

    expect(items[0].hasVideo).toBe(false);
    expect(items[0].videoAvailabilityKnown).toBe(true);
  });

  test("keeps structured artists in persisted online queue items", () => {
    const items = sanitizeListenOnlineItems([
      {
        id: "track",
        group: "playlist",
        videoId: "TESTVID007G",
        title: "Track",
        channel: "Artist A, Artist B",
        artists: [
          { name: "Artist A", browseId: "UCartistA" },
          { name: "Artist B", browseId: "UCartistB" },
        ],
        description: "",
        durationLabel: "",
      },
    ]);

    expect(items[0].artists).toEqual([
      { name: "Artist A", browseId: "UCartistA", thumbnailUrl: undefined },
      { name: "Artist B", browseId: "UCartistB", thumbnailUrl: undefined },
    ]);
  });

  test("drops stale user generated endpoint video-unavailable cache entries", () => {
    const items = sanitizeListenOnlineItems([
      {
        id: "track",
        group: "playlist",
        videoId: "TESTVID007G",
        title: "Track",
        channel: "Artist",
        description: "",
        durationLabel: "",
        musicVideoType: "MUSIC_VIDEO_TYPE_UGC",
        hasVideo: false,
        videoAvailabilityKnown: true,
      },
    ]);

    expect(items[0].hasVideo).toBeUndefined();
    expect(items[0].videoAvailabilityKnown).toBeUndefined();
  });

  test("drops stale video-capable cache entries without a metadata signal", () => {
    const items = sanitizeListenOnlineItems([
      {
        id: "track",
        group: "playlist",
        videoId: "TESTVID007G",
        title: "Track",
        channel: "Artist",
        description: "",
        durationLabel: "",
        musicVideoType: "MUSIC_VIDEO_TYPE_UGC",
        hasVideo: true,
        videoAvailabilityKnown: true,
      },
    ]);

    expect(items[0].hasVideo).toBeUndefined();
    expect(items[0].videoAvailabilityKnown).toBeUndefined();
  });

  test("uses non-video thumbnails as confirmed unavailable", () => {
    const items = sanitizeListenOnlineItems([
      {
        id: "track",
        group: "playlist",
        videoId: "TESTVID007G",
        title: "Track",
        channel: "Artist",
        description: "",
        durationLabel: "",
        musicVideoType: "MUSIC_VIDEO_TYPE_UGC",
        thumbnailUrl: "https://lh3.googleusercontent.com/art=w544-h544",
      },
    ]);

    expect(items[0].hasVideo).toBe(false);
    expect(items[0].videoAvailabilityKnown).toBe(true);
  });

  test("restores YouTube video thumbnails as video-capable", () => {
    const items = sanitizeListenOnlineItems([
      {
        id: "track",
        group: "playlist",
        videoId: "TESTVID007G",
        title: "Track",
        channel: "Artist",
        description: "",
        durationLabel: "",
        musicVideoType: "MUSIC_VIDEO_TYPE_PODCAST_EPISODE",
        thumbnailUrl: "https://i.ytimg.com/vi/TESTVID007G/hq720.jpg",
      },
    ]);

    expect(items[0].hasVideo).toBe(true);
    expect(items[0].videoAvailabilityKnown).toBe(true);
  });
});
