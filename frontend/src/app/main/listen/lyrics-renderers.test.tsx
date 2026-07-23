import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";
import {
  buildListenLyricsTimeline,
  LISTEN_LYRICS_FOCUS_STYLES,
  ListenLyricsSurface,
} from "@/app/main/listen/lyrics";
import {
  resolveListenLyricsFocusFlowRowClipInsets,
  resolveListenLyricsFocusFlowRowProgresses,
  resolveListenLyricsFocusTransition,
  type ListenLyricsFocusTransitionState,
} from "@/app/main/listen/lyrics-renderers";
import type { ListenLyricsData } from "@/app/main/listen/types";

const text = getXiaText("en");
const syncedLyrics: ListenLyricsData = {
  videoId: "focus-track",
  kind: "synced",
  source: "Example source",
  attribution: "Example contributors",
  timingQuality: "word",
  text: "きみ\nNext line",
  lines: [
    {
      startMs: 1000,
      durationMs: 2000,
      text: "きみ",
      translationText: "You",
      romanizedKind: "romanized",
      romanizedText: "kimi",
      words: [
        { startMs: 1000, text: "き" },
        { startMs: 1800, text: "み" },
      ],
    },
    { startMs: 3000, durationMs: 1600, text: "Next line" },
  ],
};

const temporalLyrics: ListenLyricsData = {
  videoId: "temporal-focus-track",
  kind: "synced",
  source: "test",
  timingQuality: "word",
  text: "Past signal\nPresent signal\nFuture signal",
  lines: [
    {
      startMs: 0,
      durationMs: 1000,
      text: "Past signal",
      romanizedKind: "romanized",
      romanizedText: "pastu",
      words: [{ startMs: 0, endMs: 1000, text: "Past signal" }],
    },
    {
      startMs: 1000,
      durationMs: 1000,
      text: "Present signal",
      romanizedKind: "romanized",
      romanizedText: "ima",
      words: [{ startMs: 1000, endMs: 2000, text: "Present signal" }],
    },
    {
      startMs: 2000,
      durationMs: 1000,
      text: "Future signal",
      romanizedKind: "romanized",
      romanizedText: "mirai",
      words: [{ startMs: 2000, endMs: 3000, text: "Future signal" }],
    },
  ],
};

const pinyinLyrics: ListenLyricsData = {
  videoId: "pinyin-focus-track",
  kind: "synced",
  source: "test",
  timingQuality: "word",
  text: "\u4f60\u597d",
  lines: [
    {
      startMs: 0,
      durationMs: 1000,
      text: "\u4f60\u597d",
      romanizedKind: "pinyin",
      romanizedText: "nǐ hǎo",
      words: [
        { startMs: 0, endMs: 500, text: "\u4f60" },
        { startMs: 500, endMs: 1000, text: "\u597d" },
      ],
    },
  ],
};

