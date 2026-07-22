import {
ChevronLeft,
ChevronRight,
ChevronUp,
ChevronDown,
Disc3,
Link2,
ListMusic,
Loader2,
Music2,
Pause,
Pencil,
Play,
Plus,
Radio,
RefreshCw,
Repeat2,
Redo2,
Shuffle,
Tags,
SkipBack,
SkipForward,
Trash2,
Undo2,
UserRound,
Video,
Volume2,
VolumeX
} from "lucide-react";
import * as React from "react";
import {
siYoutubemusic
} from "simple-icons";

import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import { ListenCoverArtwork } from "@/shared/assets/listen-cover-artwork";
import { Button } from "@/shared/ui/button";
import {
DreamSegmentSwitch,
type DreamSegmentSwitchItem,
} from "@/shared/ui/dream-segment-switch";
import {
SidebarMenu,
SidebarMenuButton,
SidebarMenuItem,
} from "@/shared/ui/sidebar";
import { StatusBadge } from "@/shared/ui/status-badge";
import {
Tooltip,
TooltipContent,
TooltipTrigger
} from "@/shared/ui/tooltip";
import {
LISTEN_CONTROL_ICON_BUTTON_CLASS,
LISTEN_CONTROL_SURFACE_CLASS,
LISTEN_LIST_ITEM_BUTTON_CLASS,
LISTEN_LIST_SECTION_TITLE_CLASS,
} from "@/shared/styles/listen";

import { clampVolume,formatProgressSeconds } from "@/app/main/listen/local-library";
import { resolveTrustedListenOnlineArtistLabel } from "@/app/main/listen/playback-helpers";
import { buildListenAvatarImageCandidates,buildListenImageCandidates,buildListenPosterCandidates } from "@/app/main/listen/storage";
import type { ListenArtistItem,ListenCategoryItem,ListenLiveStatus,ListenLiveStatusValue,ListenLocalItem,ListenMode,ListenOnlineItem,ListenPlayMode,ListenPlaylistItem,ListenPlaylistLibraryAction } from "@/app/main/listen/types";
import { doesListenThumbnailSuggestVideoContent,hasListenMusicVideoContent,isListenMusicVideoKnownNoVideo } from "@/app/main/listen/video-types";

type ListenArtworkShape = "square" | "circle";

function SimpleBrandIcon(props: {
  className?: string;
  icon: { path: string; title: string };
}) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
      className={cn("block shrink-0", props.className)}
    >
      <path d={props.icon.path} />
    </svg>
  );
}

export function ListenAvatar(props: {
  httpBaseURL: string;
  item: { channel: string; thumbnailUrl?: string; videoId?: string };
  selected?: boolean;
  shape?: ListenArtworkShape;
}) {
  const avatarCandidates = React.useMemo(
    () => buildListenAvatarImageCandidates(props.httpBaseURL, props.item),
    [props.httpBaseURL, props.item.thumbnailUrl, props.item.videoId],
  );
  const avatarCandidateKey = avatarCandidates.join("\n");
  const [avatarIndex, setAvatarIndex] = React.useState(0);
  const [imageReady, setImageReady] = React.useState(false);
  const activeAvatarURL =
    avatarCandidates[
      Math.min(avatarIndex, Math.max(avatarCandidates.length - 1, 0))
    ] ?? "";

  React.useEffect(() => {
    setAvatarIndex(0);
    setImageReady(false);
  }, [avatarCandidateKey]);

  return (
    <div
      data-selected={props.selected ? "true" : undefined}
      data-shape={props.shape === "circle" ? "circle" : undefined}
      className={cn(
        "listen-avatar relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden",
      )}
    >
      <span className="listen-artwork-placeholder absolute inset-0" />
      {!imageReady ? (
        <span className="listen-avatar__placeholder pointer-events-none relative z-0">
          {props.shape === "circle" ? (
            <UserRound className="h-4 w-4" />
          ) : (
            <Music2 className="h-4 w-4" />
          )}
        </span>
      ) : null}
      {activeAvatarURL ? (
        <img
          key={activeAvatarURL}
          src={activeAvatarURL}
          alt=""
          className={cn(
            "listen-image-reveal absolute inset-0 z-10 h-full w-full object-cover",
          )}
          data-ready={imageReady ? "true" : "false"}
          loading="eager"
          onLoad={() => setImageReady(true)}
          onError={() => {
            setImageReady(false);
            setAvatarIndex((current) => {
              if (current >= avatarCandidates.length - 1) {
                return current;
              }
              return current + 1;
            });
          }}
        />
      ) : null}
    </div>
  );
}

export function ListenLocalArtwork(props: {
  track: ListenLocalItem;
  className?: string;
}) {
  const coverURL = props.track.coverURL.trim();

  return (
    <ListenCoverArtwork
      alt=""
      candidates={[coverURL, LISTEN_DEFAULT_COVER_IMAGE_URL]}
      className={cn(
        "listen-local-artwork flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden",
        props.className,
      )}
      loading="lazy"
    />
  );
}

export function ListenOnlineArtwork(props: {
  httpBaseURL: string;
  track: ListenOnlineItem;
  className?: string;
  visualizer?: React.ReactNode;
  visualizerVisible?: boolean;
}) {
  const posterCandidates = React.useMemo(
    () => buildListenPosterCandidates(props.httpBaseURL, props.track),
    [props.httpBaseURL, props.track.thumbnailUrl, props.track.videoId],
  );
  return (
    <ListenArtworkShell
      className={props.className}
      visualizer={props.visualizer}
      visualizerVisible={props.visualizerVisible}
    >
      <>
        <ListenCoverArtwork
          alt={props.track.title}
          candidates={posterCandidates}
          className="h-full w-full"
          imageClassName="listen-cover-artwork-motion-image"
          changeSweep
        />
        <div className="listen-cover-artwork-wash pointer-events-none absolute inset-0" />
      </>
    </ListenArtworkShell>
  );
}

export function ListenActionIconButton(props: {
  label: string;
  disabled?: boolean;
  className?: string;
  tone?: "floating" | "grouped";
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">
          <Button
            type="button"
            variant="outline"
            size="compactIcon"
            shape="circle"
            className={cn(
              "h-10 w-10",
              LISTEN_CONTROL_ICON_BUTTON_CLASS,
              props.className,
            )}
            aria-label={props.label}
            title={props.label}
            disabled={props.disabled}
            onClick={props.onClick}
          >
            {props.children}
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">{props.label}</TooltipContent>
    </Tooltip>
  );
}

