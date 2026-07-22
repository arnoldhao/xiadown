import { describe, expect, test } from "bun:test";

import {
  listenOnlineItemFromPlaybackTrack,
  listenQueueStateFromPlaybackSnapshot,
  normalizeListenPlaybackSnapshot,
  type ListenPlaybackSnapshot,
} from "@/app/main/listen/playback-api";

function snapshot(overrides: Partial<ListenPlaybackSnapshot>): ListenPlaybackSnapshot {
  return {
    version: 0,
    queueIdentity: "",
    state: "paused",
    progress: 0,
    duration: 0,
    volume: 1,
    muted: false,
    shuffleEnabled: false,
    repeatMode: "off",
    queue: [],
    queueKind: "none",
    queueTitle: "",
    currentIndex: 0,
    showMiniPlayer: false,
    canUndoQueue: false,
    canRedoQueue: false,
    canAutoloadPending: false,
    ...overrides,
  };
}

describe("listen playback api adapter", () => {
  test("carries the active browse language into playback session starts", async () => {
    const source = await Bun.file(
      new URL("./playback-api.ts", import.meta.url),
    ).text();

    expect(source).toContain('callListenPlaybackSnapshotByName("SetPlaybackLanguage"');
    expect(source).toContain("language: options.language.trim()");
    expect(source).toContain("language: language.trim()");
  });

  test("sends playlist continuation anchors and queue identities in one atomic call", async () => {
    const source = await Bun.file(
      new URL("./playback-api.ts", import.meta.url),
    ).text();
    const start = source.indexOf(
      "export async function callListenPlaybackInsertAfterQueueItem",
    );
    const end = source.indexOf(
      "export async function callListenPlaybackAppendToQueue",
      start,
    );
    const adapter = source.slice(start, end);

    expect(adapter).toContain(
      'callListenPlaybackSnapshotByName("InsertAfterQueueItem"',
    );
    expect(adapter).toContain("anchorTrackId: options.anchorTrackId.trim()");
    expect(adapter).toContain(
      "expectedQueueIdentity: options.expectedQueueIdentity.trim()",
    );
  });

  test("uses the current radio item as the queue seed", () => {
    const queue = listenQueueStateFromPlaybackSnapshot(
      snapshot({
        queueKind: "radio",
        queueTitle: "Radio",
        currentIndex: 1,
        queue: [
          { id: "a", videoId: "a-video", title: "A", artist: "Artist" },
          { id: "b", videoId: "b-video", title: "B", artist: "Artist" },
        ],
      }),
    );

    expect(queue.kind).toBe("radio");
    expect(queue.kind === "radio" ? queue.seedVideoId : "").toBe("b-video");
  });

  test("keeps playback thumbnail URLs as PlayerService metadata", () => {
    const item = listenOnlineItemFromPlaybackTrack(
      {
        id: "a",
        videoId: "a-video",
        title: "A",
        artist: "Artist",
        thumbnailUrl: "https://example.com/art.jpg",
      },
      "http://127.0.0.1:5678/",
    );

    expect(item.thumbnailUrl).toBe("https://example.com/art.jpg");
  });

  test("keeps structured artists across playback queue mapping", () => {
    const item = listenOnlineItemFromPlaybackTrack({
      id: "a",
      videoId: "a-video",
      title: "A",
      artist: "Artist A, Artist B",
      artists: [
        { name: "Artist A", browseId: "UCartistA" },
        { name: "Artist B", browseId: "UCartistB" },
      ],
      artistBrowseId: "UCartistA",
      artistSource: "api-linked-multiple",
    });

    expect(item.artists).toEqual([
      { name: "Artist A", browseId: "UCartistA" },
      { name: "Artist B", browseId: "UCartistB" },
    ]);
  });

  test("normalizes lyrics time separately from playback progress", () => {
    const normalized = normalizeListenPlaybackSnapshot({
      progress: 12,
      currentTimeMs: 12_345,
    });

    expect(normalized?.progress).toBe(12);
    expect(normalized?.currentTimeMs).toBe(12345);
  });

  test("normalizes the server queue identity used by continuation CAS", () => {
    expect(
      normalizeListenPlaybackSnapshot({ queueIdentity: " client:queue-42 " })
        ?.queueIdentity,
    ).toBe("client:queue-42");
    expect(
      normalizeListenPlaybackSnapshot({ queueIdentity: 42 as never })
        ?.queueIdentity,
    ).toBe("");
  });

  test("normalizes observed audio quality without exposing auto", () => {
    expect(
      normalizeListenPlaybackSnapshot({
        observedPlaybackAudioQuality: "AUDIO_QUALITY_HIGH",
      })?.observedPlaybackAudioQuality,
    ).toBe("AUDIO_QUALITY_HIGH");
    expect(
      normalizeListenPlaybackSnapshot({
        observedPlaybackAudioQuality: "auto",
      })?.observedPlaybackAudioQuality,
    ).toBe("");
    expect(
      normalizeListenPlaybackSnapshot({
        observedPlaybackAudioQuality: "high",
      })?.observedPlaybackAudioQuality,
    ).toBe("");
  });
});
