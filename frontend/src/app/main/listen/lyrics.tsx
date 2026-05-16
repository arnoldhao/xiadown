import { Loader2,RotateCcw } from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { resolveListenLyricsIcon } from "@/app/main/listen/lyrics-icons";
import type { ListenLyricsData } from "@/app/main/listen/types";

type ListenLyricLineView = ListenLyricsData["lines"][number];
type ListenLyricWordView = NonNullable<ListenLyricLineView["words"]>[number];
type ListenLyricTimelineLine = {
  sourceIndex: number;
  startMs: number;
  endMs: number;
  activeStartMs: number;
  activeEndMs: number;
  text: string;
  romanizedText: string;
  words: ListenLyricWordView[];
};

const LISTEN_LYRICS_SCROLL_DURATION_MS = 560;
const LISTEN_LYRICS_LINE_LEAD_MS = 160;
const LISTEN_LYRICS_LINE_GRACE_MS = 420;
const LISTEN_LYRICS_WORD_LEAD_MS = 60;
const LISTEN_LYRICS_MANUAL_SCROLL_LOCK_MS = 4200;

function easeOutListenLyricsScroll(progress: number) {
  const clamped = Math.max(0, Math.min(1, progress));
  return 1 - Math.pow(1 - clamped, 3);
}

