import { describe, expect, test } from "bun:test";

import { hasTrustedListenOnlineArtist,listenArtistCountFromLabelParts,resolveListenTrackVideoAvailability,resolveTrustedListenOnlineArtistLabel,splitListenArtistLabel } from "@/app/main/listen/playback-helpers";
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

describe("listen playback artist provenance", () => {
  test("does not trust plain API text recommendation labels as artists", () => {
    const track = item({ channel: "Made for", artistSource: "api-text" });

    expect(hasTrustedListenOnlineArtist(track)).toBe(false);
    expect(resolveTrustedListenOnlineArtistLabel(track)).toBe("");
  });

  test("trusts linked API artists", () => {
    const track = item({
      channel: "Resolved Artist",
      artistBrowseId: "UCresolved",
      artistSource: "api-linked",
    });

    expect(hasTrustedListenOnlineArtist(track)).toBe(true);
    expect(resolveTrustedListenOnlineArtistLabel(track)).toBe("Resolved Artist");
  });

  test("trusts linked multi-artist labels", () => {
    const track = item({
      channel: "Artist A, Artist B",
      artistBrowseId: "UCfirst",
      artistSource: "api-linked-multiple",
    });

    expect(hasTrustedListenOnlineArtist(track)).toBe(true);
    expect(resolveTrustedListenOnlineArtistLabel(track)).toBe("Artist A, Artist B");
  });

  test("trusts artists resolved by backend track metadata", () => {
    const track = item({
      channel: "Accusefive",
      artistSource: "api-metadata",
    });

    expect(hasTrustedListenOnlineArtist(track)).toBe(true);
    expect(resolveTrustedListenOnlineArtistLabel(track)).toBe("Accusefive");
  });

  test("splits multi-artist labels without losing separators", () => {
    const parts = splitListenArtistLabel("Artist A、Artist B feat. Artist C");

    expect(parts).toEqual([
      { kind: "artist", text: "Artist A" },
      { kind: "separator", text: "、" },
      { kind: "artist", text: "Artist B" },
      { kind: "separator", text: " feat. " },
      { kind: "artist", text: "Artist C" },
    ]);
    expect(listenArtistCountFromLabelParts(parts)).toBe(3);
  });
});
