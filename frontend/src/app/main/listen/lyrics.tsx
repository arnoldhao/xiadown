import { Loader2, RotateCcw } from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { useListenLyricsVisualClock } from "@/app/main/listen/lyrics-clock";
import type { ListenLyricsFocusStyle } from "@/app/main/listen/lyrics-focus-style";
import { resolveListenLyricsIcon } from "@/app/main/listen/lyrics-icons";
import {
  projectListenLyricsDataForLocale,
  projectListenLyricsTimelineForLocale,
} from "@/app/main/listen/lyrics-chinese";
import {
  resolveListenLyricsRenderer,
  type ListenLyricsRendererMode,
  type ListenLyricsSurfaceVariant,
} from "@/app/main/listen/lyrics-renderers";
import {
  buildListenLyricsTimeline,
  buildListenLyricsTimelineKey,
  findListenActiveLyricLineIndex,
  resolveListenLyricRomanizedText,
  resolveListenLyricTranslationText,
} from "@/app/main/listen/lyrics-timeline";
import type { ListenLyricsData } from "@/app/main/listen/types";

export {
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
} from "@/app/main/listen/lyrics-timeline";
export type {
  ListenLyricsRenderer,
  ListenLyricsRendererMode,
  ListenLyricsRendererProps,
} from "@/app/main/listen/lyrics-renderers";
export type { ListenLyricsFocusStyle } from "@/app/main/listen/lyrics-focus-style";
export {
  DEFAULT_LISTEN_LYRICS_FOCUS_STYLE,
  LISTEN_LYRICS_FOCUS_STYLES,
  normalizeListenLyricsFocusStyle,
} from "@/app/main/listen/lyrics-focus-style";
export { resolveListenLyricsRenderer } from "@/app/main/listen/lyrics-renderers";

export type ListenLyricsSurfaceProps = {
  variant?: ListenLyricsSurfaceVariant;
  /** Presentation only; the scroll renderer remains the compatibility default. */
  renderer?: ListenLyricsRendererMode;
  /** Remembered independently from Dynamic/Focus and ignored by Plain. */
  focusStyle?: ListenLyricsFocusStyle;
  text: ReturnType<typeof getXiaText>;
  lyrics?: ListenLyricsData | null;
  loading?: boolean;
  error?: string;
  errorCode?: string;
  errorRetryable?: boolean;
  onRetry?: () => void;
  currentTimeMs?: number;
  /** Changes whenever the playback-to-lyric time mapping discontinuously changes. */
  clockKey?: string;
  timelineRunning?: boolean;
  playbackRate?: number;
  romanized?: boolean;
  pinyin?: boolean;
  onSeek?: (seconds: number) => void;
};

export function ListenLyricsSurface(props: ListenLyricsSurfaceProps) {
  const lyrics = props.lyrics;
  const variant = props.variant ?? "player";
  const syncedLines = lyrics?.kind === "synced" ? lyrics.lines : [];
  const timelineKey = React.useMemo(
    () => buildListenLyricsTimelineKey(lyrics),
    [lyrics],
  );
  const visualClockKey = props.clockKey
    ? `${timelineKey}\u0000${props.clockKey}`
    : timelineKey;
  const timelineLines = React.useMemo(
    () =>
      buildListenLyricsTimeline(syncedLines, {
        pinyin: props.pinyin === true,
        romanized: props.romanized === true,
      }),
    [props.pinyin, props.romanized, syncedLines],
  );
  const displayTimelineLines = React.useMemo(
    () =>
      projectListenLyricsTimelineForLocale(
        timelineLines,
        props.text.locale,
      ),
    [props.text.locale, timelineLines],
  );
  const displayPlainLyrics = React.useMemo(
    () =>
      lyrics?.kind === "plain"
        ? projectListenLyricsDataForLocale(lyrics, props.text.locale)
        : null,
    [lyrics, props.text.locale],
  );
  const visualCurrentTimeMs = useListenLyricsVisualClock({
    sourceTimeMs: props.currentTimeMs ?? 0,
    timelineKey: visualClockKey,
    running:
      props.timelineRunning === true &&
      timelineLines.length > 0 &&
      Boolean(visualClockKey),
    playbackRate: props.playbackRate,
  });
  const activeIndex = React.useMemo(
    () =>
      timelineLines.length > 0
        ? findListenActiveLyricLineIndex(
            timelineLines,
            visualCurrentTimeMs,
          )
        : -1,
    [timelineLines, visualCurrentTimeMs],
  );
  const LyricsStateIcon = resolveListenLyricsIcon(lyrics?.kind);

  if (props.loading) {
    return (
      <ListenLyricsState text={props.text}>
        <Loader2 className="h-6 w-6 listen-loading-spinner" />
        <span>{props.text.listen.loading}</span>
      </ListenLyricsState>
    );
  }

  if (props.error) {
    return (
      <ListenLyricsState text={props.text}>
        <LyricsStateIcon className="h-6 w-6" />
        <span role="alert">{props.error}</span>
        {props.errorCode ? (
          <span className="listen-lyrics-state__error-code mt-2 block">
            {props.text.listen.errorCodeLabel}: {props.errorCode}
          </span>
        ) : null}
        {props.onRetry && props.errorRetryable !== false ? (
          <button
            type="button"
            className="listen-lyrics-state__retry mt-3 inline-flex h-8 items-center justify-center gap-1.5 px-3"
            onClick={props.onRetry}
          >
            <RotateCcw className="h-3.5 w-3.5" />
            <span>{props.text.listen.retry}</span>
          </button>
        ) : null}
      </ListenLyricsState>
    );
  }

  if (!lyrics || lyrics.kind === "unavailable") {
    return (
      <ListenLyricsState text={props.text}>
        <LyricsStateIcon className="h-6 w-6" />
        <span>{props.text.listen.lyricsEmpty}</span>
      </ListenLyricsState>
    );
  }

  if (lyrics.kind === "plain") {
    const lines = buildListenPlainLyricLines(displayPlainLyrics ?? lyrics, {
      pinyin: props.pinyin,
      romanized: props.romanized,
    });
    if (lines.length === 0) {
      return (
        <ListenLyricsState text={props.text}>
          <LyricsStateIcon className="h-6 w-6" />
          <span>{props.text.listen.lyricsEmpty}</span>
        </ListenLyricsState>
      );
    }
    return (
      <ListenPlainLyricsRenderer
        variant={variant}
        lines={lines}
        timingQuality={lyrics.timingQuality}
      />
    );
  }

  if (timelineLines.length === 0) {
    return (
      <ListenLyricsState text={props.text}>
        <LyricsStateIcon className="h-6 w-6" />
        <span>{props.text.listen.lyricsEmpty}</span>
      </ListenLyricsState>
    );
  }

  const LyricsRenderer = resolveListenLyricsRenderer(props.renderer);
  return (
    <LyricsRenderer
      variant={variant}
      timelineKey={visualClockKey}
      lines={displayTimelineLines}
      activeIndex={activeIndex}
      currentTimeMs={visualCurrentTimeMs}
      timelineRunning={props.timelineRunning === true}
      timingQuality={lyrics.timingQuality}
      focusStyle={props.focusStyle}
      onSeek={props.onSeek}
    />
  );
}

