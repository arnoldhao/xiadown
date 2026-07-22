import { describe, expect, test } from "bun:test";

import {
  applyListenLocalMetadataCandidate,
  isListenLocalMetadataCommittedIndexStale,
  LISTEN_LOCAL_METADATA_INDEX_STALE_CODE,
  ListenLocalMetadataUpdateError,
  localMetadataCandidateTrack,
  parseListenLocalMetadataNumber,
  selectListenLocalMetadataCandidate,
  updateListenLocalTrackMetadata,
} from "./local-metadata";
import {
  fetchListenLyricsCached,
  forgetListenLyricsCacheVariants,
  readListenLyricsCache,
} from "./playback-helpers";
import type { ListenLocalItem, ListenLyricsCandidate } from "./types";

const titleIKnow = "\u6211\u77e5\u9053";
const titleSunnyDay = "\u6674\u5929";
const artistJayChou = "\u5468\u6770\u4f26";
const albumMinor = "16\u672a\u6210\u5e74";

function localTrack(overrides: Partial<ListenLocalItem> = {}): ListenLocalItem {
  return {
    id: "track-1",
    title: titleIKnow,
    author: "By2 - Topic",
    album: "",
    albumArtist: "",
    genre: "",
    trackNumber: 0,
    discNumber: 0,
    year: 0,
    lyricsTitle: titleIKnow,
    lyricsArtist: "By2 - Topic",
    path: `/music/${titleIKnow}.m4a`,
    previewURL: "asset://track",
    durationLabel: "4:06",
    durationSeconds: 246,
    coverURL: "",
    format: "m4a",
    audioCodec: "aac",
    sizeBytes: 1024,
    metadataWritable: true,
    playbackSupported: true,
    playbackUnsupportedReason: "",
    probeError: "",
    modTimeUnix: 1,
    createdAtUnix: 1,
    ...overrides,
  };
}

function candidate(
  overrides: Partial<ListenLyricsCandidate> = {},
): ListenLyricsCandidate {
  return {
    providerId: "lrclib",
    providerTrackId: "42",
    title: titleIKnow,
    artist: "By2",
    album: albumMinor,
    confidence: 96,
    titleScore: 100,
    artistScore: 100,
    albumScore: 0,
    durationScore: 95,
    accepted: true,
    ...overrides,
  };
}

