import { describe, expect, test } from "bun:test";

import type { ListenOnlineItem } from "./types";
import { listenArtistBrowseTrack } from "./artist-navigation";

const track: ListenOnlineItem = {
  id: "track-1",
  group: "playlist",
  videoId: "video-1",
  title: "Track",
  channel: "One, Two",
  artists: [
    { name: "One", browseId: "UC1" },
    { name: "Two", browseId: "UC2", thumbnailUrl: "two.jpg" },
  ],
  artistSource: "api-linked-multiple",
  description: "",
  durationLabel: "3:00",
};

describe("listen artist navigation", () => {
  test("prefers the selected artist browse id over a combined track label", () => {
    const result = listenArtistBrowseTrack(
      track,
      { name: "Two", browseId: "UC2" },
      [
        { kind: "artist", text: "One" },
        { kind: "separator", text: ", " },
        { kind: "artist", text: "Two" },
      ],
    );

    expect(result?.channel).toBe("Two");
    expect(result?.artistBrowseId).toBe("UC2");
    expect(result?.artists).toEqual([
      { name: "Two", browseId: "UC2", thumbnailUrl: "two.jpg" },
    ]);
  });

  test("preserves a command browse id even before track metadata contains it", () => {
    const result = listenArtistBrowseTrack(
      { ...track, artists: undefined },
      { name: "Three", browseId: "UC3", thumbnailUrl: "three.jpg" },
      [{ kind: "artist", text: "Three" }],
    );

    expect(result?.artistBrowseId).toBe("UC3");
    expect(result?.artistSource).toBe("api-linked");
    expect(result?.thumbnailUrl).toBe("three.jpg");
  });
});
