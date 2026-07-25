import {
  Loader2,
  PanelLeftClose,
  PanelLeftOpen,
  Pause,
  Play,
  Ratio,
  Square,
} from "lucide-react";
import * as React from "react";

import { ListenFullscreenVolumeControl } from "@/app/main/listen/fullscreen-volume-control";
import {
  readListenNativeVideoRadius,
  useListenNativeVideoUnderlay,
} from "@/app/main/listen/native-video-underlay";
import {
  LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO,
  normalizeListenInlineVideoAspectRatio,
  resolveListenPlaybackStatusLabel,
} from "@/app/main/listen/playback-helpers";
import {
  ListenPlayerIconButton,
  type ListenAirPlayAnchor,
} from "@/app/main/listen/playback-ui";
import { ListenScrollingText } from "@/app/main/listen/playback-controls";
import { buildListenPosterCandidates } from "@/app/main/listen/storage";
import type {
  ListenOnlineItem,
  ListenRemotePlaybackState,
} from "@/app/main/listen/types";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import { ListenCoverArtwork } from "@/shared/assets/listen-cover-artwork";
import type { Pet } from "@/shared/contracts/pets";
import { PetDisplay } from "@/shared/ui/pet-player";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

export type ListenNativeVideoRect = ListenAirPlayAnchor & {
  centerX?: number;
  centerY?: number;
  stageWidth?: number;
  stageHeight?: number;
  viewportWidth?: number;
  viewportHeight?: number;
  radius?: number;
  interactive?: boolean;
  sequence?: number;
  presentation?: "embedded-video" | "app-fullscreen";
};

export const LISTEN_LIVE_VIDEO_ASPECT_RATIO = 16 / 9;
export const LISTEN_LIVE_VIDEO_TOPBAR_HEIGHT = 74;
export const LISTEN_LIVE_VIDEO_FRAME_GAP = 10;
export const LISTEN_LIVE_VIDEO_MIN_WINDOW_WIDTH = 960;
export const LISTEN_LIVE_VIDEO_MIN_WINDOW_HEIGHT = 640;
const LISTEN_LIVE_VIDEO_GEOMETRY_SETTLE_DELAYS_MS = [
  32,
  80,
  140,
  220,
  340,
  480,
  680,
  920,
] as const;
export const LISTEN_LIVE_VIDEO_EMBED_SETTLE_MS = 360;
const LISTEN_LIVE_VIDEO_REVEAL_MS = 780;

export function createListenNativeVideoSequence(requestId: number) {
  return Date.now() * 1000 + requestId;
}

