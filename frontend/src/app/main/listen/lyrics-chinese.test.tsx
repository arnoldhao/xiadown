import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";
import {
  buildListenLyricsChineseSearchTrackVariants,
  projectListenLyricsDisplayText,
  projectListenLyricsDataForLocale,
  projectListenLyricsTimelineForLocale,
  resolveListenLyricsChineseLocale,
} from "@/app/main/listen/lyrics-chinese";
import { ListenLyricsSurface } from "@/app/main/listen/lyrics";
import { buildListenLyricsTimeline } from "@/app/main/listen/lyrics-timeline";
import type { ListenLyricsData } from "@/app/main/listen/types";

const MIXED_MOUSE_TITLE = "\u540e\u53f0\u88e1\u7684\u9f20\u6807";
const TRADITIONAL_QUEEN = "\u7687\u540e\u6a02\u968a";
const MIXED_ALBUM = "\u539f\u58f0\u5c08\u8f2f";
const TRADITIONAL_MOUSE_TITLE = "\u5f8c\u81fa\u88e1\u7684\u9f20\u6a19";
const SIMPLIFIED_MOUSE_TITLE = "\u540e\u53f0\u91cc\u7684\u9f20\u6807";
const SIMPLIFIED_QUEEN = "\u7687\u540e\u4e50\u961f";
const SIMPLIFIED_BACKEND = "\u540e\u53f0";
const TRADITIONAL_BACKEND = "\u5f8c\u81fa";
const TRADITIONAL_MUSIC = "\u97f3\u6a02";
const QUEEN = "\u7687\u540e";
const JAPANESE_LINE = "\u4eca\u65e5\u306f\u5f8c\u3067\u4f1a\u3046";
const JAPANESE_TODAY = "\u4eca\u65e5\u306f";
const JAPANESE_MEET_KANJI = "\u4f1a";
const SINGER = "\u6b4c\u624b";
const JAPANESE_VIDEO_LINE = "\u52d5\u753b\u3092\u898b\u308b";
const JAPANESE_VIDEO_KANJI = "\u52d5\u753b";
const JAPANESE_MUSIC_KANJI = "\u97f3\u697d";
const HALFWIDTH_VIDEO = "\uff84\uff9e\uff73\uff76\uff9e";
const KATAKANA_EXTENSION_CHANNEL = "\u30ed\u30c3\u30af\u31f0";
const SIMPLIFIED_SOFTWARE_HAIR = "\u8f6f\u4ef6\u548c\u5934\u53d1";
const SIMPLIFIED_SOFTWARE = "\u8f6f\u4ef6";
const SIMPLIFIED_MOUSE = "\u9f20\u6807";
const TRADITIONAL_SOFTWARE_HAIR = "\u8edf\u4ef6\u548c\u982d\u9aee";
const TRADITIONAL_SOFTWARE = "\u8edf\u4ef6";
const TRADITIONAL_MOUSE = "\u9f20\u6a19";
const SIMPLIFIED_PHRASE = "\u5934\u53d1\u5e72\u676f\u4ee5\u540e\u518d\u51fa\u53d1";
const SIMPLIFIED_PHRASE_WORDS = [
  "\u5934",
  "\u53d1",
  "\u5e72",
  "\u676f",
  "\u4ee5\u540e",
  "\u518d",
  "\u51fa",
  "\u53d1",
];
const TRADITIONAL_PHRASE = "\u982d\u9aee\u4e7e\u676f\u4ee5\u5f8c\u518d\u51fa\u767c";
const TRADITIONAL_HAIR = "\u9aee";
const TRADITIONAL_DRY = "\u4e7e";
const TRADITIONAL_EMIT = "\u767c";
const SIMPLIFIED_SOFTWARE_BACKEND = "\u8f6f\u4ef6\u548c\u540e\u53f0";
const TRADITIONAL_SOFTWARE_BACKEND = "\u8edf\u4ef6\u548c\u5f8c\u81fa";
const SUPPLEMENTARY_PHRASE = "\u575b\u{2cd03}";
const TRADITIONAL_SUPPLEMENTARY_PHRASE = "\u7f48\u9a1e";

