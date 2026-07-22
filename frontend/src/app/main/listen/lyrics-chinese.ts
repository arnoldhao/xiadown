import { ConverterBuilder, type ConverterFunction } from "opencc-js/core";
import * as simplifiedToTraditionalPreset from "opencc-js/preset/cn2t";
import * as traditionalToSimplifiedPreset from "opencc-js/preset/t2cn";

import type { ListenLyricsCandidateTrack } from "@/app/main/listen/lyrics-api";
import type { ListenLyricTimelineLine } from "@/app/main/listen/lyrics-timeline";
import type {
  ListenLyricAlternateText,
  ListenLyricsData,
  ListenLyricWord,
} from "@/app/main/listen/types";

export type ListenLyricsChineseLocale = "zh-CN" | "zh-TW";

const HAN_SCRIPT_PATTERN = /\p{Script=Han}/u;
const KANA_SCRIPT_PATTERN =
  /[\p{Script=Hiragana}\p{Script=Katakana}\u30fc\uff70]/u;
const ROMANIZATION_ROLES = new Set([
  "pinyin",
  "romanization",
  "romanized",
  "romaji",
  "transliteration",
]);
const TRANSLATION_ROLES = new Set(["translation", "translated"]);
const MAX_CHINESE_SEARCH_IDENTITIES = 4;

let simplifiedConverter: ConverterFunction | undefined;
let taiwanTraditionalConverter: ConverterFunction | undefined;

export function resolveListenLyricsChineseLocale(
  locale: string | undefined,
): ListenLyricsChineseLocale | null {
  const normalized = String(locale ?? "")
    .trim()
    .replace(/_/g, "-")
    .toLowerCase();
  if (normalized !== "zh" && !normalized.startsWith("zh-")) {
    return null;
  }
  const parts = normalized.split("-");
  if (
    parts.includes("hant") ||
    parts.includes("tw") ||
    parts.includes("hk") ||
    parts.includes("mo")
  ) {
    return "zh-TW";
  }
  return "zh-CN";
}

export function buildListenLyricsChineseSearchTrackVariants<
  Track extends ListenLyricsCandidateTrack,
>(track: Track, locale: string | undefined): Track[] {
  const chineseLocale = resolveListenLyricsChineseLocale(locale);
  if (
    !chineseLocale ||
    !hasHanScript(track.title, track.artist ?? track.channel) ||
    hasKanaScript(track.title, track.artist, track.channel)
  ) {
    return [track];
  }

  const variants: Track[] = [];
  const seen = new Set<string>();
  const append = (candidate: Track) => {
    if (variants.length >= MAX_CHINESE_SEARCH_IDENTITIES) {
      return;
    }
    const key = [candidate.title, candidate.artist ?? candidate.channel ?? ""]
      .map(normalizeSearchIdentity)
      .join("\u0000");
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    variants.push(candidate);
  };

  append(track);
  const simplified = projectSearchTrack(track, convertToSimplifiedChinese);
  const traditional = projectSearchTrack(
    track,
    convertToTaiwanTraditionalChinese,
  );
  const [preferred, alternate] =
    chineseLocale === "zh-TW"
      ? [traditional, simplified]
      : [simplified, traditional];

  append(preferred);
  append(alternate);
  [
    combineSearchTrackIdentity(track, simplified, traditional),
    combineSearchTrackIdentity(track, traditional, simplified),
  ]
    .sort(
      (left, right) =>
        searchTrackIdentityDifference(track, right) -
        searchTrackIdentityDifference(track, left),
    )
    .forEach(append);
  return variants;
}

export function projectListenLyricsDataForLocale(
  lyrics: ListenLyricsData,
  locale: string | undefined,
): ListenLyricsData {
  const converter = displayConverter(locale);
  if (!converter) {
    return lyrics;
  }
  return {
    ...lyrics,
    text: projectMultilineText(lyrics.text, converter),
    lines: lyrics.lines.map((line) => {
      const primaryContainsKana =
        hasKanaScript(line.text) || wordsContainKana(line.words);
      const projectedText = projectText(line.text, converter);
      return {
        ...line,
        text: projectedText,
        translationText: projectTranslationText(
          line.translationText,
          line.alternateTexts,
          converter,
        ),
        romanizedText: line.romanizedText,
        alternateTexts: projectAlternateTexts(line.alternateTexts, converter),
        words: primaryContainsKana
          ? line.words
          : projectWordsInContext(
              line.words,
              line.text,
              projectedText,
              converter,
            ),
      };
    }),
  };
}

