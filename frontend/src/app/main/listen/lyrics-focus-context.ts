import type { ListenLyricTimelineLine } from "@/app/main/listen/lyrics-timeline";

export type ListenLyricsFocusContextPhase =
  | "before"
  | "active"
  | "gap"
  | "after";

export type ListenLyricsFocusContextFrame = {
  phase: ListenLyricsFocusContextPhase;
  primaryIndex: number;
  previousIndex: number;
  nextIndex: number;
  previousGapMs: number | null;
  nextLeadMs: number | null;
};

type SegmenterPart = {
  segment: string;
};

type SegmenterLike = {
  segment: (input: string) => Iterable<SegmenterPart>;
};

type SegmenterConstructor = new (
  locale?: string | string[],
  options?: { granularity: "grapheme" },
) => SegmenterLike;

let listenLyricsGraphemeSegmenter: SegmenterLike | null | undefined;

/** Resolves visual neighbours without ever promoting them to karaoke current. */
export function resolveListenLyricsFocusContextFrame(
  lines: readonly ListenLyricTimelineLine[],
  primaryIndex: number,
  currentTimeMs: number,
): ListenLyricsFocusContextFrame {
  const timeMs = Math.max(0, Number.isFinite(currentTimeMs) ? currentTimeMs : 0);
  const visibleIndexes = lines
    .map((line, index) => (line.text.trim() ? index : -1))
    .filter((index) => index >= 0);
  const currentPosition = visibleIndexes.indexOf(primaryIndex);

  if (currentPosition >= 0) {
    const previousIndex = visibleIndexes[currentPosition - 1] ?? -1;
    const nextIndex = visibleIndexes[currentPosition + 1] ?? -1;
    return {
      phase: "active",
      primaryIndex,
      previousIndex,
      nextIndex,
      previousGapMs:
        previousIndex >= 0
          ? Math.max(0, lines[primaryIndex].startMs - lines[previousIndex].endMs)
          : null,
      nextLeadMs:
        nextIndex >= 0
          ? Math.max(0, lines[nextIndex].startMs - lines[primaryIndex].endMs)
          : null,
    };
  }

  let previousIndex = -1;
  let nextIndex = -1;
  for (const index of visibleIndexes) {
    const line = lines[index];
    if (line.endMs <= timeMs) {
      previousIndex = index;
      continue;
    }
    if (line.startMs > timeMs) {
      nextIndex = index;
      break;
    }
  }
  const firstIndex = visibleIndexes[0] ?? -1;
  const lastIndex = visibleIndexes[visibleIndexes.length - 1] ?? -1;
  const phase: ListenLyricsFocusContextPhase =
    firstIndex < 0 || timeMs < lines[firstIndex].startMs
      ? "before"
      : lastIndex >= 0 && timeMs >= lines[lastIndex].endMs
        ? "after"
        : "gap";
  return {
    phase,
    primaryIndex: -1,
    previousIndex,
    nextIndex,
    previousGapMs:
      previousIndex >= 0 ? Math.max(0, timeMs - lines[previousIndex].endMs) : null,
    nextLeadMs:
      nextIndex >= 0 ? Math.max(0, lines[nextIndex].startMs - timeMs) : null,
  };
}

export function segmentListenLyricsFocusGraphemes(text: string) {
  const segmenter = getListenLyricsGraphemeSegmenter();
  return segmenter
    ? [...segmenter.segment(text)].map((part) => part.segment)
    : Array.from(text.normalize("NFC"));
}

function getListenLyricsGraphemeSegmenter() {
  if (listenLyricsGraphemeSegmenter !== undefined) {
    return listenLyricsGraphemeSegmenter;
  }
  const Segmenter = (Intl as unknown as { Segmenter?: SegmenterConstructor })
    .Segmenter;
  listenLyricsGraphemeSegmenter = Segmenter
    ? new Segmenter(undefined, { granularity: "grapheme" })
    : null;
  return listenLyricsGraphemeSegmenter;
}
