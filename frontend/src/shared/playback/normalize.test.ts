import { beforeEach, describe, expect, test } from "bun:test";

import {
  normalizePlaybackSnapshot,
  playbackSessionByID,
} from "@/shared/playback/normalize";
import { usePlaybackCoordinatorStore } from "@/shared/playback/store";

describe("playback coordinator frontend contract", () => {
  beforeEach(() => usePlaybackCoordinatorStore.getState().reset());

  test("normalizes Wails event envelopes and both focus sessions", () => {
    const snapshot = normalizePlaybackSnapshot({
      data: {
        version: 12,
        audibleSessionId: "preview:file-2",
        active: {
          id: "preview:file-2",
          focus: "transient_preview",
          state: "playing",
          item: {
            id: "file-2",
            kind: "video",
            source: { provider: "local", uri: "/tmp/video.mp4" },
            title: "Video",
          },
          capabilities: { available: true, playPause: true, seek: true },
          position: 4.5,
          duration: 30,
          volume: 0.7,
          queue: [],
        },
        suspendedPersistent: {
          id: "music:track-1",
          focus: "persistent",
          state: "paused",
          item: {
            id: "track-1",
            kind: "audio",
            source: { provider: "local", uri: "/tmp/song.mp3" },
            title: "Song",
          },
          capabilities: { available: true },
          position: 10,
          duration: 100,
          volume: 1,
          queue: [],
        },
      },
    });

    expect(snapshot.version).toBe(12);
    expect(snapshot.active?.focus).toBe("transient_preview");
    expect(snapshot.active?.item.kind).toBe("video");
    expect(playbackSessionByID(snapshot, "music:track-1")?.position).toBe(10);
  });

  test("does not replace a newer event with a stale command response", () => {
    const store = usePlaybackCoordinatorStore.getState();
    store.applySnapshot({ version: 9, active: null });
    store.applySnapshot({ version: 4, active: null });
    expect(usePlaybackCoordinatorStore.getState().snapshot.version).toBe(9);
  });
});
