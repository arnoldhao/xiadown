import { describe, expect, test } from "bun:test";

import {
  deriveListenPlaybackProjection,
  stabilizeListenPlaybackProjection,
} from "@/app/main/listen/playback-store";
import type { ListenPlaybackSnapshot } from "@/app/main/listen/playback-api";

function snapshot(overrides: Partial<ListenPlaybackSnapshot>): ListenPlaybackSnapshot {
  return {
    version: 1,
    state: "playing",
    progress: 0,
    duration: 180,
    volume: 1,
    muted: false,
    shuffleEnabled: false,
    repeatMode: "off",
    queue: [
      {
        id: "one",
        videoId: "video-one",
        title: "One",
        artist: "Artist",
        thumbnailUrl: "https://example.com/one.jpg",
        hasVideo: true,
        videoAvailabilityKnown: true,
      },
    ],
    queueKind: "playlist",
    queueTitle: "Queue",
    currentIndex: 0,
    showMiniPlayer: false,
    canUndoQueue: false,
    canRedoQueue: false,
    canAutoloadPending: true,
    ...overrides,
  };
}

describe("listen playback store projection", () => {
  test("keeps track and queue references stable for progress-only snapshots", () => {
    const previous = deriveListenPlaybackProjection(snapshot({ progress: 12 }));
    const next = stabilizeListenPlaybackProjection(
      previous,
      deriveListenPlaybackProjection(snapshot({ version: 2, progress: 13 })),
    );

    expect(next.progress.currentTime).toBe(13);
    expect(next.currentItem).toBe(previous.currentItem);
    expect(next.queueState).toBe(previous.queueState);
    expect(next.queueItems).toBe(previous.queueItems);
  });

  test("replaces track reference when authoritative metadata changes", () => {
    const previous = deriveListenPlaybackProjection(snapshot({ progress: 12 }));
    const next = stabilizeListenPlaybackProjection(
      previous,
      deriveListenPlaybackProjection(
        snapshot({
          version: 2,
          progress: 13,
          queue: [
            {
              id: "one",
              videoId: "video-one",
              title: "One (Remastered)",
              artist: "Artist",
              thumbnailUrl: "https://example.com/one.jpg",
              hasVideo: true,
              videoAvailabilityKnown: true,
            },
          ],
        }),
      ),
    );

    expect(next.currentItem).not.toBe(previous.currentItem);
    expect(next.currentItem?.title).toBe("One (Remastered)");
  });
});
