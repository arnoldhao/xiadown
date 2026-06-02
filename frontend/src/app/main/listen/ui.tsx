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
import { buildListenAvatarImageCandidates,buildListenImageCandidates,buildListenPosterCandidates,buildListenTrackThumbnailCandidates } from "@/app/main/listen/storage";
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
      className={cn(
        "relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden bg-sidebar-background/68 text-xs font-semibold text-sidebar-foreground shadow-[inset_0_1px_0_hsl(var(--background)/0.20)] ring-1 ring-[hsl(var(--foreground)/0.08)]",
        props.shape === "circle" ? "rounded-full" : "rounded-2xl",
        props.selected && "ring-sidebar-primary/26",
      )}
    >
      <span className="absolute inset-0 bg-[radial-gradient(circle_at_30%_20%,hsl(var(--primary)/0.30),transparent_58%),linear-gradient(135deg,hsl(var(--muted)),hsl(var(--background)))]" />
      {!imageReady ? (
        <span className="pointer-events-none relative z-0 text-sidebar-foreground/58">
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
            "absolute inset-0 z-10 h-full w-full object-cover transition-opacity duration-150",
            imageReady ? "opacity-100" : "opacity-0",
          )}
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
  const [failedURLs, setFailedURLs] = React.useState<Set<string>>(
    () => new Set(),
  );
  const source =
    [coverURL, LISTEN_DEFAULT_COVER_IMAGE_URL]
      .filter(Boolean)
      .find((url) => !failedURLs.has(url)) ?? "";

  React.useEffect(() => {
    setFailedURLs(new Set());
  }, [props.track.coverURL]);

  return (
    <div
      className={cn(
        "flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-sidebar-background/68 text-sidebar-foreground/75 shadow-[inset_0_1px_0_hsl(var(--background)/0.20)] ring-1 ring-[hsl(var(--foreground)/0.08)]",
        props.className,
      )}
    >
      {source ? (
        <img
          src={source}
          alt=""
          className="h-full w-full object-cover"
          loading="lazy"
          onError={() =>
            setFailedURLs((current) => {
              const next = new Set(current);
              next.add(source);
              return next;
            })
          }
        />
      ) : (
        <Music2 className="h-5 w-5" />
      )}
    </div>
  );
}

