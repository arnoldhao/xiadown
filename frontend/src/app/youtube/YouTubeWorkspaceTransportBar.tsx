import {
  AudioLines,
  Captions,
  Download,
  ListVideo,
  Maximize2,
  MessageSquareText,
  Minimize2,
  Pause,
  Play,
  Settings2,
  Volume2,
  VolumeX,
} from "lucide-react";
import * as React from "react";

import type {
  WorkspaceVideoTransportOption,
  WorkspaceVideoTransportPlayback,
  WorkspaceVideoTransportControl,
} from "@/shared/video-transport";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { GlassGroup } from "@/shared/ui/glass-surface";

export interface YouTubeWorkspaceTransportLabels {
  player: string;
  previous: string;
  play: string;
  pause: string;
  next: string;
  fullscreen: string;
  exitFullscreen?: string;
  captions: string;
  audioTrack: string;
  quality: string;
  danmaku: string;
  playbackSpeed: string;
  volume: string;
  mute: string;
  unmute: string;
  download: string;
  upNext: string;
  unavailable: string;
  off: string;
}

export interface YouTubeWorkspaceTransportBarProps {
  playback: WorkspaceVideoTransportPlayback;
  labels: YouTubeWorkspaceTransportLabels;
  upNextOpen?: boolean;
  fullscreenActive?: boolean;
  fullscreenButtonRef?: React.Ref<HTMLButtonElement>;
  visibleControls?: readonly WorkspaceVideoTransportControl[];
  onPrevious: () => void;
  onTogglePlayback: () => void;
  onNext: () => void;
  onDownload?: () => void;
  onToggleUpNext?: () => void;
  onFullscreen: () => void;
  onToggleMute: () => void;
  onToggleCaptions: () => void;
  onSelectCaption: (captionId: string) => void;
  onSelectAudioTrack: (audioTrackId: string) => void;
  onSelectQuality: (qualityId: string) => void;
  onToggleDanmaku?: () => void;
  onSelectPlaybackRate: (playbackRateId: string) => void;
  onVolumeChange: (volume: number) => void;
  onSeek: (seconds: number) => void;
}

export const DEFAULT_WORKSPACE_VIDEO_TRANSPORT_CONTROLS = [
  "download",
  "playbackRate",
  "captions",
  "audioTrack",
  "quality",
  "upNext",
  "volume",
  "fullscreen",
] as const satisfies readonly WorkspaceVideoTransportControl[];

export type YouTubeVolumeEditorState = {
  draft: number;
  dragging: boolean;
  expanded: boolean;
};

export type YouTubeVolumeEditorAction =
  | { type: "sync"; volume: number }
  | { type: "input"; volume: number }
  | { type: "open" }
  | { type: "drag-start" }
  | { type: "drag-release" }
  | { type: "close" }
  | { type: "force-close" };

function clampVolume(value: number) {
  return Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
}

export function youtubeVolumeFromPointerDrag(
  startVolume: number,
  startClientX: number,
  currentClientX: number,
  trackWidth: number,
) {
  if (!Number.isFinite(trackWidth) || trackWidth <= 0) {
    return clampVolume(startVolume);
  }
  return clampVolume(
    Math.round(
      (startVolume + (currentClientX - startClientX) / trackWidth) * 100,
    ) / 100,
  );
}

type YouTubeVolumePointerDrag = {
  pointerId: number;
  startClientX: number;
  startVolume: number;
  trackWidth: number;
  lastVolume: number;
};

export function youtubeVolumeEditorReducer(
  state: YouTubeVolumeEditorState,
  action: YouTubeVolumeEditorAction,
): YouTubeVolumeEditorState {
  switch (action.type) {
    case "sync":
      return state.dragging
        ? state
        : { ...state, draft: clampVolume(action.volume) };
    case "input":
      return { ...state, draft: clampVolume(action.volume) };
    case "open":
      return state.expanded ? state : { ...state, expanded: true };
    case "drag-start":
      return state.dragging && state.expanded
        ? state
        : { ...state, dragging: true, expanded: true };
    case "drag-release":
      return !state.dragging && state.expanded
        ? state
        : { ...state, dragging: false, expanded: true };
    case "close":
      return state.dragging || !state.expanded
        ? state
        : { ...state, expanded: false };
    case "force-close":
      return !state.dragging && !state.expanded
        ? state
        : { ...state, dragging: false, expanded: false };
  }
}

