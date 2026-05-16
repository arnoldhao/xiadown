import { describe, expect, test } from "bun:test";

import {
  buildListenLyricsTimeline,
  findListenActiveLyricLineIndex,
  getListenActiveLyricWordProgress,
} from "@/app/main/listen/lyrics";

describe("listen lyrics view helpers", () => {
  test("builds active synced lyric windows with romanized text", () => {
    const timeline = buildListenLyricsTimeline(
      [
        {
          startMs: 1000,
          durationMs: 0,
          text: "첫 눈",
          romanizedKind: "romanized",
          romanizedText: "cheot nun",
        },
        { startMs: 2600, durationMs: 700, text: "again" },
      ],
      true,
    );

    expect(timeline[0].endMs).toBe(2600);
    expect(timeline[0].romanizedText).toBe("cheot nun");
    expect(findListenActiveLyricLineIndex(timeline, 700)).toBe(-1);
    expect(findListenActiveLyricLineIndex(timeline, 900)).toBe(0);
    expect(findListenActiveLyricLineIndex(timeline, 2850)).toBe(1);
  });

  test("uses provided romanized text", () => {
    const timeline = buildListenLyricsTimeline(
      [
        {
          startMs: 0,
          durationMs: 1200,
          text: "きみ",
          romanizedKind: "romanized",
          romanizedText: "kimi",
        },
      ],
      true,
    );

    expect(timeline[0].romanizedText).toBe("kimi");
  });

  test("does not synthesize missing romanized text in the frontend", () => {
    const timeline = buildListenLyricsTimeline(
      [{ startMs: 0, durationMs: 1200, text: "アイラブユー" }],
      true,
    );

    expect(timeline[0].romanizedText).toBe("");
  });

  test("separates pinyin display from romanized lyrics", () => {
    const lines = [
      {
        startMs: 0,
        durationMs: 1200,
        text: "\u4f60\u597d",
        romanizedKind: "pinyin" as const,
        romanizedText: "ni hao",
      },
    ];

    expect(
      buildListenLyricsTimeline(lines, { romanized: true, pinyin: false })[0]
        .romanizedText,
    ).toBe("");
    expect(
      buildListenLyricsTimeline(lines, { romanized: false, pinyin: true })[0]
        .romanizedText,
    ).toBe("ni hao");
  });

  test("tracks karaoke word fill progress", () => {
    const progress = getListenActiveLyricWordProgress(
      [
        { startMs: 0, text: "one" },
        { startMs: 1000, text: "two" },
      ],
      500,
      0,
      2000,
    );

    expect(progress.index).toBe(0);
    expect(progress.progress).toBeCloseTo(0.56, 2);
  });
});
