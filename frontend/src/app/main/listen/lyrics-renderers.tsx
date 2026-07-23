import * as React from "react";

import { cn } from "@/lib/utils";
import {
  resolveListenLyricsFocusContextFrame,
  segmentListenLyricsFocusGraphemes,
  type ListenLyricsFocusContextFrame,
} from "@/app/main/listen/lyrics-focus-context";
import {
  buildListenLyricsFocusLayout,
  normalizeListenLyricsFocusStyle,
  resolveListenLyricsFocusWordMotion,
  type ListenLyricsFocusStyle,
  type ListenLyricsFocusWordLayout,
  type ListenLyricsFocusWordState,
} from "@/app/main/listen/lyrics-focus-style";
import {
  buildListenLyricsFocusTimingUnits,
  expandListenLyricTimingUnits,
  getListenActiveLyricWordProgress,
  getListenLyricTimingUnitDisplayText,
  getListenLyricWordVisualState,
  LISTEN_LYRICS_LINE_GRACE_MS,
  resolveListenLyricsFocusFrame,
  type ListenLyricTimelineLine,
  type ListenLyricWordView,
} from "@/app/main/listen/lyrics-timeline";
import type { ListenLyricTimingQuality } from "@/app/main/listen/types";

export type ListenLyricsRendererMode = "scroll" | "focus";
export type ListenLyricsSurfaceVariant = "player" | "companion";

/** Stable input shared by every synchronized-lyrics renderer. */
export type ListenLyricsRendererProps = {
  variant: ListenLyricsSurfaceVariant;
  timelineKey: string;
  lines: ListenLyricTimelineLine[];
  activeIndex: number;
  currentTimeMs: number;
  timelineRunning?: boolean;
  timingQuality?: ListenLyricTimingQuality;
  focusStyle?: ListenLyricsFocusStyle;
  onSeek?: (seconds: number) => void;
};

export type ListenLyricsRenderer = React.ComponentType<
  ListenLyricsRendererProps
>;

const LISTEN_LYRICS_SCROLL_DURATION_MS = 560;
const LISTEN_LYRICS_MANUAL_SCROLL_LOCK_MS = 4200;
const useListenLyricsLayoutEffect =
  typeof window === "undefined" ? React.useEffect : React.useLayoutEffect;

const LISTEN_LYRICS_RENDERERS: Readonly<
  Record<ListenLyricsRendererMode, ListenLyricsRenderer>
> = {
  scroll: ListenScrollingLyricsRenderer,
  focus: ListenFocusLyricsRenderer,
};

export function resolveListenLyricsRenderer(
  mode: ListenLyricsRendererMode = "scroll",
) {
  return LISTEN_LYRICS_RENDERERS[mode];
}