describe("listen lyrics renderers", () => {
  test("renders focus mode on the transparent host with accessible seek", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        variant="companion"
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1500}
        romanized
        onSeek={() => undefined}
      />,
    );

    expect(markup).toContain('data-listen-lyrics-renderer="focus"');
    expect(markup).toContain('data-listen-lyrics-variant="companion"');
    expect(markup).toContain('data-listen-lyrics-focus-style="prism"');
    expect(markup).toContain("listen-lyrics-focus__stage");
    expect(markup).toContain('data-focus-scene="single-flow"');
    expect(markup).toContain('data-context-relation="next"');
    expect(markup).toContain("Next line");
    expect(markup).not.toContain("listen-lyrics-focus__keywords");
    expect(markup).not.toContain("listen-lyrics-focus__artwork");
    expect(markup).not.toContain("listen-lyrics-focus__wash");
    expect(markup).not.toContain("listen-lyrics-focus__card");
    expect(markup).not.toContain('data-material="panel"');
    expect(markup).not.toContain("<img");
    expect(markup).toContain('<button type="button"');
    expect(markup).toContain('aria-current="true"');
    expect(markup).toContain('data-listen-lyrics-timing="word"');
    expect(markup).toContain('data-kind="translation"');
    expect(markup).toContain('data-kind="romanized"');
    expect(markup).toContain("You");
    expect(markup).toContain("kimi");
    expect(markup).toContain('data-word-state="active"');
    expect(markup).toContain('data-word-state="pending"');
    expect(markup).not.toContain("listen-lyrics-focus__word-effect");
    expect(markup).not.toContain("listen-lyrics-focus__romanization-effect");
    expect(markup).not.toContain("data-effect-layer");
    expect(markup).not.toContain("data-romanization-layer");
    expect(markup).toContain("listen-lyrics-focus__word-fill");
    expect(markup).toContain('aria-hidden="true"');
    expect(markup).not.toContain("Example contributors");
  });

  test("keeps one semantic current-line DOM while progress paints stay aria-hidden", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        variant="companion"
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1500}
        romanized
        onSeek={() => undefined}
      />,
    );

    expect(markup.match(/<button type="button"/g)).toHaveLength(1);
    expect(markup.match(/aria-current="true"/g)).toHaveLength(1);
    expect(markup.match(/class="listen-lyrics-focus__word"/g)).toHaveLength(2);
    expect(markup.indexOf(">き<")).toBeLessThan(markup.indexOf(">み<"));
    expect(markup).toContain('data-kind="translation"');
    expect(markup).toContain('data-kind="romanized"');
    expect(markup).toContain('data-focus-density="sparse"');
    expect(markup).toContain("--listen-lyrics-focus-unit-");
    expect(markup.match(/aria-hidden="true"/g)?.length).toBeGreaterThanOrEqual(2);
    expect(markup).not.toContain("listen-lyrics-focus__word-effect");
    expect(markup).not.toContain("listen-lyrics-focus__romanization-effect");
  });

  test("normalizes former styles into one transparent single-flow structure", () => {
    for (const focusStyle of [
      "prism",
      "splice",
      "facet",
      "pendulum",
    ] as const) {
      const markup = renderToStaticMarkup(
        <ListenLyricsSurface
          renderer="focus"
          focusStyle={focusStyle}
          text={text}
          lyrics={syncedLyrics}
          currentTimeMs={1500}
        />,
      );
      expect(markup).toContain('data-listen-lyrics-focus-style="prism"');
      expect(markup).toContain('data-focus-scene="single-flow"');
      expect(markup).not.toContain("data-effect-layer");
      expect(markup).not.toContain("data-romanization-layer");
      expect(markup).not.toContain('data-material="panel"');
      expect(markup).not.toContain("<img");
      expect(markup.match(/aria-current="true"/g)).toHaveLength(1);
    }
  });

  test("projects romanization through the single focus flow without inventing syllable timing", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        focusStyle="prism"
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1500}
        romanized
        onSeek={() => undefined}
      />,
    );
    expect(markup).toContain('data-focus-romanization="true"');
    expect(markup).toContain('data-focus-romanization-style="prism"');
    expect(markup).toContain('data-romanization-timing="projected"');
    expect(markup).toContain(
      "--listen-lyrics-focus-romanization-progress:31.25%",
    );
    expect(markup.match(/data-kind="romanized"/g)).toHaveLength(1);
    expect(markup).not.toContain("data-romanization-layer");
    expect(markup).not.toContain("listen-lyrics-focus__romanization-effect");
    expect(markup.match(/aria-current="true"/g)).toHaveLength(1);
  });

  test("keeps pinyin and romanized display switches honest at the renderer boundary", () => {
    const renderPinyin = (display: { pinyin?: boolean; romanized?: boolean }) =>
      renderToStaticMarkup(
        <ListenLyricsSurface
          renderer="focus"
          text={text}
          lyrics={pinyinLyrics}
          currentTimeMs={500}
          pinyin={display.pinyin}
          romanized={display.romanized}
        />,
      );
    expect(renderPinyin({ pinyin: true })).toContain("nǐ hǎo");
    expect(renderPinyin({ pinyin: true })).toContain(
      'data-focus-romanization="true"',
    );
    expect(renderPinyin({ romanized: true })).not.toContain("nǐ hǎo");
    expect(renderPinyin({})).not.toContain(
      'data-focus-romanization="true"',
    );

    const romanizedDisabled = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1500}
        pinyin
      />,
    );
    expect(romanizedDisabled).not.toContain("kimi");
  });

  test("expresses past, present, and future without a decorative text layer", () => {
    const prism = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        focusStyle="prism"
        text={text}
        lyrics={temporalLyrics}
        currentTimeMs={1500}
        romanized
        onSeek={() => undefined}
      />,
    );

    expect(prism).toContain('data-focus-scene="single-flow"');
    expect(prism).toContain('data-temporal-layout="past-present-future"');
    expect(prism).toMatch(
      /data-context-relation="previous"[^>]*data-context-state="completed"[^>]*data-temporal-role="past"/,
    );
    expect(prism).toMatch(
      /data-context-relation="next"[^>]*data-context-state="pending"[^>]*data-temporal-role="future"/,
    );
    expect(prism.match(/class="listen-lyrics-focus__context"/g)).toHaveLength(
      2,
    );
    expect(prism).not.toContain("data-prism-context-layer");
    expect(prism.match(/data-context-track="romanized"/g)).toHaveLength(2);
    expect(prism).toContain("pastu");
    expect(prism).toContain("mirai");
    expect(prism.match(/data-temporal-role="present"/g)).toHaveLength(1);
    expect(prism.match(/aria-current="true"/g)).toHaveLength(1);
    expect(prism).not.toContain("listen-lyrics-focus__keywords");
    expect(prism).not.toContain("data-keyword-layer");
  });

  test("marks the current focus line even when the surface is not seekable", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        variant="player"
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1500}
      />,
    );

    expect(markup.match(/aria-current="true"/g)).toHaveLength(1);
    expect(markup).not.toContain('<button type="button"');
  });

  test("keeps the scrolling renderer as the default contract", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsSurface
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1500}
      />,
    );

    expect(markup).toContain('data-listen-lyrics-renderer="scroll"');
    expect(markup).toContain('data-listen-lyric-support="translation"');
    expect(markup).toContain('data-lyric-state="active"');
    expect(markup).toContain("listen-lyrics-karaoke__word-fill");
    expect(markup).not.toContain("listen-lyrics-focus__card");
    expect(markup).toContain("Next line");
    expect(markup).not.toContain("Example contributors");
  });

  test("names only Companion Dynamic lyrics as its semantic scroll owner", () => {
    const dynamicMarkup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="scroll"
        variant="companion"
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1500}
      />,
    );
    const focusMarkup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        variant="companion"
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={1500}
      />,
    );

    expect(dynamicMarkup).toContain(
      'data-companion-scroll-owner="lyrics"',
    );
    expect(focusMarkup).not.toContain("data-companion-scroll-owner");
  });

  test("keeps karaoke paint layers on the active dynamic line only", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="scroll"
        text={text}
        currentTimeMs={500}
        lyrics={{
          videoId: "dynamic-dom-budget",
          kind: "synced",
          source: "test",
          timingQuality: "word",
          text: "one\ntwo",
          lines: [
            {
              startMs: 0,
              durationMs: 1000,
              text: "one",
              words: [{ startMs: 0, endMs: 1000, text: "one" }],
            },
            {
              startMs: 1000,
              durationMs: 1000,
              text: "two",
              words: [{ startMs: 1000, endMs: 2000, text: "two" }],
            },
          ],
        }}
      />,
    );

    expect(markup.match(/listen-lyrics-karaoke__word-fill/g)).toHaveLength(1);
    expect(markup).toContain("one");
    expect(markup).toContain("two");
  });

  test("keeps legacy CJK and punctuation spacing identical across dynamic states", () => {
    const cjkText = "\u4f60\u597d\uff0c\u4e16\u754c";
    const lyrics: ListenLyricsData = {
      videoId: "dynamic-spacing",
      kind: "synced",
      source: "test",
      timingQuality: "word",
      text: `${cjkText}\nHello, world`,
      lines: [
        {
          startMs: 0,
          durationMs: 1000,
          text: cjkText,
          words: [
            { startMs: 0, text: "\u4f60" },
            { startMs: 200, text: "\u597d" },
            { startMs: 400, text: "\uff0c" },
            { startMs: 600, text: "\u4e16" },
            { startMs: 800, text: "\u754c" },
          ],
        },
        {
          startMs: 1000,
          durationMs: 1000,
          text: "Hello, world",
          words: [
            { startMs: 1000, text: "Hello" },
            { startMs: 1350, text: "," },
            { startMs: 1450, text: "world" },
          ],
        },
      ],
    };
    const renderAt = (currentTimeMs: number) =>
      renderToStaticMarkup(
        <ListenLyricsSurface
          renderer="scroll"
          text={text}
          currentTimeMs={currentTimeMs}
          lyrics={lyrics}
        />,
      );
    const activeText = (markup: string) =>
      [...markup.matchAll(/listen-lyrics-karaoke__word-base">([^<]*)<\/span>/g)]
        .map((match) => match[1])
        .join("");
    const inactiveText = (markup: string) =>
      [...markup.matchAll(/class="listen-lyrics-karaoke">([^<]*)<\/span>/g)]
        .map((match) => match[1])
        .join("");

    const cjkActive = renderAt(500);
    expect(activeText(cjkActive)).toBe(cjkText);
    expect(inactiveText(cjkActive)).toBe("Hello, world");

    const latinActive = renderAt(1500);
    expect(activeText(latinActive)).toBe("Hello, world");
    expect(inactiveText(latinActive)).toBe(cjkText);
  });

  test("uses plain only as a source-free fallback presentation", () => {
    const markup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        text={text}
        lyrics={{
          videoId: "plain-track",
          kind: "plain",
          source: "LRCLib",
          attribution: "LRCLIB contributors",
          timingQuality: "plain",
          text: "Plain line",
          lines: [],
        }}
      />,
    );

    expect(markup).toContain('data-listen-lyrics-renderer="plain"');
    expect(markup).toContain("Plain line");
    expect(markup).not.toContain("LRCLIB contributors");
    expect(markup).not.toContain("listen-source-watermark");
    expect(markup).not.toContain("data-listen-lyrics-focus-style");
  });

  test("shows decorative context without promoting it to current karaoke", () => {
    const activeMarkup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        text={text}
        lyrics={{
          ...syncedLyrics,
          lines: syncedLyrics.lines.map((line) => ({
            ...line,
            translationText: undefined,
            romanizedText: undefined,
          })),
        }}
        currentTimeMs={1500}
        onSeek={() => undefined}
      />,
    );

    expect(activeMarkup.match(/<button type="button"/g)).toHaveLength(1);
    expect(activeMarkup).toContain(">き<");
    expect(activeMarkup).toContain(">み<");
    expect(activeMarkup).toContain("Next line");
    expect(activeMarkup).toContain('data-context-relation="next"');
    expect(activeMarkup.match(/aria-current="true"/g)).toHaveLength(1);
    expect(activeMarkup.match(/class="listen-lyrics-focus__word"/g)).toHaveLength(2);

    const gapMarkup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        text={text}
        lyrics={syncedLyrics}
        currentTimeMs={4700}
        onSeek={() => undefined}
      />,
    );
    expect(gapMarkup).toContain('data-focus-empty="true"');
    expect(gapMarkup).toContain('data-focus-context-phase="after"');
    expect(gapMarkup).not.toContain("きみ");
    expect(gapMarkup).toContain("Next line");
    expect(gapMarkup).not.toContain('aria-current="true"');
    expect(gapMarkup).not.toContain('class="listen-lyrics-focus__word"');
  });

  test("marks line timing as an honest whole-line estimated phase", () => {
    const multilingualLine = "Hello, \u4e16\u754c 👋🏽!";
    const markup = renderToStaticMarkup(
      <ListenLyricsSurface
        renderer="focus"
        text={text}
        lyrics={{
          videoId: "line-track",
          kind: "synced",
          source: "LRCLib",
          timingQuality: "line",
          text: multilingualLine,
          lines: [
            {
              startMs: 0,
              durationMs: 2000,
              text: multilingualLine,
            },
          ],
        }}
        currentTimeMs={1000}
      />,
    );

    expect(markup).toContain('data-timing-estimated="true"');
    expect(markup).toContain('data-word-state="active"');
    expect(markup).toContain("listen-lyrics-focus__word-fill");
    expect(markup).toContain(multilingualLine);
    expect(markup.match(/class="listen-lyrics-focus__word"/g)).toHaveLength(1);
    expect(markup).toContain("--listen-lyrics-focus-unit-");
  });

  test("advances wrapped Focus flow rows in reading order by actual width", () => {
    const halfway = resolveListenLyricsFocusFlowRowProgresses(0.5, [300, 100]);
    expect(halfway[0]).toBeCloseTo(2 / 3);
    expect(halfway[1]).toBe(0);

    const secondRow = resolveListenLyricsFocusFlowRowProgresses(0.875, [
      300,
      100,
    ]);
    expect(secondRow[0]).toBe(1);
    expect(secondRow[1]).toBeCloseTo(0.5);
    expect(resolveListenLyricsFocusFlowRowProgresses(1, [300, 100])).toEqual([
      1,
      1,
    ]);
  });

  test("partitions overlapping Focus flow rows at one shared boundary", () => {
    const clips = resolveListenLyricsFocusFlowRowClipInsets([
      { start: 0, end: 0.56 },
      { start: 0.52, end: 1 },
    ]);

    expect(clips[0]?.top).toBe(0);
    expect(clips[0]?.bottom).toBeCloseTo(0.46);
    expect(clips[1]?.top).toBeCloseTo(0.54);
    expect(clips[1]?.bottom).toBe(0);
    expect(1 - (clips[0]?.bottom ?? 0)).toBe(clips[1]?.top);
  });

  test("keeps one bounded outgoing line only for nearby same-track moves", () => {
    const lines = buildListenLyricsTimeline(
      [
        { startMs: 0, durationMs: 1000, text: "one" },
        { startMs: 1000, durationMs: 1000, text: "two" },
        { startMs: 2000, durationMs: 1000, text: "three" },
      ],
      false,
    );
    const base: ListenLyricsFocusTransitionState = {
      timelineKey: "same",
      currentIndex: 0,
      outgoingIndex: -1,
      revision: 0,
      timeMs: 990,
    };
    const adjacent = resolveListenLyricsFocusTransition(
      lines,
      "same",
      1,
      1000,
      base,
    );
    expect(adjacent).toMatchObject({ currentIndex: 1, outgoingIndex: 0 });

    const observed = resolveListenLyricsFocusTransition(
      lines,
      "same",
      1,
      1050,
      adjacent,
    );
    const rapid = resolveListenLyricsFocusTransition(
      lines,
      "same",
      2,
      2000,
      observed,
    );
    expect(rapid).toMatchObject({ currentIndex: 2, outgoingIndex: 1 });

    expect(
      resolveListenLyricsFocusTransition(lines, "same", -1, 1000, base),
    ).toMatchObject({ currentIndex: -1, outgoingIndex: 0 });
    expect(
      resolveListenLyricsFocusTransition(lines, "same", 1, 5000, base),
    ).toMatchObject({ currentIndex: 1, outgoingIndex: -1 });
    expect(
      resolveListenLyricsFocusTransition(lines, "new", 1, 1000, base),
    ).toMatchObject({ currentIndex: 1, outgoingIndex: -1 });
  });

  test("ships shared material, responsive scale, and accessibility fallbacks", async () => {
    const [layoutCss, appearanceCss, source] = await Promise.all([
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./lyrics-renderers.tsx", import.meta.url)).text(),
    ]);
    const css = `${layoutCss}\n${appearanceCss}`;

    expect(css).toContain("@media (prefers-reduced-motion: reduce)");
    expect(css).toContain(".listen-lyrics-focus__primary[data-seekable=\"true\"]:hover");
    expect(css).toContain("@media (forced-colors: active)");
    expect(css).toContain("background-image: none !important");
    expect(css).toMatch(/\.listen-lyrics-focus\s*\{[^}]*background:\s*transparent;/s);
    expect(css).toMatch(
      /\.listen-lyrics-focus\s*\{[^}]*contain:\s*layout paint style;/s,
    );
    expect(css).toContain(
      "--listen-lyrics-focus-accent: hsl(\n    var(--app-accent-text, var(--sidebar-primary))\n  );",
    );
    expect(css).not.toContain(".listen-lyrics-focus__artwork");
    expect(css).not.toContain(".listen-lyrics-focus__wash");
    expect(css).not.toContain(".listen-lyrics-focus__card");
    expect(css).not.toContain(".listen-lyrics-focus__rail");
    expect(css).not.toContain(".listen-lyrics-focus__helper");
    for (const style of LISTEN_LYRICS_FOCUS_STYLES) {
      expect(css).toContain(`@keyframes listen-lyrics-focus-${style}-in`);
      expect(css).toContain(`@keyframes listen-lyrics-focus-${style}-out`);
    }
    expect(css).not.toContain(".listen-lyrics-focus__word-glow");
    expect(css).not.toContain(".listen-lyrics-focus__stage::before");
    expect(css).not.toContain(".listen-lyrics-focus__stage::after");
    expect(css).not.toContain(".listen-lyrics-focus__keyword");
    expect(css).not.toContain("data-keyword-");
    expect(css).not.toContain("--listen-lyrics-focus-keyword");
    expect(css).toContain(".listen-lyrics-focus__word-fill");
    expect(css).toContain("var(--listen-lyrics-focus-word-progress, 0%)");
    expect(css).toMatch(
      /\.listen-lyrics-focus\[data-listen-lyrics-focus-style\]\s*\.listen-lyrics-focus__word-fill\[data-focus-flow-row-state="pending"\]\s*\{[^}]*opacity:\s*0;/s,
    );
    expect(css).toContain('data-focus-density="balanced"');
    expect(css).toContain('data-focus-density="dense"');
    expect(css).toContain("clamp(1.9rem, 5.4vw, 3.6rem);");
    expect(css).toContain("clamp(1.9rem, 8.2cqw, 3.6rem);");
    expect(css).toContain("@supports (width: 1cqw)");
    expect(css).toContain("container: listen-lyrics-scroll / inline-size;");
    expect(css).toContain(
      "--listen-lyrics-scroll-size: clamp(1.375rem, 3.4cqw, 1.625rem);",
    );
    expect(css).toContain("padding-inline: clamp(0.2rem, 5cqw, 3.5rem);");
    expect(css).toContain(".listen-lyrics-scroll,\n.listen-lyrics-focus");
    expect(layoutCss).not.toMatch(/(?:-webkit-)?backdrop-filter\s*:/);
    expect(css).toContain("-webkit-mask-image: linear-gradient(");
    expect(source).not.toContain("<GlassSurface");
    expect(source).toContain("return 0.5;");
    expect(source).toContain("container.clientHeight / 2 - activeLineHeight / 2");
    expect(source).toContain("paddingBottom: viewportPadding.bottom,");
    expect(source).not.toContain("listen-lyrics-focus__keywords");
    expect(source).not.toContain("data-keyword-layer");
    expect(source).not.toContain("data-prism-context-layer");
    const canonicalFocusCss = `${layoutCss.slice(
      layoutCss.lastIndexOf("/* Canonical lyrics presentation."),
    )}\n${appearanceCss.slice(
      appearanceCss.lastIndexOf(".listen-lyrics-scroll,\n.listen-lyrics-focus"),
    )}`;
    expect(source).toContain("new ResizeObserver(scheduleSync)");
    expect(source).toContain("document.createRange");
    expect(source).toContain(
      'fonts?.addEventListener?.("loadingdone", scheduleSync)',
    );
    for (const paletteRole of [
      "--listen-lyrics-focus-current: var(--dream-primary-text);",
      "--listen-lyrics-focus-flow-leading: hsl(var(--chart-2));",
      "--listen-lyrics-focus-flow-trailing: hsl(var(--chart-5));",
      "--listen-lyrics-focus-context-previous: color-mix(",
      "--listen-lyrics-focus-context-next: color-mix(",
    ]) {
      expect(canonicalFocusCss).toContain(paletteRole);
    }
    expect(source).not.toContain("listen-lyrics-focus__word-effect");
    expect(source).not.toContain("listen-lyrics-focus__romanization-effect");
    expect(source).not.toContain("data-effect-layer");
    expect(source).not.toContain("data-romanization-layer");
    expect(css).toContain(".listen-lyrics-focus__romanization");
    expect(css).toContain(
      "var(--listen-lyrics-focus-romanization-progress, 0%)",
    );
    expect(css).toContain(".listen-lyrics-scroll__line[data-lyric-state=\"active\"]");
    expect(css).toContain(".listen-lyrics-karaoke__word-fill");
    expect(css).toMatch(
      /\.listen-lyrics-karaoke\s*\{[^}]*white-space:\s*pre-wrap;/s,
    );
    expect(css).toMatch(
      /\.listen-lyrics-karaoke__word-fill\s*\{[^}]*user-select:\s*none;/s,
    );
    expect(css).toMatch(
      /\.listen-lyrics-focus__word-fill\s*\{[^}]*user-select:\s*none;/s,
    );
    expect(layoutCss).toMatch(
      /\.listen-lyrics-focus__word\s*\{[^}]*position:\s*relative;[^}]*display:\s*inline-block;[^}]*max-width:\s*100%;/s,
    );
    expect(layoutCss).toMatch(
      /\.listen-lyrics-focus__word-fill\s*\{[^}]*position:\s*absolute;[^}]*inset:\s*0;[^}]*pointer-events:\s*none;/s,
    );
    expect(appearanceCss).toMatch(
      /\.listen-lyrics-focus\[data-listen-lyrics-focus-style\]\s*\.listen-lyrics-focus__primary\s*\{[^}]*text-shadow:\s*none;/s,
    );
    expect(source).toContain('key="current"');
    expect(source).toContain('key="outgoing"');
    expect(source).toContain(
      "key={`paint:${props.timelineKey}:${props.index}:${props.line.startMs}:${props.line.sourceIndex}:${props.focusStyle}`}",
    );
    expect(source).not.toContain("key={`current:");
    expect(source).toContain("const ListenFocusWord = React.memo");
    expect(source).toContain('data-focus-scene="single-flow"');
    expect(source).toContain('data-temporal-layout="past-present-future"');
    expect(source).toContain('data-focus-romanization="true"');
    expect(source).toContain('data-romanization-timing="projected"');
    expect(source).toContain('<ListenFocusContextCopy line={props.line} />');
    expect(source).toContain('data-context-state={resolveListenFocusContextState(props.relation)}');
    expect(css).toContain(".listen-lyrics-focus__primary-paint");
    expect(css).toContain('.listen-lyrics-focus__word[data-word-state="pending"]');
    expect(css).toContain('.listen-lyrics-focus__word[data-word-state="active"]');
    expect(css).toContain('.listen-lyrics-focus__word[data-word-state="passed"]');
    expect(css).toContain('data-listen-lyrics-focus-style="prism"');
    expect(css).toContain("text-decoration: underline 0.12em Highlight");
    expect(css).toMatch(
      /@media \(forced-colors: active\)[\s\S]*?\.listen-lyrics-focus__scene\s*\{[^}]*display:\s*none;/,
    );
    const canonicalLowHeightRule = css.slice(
      css.lastIndexOf("@media (max-height: 520px)"),
    );
    expect(canonicalLowHeightRule).toContain(
      '.listen-lyrics-focus__context[data-context-relation="previous"]',
    );
    expect(css).toContain('[data-listen-lyrics-variant="companion"]');
    expect(css).toContain(".listen-workspace-fullscreen-player .listen-lyrics-focus");
    const baseWordRule = css.match(
      /\.listen-lyrics-focus__word\s*\{([^}]*)\}/s,
    )?.[1];
    expect(baseWordRule).toBeDefined();
    expect(baseWordRule).not.toContain("will-change:");
    expect(source).not.toContain("Math.random");
  });
});
