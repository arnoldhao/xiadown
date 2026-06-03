import {
  Airplay,
  Copy,
  ExternalLink,
  ListMusic,
  Loader2,
  MoreHorizontal,
  Video,
} from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";
import {
  LISTEN_DROPDOWN_CONTENT_CLASS,
  LISTEN_DROPDOWN_ICON_SLOT_CLASS,
  LISTEN_DROPDOWN_ITEM_CLASS,
  LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS,
  LISTEN_PLAYER_ICON_BUTTON_CLASS,
} from "@/shared/styles/listen";

import { resolveListenLyricsIcon } from "@/app/main/listen/lyrics-icons";
import { resolveListenQueuePopupAnchor } from "@/app/main/listen/playback-helpers";
import type { ListenQueuePopupAnchor } from "@/app/main/listen/queue-popups";
import type { ListenLyricsKind, ListenObservedPlaybackAudioQuality } from "@/app/main/listen/types";
import {
  ListenArtworkShell,
  useListenStableImageSource,
} from "@/app/main/listen/ui";

export type ListenMediaMode = "cover" | "lyrics" | "video";

export type ListenAirPlayAnchor = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export function ListenPlayerFooter(props: {
  mediaMode: ListenMediaMode;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  sourceBadge?: React.ReactNode;
  sourceBadgeQuality?: ListenObservedPlaybackAudioQuality | "";
  hasVideo: boolean;
  videoHidden?: boolean;
  videoLoading?: boolean;
  live?: boolean;
  lyricsAvailable: boolean;
  lyricsKind?: ListenLyricsKind;
  lyricsLoading?: boolean;
  queueOpen?: boolean;
  text: ReturnType<typeof getXiaText>;
  onAirPlay?: (anchor: ListenAirPlayAnchor) => void;
  onMediaModeChange: (mode: ListenMediaMode) => void;
  onToggleQueue?: (anchor: ListenQueuePopupAnchor) => void;
}) {
  const toggleMediaMode = (mode: ListenMediaMode) => {
    props.onMediaModeChange(props.mediaMode === mode ? "cover" : mode);
  };
  const LyricsIcon = resolveListenLyricsIcon(props.lyricsKind);
  const handleAirPlayClick: React.MouseEventHandler<HTMLButtonElement> = (event) => {
    const rect = event.currentTarget.getBoundingClientRect();
    props.onAirPlay?.({
      x: rect.left,
      y: rect.top,
      width: rect.width,
      height: rect.height,
    });
  };
  const showMediaActions = !props.live;
  const showVideoAction = showMediaActions && !props.videoHidden;
  const videoModeActive = props.mediaMode === "video";
  const videoActionLoading = props.videoLoading === true && !videoModeActive;
  const videoActionDisabled =
    videoActionLoading || (!props.hasVideo && !videoModeActive);
  const handleQueueClick: React.MouseEventHandler<HTMLButtonElement> = (event) => {
    props.onToggleQueue?.(resolveListenQueuePopupAnchor(event.currentTarget));
  };
  const sourceBadgeQualityLabelKey = resolveObservedPlaybackAudioQualityLabelKey(props.sourceBadgeQuality ?? "");
  const sourceBadgeShowsAudioQuality = sourceBadgeQualityLabelKey !== "";
  const sourceBadgeTooltip = sourceBadgeQualityLabelKey
    ? props.text.listen.audioQualityOptions[sourceBadgeQualityLabelKey]
    : "";
  const sourceBadgeClassName = cn(
    "absolute left-1/2 top-1/2 flex max-w-[9rem] -translate-x-1/2 -translate-y-1/2 select-none items-center justify-center gap-1 overflow-hidden whitespace-nowrap text-[11px] font-medium uppercase leading-4 tracking-[0.22em] transition-colors [&>svg]:h-3 [&>svg]:w-3 [&>svg]:shrink-0",
    sourceBadgeShowsAudioQuality
      ? "z-20 cursor-default appearance-none border-0 bg-transparent p-0 pointer-events-auto focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sidebar-ring/50"
      : "listen-source-watermark z-0 pointer-events-none text-sidebar-foreground/24",
    sourceBadgeQualityLabelKey === "high" &&
      "text-emerald-700/80 dark:text-emerald-200/80",
    sourceBadgeQualityLabelKey === "medium" &&
      "text-sky-700/78 dark:text-sky-200/78",
    sourceBadgeQualityLabelKey === "low" &&
      "text-amber-700/80 dark:text-amber-200/80",
  );

  return (
    <footer className="relative z-20 shrink-0 px-0 pb-1 pt-2 sm:pb-2">
      <div className="relative flex h-12 w-full items-center justify-between gap-3 px-3">
        {props.sourceBadge ? (
          sourceBadgeShowsAudioQuality ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  className={sourceBadgeClassName}
                  aria-label={sourceBadgeTooltip}
                >
                  {props.sourceBadge}
                </button>
              </TooltipTrigger>
              <TooltipContent side="top">{sourceBadgeTooltip}</TooltipContent>
            </Tooltip>
          ) : (
            <div aria-hidden="true" className={sourceBadgeClassName}>
              {props.sourceBadge}
            </div>
          )
        ) : null}
        <div className="relative z-10 flex shrink-0 items-center gap-1">
          <ListenPlayerIconButton
            label={props.text.listen.airPlay}
            disabled={!props.airPlaySupported || !props.onAirPlay}
            className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
            onClick={handleAirPlayClick}
          >
            <Airplay className="h-4 w-4" />
          </ListenPlayerIconButton>
          {showVideoAction ? (
            <ListenPlayerIconButton
              label={
                props.videoLoading
                  ? props.text.listen.loading
                  : props.hasVideo
                  ? props.text.listen.video
                  : props.text.listen.noVideo
              }
              active={videoModeActive}
              disabled={videoActionDisabled}
              className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
              onClick={() => toggleMediaMode("video")}
            >
              {props.videoLoading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Video className="h-4 w-4" />
              )}
            </ListenPlayerIconButton>
          ) : null}
        </div>
        <div className="relative z-10 flex min-w-0 items-center justify-end gap-1">
          {showMediaActions ? (
            <>
              <ListenPlayerIconButton
                label={props.text.listen.lyrics}
                active={props.mediaMode === "lyrics"}
                disabled={
                  !props.lyricsAvailable && props.mediaMode !== "lyrics"
                }
                className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
                onClick={() => toggleMediaMode("lyrics")}
              >
                {props.lyricsLoading ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <LyricsIcon className="h-4 w-4" />
                )}
              </ListenPlayerIconButton>
              <ListenPlayerIconButton
                label={props.text.listen.upNext}
                active={props.queueOpen}
                disabled={!props.onToggleQueue}
                className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
                onClick={handleQueueClick}
              >
                <ListMusic className="h-4 w-4" />
              </ListenPlayerIconButton>
            </>
          ) : null}
        </div>
      </div>
    </footer>
  );
}