export function ListenScrollingLyricsRenderer(
  props: ListenLyricsRendererProps,
) {
  const activeLineRef = React.useRef<HTMLDivElement | null>(null);
  const scrollContainerRef = React.useRef<HTMLDivElement | null>(null);
  const scrollAnimationRef = React.useRef<number | null>(null);
  const scrollMeasureFrameRef = React.useRef<number | null>(null);
  const programmaticScrollUntilRef = React.useRef(0);
  const manualScrollUnlockTimerRef = React.useRef<number | null>(null);
  const manualScrollLockedRef = React.useRef(false);
  const lastCenteredLineRef = React.useRef<{
    timelineKey: string;
    activeIndex: number;
    timeMs: number;
  } | null>(null);
  const currentTimeRef = React.useRef(props.currentTimeMs);
  const [viewportPadding, setViewportPadding] = React.useState({
    bottom: 32,
    top: 32,
  });

  currentTimeRef.current = props.currentTimeMs;

  React.useEffect(() => {
    manualScrollLockedRef.current = false;
    if (manualScrollUnlockTimerRef.current !== null) {
      window.clearTimeout(manualScrollUnlockTimerRef.current);
      manualScrollUnlockTimerRef.current = null;
    }
  }, [props.timelineKey]);

  const cancelScrollAnimation = React.useCallback(() => {
    if (scrollAnimationRef.current !== null) {
      window.cancelAnimationFrame(scrollAnimationRef.current);
      scrollAnimationRef.current = null;
    }
  }, []);

  const centerActiveLine = React.useCallback(
    (behavior: "auto" | "smooth") => {
      const container = scrollContainerRef.current;
      const line = activeLineRef.current;
      if (!container || !line || props.activeIndex < 0) {
        return;
      }
      if (manualScrollLockedRef.current) {
        return;
      }
      const containerRect = container.getBoundingClientRect();
      const lineRect = line.getBoundingClientRect();
      const anchor = resolveListenLyricsScrollAnchor(container, props.variant);
      const target = Math.max(
        0,
        Math.min(
          container.scrollTop +
            lineRect.top -
            containerRect.top +
            lineRect.height / 2 -
            container.clientHeight * anchor,
          Math.max(0, container.scrollHeight - container.clientHeight),
        ),
      );
      if (Math.abs(container.scrollTop - target) < 0.75) {
        return;
      }
      const reducedMotion =
        typeof window.matchMedia === "function" &&
        window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      cancelScrollAnimation();
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
    [cancelScrollAnimation, props.activeIndex],
  );

  const scheduleActiveLineCenter = React.useCallback(
    (behavior: "auto" | "smooth") => {
      if (scrollMeasureFrameRef.current !== null) {
        window.cancelAnimationFrame(scrollMeasureFrameRef.current);
      }
      scrollMeasureFrameRef.current = window.requestAnimationFrame(() => {
        scrollMeasureFrameRef.current = null;
        centerActiveLine(behavior);
      });
    },
    [centerActiveLine],
  );

  React.useEffect(() => {
    return () => {
      cancelScrollAnimation();
      if (scrollMeasureFrameRef.current !== null) {
        window.cancelAnimationFrame(scrollMeasureFrameRef.current);
        scrollMeasureFrameRef.current = null;
      }
      if (manualScrollUnlockTimerRef.current !== null) {
        window.clearTimeout(manualScrollUnlockTimerRef.current);
        manualScrollUnlockTimerRef.current = null;
      }
    };
  }, [cancelScrollAnimation]);

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
      if (manualScrollUnlockTimerRef.current !== null) {
        window.clearTimeout(manualScrollUnlockTimerRef.current);
      }
      manualScrollUnlockTimerRef.current = window.setTimeout(() => {
        manualScrollLockedRef.current = false;
        scheduleActiveLineCenter("smooth");
      }, LISTEN_LYRICS_MANUAL_SCROLL_LOCK_MS);
    };
    container.addEventListener("scroll", handleScroll, { passive: true });
    return () => container.removeEventListener("scroll", handleScroll);
  }, [props.timelineKey, scheduleActiveLineCenter]);

  useListenLyricsLayoutEffect(() => {
    const container = scrollContainerRef.current;
    if (!container || props.lines.length === 0) {
      return;
    }
    const syncPadding = () => {
      const activeLineHeight =
        activeLineRef.current?.getBoundingClientRect().height ?? 88;
      const edgePadding = Math.max(
        0,
        container.clientHeight / 2 - activeLineHeight / 2,
      );
      const nextPadding = {
        bottom: edgePadding,
        top: edgePadding,
      };
      setViewportPadding((current) =>
        Math.abs(current.bottom - nextPadding.bottom) < 1 &&
          Math.abs(current.top - nextPadding.top) < 1
          ? current
          : nextPadding,
      );
    };
    syncPadding();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(() => {
      syncPadding();
      scheduleActiveLineCenter("auto");
    });
    observer.observe(container);
    if (activeLineRef.current) {
      observer.observe(activeLineRef.current);
    }
    return () => observer.disconnect();
  }, [
    props.activeIndex,
    props.lines.length,
    props.variant,
    scheduleActiveLineCenter,
  ]);

  useListenLyricsLayoutEffect(() => {
    if (props.activeIndex < 0) {
      lastCenteredLineRef.current = null;
      return;
    }
    const previous = lastCenteredLineRef.current;
    const lineChanged =
      !previous ||
      previous.timelineKey !== props.timelineKey ||
      previous.activeIndex !== props.activeIndex;
    const nearbyTimelineMove =
      previous?.timelineKey === props.timelineKey &&
      Math.abs(previous.activeIndex - props.activeIndex) <= 3 &&
      Math.abs(previous.timeMs - currentTimeRef.current) < 8000;
    lastCenteredLineRef.current = {
      timelineKey: props.timelineKey,
      activeIndex: props.activeIndex,
      timeMs: currentTimeRef.current,
    };
    scheduleActiveLineCenter(
      lineChanged && nearbyTimelineMove ? "smooth" : "auto",
    );
  }, [
    props.activeIndex,
    props.timelineKey,
    scheduleActiveLineCenter,
    viewportPadding,
  ]);

  return (
    <div
      data-listen-lyrics-renderer="scroll"
      data-listen-lyrics-variant={props.variant}
      data-listen-lyrics-timing={props.timingQuality}
      className="listen-lyrics-scroll h-full w-full"
    >
      <div
        ref={scrollContainerRef}
        data-companion-scroll-owner={
          props.variant === "companion" ? "lyrics" : undefined
        }
        className={cn(
          "listen-lyrics-scroll__viewport h-full overflow-y-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
          props.variant === "companion" ? "px-5 sm:px-7" : "px-4 sm:px-6",
        )}
      >
        <div
          className="listen-lyrics-scroll__track mx-auto max-w-2xl"
          style={{
            paddingBottom: viewportPadding.bottom,
            paddingTop: viewportPadding.top,
          }}
        >
          {props.lines.map((line, index) => {
            const active = index === props.activeIndex;
            const previous =
              !active &&
              line.endMs + LISTEN_LYRICS_LINE_GRACE_MS < props.currentTimeMs;
            const seekable = Boolean(props.onSeek);
            if (!line.text) {
              return (
                <div
                  key={`${line.startMs}-${line.sourceIndex}-${index}`}
                  className="h-8"
                />
              );
            }
            return (
              <div
                key={`${line.startMs}-${line.sourceIndex}-${line.text}`}
                ref={active ? activeLineRef : undefined}
                data-lyric-state={
                  active ? "active" : previous ? "passed" : "pending"
                }
                data-lyric-distance={
                  props.activeIndex < 0
                    ? 4
                    : Math.min(4, Math.abs(index - props.activeIndex))
                }
                data-lyric-script={resolveListenLyricsScript(line.text)}
                role={seekable ? "button" : undefined}
                tabIndex={seekable ? 0 : undefined}
                aria-current={active ? "true" : undefined}
                onClick={seekable ? () => seekToLine(props, index) : undefined}
                onKeyDown={
                  seekable
                    ? (event) => handleLineActivation(event, props, index)
                    : undefined
                }
                className={cn(
                  "listen-lyrics-scroll__line origin-left break-words",
                  seekable && "listen-lyrics-scroll__line--seekable",
                )}
              >
                <div className="listen-lyrics-scroll__primary">
                  {line.words.length > 0 ? (
                    <ListenKaraokeLine
                      active={active}
                      currentTimeMs={props.currentTimeMs}
                      lineStartMs={line.startMs}
                      lineEndMs={line.endMs}
                      words={line.words}
                    />
                  ) : (
                    line.text
                  )}
                </div>
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
            );
          })}
        </div>
      </div>
    </div>
  );
}

export function ListenFocusLyricsRenderer(props: ListenLyricsRendererProps) {
  const focusStyle = normalizeListenLyricsFocusStyle(props.focusStyle);
  const frame = resolveListenLyricsFocusFrame(
    props.lines,
    props.activeIndex,
    props.currentTimeMs,
  );
  const contextFrame = resolveListenLyricsFocusContextFrame(
    props.lines,
    frame.primaryIndex,
    props.currentTimeMs,
  );
  return (
    <div
      data-listen-lyrics-renderer="focus"
      data-listen-lyrics-variant={props.variant}
      data-listen-lyrics-timing={props.timingQuality}
      data-listen-lyrics-focus-style={focusStyle}
      data-timeline-running={props.timelineRunning === true ? "true" : "false"}
      className="listen-lyrics-focus"
    >
      <ListenFocusLineDeck
        {...props}
        contextFrame={contextFrame}
        focusStyle={focusStyle}
        targetIndex={frame.primaryIndex}
      />
    </div>
  );
}

const LISTEN_LYRICS_FOCUS_CROSSFADE_MS = 300;

export type ListenLyricsFocusTransitionState = {
  timelineKey: string;
  currentIndex: number;
  outgoingIndex: number;
  revision: number;
  timeMs: number;
};