describe("listen lyrics Chinese normalization", () => {
  test("normalizes supported Chinese locale families without widening others", () => {
    expect(resolveListenLyricsChineseLocale("zh-CN")).toBe("zh-CN");
    expect(resolveListenLyricsChineseLocale("zh_Hans_SG")).toBe("zh-CN");
    expect(resolveListenLyricsChineseLocale("zh-TW")).toBe("zh-TW");
    expect(resolveListenLyricsChineseLocale("zh-Hant-HK")).toBe("zh-TW");
    expect(resolveListenLyricsChineseLocale("zh-MO")).toBe("zh-TW");
    expect(resolveListenLyricsChineseLocale("en")).toBeNull();
    expect(resolveListenLyricsChineseLocale(undefined)).toBeNull();
  });

  test("builds canonical, uniform, and opposite mixed-script track variants stably", () => {
    const canonical = {
      videoId: "video-one",
      title: MIXED_MOUSE_TITLE,
      artist: TRADITIONAL_QUEEN,
      album: MIXED_ALBUM,
      durationSeconds: 213,
    };
    const variants = buildListenLyricsChineseSearchTrackVariants(
      canonical,
      "zh-TW",
    );

    expect(variants).toEqual([
      canonical,
      {
        ...canonical,
        title: TRADITIONAL_MOUSE_TITLE,
        artist: TRADITIONAL_QUEEN,
      },
      {
        ...canonical,
        title: SIMPLIFIED_MOUSE_TITLE,
        artist: SIMPLIFIED_QUEEN,
      },
      {
        ...canonical,
        title: TRADITIONAL_MOUSE_TITLE,
        artist: SIMPLIFIED_QUEEN,
      },
    ]);
    expect(variants[0]).toBe(canonical);
    expect(variants.every((variant) => variant.album === canonical.album)).toBe(
      true,
    );
    expect(
      buildListenLyricsChineseSearchTrackVariants(canonical, "zh-CN"),
    ).toEqual([canonical, variants[2], variants[1], variants[3]]);
  });

  test("covers both mixed title and artist directions within four identities", () => {
    const canonical = {
      title: SIMPLIFIED_MOUSE_TITLE,
      artist: SIMPLIFIED_QUEEN,
    };

    expect(
      buildListenLyricsChineseSearchTrackVariants(canonical, "zh-TW"),
    ).toEqual([
      canonical,
      { title: TRADITIONAL_MOUSE_TITLE, artist: TRADITIONAL_QUEEN },
      { title: TRADITIONAL_MOUSE_TITLE, artist: SIMPLIFIED_QUEEN },
      { title: SIMPLIFIED_MOUSE_TITLE, artist: TRADITIONAL_QUEEN },
    ]);
  });

  test("does not expand non-Chinese identities or non-Chinese app locales", () => {
    const englishTrack = { title: "Golden Hour", artist: "Example Artist" };
    const chineseTrack = {
      title: SIMPLIFIED_BACKEND,
      channel: TRADITIONAL_QUEEN,
    };

    expect(
      buildListenLyricsChineseSearchTrackVariants(englishTrack, "zh-CN"),
    ).toEqual([englishTrack]);
    expect(
      buildListenLyricsChineseSearchTrackVariants(chineseTrack, "en"),
    ).toEqual([chineseTrack]);
    expect(
      buildListenLyricsChineseSearchTrackVariants(
        { title: TRADITIONAL_MUSIC, artist: QUEEN },
        "zh-TW",
      ),
    ).toHaveLength(2);
  });

  test("does not expand Japanese search identities when any identity field contains Kana", () => {
    const tracks = [
      { title: JAPANESE_LINE, artist: SINGER },
      { title: JAPANESE_VIDEO_LINE, artist: SINGER },
      { title: JAPANESE_MUSIC_KANJI, artist: HALFWIDTH_VIDEO },
      { title: JAPANESE_MUSIC_KANJI, channel: KATAKANA_EXTENSION_CHANNEL },
    ];

    for (const track of tracks) {
      expect(
        buildListenLyricsChineseSearchTrackVariants(track, "zh-TW"),
      ).toEqual([track]);
    }
  });

  test("projects every textual lyric layer without mutating identity or timing", () => {
    const lyrics: ListenLyricsData = {
      videoId: "video-one",
      kind: "synced",
      source: "test",
      providerId: "provider",
      providerTrackId: "track-id",
      timingQuality: "syllable",
      confidence: 92,
      text: SIMPLIFIED_MOUSE_TITLE,
      lines: [
        {
          startMs: 1_000,
          durationMs: 2_000,
          endEstimated: true,
          text: SIMPLIFIED_MOUSE_TITLE,
          translationText: SIMPLIFIED_SOFTWARE_HAIR,
          romanizedText: "hou tai li de shu biao",
          romanizedKind: "pinyin",
          alternateTexts: [
            {
              role: "translation",
              language: "zh-CN",
              text: SIMPLIFIED_SOFTWARE,
            },
            { role: "romanization", text: "ruan jian" },
          ],
          words: [
            {
              startMs: 1_000,
              endMs: 1_500,
              text: SIMPLIFIED_MOUSE,
              endsWithSpace: false,
              syllables: [
                {
                  startMs: 1_000,
                  endMs: 1_200,
                  text: SIMPLIFIED_SOFTWARE,
                },
              ],
            },
          ],
        },
      ],
    };
    const before = structuredClone(lyrics);
    const projected = projectListenLyricsDataForLocale(lyrics, "zh-TW");

    expect(projected).not.toBe(lyrics);
    expect(projected.text).toBe(TRADITIONAL_MOUSE_TITLE);
    expect(projected.lines[0]).toMatchObject({
      startMs: 1_000,
      durationMs: 2_000,
      endEstimated: true,
      text: TRADITIONAL_MOUSE_TITLE,
      translationText: TRADITIONAL_SOFTWARE_HAIR,
      romanizedText: "hou tai li de shu biao",
      romanizedKind: "pinyin",
    });
    expect(projected.lines[0]?.alternateTexts).toEqual([
      {
        role: "translation",
        language: "zh-CN",
        text: TRADITIONAL_SOFTWARE,
      },
      { role: "romanization", text: "ruan jian" },
    ]);
    expect(projected.lines[0]?.words?.[0]).toMatchObject({
      startMs: 1_000,
      endMs: 1_500,
      text: TRADITIONAL_MOUSE,
      endsWithSpace: false,
    });
    expect(projected.lines[0]?.words?.[0]?.syllables?.[0]?.text).toBe(
      TRADITIONAL_SOFTWARE,
    );
    expect(projected.videoId).toBe(lyrics.videoId);
    expect(projected.providerTrackId).toBe(lyrics.providerTrackId);
    expect(lyrics).toEqual(before);
    expect(projectListenLyricsDataForLocale(lyrics, "en")).toBe(lyrics);
  });

  test("projects renderer view models after canonical timeline construction", () => {
    const canonical = buildListenLyricsTimeline([
      {
        startMs: 1_000,
        durationMs: 2_000,
        text: SIMPLIFIED_MOUSE_TITLE,
        translationText: SIMPLIFIED_SOFTWARE,
        romanizedText: "hou tai",
        words: [
          { startMs: 1_000, endMs: 2_000, text: SIMPLIFIED_MOUSE },
        ],
      },
    ], { romanized: true });
    const projected = projectListenLyricsTimelineForLocale(
      canonical,
      "zh-TW",
    );

    expect(projected[0]).toMatchObject({
      sourceIndex: 0,
      startMs: 1_000,
      endMs: 3_000,
      text: TRADITIONAL_MOUSE_TITLE,
      translationText: TRADITIONAL_SOFTWARE,
      romanizedText: "hou tai",
    });
    expect(projected[0]?.words[0]).toMatchObject({
      startMs: 1_000,
      endMs: 2_000,
      text: TRADITIONAL_MOUSE,
    });
    expect(canonical[0]?.text).toBe(SIMPLIFIED_MOUSE_TITLE);
  });

  test("keeps OpenCC phrase disambiguation across timed word boundaries", () => {
    const canonical = buildListenLyricsTimeline([
      {
        startMs: 1_000,
        durationMs: 3_000,
        text: SIMPLIFIED_PHRASE,
        words: SIMPLIFIED_PHRASE_WORDS.map(
          (text, index) => ({
            startMs: 1_000 + index * 300,
            endMs: 1_300 + index * 300,
            text,
          }),
        ),
      },
    ]);
    const projected = projectListenLyricsTimelineForLocale(
      canonical,
      "zh-TW",
    );

    expect(projected[0]?.text).toBe(TRADITIONAL_PHRASE);
    expect(projected[0]?.words.map((word) => word.text).join("")).toBe(
      TRADITIONAL_PHRASE,
    );
    expect(projected[0]?.words[1]?.text).toBe(TRADITIONAL_HAIR);
    expect(projected[0]?.words[2]?.text).toBe(TRADITIONAL_DRY);
    expect(projected[0]?.words.at(-1)?.text).toBe(TRADITIONAL_EMIT);
  });

  test("keeps contextual word projection when UTF-16 lengths change", () => {
    const canonical = buildListenLyricsTimeline([
      {
        startMs: 1_000,
        durationMs: 2_000,
        text: SUPPLEMENTARY_PHRASE,
        words: Array.from(SUPPLEMENTARY_PHRASE).map((text, index) => ({
          startMs: 1_000 + index * 500,
          endMs: 1_500 + index * 500,
          text,
        })),
      },
    ]);
    const projected = projectListenLyricsTimelineForLocale(
      canonical,
      "zh-TW",
    );

    expect(projected[0]?.text).toBe(TRADITIONAL_SUPPLEMENTARY_PHRASE);
    expect(projected[0]?.words.map((word) => word.text).join("")).toBe(
      TRADITIONAL_SUPPLEMENTARY_PHRASE,
    );
  });

  test("protects Japanese primary lines, words, and translations independently", () => {
    const lyrics: ListenLyricsData = {
      videoId: "japanese",
      kind: "synced",
      source: "test",
      text: `${JAPANESE_LINE}\n${SIMPLIFIED_BACKEND}`,
      lines: [
        {
          startMs: 1_000,
          durationMs: 2_000,
          text: JAPANESE_LINE,
          translationText: SIMPLIFIED_SOFTWARE_BACKEND,
          words: [
            {
              startMs: 1_000,
              endMs: 2_000,
              text: JAPANESE_TODAY,
              syllables: [
                {
                  startMs: 1_000,
                  endMs: 1_500,
                  text: JAPANESE_MEET_KANJI,
                },
              ],
            },
          ],
        },
        {
          startMs: 3_000,
          durationMs: 2_000,
          text: SIMPLIFIED_BACKEND,
          translationText: JAPANESE_VIDEO_LINE,
          words: [
            { startMs: 3_000, endMs: 5_000, text: SIMPLIFIED_BACKEND },
          ],
        },
      ],
    };
    const projected = projectListenLyricsDataForLocale(lyrics, "zh-TW");

    expect(projected.text).toBe(`${JAPANESE_LINE}\n${TRADITIONAL_BACKEND}`);
    expect(projected.lines[0]?.text).toBe(JAPANESE_LINE);
    expect(projected.lines[0]?.translationText).toBe(
      TRADITIONAL_SOFTWARE_BACKEND,
    );
    expect(projected.lines[0]?.words).toEqual(lyrics.lines[0]?.words);
    expect(projected.lines[0]?.words?.[0]?.syllables?.[0]?.text).toBe(
      JAPANESE_MEET_KANJI,
    );
    expect(projected.lines[1]?.text).toBe(TRADITIONAL_BACKEND);
    expect(projected.lines[1]?.translationText).toBe(JAPANESE_VIDEO_LINE);
    expect(projected.lines[1]?.words?.[0]?.text).toBe(TRADITIONAL_BACKEND);
    expect(projectListenLyricsDisplayText(JAPANESE_LINE, "zh-TW")).toBe(
      JAPANESE_LINE,
    );
  });

  test("protects Japanese timeline text and words while converting an independent Chinese translation", () => {
    const canonical = buildListenLyricsTimeline([
      {
        startMs: 1_000,
        durationMs: 2_000,
        text: JAPANESE_LINE,
        translationText: SIMPLIFIED_SOFTWARE_BACKEND,
        words: [
          {
            startMs: 1_000,
            endMs: 2_000,
            text: JAPANESE_MEET_KANJI,
            syllables: [
              {
                startMs: 1_000,
                endMs: 1_500,
                text: JAPANESE_MEET_KANJI,
              },
            ],
          },
        ],
      },
    ]);
    const projected = projectListenLyricsTimelineForLocale(
      canonical,
      "zh-TW",
    );

    expect(projected[0]?.text).toBe(JAPANESE_LINE);
    expect(projected[0]?.translationText).toBe(
      TRADITIONAL_SOFTWARE_BACKEND,
    );
    expect(projected[0]?.words).toEqual(canonical[0]?.words);
    expect(projected[0]?.words[0]?.syllables?.[0]?.text).toBe(
      JAPANESE_MEET_KANJI,
    );
  });

  test("protects explicit non-Chinese and Kana alternate text while retaining Chinese projection", () => {
    const lyrics: ListenLyricsData = {
      videoId: "alternate-text",
      kind: "plain",
      source: "test",
      text: SIMPLIFIED_BACKEND,
      lines: [
        {
          startMs: 0,
          durationMs: 0,
          text: SIMPLIFIED_BACKEND,
          alternateTexts: [
            {
              role: "translation",
              language: "ja",
              text: JAPANESE_VIDEO_KANJI,
            },
            { role: "translation", text: JAPANESE_LINE },
            { role: "translation", text: SIMPLIFIED_BACKEND },
            {
              role: "translation",
              language: "zh-CN",
              text: SIMPLIFIED_SOFTWARE,
            },
            { role: "romanization", text: "hou tai" },
          ],
        },
      ],
    };

    expect(
      projectListenLyricsDataForLocale(lyrics, "zh-TW").lines[0]
        ?.alternateTexts,
    ).toEqual([
      {
        role: "translation",
        language: "ja",
        text: JAPANESE_VIDEO_KANJI,
      },
      { role: "translation", text: JAPANESE_LINE },
      { role: "translation", text: TRADITIONAL_BACKEND },
      {
        role: "translation",
        language: "zh-CN",
        text: TRADITIONAL_SOFTWARE,
      },
      { role: "romanization", text: "hou tai" },
    ]);
  });

  test("applies locale projection at the shared surface for plain and synced previews", () => {
    const traditionalText = getXiaText("zh-TW");
    const simplifiedText = getXiaText("zh-CN");
    const plainLyrics: ListenLyricsData = {
      videoId: "plain",
      kind: "plain",
      source: "test",
      text: SIMPLIFIED_MOUSE_TITLE,
      lines: [
        {
          startMs: 0,
          durationMs: 0,
          text: SIMPLIFIED_MOUSE_TITLE,
          translationText: SIMPLIFIED_SOFTWARE,
          romanizedText: "hou tai li de shu biao",
        },
      ],
    };
    const syncedLyrics: ListenLyricsData = {
      ...plainLyrics,
      videoId: "synced",
      kind: "synced",
      timingQuality: "word",
      lines: [
        {
          ...plainLyrics.lines[0],
          startMs: 1_000,
          durationMs: 2_000,
          words: [
            { startMs: 1_000, endMs: 2_000, text: SIMPLIFIED_MOUSE },
          ],
        },
      ],
    };

    const traditionalPlainMarkup = renderToStaticMarkup(
      <ListenLyricsSurface
        text={traditionalText}
        lyrics={plainLyrics}
        romanized
      />,
    );
    const traditionalSyncedMarkup = renderToStaticMarkup(
      <ListenLyricsSurface
        text={traditionalText}
        lyrics={syncedLyrics}
        currentTimeMs={1_500}
      />,
    );
    const simplifiedMarkup = renderToStaticMarkup(
      <ListenLyricsSurface
        text={simplifiedText}
        lyrics={projectListenLyricsDataForLocale(syncedLyrics, "zh-TW")}
        currentTimeMs={1_500}
      />,
    );

    expect(traditionalPlainMarkup).toContain(TRADITIONAL_MOUSE_TITLE);
    expect(traditionalPlainMarkup).toContain(TRADITIONAL_SOFTWARE);
    expect(traditionalPlainMarkup).toContain("hou tai li de shu biao");
    expect(traditionalSyncedMarkup).toContain(TRADITIONAL_MOUSE);
    expect(simplifiedMarkup).toContain(SIMPLIFIED_MOUSE);
    expect(simplifiedMarkup).toContain(SIMPLIFIED_SOFTWARE);
    expect(plainLyrics.text).toBe(SIMPLIFIED_MOUSE_TITLE);
  });

  test("preserves explicit Japanese pure-Han translations on every surface", () => {
    const text = getXiaText("zh-CN");
    const plainLyrics: ListenLyricsData = {
      videoId: "plain-japanese-translation",
      kind: "plain",
      source: "test",
      text: TRADITIONAL_BACKEND,
      lines: [
        {
          startMs: 0,
          durationMs: 0,
          text: TRADITIONAL_BACKEND,
          alternateTexts: [
            {
              role: "translation",
              language: "ja",
              text: JAPANESE_VIDEO_KANJI,
            },
          ],
        },
      ],
    };
    const syncedLyrics: ListenLyricsData = {
      ...plainLyrics,
      videoId: "synced-japanese-translation",
      kind: "synced",
      lines: [
        {
          ...plainLyrics.lines[0],
          startMs: 1_000,
          durationMs: 2_000,
        },
      ],
    };

    const plainMarkup = renderToStaticMarkup(
      <ListenLyricsSurface text={text} lyrics={plainLyrics} />,
    );
    const syncedMarkup = renderToStaticMarkup(
      <ListenLyricsSurface
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1_500}
      />,
    );

    expect(plainMarkup).toContain(JAPANESE_VIDEO_KANJI);
    expect(syncedMarkup).toContain(JAPANESE_VIDEO_KANJI);
  });
});
