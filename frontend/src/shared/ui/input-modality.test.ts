import { describe, expect, test } from "bun:test";

import { isKeyboardInteraction } from "./input-modality";

function keyboardEvent(
  key: string,
  overrides: Partial<Parameters<typeof isKeyboardInteraction>[0]> = {},
) {
  return {
    altKey: false,
    ctrlKey: false,
    key,
    metaKey: false,
    ...overrides,
  };
}

describe("input modality", () => {
  test("recognizes navigation and control activation keys", () => {
    for (const key of [
      "Tab",
      "ArrowDown",
      "ArrowLeft",
      "ArrowRight",
      "ArrowUp",
      "Home",
      "End",
      "PageDown",
      "PageUp",
      "Enter",
      " ",
      "Escape",
    ]) {
      expect(isKeyboardInteraction(keyboardEvent(key))).toBe(true);
    }
  });

  test("does not treat typing or modified shortcuts as focus navigation", () => {
    expect(isKeyboardInteraction(keyboardEvent("a"))).toBe(false);
    expect(
      isKeyboardInteraction(keyboardEvent("Enter", { metaKey: true })),
    ).toBe(false);
    expect(
      isKeyboardInteraction(keyboardEvent("Tab", { ctrlKey: true })),
    ).toBe(false);
    expect(
      isKeyboardInteraction(keyboardEvent("ArrowDown", { altKey: true })),
    ).toBe(false);
  });

  test("installs capture listeners with an unknown assistive fallback", async () => {
    const source = await Bun.file(
      new URL("./input-modality.ts", import.meta.url),
    ).text();

    expect(source).toContain('root.dataset.inputModality = "unknown"');
    expect(source).toContain(
      'documentNode.addEventListener("pointerdown", handlePointerDown, true)',
    );
    expect(source).toContain(
      'documentNode.addEventListener("keydown", handleKeyDown, true)',
    );
    expect(source).toContain('root.dataset.inputModality = "pointer"');
    expect(source).toContain('root.dataset.inputModality = "keyboard"');
    expect(source).toContain("delete root.dataset.inputModality");
  });
});
