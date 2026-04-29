import {
Disc3,
Link2,
ListMusic,
Loader2,
Music2,
Pause,
Play,
Plus,
Radio,
RefreshCw,
Repeat2,
Shuffle,
Tags,
SkipBack,
SkipForward,
Trash2,
UserRound,
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
import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import { Button } from "@/shared/ui/button";
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
DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
DREAM_FM_CONTROL_SURFACE_CLASS,
DREAM_FM_LIST_ITEM_BUTTON_CLASS,
DREAM_FM_LIST_SECTION_TITLE_CLASS,
} from "@/shared/styles/dreamfm";

import { clampVolume,formatProgressSeconds } from "@/app/main/dreamfm/local-library";
import { buildDreamFMImageCandidates,buildDreamFMPosterCandidates,buildDreamFMTrackThumbnailCandidates } from "@/app/main/dreamfm/storage";
import type { DreamFMArtistItem,DreamFMCategoryItem,DreamFMLiveStatus,DreamFMLiveStatusValue,DreamFMLocalItem,DreamFMMode,DreamFMOnlineItem,DreamFMPlayMode,DreamFMPlaylistItem,DreamFMPlaylistLibraryAction } from "@/app/main/dreamfm/types";

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

export function TabIconButton(props: {
  active: boolean;
  compact?: boolean;
  label: string;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={props.label}
          data-active={props.active ? "true" : "false"}
          data-compact={props.compact ? "true" : "false"}
          className={cn(
            "flex h-8 items-center justify-center overflow-hidden rounded-xl text-sidebar-foreground/55 transition-[width,padding,background-color,color,box-shadow,transform] duration-200 ease-out active:scale-95",
            "hover:scale-[1.03] hover:bg-sidebar-background/54 hover:text-sidebar-foreground focus-visible:outline-none",
            "data-[active=true]:bg-[hsl(var(--dream-shell-top)/0.58)] data-[active=true]:text-sidebar-foreground data-[active=true]:shadow-[0_8px_22px_-18px_hsl(var(--foreground)/0.45)] dark:data-[active=true]:bg-white/10",
            props.compact ? "w-8 min-w-8 px-0" : "w-[5.25rem] min-w-8 px-1.5",
          )}
          onClick={props.onClick}
        >
          {props.children}
          <span
            className={cn(
              "block min-w-0 truncate text-xs font-medium transition-[margin,max-width,opacity,transform] duration-200 ease-out",
              props.compact
                ? "ml-0 max-w-0 -translate-x-1 opacity-0"
                : "ml-1.5 max-w-12 translate-x-0 opacity-100",
            )}
          >
            {props.label}
          </span>
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{props.label}</TooltipContent>
    </Tooltip>
  );
}