export function useListenStableImageSource(srcCandidates: string[]) {
  const candidateKey = srcCandidates.join("\n");
  const candidates = React.useMemo(() => {
    const normalized = srcCandidates.map((url) => url.trim()).filter(Boolean);
    return Array.from(new Set([...normalized, LISTEN_DEFAULT_COVER_IMAGE_URL]));
  }, [candidateKey]);
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const [visibleSrc, setVisibleSrc] = React.useState(
    LISTEN_DEFAULT_COVER_IMAGE_URL,
  );
  const activeSrc =
    candidates[Math.min(candidateIndex, Math.max(candidates.length - 1, 0))] ||
    LISTEN_DEFAULT_COVER_IMAGE_URL;
  const activeSrcRef = React.useRef(activeSrc);

  React.useEffect(() => {
    activeSrcRef.current = activeSrc;
  }, [activeSrc]);

  React.useEffect(() => {
    setCandidateIndex(0);
  }, [candidateKey]);

  React.useEffect(() => {
    const source = activeSrc.trim() || LISTEN_DEFAULT_COVER_IMAGE_URL;
    if (source === visibleSrc) {
      return;
    }
    let disposed = false;
    const commitSource = () => {
      if (!disposed && activeSrcRef.current === source) {
        setVisibleSrc(source);
      }
    };
    const advanceSource = () => {
      if (disposed || activeSrcRef.current !== source) {
        return;
      }
      setCandidateIndex((current) =>
        current + 1 < candidates.length ? current + 1 : current,
      );
    };

    if (typeof window === "undefined" || typeof window.Image === "undefined") {
      commitSource();
      return () => {
        disposed = true;
      };
    }

    const image = new window.Image();
    image.decoding = "async";
    image.loading = "eager";
    image.onload = commitSource;
    image.onerror = advanceSource;
    image.src = source;
    if (image.complete && image.naturalWidth > 0) {
      commitSource();
    }

    return () => {
      disposed = true;
      image.onload = null;
      image.onerror = null;
    };
  }, [activeSrc, candidates.length, visibleSrc]);

  return {
    activeSrc,
    visibleSrc,
    imageReady: activeSrc === visibleSrc,
  };
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
  const {
    activeSrc: activePoster,
    visibleSrc: visiblePoster,
    imageReady: posterReady,
  } = useListenStableImageSource(posterCandidates);

  return (
    <ListenArtworkShell
      className={props.className}
      visualizer={props.visualizer}
      visualizerVisible={props.visualizerVisible}
    >
      <>
        <img
          key={visiblePoster}
          src={visiblePoster}
          alt={props.track.title}
          className="block h-full w-full object-cover transition-transform duration-500 ease-out"
          loading="eager"
        />
        {posterReady ? (
          <span
            key={`cover-sweep-${activePoster}`}
            className="listen-cover-change-sweep"
            aria-hidden="true"
          />
        ) : null}
        <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(15,23,42,0.02),rgba(15,23,42,0.12))]" />
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
            className={cn(
              "h-10 w-10 rounded-full",
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
      <div className="group/volume flex items-center rounded-full">
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="compactIcon"
            className={cn(
              "h-10 w-10 rounded-full",
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
          className={cn(
            "ml-0 block w-0 overflow-hidden opacity-0 transition-[margin,width,opacity] duration-150 ease-out",
            "group-hover/volume:ml-2 group-hover/volume:w-20 group-hover/volume:opacity-100",
            "group-focus-within/volume:ml-2 group-focus-within/volume:w-20 group-focus-within/volume:opacity-100",
          )}
        >
          <span
            className={cn(
              "relative flex h-6 w-20 items-center",
              !props.hasTrack && "opacity-40",
            )}
          >
            <span className="pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden rounded-full bg-sidebar-foreground/10">
              <span
                className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary"
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
              className="relative z-10 h-6 w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
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
        "listen-artwork-shell relative isolate w-full shrink-0 overflow-visible transition-[padding] duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)] animate-in fade-in-0 zoom-in-95",
        props.className,
      )}
    >
      <div
        className={cn(
          "listen-artwork-shadow absolute inset-0 z-0 translate-y-5 rounded-[2rem] bg-black/14 blur-3xl transition-[transform,opacity] duration-300 ease-out",
          "opacity-100",
        )}
      />
      {props.visualizer}
      <div
        className="listen-artwork-frame relative z-10 aspect-square overflow-hidden rounded-[2rem] bg-white shadow-[0_28px_90px_-42px_rgba(15,23,42,0.45)] transition-[transform,box-shadow] duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)]"
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
          className="pointer-events-none absolute inset-0 z-30 rounded-[2rem] border border-white/50"
          aria-hidden="true"
        />
      </div>
    </div>
  );
}

export function ListenSourceBadge(props: {
  mode: ListenMode;
  text: ReturnType<typeof getXiaText>;
}) {
  if (props.mode === "hush") {
    return (
      <>
        <Radio className="h-3 w-3" />
        {props.text.listen.hush}
      </>
    );
  }
  if (props.mode === "muse") {
    return (
      <>
        <SimpleBrandIcon icon={siYoutubemusic} className="h-3 w-3" />
        {props.text.listen.muse}
      </>
    );
  }
  return (
    <>
      <Disc3 className="h-3 w-3" />
      {props.text.listen.linger}
    </>
  );
}