export function ListenVolumeControl(props: {
  hasTrack: boolean;
  muted: boolean;
  volume: number;
  text: ReturnType<typeof getXiaText>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const visibleVolume = props.muted ? 0 : clampVolume(props.volume);
  const muteLabel =
    props.muted || props.volume <= 0
      ? props.text.listen.unmute
      : props.text.listen.mute;
  const volumePercent = Math.round(visibleVolume * 1000) / 10;
  return (
    <Tooltip>
      <div className="listen-volume-control group/volume flex items-center">
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="compactIcon"
            shape="circle"
            className={cn(
              "h-10 w-10",
              LISTEN_CONTROL_ICON_BUTTON_CLASS,
            )}
            disabled={!props.hasTrack}
            aria-label={muteLabel}
            title={muteLabel}
            onClick={props.onToggleMute}
          >
            {props.muted || props.volume <= 0 ? (
              <VolumeX className="h-4 w-4" />
            ) : (
              <Volume2 className="h-4 w-4" />
            )}
          </Button>
        </TooltipTrigger>
        <span
          className="listen-volume-control__reveal ml-0 block w-0 overflow-hidden group-hover/volume:ml-2 group-hover/volume:w-20 group-focus-within/volume:ml-2 group-focus-within/volume:w-20"
        >
          <span
            className="listen-volume-control__slider relative flex h-6 w-20 items-center"
            data-disabled={!props.hasTrack ? "true" : undefined}
          >
            <span className="listen-volume-control__track pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden">
              <span
                className="listen-volume-control__fill absolute inset-y-0 left-0"
                style={{ width: `${volumePercent}%` }}
              />
            </span>
            <input
              type="range"
              min={0}
              max={1}
              step={0.01}
              value={visibleVolume}
              disabled={!props.hasTrack}
              aria-label={props.text.listen.volume}
              title={props.text.listen.volume}
              className="listen-volume-control__input relative z-10 h-6 w-full"
              onChange={(event) =>
                props.onVolumeChange(Number(event.target.value))
              }
            />
          </span>
        </span>
      </div>
      <TooltipContent side="top">{props.text.listen.volume}</TooltipContent>
    </Tooltip>
  );
}

export function ListenArtworkShell(props: {
  className?: string;
  visualizer?: React.ReactNode;
  visualizerVisible?: boolean;
  children: React.ReactNode;
}) {
  const [frameActive, setFrameActive] = React.useState(false);
  return (
    <div
      data-frame-active={frameActive ? "true" : "false"}
      data-visualizer-visible={props.visualizerVisible === true ? "true" : "false"}
      className={cn(
        "listen-artwork-shell relative isolate w-full shrink-0 overflow-visible",
        props.className,
      )}
    >
      <div
        className={cn(
          "listen-artwork-shadow absolute inset-0 z-0 translate-y-5",
        )}
      />
      {props.visualizer}
      <div
        className="listen-artwork-frame relative z-10 aspect-square overflow-hidden"
        onPointerEnter={() => setFrameActive(true)}
        onPointerLeave={() => setFrameActive(false)}
        onFocusCapture={() => setFrameActive(true)}
        onBlurCapture={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget)) {
            setFrameActive(false);
          }
        }}
      >
        {props.children}
        <span
          className="listen-artwork-frame__rim pointer-events-none absolute inset-0 z-30"
          aria-hidden="true"
        />
      </div>
    </div>
  );
}

export function ListenModeTabs(props: {
  mode: ListenMode;
  compact: boolean;
  text: ReturnType<typeof getXiaText>;
  labels?: Partial<Record<ListenMode, string>>;
  order?: readonly ListenMode[];
  onChange: (mode: ListenMode) => void;
}) {
  const itemsByMode: Record<ListenMode, DreamSegmentSwitchItem<ListenMode>> = {
    hush: {
      value: "hush",
      label: props.labels?.hush ?? props.text.listen.hush,
      tooltip: props.text.listen.hushTooltip,
      icon: <Radio className="h-4 w-4" />,
    },
    muse: {
      value: "muse",
      label: props.labels?.muse ?? props.text.listen.muse,
      tooltip: props.text.listen.museTooltip,
      icon: <SimpleBrandIcon icon={siYoutubemusic} className="h-4 w-4" />,
    },
    linger: {
      value: "linger",
      label: props.labels?.linger ?? props.text.listen.linger,
      tooltip: props.text.listen.lingerTooltip,
      icon: <Disc3 className="h-4 w-4" />,
    },
  };
  const items: readonly DreamSegmentSwitchItem<ListenMode>[] = (
    props.order ?? ["hush", "muse", "linger"]
  ).map((mode) => itemsByMode[mode]);

  return (
    <DreamSegmentSwitch
      value={props.mode}
      items={items}
      compact={props.compact}
      ariaLabel={items
        .map((item) => item.tooltip ?? item.label)
        .join(" / ")}
      className="listen-mode-switch"
      onValueChange={props.onChange}
    />
  );
}

