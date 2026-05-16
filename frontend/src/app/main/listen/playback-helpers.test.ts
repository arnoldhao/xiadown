import { describe, expect, test } from "bun:test";

import { resolveListenTrackVideoAvailability } from "@/app/main/listen/playback-helpers";
import type { ListenOnlineItem } from "@/app/main/listen/types";

function item(overrides: Partial<ListenOnlineItem>): ListenOnlineItem {
  return {
    id: "track",
    group: "playlist",
    videoId: "TESTVID007G",
    title: "Track",
    channel: "Artist",
    description: "",
    durationLabel: "",
    ...overrides,
  };
}

describe("listen playback video availability", () => {
  test("treats audio endpoint musicVideoType as confirmed no video", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({ musicVideoType: "MUSIC_VIDEO_TYPE_ATV" }),
        false,
      ),
    ).toBe("unavailable");
  });

  test("does not treat user generated endpoint type as confirmed video", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({ musicVideoType: "MUSIC_VIDEO_TYPE_UGC" }),
        false,
      ),
    ).toBe("checking");
  });

  test("ignores stale non-ATV unavailable metadata", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({
          musicVideoType: "MUSIC_VIDEO_TYPE_UGC",
          hasVideo: false,
          videoAvailabilityKnown: true,
        }),
        false,
      ),
    ).toBe("checking");
  });

  test("treats a non-video thumbnail as unavailable once artwork is known", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({
          musicVideoType: "MUSIC_VIDEO_TYPE_UGC",
          thumbnailUrl: "https://lh3.googleusercontent.com/art=w544-h544",
        }),
        false,
      ),
    ).toBe("unavailable");
  });

  test("infers video from YouTube video thumbnail", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({
          musicVideoType: "MUSIC_VIDEO_TYPE_PODCAST_EPISODE",
          thumbnailUrl: "https://i.ytimg.com/vi/TESTVID007G/hq720.jpg",
        }),
        false,
      ),
    ).toBe("available");
  });

  test("keeps authoritative unavailable metadata unavailable", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({
          hasVideo: false,
          videoAvailabilityKnown: true,
          musicVideoType: "MUSIC_VIDEO_TYPE_ATV",
        }),
        false,
      ),
    ).toBe("unavailable");
  });
});
