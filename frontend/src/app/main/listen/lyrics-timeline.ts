import type { ListenLyricsData } from "@/app/main/listen/types";

export type ListenLyricLineView = ListenLyricsData["lines"][number];
export type ListenLyricWordView = NonNullable<
  ListenLyricLineView["words"]
>[number];

export type ListenLyricTimelineLine = {
  sourceIndex: number;
  startMs: number;
  endMs: number;
  activeStartMs: number;
  activeEndMs: number;
  text: string;
  translationText: string;
  romanizedText: string;
  alternateTexts: NonNullable<ListenLyricLineView["alternateTexts"]>;
  words: ListenLyricWordView[];
};

export type ListenLyricDisplayOptions = {
  romanized?: boolean;
  pinyin?: boolean;
};

export type ListenLyricsFocusFrame = {
  primaryIndex: number;
};

export type ListenLyricWordVisualState = {
  state: "pending" | "active" | "passed";
  progress: number;
};

export const LISTEN_LYRICS_LINE_GRACE_MS = 420;

export function buildListenLyricsTimelineKey(
  lyrics: ListenLyricsData | null | undefined,
) {
  if (!lyrics || lyrics.kind !== "synced") {
    return "";
  }
  const identity = [
    lyrics.videoId.trim(),
    lyrics.providerId?.trim().toLowerCase() ?? "",
    lyrics.providerTrackId?.trim() ?? "",
    lyrics.timingQuality ?? "",
  ].join(":");
  if (lyrics.providerTrackId?.trim()) {
    return identity;
  }

  let hash = 0x811c9dc5;
  const mix = (value: string | number | boolean | undefined) => {
    const text = String(value ?? "");
    for (let index = 0; index < text.length; index += 1) {
      hash ^= text.charCodeAt(index);
      hash = Math.imul(hash, 0x01000193);
    }
    hash ^= 0xff;
    hash = Math.imul(hash, 0x01000193);
  };
  const mixWords = (words: ListenLyricWordView[] | undefined) => {
    for (const word of words ?? []) {
      mix(word.startMs);
      mix(word.endMs);
      mix(word.text);
      mix(word.endsWithSpace);
      mixWords(word.syllables);
    }
  };
  mix(lyrics.source);
  mix(lyrics.text);
  for (const line of lyrics.lines) {
    mix(line.startMs);
    mix(line.durationMs);
    mix(line.text);
    mix(line.translationText);
    mix(line.romanizedText);
    mixWords(line.words);
  }
  return `${identity}:${(hash >>> 0).toString(36)}`;
}

export function buildListenLyricsTimeline(
  lines: ListenLyricLineView[],
  displayOptions: boolean | ListenLyricDisplayOptions = false,
): ListenLyricTimelineLine[] {
  const options =
    typeof displayOptions === "boolean"
      ? { romanized: displayOptions, pinyin: displayOptions }
      : displayOptions;
  const normalized = lines
    .map((line, sourceIndex) => {
      const alternateTexts = normalizeListenLyricAlternateTexts(line);
      return {
        sourceIndex,
        startMs: Math.max(0, Math.floor(line.startMs)),
        durationMs: Math.max(0, Math.floor(line.durationMs)),
        text: line.text.trim(),
        translationText: resolveListenLyricTranslationText(
          line,
          alternateTexts,
        ),
        romanizedText: resolveListenLyricRomanizedText(
          { ...line, alternateTexts },
          options,
        ),
        alternateTexts,
        words: normalizeListenLyricWords(line),
      };
    })
    .sort(
      (left, right) =>
        left.startMs - right.startMs || left.sourceIndex - right.sourceIndex,
    );

  return normalized.map((line, index) => {
    const nextStartMs = normalized[index + 1]?.startMs;
    const durationEndMs =
      line.durationMs > 0 ? line.startMs + line.durationMs : 0;
    const naturalEndMs =
      durationEndMs > line.startMs
        ? durationEndMs
        : typeof nextStartMs === "number"
          ? nextStartMs
          : line.startMs + 5000;
    const endMs =
      typeof nextStartMs === "number" && nextStartMs > line.startMs
        ? Math.min(naturalEndMs, nextStartMs)
        : Math.max(line.startMs + 500, naturalEndMs);
    return {
      sourceIndex: line.sourceIndex,
      startMs: line.startMs,
      endMs,
      activeStartMs: line.startMs,
      activeEndMs: Math.max(
        line.startMs + 120,
        endMs + LISTEN_LYRICS_LINE_GRACE_MS,
      ),
      text: line.text,
      translationText: line.translationText,
      romanizedText: line.romanizedText,
      alternateTexts: line.alternateTexts,
      words: line.words,
    };
  });
}

export function findListenActiveLyricLineIndex(
  lines: ListenLyricTimelineLine[],
  currentTimeMs: number,
) {
  let activeIndex = -1;
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (currentTimeMs < line.activeStartMs) {
      break;
    }
    if (
      line.text &&
      currentTimeMs >= line.activeStartMs &&
      currentTimeMs < line.activeEndMs
    ) {
      activeIndex = index;
    }
  }
  return activeIndex;
}

