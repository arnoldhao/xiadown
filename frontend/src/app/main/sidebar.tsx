import {
  Loader2,
  Pause,
  Play,
  SkipBack,
  SkipForward,
  X,
} from "lucide-react";
import * as React from "react";

import type { ListenNowPlayingStatus } from "@/app/main/Listen";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import { ListenCoverArtwork } from "@/shared/assets/listen-cover-artwork";
import { resolveXiaMainSidebarSurface } from "@/shared/styles/xiadown";

type ListenNowPlayingControlCommand = "previous" | "toggle" | "next";
type ListenMiniPanelVariant = "hush" | "timeline";
export type ListenNowPlayingPanelSurface = "white" | "dark" | "tray";

export const resolveSidebarSurface = resolveXiaMainSidebarSurface;

export function listenStatusLabel(
  status: ListenNowPlayingStatus | null,
  text: ReturnType<typeof getXiaText>,
) {
  switch (status?.state) {
    case "idle":
      return text.listen.idleStatus;
    case "playing":
      return text.listen.playingStatus;
    case "paused":
      return text.listen.pausedStatus;
    case "loading":
      return text.listen.loadingStatus;
    case "error":
      return text.listen.errorStatus;
    default:
      return text.views.listen;
  }
}

function resolveListenProgress(status: ListenNowPlayingStatus) {
  const duration = Number.isFinite(status.progress.duration)
    ? Math.max(0, status.progress.duration)
    : 0;
  if (duration <= 0) {
    return null;
  }

  const currentTime = Number.isFinite(status.progress.currentTime)
    ? Math.max(0, Math.min(status.progress.currentTime, duration))
    : 0;
  const bufferedTime = Number.isFinite(status.progress.bufferedTime)
    ? Math.max(0, Math.min(status.progress.bufferedTime, duration))
    : 0;

  return {
    currentTime,
    duration,
    progressPercent: (currentTime / duration) * 100,
    bufferedPercent: (bufferedTime / duration) * 100,
  };
}

function resolveListenMiniPanelVariant(
  status: ListenNowPlayingStatus | null,
): ListenMiniPanelVariant {
  return status?.live === true || status?.mode === "hush" ? "hush" : "timeline";
}

function renderListenMiniControlIcon(
  state: ListenNowPlayingStatus["state"],
  isPlaying: boolean,
) {
  if (state === "loading") {
    return <Loader2 className="app-motion-spin h-3.5 w-3.5" />;
  }
  if (state === "error") {
    return <X className="h-3.5 w-3.5" />;
  }
  if (isPlaying) {
    return <Pause className="listen-mini-control-icon h-3.5 w-3.5" data-filled="true" />;
  }
  return <Play className="listen-mini-control-icon ml-0.5 h-3.5 w-3.5" data-filled="true" />;
}

function ListenMiniScrollingText(props: {
  text: string;
  className?: string;
}) {
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const contentRef = React.useRef<HTMLSpanElement | null>(null);
  const [overflow, setOverflow] = React.useState(0);
  const normalizedText = props.text.trim();
  const scrolling = overflow > 1;
  const style = scrolling
    ? ({
        "--listen-marquee-shift": `-${Math.ceil(overflow + 18)}px`,
        "--listen-marquee-duration": `${Math.min(
          12,
          Math.max(6, (overflow + 150) / 28),
        )}s`,
      } as React.CSSProperties)
    : undefined;

  React.useLayoutEffect(() => {
    const container = containerRef.current;
    const contentElement = contentRef.current;
    if (!container || !contentElement) {
      return;
    }
    const syncOverflow = () => {
      setOverflow(
        Math.max(0, contentElement.scrollWidth - container.clientWidth),
      );
    };
    syncOverflow();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(syncOverflow);
    observer.observe(container);
    observer.observe(contentElement);
    return () => observer.disconnect();
  }, [normalizedText]);

  return (
    <div
      ref={containerRef}
      className={cn(
        "listen-mini-marquee relative block max-w-full min-w-0 overflow-hidden whitespace-nowrap",
        props.className,
      )}
      data-overflow={scrolling ? "true" : "false"}
      title={normalizedText}
    >
      <span
        ref={contentRef}
        className={cn(
          "inline-block max-w-none align-top",
          scrolling ? "listen-marquee-text pr-4" : "max-w-full truncate",
        )}
        style={style}
      >
        {normalizedText}
      </span>
    </div>
  );
}