export function projectListenLyricsTimelineForLocale(
  lines: ListenLyricTimelineLine[],
  locale: string | undefined,
): ListenLyricTimelineLine[] {
  const converter = displayConverter(locale);
  if (!converter) {
    return lines;
  }
  return lines.map((line) => {
    const primaryContainsKana =
      hasKanaScript(line.text) || wordsContainKana(line.words);
    const projectedText = projectText(line.text, converter);
    return {
      ...line,
      text: projectedText,
      translationText: projectTranslationText(
        line.translationText,
        line.alternateTexts,
        converter,
      ),
      romanizedText: line.romanizedText,
      alternateTexts:
        projectAlternateTexts(line.alternateTexts, converter) ?? [],
      words: primaryContainsKana
        ? line.words
        : projectWordsInContext(
            line.words,
            line.text,
            projectedText,
            converter,
          ) ?? [],
    };
  });
}

export function projectListenLyricsDisplayText(
  value: string,
  locale: string | undefined,
) {
  const converter = displayConverter(locale);
  return converter ? projectText(value, converter) : value;
}

function projectSearchTrack<Track extends ListenLyricsCandidateTrack>(
  track: Track,
  converter: ConverterFunction,
): Track {
  if (track.artist !== undefined) {
    return {
      ...track,
      title: converter(track.title),
      artist: converter(track.artist),
    };
  }
  if (track.channel !== undefined) {
    return {
      ...track,
      title: converter(track.title),
      channel: converter(track.channel),
    };
  }
  return { ...track, title: converter(track.title) };
}

function combineSearchTrackIdentity<Track extends ListenLyricsCandidateTrack>(
  track: Track,
  titleSource: Track,
  artistSource: Track,
): Track {
  if (track.artist !== undefined) {
    return {
      ...track,
      title: titleSource.title,
      artist: artistSource.artist ?? track.artist,
    };
  }
  if (track.channel !== undefined) {
    return {
      ...track,
      title: titleSource.title,
      channel: artistSource.channel ?? track.channel,
    };
  }
  return { ...track, title: titleSource.title };
}

function searchTrackIdentityDifference(
  left: ListenLyricsCandidateTrack,
  right: ListenLyricsCandidateTrack,
) {
  return (
    searchTextDifference(left.title, right.title) +
    searchTextDifference(
      left.artist ?? left.channel ?? "",
      right.artist ?? right.channel ?? "",
    )
  );
}

function searchTextDifference(left: string, right: string) {
  const leftCharacters = Array.from(normalizeSearchIdentity(left));
  const rightCharacters = Array.from(normalizeSearchIdentity(right));
  const length = Math.max(leftCharacters.length, rightCharacters.length);
  let difference = 0;
  for (let index = 0; index < length; index += 1) {
    if (leftCharacters[index] !== rightCharacters[index]) {
      difference += 1;
    }
  }
  return difference;
}

function projectAlternateTexts(
  values: ListenLyricAlternateText[] | undefined,
  converter: ConverterFunction,
) {
  return values?.map((alternate) => ({
    ...alternate,
    text:
      ROMANIZATION_ROLES.has(alternate.role.trim().toLowerCase()) ||
      hasExplicitNonChineseLanguage(alternate.language)
        ? alternate.text
        : projectText(alternate.text, converter),
  }));
}

function projectTranslationText<Value extends string | undefined>(
  value: Value,
  alternateTexts: ListenLyricAlternateText[] | undefined,
  converter: ConverterFunction,
): Value {
  if (!value) {
    return value;
  }
  const selectedAlternate = alternateTexts?.find(
    (alternate) =>
      TRANSLATION_ROLES.has(alternate.role.trim().toLowerCase()) &&
      alternate.text.trim() === value.trim(),
  );
  const projected =
    selectedAlternate &&
    hasExplicitNonChineseLanguage(selectedAlternate.language)
    ? value
    : projectText(value, converter);
  return projected as Value;
}