/** Focus intentionally has a stricter clock than Dynamic. It never inherits
 * Dynamic's lead/grace window, previews an upcoming line, or holds a completed
 * line through a genuine timeline gap. */
export function resolveListenLyricsFocusFrame(
  lines: ListenLyricTimelineLine[],
  _activeIndex: number,
  currentTimeMs: number,
): ListenLyricsFocusFrame {
  let primaryIndex = -1;
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (currentTimeMs < line.startMs) {
      break;
    }
    if (
      line.text &&
      currentTimeMs >= line.startMs &&
      currentTimeMs < Math.max(line.startMs, line.endMs)
    ) {
      primaryIndex = index;
    }
  }
  if (primaryIndex < 0) {
    return { primaryIndex: -1 };
  }
  return { primaryIndex };
}

export function getListenActiveLyricWordProgress(
  words: ListenLyricWordView[],
  currentTimeMs: number,
  lineStartMs: number,
  lineEndMs: number,
) {
  let completedIndex = -1;
  for (let index = 0; index < words.length; index += 1) {
    const startMs = Math.max(lineStartMs, words[index].startMs);
    const nextStartMs = words[index + 1]?.startMs;
    const explicitEndMs = words[index].endMs;
    const endMs =
      typeof explicitEndMs === "number" && explicitEndMs > startMs
        ? explicitEndMs
        : typeof nextStartMs === "number" && nextStartMs > startMs
          ? nextStartMs
          : Math.max(startMs + 280, lineEndMs);
    if (currentTimeMs < startMs) {
      if (completedIndex >= 0) {
        return { index: completedIndex, progress: 1 };
      }
      break;
    }
    if (currentTimeMs < endMs) {
      return {
        index,
        progress: Math.max(
          0,
          Math.min(
            1,
            (currentTimeMs - startMs) / Math.max(1, endMs - startMs),
          ),
        ),
      };
    }
    completedIndex = index;
  }
  return completedIndex >= 0
    ? { index: completedIndex, progress: 1 }
    : { index: -1, progress: 0 };
}

export function getListenLyricWordVisualState(
  words: ListenLyricWordView[],
  index: number,
  currentTimeMs: number,
  lineStartMs: number,
  lineEndMs: number,
): ListenLyricWordVisualState {
  const word = words[index];
  if (!word) {
    return { state: "pending", progress: 0 };
  }
  const startMs = Math.max(lineStartMs, word.startMs);
  const nextStartMs = words[index + 1]?.startMs;
  const explicitEndMs = word.endMs;
  const endMs = Math.min(
    Math.max(startMs + 1, lineEndMs),
    typeof explicitEndMs === "number" && explicitEndMs > startMs
      ? explicitEndMs
      : typeof nextStartMs === "number" && nextStartMs > startMs
        ? nextStartMs
        : Math.max(startMs + 280, lineEndMs),
  );
  if (currentTimeMs < startMs) {
    return { state: "pending", progress: 0 };
  }
  if (currentTimeMs >= endMs) {
    return { state: "passed", progress: 1 };
  }
  return {
    state: "active",
    progress: Math.max(
      0,
      Math.min(1, (currentTimeMs - startMs) / Math.max(1, endMs - startMs)),
    ),
  };
}

/** Builds a visual-only word clock for providers that expose line timing but
 * no word timing. The source model and seek boundary remain untouched. */
export function buildListenLyricsFocusTimingUnits(
  line: Pick<
    ListenLyricTimelineLine,
    "endMs" | "startMs" | "text" | "words"
  >,
): ListenLyricWordView[] {
  if (line.words.length > 0) {
    return applyListenLyricsSourceSpacing(
      expandListenLyricTimingUnits(line.words),
      line.text,
    );
  }
  if (!line.text) {
    return [];
  }
  // Line timing is deliberately presented as one whole-line phase. Evenly
  // timed synthetic words would imply precision the provider did not send.
  return [{ startMs: line.startMs, endMs: line.endMs, text: line.text }];
}

export function resolveListenLyricRomanizedText(
  line: Pick<
    ListenLyricLineView,
    "alternateTexts" | "romanizedKind" | "romanizedText"
  >,
  options: ListenLyricDisplayOptions,
) {
  const kind = line.romanizedKind;
  if (kind === "pinyin") {
    if (options.pinyin !== true) {
      return "";
    }
    return (
      line.romanizedText?.trim() ||
      findListenLyricAlternateText(line.alternateTexts, ["pinyin"])
    );
  }
  if (options.romanized !== true) {
    return "";
  }
  return (
    line.romanizedText?.trim() ||
    findListenLyricAlternateText(line.alternateTexts, [
      "romanization",
      "romanized",
      "transliteration",
    ])
  );
}

export function resolveListenLyricTranslationText(
  line: Pick<ListenLyricLineView, "alternateTexts" | "translationText">,
  normalizedAlternates = normalizeListenLyricAlternateTexts(line),
) {
  return (
    line.translationText?.trim() ||
    findListenLyricAlternateText(normalizedAlternates, [
      "translation",
      "translated",
    ])
  );
}