function resolveObservedPlaybackAudioQualityLabelKey(
  quality: ListenObservedPlaybackAudioQuality | "",
): "" | "low" | "medium" | "high" {
  switch (quality) {
    case "AUDIO_QUALITY_LOW":
      return "low";
    case "AUDIO_QUALITY_HIGH":
      return "high";
    case "AUDIO_QUALITY_MEDIUM":
      return "medium";
    default:
      return "";
  }
}

export function ListenPlayerMoreMenu(props: {
  text: ReturnType<typeof getXiaText>;
  disabled?: boolean;
  onOpenPage: () => void;
  onCopyLink: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          disabled={props.disabled}
          className={cn(LISTEN_PLAYER_ICON_BUTTON_CLASS)}
          aria-label={props.text.listen.more}
        >
          <MoreHorizontal className="h-4 w-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="bottom"
        align="end"
        sideOffset={8}
        className={LISTEN_DROPDOWN_CONTENT_CLASS}
      >
        <div className="grid">
          <DropdownMenuItem
            className={LISTEN_DROPDOWN_ITEM_CLASS}
            disabled={props.disabled}
            onSelect={props.onOpenPage}
          >
            <div className={LISTEN_DROPDOWN_ICON_SLOT_CLASS}>
              <ExternalLink className="h-3.5 w-3.5" />
            </div>
            <span className="truncate">
              {props.text.listen.openPage}
            </span>
          </DropdownMenuItem>
          <DropdownMenuItem
            className={LISTEN_DROPDOWN_ITEM_CLASS}
            disabled={props.disabled}
            onSelect={props.onCopyLink}
          >
            <div className={LISTEN_DROPDOWN_ICON_SLOT_CLASS}>
              <Copy className="h-3.5 w-3.5" />
            </div>
            <span className="truncate">
              {props.text.listen.copyLink}
            </span>
          </DropdownMenuItem>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function ListenPlayerIconButton(props: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  className?: string;
  tooltip?: boolean;
  tooltipSide?: "top" | "bottom" | "left" | "right";
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
}) {
  const button = (
    <span className="wails-no-drag inline-flex">
      <button
        type="button"
        data-active={props.active ? "true" : "false"}
        disabled={props.disabled}
        className={cn(
          LISTEN_PLAYER_ICON_BUTTON_CLASS,
          props.className,
        )}
        aria-label={props.label}
        title={props.tooltip === false ? undefined : props.label}
        onClick={props.onClick}
      >
        {props.children}
      </button>
    </span>
  );

  if (props.tooltip === false) {
    return button;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{button}</TooltipTrigger>
      <TooltipContent side={props.tooltipSide ?? "top"}>
        {props.label}
      </TooltipContent>
    </Tooltip>
  );
}

export function ListenCompactCoverSurface(props: {
  srcCandidates: string[];
  title: string;
}) {
  const {
    activeSrc,
    visibleSrc,
    imageReady,
  } = useListenStableImageSource(props.srcCandidates);

  return (
    <div className="relative h-16 w-16 shrink-0 overflow-hidden rounded-[1.15rem] bg-white shadow-[0_10px_28px_-18px_rgba(15,23,42,0.48)]">
      <img
        key={visibleSrc}
        src={visibleSrc}
        alt={props.title}
        className="block h-full w-full object-cover"
        loading="eager"
      />
      {imageReady ? (
        <span
          key={`compact-cover-sweep-${activeSrc}`}
          className="listen-cover-change-sweep"
          aria-hidden="true"
        />
      ) : null}
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(15,23,42,0.02),rgba(15,23,42,0.12))]" />
      <span
        className="pointer-events-none absolute inset-0 z-30 rounded-[1.15rem] border border-white/50 ring-1 ring-[hsl(var(--foreground)/0.07)]"
        aria-hidden="true"
      />
    </div>
  );
}

export function ListenLocalCoverSurface(props: {
  src: string;
  title: string;
  visualizer?: React.ReactNode;
  visualizerVisible?: boolean;
}) {
  return (
    <ListenArtworkShell
      className="!w-full"
      visualizer={props.visualizer}
      visualizerVisible={props.visualizerVisible}
    >
      <img
        src={props.src}
        alt={props.title}
        className="block h-full w-full object-cover transition-transform duration-500 ease-out"
        loading="eager"
      />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(15,23,42,0.02),rgba(15,23,42,0.12))]" />
    </ListenArtworkShell>
  );
}