export function resolveListenLyricsFocusTransition(
  lines: ListenLyricTimelineLine[],
  timelineKey: string,
  targetIndex: number,
  currentTimeMs: number,
  previous: ListenLyricsFocusTransitionState,
): ListenLyricsFocusTransitionState {
  if (
    previous.timelineKey === timelineKey &&
    previous.currentIndex === targetIndex
  ) {
    return previous.timeMs === currentTimeMs
      ? previous
      : { ...previous, timeMs: currentTimeMs };
  }
  const sameTimeline = previous.timelineKey === timelineKey;
  const distantClockMove = Math.abs(previous.timeMs - currentTimeMs) > 1200;
  const neighboring =
    sameTimeline &&
    previous.currentIndex >= 0 &&
    targetIndex >= 0 &&
    Math.abs(previous.currentIndex - targetIndex) === 1;
  const previousLine = lines[previous.currentIndex];
  const targetLine = lines[targetIndex];
  const touchesAtBoundary =
    neighboring && previousLine && targetLine
      ? targetIndex > previous.currentIndex
        ? Math.abs(previousLine.endMs - targetLine.startMs) <= 48
        : Math.abs(targetLine.endMs - previousLine.startMs) <= 48
      : false;
  const exitsToQuietFrame =
    sameTimeline && previous.currentIndex >= 0 && targetIndex < 0;
  return {
    timelineKey,
    currentIndex: targetIndex,
    outgoingIndex:
      !distantClockMove && (touchesAtBoundary || exitsToQuietFrame)
        ? previous.currentIndex
        : -1,
    revision: previous.revision + 1,
    timeMs: currentTimeMs,
  };
}

type ListenFocusLineDeckProps = ListenLyricsRendererProps & {
  contextFrame: ListenLyricsFocusContextFrame;
  focusStyle: ListenLyricsFocusStyle;
  targetIndex: number;
};

class ListenFocusLineDeck extends React.PureComponent<
  ListenFocusLineDeckProps,
  ListenLyricsFocusTransitionState
> {
  state: ListenLyricsFocusTransitionState = {
    timelineKey: this.props.timelineKey,
    currentIndex: this.props.targetIndex,
    outgoingIndex: -1,
    revision: 0,
    timeMs: this.props.currentTimeMs,
  };

  private outgoingTimer: number | null = null;

  static getDerivedStateFromProps(
    props: ListenFocusLineDeckProps,
    state: ListenLyricsFocusTransitionState,
  ) {
    const next = resolveListenLyricsFocusTransition(
      props.lines,
      props.timelineKey,
      props.targetIndex,
      props.currentTimeMs,
      state,
    );
    if (next === state) {
      return null;
    }
    return next.outgoingIndex >= 0 &&
      (prefersReducedFocusMotion() || props.timelineRunning === false)
      ? { ...next, outgoingIndex: -1 }
      : next;
  }

  componentDidUpdate(
    previousProps: ListenFocusLineDeckProps,
    previousState: ListenLyricsFocusTransitionState,
  ) {
    if (
      previousProps.timelineRunning !== this.props.timelineRunning &&
      this.props.timelineRunning === false
    ) {
      this.clearOutgoingTimer();
      if (this.state.outgoingIndex >= 0) {
        this.setState({ outgoingIndex: -1 });
      }
      return;
    }
    if (previousState.revision === this.state.revision) {
      return;
    }
    this.clearOutgoingTimer();
    if (this.state.outgoingIndex < 0 || prefersReducedFocusMotion()) {
      if (this.state.outgoingIndex >= 0) {
        this.setState({ outgoingIndex: -1 });
      }
      return;
    }
    const revision = this.state.revision;
    this.outgoingTimer = window.setTimeout(() => {
      this.outgoingTimer = null;
      this.setState((current) =>
        current.revision === revision
          ? { outgoingIndex: -1 }
          : null,
      );
    }, LISTEN_LYRICS_FOCUS_CROSSFADE_MS);
  }

  componentWillUnmount() {
    this.clearOutgoingTimer();
  }

  private clearOutgoingTimer() {
    if (this.outgoingTimer !== null) {
      window.clearTimeout(this.outgoingTimer);
      this.outgoingTimer = null;
    }
  }

  private handlePrimaryNavigation = (
    event: React.KeyboardEvent<HTMLButtonElement>,
  ) => {
    const visibleIndexes = this.props.lines
      .map((line, index) => (line.text ? index : -1))
      .filter((index) => index >= 0);
    const position = visibleIndexes.indexOf(this.state.currentIndex);
    const previousIndex = visibleIndexes[position - 1] ?? -1;
    const nextIndex = visibleIndexes[position + 1] ?? -1;
    if (event.key === "ArrowUp" || event.key === "ArrowLeft") {
      if (previousIndex >= 0) {
        event.preventDefault();
        seekToLine(this.props, previousIndex);
      }
      return;
    }
    if (
      (event.key === "ArrowDown" || event.key === "ArrowRight") &&
      nextIndex >= 0
    ) {
      event.preventDefault();
      seekToLine(this.props, nextIndex);
    }
  };

  render() {
    const current = this.props.lines[this.state.currentIndex];
    const outgoing = this.props.lines[this.state.outgoingIndex];
    const Scene = LISTEN_FOCUS_SCENES[this.props.focusStyle];
    return (
      <div
        className="listen-lyrics-focus__stage"
        data-focus-context-phase={this.props.contextFrame.phase}
      >
        <Scene
          frame={this.props.contextFrame}
          lines={this.props.lines}
          suppressedContextIndex={this.state.outgoingIndex}
        />
        <div
          className="listen-lyrics-focus__body"
          data-focus-empty={current ? "false" : "true"}
        >
          {outgoing ? (
            <ListenFocusLine
              key="outgoing"
              {...this.props}
              index={this.state.outgoingIndex}
              line={outgoing}
              phase="outgoing"
            />
          ) : null}
          {current ? (
            <ListenFocusLine
              key="current"
              {...this.props}
              index={this.state.currentIndex}
              line={current}
              phase="current"
              onNavigate={this.handlePrimaryNavigation}
            />
          ) : null}
        </div>
      </div>
    );
  }
}

type ListenFocusSceneProps = {
  frame: ListenLyricsFocusContextFrame;
  lines: readonly ListenLyricTimelineLine[];
  suppressedContextIndex: number;
};

const LISTEN_FOCUS_SCENES: Readonly<
  Record<ListenLyricsFocusStyle, React.ComponentType<ListenFocusSceneProps>>
