import { describe, expect, test } from "bun:test";

import {
  buildListenLyricsFocusTimingUnits,
  buildListenLyricsTimeline,
  buildListenLyricsTimelineKey,
  expandListenLyricTimingUnits,
  findListenActiveLyricLineIndex,
  getListenActiveLyricWordProgress,
  getListenLyricTimingUnitDisplayText,
  getListenLyricWordDisplayText,
  getListenLyricWordVisualState,
  resolveListenLyricsFocusFrame,
} from "@/app/main/listen/lyrics";

describe("listen lyrics view helpers", () => {
  test("changes the timeline key when a same-track lyric version changes", () => {
    const base = {
      videoId: "same-track",
      kind: "synced" as const,
      source: "LRCLib",
      providerId: "lrclib",
      timingQuality: "line" as const,
      text: "Line",
      lines: [{ startMs: 1000, durationMs: 1000, text: "Line" }],
    };
    const first = buildListenLyricsTimelineKey({
      ...base,
      providerTrackId: "41",
    });
    const second = buildListenLyricsTimelineKey({
      ...base,
      providerTrackId: "42",
    });
    const fallbackChanged = buildListenLyricsTimelineKey({
      ...base,
      source: "Local sidecar",
      lines: [{ startMs: 1000, durationMs: 1000, text: "Changed" }],
    });

    expect(first).not.toBe(second);
    expect(fallbackChanged).not.toBe(buildListenLyricsTimelineKey(base));
  });

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
    expect(findListenActiveLyricLineIndex(timeline, 999)).toBe(-1);
    expect(findListenActiveLyricLineIndex(timeline, 1000)).toBe(0);
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
    expect(progress.progress).toBe(0.5);
  });

  test("uses exact half-open word boundaries without a hidden lead", () => {
    const words = [
      { startMs: 1000, endMs: 1500, text: "one" },
      { startMs: 1500, endMs: 2000, text: "two" },
    ];

    expect(getListenActiveLyricWordProgress(words, 999, 1000, 2000)).toEqual({
      index: -1,
      progress: 0,
    });
    expect(getListenActiveLyricWordProgress(words, 1000, 1000, 2000)).toEqual({
      index: 0,
      progress: 0,
    });
    expect(getListenActiveLyricWordProgress(words, 1500, 1000, 2000)).toEqual({
      index: 1,
      progress: 0,
    });
  });

  test("prefers explicit word ends and preserves exact spacing metadata", () => {
    const progress = getListenActiveLyricWordProgress(
      [
        { startMs: 0, endMs: 200, text: "ni", endsWithSpace: false },
        { startMs: 1000, endMs: 1400, text: "hao", endsWithSpace: true },
      ],
      500,
      0,
      2000,
    );

    expect(progress).toEqual({ index: 0, progress: 1 });
    expect(
      getListenLyricWordDisplayText({
        startMs: 0,
        text: "ni ",
        endsWithSpace: false,
      }),
    ).toBe("ni");
    expect(
      getListenLyricWordDisplayText({
        startMs: 0,
        text: "hello",
        endsWithSpace: true,
      }),
    ).toBe("hello ");
    expect(getListenLyricWordDisplayText({ startMs: 0, text: "legacy" })).toBe(
      "legacy ",
    );
  });

  test("uses syllable timing while preserving the parent word boundary", () => {
    const units = expandListenLyricTimingUnits([
      {
        startMs: 0,
        endMs: 600,
        text: "hello",
        endsWithSpace: true,
        syllables: [
          { startMs: 0, endMs: 250, text: "hel", endsWithSpace: false },
          { startMs: 250, endMs: 600, text: "lo", endsWithSpace: false },
        ],
      },
    ]);

    expect(units).toHaveLength(2);
    expect(units[0].endsWithSpace).toBe(false);
    expect(units[1].endsWithSpace).toBe(true);
    expect(units.map(getListenLyricWordDisplayText).join("")).toBe("hello ");
  });

  test("uses strict focus frames without previewing or holding gap lines", () => {
    const timeline = buildListenLyricsTimeline(
      [
        { startMs: 1000, durationMs: 500, text: "first" },
        { startMs: 5000, durationMs: 900, text: "second" },
      ],
      false,
    );

    expect(resolveListenLyricsFocusFrame(timeline, -1, 0).primaryIndex).toBe(-1);
    expect(resolveListenLyricsFocusFrame(timeline, 0, 900).primaryIndex).toBe(
      -1,
    );
    expect(resolveListenLyricsFocusFrame(timeline, 0, 1000).primaryIndex).toBe(
      0,
    );
    expect(resolveListenLyricsFocusFrame(timeline, 0, 1499).primaryIndex).toBe(
      0,
    );
    expect(resolveListenLyricsFocusFrame(timeline, 0, 1500).primaryIndex).toBe(
      -1,
    );
    expect(resolveListenLyricsFocusFrame(timeline, -1, 3200).primaryIndex).toBe(
      -1,
    );
    expect(resolveListenLyricsFocusFrame(timeline, 1, 5000).primaryIndex).toBe(
      1,
    );
    expect(resolveListenLyricsFocusFrame(timeline, 1, 5900).primaryIndex).toBe(
      -1,
    );
  });

  test("keeps line-only focus honest and preserves source typography", () => {
    const multilingualLine = "Hello, \u4e16\u754c 👋🏽!";
    const lineOnly = buildListenLyricsTimeline(
      [{ startMs: 1000, durationMs: 2000, text: multilingualLine }],
      false,
    )[0];
    const estimated = buildListenLyricsFocusTimingUnits(lineOnly);

    expect(estimated).toEqual([
      {
        startMs: 1000,
        endMs: 3000,
        text: multilingualLine,
      },
    ]);

    const timed = buildListenLyricsFocusTimingUnits({
      ...lineOnly,
      words: [
        { startMs: 1000, endMs: 1600, text: "Hello" },
        { startMs: 1600, endMs: 2200, text: "\u4e16\u754c" },
        { startMs: 2200, endMs: 3000, text: "👋🏽" },
      ],
    });
    expect(
      timed.map((_, index) =>
        getListenLyricTimingUnitDisplayText(timed, index),
      ).join(""),
    ).toBe(multilingualLine);

    const koreanLine = { ...lineOnly, text: "안녕 세상" };
    const korean = buildListenLyricsFocusTimingUnits({
      ...koreanLine,
      words: [
        { startMs: 1000, endMs: 2000, text: "안녕" },
        { startMs: 2000, endMs: 3000, text: "세상" },
      ],
    });
    expect(
      korean.map((_, index) =>
        getListenLyricTimingUnitDisplayText(korean, index),
      ).join(""),
    ).toBe("안녕 세상");
  });

  test("resolves strict pending, active, and passed word phases", () => {
    const words = [
      { startMs: 1000, endMs: 1500, text: "one" },
      { startMs: 1500, endMs: 2000, text: "two" },
    ];

    expect(getListenLyricWordVisualState(words, 0, 900, 1000, 2000)).toEqual({
      state: "pending",
      progress: 0,
    });
    expect(
      getListenLyricWordVisualState(words, 0, 1250, 1000, 2000),
    ).toEqual({ state: "active", progress: 0.5 });
    expect(
      getListenLyricWordVisualState(words, 0, 1600, 1000, 2000),
    ).toEqual({ state: "passed", progress: 1 });
  });

  test("keeps romanization on the strict focus line", () => {
    const timeline = buildListenLyricsTimeline(
      [
        {
          startMs: 0,
          durationMs: 1000,
          text: "きみ",
          romanizedKind: "romanized",
          romanizedText: "kimi",
        },
        { startMs: 1000, durationMs: 1000, text: "next" },
      ],
      { romanized: true },
    );

    expect(timeline[0].romanizedText).toBe("kimi");
    expect(resolveListenLyricsFocusFrame(timeline, 0, 400)).toEqual({
      primaryIndex: 0,
    });
  });

  test("keeps translated alternates on the strict focus line", () => {
    const timeline = buildListenLyricsTimeline(
      [
        {
          startMs: 0,
          durationMs: 1000,
          text: "hello",
          romanizedKind: "romanized",
          romanizedText: "hello romanized",
          alternateTexts: [
            { role: "translation", language: "en", text: "hello there" },
          ],
        },
      ],
      { romanized: true },
    );

    expect(timeline[0].translationText).toBe("hello there");
    expect(timeline[0].alternateTexts).toEqual([
      { role: "translation", language: "en", text: "hello there" },
    ]);
    expect(resolveListenLyricsFocusFrame(timeline, 0, 400)).toEqual({
      primaryIndex: 0,
    });
  });

  test("uses the offset-aware visual key for renderer transitions", async () => {
    const source = await Bun.file(new URL("./lyrics.tsx", import.meta.url)).text();
    expect(source).toContain("timelineKey={visualClockKey}");
    expect(source).toContain('`${timelineKey}\\u0000${props.clockKey}`');
  });
});