export function YouTubeWorkspaceTransportBar({
  playback,
  labels,
  upNextOpen = false,
  fullscreenActive = false,
  fullscreenButtonRef,
  visibleControls = DEFAULT_WORKSPACE_VIDEO_TRANSPORT_CONTROLS,
  onTogglePlayback,
  onDownload,
  onToggleUpNext,
  onFullscreen,
  onToggleMute,
  onToggleCaptions,
  onSelectCaption,
  onSelectAudioTrack,
  onSelectQuality,
  onToggleDanmaku,
  onSelectPlaybackRate,
  onVolumeChange,
  onSeek,
}: YouTubeWorkspaceTransportBarProps) {
  const { capabilities, descriptor, status } = playback;
  const normalizedVolume = clampVolume(playback.volume);
  const [volumeEditor, dispatchVolumeEditor] = React.useReducer(
    youtubeVolumeEditorReducer,
    { draft: normalizedVolume, dragging: false, expanded: false },
  );
  const volumeExpanded = volumeEditor.expanded;
  const volumeRegionRef = React.useRef<HTMLDivElement>(null);
  const volumeTriggerRef = React.useRef<HTMLButtonElement>(null);
  const volumeSliderRef = React.useRef<HTMLInputElement>(null);
  const volumePointerActiveRef = React.useRef<YouTubeVolumePointerDrag | null>(
    null,
  );
  const volumeKeyboardActiveRef = React.useRef(false);
  const playing = status.state === "playing";
  const duration = Math.max(
    0,
    Number(status.duration || descriptor.durationSeconds || 0),
  );
  const currentTime = Math.max(0, Number(status.currentTime || 0));
  const boundedCurrentTime = duration > 0
    ? Math.min(currentTime, duration)
    : currentTime;
  const progress = duration > 0
    ? Math.min(100, (boundedCurrentTime / duration) * 100)
    : 0;
  const remaining = Math.max(0, duration - boundedCurrentTime);
  const title = descriptor.title.trim() || status.title?.trim() || "";
  const visibleControlSet = new Set<WorkspaceVideoTransportControl>(visibleControls);

  const finishVolumeDrag = React.useCallback((pointerId: number) => {
    if (volumePointerActiveRef.current?.pointerId !== pointerId) {
      return;
    }
    volumePointerActiveRef.current = null;
    dispatchVolumeEditor({ type: "drag-release" });
  }, []);

  React.useEffect(() => {
    if (!capabilities.volume) {
      volumeKeyboardActiveRef.current = false;
      // A player status refresh can transiently lose optional page controls.
      // `close` deliberately does nothing during an active pointer drag.
      dispatchVolumeEditor({ type: "close" });
      return;
    }
    if (volumeExpanded) {
      volumeSliderRef.current?.focus();
    }
  }, [capabilities.volume, volumeExpanded]);

  React.useEffect(() => {
    dispatchVolumeEditor({ type: "sync", volume: normalizedVolume });
  }, [normalizedVolume]);

  React.useEffect(() => {
    if (!volumeExpanded || typeof document === "undefined") {
      return;
    }
    const closeOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && !volumeRegionRef.current?.contains(target)) {
        if (volumePointerActiveRef.current !== null) {
          return;
        }
        dispatchVolumeEditor({ type: "close" });
      }
    };
    const finishOnPointerRelease = (event: PointerEvent) => {
      finishVolumeDrag(event.pointerId);
    };
    const updateOnPointerMove = (event: PointerEvent) => {
      const drag = volumePointerActiveRef.current;
      if (!drag || drag.pointerId !== event.pointerId) {
        return;
      }
      event.preventDefault();
      const nextVolume = youtubeVolumeFromPointerDrag(
        drag.startVolume,
        drag.startClientX,
        event.clientX,
        drag.trackWidth,
      );
      if (nextVolume === drag.lastVolume) {
        return;
      }
      drag.lastVolume = nextVolume;
      dispatchVolumeEditor({ type: "input", volume: nextVolume });
      onVolumeChange(nextVolume);
    };
    document.addEventListener("pointerdown", closeOnOutsidePointer, true);
    window.addEventListener("pointermove", updateOnPointerMove, true);
    window.addEventListener("pointerup", finishOnPointerRelease, true);
    window.addEventListener("pointercancel", finishOnPointerRelease, true);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer, true);
      window.removeEventListener("pointermove", updateOnPointerMove, true);
      window.removeEventListener("pointerup", finishOnPointerRelease, true);
      window.removeEventListener("pointercancel", finishOnPointerRelease, true);
    };
  }, [finishVolumeDrag, onVolumeChange, volumeExpanded]);

  return (
    <GlassGroup
      asChild
      elevation="floating"
      shape="card"
      surfaceRole="control"
    >
      <footer
        className="youtube-workspace-transport"
        role="region"
        aria-label={labels.player}
        data-visible-controls={visibleControls.join(" ")}
      >
        <div className="youtube-workspace-transport-section youtube-workspace-transport-left">
          <TransportButton
            label={playing ? labels.pause : labels.play}
            enabled={capabilities.playPause}
            unavailable={labels.unavailable}
            prominent
            onClick={onTogglePlayback}
          >
            {playing ? <Pause /> : <Play />}
          </TransportButton>
        </div>

        <div
          className="youtube-workspace-transport-timeline"
          role={duration > 0 ? undefined : "progressbar"}
          aria-label={duration > 0 ? undefined : title}
          aria-valuemin={duration > 0 ? undefined : 0}
        >
          <span className="youtube-workspace-transport-times" aria-hidden="true">
            <span>{formatTransportTime(boundedCurrentTime)}</span>
            <span>-{formatTransportTime(remaining)}</span>
          </span>
          <span className="youtube-workspace-transport-progress" aria-hidden="true">
            <span style={{ width: `${progress}%` }} />
          </span>
          {duration > 0 ? (
            <input
              className="youtube-workspace-transport-timeline-input"
              type="range"
              min={0}
              max={duration}
              step={1}
              value={boundedCurrentTime}
              aria-label={title}
              disabled={capabilities.seek === false}
              onChange={(event) => onSeek(Number(event.currentTarget.value))}
            />
          ) : null}
        </div>

        <div
          ref={volumeRegionRef}
          className="youtube-workspace-transport-right"
          data-volume-expanded={volumeExpanded ? "true" : "false"}
          data-volume-dragging={volumeEditor.dragging ? "true" : "false"}
          onKeyDown={(event) => {
            if (event.key === "Escape") {
              volumePointerActiveRef.current = null;
              volumeKeyboardActiveRef.current = false;
              dispatchVolumeEditor({ type: "force-close" });
              window.requestAnimationFrame(() => volumeTriggerRef.current?.focus());
            }
          }}
        >
          <div className="youtube-workspace-transport-right-actions">
            {visibleControlSet.has("download") ? (
              <TransportButton
                className="youtube-workspace-transport-action-download"
                label={labels.download}
                enabled={Boolean(onDownload)}
                unavailable={labels.unavailable}
                onClick={onDownload}
              >
                <Download />
              </TransportButton>
            ) : null}
            {visibleControlSet.has("playbackRate") ? (
              <PlaybackRateControl
                playback={playback}
                labels={labels}
                onSelectPlaybackRate={onSelectPlaybackRate}
              />
            ) : null}
            {visibleControlSet.has("captions") ? (
              <CaptionsControl
                playback={playback}
                labels={labels}
                onToggleCaptions={onToggleCaptions}
                onSelectCaption={onSelectCaption}
              />
            ) : null}
            {visibleControlSet.has("audioTrack") ? (
              <AudioTrackControl
                playback={playback}
                labels={labels}
                onSelectAudioTrack={onSelectAudioTrack}
              />
            ) : null}
            {visibleControlSet.has("quality") ? (
              <QualityControl
                playback={playback}
                labels={labels}
                onSelectQuality={onSelectQuality}
              />
            ) : null}
            {visibleControlSet.has("danmaku") ? (
              <TransportButton
                label={labels.danmaku}
                enabled={capabilities.danmaku === true && Boolean(onToggleDanmaku)}
                unavailable={labels.unavailable}
                active={status.danmakuEnabled === true}
                onClick={onToggleDanmaku}
              >
                <MessageSquareText />
              </TransportButton>
            ) : null}
            {visibleControlSet.has("upNext") ? (
              <TransportButton
                label={labels.upNext}
                enabled={Boolean(onToggleUpNext)}
                unavailable={labels.unavailable}
                active={upNextOpen}
                onClick={onToggleUpNext}
              >
                <ListVideo />
              </TransportButton>
            ) : null}
            {visibleControlSet.has("volume") ? (
              <TransportButton
                buttonRef={volumeTriggerRef}
                label={labels.volume}
                enabled={capabilities.volume}
                unavailable={labels.unavailable}
                onClick={() => {
                  volumePointerActiveRef.current = null;
                  volumeKeyboardActiveRef.current = false;
                  dispatchVolumeEditor({ type: "open" });
                }}
              >
                {playback.muted ? <VolumeX /> : <Volume2 />}
              </TransportButton>
            ) : null}
            {visibleControlSet.has("fullscreen") ? (
              <TransportButton
                buttonRef={fullscreenButtonRef}
                label={fullscreenActive
                  ? labels.exitFullscreen || labels.fullscreen
                  : labels.fullscreen}
                enabled={capabilities.fullscreen}
                unavailable={labels.unavailable}
                active={fullscreenActive}
                onClick={onFullscreen}
              >
                {fullscreenActive ? <Minimize2 /> : <Maximize2 />}
              </TransportButton>
            ) : null}
          </div>
          {volumeExpanded ? (
            <div className="youtube-workspace-volume-editor">
              <TransportButton
                label={playback.muted ? labels.unmute : labels.mute}
                enabled={capabilities.volume}
                unavailable={labels.unavailable}
                active={playback.muted}
                onClick={onToggleMute}
              >
                {playback.muted ? <VolumeX /> : <Volume2 />}
              </TransportButton>
              <input
                ref={volumeSliderRef}
                type="range"
                min={0}
                max={100}
                step={1}
                value={Math.round(volumeEditor.draft * 100)}
                aria-label={labels.volume}
                aria-valuetext={`${Math.round(volumeEditor.draft * 100)}%`}
                onPointerDown={(event) => {
                  if (event.pointerType === "mouse" && event.button !== 0) {
                    return;
                  }
                  event.preventDefault();
                  const trackWidth = event.currentTarget.getBoundingClientRect().width;
                  volumePointerActiveRef.current = {
                    pointerId: event.pointerId,
                    startClientX: event.clientX,
                    startVolume: volumeEditor.draft,
                    trackWidth,
                    lastVolume: volumeEditor.draft,
                  };
                  dispatchVolumeEditor({ type: "drag-start" });
                }}
                onKeyDown={(event) => {
                  if (isVolumeAdjustmentKey(event.key)) {
                    volumeKeyboardActiveRef.current = true;
                  }
                }}
                onKeyUp={() => {
                  volumeKeyboardActiveRef.current = false;
                }}
                onBlur={() => {
                  volumeKeyboardActiveRef.current = false;
                }}
                onInput={(event) => {
                  // Pointer adjustment is handled from explicit pointermove
                  // deltas. Ignore WebKit's native range click-to-jump input.
                  if (!volumeKeyboardActiveRef.current) {
                    event.currentTarget.value = String(
                      Math.round(volumeEditor.draft * 100),
                    );
                    return;
                  }
                  const nextVolume = Number(event.currentTarget.value) / 100;
                  dispatchVolumeEditor({ type: "input", volume: nextVolume });
                  onVolumeChange(nextVolume);
                }}
              />
            </div>
          ) : null}
        </div>
      </footer>
    </GlassGroup>
  );
}