/** Honors explicit spacing metadata while preserving legacy payload behavior. */
export function getListenLyricWordDisplayText(word: ListenLyricWordView) {
  if (typeof word.endsWithSpace === "boolean") {
    return `${word.text.replace(/\s+$/, "")}${word.endsWithSpace ? " " : ""}`;
  }
  return /\s$/.test(word.text) ? word.text : `${word.text} `;
}

/** Context-aware display text for the visual renderer. Explicit provider
 * spacing always wins; legacy payloads infer only the spaces that typography
 * actually needs, avoiding gaps around CJK and punctuation. */
export function getListenLyricTimingUnitDisplayText(
  words: ListenLyricWordView[],
  index: number,
) {
  const word = words[index];
  if (!word) {
    return "";
  }
  if (/\s$/.test(word.text)) {
    return word.text;
  }
  if (typeof word.endsWithSpace === "boolean") {
    return getListenLyricWordDisplayText(word);
  }
  const nextText = words[index + 1]?.text.trimStart() ?? "";
  return shouldInsertListenLyricsLegacySpace(word.text, nextText)
    ? `${word.text} `
    : word.text;
}

/** Uses the most precise timing available while retaining the parent word's
 * trailing-space contract on the final syllable. */
export function expandListenLyricTimingUnits(
  words: ListenLyricWordView[],
): ListenLyricWordView[] {
  const units: ListenLyricWordView[] = [];
  for (const word of words) {
    const syllables = word.syllables?.length
      ? expandListenLyricTimingUnits(word.syllables)
      : [];
    if (syllables.length === 0) {
      units.push(word);
      continue;
    }
    for (let index = 0; index < syllables.length; index += 1) {
      const syllable = syllables[index];
      units.push(
        index === syllables.length - 1
          ? { ...syllable, endsWithSpace: word.endsWithSpace }
          : syllable,
      );
    }
  }
  return units;
}

function normalizeListenLyricWords(line: ListenLyricLineView) {
  return [...(line.words ?? [])]
    .filter((word) => word.text.trim())
    .sort((left, right) => left.startMs - right.startMs);
}

function normalizeListenLyricAlternateTexts(
  line: Pick<ListenLyricLineView, "alternateTexts">,
) {
  return (line.alternateTexts ?? [])
    .map((alternate) => ({
      ...alternate,
      role: alternate.role.trim(),
      language: alternate.language?.trim() || undefined,
      text: alternate.text.trim(),
    }))
    .filter((alternate) => alternate.role && alternate.text);
}

function findListenLyricAlternateText(
  alternateTexts: ListenLyricLineView["alternateTexts"],
  roles: string[],
) {
  const expectedRoles = new Set(roles);
  return (
    alternateTexts?.find((alternate) =>
      expectedRoles.has(alternate.role.trim().toLowerCase()),
    )?.text.trim() || ""
  );
}

function applyListenLyricsSourceSpacing(
  words: ListenLyricWordView[],
  sourceText: string,
) {
  let cursor = 0;
  const matches = words.map((word) => {
    const needle = word.text.trim();
    if (!needle) {
      return null;
    }
    const matchIndex = sourceText.indexOf(needle, cursor);
    if (matchIndex < 0) {
      return null;
    }
    const matchEnd = matchIndex + needle.length;
    cursor = matchEnd;
    return { start: matchIndex, end: matchEnd, needle };
  });
  if (matches.some((match) => match === null)) {
    return words;
  }
  return words.map((word, index) => {
    const match = matches[index];
    if (!match) {
      return word;
    }
    const nextStart = matches[index + 1]?.start ?? sourceText.length;
    const prefix = index === 0 ? sourceText.slice(0, match.start) : "";
    const suffix = sourceText.slice(match.end, nextStart);
    const displayText = `${prefix}${match.needle}${suffix}`;
    return {
      ...word,
      text: displayText,
      endsWithSpace: /\s$/u.test(displayText),
    };
  });
}

function shouldInsertListenLyricsLegacySpace(
  currentText: string,
  nextText: string,
) {
  if (!currentText || !nextText) {
    return false;
  }
  const currentCharacters = Array.from(currentText.trimEnd());
  const currentLast = currentCharacters[currentCharacters.length - 1] ?? "";
  const nextFirst = Array.from(nextText)[0] ?? "";
  if (
    /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}]/u.test(
      currentLast,
    ) ||
    /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}]/u.test(
      nextFirst,
    )
  ) {
    return false;
  }
  if (/^[,.;:!?%…。，！？；：、)\]}》」』”’'\-]/u.test(nextFirst)) {
    return false;
  }
  if (/[(\[{《「『“‘\-/]$/u.test(currentLast)) {
    return false;
  }
  return /[\p{L}\p{N},.;:!?%…。，！？；：、)\]}》」』”’']$/u.test(
    currentLast,
  ) && /^[\p{L}\p{N}]/u.test(nextFirst);
}
