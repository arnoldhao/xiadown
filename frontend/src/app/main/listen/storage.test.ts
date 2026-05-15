import { describe, expect, test } from "bun:test";

import {
  buildListenImageCacheURL,
  buildListenHighQualityThumbnailURL,
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

  test("drops stale audio endpoint video-unavailable cache entries", () => {
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

    expect(items[0].hasVideo).toBeUndefined();
    expect(items[0].videoAvailabilityKnown).toBeUndefined();
  });
});