> = {
  prism: ListenFocusPrismScene,
  splice: ListenFocusSpliceScene,
  facet: ListenFocusFacetScene,
  pendulum: ListenFocusPendulumScene,
};

function ListenFocusPrismScene(props: ListenFocusSceneProps) {
  const context = resolveListenFocusSceneContext(props);
  return (
    <div
      className="listen-lyrics-focus__scene listen-lyrics-focus__scene--prism"
      data-focus-scene="single-flow"
      data-temporal-layout="past-present-future"
      aria-hidden="true"
    >
      <ListenFocusContextLine
        line={context.previous}
        relation="previous"
        distance={resolveListenFocusContextDistance(props.frame.previousGapMs)}
      />
      <ListenFocusContextLine
        line={context.next}
        relation="next"
        distance={resolveListenFocusContextDistance(props.frame.nextLeadMs)}
      />
    </div>
  );
}

function ListenFocusSpliceScene(props: ListenFocusSceneProps) {
  const context = resolveListenFocusSceneContext(props);
  return (
    <div
      className="listen-lyrics-focus__scene listen-lyrics-focus__scene--splice"
      data-focus-scene="splice-score"
      data-temporal-layout="past-present-future"
      aria-hidden="true"
    >
      <ListenFocusSpliceContext
        line={context.previous}
        relation="previous"
        distance={resolveListenFocusContextDistance(props.frame.previousGapMs)}
      />
      <ListenFocusSpliceContext
        line={context.next}
        relation="next"
        distance={resolveListenFocusContextDistance(props.frame.nextLeadMs)}
      />
    </div>
  );
}

function ListenFocusFacetScene(props: ListenFocusSceneProps) {
  const context = resolveListenFocusSceneContext(props);
  return (
    <div
      className="listen-lyrics-focus__scene listen-lyrics-focus__scene--facet"
      data-focus-scene="facet-camera"
      data-temporal-layout="past-present-future"
      aria-hidden="true"
    >
      <ListenFocusContextLine
        line={context.previous}
        relation="previous"
        distance={resolveListenFocusContextDistance(props.frame.previousGapMs)}
        plane
      />
      <ListenFocusContextLine
        line={context.next}
        relation="next"
        distance={resolveListenFocusContextDistance(props.frame.nextLeadMs)}
        plane
      />
    </div>
  );
}

function ListenFocusPendulumScene(props: ListenFocusSceneProps) {
  const context = resolveListenFocusSceneContext(props);
  return (
    <div
      className="listen-lyrics-focus__scene listen-lyrics-focus__scene--pendulum"
      data-focus-scene="pendulum-orbit"
      data-temporal-layout="past-present-future"
      aria-hidden="true"
    >
      <ListenFocusPendulumArc
        line={context.previous}
        relation="previous"
        distance={resolveListenFocusContextDistance(props.frame.previousGapMs)}
      />
      <ListenFocusPendulumArc
        line={context.next}
        relation="next"
        distance={resolveListenFocusContextDistance(props.frame.nextLeadMs)}
      />
    </div>
  );
}

type ListenFocusContextRelation = "previous" | "next";

function ListenFocusContextLine(props: {
  distance: "near" | "far";
  line?: ListenLyricTimelineLine;
  plane?: boolean;
  relation: ListenFocusContextRelation;
}) {
  if (!props.line) {
    return null;
  }
  return (
    <p
      className="listen-lyrics-focus__context"
      data-context-distance={props.distance}
      data-context-plane={props.plane ? "true" : undefined}
      data-context-relation={props.relation}
      data-context-state={resolveListenFocusContextState(props.relation)}
      data-temporal-role={resolveListenFocusTemporalRole(props.relation)}
    >
      <ListenFocusContextCopy line={props.line} />
    </p>
  );
}

function ListenFocusSpliceContext(props: {
  distance: "near" | "far";
  line?: ListenLyricTimelineLine;
  relation: ListenFocusContextRelation;
}) {
  if (!props.line) {
    return null;
  }
  return (
    <p
      className="listen-lyrics-focus__context listen-lyrics-focus__context--splice"
      data-context-distance={props.distance}
      data-context-relation={props.relation}
      data-context-state={resolveListenFocusContextState(props.relation)}
      data-temporal-role={resolveListenFocusTemporalRole(props.relation)}
    >
      <ListenFocusContextCopy
        line={props.line}
        slice={props.relation === "previous" ? "bottom" : "top"}
      />
    </p>
  );
}

function ListenFocusContextCopy(props: {
  line: ListenLyricTimelineLine;
  slice?: "top" | "bottom";
}) {
  const romanizedText = resolveListenFocusRomanizedText(props.line);
  return (
    <>
      <span data-context-track="primary" data-context-slice={props.slice}>
        {props.line.text}
      </span>
      {romanizedText ? (
        <span data-context-track="romanized" data-context-slice={props.slice}>
          {romanizedText}
        </span>
      ) : null}
    </>
  );
}

type ListenFocusArcCSSProperties = React.CSSProperties & {
  "--listen-lyrics-focus-arc-x": string;
  "--listen-lyrics-focus-arc-y": string;
  "--listen-lyrics-focus-arc-angle": string;
};

function ListenFocusPendulumArc(props: {
  distance: "near" | "far";
  line?: ListenLyricTimelineLine;
  relation: ListenFocusContextRelation;
}) {
  if (!props.line) {
    return null;
  }
  const graphemes = segmentListenLyricsFocusGraphemes(props.line.text);
  if (graphemes.length > 56) {
    return (
      <ListenFocusContextLine
        distance={props.distance}
        line={props.line}
        relation={props.relation}
      />
    );
  }
  const romanizedText = resolveListenFocusRomanizedText(props.line);
  return (
    <p
      className="listen-lyrics-focus__context listen-lyrics-focus__context--arc"
      data-context-distance={props.distance}
      data-context-relation={props.relation}
      data-context-state={resolveListenFocusContextState(props.relation)}
      data-temporal-role={resolveListenFocusTemporalRole(props.relation)}
    >
      <ListenFocusPendulumArcTrack
        graphemes={graphemes}
        relation={props.relation}
        track="primary"
      />
      {romanizedText ? (
        <ListenFocusPendulumArcRomanization
          relation={props.relation}
          text={romanizedText}
        />
      ) : null}
    </p>
  );
}