export function DreamFMAvatar(props: {
  httpBaseURL: string;
  item: { channel: string; thumbnailUrl?: string; videoId?: string };
  selected?: boolean;
}) {
  const avatarCandidates = React.useMemo(
    () =>
      props.item.videoId?.trim()
        ? buildDreamFMTrackThumbnailCandidates(props.httpBaseURL, {
            videoId: props.item.videoId,
            thumbnailUrl: props.item.thumbnailUrl,
          })
        : buildDreamFMImageCandidates(
            props.httpBaseURL,
            props.item.thumbnailUrl ?? "",
          ),
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
        "relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-sidebar-background/68 text-xs font-semibold text-sidebar-foreground shadow-[inset_0_1px_0_hsl(var(--background)/0.20)] ring-1 ring-[hsl(var(--foreground)/0.08)]",
        props.selected && "ring-sidebar-primary/26",
      )}
    >
      <span className="absolute inset-0 bg-[radial-gradient(circle_at_30%_20%,hsl(var(--primary)/0.30),transparent_58%),linear-gradient(135deg,hsl(var(--muted)),hsl(var(--background)))]" />
      {!imageReady ? (
        <span className="pointer-events-none relative z-0 text-[11px]">
          {props.item.channel.slice(0, 2).toUpperCase()}
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

export function DreamFMLocalArtwork(props: { track: DreamFMLocalItem }) {
  const coverURL = props.track.coverURL.trim();
  const [failedURLs, setFailedURLs] = React.useState<Set<string>>(
    () => new Set(),
  );
  const source =
    [coverURL, DEFAULT_COVER_IMAGE_URL]
      .filter(Boolean)
      .find((url) => !failedURLs.has(url)) ?? "";

  React.useEffect(() => {
    setFailedURLs(new Set());
  }, [props.track.coverURL]);

  return (
    <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-sidebar-background/68 text-sidebar-foreground/75 shadow-[inset_0_1px_0_hsl(var(--background)/0.20)] ring-1 ring-[hsl(var(--foreground)/0.08)]">
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

export function DreamFMOnlineArtwork(props: {
  httpBaseURL: string;
  track: DreamFMOnlineItem;
  liveLabel: string;
  className?: string;
}) {
  const posterCandidates = React.useMemo(
    () => buildDreamFMPosterCandidates(props.httpBaseURL, props.track),
    [props.httpBaseURL, props.track.thumbnailUrl, props.track.videoId],
  );
  const posterCandidateKey = posterCandidates.join("\n");
  const [posterIndex, setPosterIndex] = React.useState(0);
  const activePoster =
    posterCandidates[
      Math.min(posterIndex, Math.max(posterCandidates.length - 1, 0))
    ] || DEFAULT_COVER_IMAGE_URL;

  React.useEffect(() => {
    setPosterIndex(0);
  }, [posterCandidateKey]);

  return (
    <DreamFMArtworkShell
      className={props.className}
      badge={
        props.track.group === "live" ? (
          <>
            <Radio className="h-3 w-3" />
            {props.liveLabel}
          </>
        ) : undefined
      }
    >
      <>
        <img
          key={activePoster}
          src={activePoster}
          alt={props.track.title}
          className="h-full w-full object-cover transition-transform duration-500 ease-out group-hover/dream-fm-artwork:scale-[1.035]"
          loading="eager"
          onError={() => {
            setPosterIndex((current) => {
              if (current >= posterCandidates.length - 1) {
                return current;
              }
              return current + 1;
            });
          }}
        />
        <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(15,23,42,0.02),rgba(15,23,42,0.12))]" />
      </>
    </DreamFMArtworkShell>
  );
}

export function DreamFMActionIconButton(props: {
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
              DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
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

export function DreamFMVolumeControl(props: {
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
      ? props.text.dreamFm.unmute
      : props.text.dreamFm.mute;
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
              DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
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
              aria-label={props.text.dreamFm.volume}
              title={props.text.dreamFm.volume}
              className="relative z-10 h-6 w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
              onChange={(event) =>
                props.onVolumeChange(Number(event.target.value))
              }
            />
          </span>
        </span>
      </div>
      <TooltipContent side="top">{props.text.dreamFm.volume}</TooltipContent>
    </Tooltip>
  );
}

export function DreamFMArtworkShell(props: {
  badge?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        "group/dream-fm-artwork relative w-[min(18rem,62vw,48vh)] shrink-0 animate-in fade-in-0 zoom-in-95 duration-300 sm:w-[min(22rem,50vw,54vh)] xl:w-[min(25rem,34vw,58vh)]",
        props.className,
      )}
    >
      <div className="absolute inset-0 translate-y-5 rounded-[2rem] bg-black/14 blur-3xl transition-[transform,opacity] duration-300 ease-out group-hover/dream-fm-artwork:translate-y-6 group-hover/dream-fm-artwork:scale-105 group-hover/dream-fm-artwork:opacity-80" />
      <div className="relative aspect-square overflow-hidden rounded-[2rem] border border-white/50 bg-white shadow-[0_28px_90px_-42px_rgba(15,23,42,0.45)] transition-[transform,box-shadow] duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)] group-hover/dream-fm-artwork:-translate-y-1 group-hover/dream-fm-artwork:scale-[1.012] group-hover/dream-fm-artwork:shadow-[0_34px_100px_-46px_rgba(15,23,42,0.56)]">
        {props.children}
        {props.badge ? (
          <div className="absolute left-3 top-3 inline-flex items-center gap-1 rounded-full bg-black/66 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-white">
            {props.badge}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function DreamFMModeTabs(props: {
  mode: DreamFMMode;
  compact: boolean;
  text: ReturnType<typeof getXiaText>;
  onChange: (mode: DreamFMMode) => void;
}) {
  return (
    <div className="dream-fm-list-control-surface dream-fm-list-control-surface-top ml-auto inline-flex min-w-0 max-w-full shrink overflow-hidden rounded-2xl p-0.5 transition-[width,background-color,box-shadow] duration-200 ease-out">
      <TabIconButton
        active={props.mode === "live"}
        compact={props.compact}
        label={props.text.dreamFm.live}
        onClick={() => props.onChange("live")}
      >
        <Radio className="h-4 w-4" />
      </TabIconButton>
      <TabIconButton
        active={props.mode === "online"}
        compact={props.compact}
        label={props.text.dreamFm.online}
        onClick={() => props.onChange("online")}
      >
        <SimpleBrandIcon icon={siYoutubemusic} className="h-4 w-4" />
      </TabIconButton>
      <TabIconButton
        active={props.mode === "local"}
        compact={props.compact}
        label={props.text.dreamFm.local}
        onClick={() => props.onChange("local")}
      >
        <Disc3 className="h-4 w-4" />
      </TabIconButton>
    </div>
  );
}

export function DreamFMLocalListControls(props: {
  text: ReturnType<typeof getXiaText>;
  refreshing: boolean;
  clearingMissing: boolean;
  onRefresh: () => void;
  onClearMissing: () => void;
}) {
  return (
    <div className="dream-fm-list-control-surface dream-fm-list-control-surface-bottom pointer-events-auto inline-flex w-auto gap-1 rounded-[1.35rem] p-1.5">
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={props.text.dreamFm.localRefresh}
            title={props.text.dreamFm.localRefresh}
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
          {props.text.dreamFm.localRefresh}
        </TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={props.text.dreamFm.localClearMissing}
            title={props.text.dreamFm.localClearMissing}
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
          {props.text.dreamFm.localClearMissing}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

export function DreamFMConnectionPromptCard(props: {
  message: string;
  actionLabel: string;
  icon?: React.ReactNode;
  onAction: () => void;
}) {
  return (
    <div className="flex min-h-full items-center justify-center px-2 py-6">
      <div className="dream-fm-list-control-surface dream-fm-list-control-surface-top relative w-full max-w-[17rem] rounded-[26px] px-5 py-6 text-center">
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

export function DreamFMTransportActions(props: {
  hasTrack: boolean;
  playing: boolean;
  loading?: boolean;
  previousDisabled?: boolean;
  nextDisabled?: boolean;
  playModeDisabled?: boolean;
  showQueueControls?: boolean;
  muted: boolean;
  volume: number;
  playMode: DreamFMPlayMode;
  text: ReturnType<typeof getXiaText>;
  onPrevious: () => void;
  onNext: () => void;
  onTogglePlayMode: () => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const playbackLabel = props.loading
    ? props.text.dreamFm.loading
    : props.playing
      ? props.text.dreamFm.pause
      : props.text.dreamFm.play;
  const playModeLabel =
    props.playMode === "shuffle"
      ? props.text.dreamFm.playModeShuffle
      : props.playMode === "repeat"
        ? props.text.dreamFm.playModeRepeat
        : props.text.dreamFm.playModeOrder;
  return (
    <div className="flex justify-center">
      <div className="dream-fm-list-control-surface dream-fm-list-control-surface-bottom inline-flex flex-wrap items-center justify-center gap-3 rounded-full px-3 py-2.5">
        {props.showQueueControls === false ? null : (
          <>
            <DreamFMActionIconButton
              label={`${props.text.dreamFm.playbackMode}: ${playModeLabel}`}
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
            </DreamFMActionIconButton>
            <DreamFMActionIconButton
              label={props.text.dreamFm.previous}
              disabled={!props.hasTrack || props.previousDisabled}
              onClick={props.onPrevious}
            >
              <SkipBack className="h-4 w-4" />
            </DreamFMActionIconButton>
          </>
        )}
        <DreamFMActionIconButton
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
        </DreamFMActionIconButton>
        {props.showQueueControls === false ? null : (
          <DreamFMActionIconButton
            label={props.text.dreamFm.next}
            disabled={!props.hasTrack || props.nextDisabled}
            onClick={props.onNext}
          >
            <SkipForward className="h-4 w-4" />
          </DreamFMActionIconButton>
        )}
        <DreamFMVolumeControl
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

export function DreamFMProgressBar(props: {
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

export function DreamFMOnlineGroup(props: {
  title: string;
  hideTitle?: boolean;
  items: DreamFMOnlineItem[];
  selectedId: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  onPlayAll?: () => void;
  onShuffle?: () => void;
  onClear?: () => void;
  clearLabel?: string;
  onRemove?: (item: DreamFMOnlineItem) => void;
  removeLabel?: string;
  liveStatuses?: Record<string, DreamFMLiveStatus>;
  onSelect: (item: DreamFMOnlineItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  const hasHeaderActions = Boolean(
    props.onPlayAll || props.onShuffle || props.onClear,
  );
  const headerTitle = props.hideTitle ? "" : props.title.trim();
  return (
    <div>
      {headerTitle || hasHeaderActions ? (
        <div className="mb-2 flex min-h-7 items-center justify-between gap-2 px-2">
          <div className="min-w-0 truncate text-xs font-semibold text-sidebar-foreground/58">
            {headerTitle}
          </div>
          {hasHeaderActions ? (
            <div className={DREAM_FM_CONTROL_SURFACE_CLASS}>
              {props.onPlayAll ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      className={cn(
                        "h-7 w-7 rounded-full",
                        DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.text.dreamFm.playAll}
                      title={props.text.dreamFm.playAll}
                      onClick={props.onPlayAll}
                    >
                      <Play className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.text.dreamFm.playAll}
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
                        DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.text.dreamFm.shuffleAll}
                      title={props.text.dreamFm.shuffleAll}
                      onClick={props.onShuffle}
                    >
                      <Shuffle className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.text.dreamFm.shuffleAll}
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
                        DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                        "hover:bg-destructive/10 hover:text-destructive",
                      )}
                      aria-label={props.clearLabel ?? props.text.dreamFm.clearQueue}
                      title={props.clearLabel ?? props.text.dreamFm.clearQueue}
                      onClick={props.onClear}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.clearLabel ?? props.text.dreamFm.clearQueue}
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
          return (
            <SidebarMenuItem
              key={item.id}
              className={cn(props.onRemove && "flex items-center gap-1.5")}
            >
              <SidebarMenuButton
                type="button"
                isActive={selected}
                className={cn(
                  "min-h-16",
                  DREAM_FM_LIST_ITEM_BUTTON_CLASS,
                  props.onRemove && "min-w-0 flex-1",
                )}
                onClick={() => props.onSelect(item)}
              >
                <DreamFMAvatar
                  httpBaseURL={props.httpBaseURL}
                  item={item}
                  selected={selected}
                />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium text-sidebar-foreground">
                    {item.title}
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
                {item.group === "live" ? (
                  <DreamFMLiveStatusBadge
                    status={
                      props.liveStatuses
                        ? props.liveStatuses[item.videoId]?.status ?? "checking"
                        : "live"
                    }
                    text={props.text}
                  />
                ) : null}
              </SidebarMenuButton>
              {props.onRemove ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex h-16 w-9 shrink-0 items-center justify-center self-center">
                      <Button
                        type="button"
                        variant="outline"
                        size="compactIcon"
                        className={cn(
                          "h-8 w-8 rounded-full",
                          DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                          "hover:bg-destructive/10 hover:text-destructive",
                        )}
                        aria-label={
                          props.removeLabel ?? props.text.dreamFm.removeFromQueue
                        }
                        title={
                          props.removeLabel ?? props.text.dreamFm.removeFromQueue
                        }
                        onClick={() => props.onRemove?.(item)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="left">
                    {props.removeLabel ?? props.text.dreamFm.removeFromQueue}
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

function DreamFMLiveStatusBadge(props: {
  status: DreamFMLiveStatusValue;
  text: ReturnType<typeof getXiaText>;
}) {
  const live = props.status === "live";
  const checking = props.status === "checking";
  const upcoming = props.status === "upcoming";
  const label =
    props.status === "live"
      ? props.text.dreamFm.liveStatusLive
      : props.status === "offline"
        ? props.text.dreamFm.liveStatusOffline
        : props.status === "upcoming"
          ? props.text.dreamFm.liveStatusUpcoming
          : props.status === "unavailable"
            ? props.text.dreamFm.liveStatusUnavailable
            : props.status === "checking"
              ? props.text.dreamFm.liveStatusChecking
              : props.text.dreamFm.liveStatusUnknown;
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

export function DreamFMPlaylistGroup(props: {
  title: string;
  items: DreamFMPlaylistItem[];
  selectedPlaylistId: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  savedPlaylistIds: Set<string>;
  playlistMutationAction: DreamFMPlaylistLibraryAction | null;
  playlistMutationPlaylistId: string;
  onSelect: (item: DreamFMPlaylistItem) => void;
  onToggleLibrary?: (
    item: DreamFMPlaylistItem,
    action: DreamFMPlaylistLibraryAction,
  ) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <div>
      <div className={DREAM_FM_LIST_SECTION_TITLE_CLASS}>
        {props.title}
      </div>
      <SidebarMenu className="gap-1.5">
        {props.items.map((item) => {
          const selected = item.playlistId === props.selectedPlaylistId;
          const isSaved = props.savedPlaylistIds.has(item.playlistId);
          const isMutating =
            props.playlistMutationPlaylistId === item.playlistId;
          const actionLabel = props.text.dreamFm.savePlaylist;
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
                  DREAM_FM_LIST_ITEM_BUTTON_CLASS,
                )}
                onClick={() => props.onSelect(item)}
              >
                <DreamFMAvatar
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
                          DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
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
                    {isMutating ? props.text.dreamFm.savePlaylist : actionLabel}
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

export function DreamFMCategoryGroup(props: {
  title: string;
  items: DreamFMCategoryItem[];
  selectedCategoryId?: string;
  onSelect: (item: DreamFMCategoryItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <div>
      <div className={DREAM_FM_LIST_SECTION_TITLE_CLASS}>
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
                className={cn("min-h-14", DREAM_FM_LIST_ITEM_BUTTON_CLASS)}
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

export function DreamFMArtistGroup(props: {
  title: string;
  items: DreamFMArtistItem[];
  selectedArtistId?: string;
  httpBaseURL: string;
  onSelect: (item: DreamFMArtistItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  return (
    <div>
      <div className={DREAM_FM_LIST_SECTION_TITLE_CLASS}>
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
                className={cn("min-h-16", DREAM_FM_LIST_ITEM_BUTTON_CLASS)}
                onClick={() => props.onSelect(item)}
              >
                <DreamFMAvatar
                  httpBaseURL={props.httpBaseURL}
                  item={{
                    channel: item.name,
                    thumbnailUrl: item.thumbnailUrl,
                  }}
                  selected={selected}
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
