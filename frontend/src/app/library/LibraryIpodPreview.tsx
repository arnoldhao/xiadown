import {
  Eye,
  FastForward,
  LoaderCircle,
  Pause,
  Play,
  Rewind,
  RotateCcw,
  Volume2,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import * as React from "react";

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";

import { LibraryArtwork } from "./LibraryArtwork";
import {
  capturePointerZoomAnchor,
  restorePointerZoomAnchor,
  zoomAfterWheel,
  type PointerZoomAnchor,
} from "./library-pointer-zoom";
import type {
  LibraryItemCategory,
  LibraryOtherGroup,
  LibraryWorkspaceLabels,
} from "./types";

const PLAYBACK_SKIP_SECONDS = 10;
const MIN_IMAGE_ZOOM = 0.5;
const MAX_IMAGE_ZOOM = 3;
const IMAGE_ZOOM_STEP = 0.25;

export function clampPlaybackTime(value: number, duration: number) {
  const safeDuration = Number.isFinite(duration) && duration > 0 ? duration : 0;
  if (!Number.isFinite(value)) return 0;
  return Math.min(Math.max(0, value), safeDuration);
}

export function clampImageZoom(value: number) {
  if (!Number.isFinite(value)) return 1;
  return Math.min(MAX_IMAGE_ZOOM, Math.max(MIN_IMAGE_ZOOM, value));
}

export function imageZoomAfterWheel(
  current: number,
  deltaY: number,
  deltaMode = 0,
  viewportHeight = 1,
) {
  return zoomAfterWheel(
    current,
    deltaY,
    deltaMode,
    viewportHeight,
    clampImageZoom,
  );
}

function formatPlaybackSeconds(value: number) {
  if (!Number.isFinite(value) || value < 0) return "0:00";
  const seconds = Math.floor(value);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remaining = seconds % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(remaining).padStart(2, "0")}`
    : `${minutes}:${String(remaining).padStart(2, "0")}`;
}

interface IpodControlWheelProps {
  topLabel: string;
  leftLabel: string;
  rightLabel: string;
  bottomLabel: string;
  topIcon: React.ReactNode;
  leftIcon: React.ReactNode;
  rightIcon: React.ReactNode;
  bottomIcon: React.ReactNode;
  onTop: () => void;
  onLeft: () => void;
  onRight: () => void;
  onBottom: () => void;
  topPressed?: boolean;
  disabled?: boolean;
  topDisabled?: boolean;
  leftDisabled?: boolean;
  rightDisabled?: boolean;
  bottomDisabled?: boolean;
}

export function IpodControlWheel(props: IpodControlWheelProps) {
  const button = (
    position: "top" | "left" | "right" | "bottom",
    label: string,
    icon: React.ReactNode,
    onClick: () => void,
    pressed?: boolean,
    disabled?: boolean,
  ) => (
    <button
      aria-label={label}
      aria-pressed={pressed}
      className="app-library-ipod__wheel-button"
      data-position={position}
      disabled={props.disabled || disabled}
      onClick={onClick}
      title={label}
      type="button"
    >
      {icon}
    </button>
  );

  return (
    <div className="app-library-ipod__wheel" role="group">
      {button(
        "top",
        props.topLabel,
        props.topIcon,
        props.onTop,
        props.topPressed,
        props.topDisabled,
      )}
      {button(
        "left",
        props.leftLabel,
        props.leftIcon,
        props.onLeft,
        undefined,
        props.leftDisabled,
      )}
      <span className="app-library-ipod__wheel-hub" aria-hidden="true" />
      {button(
        "right",
        props.rightLabel,
        props.rightIcon,
        props.onRight,
        undefined,
        props.rightDisabled,
      )}
      {button(
        "bottom",
        props.bottomLabel,
        props.bottomIcon,
        props.onBottom,
        undefined,
        props.bottomDisabled,
      )}
    </div>
  );
}

interface CommonIpodPreviewProps {
  title: string;
  coverURL: string;
  fallbackCoverURL?: string;
  sourceURL?: string;
  labels: LibraryWorkspaceLabels;
  otherGroup?: LibraryOtherGroup;
}

function PlayableIpodPreview(
  props: CommonIpodPreviewProps & { category: "video" | "audio" },
) {
  const mediaRef = React.useRef<HTMLMediaElement | null>(null);
  const dialogMediaRef = React.useRef<HTMLMediaElement | null>(null);
  const [playing, setPlaying] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [unavailable, setUnavailable] = React.useState(false);
  const [currentTime, setCurrentTime] = React.useState(0);
  const [duration, setDuration] = React.useState(0);
  const [volume, setVolume] = React.useState(1);
  const [hasVideoFrame, setHasVideoFrame] = React.useState(false);
  const [rangeMode, setRangeMode] = React.useState<"progress" | "volume">("progress");
  const [dialogOpen, setDialogOpen] = React.useState(false);

  React.useEffect(() => {
    const media = mediaRef.current;
    media?.pause();
    setPlaying(false);
    setLoading(false);
    setUnavailable(false);
    setCurrentTime(0);
    setDuration(0);
    setHasVideoFrame(false);
    setRangeMode("progress");
    setDialogOpen(false);
    return () => {
      media?.pause();
      dialogMediaRef.current?.pause();
    };
  }, [props.category, props.sourceURL]);

  React.useEffect(() => {
    if (mediaRef.current) mediaRef.current.volume = volume;
    if (dialogMediaRef.current) dialogMediaRef.current.volume = volume;
  }, [volume]);

  const togglePlayback = React.useCallback(() => {
    const media = mediaRef.current;
    if (!media || unavailable) return;
    if (media.paused) {
      setLoading(true);
      void media.play().catch(() => {
        setLoading(false);
        setPlaying(false);
      });
    } else {
      media.pause();
    }
  }, [unavailable]);

  const seek = React.useCallback((value: number) => {
    const media = mediaRef.current;
    if (!media || !Number.isFinite(value)) return;
    const next = clampPlaybackTime(value, media.duration);
    media.currentTime = next;
    setCurrentTime(next);
  }, []);

  const skip = React.useCallback((delta: number) => {
    seek(currentTime + delta);
  }, [currentTime, seek]);

  const updateDialogOpen = React.useCallback((next: boolean) => {
    const inlineMedia = mediaRef.current;
    const dialogMedia = dialogMediaRef.current;
    if (next) {
      inlineMedia?.pause();
      setPlaying(false);
      setLoading(false);
      setDialogOpen(true);
      return;
    }
    const nextTime = dialogMedia && Number.isFinite(dialogMedia.currentTime)
      ? dialogMedia.currentTime
      : currentTime;
    dialogMedia?.pause();
    if (inlineMedia && Number.isFinite(nextTime)) {
      inlineMedia.currentTime = clampPlaybackTime(
        nextTime,
        inlineMedia.duration,
      );
    }
    setCurrentTime(nextTime);
    setPlaying(false);
    setLoading(false);
    setDialogOpen(false);
  }, [currentTime]);

  const onRangeChange = (value: number) => {
    if (rangeMode === "volume") {
      setVolume(Math.min(1, Math.max(0, value)));
      return;
    }
    seek(value);
  };
  const rangeValue = rangeMode === "volume" ? volume : Math.min(currentTime, duration || 0);
  const rangeMax = rangeMode === "volume" ? 1 : Math.max(duration, 0);
  const rangePercent = rangeMode === "volume"
    ? volume * 100
    : duration > 0
      ? Math.min(100, Math.max(0, (currentTime / duration) * 100))
      : 0;
  const playLabel = loading ? props.labels.loading : playing ? props.labels.pause : props.labels.play;
  const sourceURL = props.sourceURL?.trim() ?? "";
  const attachMediaRef = (node: HTMLMediaElement | null) => {
    mediaRef.current = node;
  };
  const attachDialogMediaRef = (node: HTMLMediaElement | null) => {
    dialogMediaRef.current = node;
    if (node) node.volume = volume;
  };
  const prepareDialogPlayback = (
    event: React.SyntheticEvent<HTMLMediaElement>,
  ) => {
    const media = event.currentTarget;
    media.currentTime = clampPlaybackTime(currentTime, media.duration);
    void media.play().catch(() => {
      setPlaying(false);
    });
  };
  const mediaEvents = {
    onCanPlay: () => {
      setLoading(false);
      setUnavailable(false);
    },
    onDurationChange: (event: React.SyntheticEvent<HTMLMediaElement>) => {
      const next = Number.isFinite(event.currentTarget.duration)
        ? event.currentTarget.duration
        : 0;
      setDuration(next);
    },
    onEnded: () => {
      setPlaying(false);
      setLoading(false);
    },
    onError: () => {
      setPlaying(false);
      setLoading(false);
      setUnavailable(true);
      setHasVideoFrame(false);
    },
    onPause: () => {
      setPlaying(false);
      setLoading(false);
    },
    onPlaying: () => {
      setPlaying(true);
      setLoading(false);
      setUnavailable(false);
      setHasVideoFrame(true);
    },
    onTimeUpdate: (event: React.SyntheticEvent<HTMLMediaElement>) => {
      setCurrentTime(event.currentTarget.currentTime);
    },
    onWaiting: () => setLoading(true),
  };
  const dialogMediaEvents = {
    onDurationChange: (event: React.SyntheticEvent<HTMLMediaElement>) => {
      const next = Number.isFinite(event.currentTarget.duration)
        ? event.currentTarget.duration
        : 0;
      setDuration(next);
    },
    onEnded: () => setPlaying(false),
    onError: () => {
      setPlaying(false);
      setUnavailable(true);
    },
    onLoadedMetadata: prepareDialogPlayback,
    onPause: () => setPlaying(false),
    onPlaying: () => {
      setPlaying(true);
      setUnavailable(false);
    },
    onTimeUpdate: (event: React.SyntheticEvent<HTMLMediaElement>) => {
      setCurrentTime(event.currentTarget.currentTime);
    },
  };

  return (
    <>
      <div className="app-library-ipod" data-media-kind={props.category}>
        <div className="app-library-ipod__screen">
          <div className="app-library-ipod__display">
            {props.category === "video" ? (
              <>
                <video
                  {...mediaEvents}
                  aria-label={props.title}
                  playsInline
                  preload="metadata"
                  ref={attachMediaRef}
                  src={sourceURL || undefined}
                />
                {!hasVideoFrame ? (
                  <LibraryArtwork
                    alt=""
                    category="video"
                    className="app-library-ipod__video-poster"
                    fallbackSrc={props.fallbackCoverURL}
                    src={props.coverURL}
                  />
                ) : null}
              </>
            ) : (
              <>
                <LibraryArtwork
                  alt=""
                  category="audio"
                  fallbackSrc={props.fallbackCoverURL}
                  src={props.coverURL}
                />
                <audio
                  {...mediaEvents}
                  aria-hidden="true"
                  preload="metadata"
                  ref={attachMediaRef}
                  src={sourceURL || undefined}
                />
              </>
            )}
            {loading ? (
              <LoaderCircle
                aria-hidden="true"
                className="app-library-ipod__loading app-motion-spin"
              />
            ) : null}
          </div>
          <div className="app-library-ipod__range">
            <input
              aria-label={rangeMode === "volume" ? props.labels.volume : props.labels.seek}
              aria-valuetext={rangeMode === "volume"
                ? `${Math.round(volume * 100)}%`
                : `${formatPlaybackSeconds(currentTime)} / ${formatPlaybackSeconds(duration)}`}
              disabled={unavailable || (rangeMode === "progress" && duration <= 0)}
              max={rangeMax}
              min={0}
              onChange={(event) => onRangeChange(Number(event.currentTarget.value))}
              step={rangeMode === "volume" ? 0.01 : 0.1}
              style={{ "--app-library-ipod-range-value": `${rangePercent}%` } as React.CSSProperties}
              type="range"
              value={rangeValue}
            />
            <div>
              <span>
                {rangeMode === "volume"
                  ? props.labels.volume
                  : formatPlaybackSeconds(currentTime)}
              </span>
              <button
                aria-label={props.labels.volume}
                aria-pressed={rangeMode === "volume"}
                className="app-library-ipod__range-mode"
                disabled={unavailable || !sourceURL}
                onClick={() => setRangeMode((current) =>
                  current === "progress" ? "volume" : "progress")}
                title={props.labels.volume}
                type="button"
              >
                <Volume2 aria-hidden="true" size={13} />
              </button>
              <span>
                {rangeMode === "volume"
                  ? `${Math.round(volume * 100)}%`
                  : formatPlaybackSeconds(duration)}
              </span>
            </div>
          </div>
        </div>
        <IpodControlWheel
          bottomIcon={loading
            ? <LoaderCircle aria-hidden="true" className="app-motion-spin" size={18} />
            : playing
              ? <Pause aria-hidden="true" size={18} />
              : <Play aria-hidden="true" size={18} />}
          bottomLabel={playLabel}
          disabled={unavailable || !sourceURL}
          leftIcon={<Rewind aria-hidden="true" size={18} />}
          leftLabel={`${props.labels.seek} −${PLAYBACK_SKIP_SECONDS}s`}
          onBottom={togglePlayback}
          onLeft={() => skip(-PLAYBACK_SKIP_SECONDS)}
          onRight={() => skip(PLAYBACK_SKIP_SECONDS)}
          onTop={() => updateDialogOpen(true)}
          rightIcon={<FastForward aria-hidden="true" size={18} />}
          rightLabel={`${props.labels.seek} +${PLAYBACK_SKIP_SECONDS}s`}
          topIcon={<Eye aria-hidden="true" size={18} />}
          topLabel={props.labels.preview}
        />
        {unavailable ? (
          <p className="app-library-ipod__error" role="alert">
            {props.labels.loadFailed}
          </p>
        ) : null}
      </div>
      <Dialog open={dialogOpen} onOpenChange={updateDialogOpen}>
        <DialogContent className="app-library-playable-dialog app-media-preview-dialog min-w-0 max-w-none">
          <DialogHeader className="app-media-preview-dialog-header app-library-playable-dialog__header">
            <DialogTitle
              className="app-library-playable-dialog__title"
              title={props.title}
            >
              {props.title}
            </DialogTitle>
          </DialogHeader>
          <div
            className="app-library-playable-dialog__stage app-media-preview-dialog-stage"
            data-kind={props.category}
          >
            {props.category === "video" ? (
              <video
                {...dialogMediaEvents}
                aria-label={props.title}
                autoPlay
                controls
                playsInline
                preload="metadata"
                ref={attachDialogMediaRef}
                src={sourceURL || undefined}
              />
            ) : (
              <div className="app-library-playable-dialog__audio">
                <LibraryArtwork
                  alt=""
                  category="audio"
                  fallbackSrc={props.fallbackCoverURL}
                  src={props.coverURL}
                />
                <audio
                  {...dialogMediaEvents}
                  aria-label={props.title}
                  autoPlay
                  controls
                  preload="metadata"
                  ref={attachDialogMediaRef}
                  src={sourceURL || undefined}
                />
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

function ArtworkIpodPreview(
  props: CommonIpodPreviewProps & { category: LibraryItemCategory },
) {
  const dialogStageRef = React.useRef<HTMLDivElement | null>(null);
  const dialogAnchorRef = React.useRef<PointerZoomAnchor | null>(null);
  const [zoom, setZoom] = React.useState(1);
  const [dialogZoom, setDialogZoom] = React.useState(1);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const sourceURL = props.category === "image"
    ? props.sourceURL?.trim() || props.coverURL
    : props.coverURL;

  React.useEffect(() => {
    setZoom(1);
    setDialogZoom(1);
    setDialogOpen(false);
    dialogAnchorRef.current = null;
  }, [props.category, sourceURL]);

  const updateZoom = (next: number) => {
    setZoom(clampImageZoom(next));
  };
  const updateDialogZoomFromWheel = (
    event: React.WheelEvent<HTMLDivElement>,
  ) => {
    const next = imageZoomAfterWheel(
      dialogZoom,
      event.deltaY,
      event.deltaMode,
      event.currentTarget.clientHeight,
    );
    event.preventDefault();
    if (next === dialogZoom) return;
    dialogAnchorRef.current = capturePointerZoomAnchor(
      event.currentTarget,
      event.clientX,
      event.clientY,
    );
    setDialogZoom(next);
    window.requestAnimationFrame(() => {
      const stage = dialogStageRef.current;
      const anchor = dialogAnchorRef.current;
      if (!stage || !anchor) return;
      restorePointerZoomAnchor(stage, anchor);
      dialogAnchorRef.current = null;
    });
  };
  const zoomLabel = `${Math.round(zoom * 100)}%`;

  return (
    <>
      <div className="app-library-ipod" data-media-kind={props.category}>
        <div className="app-library-ipod__screen">
          <div className="app-library-ipod__display app-library-ipod__display--image">
            <div style={{ transform: `scale(${zoom})` }}>
              <LibraryArtwork
                alt={props.title}
                category={props.category}
                fallbackSrc={props.fallbackCoverURL}
                otherGroup={props.otherGroup}
                src={sourceURL}
              />
            </div>
          </div>
          <div className="app-library-ipod__range">
            <input
              aria-label={props.labels.size}
              aria-valuetext={zoomLabel}
              max={MAX_IMAGE_ZOOM}
              min={MIN_IMAGE_ZOOM}
              onChange={(event) => updateZoom(Number(event.currentTarget.value))}
              step={IMAGE_ZOOM_STEP}
              style={{
                "--app-library-ipod-range-value": `${((zoom - MIN_IMAGE_ZOOM) / (MAX_IMAGE_ZOOM - MIN_IMAGE_ZOOM)) * 100}%`,
              } as React.CSSProperties}
              type="range"
              value={zoom}
            />
            <div><span>{props.labels.size}</span><span>{zoomLabel}</span></div>
          </div>
        </div>
        <IpodControlWheel
          bottomIcon={<RotateCcw aria-hidden="true" size={18} />}
          bottomLabel={props.labels.reset}
          leftIcon={<ZoomOut aria-hidden="true" size={18} />}
          leftLabel={`${props.labels.size} −`}
          onBottom={() => setZoom(1)}
          onLeft={() => updateZoom(zoom - IMAGE_ZOOM_STEP)}
          onRight={() => updateZoom(zoom + IMAGE_ZOOM_STEP)}
          onTop={() => {
            setDialogZoom(1);
            setDialogOpen(true);
          }}
          rightIcon={<ZoomIn aria-hidden="true" size={18} />}
          rightLabel={`${props.labels.size} +`}
          topIcon={<Eye aria-hidden="true" size={18} />}
          topLabel={props.labels.preview}
        />
      </div>
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="app-library-ipod-dialog app-media-preview-dialog min-w-0 max-w-none">
          <DialogHeader className="app-media-preview-dialog-header app-library-ipod-dialog__header">
            <DialogTitle
              className="app-library-ipod-dialog__title"
              title={props.title}
            >
              {props.title}
            </DialogTitle>
          </DialogHeader>
          <div
            aria-label={`${props.labels.preview}: ${props.title} · ${Math.round(dialogZoom * 100)}%`}
            className="app-library-ipod-dialog__stage app-media-preview-dialog-stage"
            data-zoomed={dialogZoom === 1 ? undefined : "true"}
            onWheel={updateDialogZoomFromWheel}
            ref={dialogStageRef}
          >
            <div
              className="app-library-ipod-dialog__zoom-content"
              style={{
                height: `${dialogZoom * 100}%`,
                width: `${dialogZoom * 100}%`,
              }}
            >
              <LibraryArtwork
                alt={props.title}
                category={props.category}
                className="app-library-ipod-dialog__image app-media-preview-dialog-image"
                fallbackSrc={props.fallbackCoverURL}
                otherGroup={props.otherGroup}
                src={sourceURL}
              />
            </div>
            <span className="app-library-ipod-dialog__zoom-indicator" aria-hidden="true">
              {Math.round(dialogZoom * 100)}%
            </span>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function LibraryIpodPreview(
  props: CommonIpodPreviewProps & {
    category: LibraryItemCategory;
  },
) {
  return props.category === "video" || props.category === "audio"
    ? <PlayableIpodPreview {...props} category={props.category} />
    : <ArtworkIpodPreview {...props} category={props.category} />;
}