describe("local metadata completion", () => {
  test("builds a duration-backed identity request from the editable draft", () => {
    const result = localMetadataCandidateTrack(localTrack(), {
      title: titleIKnow,
      author: "By2 - Topic",
      album: "",
      albumArtist: "",
      genre: "",
      trackNumber: 0,
      discNumber: 0,
      year: 0,
    });
    expect(result).toMatchObject({
      title: titleIKnow,
      artist: "By2 - Topic",
      durationSeconds: 246,
    });
  });

  test("uses the parsed filename identity when artist tags are missing", () => {
    const result = localMetadataCandidateTrack(
      localTrack({
        title: "Artist - Title",
        author: "",
        lyricsTitle: "Title",
        lyricsArtist: "Artist",
        path: "/music/Artist - Title.flac",
      }),
      {
        title: "Artist - Title",
        author: "",
        album: "",
        albumArtist: "",
        genre: "",
        trackNumber: 0,
        discNumber: 0,
        year: 0,
      },
    );
    expect(result).toMatchObject({ title: "Title", artist: "Artist" });
  });

  test("keeps an edited title and never replaces an existing artist", () => {
    const result = localMetadataCandidateTrack(
      localTrack({
        title: "Artist - Title",
        author: "Known Artist",
        lyricsTitle: "Title",
        lyricsArtist: "Known Artist",
      }),
      {
        title: "User title",
        author: "Draft Artist",
        album: "",
        albumArtist: "",
        genre: "",
        trackNumber: 0,
        discNumber: 0,
        year: 0,
      },
    );
    expect(result).toMatchObject({
      title: "User title",
      artist: "Draft Artist",
    });
  });

  test("only selects an accepted candidate with corroborating evidence", () => {
    expect(
      selectListenLocalMetadataCandidate([
        candidate({
          providerTrackId: "title-only",
          artistScore: 0,
          albumScore: 0,
          durationScore: 0,
        }),
        candidate(),
      ])?.providerTrackId,
    ).toBe("42");
    expect(
      selectListenLocalMetadataCandidate([
        candidate({ accepted: false, confidence: 99 }),
      ]),
    ).toBeNull();
  });

  test("allows an accepted cross-script artist match only with duration evidence", () => {
    const crossScript = candidate({
      title: titleSunnyDay,
      artist: artistJayChou,
      artistScore: 0,
      albumScore: 0,
      durationScore: 100,
      accepted: true,
    });
    expect(selectListenLocalMetadataCandidate([crossScript])).toBe(crossScript);
    expect(
      selectListenLocalMetadataCandidate([
        { ...crossScript, durationScore: 0 },
      ]),
    ).toBeNull();
  });

  test("applies canonical identity without erasing advanced tags", () => {
    const result = applyListenLocalMetadataCandidate(
      {
        title: titleIKnow,
        author: "By2 - Topic",
        album: "",
        albumArtist: "Various Artists",
        genre: "Pop",
        trackNumber: 3,
        discNumber: 1,
        year: 2008,
      },
      candidate(),
    );
    expect(result).toEqual({
      title: titleIKnow,
      author: "By2",
      album: albumMinor,
      albumArtist: "Various Artists",
      genre: "Pop",
      trackNumber: 3,
      discNumber: 1,
      year: 2008,
    });
  });

  test("cleans a Topic-derived album artist without overwriting an independent credit", () => {
    expect(
      applyListenLocalMetadataCandidate(
        {
          title: titleIKnow,
          author: "By2 - Topic",
          album: "",
          albumArtist: "By2 - Topic",
          genre: "",
          trackNumber: 0,
          discNumber: 0,
          year: 0,
        },
        candidate(),
      ).albumArtist,
    ).toBe("By2");

    expect(
      applyListenLocalMetadataCandidate(
        {
          title: titleIKnow,
          author: "By2 - Topic",
          album: "",
          albumArtist: "Various Artists",
          genre: "",
          trackNumber: 0,
          discNumber: 0,
          year: 0,
        },
        candidate(),
      ).albumArtist,
    ).toBe("Various Artists");
  });

  test("normalizes numeric inputs to supported tag ranges", () => {
    expect(parseListenLocalMetadataNumber(" 12 ")).toBe(12);
    expect(parseListenLocalMetadataNumber("-1")).toBe(0);
    expect(parseListenLocalMetadataNumber("not-a-number")).toBe(0);
    expect(parseListenLocalMetadataNumber("12000")).toBe(9999);
  });
});