function PlaybackRateControl({
  playback,
  labels,
  onSelectPlaybackRate,
}: {
  playback: WorkspaceVideoTransportPlayback;
  labels: YouTubeWorkspaceTransportLabels;
  onSelectPlaybackRate: (value: string) => void;
}) {
  const options = playback.status.playbackRateOptions || [];
  const currentID = playback.status.selections?.playbackRateId || "1";
  const currentLabel = formatPlaybackRateLabel(currentID);
  const available =
    playback.capabilities.playbackRate === true && options.length > 0;
  const controlLabel = `${labels.playbackSpeed}: ${currentLabel}`;

  if (!available) {
    return (
      <TransportButton
        className="youtube-workspace-transport-speed"
        label={controlLabel}
        enabled={false}
        unavailable={labels.unavailable}
      >
        <span aria-hidden="true">{currentLabel}</span>
      </TransportButton>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="compactIcon"
          shape="control"
          className="youtube-workspace-transport-button youtube-workspace-transport-speed"
          aria-label={controlLabel}
          title={controlLabel}
        >
          <span aria-hidden="true">{currentLabel}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="top"
        align="center"
        className="youtube-workspace-settings-menu"
      >
        <DropdownMenuLabel>{labels.playbackSpeed}</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={currentID}
          onValueChange={onSelectPlaybackRate}
        >
          {options.map((option) => (
            <DropdownMenuRadioItem key={option.id} value={option.id}>
              {formatPlaybackRateLabel(option.id || option.label)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function CaptionsControl({
  playback,
  labels,
  onToggleCaptions,
  onSelectCaption,
}: {
  playback: WorkspaceVideoTransportPlayback;
  labels: YouTubeWorkspaceTransportLabels;
  onToggleCaptions: () => void;
  onSelectCaption: (value: string) => void;
}) {
  const { capabilities, status } = playback;
  const captionOptions = status.captionOptions || [];
  const available = capabilities.captions && captionOptions.length > 0;

  if (!available) {
    return (
      <TransportButton
        label={labels.captions}
        enabled={false}
        unavailable={labels.unavailable}
      >
        <Captions />
      </TransportButton>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="compactIcon"
          shape="control"
          className="youtube-workspace-transport-button"
          aria-label={labels.captions}
          title={labels.captions}
          data-active={Boolean(status.selections?.captionId)}
        >
          <Captions />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="top"
        align="center"
        className="youtube-workspace-settings-menu"
      >
        <DropdownMenuLabel className="flex items-center gap-2">
          <Captions className="h-3.5 w-3.5" />
          {labels.captions}
        </DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={status.selections?.captionId || ""}
          onValueChange={onSelectCaption}
        >
          <DropdownMenuRadioItem value="">{labels.off}</DropdownMenuRadioItem>
          {captionOptions.map((option) => (
            <DropdownMenuRadioItem key={option.id} value={option.id}>
              <span className="min-w-0 flex-1 truncate">{option.label}</span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function AudioTrackControl({
  playback,
  labels,
  onSelectAudioTrack,
}: {
  playback: WorkspaceVideoTransportPlayback;
  labels: YouTubeWorkspaceTransportLabels;
  onSelectAudioTrack: (value: string) => void;
}) {
  const options = playback.status.audioTrackOptions || [];
  const available = playback.capabilities.audioTrack && options.length > 0;

  if (!available) {
    return (
      <TransportButton
        className="youtube-workspace-transport-action-audio"
        label={labels.audioTrack}
        enabled={false}
        unavailable={labels.unavailable}
      >
        <AudioLines />
      </TransportButton>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="compactIcon"
          shape="control"
          className="youtube-workspace-transport-button youtube-workspace-transport-action-audio"
          aria-label={labels.audioTrack}
          title={labels.audioTrack}
        >
          <AudioLines />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="top"
        align="center"
        className="youtube-workspace-settings-menu"
      >
        <DropdownMenuLabel className="flex items-center gap-2">
          <AudioLines className="h-3.5 w-3.5" />
          {labels.audioTrack}
        </DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={playback.status.selections?.audioTrackId || ""}
          onValueChange={onSelectAudioTrack}
        >
          {options.map((option) => (
            <DropdownMenuRadioItem key={option.id} value={option.id}>
              <span className="min-w-0 flex-1 truncate">{option.label}</span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function QualityControl({
  playback,
  labels,
  onSelectQuality,
}: {
  playback: WorkspaceVideoTransportPlayback;
  labels: YouTubeWorkspaceTransportLabels;
  onSelectQuality: (value: string) => void;
}) {
  const options = playback.status.qualityOptions || [];
  const available = playback.capabilities.quality && options.length > 0;

  if (!available) {
    return (
      <TransportButton
        label={labels.quality}
        enabled={false}
        unavailable={labels.unavailable}
      >
        <Settings2 />
      </TransportButton>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="compactIcon"
          shape="control"
          className="youtube-workspace-transport-button"
          aria-label={labels.quality}
          title={labels.quality}
        >
          <Settings2 />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="top"
        align="center"
        className="youtube-workspace-settings-menu"
      >
        <DropdownMenuLabel>{labels.quality}</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={playback.status.selections?.qualityId || ""}
          onValueChange={onSelectQuality}
        >
          {options.map((option) => (
            <DropdownMenuRadioItem key={option.id} value={option.id}>
              <span className="min-w-0 flex-1 truncate">
                {formatYouTubeQualityLabel(option)}
              </span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

const youtubeQualityLabels: Record<string, string> = {
  auto: "Auto",
  highres: "Highest quality",
  hd2160: "2160p · 4K",
  hd1440: "1440p · QHD",
  hd1080: "1080p · Full HD",
  hd720: "720p · HD",
  large: "480p · SD",
  medium: "360p",
  small: "240p",
  tiny: "144p",
};

export function formatYouTubeQualityLabel(option: WorkspaceVideoTransportOption) {
  const id = option.id.trim().toLowerCase();
  const label = option.label.trim();
  const normalizedLabel = label.toLowerCase();
  const known = youtubeQualityLabels[id] || youtubeQualityLabels[normalizedLabel];
  if (known) {
    return known;
  }
  const resolution =
    id.match(/^hd(\d{3,4})$/)?.[1] ||
    normalizedLabel.match(/^hd(\d{3,4})$/)?.[1];
  if (resolution) {
    return `${resolution}p`;
  }
  const fallback = label || option.id;
  return fallback
    ? `${fallback.charAt(0).toLocaleUpperCase()}${fallback.slice(1)}`
    : "Auto";
}

export function formatPlaybackRateLabel(value: string) {
  const rate = Number.parseFloat(value);
  if (!Number.isFinite(rate) || rate <= 0) {
    return "1×";
  }
  return `${Number(rate.toFixed(2))}×`;
}

function isVolumeAdjustmentKey(key: string) {
  return [
    "ArrowDown",
    "ArrowLeft",
    "ArrowRight",
    "ArrowUp",
    "End",
    "Home",
    "PageDown",
    "PageUp",
  ].includes(key);
}

function formatTransportTime(seconds: number) {
  const normalized = Number.isFinite(seconds)
    ? Math.max(0, Math.floor(seconds))
    : 0;
  const hours = Math.floor(normalized / 3600);
  const minutes = Math.floor((normalized % 3600) / 60);
  const remainder = normalized % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`
    : `${minutes}:${String(remainder).padStart(2, "0")}`;
}

function TransportButton({
  buttonRef,
  className,
  label,
  enabled,
  unavailable,
  prominent = false,
  active,
  onClick,
  children,
}: {
  buttonRef?: React.Ref<HTMLButtonElement>;
  className?: string;
  label: string;
  enabled: boolean;
  unavailable: string;
  prominent?: boolean;
  active?: boolean;
  onClick?: () => void;
  children: React.ReactNode;
}) {
  return (
    <Button
      ref={buttonRef}
      type="button"
      variant="ghost"
      size="compactIcon"
      shape="control"
      className={[
        "youtube-workspace-transport-button",
        className,
      ].filter(Boolean).join(" ")}
      data-prominent={prominent ? "true" : undefined}
      aria-label={label}
      aria-pressed={active}
      data-active={active}
      title={enabled ? label : `${label} · ${unavailable}`}
      disabled={!enabled}
      onClick={onClick}
    >
      {children}
    </Button>
  );
}
