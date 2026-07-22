import { afterEach, describe, expect, test } from "bun:test";

import {
  FUN_BUTTON_EFFECTS,
  pickFunButtonEffect,
  type FunButtonEffect,
} from "./fun-button-effect";

const originalRandom = Math.random;
const motionCSSURL = new URL("../styles/dream/motion.css", import.meta.url);

afterEach(() => {
  Math.random = originalRandom;
});

describe("fun button effect registry", () => {
  test("exposes exactly ten unique redesigned effects", () => {
    const expected: FunButtonEffect[] = [
      "aurora",
      "prism",
      "comet",
      "plasma",
      "magnetic",
      "ripple",
      "stardust",
      "orbit",
      "shards",
      "hologram",
    ];

    expect(FUN_BUTTON_EFFECTS).toEqual(expected);
    expect(FUN_BUTTON_EFFECTS).toHaveLength(10);
    expect(new Set(FUN_BUTTON_EFFECTS).size).toBe(10);
  });

  test("selects every effect from its corresponding random bucket", () => {
    FUN_BUTTON_EFFECTS.forEach((effect, index) => {
      Math.random = () => (index + 0.5) / FUN_BUTTON_EFFECTS.length;
      expect(pickFunButtonEffect()).toBe(effect);
    });
  });

  test("handles valid edges and falls back safely for invalid random values", () => {
    Math.random = () => 0;
    expect(pickFunButtonEffect()).toBe("aurora");

    Math.random = () => 1 - Number.EPSILON;
    expect(pickFunButtonEffect()).toBe("hologram");

    for (const invalidValue of [-1, 1, Number.NaN, Number.POSITIVE_INFINITY]) {
      Math.random = () => invalidValue;
      expect(pickFunButtonEffect()).toBe("aurora");
    }
  });

  test("ships one visible decorative recipe per effect with motion fallbacks", async () => {
    const source = await Bun.file(motionCSSURL).text();

    for (const effect of FUN_BUTTON_EFFECTS) {
      expect(source).toContain(`data-effect="${effect}"`);
    }
    for (const retiredEffect of [
      "water",
      "fire",
      "cloud",
      "sun",
      "mist",
      "shadow",
    ]) {
      expect(source).not.toContain(`data-effect="${retiredEffect}"`);
    }
    expect(source).toContain(
      "opacity: var(--app-fun-button-before-opacity)",
    );
    expect(source).toContain(
      "opacity: var(--app-fun-button-after-opacity)",
    );
    expect(source).toContain("-webkit-clip-path: inset(0 round 9999px)");
    expect(source).toContain("\n    clip-path: inset(0 round 9999px);");
    expect(source).toContain("background-clip: padding-box");
    expect(source).toContain("@media (prefers-reduced-motion: reduce)");
    expect(source).toContain("@media (forced-colors: active)");
  });
});