function ListenFocusPendulumArcRomanization(props: {
  relation: ListenFocusContextRelation;
  text: string;
}) {
  const graphemes = segmentListenLyricsFocusGraphemes(props.text);
  if (graphemes.length > 72) {
    return (
      <span data-arc-track="linear" data-context-track="romanized">
        {props.text}
      </span>
    );
  }
  return (
    <ListenFocusPendulumArcTrack
      graphemes={graphemes}
      relation={props.relation}
      track="romanized"
    />
  );
}

function ListenFocusPendulumArcTrack(props: {
  graphemes: readonly string[];
  relation: ListenFocusContextRelation;
  track: "primary" | "romanized";
}) {
  const romanized = props.track === "romanized";
  return (
    <span data-arc-track={props.track} data-context-track={props.track}>
      {props.graphemes.map((grapheme, index) => {
        const unit = props.graphemes.length <= 1
          ? 0
          : index / (props.graphemes.length - 1) * 2 - 1;
        const previous = props.relation === "previous";
        const horizontalReach = romanized
          ? previous ? 33 : 36
          : previous ? 36 : 39;
        const arcY = romanized
          ? previous ? 20 + unit * unit * 3 : 18 + unit * unit * 5
          : previous ? 14 + unit * unit * 4 : 11 + unit * unit * 7;
        const arcAngle = unit * (romanized
          ? previous ? 4 : 6
          : previous ? 7 : 11);
        const style = {
          "--listen-lyrics-focus-arc-x": `${50 + unit * horizontalReach}%`,
          "--listen-lyrics-focus-arc-y": `${arcY}%`,
          "--listen-lyrics-focus-arc-angle": `${arcAngle}deg`,
        } as ListenFocusArcCSSProperties;
        return (
          <span
            key={`${props.track}:${index}:${grapheme}`}
            data-arc-character="true"
            style={style}
          >
            {grapheme}
          </span>
        );
      })}
    </span>
  );
}

function resolveListenFocusSceneContext(props: ListenFocusSceneProps) {
  const previousIndex = props.frame.previousIndex;
  const nextIndex = props.frame.nextIndex;
  return {
    previous:
      previousIndex >= 0 && previousIndex !== props.suppressedContextIndex
        ? props.lines[previousIndex]
        : undefined,
    next:
      nextIndex >= 0 && nextIndex !== props.suppressedContextIndex
        ? props.lines[nextIndex]
        : undefined,
  };
}

function resolveListenFocusContextDistance(distanceMs: number | null) {
  return distanceMs !== null && distanceMs > 12000 ? "far" : "near";
}

function resolveListenFocusContextState(relation: ListenFocusContextRelation) {
  return relation === "previous" ? "completed" : "pending";
}

function resolveListenFocusTemporalRole(relation: ListenFocusContextRelation) {
  return relation === "previous" ? "past" : "future";
}

function ListenFocusLine(
  props: ListenLyricsRendererProps & {
    focusStyle: ListenLyricsFocusStyle;
    index: number;
    line: ListenLyricTimelineLine;
    phase: "current" | "outgoing";
    onNavigate?: (event: React.KeyboardEvent<HTMLButtonElement>) => void;
  },
) {
  const current = props.phase === "current";
  const seekable = current && Boolean(props.onSeek);
  const romanizedText = resolveListenFocusRomanizedText(props.line);
  // Remount only this non-interactive paint layer so line-entry animation can
  // restart without replacing the focused seek button at every line boundary.
  const primaryPaint = (
    <span
      key={`paint:${props.timelineKey}:${props.index}:${props.line.startMs}:${props.line.sourceIndex}:${props.focusStyle}`}
      className="listen-lyrics-focus__primary-paint"
    >
      <ListenFocusKaraokeLine
        currentTimeMs={props.currentTimeMs}
        focusStyle={props.focusStyle}
        line={props.line}
      />
      {romanizedText ? (
        <ListenFocusRomanizationPaint
          currentTimeMs={props.currentTimeMs}
          focusStyle={props.focusStyle}
          line={props.line}
          text={romanizedText}
        />
      ) : null}
    </span>
  );
  return (
    <div
      className="listen-lyrics-focus__line"
      data-has-romanization={romanizedText ? "true" : "false"}
      data-phase={props.phase}
      data-temporal-role={current ? "present" : "transition"}
      aria-hidden={current ? undefined : "true"}
      aria-current={current ? "true" : undefined}
    >
      {seekable ? (
        <button
          type="button"
          className="listen-lyrics-focus__primary"
          data-active="true"
          data-seekable="true"
          onClick={() => seekToLine(props, props.index)}
          onKeyDown={props.onNavigate}
        >
          {primaryPaint}
        </button>
      ) : (
        <div
          className="listen-lyrics-focus__primary"
          data-active={current ? "true" : "false"}
          data-seekable="false"
        >
          {primaryPaint}
        </div>
      )}
      {props.line.translationText ? (
        <div
          key={`${props.line.startMs}-${props.line.translationText}`}
          className="listen-lyrics-focus__support"
        >
          <p data-kind="translation">{props.line.translationText}</p>
        </div>
      ) : null}
    </div>
  );
}

