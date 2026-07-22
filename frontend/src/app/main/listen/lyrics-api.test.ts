import { describe, expect, test } from "bun:test";

import {
  listenLyricsSearchVariantPayloads,
  normalizeListenLyricsCandidates,
  normalizeListenLyricsSnapshot,
  resolveListenLyricsOnlineArtist,
} from "@/app/main/listen/lyrics-api";

const MIXED_CHINESE_TITLE = "\u540e\u53f0\u88e1\u7684\u97f3\u4e50";
const TRADITIONAL_ARTIST = "\u5468\u6770\u502b";
const TRADITIONAL_ALBUM = "\u539f\u8072\u5c08\u8f2f";
const SIMPLIFIED_TITLE = "\u540e\u53f0\u91cc\u7684\u97f3\u4e50";
const SIMPLIFIED_ARTIST = "\u5468\u6770\u4f26";
const TRADITIONAL_TITLE = "\u5f8c\u81fa\u88e1\u7684\u97f3\u6a02";

describe("listen lyrics api adapter", () => {
  test("sends only deduplicated alternate Chinese search identities", () => {
    expect(
      listenLyricsSearchVariantPayloads(
        {
          title: MIXED_CHINESE_TITLE,
          channel: TRADITIONAL_ARTIST,
          album: TRADITIONAL_ALBUM,
        },
        "zh-CN",
      ),
    ).toEqual([
      { title: SIMPLIFIED_TITLE, artist: SIMPLIFIED_ARTIST },
      { title: TRADITIONAL_TITLE, artist: TRADITIONAL_ARTIST },
      { title: TRADITIONAL_TITLE, artist: SIMPLIFIED_ARTIST },
    ]);
    expect(
      listenLyricsSearchVariantPayloads(
        { title: "Golden Hour", artist: "Example Artist" },
        "zh-CN",
      ),
    ).toEqual([]);
  });

  test("prefers linked artist metadata when the legacy channel is empty", () => {
    expect(
      resolveListenLyricsOnlineArtist({
        channel: "",
        artists: [
          { name: "AGA", browseId: "UC-aga" },
          { name: "AGA" },
          { name: "Guest", browseId: "UC-guest" },
        ],
      }),
    ).toBe("AGA, Guest");
    expect(
      resolveListenLyricsOnlineArtist({ channel: "Fallback Artist" }),
    ).toBe("Fallback Artist");
  });

  test("normalizes lyrics snapshots from the lyrics service", () => {
    const normalized = normalizeListenLyricsSnapshot({
      videoId: "a-video",
      kind: "plain",
      source: "test",
      providerId: " lrclib ",
      providerTrackId: " 42 ",
      attribution: " LRCLIB contributors ",
      timingQuality: "word",
      confidence: 105.4,
      text: "hello",
      errorCode: "lyrics_timeout",
      retryable: true,
      lines: [
        {
          startMs: 100,
          durationMs: 600,
          text: "hello world",
          translationText: " translated world ",
          alternateTexts: [
            { role: "translation", language: " en ", text: " translated world " },
            { role: "", text: "ignored" },
          ],
          words: [
            { startMs: 100, endMs: 350, text: "hello", endsWithSpace: true },
            {
              startMs: 350,
              endMs: 700,
              text: "world",
              endsWithSpace: false,
              syllables: [
                { startMs: 350, endMs: 500, text: "wor", endsWithSpace: false },
              ],
            },
          ],
        },
      ],
      loading: false,
    });

    expect(normalized?.videoId).toBe("a-video");
    expect(normalized?.kind).toBe("plain");
    expect(normalized?.text).toBe("hello");
    expect(normalized?.errorCode).toBe("lyrics_timeout");
    expect(normalized?.retryable).toBe(true);
    expect(normalized?.providerId).toBe("lrclib");
    expect(normalized?.providerTrackId).toBe("42");
    expect(normalized?.attribution).toBe("LRCLIB contributors");
    expect(normalized?.timingQuality).toBe("word");
    expect(normalized?.confidence).toBe(100);
    expect(normalized?.lines[0]?.translationText).toBe("translated world");
    expect(normalized?.lines[0]?.alternateTexts).toEqual([
      { role: "translation", language: "en", text: "translated world" },
    ]);
    expect(normalized?.lines[0]?.words?.[0]).toEqual({
      startMs: 100,
      endMs: 350,
      text: "hello",
      endsWithSpace: true,
    });
    expect(normalized?.lines[0]?.words?.[1]?.endsWithSpace).toBe(false);
    expect(normalized?.lines[0]?.words?.[1]?.syllables?.[0]?.text).toBe("wor");
  });

  test("normalizes candidate evidence and rejects malformed identities", () => {
    const candidates = normalizeListenLyricsCandidates({
      data: [
        {
          providerId: " LRCLIB ",
          providerTrackId: " 42 ",
          title: " Song ",
          artist: " Artist ",
          timingQuality: "line",
          confidence: 101,
          titleScore: 98.4,
          artistScore: 95,
          albumScore: -1,
          durationScore: 88,
          durationDiff: 1.2,
          accepted: true,
        },
        { providerId: "", providerTrackId: "missing" },
      ],
    });

    expect(candidates).toHaveLength(1);
    expect(candidates[0]).toMatchObject({
      providerId: "lrclib",
      providerTrackId: "42",
      title: "Song",
      artist: "Artist",
      timingQuality: "line",
      confidence: 100,
      titleScore: 98,
      albumScore: 0,
      accepted: true,
    });
  });

});
