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
  test("does not treat audio endpoint musicVideoType as confirmed no video", () => {
    expect(
      resolveListenTrackVideoAvailability(
        item({ musicVideoType: "MUSIC_VIDEO_TYPE_ATV" }),
        false,
      ),
    ).toBe("checking");
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
