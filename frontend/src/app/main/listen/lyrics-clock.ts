import * as React from "react";

type ListenLyricsVisualClockOptions = {
  sourceTimeMs: number;
  timelineKey: string;
  running: boolean;
  playbackRate?: number;
};

export type ListenLyricsVisualClockAnchor = {
  sourceMs: number;
  anchorMs: number;
  running: boolean;
  playbackRate: number;
  key: string;
};

/**
 * Native playback samples arrive at a lower cadence than the visual frame.
 * Only absorb tiny delivery jitter; user offsets, seeks and stale snapshots
 * must re-anchor immediately instead of becoming visible phase lag.
 */
const LISTEN_LYRICS_CLOCK_JITTER_WINDOW_MS = 80;
const LISTEN_LYRICS_CLOCK_JITTER_CORRECTION = 0.5;

export function normalizeListenLyricsClockTimeMs(value: number) {
  return Number.isFinite(value) ? Math.max(0, value) : 0;
}

export function normalizeListenLyricsPlaybackRate(value: number | undefined) {
  return Number.isFinite(value) && Number(value) > 0
    ? Math.min(16, Number(value))
    : 1;
}

export function resolveListenLyricsVisualClockAnchor(options: {
  clock: ListenLyricsVisualClockAnchor;
  sourceTimeMs: number;
  timelineKey: string;
  running: boolean;
  playbackRate?: number;
  nowMs: number;
}): ListenLyricsVisualClockAnchor {
  const sourceTimeMs = normalizeListenLyricsClockTimeMs(options.sourceTimeMs);
  const nowMs = normalizeListenLyricsClockTimeMs(options.nowMs);
  const currentSourceMs = normalizeListenLyricsClockTimeMs(
    options.clock.sourceMs,
  );
  const currentAnchorMs = normalizeListenLyricsClockTimeMs(
    options.clock.anchorMs,
  );
  const currentPlaybackRate = normalizeListenLyricsPlaybackRate(
    options.clock.playbackRate,
  );
  const playbackRate = normalizeListenLyricsPlaybackRate(
    options.playbackRate,
  );
  const sameTimeline = options.clock.key === options.timelineKey;
  const samePlaybackRate = currentPlaybackRate === playbackRate;
  const predicted =
    options.clock.running && sameTimeline
      ? normalizeListenLyricsClockTimeMs(
          currentSourceMs +
            Math.max(0, nowMs - currentAnchorMs) * currentPlaybackRate,
        )
      : currentSourceMs;
  const drift = sourceTimeMs - predicted;
  const nextSourceMs =
    options.running &&
    sameTimeline &&
    samePlaybackRate &&
    Math.abs(drift) <= LISTEN_LYRICS_CLOCK_JITTER_WINDOW_MS
      ? normalizeListenLyricsClockTimeMs(
          predicted + drift * LISTEN_LYRICS_CLOCK_JITTER_CORRECTION,
        )
      : sourceTimeMs;

  return {
    sourceMs: nextSourceMs,
    anchorMs: nowMs,
    running: options.running,
    playbackRate,
    key: options.timelineKey,
  };
}

export function resolveListenLyricsVisualClockFrame(options: {
  clock: ListenLyricsVisualClockAnchor;
  timelineKey: string;
  nowMs: number;
}) {
  const sourceMs = normalizeListenLyricsClockTimeMs(options.clock.sourceMs);
  if (!options.clock.running || options.clock.key !== options.timelineKey) {
    return sourceMs;
  }
  const nowMs = normalizeListenLyricsClockTimeMs(options.nowMs);
  const anchorMs = normalizeListenLyricsClockTimeMs(options.clock.anchorMs);
  const playbackRate = normalizeListenLyricsPlaybackRate(
    options.clock.playbackRate,
  );
  return normalizeListenLyricsClockTimeMs(
    sourceMs + Math.max(0, nowMs - anchorMs) * playbackRate,
  );
}

export function resolveListenLyricsVisualClockRenderTime(options: {
  sourceTimeMs: number;
  visualTimeMs: number;
  clockKey: string;
  timelineKey: string;
}) {
  return options.clockKey === options.timelineKey
    ? normalizeListenLyricsClockTimeMs(options.visualTimeMs)
    : normalizeListenLyricsClockTimeMs(options.sourceTimeMs);
}

/**
 * Smooths the comparatively sparse native-player updates for renderers while
 * still snapping on seeks and track changes. The source clock remains the
 * authority; renderers only consume the interpolated value.
 */
export function useListenLyricsVisualClock(
  options: ListenLyricsVisualClockOptions,
) {
  const sourceTimeMs = normalizeListenLyricsClockTimeMs(options.sourceTimeMs);
  const clockRef = React.useRef<ListenLyricsVisualClockAnchor>({
    sourceMs: sourceTimeMs,
    anchorMs: 0,
    running: false,
    playbackRate: normalizeListenLyricsPlaybackRate(options.playbackRate),
    key: "",
  });
  const [visualTimeMs, setVisualTimeMs] = React.useState(sourceTimeMs);

  React.useEffect(() => {
    const nextClock = resolveListenLyricsVisualClockAnchor({
      clock: clockRef.current,
      sourceTimeMs,
      timelineKey: options.timelineKey,
      running: options.running,
      playbackRate: options.playbackRate,
      nowMs: performance.now(),
    });
    clockRef.current = nextClock;
    setVisualTimeMs(nextClock.sourceMs);
  }, [
    options.playbackRate,
    options.running,
    options.timelineKey,
    sourceTimeMs,
  ]);

  React.useEffect(() => {
    if (!options.running || !options.timelineKey) {
      return;
    }
    let frame = 0;
    let lastPaintAt = 0;
    const tick = (now: number) => {
      const clock = clockRef.current;
      if (
        clock.running &&
        clock.key === options.timelineKey &&
        now - lastPaintAt >= 32
      ) {
        lastPaintAt = now;
        setVisualTimeMs(
          resolveListenLyricsVisualClockFrame({
            clock,
            timelineKey: options.timelineKey,
            nowMs: now,
          }),
        );
      }
      frame = window.requestAnimationFrame(tick);
    };
    frame = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frame);
  }, [options.running, options.timelineKey]);

  return resolveListenLyricsVisualClockRenderTime({
    sourceTimeMs,
    visualTimeMs,
    clockKey: clockRef.current.key,
    timelineKey: options.timelineKey,
  });
}