export function ListenLocalListControls(props: {
  text: ReturnType<typeof getXiaText>;
  refreshing: boolean;
  clearingMissing: boolean;
  onRefresh: () => void;
  onClearMissing: () => void;
}) {
  return (
    <div className="listen-local-list-controls listen-list-control-surface listen-list-control-surface-bottom pointer-events-auto inline-flex w-auto gap-1 p-1.5">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            shape="circle"
            aria-label={props.text.listen.localRefresh}
            title={props.text.listen.localRefresh}
            disabled={props.refreshing}
            data-active={props.refreshing ? "true" : "false"}
            className="listen-list-toolbar-button relative z-10 h-9 w-9"
            onClick={props.onRefresh}
          >
            <RefreshCw
              className={cn("h-4 w-4", props.refreshing ? "listen-loading-spinner" : "")}
            />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="top">
          {props.text.listen.localRefresh}
        </TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            shape="circle"
            aria-label={props.text.listen.localClearMissing}
            title={props.text.listen.localClearMissing}
            disabled={props.clearingMissing}
            data-active={props.clearingMissing ? "true" : "false"}
            className="listen-list-toolbar-button relative z-10 h-9 w-9"
            onClick={props.onClearMissing}
          >
            {props.clearingMissing ? (
              <Loader2 className="h-4 w-4 listen-loading-spinner" />
            ) : (
              <Trash2 className="h-4 w-4" />
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent side="top">
          {props.text.listen.localClearMissing}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

export function ListenConnectionPromptCard(props: {
  message: string;
  actionLabel: string;
  icon?: React.ReactNode;
  onAction: () => void;
}) {
  return (
    <div className="flex min-h-full items-center justify-center px-2 py-6">
      <div className="listen-connection-prompt listen-list-control-surface listen-list-control-surface-top relative w-full max-w-[17rem] px-5 py-6">
        <div className="relative flex flex-col items-center gap-4">
          <div className="listen-connection-prompt__icon flex h-12 w-12 items-center justify-center">
            {props.icon ?? <Link2 className="h-5 w-5" />}
          </div>
          <p className="listen-connection-prompt__message">{props.message}</p>
          <Button
            type="button"
            shape="capsule"
            className="listen-connection-prompt__action"
            onClick={props.onAction}
          >
            {props.actionLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function ListenTransportActions(props: {
  hasTrack: boolean;
  playing: boolean;
  loading?: boolean;
  previousDisabled?: boolean;
  nextDisabled?: boolean;
  playModeDisabled?: boolean;
  showQueueControls?: boolean;
  muted: boolean;
  volume: number;
  playMode: ListenPlayMode;
  text: ReturnType<typeof getXiaText>;
  onPrevious: () => void;
  onNext: () => void;
  onTogglePlayMode: () => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const playbackLabel = props.loading
    ? props.text.listen.loading
    : props.playing
      ? props.text.listen.pause
      : props.text.listen.play;
  const playModeLabel =
    props.playMode === "shuffle"
      ? props.text.listen.playModeShuffle
      : props.playMode === "repeat"
        ? props.text.listen.playModeRepeat
        : props.text.listen.playModeOrder;
  return (
    <div className="flex justify-center">
      <div className="listen-transport-actions listen-list-control-surface listen-list-control-surface-bottom inline-flex flex-wrap items-center justify-center gap-3 px-3 py-2.5">
        {props.showQueueControls === false ? null : (
          <>
            <ListenActionIconButton
              label={`${props.text.listen.playbackMode}: ${playModeLabel}`}
              className={cn(
                props.playMode !== "order" && "listen-action-icon-button--active",
              )}
              disabled={!props.hasTrack || props.playModeDisabled}
              onClick={props.onTogglePlayMode}
            >
              {props.playMode === "shuffle" ? (
                <Shuffle className="h-4 w-4" />
              ) : props.playMode === "repeat" ? (
                <Repeat2 className="h-4 w-4" />
              ) : (
                <ListMusic className="h-4 w-4" />
              )}
            </ListenActionIconButton>
            <ListenActionIconButton
              label={props.text.listen.previous}
              disabled={!props.hasTrack || props.previousDisabled}
              onClick={props.onPrevious}
            >
              <SkipBack className="h-4 w-4" />
            </ListenActionIconButton>
          </>
        )}
        <ListenActionIconButton
          label={playbackLabel}
          className="listen-primary-play-button listen-primary-play-button-hover h-12 w-12"
          disabled={!props.hasTrack || props.loading}
          onClick={props.onTogglePlayback}
        >
          {props.loading ? (
            <Loader2 className="h-5 w-5 listen-loading-spinner" />
          ) : props.playing ? (
            <Pause className="h-5 w-5" />
          ) : (
            <Play className="h-5 w-5 translate-x-px" />
          )}
        </ListenActionIconButton>
        {props.showQueueControls === false ? null : (
          <ListenActionIconButton
            label={props.text.listen.next}
            disabled={!props.hasTrack || props.nextDisabled}
            onClick={props.onNext}
          >
            <SkipForward className="h-4 w-4" />
          </ListenActionIconButton>
        )}
        <ListenVolumeControl
          hasTrack={props.hasTrack}
          muted={props.muted}
          volume={props.volume}
          text={props.text}
          onToggleMute={props.onToggleMute}
          onVolumeChange={props.onVolumeChange}
        />
      </div>
    </div>
  );
}

export function ListenProgressBar(props: {
  currentTime: number;
  duration: number;
  bufferedTime?: number;
  tone?: "default" | "light";
  className?: string;
  ariaLabel: string;
  onSeek?: (seconds: number) => void;
}) {
  const duration = Number.isFinite(props.duration)
    ? Math.max(0, props.duration)
    : 0;
  const currentTime = Number.isFinite(props.currentTime)
    ? Math.max(0, Math.min(props.currentTime, duration || props.currentTime))
    : 0;
  const bufferedTime = Number.isFinite(props.bufferedTime)
    ? Math.max(
        0,
        Math.min(props.bufferedTime ?? 0, duration || props.bufferedTime || 0),
      )
    : 0;
  const progress = duration > 0 ? Math.min(1, currentTime / duration) : 0;
  const bufferedProgress =
    duration > 0 ? Math.min(1, bufferedTime / duration) : 0;
  const lightTone = props.tone === "light";
  const canSeek = duration > 0 && Boolean(props.onSeek);

  return (
    <div
      className={cn("listen-progress-bar w-full max-w-2xl px-1", props.className)}
      data-tone={lightTone ? "light" : "default"}
    >
      <div
        className="listen-progress-bar__interaction relative mb-2 h-5"
        data-seekable={canSeek ? "true" : undefined}
      >
        <div
          className="listen-progress-bar__track absolute inset-x-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden"
        >
          <div
            className="listen-progress-bar__buffer absolute inset-y-0 left-0"
            style={{ width: `${bufferedProgress * 100}%` }}
          />
          <div
            className="listen-progress-bar__played absolute inset-y-0 left-0 h-full"
            style={{ width: `${progress * 100}%` }}
          />
        </div>
        {canSeek ? (
          <>
            <span
              aria-hidden="true"
              className="listen-progress-bar__thumb pointer-events-none absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2"
              style={{ left: `${progress * 100}%` }}
            />
            <input
              type="range"
              min={0}
              max={duration}
              step={0.1}
              value={currentTime}
              aria-label={props.ariaLabel}
              className="listen-progress-bar__input absolute inset-0 z-10 h-full w-full"
              onChange={(event) => {
                const nextTime = Number(event.currentTarget.value);
                if (Number.isFinite(nextTime)) {
                  props.onSeek?.(Math.max(0, Math.min(nextTime, duration)));
                }
              }}
            />
          </>
        ) : null}
      </div>
      <div
        className="listen-progress-bar__timestamps flex items-center justify-between"
      >
        <span>{formatProgressSeconds(currentTime)}</span>
        <span>{formatProgressSeconds(duration)}</span>
      </div>
    </div>
  );
}

export function ListenOnlineGroup(props: {
  title: string;
  hideTitle?: boolean;
  items: ListenOnlineItem[];
  selectedId: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  onPlayAll?: () => void;
  onShuffle?: () => void;
  onClear?: () => void;
  clearLabel?: string;
  onUndo?: () => void;
  undoDisabled?: boolean;
  onRedo?: () => void;
  redoDisabled?: boolean;
  canEdit?: (item: ListenOnlineItem) => boolean;
  onEdit?: (item: ListenOnlineItem) => void;
  editLabel?: string;
  canRemove?: (item: ListenOnlineItem) => boolean;
  onRemove?: (item: ListenOnlineItem) => void;
  removeLabel?: string;
  onMove?: (item: ListenOnlineItem, direction: -1 | 1) => void;
  liveStatuses?: Record<string, ListenLiveStatus>;
  onSelect: (item: ListenOnlineItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  const hasHeaderActions = Boolean(
    props.onPlayAll || props.onShuffle || props.onUndo || props.onRedo || props.onClear,
  );
  const headerTitle = props.hideTitle ? "" : props.title.trim();
  return (
    <div className="listen-online-group">
      {headerTitle || hasHeaderActions ? (
        <div className="wails-drag mb-2 flex min-h-7 items-center justify-between gap-2 px-2">
          <div className="listen-online-group__title min-w-0 truncate">
            {headerTitle}
          </div>
          {hasHeaderActions ? (
            <div className={cn("wails-no-drag", LISTEN_CONTROL_SURFACE_CLASS)}>
              {props.onPlayAll ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      shape="circle"
                      className={cn(
                        "h-7 w-7",
                        LISTEN_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.text.listen.playAll}
                      title={props.text.listen.playAll}
                      onClick={props.onPlayAll}
                    >
                      <Play className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.text.listen.playAll}
                  </TooltipContent>
                </Tooltip>
              ) : null}
              {props.onShuffle ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      shape="circle"
                      className={cn(
                        "h-7 w-7",
                        LISTEN_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.text.listen.shuffleAll}
                      title={props.text.listen.shuffleAll}
                      onClick={props.onShuffle}
                    >
                      <Shuffle className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.text.listen.shuffleAll}
                  </TooltipContent>
                </Tooltip>
              ) : null}
              {props.onUndo ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      shape="circle"
                      className={cn(
                        "h-7 w-7",
                        LISTEN_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.text.listen.undoQueue}
                      title={props.text.listen.undoQueue}
                      disabled={props.undoDisabled}
                      onClick={props.onUndo}
                    >
                      <Undo2 className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.text.listen.undoQueue}
                  </TooltipContent>
                </Tooltip>
              ) : null}
              {props.onRedo ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      shape="circle"
                      className={cn(
                        "h-7 w-7",
                        LISTEN_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.text.listen.redoQueue}
                      title={props.text.listen.redoQueue}
                      disabled={props.redoDisabled}
                      onClick={props.onRedo}
                    >
                      <Redo2 className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.text.listen.redoQueue}
                  </TooltipContent>
                </Tooltip>
              ) : null}
              {props.onClear ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      tone="destructive"
                      shape="circle"
                      className={cn(
                        "h-7 w-7",
                        LISTEN_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.clearLabel ?? props.text.listen.clearQueue}
                      title={props.clearLabel ?? props.text.listen.clearQueue}
                      onClick={props.onClear}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.clearLabel ?? props.text.listen.clearQueue}
                  </TooltipContent>
                </Tooltip>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}
      <SidebarMenu className="gap-1.5">
        {props.items.map((item) => {
          const selected = item.id === props.selectedId;
          const durationLabel = item.group === "live" ? "" : item.durationLabel;
          const songPlayCountLabel = item.playCountLabel?.trim() ?? "";
          const metadataParts = [
            item.channel,
            songPlayCountLabel,
            durationLabel,
          ].filter(Boolean);
          const canEdit = Boolean(props.onEdit && (!props.canEdit || props.canEdit(item)));
          const canRemove = Boolean(props.onRemove && (!props.canRemove || props.canRemove(item)));
          const canMove = Boolean(props.onMove);
          const hasRowActions = canEdit || canRemove || canMove;
          const visibleLiveStatus =
            item.group === "live"
              ? resolveVisibleListenLiveStatus(props.liveStatuses?.[item.videoId])
              : "";
          const showVideoIndicator =
            item.group !== "live" && hasListenMuseItemVideo(item);
          return (
            <SidebarMenuItem
              key={item.id}
              className={cn(hasRowActions && "flex items-center gap-1.5")}
            >
              <SidebarMenuButton
                type="button"
                isActive={selected}
                className={cn(
                  "min-h-16",
                  LISTEN_LIST_ITEM_BUTTON_CLASS,
                  hasRowActions && "min-w-0 flex-1",
                )}
                onClick={() => props.onSelect(item)}
              >
                <ListenAvatar
                  httpBaseURL={props.httpBaseURL}
                  item={item}
                  selected={selected}
                />
                <div className="min-w-0 flex-1">
                  <div className="listen-list-row__title flex min-w-0 items-center gap-1.5">
                    <span className="min-w-0 truncate">{item.title}</span>
                    {showVideoIndicator ? <ListenMuseVideoIndicator /> : null}
                  </div>
                  <div className="listen-list-row__metadata truncate">
                    {metadataParts.join(" · ")}
                  </div>
                  {item.description ? (
                    <div className="listen-list-row__description truncate">
                      {item.description}
                    </div>
                  ) : null}
                </div>
                {visibleLiveStatus ? (
                  <ListenLiveStatusBadge
                    status={visibleLiveStatus}
                    text={props.text}
                  />
                ) : null}
              </SidebarMenuButton>
              {canEdit ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex h-16 w-9 shrink-0 items-center justify-center self-center">
                      <Button
                        type="button"
                        variant="outline"
                        size="compactIcon"
                        shape="circle"
                        className={cn(
                          "h-8 w-8",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                        )}
                        aria-label={props.editLabel ?? props.text.listen.editChannel}
                        title={props.editLabel ?? props.text.listen.editChannel}
                        onClick={() => props.onEdit?.(item)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="left">
                    {props.editLabel ?? props.text.listen.editChannel}
                  </TooltipContent>
                </Tooltip>
              ) : null}
              {canRemove ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex h-16 w-9 shrink-0 items-center justify-center self-center">
                      <Button
                        type="button"
                        variant="outline"
                        size="compactIcon"
                        tone="destructive"
                        shape="circle"
                        className={cn(
                          "h-8 w-8",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                        )}
                        aria-label={
                          props.removeLabel ?? props.text.listen.removeFromQueue
                        }
                        title={
                          props.removeLabel ?? props.text.listen.removeFromQueue
                        }
                        onClick={() => props.onRemove?.(item)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="left">
                    {props.removeLabel ?? props.text.listen.removeFromQueue}
                  </TooltipContent>
                </Tooltip>
              ) : null}
              {canMove ? (
                <span className="flex h-16 w-9 shrink-0 flex-col items-center justify-center gap-1 self-center">
                  <ListenQueueMoveButton
                    label={props.text.listen.moveQueueItemUp}
                    disabled={props.items[0]?.id === item.id}
                    onClick={() => props.onMove?.(item, -1)}
                  >
                    <ChevronUp className="h-3.5 w-3.5" />
                  </ListenQueueMoveButton>
                  <ListenQueueMoveButton
                    label={props.text.listen.moveQueueItemDown}
                    disabled={props.items[props.items.length - 1]?.id === item.id}
                    onClick={() => props.onMove?.(item, 1)}
                  >
                    <ChevronDown className="h-3.5 w-3.5" />
                  </ListenQueueMoveButton>
                </span>
              ) : null}
            </SidebarMenuItem>
          );
        })}
      </SidebarMenu>
    </div>
  );
}

function ListenQueueMoveButton(props: {
  label: string;
  disabled?: boolean;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="compactIcon"
          shape="circle"
          className={cn("h-7 w-7", LISTEN_CONTROL_ICON_BUTTON_CLASS)}
          aria-label={props.label}
          title={props.label}
          disabled={props.disabled}
          onClick={props.onClick}
        >
          {props.children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="left">{props.label}</TooltipContent>
    </Tooltip>
  );
}

export function ListenHorizontalCardRow(props: {
  previousLabel: string;
  nextLabel: string;
  children: React.ReactNode;
}) {
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const [canScrollLeft, setCanScrollLeft] = React.useState(false);
  const [canScrollRight, setCanScrollRight] = React.useState(false);

  const updateScrollState = React.useCallback(() => {
    const element = scrollRef.current;
    if (!element) {
      setCanScrollLeft(false);
      setCanScrollRight(false);
      return;
    }
    const lastChild = element.lastElementChild;
    setCanScrollLeft(element.scrollLeft > 1);
    if (!lastChild) {
      setCanScrollRight(false);
      return;
    }
    const rowRect = element.getBoundingClientRect();
    const childRect = lastChild.getBoundingClientRect();
    setCanScrollRight(childRect.right - rowRect.right > 1);
  }, []);

  React.useLayoutEffect(() => {
    const element = scrollRef.current;
    if (!element || typeof window === "undefined") {
      return;
    }

    let frame = 0;
    const scheduleUpdate = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(updateScrollState);
    };
    const resizeObserver =
      typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(scheduleUpdate)
        : null;

    updateScrollState();
    element.addEventListener("scroll", scheduleUpdate, { passive: true });
    window.addEventListener("resize", scheduleUpdate);
    resizeObserver?.observe(element);
    Array.from(element.children).forEach((child) => resizeObserver?.observe(child));

    return () => {
      window.cancelAnimationFrame(frame);
      element.removeEventListener("scroll", scheduleUpdate);
      window.removeEventListener("resize", scheduleUpdate);
      resizeObserver?.disconnect();
    };
  }, [props.children, updateScrollState]);

  const scrollByPage = React.useCallback((direction: 1 | -1) => {
    const element = scrollRef.current;
    if (!element) {
      return;
    }
    element.scrollBy({
      left: direction * Math.max(element.clientWidth * 0.82, 160),
      behavior: "smooth",
    });
  }, []);

  return (
    <div className="listen-horizontal-card-row relative min-w-0 overflow-hidden">
      <div
        ref={scrollRef}
        className="flex min-w-0 snap-x snap-mandatory gap-4 overflow-x-auto overflow-y-hidden pb-1 pr-10 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
      >
        {props.children}
      </div>
      {canScrollLeft ? (
        <ListenHorizontalScrollButton
          side="left"
          label={props.previousLabel}
          onClick={() => scrollByPage(-1)}
        />
      ) : null}
      {canScrollRight ? (
        <ListenHorizontalScrollButton
          side="right"
          label={props.nextLabel}
          onClick={() => scrollByPage(1)}
        />
      ) : null}
    </div>
  );
}

function ListenHorizontalScrollButton(props: {
  side: "left" | "right";
  label: string;
  onClick: () => void;
}) {
  const isLeft = props.side === "left";
  return (
    <div
      className={cn(
        "listen-horizontal-scroll-fade absolute top-0 z-30 h-[10rem] w-20",
        isLeft ? "left-0" : "right-0",
      )}
      data-side={props.side}
      onPointerDown={(event) => event.stopPropagation()}
    >
      <button
        type="button"
        className={cn(
          "listen-horizontal-scroll-button flex h-full w-full items-center",
          isLeft ? "justify-start pl-1.5" : "justify-end pr-1.5",
        )}
        aria-label={props.label}
        title={props.label}
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          props.onClick();
        }}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <span className="app-listen-horizontal-scroll-control flex h-10 w-10 items-center justify-center">
          {isLeft ? (
            <ChevronLeft className="h-4 w-4" />
          ) : (
            <ChevronRight className="h-4 w-4" />
          )}
        </span>
      </button>
    </div>
  );
}

export function ListenMuseTrackGroup(props: {
  title: string;
  hideTitle?: boolean;
  items: ListenOnlineItem[];
  selectedId: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  onPlayAll?: () => void;
  onShuffle?: () => void;
  onSelect: (item: ListenOnlineItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <ListenMuseGroupFrame
      title={props.title}
      hideTitle={props.hideTitle}
      text={props.text}
      onPlayAll={props.onPlayAll}
      onShuffle={props.onShuffle}
    >
      <ListenHorizontalCardRow
        previousLabel={props.text.listen.previous}
        nextLabel={props.text.listen.next}
      >
        {props.items.map((item) => (
          <ListenMuseTrackCard
            key={item.id}
            item={item}
            selected={item.id === props.selectedId}
            httpBaseURL={props.httpBaseURL}
            onSelect={() => props.onSelect(item)}
          />
        ))}
      </ListenHorizontalCardRow>
    </ListenMuseGroupFrame>
  );
}

export function ListenMusePlaylistGroup(props: {
  title: string;
  items: ListenPlaylistItem[];
  selectedPlaylistId: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  onSelect: (item: ListenPlaylistItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <ListenMuseGroupFrame title={props.title} text={props.text}>
      <ListenHorizontalCardRow
        previousLabel={props.text.listen.previous}
        nextLabel={props.text.listen.next}
      >
        {props.items.map((item) => (
          <ListenMusePlaylistCard
            key={item.id}
            item={item}
            selected={item.playlistId === props.selectedPlaylistId}
            httpBaseURL={props.httpBaseURL}
            onSelect={() => props.onSelect(item)}
          />
        ))}
      </ListenHorizontalCardRow>
    </ListenMuseGroupFrame>
  );
}

export function ListenMuseArtistGroup(props: {
  title: string;
  items: ListenArtistItem[];
  selectedArtistId?: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  onSelect: (item: ListenArtistItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <ListenMuseGroupFrame title={props.title} text={props.text}>
      <ListenHorizontalCardRow
        previousLabel={props.text.listen.previous}
        nextLabel={props.text.listen.next}
      >
        {props.items.map((item) => (
          <ListenMuseArtistCard
            key={item.id}
            item={item}
            selected={item.browseId === props.selectedArtistId}
            httpBaseURL={props.httpBaseURL}
            onSelect={() => props.onSelect(item)}
          />
        ))}
      </ListenHorizontalCardRow>
    </ListenMuseGroupFrame>
  );
}

export function ListenMuseCategoryGroup(props: {
  title: string;
  items: ListenCategoryItem[];
  selectedCategoryId?: string;
  text: ReturnType<typeof getXiaText>;
  onSelect: (item: ListenCategoryItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <ListenMuseGroupFrame title={props.title} text={props.text}>
      <ListenHorizontalCardRow
        previousLabel={props.text.listen.previous}
        nextLabel={props.text.listen.next}
      >
        {props.items.map((item) => (
          <ListenMuseCategoryCard
            key={item.id}
            item={item}
            selected={item.id === props.selectedCategoryId}
            onSelect={() => props.onSelect(item)}
          />
        ))}
      </ListenHorizontalCardRow>
    </ListenMuseGroupFrame>
  );
}

export function ListenMuseTrackList(props: {
  items: ListenOnlineItem[];
  selectedId: string;
  httpBaseURL: string;
  artistFallback?: string;
  layout?: "default" | "album";
  onSelect: (item: ListenOnlineItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <div className="listen-track-list space-y-1.5">
      {props.items.map((item, index) => {
        const selected = item.id === props.selectedId;
        const artistLabel = resolveListenMuseTrackListArtist(
          item,
          props.artistFallback,
        );
        if (props.layout === "album") {
          return (
            <button
              key={item.id}
              type="button"
              className={cn(
                "listen-track-list-row grid min-h-14 w-full grid-cols-[2rem_minmax(0,1fr)_3.25rem] items-center gap-2 px-2 py-2",
              )}
              data-selected={selected ? "true" : undefined}
              onClick={() => props.onSelect(item)}
            >
              <span
                className={cn(
                  "listen-track-list-row__index flex h-7 w-7 shrink-0 items-center justify-center",
                )}
              >
                {index + 1}
              </span>
              <span className="flex min-w-0 items-center gap-2">
                <ListenMuseListArtwork
                  item={item}
                  selected={selected}
                  httpBaseURL={props.httpBaseURL}
                />
                <span className="min-w-0 flex-1">
                  <ListenMuseTrackTitle item={item} />
                  <span className="listen-track-list-row__secondary block truncate">
                    {artistLabel}
                  </span>
                </span>
              </span>
              <span className="listen-track-list-row__duration justify-self-end">
                {item.durationLabel}
              </span>
            </button>
          );
        }
        const albumLabel = resolveListenMuseTrackListAlbum(item, artistLabel);
        return (
          <button
            key={item.id}
            type="button"
            className={cn(
              "listen-track-list-row grid min-h-14 w-full grid-cols-[minmax(0,1.45fr)_minmax(0,0.82fr)_minmax(0,0.92fr)_3.25rem] items-center gap-2 px-2 py-2",
            )}
            data-selected={selected ? "true" : undefined}
            onClick={() => props.onSelect(item)}
          >
            <span className="flex min-w-0 items-center gap-2">
              <ListenMuseListArtwork
                item={item}
                selected={selected}
                httpBaseURL={props.httpBaseURL}
              />
              <ListenMuseTrackTitle item={item} />
            </span>
            <span className="listen-track-list-row__secondary min-w-0 truncate">
              {artistLabel}
            </span>
            <span className="listen-track-list-row__tertiary min-w-0 truncate">
              {albumLabel}
            </span>
            <span className="listen-track-list-row__duration justify-self-end">
              {item.durationLabel}
            </span>
          </button>
        );
      })}
    </div>
  );
}

export function ListenMuseTrackListGroup(props: {
  title: string;
  items: ListenOnlineItem[];
  selectedId: string;
  httpBaseURL: string;
  artistFallback?: string;
  maxItems?: number;
  text: ReturnType<typeof getXiaText>;
  onPlayAll?: () => void;
  onShuffle?: () => void;
  onSeeAll?: () => void;
  onSelect: (item: ListenOnlineItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  const visibleItems =
    props.maxItems && props.maxItems > 0
      ? props.items.slice(0, props.maxItems)
      : props.items;
  const hasHeaderActions = Boolean(
    props.onPlayAll || props.onShuffle || props.onSeeAll,
  );
  return (
    <section className="listen-muse-track-list-group min-w-0 !mt-7 space-y-3 overflow-hidden first:!mt-0">
      <div className="wails-drag flex min-h-7 items-center justify-between gap-2 px-2">
        <div className="listen-muse-group__title min-w-0 truncate">
          {props.title}
        </div>
        {hasHeaderActions ? (
          <div className="wails-no-drag flex shrink-0 items-center gap-1">
            {props.onPlayAll ? (
              <ListenMuseHeaderIconButton
                label={props.text.listen.playAll}
                onClick={props.onPlayAll}
              >
                <Play className="h-3.5 w-3.5" />
              </ListenMuseHeaderIconButton>
            ) : null}
            {props.onShuffle ? (
              <ListenMuseHeaderIconButton
                label={props.text.listen.shuffleAll}
                onClick={props.onShuffle}
              >
                <Shuffle className="h-3.5 w-3.5" />
              </ListenMuseHeaderIconButton>
            ) : null}
            {props.onSeeAll ? (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                shape="capsule"
                className="listen-muse-see-all shrink-0 px-2 py-1"
                onClick={props.onSeeAll}
              >
                {props.text.listen.seeAll}
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
      <ListenMuseTrackList
        items={visibleItems}
        selectedId={props.selectedId}
        httpBaseURL={props.httpBaseURL}
        artistFallback={props.artistFallback}
        onSelect={props.onSelect}
      />
    </section>
  );
}

export function hasListenMuseItemVideo(item: ListenOnlineItem) {
  return (
    hasListenMusicVideoContent(item.musicVideoType) ||
    (!isListenMusicVideoKnownNoVideo(item.musicVideoType) &&
      doesListenThumbnailSuggestVideoContent(item.videoId, item.thumbnailUrl))
  );
}

export function ListenMuseVideoIndicator(props: { className?: string } = {}) {
  return (
    <Video
      aria-hidden="true"
      className={cn(
        "listen-muse-video-indicator h-3.5 w-3.5 shrink-0",
        props.className,
      )}
      strokeWidth={1.8}
    />
  );
}

function ListenMuseTrackTitle(props: { item: ListenOnlineItem }) {
  const hasVideo = hasListenMuseItemVideo(props.item);
  return (
    <span className="flex min-w-0 flex-1 items-center gap-1.5">
      <span className="listen-track-list-row__title min-w-0 truncate">
        {props.item.title}
      </span>
      {hasVideo ? <ListenMuseVideoIndicator /> : null}
    </span>
  );
}

function resolveListenMuseTrackListArtist(
  item: ListenOnlineItem,
  fallback?: string,
) {
  return resolveTrustedListenOnlineArtistLabel(item, fallback);
}

function resolveListenMuseTrackListAlbum(
  item: ListenOnlineItem,
  artistLabel: string,
) {
  const album = item.description.trim();
  if (!album || album === artistLabel || album === item.channel.trim()) {
    return "";
  }
  return album;
}

function ListenMuseGroupFrame(props: {
  title: string;
  hideTitle?: boolean;
  text: ReturnType<typeof getXiaText>;
  children: React.ReactNode;
  onPlayAll?: () => void;
  onShuffle?: () => void;
}) {
  const headerTitle = props.hideTitle ? "" : props.title.trim();
  const hasActions = Boolean(props.onPlayAll || props.onShuffle);
  return (
    <section className="listen-muse-group-frame min-w-0 !mt-7 space-y-3 overflow-hidden first:!mt-0">
      {headerTitle || hasActions ? (
        <div className="wails-drag flex min-h-7 items-center justify-between gap-2 px-2">
          <div className="listen-muse-group__title min-w-0 truncate">
            {headerTitle}
          </div>
          {hasActions ? (
            <div className="app-dream-button-group app-completed-toolbar-actions wails-no-drag inline-flex h-9 shrink-0 items-center p-0.5">
              {props.onPlayAll ? (
                <ListenMuseHeaderIconButton
                  label={props.text.listen.playAll}
                  onClick={props.onPlayAll}
                >
                  <Play className="h-3.5 w-3.5" />
                </ListenMuseHeaderIconButton>
              ) : null}
              {props.onShuffle ? (
                <ListenMuseHeaderIconButton
                  label={props.text.listen.shuffleAll}
                  onClick={props.onShuffle}
                >
                  <Shuffle className="h-3.5 w-3.5" />
                </ListenMuseHeaderIconButton>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}
      {props.children}
    </section>
  );
}

function ListenMuseHeaderIconButton(props: {
  label: string;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="app-completed-toolbar-button h-8 w-8 p-0"
          aria-label={props.label}
          title={props.label}
          onClick={props.onClick}
        >
          {props.children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{props.label}</TooltipContent>
    </Tooltip>
  );
}

function ListenMuseTrackCard(props: {
  item: ListenOnlineItem;
  selected: boolean;
  httpBaseURL: string;
  onSelect: () => void;
}) {
  const hasVideo = hasListenMuseItemVideo(props.item);
  return (
    <div
      className="listen-muse-card group/muse-card relative w-[10rem] shrink-0 snap-start"
      data-selected={props.selected ? "true" : undefined}
    >
      <button
        type="button"
        className="listen-muse-card__artwork-button block w-full"
        onClick={props.onSelect}
      >
        <ListenMuseCardArtwork
          httpBaseURL={props.httpBaseURL}
          item={props.item}
          selected={props.selected}
          liftOnHover={false}
          softenOnHover
        >
          <span className="listen-muse-cover-overlay listen-playback-hover-layer pointer-events-none absolute inset-0 z-10 flex items-center justify-center">
            <span className="listen-playback-hover-button listen-primary-play-button flex h-10 w-10 items-center justify-center">
              <Play className="listen-playback-hover-icon ml-0.5 h-4 w-4" />
            </span>
          </span>
        </ListenMuseCardArtwork>
      </button>
      <button
        type="button"
        className="listen-muse-card__identity-button block w-full"
        onClick={props.onSelect}
      >
        <ListenMuseCardText
          title={props.item.title}
          subtitle={resolveListenMuseTrackCardSubtitle(props.item)}
          hasVideo={hasVideo}
        />
      </button>
    </div>
  );
}

function ListenMusePlaylistCard(props: {
  item: ListenPlaylistItem;
  selected: boolean;
  httpBaseURL: string;
  onSelect: () => void;
}) {
  return (
    <div
      className="listen-muse-card group/muse-card relative w-[10rem] shrink-0 snap-start"
      data-selected={props.selected ? "true" : undefined}
    >
      <button
        type="button"
        className="listen-muse-card__artwork-button block w-full"
        onClick={props.onSelect}
      >
        <ListenMuseCardArtwork
          httpBaseURL={props.httpBaseURL}
          item={{
            title: props.item.title,
            channel: props.item.channel,
            thumbnailUrl: props.item.thumbnailUrl,
          }}
          selected={props.selected}
        />
        <ListenMuseCardText title={props.item.title} subtitle="" />
      </button>
    </div>
  );
}

function ListenMuseArtistCard(props: {
  item: ListenArtistItem;
  selected: boolean;
  httpBaseURL: string;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      className="listen-muse-card group/muse-card block w-[10rem] shrink-0 snap-start"
      data-card-justification="middle"
      data-selected={props.selected ? "true" : undefined}
      onClick={props.onSelect}
    >
      <ListenMuseCardArtwork
        httpBaseURL={props.httpBaseURL}
        item={{
          title: props.item.name,
          channel: props.item.subtitle || "YouTube Music",
          thumbnailUrl: props.item.thumbnailUrl,
        }}
        selected={props.selected}
        shape="circle"
      />
      <ListenMuseCardText
        title={props.item.name}
        subtitle={props.item.subtitle || "YouTube Music"}
        align="center"
      />
    </button>
  );
}

function ListenMuseCategoryCard(props: {
  item: ListenCategoryItem;
  selected: boolean;
  onSelect: () => void;
}) {
  const swatch =
    props.item.colorHex && /^#[0-9a-fA-F]{6}$/.test(props.item.colorHex)
      ? props.item.colorHex
      : "";
  return (
    <button
      type="button"
      className="listen-muse-card group/muse-card block w-[10rem] shrink-0 snap-start"
      data-selected={props.selected ? "true" : undefined}
      onClick={props.onSelect}
    >
      <span
        className="listen-muse-card-artwork listen-muse-category-artwork relative flex aspect-square w-full items-center justify-center overflow-hidden"
        data-category-color={swatch ? "true" : undefined}
        data-selected={props.selected ? "true" : undefined}
        data-lift="true"
        style={
          swatch
            ? ({ "--listen-category-color": swatch } as React.CSSProperties)
            : undefined
        }
      >
        {swatch ? (
          <span className="listen-category-color-strip absolute inset-x-0 bottom-0 h-1" />
        ) : null}
        <Tags className="h-7 w-7" />
      </span>
      <ListenMuseCardText title={props.item.title} subtitle="" />
    </button>
  );
}

function ListenMuseCardArtwork(props: {
  httpBaseURL: string;
  item: { title: string; channel: string; thumbnailUrl?: string; videoId?: string };
  selected: boolean;
  shape?: ListenArtworkShape;
  liftOnHover?: boolean;
  softenOnHover?: boolean;
  children?: React.ReactNode;
}) {
  const candidates = React.useMemo(
    () =>
      props.shape === "circle"
        ? buildListenImageCandidates(
            props.httpBaseURL,
            props.item.thumbnailUrl ?? "",
          )
        : buildListenPosterCandidates(props.httpBaseURL, {
            videoId: props.item.videoId,
            thumbnailUrl: props.item.thumbnailUrl,
          }),
    [props.httpBaseURL, props.item.thumbnailUrl, props.item.videoId, props.shape],
  );
  const liftOnHover = props.liftOnHover !== false;
  const isCircle = props.shape === "circle";

  return (
    <span
      className="listen-muse-card-artwork relative block aspect-square w-full overflow-hidden"
      data-shape={isCircle ? "circle" : undefined}
      data-lift={liftOnHover ? "true" : "false"}
      data-selected={props.selected ? "true" : undefined}
    >
      {candidates.length > 0 ? (
        <ListenCoverArtwork
          alt=""
          candidates={candidates}
          className="h-full w-full"
          imageClassName="listen-muse-card-artwork__image"
          loading="lazy"
          softenOnHover={props.softenOnHover}
        />
      ) : (
        <span
          aria-hidden="true"
          className={cn(
            "listen-muse-card-artwork__placeholder flex h-full w-full items-center justify-center",
          )}
          data-shape={isCircle ? "circle" : undefined}
        >
          {isCircle ? <UserRound className="h-7 w-7" /> : null}
        </span>
      )}
      {props.children}
    </span>
  );
}

function ListenMuseCardText(props: {
  title: string;
  subtitle: string;
  hasVideo?: boolean;
  align?: "start" | "center";
}) {
  const centered = props.align === "center";
  return (
    <span
      className="listen-muse-card-text block min-w-0 px-0.5 pt-2"
      data-listen-card-align={centered ? "center" : "start"}
    >
      <span
        className={cn(
          "flex min-w-0 items-center gap-1.5",
          centered && "justify-center",
        )}
      >
        <span
          className={cn(
            "listen-muse-card-text__title min-w-0 truncate",
            centered && "w-full",
          )}
        >
          {props.title}
        </span>
        {props.hasVideo ? <ListenMuseVideoIndicator /> : null}
      </span>
      {props.subtitle ? (
        <span
          className={cn(
            "listen-muse-card-text__subtitle block truncate",
            centered && "w-full",
          )}
        >
          {props.subtitle}
        </span>
      ) : null}
    </span>
  );
}

function resolveListenMuseTrackCardSubtitle(item: ListenOnlineItem) {
  return resolveTrustedListenOnlineArtistLabel(item) || item.title.trim();
}

function ListenMuseListArtwork(props: {
  httpBaseURL: string;
  item: ListenOnlineItem;
  selected: boolean;
}) {
  const candidates = React.useMemo(
    () => buildListenPosterCandidates(props.httpBaseURL, props.item),
    [props.httpBaseURL, props.item.thumbnailUrl, props.item.videoId],
  );

  return (
    <span
      className="listen-muse-list-artwork relative flex h-10 w-10 shrink-0 overflow-hidden"
      data-selected={props.selected ? "true" : undefined}
    >
      <ListenCoverArtwork
        alt=""
        candidates={candidates}
        className="h-full w-full"
        loading="lazy"
      />
    </span>
  );
}

function resolveVisibleListenLiveStatus(status: ListenLiveStatus | undefined): ListenLiveStatusValue | "" {
  if (!status) {
    return "";
  }
  return status.status === "offline" ||
    status.status === "upcoming" ||
    status.status === "unavailable"
    ? status.status
    : "";
}

function ListenLiveStatusBadge(props: {
  status: ListenLiveStatusValue;
  text: ReturnType<typeof getXiaText>;
}) {
  const live = props.status === "live";
  const checking = props.status === "checking";
  const upcoming = props.status === "upcoming";
  const tone = live
    ? "danger"
    : checking
      ? "busy"
      : upcoming
        ? "warning"
        : props.status === "unavailable"
          ? "danger"
          : "muted";
  const label =
    props.status === "live"
      ? props.text.listen.liveStatusLive
      : props.status === "offline"
        ? props.text.listen.liveStatusOffline
        : props.status === "upcoming"
          ? props.text.listen.liveStatusUpcoming
          : props.status === "unavailable"
            ? props.text.listen.liveStatusUnavailable
            : props.status === "checking"
              ? props.text.listen.liveStatusChecking
              : props.text.listen.liveStatusUnknown;
  return (
    <StatusBadge
      data-status={props.status}
      tone={tone}
      className="listen-live-status-badge shrink-0"
      icon={checking ? <Loader2 /> : <Radio />}
    >
      {label}
    </StatusBadge>
  );
}

export function ListenPlaylistGroup(props: {
  title: string;
  items: ListenPlaylistItem[];
  selectedPlaylistId: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  savedPlaylistIds: Set<string>;
  playlistMutationAction: ListenPlaylistLibraryAction | null;
  playlistMutationPlaylistId: string;
  onSelect: (item: ListenPlaylistItem) => void;
  onToggleLibrary?: (
    item: ListenPlaylistItem,
    action: ListenPlaylistLibraryAction,
  ) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <div className="listen-playlist-group">
      <div className={LISTEN_LIST_SECTION_TITLE_CLASS}>
        {props.title}
      </div>
      <SidebarMenu className="gap-1.5">
        {props.items.map((item) => {
          const selected = item.playlistId === props.selectedPlaylistId;
          const isSaved = props.savedPlaylistIds.has(item.playlistId);
          const isMutating =
            props.playlistMutationPlaylistId === item.playlistId;
          const actionLabel = props.text.listen.savePlaylist;
          return (
            <SidebarMenuItem
              key={item.id}
              className="flex items-center gap-1.5"
            >
              <SidebarMenuButton
                type="button"
                isActive={selected}
                className={cn(
                  "min-h-16 min-w-0 flex-1",
                  LISTEN_LIST_ITEM_BUTTON_CLASS,
                )}
                onClick={() => props.onSelect(item)}
              >
                <ListenAvatar
                  httpBaseURL={props.httpBaseURL}
                  item={item}
                  selected={selected}
                />
                <div className="min-w-0 flex-1">
                  <div className="listen-list-row__title truncate">
                    {item.title}
                  </div>
                  <div className="listen-list-row__metadata truncate">
                    {item.channel}
                  </div>
                  {item.description ? (
                    <div className="listen-list-row__description truncate">
                      {item.description}
                    </div>
                  ) : null}
                </div>
              </SidebarMenuButton>
              {props.onToggleLibrary && !isSaved ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex h-16 w-10 shrink-0 items-center justify-center self-center">
                      <Button
                        type="button"
                        variant="outline"
                        size="compactIcon"
                        shape="circle"
                        disabled={isMutating}
                        className={cn(
                          "listen-playlist-library-button h-8 w-8 shrink-0",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                        )}
                        aria-label={actionLabel}
                        title={actionLabel}
                        onClick={() => props.onToggleLibrary?.(item, "add")}
                      >
                        {isMutating ? (
                          <Loader2 className="h-4 w-4 listen-loading-spinner" />
                        ) : (
                          <Plus className="h-4 w-4" />
                        )}
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="left">
                    {isMutating ? props.text.listen.savePlaylist : actionLabel}
                  </TooltipContent>
                </Tooltip>
              ) : null}
            </SidebarMenuItem>
          );
        })}
      </SidebarMenu>
    </div>
  );
}

export function ListenCategoryGroup(props: {
  title: string;
  items: ListenCategoryItem[];
  selectedCategoryId?: string;
  onSelect: (item: ListenCategoryItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <div className="listen-category-group">
      <div className={LISTEN_LIST_SECTION_TITLE_CLASS}>
        {props.title}
      </div>
      <SidebarMenu className="gap-1.5">
        {props.items.map((item) => {
          const selected = item.id === props.selectedCategoryId;
          const swatch =
            item.colorHex && /^#[0-9a-fA-F]{6}$/.test(item.colorHex)
              ? item.colorHex
              : "";
          return (
            <SidebarMenuItem key={item.id}>
              <SidebarMenuButton
                type="button"
                isActive={selected}
                className={cn("min-h-14", LISTEN_LIST_ITEM_BUTTON_CLASS)}
                onClick={() => props.onSelect(item)}
              >
                <span
                  className="listen-category-swatch relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden"
                  data-category-color={swatch ? "true" : undefined}
                  style={
                    swatch
                      ? ({ "--listen-category-color": swatch } as React.CSSProperties)
                      : undefined
                  }
                >
                  <span className="listen-category-color-strip absolute left-0 top-0 h-full w-1" />
                  <Tags className="relative h-4 w-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="listen-list-row__title truncate">
                    {item.title}
                  </div>
                </div>
              </SidebarMenuButton>
            </SidebarMenuItem>
          );
        })}
      </SidebarMenu>
    </div>
  );
}

export function ListenArtistGroup(props: {
  title: string;
  items: ListenArtistItem[];
  selectedArtistId?: string;
  httpBaseURL: string;
  onSelect: (item: ListenArtistItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <div className="listen-artist-group">
      <div className={LISTEN_LIST_SECTION_TITLE_CLASS}>
        {props.title}
      </div>
      <SidebarMenu className="gap-1.5">
        {props.items.map((item) => {
          const selected = item.browseId === props.selectedArtistId;
          return (
            <SidebarMenuItem key={item.id}>
              <SidebarMenuButton
                type="button"
                isActive={selected}
                className={cn("min-h-16", LISTEN_LIST_ITEM_BUTTON_CLASS)}
                onClick={() => props.onSelect(item)}
              >
                <ListenAvatar
                  httpBaseURL={props.httpBaseURL}
                  item={{
                    channel: item.name,
                    thumbnailUrl: item.thumbnailUrl,
                  }}
                  selected={selected}
                  shape="circle"
                />
                <div className="min-w-0 flex-1">
                  <div className="listen-list-row__title truncate">
                    {item.name}
                  </div>
                  <div className="listen-list-row__metadata truncate">
                    {item.subtitle || "YouTube Music"}
                  </div>
                </div>
                <UserRound className="listen-list-row__trailing-icon h-4 w-4 shrink-0" />
              </SidebarMenuButton>
            </SidebarMenuItem>
          );
        })}
      </SidebarMenu>
    </div>
  );
}