export function ListenLyricsSurface(props: {
  text: ReturnType<typeof getXiaText>;
  lyrics?: ListenLyricsData | null;
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
  currentTimeMs?: number;
  timelineRunning?: boolean;
  romanized?: boolean;
  pinyin?: boolean;
  onSeek?: (seconds: number) => void;
}) {
  const activeLineRef = React.useRef<HTMLDivElement | null>(null);
  const scrollContainerRef = React.useRef<HTMLDivElement | null>(null);
  const scrollAnimationRef = React.useRef<number | null>(null);
  const scrollMeasureFrameRef = React.useRef<number | null>(null);
  const programmaticScrollUntilRef = React.useRef(0);
  const manualScrollUnlockTimerRef = React.useRef<number | null>(null);
  const manualScrollLockedRef = React.useRef(false);
  const visualClockRef = React.useRef({
    sourceMs: 0,
    anchorMs: 0,
    running: false,
    key: "",
  });
  const lastCenteredLineRef = React.useRef<{
    videoId: string;
    activeIndex: number;
    timeMs: number;
  } | null>(null);
  const [lyricsViewportPadding, setLyricsViewportPadding] =
    React.useState(32);
  const [manualScrollLocked, setManualScrollLocked] = React.useState(false);
  const lyrics = props.lyrics;
  const LyricsStateIcon = resolveListenLyricsIcon(lyrics?.kind);
  const syncedLines = lyrics?.kind === "synced" ? lyrics.lines : [];
  const currentTimeMs = Math.max(0, props.currentTimeMs ?? 0);
  const timelineRunning = props.timelineRunning === true;
  const timelineClockKey =
    lyrics?.kind === "synced" ? `${lyrics.videoId}:synced` : "";
  const [visualCurrentTimeMs, setVisualCurrentTimeMs] =
    React.useState(currentTimeMs);
  const visualCurrentTimeRef = React.useRef(visualCurrentTimeMs);
  const timelineLines = React.useMemo(
    () =>
      buildListenLyricsTimeline(syncedLines, {
        pinyin: props.pinyin === true,
        romanized: props.romanized === true,
      }),
    [props.pinyin, props.romanized, syncedLines],
  );
  const timelineClockRunning =
    timelineRunning && timelineLines.length > 0 && Boolean(timelineClockKey);
  const activeIndex = React.useMemo(() => {
    if (timelineLines.length === 0) {
      return -1;
    }
    return findListenActiveLyricLineIndex(
      timelineLines,
      visualCurrentTimeMs,
    );
  }, [timelineLines, visualCurrentTimeMs]);

  React.useEffect(() => {
    manualScrollLockedRef.current = manualScrollLocked;
  }, [manualScrollLocked]);

  visualCurrentTimeRef.current = visualCurrentTimeMs;

  React.useEffect(() => {
    manualScrollLockedRef.current = false;
    setManualScrollLocked(false);
    if (manualScrollUnlockTimerRef.current !== null) {
      window.clearTimeout(manualScrollUnlockTimerRef.current);
      manualScrollUnlockTimerRef.current = null;
    }
  }, [lyrics?.videoId]);

  const cancelLyricScrollAnimation = React.useCallback(() => {
    if (scrollAnimationRef.current !== null) {
      window.cancelAnimationFrame(scrollAnimationRef.current);
      scrollAnimationRef.current = null;
    }
  }, []);

  const centerActiveLyricLine = React.useCallback(
    (behavior: "auto" | "smooth") => {
      const container = scrollContainerRef.current;
      const line = activeLineRef.current;
      if (!container || !line || activeIndex < 0) {
        return;
      }
      if (manualScrollLockedRef.current) {
        return;
      }
      const containerRect = container.getBoundingClientRect();
      const lineRect = line.getBoundingClientRect();
      const target = Math.max(
        0,
        Math.min(
          container.scrollTop +
            lineRect.top -
            containerRect.top +
            lineRect.height / 2 -
            container.clientHeight / 2,
          Math.max(0, container.scrollHeight - container.clientHeight),
        ),
      );
      if (Math.abs(container.scrollTop - target) < 0.75) {
        return;
      }
      const reducedMotion =
        typeof window.matchMedia === "function" &&
        window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      cancelLyricScrollAnimation();
      if (behavior === "auto" || reducedMotion) {
        programmaticScrollUntilRef.current = performance.now() + 220;
        container.scrollTop = target;
        return;
      }
      const startTop = container.scrollTop;
      const distance = target - startTop;
      const startedAt = performance.now();
      const tick = (now: number) => {
        const progress =
          (now - startedAt) / LISTEN_LYRICS_SCROLL_DURATION_MS;
        programmaticScrollUntilRef.current = now + 220;
        container.scrollTop =
          startTop + distance * easeOutListenLyricsScroll(progress);
        if (progress < 1) {
          scrollAnimationRef.current = window.requestAnimationFrame(tick);
        } else {
          scrollAnimationRef.current = null;
          programmaticScrollUntilRef.current = performance.now() + 360;
          container.scrollTop = target;
        }
      };
      scrollAnimationRef.current = window.requestAnimationFrame(tick);
    },
    [activeIndex, cancelLyricScrollAnimation],
  );

  const scheduleActiveLyricCenter = React.useCallback(
    (behavior: "auto" | "smooth") => {
      if (scrollMeasureFrameRef.current !== null) {
        window.cancelAnimationFrame(scrollMeasureFrameRef.current);
      }
      scrollMeasureFrameRef.current = window.requestAnimationFrame(() => {
        scrollMeasureFrameRef.current = null;
        centerActiveLyricLine(behavior);
      });
    },
    [centerActiveLyricLine],
  );

  React.useEffect(() => {
    const now = performance.now();
    const clock = visualClockRef.current;
    const predicted =
      clock.running && clock.key === timelineClockKey
        ? clock.sourceMs + now - clock.anchorMs
        : clock.sourceMs;
    const drift = currentTimeMs - predicted;
    const nextSourceMs =
      timelineClockRunning &&
      clock.key === timelineClockKey &&
      Math.abs(drift) < 500
        ? Math.max(0, predicted + drift * 0.35)
        : currentTimeMs;
    visualClockRef.current = {
      sourceMs: nextSourceMs,
      anchorMs: now,
      running: timelineClockRunning,
      key: timelineClockKey,
    };
    setVisualCurrentTimeMs(nextSourceMs);
  }, [currentTimeMs, timelineClockKey, timelineClockRunning]);

  React.useEffect(() => {
    if (!timelineClockRunning || !timelineClockKey) {
      return;
    }
    let frame = 0;
    let lastPaintAt = 0;
    const tick = (now: number) => {
      const clock = visualClockRef.current;
      if (
        clock.running &&
        clock.key === timelineClockKey &&
        now - lastPaintAt >= 32
      ) {
        lastPaintAt = now;
        setVisualCurrentTimeMs(
          Math.max(0, clock.sourceMs + now - clock.anchorMs),
        );
      }
      frame = window.requestAnimationFrame(tick);
    };
    frame = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frame);
  }, [timelineClockKey, timelineClockRunning]);

  React.useEffect(() => {
    return () => {
      cancelLyricScrollAnimation();
      if (scrollMeasureFrameRef.current !== null) {
        window.cancelAnimationFrame(scrollMeasureFrameRef.current);
        scrollMeasureFrameRef.current = null;
      }
      if (manualScrollUnlockTimerRef.current !== null) {
        window.clearTimeout(manualScrollUnlockTimerRef.current);
        manualScrollUnlockTimerRef.current = null;
      }
    };
  }, [cancelLyricScrollAnimation]);

  React.useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) {
      return;
    }
    const handleScroll = () => {
      if (performance.now() < programmaticScrollUntilRef.current) {
        return;
      }
      manualScrollLockedRef.current = true;
      setManualScrollLocked(true);
      if (manualScrollUnlockTimerRef.current !== null) {
        window.clearTimeout(manualScrollUnlockTimerRef.current);
      }
      manualScrollUnlockTimerRef.current = window.setTimeout(() => {
        manualScrollLockedRef.current = false;
        setManualScrollLocked(false);
        scheduleActiveLyricCenter("smooth");
      }, LISTEN_LYRICS_MANUAL_SCROLL_LOCK_MS);
    };
    container.addEventListener("scroll", handleScroll, { passive: true });
    return () => container.removeEventListener("scroll", handleScroll);
  }, [lyrics?.videoId, scheduleActiveLyricCenter]);

  React.useLayoutEffect(() => {
    const container = scrollContainerRef.current;
    if (!container || timelineLines.length === 0) {
      return;
    }
    const syncPadding = () => {
      const nextPadding = Math.max(24, container.clientHeight / 2 - 44);
      setLyricsViewportPadding((current) =>
        Math.abs(current - nextPadding) < 1 ? current : nextPadding,
      );
    };
    syncPadding();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(() => {
      syncPadding();
      scheduleActiveLyricCenter("auto");
    });
    observer.observe(container);
    if (activeLineRef.current) {
      observer.observe(activeLineRef.current);
    }
    return () => observer.disconnect();
  }, [activeIndex, scheduleActiveLyricCenter, timelineLines.length]);

  React.useLayoutEffect(() => {
    if (activeIndex < 0) {
      lastCenteredLineRef.current = null;
      return;
    }
    const videoId = lyrics?.videoId ?? "";
    const previous = lastCenteredLineRef.current;
    const lineChanged =
      !previous ||
      previous.videoId !== videoId ||
      previous.activeIndex !== activeIndex;
    const nearbyTimelineMove =
      previous?.videoId === videoId &&
      Math.abs(previous.activeIndex - activeIndex) <= 3 &&
      Math.abs(previous.timeMs - visualCurrentTimeRef.current) < 8000;
    lastCenteredLineRef.current = {
      videoId,
      activeIndex,
      timeMs: visualCurrentTimeRef.current,
    };
    scheduleActiveLyricCenter(
      lineChanged && nearbyTimelineMove ? "smooth" : "auto",
    );
  }, [
    activeIndex,
    lyrics?.videoId,
    lyricsViewportPadding,
    scheduleActiveLyricCenter,
  ]);

  if (props.loading) {
    return (
      <ListenLyricsState text={props.text}>
        <Loader2 className="h-6 w-6 animate-spin" />
        <span>{props.text.listen.loading}</span>
      </ListenLyricsState>
    );
  }

  if (props.error) {
    return (
      <ListenLyricsState text={props.text}>
        <LyricsStateIcon className="h-6 w-6" />
        <span className="whitespace-pre-wrap">{props.error}</span>
        {props.onRetry ? (
          <button
            type="button"
            className="mt-3 inline-flex h-8 items-center justify-center gap-1.5 rounded-full bg-sidebar-primary/10 px-3 text-xs font-semibold text-sidebar-primary transition hover:bg-sidebar-primary/16 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sidebar-primary/40"
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
    const lines = lyrics.text
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line, index) => {
        const responseLine = lyrics.lines[index];
        const responseRomanized =
          responseLine?.text.trim() === line
            ? responseLine.romanizedText?.trim()
            : "";
        return {
          text: line,
          romanizedText: resolveListenLyricRomanizedText(
            {
              romanizedKind: responseLine?.romanizedKind,
              romanizedText: responseRomanized,
            },
            {
              pinyin: props.pinyin === true,
              romanized: props.romanized === true,
            },
          ),
        };
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
      <div className="relative h-full w-full overflow-hidden">
        <div className="h-full overflow-y-auto px-3 py-5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden sm:px-5">
          <div className="mx-auto max-w-2xl space-y-4 pb-10 pt-2 text-left">
            {lines.map((line, index) => (
              <div
                key={`${index}-${line.text}`}
                className="break-words text-lg font-semibold leading-8 text-sidebar-foreground/78 sm:text-xl sm:leading-9"
              >
                <div>{line.text}</div>
                {line.romanizedText ? (
                  <div className="mt-1 text-sm font-medium leading-6 text-sidebar-foreground/48 sm:text-base">
                    {line.romanizedText}
                  </div>
                ) : null}
              </div>
            ))}
            <ListenLyricsSource source={lyrics.source} />
          </div>
        </div>
      </div>
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

  return (
    <div className="relative h-full w-full overflow-hidden">
      <div
        ref={scrollContainerRef}
        className="h-full overflow-y-auto px-3 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden sm:px-5"
      >
        <div
          className="mx-auto max-w-2xl text-left"
          style={{
            paddingBottom: lyricsViewportPadding + 24,
            paddingTop: lyricsViewportPadding,
          }}
        >
          {timelineLines.map((line, index) => {
            const active = index === activeIndex;
            const previous =
              !active &&
              line.endMs + LISTEN_LYRICS_LINE_GRACE_MS <
                visualCurrentTimeMs;
            const text = line.text;
            const words = line.words;
            const seekable = Boolean(props.onSeek);
            const handleSeek = () => {
              props.onSeek?.(line.startMs / 1000);
            };
            if (!text) {
              return (
                <div
                  key={`${line.startMs}-${line.sourceIndex}-${index}`}
                  className="h-8"
                />
              );
            }
            return (
              <div
                key={`${line.startMs}-${line.sourceIndex}-${text}`}
                ref={active ? activeLineRef : undefined}
                role={seekable ? "button" : undefined}
                tabIndex={seekable ? 0 : undefined}
                onClick={seekable ? handleSeek : undefined}
                onKeyDown={
                  seekable
                    ? (event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          handleSeek();
                        }
                      }
                    : undefined
                }
                className={cn(
                  "origin-left break-words py-2.5 text-xl font-semibold leading-9 transition-[color,opacity,transform] duration-500 ease-out will-change-transform sm:text-2xl sm:leading-10",
                  seekable &&
                    "cursor-pointer rounded-md focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sidebar-primary/40",
                  active
                    ? "translate-x-2 scale-100 text-[hsl(var(--chart-1))] opacity-100"
                    : previous
                      ? "translate-x-0 scale-[0.92] text-[hsl(var(--chart-2)/0.34)] opacity-48"
                      : "translate-x-0 scale-[0.94] text-[hsl(var(--chart-3)/0.42)] opacity-62",
                )}
              >
                <div>
                  {words.length > 0 ? (
                    <ListenKaraokeLine
                      active={active}
                      currentTimeMs={visualCurrentTimeMs}
                      lineStartMs={line.startMs}
                      lineEndMs={line.endMs}
                      words={words}
                    />
                  ) : (
                    text
                  )}
                </div>
                {line.romanizedText ? (
                  <div className="mt-1 text-sm font-medium leading-6 text-sidebar-foreground/48 sm:text-base">
                    {line.romanizedText}
                  </div>
                ) : null}
              </div>
            );
          })}
          <ListenLyricsSource source={lyrics.source} />
        </div>
      </div>
    </div>
  );
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
    .map((line, sourceIndex) => ({
      sourceIndex,
      startMs: Math.max(0, Math.floor(line.startMs)),
      durationMs: Math.max(0, Math.floor(line.durationMs)),
      text: line.text.trim(),
      romanizedText: resolveListenLyricRomanizedText(line, options),
      words: normalizeListenLyricWords(line),
    }))
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
      activeStartMs: Math.max(0, line.startMs - LISTEN_LYRICS_LINE_LEAD_MS),
      activeEndMs: Math.max(
        line.startMs + 120,
        endMs + LISTEN_LYRICS_LINE_GRACE_MS,
      ),
      text: line.text,
      romanizedText: line.romanizedText,
      words: line.words,
    };
  });
}