function ListenFocusRomanizationPaint(props: {
  currentTimeMs: number;
  focusStyle: ListenLyricsFocusStyle;
  line: ListenLyricTimelineLine;
  text: string;
}) {
  const timingUnits = React.useMemo(
    () => buildListenLyricsFocusTimingUnits(props.line),
    [props.line],
  );
  const layout = React.useMemo(
    () => buildListenLyricsFocusLayout([{ text: props.text }], props.focusStyle),
    [props.focusStyle, props.text],
  );
  const activeUnit = getListenActiveLyricWordProgress(
    timingUnits,
    props.currentTimeMs,
    props.line.startMs,
    props.line.endMs,
  );
  const activeProjectedProgress = resolveListenFocusProjectedProgress(
    activeUnit.index,
    activeUnit.progress,
    timingUnits.length,
  );
  const state = resolveListenFocusLineState(props.line, props.currentTimeMs);
  const projectedProgress = state === "pending"
    ? 0
    : state === "passed"
      ? 1
      : activeProjectedProgress;
  const motion = resolveListenLyricsFocusWordMotion(
    layout.words[0] ?? {
      index: 0,
      text: props.text,
      direction: 1,
      phase: 0,
      liftEm: 0.1,
      depth: 0.9,
    },
    state,
    projectedProgress,
  );
  const style = {
    "--listen-lyrics-focus-romanization-progress": `${roundListenFocusMotionValue(projectedProgress * 100)}%`,
    "--listen-lyrics-focus-romanization-prism-offset": `${roundListenFocusMotionValue(motion.prismOffsetEm * 0.52)}em`,
    "--listen-lyrics-focus-romanization-splice-shift": `${roundListenFocusMotionValue(motion.spliceShiftEm * 0.42)}em`,
    "--listen-lyrics-focus-romanization-facet-angle": `${roundListenFocusMotionValue(motion.facetAngleDeg * 0.34)}deg`,
    "--listen-lyrics-focus-romanization-facet-depth": `${roundListenFocusMotionValue(motion.facetDepthEm * 0.42)}em`,
    "--listen-lyrics-focus-romanization-facet-line-scale": roundListenFocusMotionValue(
      1.035 - projectedProgress * 0.065,
    ),
    "--listen-lyrics-focus-romanization-facet-line-z": `${roundListenFocusMotionValue(0.3 - projectedProgress * 0.55)}rem`,
    "--listen-lyrics-focus-romanization-pendulum-angle": `${roundListenFocusMotionValue(motion.pendulumAngleDeg * -0.2)}deg`,
    "--listen-lyrics-focus-romanization-pendulum-y": `${roundListenFocusMotionValue(motion.pendulumYEm * 0.26)}em`,
    "--listen-lyrics-focus-romanization-scale": roundListenFocusMotionValue(
      1 + (motion.scale - 1) * 0.3,
    ),
  } as ListenFocusWordCSSProperties;
  return (
    <span
      className="listen-lyrics-focus__romanization"
      data-focus-romanization="true"
      data-focus-romanization-style={props.focusStyle}
      data-focus-density={layout.density}
      data-kind="romanized"
      data-listen-lyric-support="romanization"
      data-romanization-state={state}
      data-romanization-timing="projected"
      style={style}
    >
      <span className="listen-lyrics-focus__romanization-body">
        {props.text}
      </span>
    </span>
  );
}

function resolveListenFocusProjectedProgress(
  activeIndex: number,
  activeProgress: number,
  unitCount: number,
) {
  if (unitCount <= 0 || activeIndex < 0) {
    return 0;
  }
  return Math.min(
    1,
    Math.max(0, (activeIndex + Math.min(1, activeProgress)) / unitCount),
  );
}

function resolveListenFocusLineState(
  line: ListenLyricTimelineLine,
  currentTimeMs: number,
): ListenLyricsFocusWordState {
  if (currentTimeMs < line.startMs) {
    return "pending";
  }
  return currentTimeMs >= line.endMs ? "passed" : "active";
}

function resolveListenFocusRomanizedText(line: ListenLyricTimelineLine) {
  const romanizedText = line.romanizedText.trim();
  return romanizedText && romanizedText !== line.translationText.trim()
    ? romanizedText
    : "";
}

function roundListenFocusMotionValue(value: number) {
  return Math.round(value * 1000) / 1000;
}

function ListenFocusKaraokeLine(props: {
  currentTimeMs: number;
  focusStyle: ListenLyricsFocusStyle;
  line: ListenLyricTimelineLine;
}) {
  const timingUnits = React.useMemo(
    () => buildListenLyricsFocusTimingUnits(props.line),
    [props.line],
  );
  const layout = React.useMemo(
    () => buildListenLyricsFocusLayout(timingUnits, props.focusStyle),
    [props.focusStyle, timingUnits],
  );
  const estimated = props.line.words.length === 0;
  const displayTexts = React.useMemo(
    () =>
      timingUnits.map((word, index) =>
        estimated
          ? word.text
          : getListenLyricTimingUnitDisplayText(timingUnits, index),
      ),
    [estimated, timingUnits],
  );
  const lineProgress = Math.min(
    1,
    Math.max(
      0,
      (props.currentTimeMs - props.line.startMs) /
        Math.max(1, props.line.endMs - props.line.startMs),
    ),
  );
  const copyStyle = {
    "--listen-lyrics-focus-facet-line-scale":
      Math.round((1.07 - lineProgress * 0.1) * 1000) / 1000,
    "--listen-lyrics-focus-facet-line-z": `${
      Math.round((0.8 - lineProgress * 1.3) * 1000) / 1000
    }rem`,
  } as ListenFocusWordCSSProperties;
  return (
    <span
      className="listen-lyrics-focus__copy"
      data-timing-estimated={estimated ? "true" : "false"}
      data-script={resolveListenLyricsScript(props.line.text)}
      data-focus-density={layout.density}
      style={copyStyle}
    >
      {timingUnits.map((word, index) => {
        const visual = getListenLyricWordVisualState(
          timingUnits,
          index,
          props.currentTimeMs,
          props.line.startMs,
          props.line.endMs,
        );
        return (
          <ListenFocusWord
            key={`${word.startMs}-${index}-${word.text}`}
            displayText={displayTexts[index] ?? word.text}
            layout={layout.words[index]}
            progress={visual.progress}
            state={visual.state}
          />
        );
      })}
    </span>
  );
}

type ListenFocusWordProps = {
  displayText: string;
  layout?: ListenLyricsFocusWordLayout;
  progress: number;
  state: ListenLyricsFocusWordState;
};

const ListenFocusWord = React.memo(function ListenFocusWord(
  props: ListenFocusWordProps,
) {
  const wordRef = React.useRef<HTMLSpanElement | null>(null);
  const bodyRef = React.useRef<HTMLSpanElement | null>(null);
  const flowRows = useListenFocusFlowRows(
    wordRef,
    bodyRef,
    props.displayText,
  );
  const rowProgresses = resolveListenLyricsFocusFlowRowProgresses(
    props.progress,
    flowRows.map((row) => row.width),
  );
  const style = resolveListenLyricsFocusWordStyle(
    props.layout,
    props.state,
    props.progress,
  );
  return (
    <span
      ref={wordRef}
      className="listen-lyrics-focus__word"
      data-word-state={props.state}
      data-direction={props.layout?.direction ?? 1}
      style={style}
    >
      <span ref={bodyRef} className="listen-lyrics-focus__word-body">
        {props.displayText}
      </span>
      {flowRows.map((row, index) => {
        const rowProgress = rowProgresses[index] ?? 0;
        const sweepProgress = row.left + row.width * rowProgress;
        const fillStyle = {
          "--listen-lyrics-focus-word-progress": `${
            Math.round(sweepProgress * 1000) / 10
          }%`,
          clipPath: `inset(${roundListenFocusMotionValue(row.top * 100)}% 0 ${
            roundListenFocusMotionValue(row.bottom * 100)
          }% 0)`,
        } as ListenFocusWordCSSProperties;
        return (
          <span
            key={`${index}:${row.top}:${row.left}:${row.width}`}
            className="listen-lyrics-focus__word-fill"
            data-focus-flow-row="true"
            data-focus-flow-row-count={flowRows.length}
            data-focus-flow-row-index={index}
            data-focus-flow-row-state={
              rowProgress >= 1
                ? "passed"
                : rowProgress > 0
                  ? "active"
                  : "pending"
            }
            aria-hidden="true"
            style={fillStyle}
          >
            {props.displayText}
          </span>
        );
      })}
    </span>
  );
});

