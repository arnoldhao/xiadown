import { describe, expect, test } from "bun:test";

import {
  buildListenLyricsFocusLayout,
  DEFAULT_LISTEN_LYRICS_FOCUS_STYLE,
  isListenLyricsFocusStyle,
  LISTEN_LYRICS_FOCUS_STYLES,
  normalizeListenLyricsFocusStyle,
  resolveListenLyricsFocusWordMotion,
} from "@/app/main/listen/lyrics-focus-style";

describe("listen lyrics focus effects", () => {
  test("exposes one composition and normalizes every former value", () => {
    expect(LISTEN_LYRICS_FOCUS_STYLES).toEqual(["prism"]);
    expect(DEFAULT_LISTEN_LYRICS_FOCUS_STYLE).toBe("prism");
    expect(isListenLyricsFocusStyle("prism")).toBe(true);
    for (const legacyStyle of ["splice", "facet", "pendulum"]) {
      expect(isListenLyricsFocusStyle(legacyStyle)).toBe(false);
    }
    for (const former of [
      undefined,
      "prism",
      "splice",
      "facet",
      "pendulum",
      "halo",
      "kinetic-cover",
      "editorial",
      "unknown",
    ]) {
      expect(normalizeListenLyricsFocusStyle(former)).toBe("prism");
    }
  });

  test("builds stable source-order metadata without Math.random", () => {
    const units = [
      { text: "Keep " },
      { text: "the " },
      { text: "source " },
      { text: "order 👋🏽" },
    ];
    for (const style of ["prism", "splice", "facet", "pendulum"] as const) {
      const first = buildListenLyricsFocusLayout(units, style);
      const second = buildListenLyricsFocusLayout(units, style);
      expect(second).toEqual(first);
      expect(first.style).toBe("prism");
      expect(first.words.map((word) => word.index)).toEqual([0, 1, 2, 3]);
      expect(first.words.map((word) => word.text)).toEqual(
        units.map((unit) => unit.text),
      );
      expect(
        first.words.every(
          (word) => word.direction === -1 || word.direction === 1,
        ),
      ).toBe(true);
    }
  });

  test("keeps line-only timing as one whole-sentence motion unit", () => {
    const text = "Hello, \u4e16\u754c 👋🏽!";
    const layout = buildListenLyricsFocusLayout([{ text }], "facet");
    expect(layout.style).toBe("prism");
    expect(layout.words).toHaveLength(1);
    expect(layout.words[0]?.text).toBe(text);
  });

  test("classifies long lines as dense so the renderer can scale them down", () => {
    const layout = buildListenLyricsFocusLayout(
      Array.from({ length: 14 }, (_, index) => ({ text: `word-${index} ` })),
      "splice",
    );
    expect(layout.density).toBe("dense");
  });

  test("resolves finite bounded transform values for every state", () => {
    const word = buildListenLyricsFocusLayout(
      [{ text: "motion" }],
      "pendulum",
    ).words[0]!;
    for (const state of ["pending", "active", "passed"] as const) {
      for (const progress of [-1, 0, 0.35, 1, 2, Number.NaN]) {
        const motion = resolveListenLyricsFocusWordMotion(
          word,
          state,
          progress,
        );
        expect(Object.values(motion).every(Number.isFinite)).toBe(true);
        expect(Math.abs(motion.prismOffsetEm)).toBeLessThanOrEqual(0.08);
        expect(motion.scale).toBeGreaterThanOrEqual(0.97);
        expect(motion.scale).toBeLessThanOrEqual(1.055);
      }
    }
  });

  test("gives the single Prism flow a restrained active emphasis", () => {
    const word = buildListenLyricsFocusLayout(
      [{ text: "depth" }],
      "prism",
    ).words[0]!;
    const pending = resolveListenLyricsFocusWordMotion(
      word,
      "pending",
      0,
    );
    const activeStart = resolveListenLyricsFocusWordMotion(
      word,
      "active",
      0.5,
    );
    const passed = resolveListenLyricsFocusWordMotion(word, "passed", 1);

    expect(Math.abs(activeStart.prismOffsetEm)).toBeGreaterThan(
      Math.abs(pending.prismOffsetEm),
    );
    expect(Math.abs(pending.prismOffsetEm)).toBeGreaterThan(
      Math.abs(passed.prismOffsetEm),
    );
    expect(activeStart.scale).toBeGreaterThan(1);
    expect(passed.scale).toBe(1);
  });
});