type ListenLyricDisplayOptions = {
  romanized?: boolean;
  pinyin?: boolean;
};

function resolveListenLyricRomanizedText(
  line: Pick<ListenLyricLineView, "romanizedKind" | "romanizedText">,
  options: ListenLyricDisplayOptions,
) {
  const kind = line.romanizedKind;
  if (kind === "pinyin") {
    return options.pinyin === true ? line.romanizedText?.trim() || "" : "";
  }
  if (options.romanized !== true) {
    return "";
  }
  return line.romanizedText?.trim() || "";
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

function normalizeListenLyricWords(line: ListenLyricLineView) {
  return [...(line.words ?? [])]
    .filter((word) => word.text.trim())
    .sort((left, right) => left.startMs - right.startMs);
}

function ListenLyricsSource(props: { source: string }) {
  const source = props.source.trim();
  if (!source) {
    return null;
  }
  return (
    <div className="flex justify-center pt-6">
      <div className="listen-source-watermark flex max-w-full select-none items-center justify-center gap-1.5 overflow-hidden whitespace-nowrap text-[11px] font-medium uppercase leading-4 tracking-[0.22em]">
        <span aria-hidden="true" className="shrink-0">
          ©
        </span>
        <span className="truncate">{source}</span>
      </div>
    </div>
  );
}

export function getListenActiveLyricWordProgress(
  words: ListenLyricWordView[],
  currentTimeMs: number,
  lineStartMs: number,
  lineEndMs: number,
) {
  const visibleTimeMs = currentTimeMs + LISTEN_LYRICS_WORD_LEAD_MS;
  for (let index = 0; index < words.length; index += 1) {
    const startMs = Math.max(lineStartMs, words[index].startMs);
    const nextStartMs = words[index + 1]?.startMs;
    const endMs =
      typeof nextStartMs === "number" && nextStartMs > startMs
        ? nextStartMs
        : Math.max(startMs + 280, lineEndMs);
    if (visibleTimeMs < startMs) {
      if (index === 0 && currentTimeMs >= lineStartMs - 120) {
        return { index: 0, progress: 0 };
      }
      break;
    }
    if (visibleTimeMs <= endMs || index === words.length - 1) {
      return {
        index,
        progress: Math.max(
          0,
          Math.min(
            1,
            (visibleTimeMs - startMs) / Math.max(1, endMs - startMs),
          ),
        ),
      };
    }
  }
  return { index: -1, progress: 0 };
}

function ListenKaraokeLine(props: {
  active: boolean;
  currentTimeMs: number;
  lineStartMs: number;
  lineEndMs: number;
  words: ListenLyricWordView[];
}) {
  const activeWord = props.active
    ? getListenActiveLyricWordProgress(
        props.words,
        props.currentTimeMs,
        props.lineStartMs,
        props.lineEndMs,
      )
    : { index: -1, progress: 0 };

  return (
    <span>
      {props.words.map((word, index) => {
        const wordActive = index === activeWord.index;
        const wordPast = activeWord.index >= 0 && index < activeWord.index;
        const text = word.text;
        const fillPercent = wordActive
          ? Math.round(activeWord.progress * 1000) / 10
          : wordPast
            ? 100
            : 0;
        const wordStyle =
          props.active && wordActive
            ? ({
                backgroundImage: [
                  "linear-gradient(90deg,",
                  "hsl(var(--chart-1)) 0%,",
                  `hsl(var(--chart-1)) ${fillPercent}%,`,
                  `hsl(var(--chart-3) / 0.48) ${fillPercent}%,`,
                  "hsl(var(--chart-3) / 0.48) 100%)",
                ].join(" "),
                WebkitBackgroundClip: "text",
                backgroundClip: "text",
                color: "transparent",
              } as React.CSSProperties)
            : undefined;
        return (
          <span
            key={`${word.startMs}-${index}-${text}`}
            className={cn(
              "inline transition-colors duration-200",
              props.active &&
                (wordActive
                  ? "text-[hsl(var(--chart-1))]"
                  : wordPast
                    ? "text-[hsl(var(--chart-2)/0.78)]"
                    : "text-[hsl(var(--chart-3)/0.48)]"),
            )}
            style={wordStyle}
          >
            {text}
            {/\s$/.test(text) ? "" : " "}
          </span>
        );
      })}
    </span>
  );
}

function ListenLyricsState(props: {
  text: ReturnType<typeof getXiaText>;
  children: React.ReactNode;
}) {
  return (
    <div className="relative flex h-full w-full flex-col items-center justify-center overflow-hidden px-6 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[1.25rem] bg-sidebar-primary/10 text-sidebar-primary">
        {React.Children.toArray(props.children)[0]}
      </div>
      <div className="mt-4 text-sm font-semibold text-sidebar-foreground">
        {props.text.listen.lyrics}
      </div>
      <div className="mt-1 max-w-full break-words text-sm text-sidebar-foreground/56">
        {React.Children.toArray(props.children).slice(1)}
      </div>
    </div>
  );
}