function projectWords(
  words: ListenLyricWord[] | undefined,
  converter: ConverterFunction,
): ListenLyricWord[] | undefined {
  return words?.map((word) => ({
    ...word,
    text: converter(word.text),
    syllables: projectWords(word.syllables, converter),
  }));
}

function projectWordsInContext(
  words: ListenLyricWord[] | undefined,
  sourceText: string,
  projectedText: string,
  converter: ConverterFunction,
): ListenLyricWord[] | undefined {
  if (!words) {
    return undefined;
  }
  const sourceCharacters = Array.from(sourceText);
  const projectedCharacters = Array.from(projectedText);
  if (sourceCharacters.length !== projectedCharacters.length) {
    return projectWords(words, converter);
  }

  let cursor = 0;
  const matches = words.map((word) => {
    const needle = word.text.trim();
    if (!needle) {
      return null;
    }
    const start = sourceText.indexOf(needle, cursor);
    if (start < 0) {
      return null;
    }
    const end = start + needle.length;
    cursor = end;
    const characterStart = Array.from(sourceText.slice(0, start)).length;
    const characterEnd =
      characterStart + Array.from(sourceText.slice(start, end)).length;
    return {
      needle,
      projected: projectedCharacters
        .slice(characterStart, characterEnd)
        .join(""),
    };
  });
  if (matches.some((match) => match === null)) {
    return projectWords(words, converter);
  }

  return words.map((word, index) => {
    const match = matches[index];
    if (!match) {
      return projectWord(word, converter);
    }
    const needleStart = word.text.indexOf(match.needle);
    const prefix = needleStart > 0 ? word.text.slice(0, needleStart) : "";
    const suffix = word.text.slice(needleStart + match.needle.length);
    const projectedWordText = `${prefix}${match.projected}${suffix}`;
    return {
      ...word,
      text: projectedWordText,
      syllables: projectWordsInContext(
        word.syllables,
        word.text,
        projectedWordText,
        converter,
      ),
    };
  });
}

function projectWord(
  word: ListenLyricWord,
  converter: ConverterFunction,
): ListenLyricWord {
  return {
    ...word,
    text: converter(word.text),
    syllables: projectWords(word.syllables, converter),
  };
}

function wordsContainKana(words: ListenLyricWord[] | undefined): boolean {
  return (words ?? []).some(
    (word) =>
      hasKanaScript(word.text) || wordsContainKana(word.syllables),
  );
}

function displayConverter(locale: string | undefined) {
  switch (resolveListenLyricsChineseLocale(locale)) {
    case "zh-CN":
      return convertToSimplifiedChinese;
    case "zh-TW":
      return convertToTaiwanTraditionalChinese;
    default:
      return null;
  }
}

function convertToSimplifiedChinese(value: string) {
  simplifiedConverter ??= ConverterBuilder(traditionalToSimplifiedPreset)({
    from: "t",
    to: "cn",
  });
  return simplifiedConverter(value);
}

function convertToTaiwanTraditionalChinese(value: string) {
  taiwanTraditionalConverter ??= ConverterBuilder(
    simplifiedToTraditionalPreset,
  )({
    from: "cn",
    to: "tw",
  });
  return taiwanTraditionalConverter(value);
}

function hasHanScript(...values: Array<string | undefined>) {
  return values.some((value) => HAN_SCRIPT_PATTERN.test(value ?? ""));
}

function hasKanaScript(...values: Array<string | undefined>) {
  return values.some((value) => KANA_SCRIPT_PATTERN.test(value ?? ""));
}

function hasExplicitNonChineseLanguage(language: string | undefined) {
  const normalized = String(language ?? "").trim();
  return normalized !== "" && !resolveListenLyricsChineseLocale(normalized);
}

function projectText(value: string, converter: ConverterFunction) {
  return hasKanaScript(value) ? value : converter(value);
}

function projectMultilineText(value: string, converter: ConverterFunction) {
  return value.replace(/[^\r\n]+/g, (line) => projectText(line, converter));
}

function normalizeSearchIdentity(value: string) {
  return value.normalize("NFKC").trim().toLocaleLowerCase().replace(/\s+/g, " ");
}