export function ListenInlineVideoSurface(props: {
  variant: "compact" | "wide" | "fullscreen";
  active: boolean;
  visible: boolean;
	geometrySuspended: boolean;
  aspectRatio?: number;
  pet: Pet | null;
  petImageURL: string;
  title: string;
  text: ReturnType<typeof getXiaText>;
  onRectChange?: (
    rect: ListenNativeVideoRect,
  ) => boolean | void | Promise<boolean | void>;
}) {
  const frameRef = React.useRef<HTMLDivElement | null>(null);
  const stageRef = React.useRef<HTMLDivElement | null>(null);
  const aspectRatio = normalizeListenInlineVideoAspectRatio(
    props.aspectRatio ?? LISTEN_INLINE_VIDEO_FALLBACK_ASPECT_RATIO,
  );
  const [frameSize, setFrameSize] = React.useState({ width: 0, height: 0 });
  const [visualVisible, setVisualVisible] = React.useState(false);
  const rectRevealRequestRef = React.useRef(0);
  const {
    resetHole: resetNativeVideoHole,
    setHole: setNativeVideoHole,
  } = useListenNativeVideoUnderlay(
	props.visible && !props.geometrySuspended,
  );
  const frameReady = frameSize.width > 1 && frameSize.height > 1;
  const geometrySignature = [
    props.variant,
    Math.round(aspectRatio * 1000) / 1000,
    Math.round(frameSize.width * 2) / 2,
    Math.round(frameSize.height * 2) / 2,
  ].join(":");

  React.useEffect(() => {
    if (!props.visible) {
      setVisualVisible(false);
    }
  }, [props.visible]);

	React.useLayoutEffect(() => {
		if (!props.geometrySuspended) {
			return;
		}
		rectRevealRequestRef.current += 1;
		setVisualVisible(false);
		resetNativeVideoHole();
	}, [props.geometrySuspended, resetNativeVideoHole]);

  React.useLayoutEffect(() => {
    rectRevealRequestRef.current += 1;
    setVisualVisible(false);
  }, [geometrySignature]);

  React.useLayoutEffect(() => {
    const frame = frameRef.current;
    if (!frame || typeof ResizeObserver === "undefined") {
      return;
    }
    const sync = () => {
      const rect = frame.getBoundingClientRect();
      const width = Math.max(0, rect.width);
      const height = Math.max(0, rect.height);
      setFrameSize((current) => {
        if (
          Math.abs(current.width - width) < 0.5 &&
          Math.abs(current.height - height) < 0.5
        ) {
          return current;
        }
        return { width, height };
      });
    };
    sync();
    const observer = new ResizeObserver(sync);
    observer.observe(frame);
    return () => observer.disconnect();
  }, []);

  React.useLayoutEffect(() => {
    const onRectChange = props.onRectChange;
    if (
	  !props.active ||
	  props.geometrySuspended ||
	  !onRectChange ||
	  !frameReady
	) {
      return;
    }
    const element = stageRef.current;
    if (!element) {
      return;
    }
    let readFrame = 0;
    let commitFrame = 0;
    let lastRectSignature = "";
    let revealRetryCount = 0;
    const timers: number[] = [];
    const readRadius = () => readListenNativeVideoRadius(element);
    const pushRect = (force = false) => {
      const rect = element.getBoundingClientRect();
      if (rect.width < 1 || rect.height < 1) {
        return;
      }
      const radius = readRadius();
      const frameRect = frameRef.current?.getBoundingClientRect() ?? rect;
      const viewportWidth = Math.max(1, window.innerWidth);
      const viewportHeight = Math.max(1, window.innerHeight);
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const signature = [
        Math.round(rect.left * 2) / 2,
        Math.round(rect.top * 2) / 2,
        Math.round(rect.width * 2) / 2,
        Math.round(rect.height * 2) / 2,
        Math.round(frameRect.width * 2) / 2,
        Math.round(frameRect.height * 2) / 2,
        Math.round(viewportWidth * 2) / 2,
        Math.round(viewportHeight * 2) / 2,
        Math.round(radius * 2) / 2,
      ].join(":");
      const geometryChanged = signature !== lastRectSignature;
      if (!force && !geometryChanged) {
        return;
      }
      lastRectSignature = signature;
      if (geometryChanged) {
        revealRetryCount = 0;
      }
      rectRevealRequestRef.current += 1;
      const requestToken = rectRevealRequestRef.current;
      setVisualVisible(false);
      resetNativeVideoHole();
      const scheduleRevealRetry = () => {
        if (rectRevealRequestRef.current !== requestToken) {
          return;
        }
        if (revealRetryCount >= 18) {
          return;
        }
        revealRetryCount += 1;
        timers.push(window.setTimeout(() => syncRect(true), 220));
      };
      const nativeRect = {
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        centerX,
        centerY,
        stageWidth: Math.max(1, frameRect.width),
        stageHeight: Math.max(1, frameRect.height),
        viewportWidth,
        viewportHeight,
        radius,
      };
      let applyResult: boolean | void | Promise<boolean | void>;
      try {
        applyResult = onRectChange(nativeRect);
      } catch {
        return;
      }
      void Promise.resolve(applyResult).then((shown) => {
        if (rectRevealRequestRef.current !== requestToken || shown === false) {
          if (shown === false) {
            scheduleRevealRetry();
          }
          return;
        }
        revealRetryCount = 0;
        const nextRect = element.getBoundingClientRect();
        if (nextRect.width < 1 || nextRect.height < 1) {
          return;
        }
        setNativeVideoHole(nextRect, readRadius());
        setVisualVisible(true);
      }).catch(() => {
        scheduleRevealRetry();
      });
    };
    const cancelScheduledRect = () => {
      window.cancelAnimationFrame(readFrame);
      window.cancelAnimationFrame(commitFrame);
      readFrame = 0;
      commitFrame = 0;
    };
    const syncRect = (force = false) => {
      cancelScheduledRect();
      readFrame = window.requestAnimationFrame(() => {
        commitFrame = window.requestAnimationFrame(() => pushRect(force));
      });
    };
    const scheduleRect = () => syncRect();
    syncRect(true);
    LISTEN_LIVE_VIDEO_GEOMETRY_SETTLE_DELAYS_MS.forEach((delay) => {
      timers.push(window.setTimeout(scheduleRect, delay));
    });
    const observer =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(scheduleRect);
    observer?.observe(element);
    if (frameRef.current) {
      observer?.observe(frameRef.current);
    }
    window.addEventListener("resize", scheduleRect);
    window.addEventListener("scroll", scheduleRect, true);
    window.visualViewport?.addEventListener("resize", scheduleRect);
    window.visualViewport?.addEventListener("scroll", scheduleRect);
    return () => {
      rectRevealRequestRef.current += 1;
      cancelScheduledRect();
      timers.forEach((timer) => window.clearTimeout(timer));
      observer?.disconnect();
      window.removeEventListener("resize", scheduleRect);
      window.removeEventListener("scroll", scheduleRect, true);
      window.visualViewport?.removeEventListener("resize", scheduleRect);
      window.visualViewport?.removeEventListener("scroll", scheduleRect);
      resetNativeVideoHole();
    };
  }, [
    aspectRatio,
    frameReady,
    frameSize.height,
    frameSize.width,
    props.active,
	props.geometrySuspended,
    props.onRectChange,
    props.variant,
    resetNativeVideoHole,
    setNativeVideoHole,
  ]);

  const stageStyle = React.useMemo<React.CSSProperties | undefined>(() => {
    if (props.variant === "fullscreen") {
      return undefined;
    }
    if (frameSize.width <= 1 || frameSize.height <= 1) {
      return {
        aspectRatio,
      };
    }
    let width = frameSize.width;
    let height = width / aspectRatio;
    if (height > frameSize.height) {
      height = frameSize.height;
      width = height * aspectRatio;
    }
    return {
      width: `${Math.max(1, width)}px`,
      height: `${Math.max(1, height)}px`,
      aspectRatio,
    };
  }, [aspectRatio, frameSize.height, frameSize.width, props.variant]);

  return (
    <div
      ref={frameRef}
      className={cn(
        "listen-inline-video-frame",
        props.variant === "fullscreen"
          ? "listen-inline-video-frame-fullscreen"
          : props.variant === "wide"
            ? "listen-inline-video-frame-wide"
            : "listen-inline-video-frame-compact",
      )}
      data-native-video={visualVisible ? "underlay" : "pending"}
    >
      <div
        ref={stageRef}
        className="listen-inline-video-stage"
        style={stageStyle}
        data-native-video={visualVisible ? "underlay" : "pending"}
      >
        {!visualVisible ? (
          <div className="listen-inline-video-pending-layer">
            <PetDisplay
              pet={props.pet}
              imageUrl={props.petImageURL}
              animation="review"
              alt={props.title || props.text.listen.video}
              fallbackSrc={LISTEN_DEFAULT_COVER_IMAGE_URL}
              size={88}
              className="h-24 w-24"
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function ListenLiveVideoShell(props: {
  videoId: string;
  liveVideoModeActive: boolean;
  liveVideoVisible: boolean;
	geometrySuspended: boolean;
  track?: ListenOnlineItem;
  httpBaseURL: string;
  pet: Pet | null;
  petImageURL: string;
  title: string;
  subtitle: string;
  subtitleDanger?: boolean;
  listOpen: boolean;
  reserveWindowControls: boolean;
  playing: boolean;
  loading: boolean;
  playbackState?: ListenRemotePlaybackState;
  disabled?: boolean;
  muted: boolean;
  volume: number;
  text: ReturnType<typeof getXiaText>;
  onToggleList?: () => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
  onStopPlayback?: () => void;
  onFitLiveVideoWindow?: () => void;
  onLiveVideoRectChange?: (
    rect: ListenNativeVideoRect,
  ) => boolean | void | Promise<boolean | void>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const coverAreaRef = React.useRef<HTMLDivElement | null>(null);
  const previousLiveVideoVisibleRef = React.useRef(props.liveVideoVisible);
  const rectRevealRequestRef = React.useRef(0);
  const [visualLiveVideoVisible, setVisualLiveVideoVisible] =
    React.useState(props.liveVideoVisible);
  const visualLiveVideoVisibleRef = React.useRef(visualLiveVideoVisible);
  const [liveVideoRevealActive, setLiveVideoRevealActive] =
    React.useState(false);
  const {
    resetHole: resetNativeVideoHole,
    setHole: setNativeVideoHole,
  } = useListenNativeVideoUnderlay(
	props.liveVideoVisible && !props.geometrySuspended,
  );
  const playbackState =
    props.playbackState ??
    (props.loading ? "loading" : props.playing ? "playing" : "idle");
  const playLabel = props.playing ? props.text.listen.pause : props.text.listen.play;
  const playbackDisabled = props.disabled === true;
  const stopDisabled =
    playbackDisabled ||
    !props.onStopPlayback ||
    (!props.playing && !props.loading && playbackState === "idle");
  const statusLabel = resolveListenPlaybackStatusLabel(playbackState, props.text);
  const statusClass = resolveListenLivePlaybackStatusClass(playbackState);
  const listLabel = props.listOpen
    ? props.text.listen.collapseList
    : props.text.listen.openList;
  const titleLabel = props.title || props.text.listen.selectStation;
  const authorLabel = props.subtitle.trim();
  React.useEffect(() => {
    visualLiveVideoVisibleRef.current = visualLiveVideoVisible;
  }, [visualLiveVideoVisible]);

  React.useEffect(() => {
    const wasVisible = previousLiveVideoVisibleRef.current;
    previousLiveVideoVisibleRef.current = props.liveVideoVisible;
    if (!props.liveVideoVisible) {
      rectRevealRequestRef.current += 1;
      visualLiveVideoVisibleRef.current = false;
      setVisualLiveVideoVisible(false);
      setLiveVideoRevealActive(false);
      return;
    }
    if (!wasVisible) {
      setLiveVideoRevealActive(false);
    }
  }, [props.liveVideoVisible]);

	React.useLayoutEffect(() => {
		if (!props.geometrySuspended) {
			return;
		}
		rectRevealRequestRef.current += 1;
		visualLiveVideoVisibleRef.current = false;
		setVisualLiveVideoVisible(false);
		setLiveVideoRevealActive(false);
		resetNativeVideoHole();
	}, [props.geometrySuspended, resetNativeVideoHole]);

  React.useLayoutEffect(() => {
    if (
	  !props.liveVideoModeActive ||
	  !props.liveVideoVisible ||
	  props.geometrySuspended
	) {
      return;
    }
    const element = coverAreaRef.current;
    if (!element) {
      return;
    }
    let frame = 0;
    const revealCurrentRect = () => {
      const rect = element.getBoundingClientRect();
      if (rect.width < 1 || rect.height < 1) {
        return;
      }
      setNativeVideoHole(rect, readListenNativeVideoRadius(element));
      visualLiveVideoVisibleRef.current = true;
      setVisualLiveVideoVisible(true);
    };
    frame = window.requestAnimationFrame(revealCurrentRect);
    return () => window.cancelAnimationFrame(frame);
  }, [
	props.geometrySuspended,
	props.liveVideoModeActive,
	props.liveVideoVisible,
	props.videoId,
	setNativeVideoHole,
  ]);

  React.useLayoutEffect(() => {
    const onLiveVideoRectChange = props.onLiveVideoRectChange;
    if (
	  !props.liveVideoModeActive ||
	  props.geometrySuspended ||
	  !onLiveVideoRectChange
	) {
      return;
    }
    const element = coverAreaRef.current;
    if (!element) {
      return;
    }
    let readFrame = 0;
    let commitFrame = 0;
    let lastRectSignature = "";
    const timers: number[] = [];
    const readRadius = () => readListenNativeVideoRadius(element);
    const pushRect = (force = false) => {
      const rect = element.getBoundingClientRect();
      if (rect.width < 1 || rect.height < 1) {
        return;
      }
      const radius = readRadius();
      const viewportWidth = Math.max(1, window.innerWidth);
      const viewportHeight = Math.max(1, window.innerHeight);
      const shellElement = element.closest(".listen-live-video-shell");
      const shellRect =
        shellElement instanceof Element
          ? shellElement.getBoundingClientRect()
          : null;
      const stageWidth = shellRect
        ? Math.max(1, shellRect.width - LISTEN_LIVE_VIDEO_FRAME_GAP * 2)
        : Math.max(1, viewportWidth - LISTEN_LIVE_VIDEO_FRAME_GAP * 2);
      const stageHeight = shellRect
        ? Math.max(
            1,
            shellRect.height -
              LISTEN_LIVE_VIDEO_TOPBAR_HEIGHT -
              LISTEN_LIVE_VIDEO_FRAME_GAP * 2,
          )
        : Math.max(
            1,
            viewportHeight -
              LISTEN_LIVE_VIDEO_TOPBAR_HEIGHT -
              LISTEN_LIVE_VIDEO_FRAME_GAP * 2,
          );
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const signature = [
        Math.round(rect.left * 2) / 2,
        Math.round(rect.top * 2) / 2,
        Math.round(rect.width * 2) / 2,
        Math.round(rect.height * 2) / 2,
        Math.round(centerX * 2) / 2,
        Math.round(centerY * 2) / 2,
        Math.round(stageWidth * 2) / 2,
        Math.round(stageHeight * 2) / 2,
        Math.round(viewportWidth * 2) / 2,
        Math.round(viewportHeight * 2) / 2,
        Math.round(radius * 2) / 2,
      ].join(":");
      if (!force && signature === lastRectSignature) {
        return;
      }
      lastRectSignature = signature;
      rectRevealRequestRef.current += 1;
      const requestToken = rectRevealRequestRef.current;
      const wasVisuallyVisible = visualLiveVideoVisibleRef.current;
      visualLiveVideoVisibleRef.current = false;
      resetNativeVideoHole();
      setVisualLiveVideoVisible(false);
      setLiveVideoRevealActive(false);
      const applyResult = onLiveVideoRectChange({
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        centerX,
        centerY,
        stageWidth,
        stageHeight,
        viewportWidth,
        viewportHeight,
        radius,
      });
      void Promise.resolve(applyResult).then((shown) => {
        if (rectRevealRequestRef.current !== requestToken || shown === false) {
          return;
        }
        const nextRect = element.getBoundingClientRect();
        if (nextRect.width < 1 || nextRect.height < 1) {
          return;
        }
        setNativeVideoHole(nextRect, readRadius());
        if (!wasVisuallyVisible) {
          setLiveVideoRevealActive(true);
          timers.push(window.setTimeout(() => {
            if (rectRevealRequestRef.current !== requestToken) {
              return;
            }
            setLiveVideoRevealActive(false);
          }, LISTEN_LIVE_VIDEO_REVEAL_MS));
        }
        visualLiveVideoVisibleRef.current = true;
        setVisualLiveVideoVisible(true);
      }).catch(() => {});
    };
    const cancelScheduledRect = () => {
      window.cancelAnimationFrame(readFrame);
      window.cancelAnimationFrame(commitFrame);
      readFrame = 0;
      commitFrame = 0;
    };
    const syncRect = (force = false) => {
      cancelScheduledRect();
      readFrame = window.requestAnimationFrame(() => {
        commitFrame = window.requestAnimationFrame(() => pushRect(force));
      });
    };
    const scheduleRect = () => syncRect();
    syncRect(true);
    LISTEN_LIVE_VIDEO_GEOMETRY_SETTLE_DELAYS_MS.forEach((delay) => {
      timers.push(window.setTimeout(scheduleRect, delay));
    });
    const shell = element.closest(".listen-live-video-shell");
    const content = element.closest(".listen-content-surface");
    const animatedTargets = [shell, content].filter(
      (target): target is Element => target instanceof Element,
    );
    const observer =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(scheduleRect);
    observer?.observe(element);
    animatedTargets.forEach((target) => {
      target.addEventListener("animationend", scheduleRect);
      target.addEventListener("transitionend", scheduleRect);
    });
    window.addEventListener("resize", scheduleRect);
    window.addEventListener("scroll", scheduleRect, true);
    window.visualViewport?.addEventListener("resize", scheduleRect);
    window.visualViewport?.addEventListener("scroll", scheduleRect);
    return () => {
      rectRevealRequestRef.current += 1;
      cancelScheduledRect();
      timers.forEach((timer) => window.clearTimeout(timer));
      observer?.disconnect();
      animatedTargets.forEach((target) => {
        target.removeEventListener("animationend", scheduleRect);
        target.removeEventListener("transitionend", scheduleRect);
      });
      window.removeEventListener("resize", scheduleRect);
      window.removeEventListener("scroll", scheduleRect, true);
      window.visualViewport?.removeEventListener("resize", scheduleRect);
      window.visualViewport?.removeEventListener("scroll", scheduleRect);
      resetNativeVideoHole();
    };
  }, [
	props.geometrySuspended,
    props.liveVideoModeActive,
    props.onLiveVideoRectChange,
    props.videoId,
    resetNativeVideoHole,
    setNativeVideoHole,
  ]);
  return (
    <div
      className={cn(
        "listen-live-video-shell listen-video-shell",
        props.reserveWindowControls && "listen-video-shell-windows",
      )}
    >
      <div className="wails-no-drag absolute left-3 top-3 z-40 sm:left-5">
        <ListenPlayerIconButton
          label={listLabel}
          tooltipSide="bottom"
          className="listen-video-expand-button"
          onClick={props.onToggleList}
        >
          {props.listOpen ? (
            <PanelLeftClose className="h-4 w-4" />
          ) : (
            <PanelLeftOpen className="h-4 w-4" />
          )}
        </ListenPlayerIconButton>
      </div>
      <header
        className={cn(
          "listen-video-topbar wails-drag",
          props.reserveWindowControls && "listen-video-topbar-windows",
        )}
      >
        <div className="listen-video-info-area">
          <ListenFullscreenChannelCover
            httpBaseURL={props.httpBaseURL}
            track={props.track}
            title={titleLabel}
          />
          <div className="listen-video-info">
            <div className="listen-video-title-line">
              <h1>
                <ListenScrollingText
                  text={titleLabel}
                  as="span"
                />
              </h1>
              {authorLabel ? (
                <>
                  <span className="listen-video-title-separator" aria-hidden="true">
                    ·
                  </span>
                  <span
                    className={cn(
                      "listen-video-author",
                      props.subtitleDanger &&
                        "listen-playback-status-subtitle",
                    )}
                  >
                    <ListenScrollingText text={authorLabel} as="span" />
                  </span>
                </>
              ) : null}
            </div>
            <div className="listen-video-status-cluster">
              {visualLiveVideoVisible && props.onFitLiveVideoWindow ? (
                <div className="listen-video-fit-group wails-no-drag">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        className="listen-video-fit-group-button"
                        aria-label={props.text.listen.fitWindow}
                        onClick={props.onFitLiveVideoWindow}
                      >
                        <Ratio className="h-3 w-3" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="bottom">
                      {props.text.listen.fitWindow}
                    </TooltipContent>
                  </Tooltip>
                </div>
              ) : null}
              {!props.subtitleDanger ? (
                <span
                  className={cn("listen-video-playback-status", statusClass)}
                >
                  <span>{statusLabel}</span>
                </span>
              ) : null}
            </div>
          </div>
          <div className="listen-video-actions wails-no-drag">
            <ListenFullscreenVolumeControl
              muted={props.muted}
              volume={props.volume}
              text={props.text}
              onToggleMute={props.onToggleMute}
              onVolumeChange={props.onVolumeChange}
            />
            <ListenPlayerIconButton
              label={playLabel}
              tooltip={false}
              disabled={playbackDisabled}
              className="listen-video-action-button listen-video-action-button-primary"
              onClick={props.onTogglePlayback}
            >
              {props.loading ? (
                <Loader2 className="h-4 w-4 listen-loading-spinner" />
              ) : props.playing ? (
                <Pause className="listen-playback-icon--filled h-4 w-4" />
              ) : (
                <Play className="listen-playback-icon--filled ml-0.5 h-4 w-4" />
              )}
            </ListenPlayerIconButton>
            <ListenPlayerIconButton
              label={props.text.listen.stop}
              tooltip={false}
              disabled={stopDisabled}
              className="listen-video-action-button"
              onClick={props.onStopPlayback}
            >
              <Square className="listen-playback-icon--filled h-3.5 w-3.5" />
            </ListenPlayerIconButton>
          </div>
        </div>
      </header>
      <div
        ref={coverAreaRef}
        className="listen-video-cover-area"
        data-native-video={visualLiveVideoVisible ? "underlay" : "pending"}
        data-reveal={liveVideoRevealActive ? "true" : undefined}
      >
        {(!visualLiveVideoVisible || liveVideoRevealActive) ? (
          <div
            className="listen-video-pending-layer"
            data-handoff={liveVideoRevealActive ? "true" : undefined}
          >
            <PetDisplay
              pet={props.pet}
              imageUrl={props.petImageURL}
              animation="review"
              alt={props.title || props.text.listen.selectStation}
              fallbackSrc={LISTEN_DEFAULT_COVER_IMAGE_URL}
              size={88}
              className="h-24 w-24"
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}

function ListenFullscreenChannelCover(props: {
  httpBaseURL: string;
  track?: ListenOnlineItem;
  title: string;
}) {
  return (
    <div className="listen-video-avatar-button" aria-hidden="true">
      <ListenLiveFlatCoverImage
        httpBaseURL={props.httpBaseURL}
        track={props.track}
        title={props.title}
        className="h-full w-full object-cover"
      />
    </div>
  );
}

function ListenLiveFlatCoverImage(props: {
  httpBaseURL: string;
  track?: ListenOnlineItem;
  title: string;
  className?: string;
}) {
  const imageKey = `${props.httpBaseURL}:${props.track?.id ?? ""}:${props.track?.thumbnailUrl ?? ""}:${props.track?.videoId ?? ""}`;
  const candidates = React.useMemo(() => (
    props.track
      ? buildListenPosterCandidates(props.httpBaseURL, props.track)
      : [LISTEN_DEFAULT_COVER_IMAGE_URL]
  ), [props.httpBaseURL, props.track]);

  return (
    <ListenCoverArtwork
      key={imageKey}
      alt={props.title}
      candidates={candidates}
      className={cn("listen-live-flat-cover-image block", props.className)}
      draggable={false}
    />
  );
}

function resolveListenLivePlaybackStatusClass(state: ListenRemotePlaybackState) {
  switch (state) {
    case "playing":
      return "is-playing";
    case "loading":
    case "buffering":
      return "is-loading";
    case "error":
      return "is-error";
    default:
      return "";
  }
}