type ListenPlainLyricLine = {
  text: string;
  translationText: string;
  romanizedText: string;
};

function buildListenPlainLyricLines(
  lyrics: ListenLyricsData,
  display: { romanized?: boolean; pinyin?: boolean },
): ListenPlainLyricLine[] {
  return lyrics.text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line, index) => {
      const responseLine = lyrics.lines[index];
      const matchedLine =
        responseLine?.text.trim() === line ? responseLine : undefined;
      return {
        text: line,
        translationText: resolveListenLyricTranslationText({
          alternateTexts: matchedLine?.alternateTexts,
          translationText: matchedLine?.translationText,
        }),
        romanizedText: resolveListenLyricRomanizedText(
          {
            alternateTexts: matchedLine?.alternateTexts,
            romanizedKind: matchedLine?.romanizedKind,
            romanizedText: matchedLine?.romanizedText?.trim(),
          },
          {
            pinyin: display.pinyin === true,
            romanized: display.romanized === true,
          },
        ),
      };
    });
}

function ListenPlainLyricsRenderer(props: {
  variant: ListenLyricsSurfaceVariant;
  lines: ListenPlainLyricLine[];
  timingQuality?: ListenLyricsData["timingQuality"];
}) {
  return (
    <div
      data-listen-lyrics-renderer="plain"
      data-listen-lyrics-variant={props.variant}
      data-listen-lyrics-timing={props.timingQuality}
      className="relative h-full w-full overflow-hidden"
    >
      <div
        className={cn(
          "h-full overflow-y-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
          props.variant === "companion"
            ? "px-5 py-6 sm:px-6"
            : "px-3 py-5 sm:px-5",
        )}
      >
        <div className="listen-plain-lyrics__track mx-auto max-w-2xl space-y-4 pb-10 pt-2">
          {props.lines.map((line, index) => (
            <div
              key={`${index}-${line.text}`}
              className={cn(
                "listen-plain-lyrics__line break-words",
                props.variant === "companion"
                  ? "listen-plain-lyrics__line--companion"
                  : "listen-plain-lyrics__line--player",
              )}
            >
              <div>{line.text}</div>
              {line.translationText ? (
                <div
                  className="listen-lyrics-line__translation mt-1"
                  data-listen-lyric-support="translation"
                >
                  {line.translationText}
                </div>
              ) : null}
              {line.romanizedText ? (
                <div
                  className="listen-lyrics-line__romanization mt-1"
                  data-listen-lyric-support="romanization"
                >
                  {line.romanizedText}
                </div>
              ) : null}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function ListenLyricsState(props: {
  text: ReturnType<typeof getXiaText>;
  children: React.ReactNode;
}) {
  return (
    <div className="listen-lyrics-state relative flex h-full w-full flex-col items-center justify-center overflow-hidden px-6">
      <div className="listen-lyrics-state__icon mx-auto flex h-14 w-14 items-center justify-center">
        {React.Children.toArray(props.children)[0]}
      </div>
      <div className="listen-lyrics-state__title mt-4">
        {props.text.listen.lyrics}
      </div>
      <div className="listen-lyrics-state__message mt-1 max-w-full break-words">
        {React.Children.toArray(props.children).slice(1)}
      </div>
    </div>
  );
}
