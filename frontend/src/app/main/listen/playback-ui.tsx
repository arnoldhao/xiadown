import {
  Airplay,
  Copy,
  ExternalLink,
  Fullscreen,
  ListMusic,
  Loader2,
  Maximize2,
  MoreHorizontal,
  Shrink,
  Video,
  Volume2,
  VolumeX,
} from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { ListenCoverArtwork } from "@/shared/assets/listen-cover-artwork";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { GlassGroup } from "@/shared/ui/glass-surface";
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
import type { ListenLyricsKind,ListenPlayerPresentation } from "@/app/main/listen/types";
import {
  ListenArtworkShell,
} from "@/app/main/listen/ui";

export type ListenMediaMode = "cover" | "lyrics" | "video";

export type ListenAirPlayAnchor = {
  x: number;
  y: number;
  width: number;
  height: number;
};

function ListenPlayerFooterPill({
  workspaceFullscreen,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & {
  workspaceFullscreen: boolean;
}) {
  if (!workspaceFullscreen) {
    return <div {...props} />;
  }
  return (
    <GlassGroup
      {...props}
      elevation="floating"
      shape="capsule"
      surfaceRole="control"
    />
  );
}

export function ListenPlayerFooter(props: {
  mediaMode: ListenMediaMode;
  presentation?: ListenPlayerPresentation;
  workspaceFullscreen?: boolean;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  sourceBadge?: React.ReactNode;
  sourceLabel?: string;
  leading?: React.ReactNode;
  fullscreenTransport?: React.ReactNode;
  hasVideo: boolean;
  videoHidden?: boolean;
  videoLoading?: boolean;
  live?: boolean;
  lyricsAvailable: boolean;
  lyricsKind?: ListenLyricsKind;
  lyricsLoading?: boolean;
  showMediaActions?: boolean;
  queueOpen?: boolean;
  muted?: boolean;
  text: ReturnType<typeof getXiaText>;
  onAirPlay?: (anchor: ListenAirPlayAnchor) => void;
  onMediaModeChange: (mode: ListenMediaMode) => void;
  onToggleQueue?: (anchor: ListenQueuePopupAnchor) => void;
  onToggleMute?: () => void;
  onOpenSource?: () => void;
  lyricsControls?: React.ReactNode;
  companionControls?: React.ReactNode;
  videoAppFullscreen?: boolean;
  onToggleVideoAppFullscreen?: () => void;
  onRequestVideoFullscreen?: () => void;
  onRequestFullscreen?: () => void;
}) {
  const presentation = props.presentation ??
    (props.workspaceFullscreen ? "fullscreen" : "page");
  const workspaceCompanion = presentation === "companion";
  const workspaceFullscreen = presentation === "fullscreen";
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
  const showVideoAction =
    !props.videoHidden && workspaceFullscreen;
  const showLyricsAction = props.showMediaActions !== false && !props.live;
  const showQueueAction =
    props.showMediaActions !== false &&
    !props.live &&
    (!workspaceCompanion || Boolean(props.onToggleQueue));
  const videoModeActive = props.mediaMode === "video";
  const videoActionLoading = props.videoLoading === true && !videoModeActive;
  const videoActionDisabled =
    videoActionLoading || (!props.hasVideo && !videoModeActive);
  const handleQueueClick: React.MouseEventHandler<HTMLButtonElement> = (event) => {
    props.onToggleQueue?.(resolveListenQueuePopupAnchor(event.currentTarget));
  };
  const sourceBadgeClassName = cn(
    "listen-player-source-badge flex min-w-0 select-none items-center gap-1.5 overflow-hidden whitespace-nowrap [&>svg]:h-3 [&>svg]:w-3 [&>svg]:shrink-0",
    workspaceCompanion
      ? "listen-player-footer__source relative z-10 max-w-[11rem]"
      : "listen-source-watermark pointer-events-none absolute left-1/2 top-1/2 z-0 max-w-[9rem] -translate-x-1/2 -translate-y-1/2 justify-center",
  );
  const showLyricsControls =
    !props.queueOpen &&
    props.mediaMode === "lyrics" &&
    Boolean(props.lyricsControls);
  const companionControls =
    props.companionControls ??
    (workspaceCompanion && showLyricsControls ? props.lyricsControls : undefined);
  const showFullscreenContext =
    workspaceFullscreen &&
    (showLyricsControls ||
      (videoModeActive &&
        Boolean(
          props.onToggleVideoAppFullscreen || props.onRequestVideoFullscreen,
        )));
  const footerGroupClassName = cn(
    "listen-player-footer__bar relative flex h-12 w-full items-center gap-3 px-3",
    workspaceFullscreen
      ? "justify-end px-1.5"
      : "justify-between",
    workspaceCompanion && "listen-player-footer__bar--companion",
  );
  const sourceControl = props.sourceBadge ? (
    workspaceCompanion && props.sourceLabel ? (
      <ListenPlayerIconButton
        label={props.sourceLabel}
        disabled={!props.onOpenSource}
        className={cn(
          LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS,
          "listen-player-footer__source-button",
        )}
        onClick={props.onOpenSource}
      >
        {props.sourceBadge}
      </ListenPlayerIconButton>
    ) : (
      <div aria-hidden="true" className={sourceBadgeClassName}>
        {props.sourceBadge}
      </div>
    )
  ) : null;

  return (
    <footer
      data-media-chrome={props.videoAppFullscreen ? "dark" : undefined}
      data-video-transport={props.fullscreenTransport ? "true" : undefined}
      className={cn(
        "z-20",
        workspaceFullscreen
          ? "listen-workspace-fullscreen-player__footer absolute bottom-4 right-5 flex items-center gap-2"
          : "relative shrink-0 px-0 pb-1 pt-2 sm:pb-2",
      )}
    >
      {props.fullscreenTransport ? (
        <ListenPlayerFooterPill
          workspaceFullscreen={workspaceFullscreen}
          className={cn(
            footerGroupClassName,
            "listen-player-footer__transport-group min-w-0 flex-1 justify-start",
          )}
          role="group"
          aria-label={props.text.listen.nowPlaying}
        >
          {props.fullscreenTransport}
        </ListenPlayerFooterPill>
      ) : null}
      {showFullscreenContext ? (
        <ListenPlayerFooterPill
          workspaceFullscreen={workspaceFullscreen}
          className={cn(
            footerGroupClassName,
            "listen-player-footer__context-group shrink-0",
          )}
          role="group"
          aria-label={
            showLyricsControls
              ? props.text.listen.lyricsSettings
              : props.text.listen.video
          }
        >
          {showLyricsControls ? props.lyricsControls : null}
          {videoModeActive && props.onToggleVideoAppFullscreen ? (
            <ListenPlayerIconButton
              label={
                props.videoAppFullscreen
                  ? props.text.listen.windowFullscreenExit
                  : props.text.listen.windowFullscreenEnter
              }
              active={props.videoAppFullscreen}
              className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
              onClick={props.onToggleVideoAppFullscreen}
            >
              {props.videoAppFullscreen ? (
                <Shrink className="h-4 w-4" />
              ) : (
                <Maximize2 className="h-4 w-4" />
              )}
            </ListenPlayerIconButton>
          ) : null}
          {videoModeActive && props.onRequestVideoFullscreen ? (
            <ListenPlayerIconButton
              label={props.text.completed.previewEnterFullscreen}
              className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
              onClick={props.onRequestVideoFullscreen}
            >
              <Fullscreen className="h-4 w-4" />
            </ListenPlayerIconButton>
          ) : null}
        </ListenPlayerFooterPill>
      ) : null}
      <ListenPlayerFooterPill
        workspaceFullscreen={workspaceFullscreen}
        className={footerGroupClassName}
      >
        {workspaceCompanion ? (
          <>
            <div
              data-footer-region="leading"
              className="listen-player-footer__companion-leading relative z-10 flex min-w-0 shrink-0 items-center gap-1"
            >
              {sourceControl}
              {props.onRequestFullscreen ? (
                <ListenPlayerIconButton
                  label={props.text.completed.previewEnterFullscreen}
                  className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
                  onClick={props.onRequestFullscreen}
                >
                  <Fullscreen className="h-4 w-4" />
                </ListenPlayerIconButton>
              ) : null}
              {props.leading}
            </div>
            {companionControls ? (
              <div
                data-footer-region="dynamic"
                className="listen-player-footer__center-context absolute left-1/2 top-1/2 z-20 -translate-x-1/2 -translate-y-1/2"
              >
                {companionControls}
              </div>
            ) : null}
            <div
              data-footer-region="trailing"
              className="listen-player-footer__actions relative z-10 flex min-w-0 shrink-0 items-center justify-end gap-1"
            >
              {showLyricsAction ? (
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
                    <Loader2 className="h-4 w-4 listen-loading-spinner" />
                  ) : (
                    <LyricsIcon className="h-4 w-4" />
                  )}
                </ListenPlayerIconButton>
              ) : null}
              {showQueueAction ? (
                <ListenPlayerIconButton
                  label={props.text.listen.upNext}
                  active={props.queueOpen}
                  disabled={!props.onToggleQueue}
                  className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
                  onClick={handleQueueClick}
                >
                  <ListMusic className="h-4 w-4" />
                </ListenPlayerIconButton>
              ) : null}
            </div>
          </>
        ) : (
          sourceControl
        )}
        {presentation !== "page" ? null : (
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
                  <Loader2 className="h-4 w-4 listen-loading-spinner" />
                ) : (
                  <Video className="h-4 w-4" />
                )}
              </ListenPlayerIconButton>
            ) : null}
          </div>
        )}
        {!workspaceCompanion && props.leading ? (
          <div className="listen-player-footer__leading relative z-10 flex min-w-0 items-center">
            {props.leading}
          </div>
        ) : null}
        {!workspaceCompanion ? (
          <div className="listen-player-footer__actions relative z-10 flex min-w-0 shrink-0 items-center justify-end gap-1">
            {workspaceFullscreen && showVideoAction ? (
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
                  <Loader2 className="h-4 w-4 listen-loading-spinner" />
                ) : (
                  <Video className="h-4 w-4" />
                )}
              </ListenPlayerIconButton>
            ) : null}
            {showLyricsAction ? (
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
                  <Loader2 className="h-4 w-4 listen-loading-spinner" />
                ) : (
                  <LyricsIcon className="h-4 w-4" />
                )}
              </ListenPlayerIconButton>
            ) : null}
            {showQueueAction ? (
              <ListenPlayerIconButton
                label={props.text.listen.upNext}
                active={props.queueOpen}
                disabled={!props.onToggleQueue}
                className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
                onClick={handleQueueClick}
              >
                <ListMusic className="h-4 w-4" />
              </ListenPlayerIconButton>
            ) : null}
            {workspaceFullscreen && props.onToggleMute ? (
              <ListenPlayerIconButton
                label={
                  props.muted
                    ? props.text.listen.unmute
                    : props.text.listen.mute
                }
                active={props.muted}
                className={LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS}
                onClick={props.onToggleMute}
              >
                {props.muted ? (
                  <VolumeX className="h-4 w-4" />
                ) : (
                  <Volume2 className="h-4 w-4" />
                )}
              </ListenPlayerIconButton>
            ) : null}
          </div>
        ) : null}
      </ListenPlayerFooterPill>
    </footer>
  );
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
        <Button
          type="button"
          variant="ghost"
          size="icon"
          shape="circle"
          disabled={props.disabled}
          className={cn(LISTEN_PLAYER_ICON_BUTTON_CLASS)}
          aria-label={props.text.listen.more}
        >
          <MoreHorizontal className="h-4 w-4" />
        </Button>
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
  buttonRef?: React.Ref<HTMLButtonElement>;
  active?: boolean;
  disabled?: boolean;
  className?: string;
  wrapperClassName?: string;
  tooltip?: boolean;
  tooltipSide?: "top" | "bottom" | "left" | "right";
  "aria-haspopup"?: React.AriaAttributes["aria-haspopup"];
  "aria-expanded"?: boolean;
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
}) {
  const button = (
    <span className={cn("wails-no-drag inline-flex", props.wrapperClassName)}>
      <Button
        ref={props.buttonRef}
        type="button"
        variant="ghost"
        size="icon"
        shape="circle"
        data-active={props.active ? "true" : "false"}
        disabled={props.disabled}
        className={cn(
          LISTEN_PLAYER_ICON_BUTTON_CLASS,
          props.className,
        )}
        aria-label={props.label}
        aria-haspopup={props["aria-haspopup"]}
        aria-expanded={props["aria-expanded"]}
        title={props.tooltip === false ? undefined : props.label}
        onClick={props.onClick}
      >
        {props.children}
      </Button>
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
  return (
    <div className="listen-compact-cover-surface relative h-16 w-16 shrink-0 overflow-hidden">
      <ListenCoverArtwork
        alt={props.title}
        candidates={props.srcCandidates}
        className="h-full w-full"
        changeSweep
      />
      <div className="listen-cover-artwork-wash pointer-events-none absolute inset-0" />
      <span
        className="listen-compact-cover-surface__rim pointer-events-none absolute inset-0 z-30"
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
      <ListenCoverArtwork
        alt={props.title}
        candidates={[props.src]}
        className="h-full w-full"
        imageClassName="listen-cover-artwork-motion-image"
        changeSweep
      />
      <div className="listen-cover-artwork-wash pointer-events-none absolute inset-0" />
    </ListenArtworkShell>
  );
}