type ListenFocusFlowRow = {
  top: number;
  bottom: number;
  left: number;
  width: number;
};

const LISTEN_FOCUS_SINGLE_FLOW_ROW: readonly ListenFocusFlowRow[] = [
  { top: 0, bottom: 0, left: 0, width: 1 },
];

/**
 * Maps one timing unit onto its visual rows in reading order. Row widths are
 * used as the distance budget so a short final row does not consume the same
 * playback time as a full row.
 */
export function resolveListenLyricsFocusFlowRowProgresses(
  progress: number,
  rowWidths: readonly number[],
) {
  if (rowWidths.length === 0) {
    return [];
  }
  const widths = rowWidths.map((width) =>
    Number.isFinite(width) && width > 0 ? width : 0,
  );
  const totalWidth = widths.reduce((total, width) => total + width, 0);
  if (totalWidth <= 0) {
    return rowWidths.map((_, index) =>
      Math.min(1, Math.max(0, progress * rowWidths.length - index)),
    );
  }
  const travelled = Math.min(1, Math.max(0, progress)) * totalWidth;
  let rowStart = 0;
  return widths.map((width) => {
    const rowProgress = width <= 0
      ? travelled >= rowStart ? 1 : 0
      : Math.min(1, Math.max(0, (travelled - rowStart) / width));
    rowStart += width;
    return rowProgress;
  });
}

/**
 * Partitions wrapped text into mutually exclusive vertical clip bands. Font
 * client rects can overlap when the line height is tight, so adjacent rows
 * share one midpoint boundary instead of independently using their raw edges.
 */
export function resolveListenLyricsFocusFlowRowClipInsets(
  rows: readonly { start: number; end: number }[],
) {
  if (rows.length === 0) {
    return [];
  }
  const normalizedRows = rows.map((row) => {
    const start = Math.min(
      1,
      Math.max(0, Number.isFinite(row.start) ? row.start : 0),
    );
    const end = Math.min(
      1,
      Math.max(start, Number.isFinite(row.end) ? row.end : start),
    );
    return { start, end };
  });
  const boundaries: number[] = [];
  for (let index = 0; index < normalizedRows.length - 1; index += 1) {
    const current = normalizedRows[index];
    const next = normalizedRows[index + 1];
    const midpoint = ((current?.end ?? 0) + (next?.start ?? 1)) / 2;
    boundaries.push(
      Math.min(1, Math.max(boundaries[index - 1] ?? 0, midpoint)),
    );
  }
  return normalizedRows.map((_, index) => ({
    top: index === 0 ? 0 : boundaries[index - 1] ?? 0,
    bottom: index === normalizedRows.length - 1
      ? 0
      : 1 - (boundaries[index] ?? 1),
  }));
}

function useListenFocusFlowRows(
  wordRef: React.RefObject<HTMLSpanElement>,
  bodyRef: React.RefObject<HTMLSpanElement>,
  text: string,
) {
  const [rows, setRows] = React.useState<readonly ListenFocusFlowRow[]>(
    LISTEN_FOCUS_SINGLE_FLOW_ROW,
  );

  useListenLyricsLayoutEffect(() => {
    const word = wordRef.current;
    const body = bodyRef.current;
    if (!word || !body) {
      return;
    }
    let disposed = false;
    let measureFrame: number | null = null;
    const sync = () => {
      if (disposed) {
        return;
      }
      const nextRows = measureListenFocusFlowRows(word, body);
      setRows((current) =>
        equalListenFocusFlowRows(current, nextRows) ? current : nextRows,
      );
    };
    const scheduleSync = () => {
      if (measureFrame !== null || typeof window === "undefined") {
        return;
      }
      measureFrame = window.requestAnimationFrame(() => {
        measureFrame = null;
        sync();
      });
    };

    sync();
    scheduleSync();
    const observer = typeof ResizeObserver === "undefined"
      ? null
      : new ResizeObserver(scheduleSync);
    observer?.observe(word);
    if (word.parentElement) {
      observer?.observe(word.parentElement);
    }
    window.addEventListener("resize", scheduleSync);

    const fonts = document.fonts;
    fonts?.addEventListener?.("loadingdone", scheduleSync);
    void fonts?.ready?.then(scheduleSync);
    return () => {
      disposed = true;
      observer?.disconnect();
      window.removeEventListener("resize", scheduleSync);
      fonts?.removeEventListener?.("loadingdone", scheduleSync);
      if (measureFrame !== null) {
        window.cancelAnimationFrame(measureFrame);
      }
    };
  }, [bodyRef, text, wordRef]);

  return rows;
}