export function ListenModeTabs(props: {
  mode: ListenMode;
  compact: boolean;
  text: ReturnType<typeof getXiaText>;
  onChange: (mode: ListenMode) => void;
}) {
  const items: readonly DreamSegmentSwitchItem<ListenMode>[] = [
    {
      value: "hush",
      label: props.text.listen.hush,
      tooltip: props.text.listen.hushTooltip,
      icon: <Radio className="h-4 w-4" />,
    },
    {
      value: "muse",
      label: props.text.listen.muse,
      tooltip: props.text.listen.museTooltip,
      icon: <SimpleBrandIcon icon={siYoutubemusic} className="h-4 w-4" />,
    },
    {
      value: "linger",
      label: props.text.listen.linger,
      tooltip: props.text.listen.lingerTooltip,
      icon: <Disc3 className="h-4 w-4" />,
    },
  ];

  return (
    <DreamSegmentSwitch
      value={props.mode}
      items={items}
      compact={props.compact}
      ariaLabel={`${props.text.listen.hushTooltip} / ${props.text.listen.museTooltip} / ${props.text.listen.lingerTooltip}`}
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
    <div className="listen-list-control-surface listen-list-control-surface-bottom pointer-events-auto inline-flex w-auto gap-1 rounded-[1.35rem] p-1.5">
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={props.text.listen.localRefresh}
            title={props.text.listen.localRefresh}
            disabled={props.refreshing}
            data-active={props.refreshing ? "true" : "false"}
            className={cn(
              "relative z-10 flex h-9 w-9 items-center justify-center rounded-2xl text-sidebar-foreground/55 transition-[color,transform,opacity,background-color,box-shadow] duration-200 ease-out active:scale-95",
              "hover:text-sidebar-foreground focus-visible:outline-none disabled:pointer-events-none disabled:opacity-70",
              "data-[active=true]:bg-[hsl(var(--dream-shell-top)/0.68)] data-[active=true]:text-sidebar-foreground data-[active=true]:shadow-[0_10px_28px_-20px_hsl(var(--foreground)/0.62),inset_0_0_0_1px_hsl(var(--foreground)/0.07)] dark:data-[active=true]:bg-white/10",
            )}
            onClick={props.onRefresh}
          >
            <RefreshCw
              className={cn("h-4 w-4", props.refreshing ? "animate-spin" : "")}
            />
          </button>
        </TooltipTrigger>
        <TooltipContent side="top">
          {props.text.listen.localRefresh}
        </TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={props.text.listen.localClearMissing}
            title={props.text.listen.localClearMissing}
            disabled={props.clearingMissing}
            data-active={props.clearingMissing ? "true" : "false"}
            className={cn(
              "relative z-10 flex h-9 w-9 items-center justify-center rounded-2xl text-sidebar-foreground/55 transition-[color,transform,opacity,background-color,box-shadow] duration-200 ease-out active:scale-95",
              "hover:text-sidebar-foreground focus-visible:outline-none disabled:pointer-events-none disabled:opacity-70",
              "data-[active=true]:bg-[hsl(var(--dream-shell-top)/0.68)] data-[active=true]:text-sidebar-foreground data-[active=true]:shadow-[0_10px_28px_-20px_hsl(var(--foreground)/0.62),inset_0_0_0_1px_hsl(var(--foreground)/0.07)] dark:data-[active=true]:bg-white/10",
            )}
            onClick={props.onClearMissing}
          >
            {props.clearingMissing ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Trash2 className="h-4 w-4" />
            )}
          </button>
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
      <div className="listen-list-control-surface listen-list-control-surface-top relative w-full max-w-[17rem] rounded-[26px] px-5 py-6 text-center">
        <div className="relative flex flex-col items-center gap-4">
          <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[hsl(var(--dream-shell-top)/0.42)] text-sidebar-foreground/70 shadow-[inset_0_0_0_1px_hsl(var(--foreground)/0.07)]">
            {props.icon ?? <Link2 className="h-5 w-5" />}
          </div>
          <p className="text-sm leading-6 text-sidebar-foreground/78">{props.message}</p>
          <Button
            type="button"
            className="rounded-full bg-sidebar-primary px-5 text-sidebar-primary-foreground shadow-[0_18px_40px_-24px_hsl(var(--sidebar-primary)/0.68)] hover:bg-sidebar-primary/90"
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
      <div className="listen-list-control-surface listen-list-control-surface-bottom inline-flex flex-wrap items-center justify-center gap-3 rounded-full px-3 py-2.5">
        {props.showQueueControls === false ? null : (
          <>
            <ListenActionIconButton
              label={`${props.text.listen.playbackMode}: ${playModeLabel}`}
              className={cn(
                props.playMode !== "order" &&
                  "bg-sidebar-primary/12 text-sidebar-primary hover:bg-sidebar-primary/14 hover:text-sidebar-primary shadow-[inset_0_0_0_1px_hsl(var(--sidebar-primary)/0.18)]",
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
          className="h-12 w-12 rounded-full border-transparent bg-sidebar-primary text-sidebar-primary-foreground shadow-[0_18px_42px_-18px_hsl(var(--sidebar-primary)/0.65)] hover:bg-sidebar-primary/90 hover:text-sidebar-primary-foreground"
          disabled={!props.hasTrack || props.loading}
          onClick={props.onTogglePlayback}
        >
          {props.loading ? (
            <Loader2 className="h-5 w-5 animate-spin" />
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
    <div className={cn("w-full max-w-2xl px-1", props.className)}>
      <div
        className={cn(
          "relative mb-2 h-5",
          canSeek ? "cursor-pointer" : undefined,
        )}
      >
        <div
          className={cn(
            "absolute inset-x-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden rounded-full",
            lightTone ? "bg-white/12" : "bg-foreground/10",
          )}
        >
          <div
            className={cn(
              "absolute inset-y-0 left-0 rounded-full transition-[width] duration-300",
              lightTone ? "bg-white/26" : "bg-sidebar-foreground/12",
            )}
            style={{ width: `${bufferedProgress * 100}%` }}
          />
          <div
            className={cn(
              "absolute inset-y-0 left-0 h-full rounded-full transition-[width] duration-150",
              lightTone
                ? "bg-[linear-gradient(90deg,rgba(255,255,255,0.95),rgba(255,255,255,0.62))] shadow-[0_0_24px_rgba(255,255,255,0.18)]"
                : "bg-[linear-gradient(90deg,hsl(var(--sidebar-primary)),hsl(var(--sidebar-primary)/0.72))] shadow-[0_0_24px_hsl(var(--sidebar-primary)/0.35)]",
            )}
            style={{ width: `${progress * 100}%` }}
          />
        </div>
        {canSeek ? (
          <>
            <span
              aria-hidden="true"
              className={cn(
                "pointer-events-none absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full border shadow-sm transition-[left] duration-150",
                lightTone
                  ? "border-white/70 bg-white"
                  : "border-sidebar-background bg-sidebar-primary",
              )}
              style={{ left: `${progress * 100}%` }}
            />
            <input
              type="range"
              min={0}
              max={duration}
              step={0.1}
              value={currentTime}
              aria-label={props.ariaLabel}
              className="absolute inset-0 z-10 h-full w-full cursor-pointer opacity-0"
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
        className={cn(
          "flex items-center justify-between text-[11px] font-medium tabular-nums",
          lightTone ? "text-white/62" : "text-sidebar-foreground/46",
        )}
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
          <div className="min-w-0 truncate text-xs font-semibold text-sidebar-foreground/58">
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
                      className={cn(
                        "h-7 w-7 rounded-full",
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
                      className={cn(
                        "h-7 w-7 rounded-full",
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
                      className={cn(
                        "h-7 w-7 rounded-full",
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
                      className={cn(
                        "h-7 w-7 rounded-full",
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
                      className={cn(
                        "h-7 w-7 rounded-full",
                        LISTEN_CONTROL_ICON_BUTTON_CLASS,
                        "hover:bg-destructive/10 hover:text-destructive",
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
                  <div className="flex min-w-0 items-center gap-1.5 text-sm font-medium text-sidebar-foreground">
                    <span className="min-w-0 truncate">{item.title}</span>
                    {showVideoIndicator ? <ListenMuseVideoIndicator /> : null}
                  </div>
                  <div className="truncate text-xs text-sidebar-foreground/58">
                    {metadataParts.join(" · ")}
                  </div>
                  {item.description ? (
                    <div className="truncate text-[11px] text-sidebar-foreground/48">
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
                        className={cn(
                          "h-8 w-8 rounded-full",
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
                        className={cn(
                          "h-8 w-8 rounded-full",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                          "hover:bg-destructive/10 hover:text-destructive",
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
          className={cn("h-7 w-7 rounded-full", LISTEN_CONTROL_ICON_BUTTON_CLASS)}
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
        className="flex min-w-0 snap-x snap-mandatory gap-2 overflow-x-auto overflow-y-hidden pb-1 pr-10 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
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
        "absolute top-0 z-30 h-[7.25rem] w-20",
        isLeft
          ? "left-0 bg-gradient-to-r from-[hsl(var(--sidebar-background)/0.94)] via-[hsl(var(--sidebar-background)/0.68)] to-transparent"
          : "right-0 bg-gradient-to-l from-[hsl(var(--sidebar-background)/0.94)] via-[hsl(var(--sidebar-background)/0.68)] to-transparent",
      )}
      onPointerDown={(event) => event.stopPropagation()}
    >
      <button
        type="button"
        className={cn(
          "listen-horizontal-scroll-button flex h-full w-full items-center text-sidebar-foreground/58 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/35",
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
        <span className="flex h-10 w-10 items-center justify-center rounded-full border border-sidebar-border/45 bg-sidebar-background/78 shadow-[0_14px_30px_-24px_hsl(var(--foreground)/0.86),inset_0_1px_0_hsl(var(--background)/0.22)] backdrop-blur-md transition-[transform,background-color,color,box-shadow] duration-200 ease-out hover:scale-[1.04] hover:bg-sidebar-background/92 hover:text-sidebar-foreground active:scale-95">
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
                "listen-track-list-row grid min-h-14 w-full grid-cols-[2rem_minmax(0,1fr)_3.25rem] items-center gap-2 rounded-2xl border border-transparent px-2 py-2 text-left transition-[transform,background-color,border-color] duration-200 ease-out active:scale-[0.99] focus-visible:outline-none",
                selected
                  ? "border-sidebar-primary/18 bg-sidebar-primary/10"
                  : "hover:-translate-y-0.5 hover:bg-sidebar-background/54",
              )}
              data-selected={selected ? "true" : undefined}
              onClick={() => props.onSelect(item)}
            >
              <span
                className={cn(
                  "flex h-7 w-7 shrink-0 items-center justify-center text-[11px] font-semibold tabular-nums",
                  selected ? "text-sidebar-primary" : "text-sidebar-foreground/38",
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
                  <span className="block truncate text-xs text-sidebar-foreground/58">
                    {artistLabel}
                  </span>
                </span>
              </span>
              <span className="justify-self-end text-right text-[11px] font-medium tabular-nums text-sidebar-foreground/42">
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
              "listen-track-list-row grid min-h-14 w-full grid-cols-[minmax(0,1.45fr)_minmax(0,0.82fr)_minmax(0,0.92fr)_3.25rem] items-center gap-2 rounded-2xl border border-transparent px-2 py-2 text-left transition-[transform,background-color,border-color] duration-200 ease-out active:scale-[0.99] focus-visible:outline-none",
              selected
                ? "border-sidebar-primary/18 bg-sidebar-primary/10"
                : "hover:-translate-y-0.5 hover:bg-sidebar-background/54",
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
            <span className="min-w-0 truncate text-xs font-medium text-sidebar-foreground/58">
              {artistLabel}
            </span>
            <span className="min-w-0 truncate text-xs font-medium text-sidebar-foreground/46">
              {albumLabel}
            </span>
            <span className="justify-self-end text-right text-[11px] font-medium tabular-nums text-sidebar-foreground/42">
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
  return (
    <section className="listen-muse-track-list-group min-w-0 space-y-2 overflow-hidden">
      <div className="flex min-h-7 items-center justify-between gap-2 px-2">
        <div className="min-w-0 truncate text-xs font-semibold text-sidebar-foreground/58">
          {props.title}
        </div>
        {props.onSeeAll ? (
          <button
            type="button"
            className="shrink-0 rounded-full px-2 py-1 text-[11px] font-semibold text-sidebar-primary/78 transition-[background-color,color] duration-150 ease-out hover:bg-sidebar-primary/10 hover:text-sidebar-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/35"
            onClick={props.onSeeAll}
          >
            {props.text.listen.seeAll}
          </button>
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
        "h-3.5 w-3.5 shrink-0 text-sidebar-foreground/28",
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
      <span className="min-w-0 truncate text-sm font-medium text-sidebar-foreground">
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
    <section className="listen-muse-group-frame min-w-0 space-y-2 overflow-hidden">
      {headerTitle || hasActions ? (
        <div className="wails-drag flex min-h-7 items-center justify-between gap-2 px-2">
          <div className="min-w-0 truncate text-xs font-semibold text-sidebar-foreground/58">
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
      className="listen-muse-card group/muse-card relative w-[7.25rem] shrink-0 snap-start rounded-lg"
      data-selected={props.selected ? "true" : undefined}
    >
      <button
        type="button"
        className="block w-full rounded-lg text-left outline-none transition focus-visible:ring-2 focus-visible:ring-ring/35"
        onClick={props.onSelect}
      >
        <ListenMuseCardArtwork
          httpBaseURL={props.httpBaseURL}
          item={props.item}
          selected={props.selected}
          liftOnHover={false}
          softenOnHover
        >
          <span className="listen-playback-hover-layer pointer-events-none absolute inset-0 z-10 flex items-center justify-center bg-black/16 opacity-0">
            <span className="listen-playback-hover-button flex h-10 w-10 items-center justify-center rounded-full bg-sidebar-primary text-sidebar-primary-foreground shadow-[0_18px_42px_-28px_hsl(var(--sidebar-primary)/0.72)]">
              <Play className="ml-0.5 h-4 w-4 fill-current" />
            </span>
          </span>
        </ListenMuseCardArtwork>
      </button>
      <button
        type="button"
        className="block w-full rounded-md text-left outline-none transition focus-visible:ring-2 focus-visible:ring-ring/35"
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
      className="listen-muse-card group/muse-card relative w-[7.25rem] shrink-0 snap-start rounded-lg"
      data-selected={props.selected ? "true" : undefined}
    >
      <button
        type="button"
        className="block w-full rounded-lg text-left outline-none transition focus-visible:ring-2 focus-visible:ring-ring/35"
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
      className="listen-muse-card group/muse-card block w-[7.25rem] shrink-0 snap-start rounded-lg text-left outline-none transition focus-visible:ring-2 focus-visible:ring-ring/35"
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
      className="listen-muse-card group/muse-card block w-[7.25rem] shrink-0 snap-start rounded-lg text-left outline-none transition focus-visible:ring-2 focus-visible:ring-ring/35"
      data-selected={props.selected ? "true" : undefined}
      onClick={props.onSelect}
    >
      <span
        className={cn(
          "listen-muse-card-artwork relative flex aspect-square w-full items-center justify-center overflow-hidden rounded-lg bg-sidebar-background/65 text-sidebar-foreground/68 shadow-[0_14px_30px_-24px_hsl(var(--foreground)/0.72)] ring-1 ring-[hsl(var(--foreground)/0.08)] transition-[transform,box-shadow] duration-200 ease-out group-hover/muse-card:-translate-y-0.5 group-hover/muse-card:shadow-[0_18px_38px_-28px_hsl(var(--foreground)/0.86)]",
          props.selected && "ring-sidebar-primary/35",
        )}
        style={swatch ? { backgroundColor: `${swatch}20` } : undefined}
      >
        {swatch ? (
          <span
            className="absolute inset-x-0 bottom-0 h-1"
            style={{ backgroundColor: swatch }}
          />
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
      props.item.videoId?.trim()
        ? buildListenTrackThumbnailCandidates(props.httpBaseURL, {
            videoId: props.item.videoId,
            thumbnailUrl: props.item.thumbnailUrl,
          })
        : buildListenImageCandidates(
            props.httpBaseURL,
            props.item.thumbnailUrl ?? "",
          ),
    [props.httpBaseURL, props.item.thumbnailUrl, props.item.videoId],
  );
  const candidateKey = candidates.join("\n");
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const source = candidates[candidateIndex] ?? "";

  React.useEffect(() => {
    setCandidateIndex(0);
  }, [candidateKey]);
  const liftOnHover = props.liftOnHover !== false;
  const isCircle = props.shape === "circle";

  return (
    <span
      className={cn(
        "listen-muse-card-artwork relative block aspect-square w-full overflow-hidden bg-sidebar-background/65 shadow-[0_14px_30px_-24px_hsl(var(--foreground)/0.72)] ring-1 ring-[hsl(var(--foreground)/0.08)] duration-200 ease-out",
        isCircle ? "rounded-full" : "rounded-lg",
        liftOnHover
          ? "transition-[transform,box-shadow] group-hover/muse-card:-translate-y-0.5 group-hover/muse-card:shadow-[0_18px_38px_-28px_hsl(var(--foreground)/0.86)]"
          : "transition-[box-shadow] group-hover/muse-card:shadow-[0_18px_38px_-28px_hsl(var(--foreground)/0.86)]",
        props.selected && "ring-sidebar-primary/35",
      )}
    >
      {source ? (
        <>
          <img
            src={source}
            alt=""
            className="h-full w-full object-cover transition-transform duration-300 ease-out group-hover/muse-card:scale-[1.045] group-focus-within/muse-card:scale-[1.045]"
            loading="lazy"
            onError={() => setCandidateIndex((current) => current + 1)}
          />
          {props.softenOnHover ? (
            <img
              src={source}
              alt=""
              aria-hidden="true"
              className="listen-hover-soften-image pointer-events-none absolute inset-0 h-full w-full scale-[1.08] object-cover opacity-0 blur-[5px]"
            />
          ) : null}
        </>
      ) : (
        <span className="flex h-full w-full items-center justify-center text-xl font-semibold text-sidebar-foreground/58">
          <Music2 className="h-7 w-7" />
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
}) {
  return (
    <span className="block min-w-0 px-0.5 pt-1.5">
      <span className="flex min-w-0 items-center gap-1.5">
        <span className="min-w-0 truncate text-xs font-semibold leading-4 text-sidebar-foreground/64">
          {props.title}
        </span>
        {props.hasVideo ? <ListenMuseVideoIndicator /> : null}
      </span>
      {props.subtitle ? (
        <span className="block truncate text-[10px] font-medium leading-4 text-sidebar-foreground/42">
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
  const candidateKey = candidates.join("\n");
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const source =
    candidates[
      Math.min(candidateIndex, Math.max(candidates.length - 1, 0))
    ] || LISTEN_DEFAULT_COVER_IMAGE_URL;

  React.useEffect(() => {
    setCandidateIndex(0);
  }, [candidateKey]);

  return (
    <span
      className={cn(
        "relative flex h-10 w-10 shrink-0 overflow-hidden rounded-xl bg-muted ring-1 ring-border/70",
        props.selected && "ring-primary/30",
      )}
    >
      <img
        key={source}
        src={source}
        alt=""
        className="h-full w-full object-cover"
        loading="lazy"
        onError={() => setCandidateIndex((current) => current + 1)}
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
    <span
      data-status={props.status}
      className={cn(
        "inline-flex h-5 shrink-0 items-center gap-1 rounded-full px-1.5 text-[10px] font-semibold",
        live && "bg-red-500/12 text-red-600 dark:text-red-300",
        checking && "bg-sidebar-background/62 text-sidebar-foreground/50",
        upcoming && "bg-amber-500/12 text-amber-700 dark:text-amber-300",
        !live && !checking && !upcoming && "bg-sidebar-background/62 text-sidebar-foreground/54",
      )}
    >
      {checking ? (
        <Loader2 className="h-3 w-3 animate-spin" />
      ) : (
        <Radio className={cn("h-3 w-3", live && "fill-current")} />
      )}
      <span className="whitespace-nowrap">{label}</span>
    </span>
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
                  <div className="truncate text-sm font-medium text-sidebar-foreground">
                    {item.title}
                  </div>
                  <div className="truncate text-xs text-sidebar-foreground/58">
                    {item.channel}
                  </div>
                  {item.description ? (
                    <div className="truncate text-[11px] text-sidebar-foreground/48">
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
                        disabled={isMutating}
                        className={cn(
                          "h-8 w-8 shrink-0 rounded-full",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                          "hover:bg-sidebar-primary/10 hover:text-sidebar-primary",
                          isSaved &&
                            "bg-sidebar-primary/10 text-sidebar-primary hover:bg-sidebar-primary/12",
                        )}
                        aria-label={actionLabel}
                        title={actionLabel}
                        onClick={() => props.onToggleLibrary?.(item, "add")}
                      >
                        {isMutating ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
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
                  className="relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-sidebar-background/68 text-sidebar-foreground shadow-[inset_0_1px_0_hsl(var(--background)/0.20)] ring-1 ring-[hsl(var(--foreground)/0.08)]"
                  style={swatch ? { backgroundColor: `${swatch}20` } : undefined}
                >
                  <span
                    className="absolute left-0 top-0 h-full w-1"
                    style={swatch ? { backgroundColor: swatch } : undefined}
                  />
                  <Tags className="relative h-4 w-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium text-sidebar-foreground">
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
                  <div className="truncate text-sm font-medium text-sidebar-foreground">
                    {item.name}
                  </div>
                  <div className="truncate text-xs text-sidebar-foreground/58">
                    {item.subtitle || "YouTube Music"}
                  </div>
                </div>
                <UserRound className="h-4 w-4 shrink-0 text-sidebar-foreground/48" />
              </SidebarMenuButton>
            </SidebarMenuItem>
          );
        })}
      </SidebarMenu>
    </div>
  );
}