describe("local metadata update errors", () => {
  test("exposes a committed-index-stale 503 as a typed terminal error", async () => {
    const originalFetch = globalThis.fetch;
    const lyricsID = "local:track-1";
    let fetchCalls = 0;
    globalThis.fetch = (async () => {
      fetchCalls += 1;
      if (fetchCalls === 1) {
        return new Response(
          JSON.stringify({
            videoId: lyricsID,
            kind: "plain",
            source: "test",
            text: "old cached lyrics",
            lines: [
              { startMs: 0, durationMs: 0, text: "old cached lyrics" },
            ],
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        );
      }
      return new Response(
        JSON.stringify({
          error: {
            code: LISTEN_LOCAL_METADATA_INDEX_STALE_CODE,
            message:
              "metadata was written, but the local index could not be refreshed",
          },
        }),
        {
          status: 503,
          headers: { "Content-Type": "application/json" },
        },
      );
    }) as typeof fetch;

    try {
      await fetchListenLyricsCached(
        "http://127.0.0.1:34115",
        {
          lyricsId: lyricsID,
          title: titleIKnow,
          artist: "By2",
          localPath: `/music/${titleIKnow}.m4a`,
        },
        246,
        "en",
        { synced: true },
      );
      expect(readListenLyricsCache(lyricsID, "en", { synced: true })).not
        .toBeNull();

      let caught: unknown;
      try {
        await updateListenLocalTrackMetadata({
          httpBaseURL: "http://127.0.0.1:34115",
          track: localTrack(),
          draft: {
            title: titleIKnow,
            author: "By2",
            album: albumMinor,
            albumArtist: "By2",
            genre: "Pop",
            trackNumber: 1,
            discNumber: 1,
            year: 2008,
          },
        });
      } catch (error) {
        caught = error;
      }

      expect(caught).toBeInstanceOf(ListenLocalMetadataUpdateError);
      if (!(caught instanceof ListenLocalMetadataUpdateError)) {
        throw new Error("expected a typed local metadata update error");
      }
      expect(caught.status).toBe(503);
      expect(caught.code).toBe(LISTEN_LOCAL_METADATA_INDEX_STALE_CODE);
      expect(caught.message).toBe(
        "metadata was written, but the local index could not be refreshed",
      );
      expect(isListenLocalMetadataCommittedIndexStale(caught)).toBeTrue();
      expect(readListenLyricsCache(lyricsID, "en", { synced: true })).toBeNull();
    } finally {
      globalThis.fetch = originalFetch;
      forgetListenLyricsCacheVariants(lyricsID);
    }
  });

  test("preserves a backend error code on other metadata failures", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          error: {
            code: "metadata_file_changed",
            message: "the source file changed",
          },
        }),
        {
          status: 409,
          headers: { "Content-Type": "application/json" },
        },
      )) as typeof fetch;

    try {
      let caught: unknown;
      try {
        await updateListenLocalTrackMetadata({
          httpBaseURL: "http://127.0.0.1:34115",
          track: localTrack(),
          draft: {
            title: titleIKnow,
            author: "By2",
            album: "",
            albumArtist: "",
            genre: "",
            trackNumber: 0,
            discNumber: 0,
            year: 0,
          },
        });
      } catch (error) {
        caught = error;
      }

      expect(caught).toBeInstanceOf(ListenLocalMetadataUpdateError);
      if (!(caught instanceof ListenLocalMetadataUpdateError)) {
        throw new Error("expected a typed local metadata update error");
      }
      expect(caught.status).toBe(409);
      expect(caught.code).toBe("metadata_file_changed");
      expect(caught.message).toBe("the source file changed");
      expect(isListenLocalMetadataCommittedIndexStale(caught)).toBeFalse();
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("does not treat a generic 503 as proof that the file was committed", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({ error: { message: "local library unavailable" } }),
        {
          status: 503,
          headers: { "Content-Type": "application/json" },
        },
      )) as typeof fetch;

    try {
      let caught: unknown;
      try {
        await updateListenLocalTrackMetadata({
          httpBaseURL: "http://127.0.0.1:34115",
          track: localTrack(),
          draft: {
            title: titleIKnow,
            author: "By2",
            album: "",
            albumArtist: "",
            genre: "",
            trackNumber: 0,
            discNumber: 0,
            year: 0,
          },
        });
      } catch (error) {
        caught = error;
      }

      expect(caught).toBeInstanceOf(ListenLocalMetadataUpdateError);
      if (!(caught instanceof ListenLocalMetadataUpdateError)) {
        throw new Error("expected a typed local metadata update error");
      }
      expect(caught.status).toBe(503);
      expect(caught.code).toBe("local_metadata_update_failed");
      expect(isListenLocalMetadataCommittedIndexStale(caught)).toBeFalse();
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("locks the editor after a committed write and keeps close available", async () => {
    const source = await Bun.file(
      new URL("./LocalMetadataEditor.tsx", import.meta.url),
    ).text();
    expect(source).toContain('setMessage("committed-index-stale")');
    expect(source).toContain("setCommittedIndexStale(true)");
    expect(source).toContain(
      "!track?.metadataWritable || saving || committedIndexStale",
    );
    expect(source).toContain("disabled={saving || committedIndexStale}");
    expect(source).toContain("await props.onSaved(targetTrack)");
    expect(source).toContain("disabled={saving}");
  });

  test("uses the app locale when matching lyrics-backed metadata", async () => {
    const source = await Bun.file(
      new URL("./LocalMetadataEditor.tsx", import.meta.url),
    ).text();
    expect(source).toContain("language: props.text.locale");
  });
});