function measureListenFocusFlowRows(
  word: HTMLSpanElement,
  body: HTMLSpanElement,
): readonly ListenFocusFlowRow[] {
  if (
    typeof document === "undefined" ||
    typeof document.createRange !== "function"
  ) {
    return LISTEN_FOCUS_SINGLE_FLOW_ROW;
  }
  const bounds = word.getBoundingClientRect();
  if (bounds.width <= 0 || bounds.height <= 0) {
    return LISTEN_FOCUS_SINGLE_FLOW_ROW;
  }
  const range = document.createRange();
  range.selectNodeContents(body);
  if (typeof range.getClientRects !== "function") {
    return LISTEN_FOCUS_SINGLE_FLOW_ROW;
  }
  const fragments = Array.from(range.getClientRects())
    .filter((rect) => rect.width > 0.5 && rect.height > 0.5)
    .sort((left, right) => left.top - right.top || left.left - right.left);
  if (fragments.length === 0) {
    return LISTEN_FOCUS_SINGLE_FLOW_ROW;
  }

  const merged: Array<{
    top: number;
    right: number;
    bottom: number;
    left: number;
  }> = [];
  for (const fragment of fragments) {
    const row = merged[merged.length - 1];
    const sameRow = row &&
      Math.abs(row.top - fragment.top) <=
        Math.max(1, fragment.height * 0.12);
    if (!sameRow) {
      merged.push({
        top: fragment.top,
        right: fragment.right,
        bottom: fragment.bottom,
        left: fragment.left,
      });
      continue;
    }
    row.top = Math.min(row.top, fragment.top);
    row.right = Math.max(row.right, fragment.right);
    row.bottom = Math.max(row.bottom, fragment.bottom);
    row.left = Math.min(row.left, fragment.left);
  }

  const verticalInsets = resolveListenLyricsFocusFlowRowClipInsets(
    merged.map((row) => ({
      start: Math.min(
        1,
        Math.max(0, (row.top - bounds.top) / bounds.height),
      ),
      end: Math.min(
        1,
        Math.max(0, (row.bottom - bounds.top) / bounds.height),
      ),
    })),
  );
  return merged.map((row, index) => {
    const left = Math.min(
      1,
      Math.max(0, (row.left - bounds.left) / bounds.width),
    );
    const right = Math.min(
      1,
      Math.max(left, (row.right - bounds.left) / bounds.width),
    );
    return {
      top: verticalInsets[index]?.top ?? 0,
      bottom: verticalInsets[index]?.bottom ?? 0,
      left,
      width: Math.max(0.001, right - left),
    };
  });
}

function equalListenFocusFlowRows(
  left: readonly ListenFocusFlowRow[],
  right: readonly ListenFocusFlowRow[],
) {
  return left.length === right.length && left.every((row, index) => {
    const candidate = right[index];
    return candidate &&
      Math.abs(row.top - candidate.top) < 0.001 &&
      Math.abs(row.bottom - candidate.bottom) < 0.001 &&
      Math.abs(row.left - candidate.left) < 0.001 &&
      Math.abs(row.width - candidate.width) < 0.001;
  });
}

type ListenFocusWordCSSProperties = React.CSSProperties & {
  [name: `--listen-lyrics-focus-${string}`]: string | number;
};

function resolveListenLyricsFocusWordStyle(
  layout: ListenLyricsFocusWordLayout | undefined,
  state: ListenLyricsFocusWordState,
  progress: number,
): ListenFocusWordCSSProperties {
  const resolvedLayout = layout ?? {
    index: 0,
    text: "",
    direction: 1,
    phase: 0,
    liftEm: 0.1,
    depth: 0.9,
  };
  const motion = resolveListenLyricsFocusWordMotion(
    resolvedLayout,
    state,
    progress,
  );
  return {
    "--listen-lyrics-focus-word-progress": `${
      Math.round(Math.min(1, Math.max(0, progress)) * 1000) / 10
    }%`,
    "--listen-lyrics-focus-prism-offset": `${motion.prismOffsetEm}em`,
    "--listen-lyrics-focus-splice-shift": `${motion.spliceShiftEm}em`,
    "--listen-lyrics-focus-facet-angle": `${motion.facetAngleDeg}deg`,
    "--listen-lyrics-focus-facet-depth": `${motion.facetDepthEm}em`,
    "--listen-lyrics-focus-facet-scale": motion.facetScale,
    "--listen-lyrics-focus-pendulum-angle": `${motion.pendulumAngleDeg}deg`,
    "--listen-lyrics-focus-pendulum-y": `${motion.pendulumYEm}em`,
    "--listen-lyrics-focus-unit-scale": motion.scale,
  };
}

function resolveListenLyricsScript(text: string) {
  if (/[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}]/u.test(text)) {
    return "cjk";
  }
  if (/\p{Script=Hangul}/u.test(text)) {
    return "hangul";
  }
  return "latin";
}

function prefersReducedFocusMotion() {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

function ListenKaraokeLine(props: {
  active: boolean;
  currentTimeMs: number;
  lineStartMs: number;
  lineEndMs: number;
  words: ListenLyricWordView[];
}) {
  const timingUnits = expandListenLyricTimingUnits(props.words);
  if (!props.active) {
    return (
      <span className="listen-lyrics-karaoke">
        {timingUnits
          .map((_, index) =>
            getListenLyricTimingUnitDisplayText(timingUnits, index),
          )
          .join("")}
      </span>
    );
  }
  const activeWord = getListenActiveLyricWordProgress(
    timingUnits,
    props.currentTimeMs,
    props.lineStartMs,
    props.lineEndMs,
  );
  return (
    <span className="listen-lyrics-karaoke">
      {timingUnits.map((word, index) => {
        const wordActive = index === activeWord.index;
        const wordPast = activeWord.index >= 0 && index < activeWord.index;
        const fillPercent = wordActive
          ? Math.round(activeWord.progress * 1000) / 10
          : wordPast
            ? 100
            : 0;
        const wordState = wordPast
          ? "passed"
          : wordActive
            ? "active"
            : "pending";
        const displayText = getListenLyricTimingUnitDisplayText(
          timingUnits,
          index,
        );
        const wordStyle = {
          "--listen-lyrics-word-progress": `${fillPercent}%`,
        } as React.CSSProperties;
        return (
          <span
            key={`${word.startMs}-${index}-${word.text}`}
            className="listen-lyrics-karaoke__word"
            data-word-state={wordState}
            style={wordStyle}
          >
            <span className="listen-lyrics-karaoke__word-base">
              {displayText}
            </span>
            <span
              className="listen-lyrics-karaoke__word-fill"
              aria-hidden="true"
            >
              {displayText}
            </span>
          </span>
        );
      })}
    </span>
  );
}

function seekToLine(props: ListenLyricsRendererProps, index: number) {
  const line = props.lines[index];
  if (line) {
    props.onSeek?.(line.startMs / 1000);
  }
}

function handleLineActivation(
  event: React.KeyboardEvent<HTMLDivElement>,
  props: ListenLyricsRendererProps,
  index: number,
) {
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    seekToLine(props, index);
  }
}

function easeOutListenLyricsScroll(progress: number) {
  const clamped = Math.max(0, Math.min(1, progress));
  return 1 - Math.pow(1 - clamped, 3);
}

function resolveListenLyricsScrollAnchor(
  _container: HTMLElement,
  _variant: ListenLyricsSurfaceVariant,
) {
  return 0.5;
}