function resolveMiniPanelText(
  status: ListenNowPlayingStatus | null,
  text: ReturnType<typeof getXiaText>,
) {
  if (!status || status.state === "idle") {
    return {
      title: text.views.listen,
      subtitle: text.listen.idleSubtitle,
    };
  }

  return {
    title: status.title.trim() || text.listen.nowPlaying,
    subtitle: status.subtitle.trim() || text.listen.nowPlaying,
  };
}

function ListenNowPlayingPanelArtwork(props: {
  status: ListenNowPlayingStatus | null;
}) {
  if (!props.status || props.status.state === "idle") {
    return (
      <ListenCoverArtwork
        alt=""
        candidates={[LISTEN_DEFAULT_COVER_IMAGE_URL]}
        className="h-full w-full"
        loading="lazy"
      />
    );
  }

  return <ListenSidebarArtwork status={props.status} />;
}

function ListenNowPlayingPanelTransport(props: {
  status: ListenNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
  onControlCommand?: (command: ListenNowPlayingControlCommand) => void;
}) {
  const state = props.status?.state ?? "idle";
  const variant = resolveListenMiniPanelVariant(props.status);
  const canControl = Boolean(
    props.onControlCommand &&
      props.status?.canControl &&
      state !== "idle" &&
      state !== "loading",
  );
  const canPrevious = canControl && props.status?.canPrevious !== false;
  const canNext = canControl && props.status?.canNext !== false;
  const isPlaying = state === "playing";
  const playLabel = isPlaying ? props.text.listen.pause : props.text.listen.play;
  const primaryLabel =
    state === "loading"
      ? props.text.listen.loading
      : state === "error"
        ? props.text.listen.errorStatus
        : playLabel;

  if (variant === "hush") {
    return (
      <div className="listen-mini-transport" data-variant="hush">
        <button
          type="button"
          className="listen-mini-primary-control"
          aria-label={primaryLabel}
          disabled={!canControl}
          onClick={() => props.onControlCommand?.("toggle")}
        >
          {renderListenMiniControlIcon(state, isPlaying)}
        </button>
      </div>
    );
  }

  return (
    <div className="listen-mini-transport" data-variant="timeline">
      <button
        type="button"
        className="listen-mini-side-control"
        aria-label={props.text.listen.previous}
        disabled={!canPrevious}
        onClick={() => props.onControlCommand?.("previous")}
      >
        <SkipBack className="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        className="listen-mini-primary-control"
        aria-label={primaryLabel}
        disabled={!canControl}
        onClick={() => props.onControlCommand?.("toggle")}
      >
        {renderListenMiniControlIcon(state, isPlaying)}
      </button>
      <button
        type="button"
        className="listen-mini-side-control"
        aria-label={props.text.listen.next}
        disabled={!canNext}
        onClick={() => props.onControlCommand?.("next")}
      >
        <SkipForward className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

function ListenNowPlayingPanelProgress(props: {
  status: ListenNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
}) {
  const variant = resolveListenMiniPanelVariant(props.status);
  if (variant === "hush") {
    const state = props.status?.state ?? "idle";
    return (
      <div className="listen-mini-progress-row" data-variant="hush">
        <div
          className="listen-mini-live-progress"
          data-state={state}
          role="status"
          aria-label={`LIVE · ${listenStatusLabel(props.status, props.text)}`}
        >
          <span aria-hidden="true" className="listen-mini-live-line" />
          <span
            aria-hidden="true"
            className="listen-mini-live-label"
          >
            LIVE
          </span>
          <span aria-hidden="true" className="listen-mini-live-dot" />
        </div>
      </div>
    );
  }

  const progress =
    props.status && props.status.state !== "idle"
      ? resolveListenProgress(props.status)
      : null;

  return (
    <div className="listen-mini-progress-row" data-variant="timeline">
      <div
        className="listen-mini-progress-track"
        data-state={
          progress
            ? "ready"
            : props.status?.state === "loading"
              ? "loading"
              : "empty"
        }
        role={progress ? "progressbar" : undefined}
        aria-valuemin={progress ? 0 : undefined}
        aria-valuemax={progress ? Math.round(progress.duration) : undefined}
        aria-valuenow={progress ? Math.round(progress.currentTime) : undefined}
      >
        {progress ? (
          <>
            <span
              aria-hidden="true"
              className="listen-mini-progress-buffer"
              style={{ width: `${progress.bufferedPercent}%` }}
            />
            <span
              aria-hidden="true"
              className="listen-mini-progress-value"
              style={{ width: `${progress.progressPercent}%` }}
            />
          </>
        ) : props.status?.state === "loading" ? (
          <span aria-hidden="true" className="listen-mini-progress-loading" />
        ) : null}
      </div>
    </div>
  );
}

export function ListenNowPlayingHoverPanel(props: {
  status: ListenNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
  className?: string;
  /** The surrounding disclosure owns the one and only glass material. */
  embedded?: boolean;
  surface?: ListenNowPlayingPanelSurface;
  onControlCommand?: (command: ListenNowPlayingControlCommand) => void;
}) {
  const text = resolveMiniPanelText(props.status, props.text);
  const surface = props.surface ?? "white";

  return (
    <div
      className={cn(
        "listen-now-playing-panel",
        props.className,
      )}
      data-embedded={props.embedded ? "true" : undefined}
      data-subtitle-tone={props.status?.subtitleTone}
      data-surface={surface}
      aria-label={`${props.text.listen.nowPlaying}: ${text.title}`}
    >
      <div className="listen-panel-layout relative grid h-full min-w-0 grid-cols-2 overflow-hidden">
        <div className="relative min-w-0 overflow-visible">
          <div className="listen-panel-artwork-glow">
            <ListenNowPlayingPanelArtwork status={props.status} />
          </div>
          <div className="listen-panel-artwork-main">
            <ListenNowPlayingPanelArtwork status={props.status} />
          </div>
        </div>
        <div aria-hidden="true" className="listen-panel-color-wash" />
        <div aria-hidden="true" className="listen-panel-blur-veil" />
        <div aria-hidden="true" className="listen-panel-bottom-vignette" />
        <div aria-hidden="true" className="listen-panel-grain" />
        <div aria-hidden="true" className="listen-panel-ring" />
        <div className="relative z-20 col-start-2 flex h-full min-w-0 flex-col px-2.5 py-2.5">
          <div className="listen-panel-body flex min-h-0 flex-1 flex-col items-center justify-center">
            <ListenMiniScrollingText
              text={text.title}
              className="listen-panel-title"
            />
            <ListenMiniScrollingText
              text={text.subtitle}
              className="listen-panel-subtitle"
            />
          </div>
          <ListenNowPlayingPanelTransport
            status={props.status}
            text={props.text}
            onControlCommand={props.onControlCommand}
          />
          <ListenNowPlayingPanelProgress
            status={props.status}
            text={props.text}
          />
        </div>
      </div>
    </div>
  );
}

export function resolveListenArtworkCandidates(
  status: ListenNowPlayingStatus,
) {
  const seen = new Set<string>();
  const sources = [status.artworkURL, ...(status.artworkCandidates ?? [])];
  const candidates: string[] = [];
  for (const value of sources) {
    const candidate = String(value || "").trim();
    if (
      !candidate ||
      candidate === LISTEN_DEFAULT_COVER_IMAGE_URL ||
      seen.has(candidate)
    ) {
      continue;
    }
    seen.add(candidate);
    candidates.push(candidate);
  }
  candidates.push(LISTEN_DEFAULT_COVER_IMAGE_URL);
  return candidates;
}

export function ListenSidebarArtwork(props: {
  status: ListenNowPlayingStatus;
  className?: string;
}) {
  const candidates = resolveListenArtworkCandidates(props.status);
  return (
    <ListenCoverArtwork
      alt=""
      candidates={candidates}
      className={cn("h-full w-full", props.className)}
      loading="lazy"
    />
  );
}
