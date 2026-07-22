import { describe, expect, test } from "bun:test";

import {
  resolveListenLyricsFocusContextFrame,
  segmentListenLyricsFocusGraphemes,
} from "@/app/main/listen/lyrics-focus-context";
import { buildListenLyricsTimeline } from "@/app/main/listen/lyrics-timeline";

describe("listen lyrics focus context", () => {
  const lines = buildListenLyricsTimeline([
    { startMs: 1000, durationMs: 1000, text: "Previous signal" },
    { startMs: 2000, durationMs: 1000, text: "" },
    { startMs: 3000, durationMs: 1000, text: "Electric heartbeat" },
    { startMs: 5000, durationMs: 1000, text: "Future skyline" },
  ]);

  test("skips empty rows and resolves active neighbours", () => {
    expect(resolveListenLyricsFocusContextFrame(lines, 2, 3500)).toMatchObject({
      phase: "active",
      primaryIndex: 2,
      previousIndex: 0,
      nextIndex: 3,
    });
  });

  test("keeps visual anchors through real gaps without inventing a current", () => {
    expect(resolveListenLyricsFocusContextFrame(lines, -1, 4500)).toMatchObject({
      phase: "gap",
      primaryIndex: -1,
      previousIndex: 2,
      nextIndex: 3,
    });
    expect(resolveListenLyricsFocusContextFrame(lines, -1, 200)).toMatchObject({
      phase: "before",
      previousIndex: -1,
      nextIndex: 0,
    });
    expect(resolveListenLyricsFocusContextFrame(lines, -1, 7000)).toMatchObject({
      phase: "after",
      previousIndex: 3,
      nextIndex: -1,
    });
  });

  test("segments complex graphemes without splitting composed glyphs", () => {
    expect(segmentListenLyricsFocusGraphemes("e\u0301👋🏽👨‍👩‍👧‍👦")).toHaveLength(3);
  });
});
