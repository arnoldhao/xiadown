import { Check,ChevronDown,ChevronUp,Pencil,Redo2,Trash2,Undo2,X } from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import { LISTEN_PLAYER_ICON_BUTTON_CLASS } from "@/shared/styles/listen";
import { Tooltip,TooltipContent,TooltipTrigger } from "@/shared/ui/tooltip";
import { resolveTrustedListenOnlineArtistLabel } from "@/app/main/listen/playback-helpers";
import { buildListenPosterCandidates } from "@/app/main/listen/storage";
import type { ListenLocalItem,ListenOnlineItem } from "@/app/main/listen/types";
import { hasListenMuseItemVideo,ListenMuseVideoIndicator } from "@/app/main/listen/ui";

export type ListenQueuePopupAnchor = {
  x: number;
  y: number;
  width: number;
  height: number;
  rootWidth: number;
};

export function ListenPlaybackQueuePopup(props: {
  anchor?: ListenQueuePopupAnchor | null;
  queueTitle: string;
  queueItems: ListenOnlineItem[];
  selectedQueueId: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  onClearQueue?: () => void;
  onRemoveQueueItem?: (item: ListenOnlineItem) => void;
  onMoveQueueItem?: (item: ListenOnlineItem, direction: -1 | 1) => void;
  onUndoQueueEdit?: () => void;
  onRedoQueueEdit?: () => void;
  queueCanUndo?: boolean;
  queueCanRedo?: boolean;
  onSelectQueueTrack: (item: ListenOnlineItem) => void;
  onClose: () => void;
}) {
  const anchorStyle = resolveListenQueuePopupStyle(props.anchor);
  const [editing, setEditing] = React.useState(false);
  const canEdit = props.queueItems.length > 0 && Boolean(props.onRemoveQueueItem);
  const canClear =
    props.queueItems.length > 0 &&
    Boolean(props.onClearQueue) &&
    props.queueItems.some((item) => item.id !== props.selectedQueueId);
  const handleClearQueue = () => {
    if (!canClear) {
      return;
    }
    setEditing(false);
    props.onClearQueue?.();
  };
  const queueCountLabel = props.text.listen.playlistTrackCount.replace(
    "{count}",
    String(props.queueItems.length),
  );
  const showEditFooter =
    editing &&
    Boolean(props.onUndoQueueEdit || props.onRedoQueueEdit || props.onClearQueue);

  return (
    <>
      <div
        aria-hidden="true"
        className="absolute inset-0 z-[25] cursor-default"
        onPointerDown={props.onClose}
      />
      <div
        className="app-menu-content app-motion-surface absolute bottom-16 left-1/2 z-30 flex max-h-[min(32rem,calc(100%-5.5rem))] w-[min(18rem,calc(100%-1.5rem))] min-w-0 -translate-x-1/2 flex-col rounded-[1.35rem] p-1.5 animate-in fade-in-0 slide-in-from-bottom-2 zoom-in-95 duration-200"
        style={anchorStyle}
      >
      <div className="flex h-14 shrink-0 items-center justify-between gap-3 px-3">
        <div className="min-w-0 pl-0.5">
          <div className="truncate text-sm font-semibold text-sidebar-foreground">
            {props.text.listen.upNext}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {!editing && props.onClearQueue ? (
            <ListenQueueHeaderAction
              label={props.text.actions.clear}
              icon={<Trash2 className="h-3.5 w-3.5" />}
              danger
              disabled={!canClear}
              onClick={handleClearQueue}
            />
          ) : null}
          {editing ? (
            <span className="max-w-[5.75rem] truncate px-1 text-[11px] font-semibold text-sidebar-foreground/46">
              {queueCountLabel}
            </span>
          ) : null}
          {props.onRemoveQueueItem ? (
            <ListenQueueHeaderAction
              label={editing ? props.text.listen.doneQueue : props.text.listen.editQueue}
              icon={
                editing ? (
                  <Check className="h-3.5 w-3.5" />
                ) : (
                  <Pencil className="h-3.5 w-3.5" />
                )
              }
              disabled={!canEdit}
              onClick={() => setEditing((current) => !current)}
            />
          ) : null}
        </div>
      </div>
      {props.queueItems.length === 0 ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 pb-3 text-center text-sm text-sidebar-foreground/58">
          {props.text.listen.upNextEmpty}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto px-0 pb-0 pt-1">
          <div className="space-y-1.5">
            {props.queueItems.map((item, index) => {
              const selected = item.id === props.selectedQueueId;
              const artistLabel = resolveListenUpNextArtistLabel(item);
              const hasVideo = hasListenMuseItemVideo(item);
              return (
                <div
                  key={item.id}
                  className={cn(
                    "group flex min-h-14 items-center gap-2 rounded-2xl border border-transparent px-2 py-2 transition-[transform,background-color,border-color] duration-200 ease-out active:scale-[0.99]",
                    selected
                      ? "border-sidebar-primary/18 bg-sidebar-primary/10"
                      : "hover:-translate-y-0.5 hover:bg-sidebar-background/54",
                  )}
                >
                  <ListenQueueLeadingSlot
                    index={index}
                    editing={editing}
                    selected={selected}
                    removeLabel={props.text.listen.removeFromQueue}
                    moveUpLabel={props.text.listen.moveQueueItemUp}
                    moveDownLabel={props.text.listen.moveQueueItemDown}
                    onRemove={
                      props.onRemoveQueueItem
                        ? () => props.onRemoveQueueItem?.(item)
                        : undefined
                    }
                    onMoveUp={
                      props.onMoveQueueItem && index > 0
                        ? () => props.onMoveQueueItem?.(item, -1)
                        : undefined
                    }
                    onMoveDown={
                      props.onMoveQueueItem && index < props.queueItems.length - 1
                        ? () => props.onMoveQueueItem?.(item, 1)
                        : undefined
                    }
                  />
                  <button
                    type="button"
                    className="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_2.75rem] items-center gap-2 text-left focus-visible:outline-none"
                    onClick={() => props.onSelectQueueTrack(item)}
                  >
                    <span className="flex min-w-0 items-center gap-2">
                      <ListenQueueArtwork
                        httpBaseURL={props.httpBaseURL}
                        item={item}
                        selected={selected}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="flex min-w-0 items-center gap-1.5 text-sm font-medium text-sidebar-foreground">
                          <span className="min-w-0 truncate">{item.title}</span>
                          {hasVideo ? <ListenMuseVideoIndicator /> : null}
                        </span>
                        <span className="block truncate text-xs text-sidebar-foreground/58">
                          {artistLabel}
                        </span>
                      </span>
                    </span>
                    <span className="justify-self-end text-right text-[11px] font-medium tabular-nums text-sidebar-foreground/42">
                      {item.durationLabel}
                    </span>
                  </button>
                </div>
              );
            })}
          </div>
        </div>
      )}
      {showEditFooter ? (
        <div className="flex shrink-0 flex-wrap items-center justify-start gap-1.5 border-t border-sidebar-border/35 px-3 py-2">
          {props.onUndoQueueEdit ? (
            <ListenQueueHeaderAction
              label={props.text.listen.undoQueue}
              icon={<Undo2 className="h-3.5 w-3.5" />}
              disabled={!props.queueCanUndo}
              onClick={props.onUndoQueueEdit}
            />
          ) : null}
          {props.onRedoQueueEdit ? (
            <ListenQueueHeaderAction
              label={props.text.listen.redoQueue}
              icon={<Redo2 className="h-3.5 w-3.5" />}
              disabled={!props.queueCanRedo}
              onClick={props.onRedoQueueEdit}
            />
          ) : null}
          {props.onClearQueue ? (
            <ListenQueueHeaderAction
              label={props.text.actions.clear}
              icon={<Trash2 className="h-3.5 w-3.5" />}
              danger
              disabled={!canClear}
              onClick={handleClearQueue}
            />
          ) : null}
        </div>
      ) : null}
      </div>
    </>
  );
}

function resolveListenUpNextArtistLabel(item: ListenOnlineItem) {
  let artist = resolveTrustedListenOnlineArtistLabel(item);
  if (artist.startsWith("Album, ")) {
    artist = artist.slice(7).trim();
  }
  const normalized = artist.toLowerCase();
  if (
    !artist ||
    normalized === "album" ||
    normalized === "youtube" ||
    normalized === "youtube music" ||
    normalized === "unknown artist"
  ) {
    return "";
  }
  return artist;
}

function ListenQueueHeaderAction(props: {
  label: string;
  icon: React.ReactNode;
  danger?: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={props.label}
      title={props.label}
      disabled={props.disabled}
      className={cn(
        "inline-flex h-7 max-w-full items-center justify-center gap-1.5 rounded-full px-2.5 text-xs font-semibold transition-[background-color,color,opacity] duration-150 ease-out focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sidebar-primary/35 disabled:pointer-events-none disabled:opacity-35",
        props.danger
          ? "text-destructive hover:bg-destructive/10"
          : "text-sidebar-foreground/62 hover:bg-sidebar-background/54 hover:text-sidebar-foreground",
      )}
      onClick={props.onClick}
    >
      <span className="shrink-0">{props.icon}</span>
      <span className="min-w-0 truncate">{props.label}</span>
    </button>
  );
}

function resolveListenQueuePopupStyle(
  anchor: ListenQueuePopupAnchor | null | undefined,
): React.CSSProperties | undefined {
  if (!anchor || anchor.rootWidth < 560) {
    return undefined;
  }
  const centerX = Math.round((anchor.x + anchor.width / 2) * 10) / 10;
  return {
    left: `clamp(9.75rem, ${centerX}px, calc(100% - 9.75rem))`,
  };
}

function ListenQueueLeadingSlot(props: {
  index: number;
  editing: boolean;
  selected: boolean;
  removeLabel: string;
  moveUpLabel: string;
  moveDownLabel: string;
  onRemove?: () => void;
  onMoveUp?: () => void;
  onMoveDown?: () => void;
}) {
  if (props.editing && props.onRemove) {
    return (
      <span className="flex shrink-0 items-center gap-1">
        <button
          type="button"
          className="flex h-7 w-7 items-center justify-center rounded-full text-destructive transition hover:bg-destructive/10 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-destructive/35"
          aria-label={props.removeLabel}
          title={props.removeLabel}
          onClick={(event) => {
            event.stopPropagation();
            props.onRemove?.();
          }}
        >
          <X className="h-3.5 w-3.5" />
        </button>
        <span className="flex flex-col gap-0.5">
          <ListenQueueMoveIconButton
            label={props.moveUpLabel}
            disabled={!props.onMoveUp}
            onClick={props.onMoveUp}
          >
            <ChevronUp className="h-3 w-3" />
          </ListenQueueMoveIconButton>
          <ListenQueueMoveIconButton
            label={props.moveDownLabel}
            disabled={!props.onMoveDown}
            onClick={props.onMoveDown}
          >
            <ChevronDown className="h-3 w-3" />
          </ListenQueueMoveIconButton>
        </span>
      </span>
    );
  }

  return (
    <span
      className={cn(
        "flex h-7 w-7 shrink-0 items-center justify-center text-[11px] font-semibold tabular-nums",
        props.selected ? "text-sidebar-primary" : "text-sidebar-foreground/38",
      )}
      aria-hidden="true"
    >
      {props.selected ? <ListenQueuePlayingIndicator /> : props.index + 1}
    </span>
  );
}

function ListenQueueMoveIconButton(props: {
  label: string;
  disabled?: boolean;
  children: React.ReactNode;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={props.label}
      title={props.label}
      disabled={props.disabled}
      className="flex h-3.5 w-5 items-center justify-center rounded-full text-sidebar-foreground/50 transition hover:bg-sidebar-background/54 hover:text-sidebar-foreground focus-visible:outline-none disabled:pointer-events-none disabled:opacity-25"
      onClick={(event) => {
        event.stopPropagation();
        props.onClick?.();
      }}
    >
      {props.children}
    </button>
  );
}

function ListenQueuePlayingIndicator() {
  return (
    <span className="flex h-4 items-end justify-center gap-0.5">
      <span className="h-2 w-0.5 animate-pulse rounded-full bg-current [animation-delay:-240ms]" />
      <span className="h-3.5 w-0.5 animate-pulse rounded-full bg-current [animation-delay:-120ms]" />
      <span className="h-2.5 w-0.5 animate-pulse rounded-full bg-current" />
    </span>
  );
}

export function ListenLocalPlaybackQueuePopup(props: {
  anchor?: ListenQueuePopupAnchor | null;
  queueTitle: string;
  queueItems: ListenLocalItem[];
  selectedQueueId: string;
  text: ReturnType<typeof getXiaText>;
  onSelectQueueTrack: (item: ListenLocalItem) => void;
  onClose: () => void;
}) {
  const anchorStyle = resolveListenQueuePopupStyle(props.anchor);
  return (
    <>
      <div
        aria-hidden="true"
        className="absolute inset-0 z-[25] cursor-default"
        onPointerDown={props.onClose}
      />
      <div
        className="app-menu-content app-motion-surface absolute bottom-16 left-1/2 z-30 flex max-h-[min(32rem,calc(100%-5.5rem))] w-[min(18rem,calc(100%-1.5rem))] min-w-0 -translate-x-1/2 flex-col rounded-[1.35rem] p-1.5 animate-in fade-in-0 slide-in-from-bottom-2 zoom-in-95 duration-200"
        style={anchorStyle}
      >
      <div className="flex h-14 shrink-0 items-center justify-between gap-3 px-3">
        <div className="min-w-0 pl-0.5">
          <div className="truncate text-sm font-semibold text-sidebar-foreground">
            {props.text.listen.upNext}
          </div>
        </div>
        <ListenQueueIconButton
          label={props.text.actions.close}
          className="h-9 w-9"
          onClick={props.onClose}
        >
          <X className="h-4 w-4" />
        </ListenQueueIconButton>
      </div>
      {props.queueItems.length === 0 ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 pb-3 text-center text-sm text-sidebar-foreground/58">
          {props.text.listen.upNextEmpty}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto px-0 pb-0 pt-1">
          <div className="space-y-1.5">
            {props.queueItems.map((item) => {
              const selected = item.id === props.selectedQueueId;
              return (
                <button
                  key={item.id}
                  type="button"
                  className={cn(
                    "grid min-h-14 w-full grid-cols-[minmax(0,1fr)_2.75rem] items-center gap-2 rounded-2xl border border-transparent px-2 py-2 text-left transition-[transform,background-color,border-color] duration-200 ease-out active:scale-[0.99] focus-visible:outline-none",
                    selected
                      ? "border-sidebar-primary/18 bg-sidebar-primary/10"
                      : "hover:-translate-y-0.5 hover:bg-sidebar-background/54",
                  )}
                  onClick={() => props.onSelectQueueTrack(item)}
                >
                  <span className="flex min-w-0 items-center gap-2">
                    <ListenLocalQueueArtwork item={item} selected={selected} />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-sidebar-foreground">
                        {item.title}
                      </span>
                      <span className="block truncate text-xs text-sidebar-foreground/58">
                        {item.author}
                      </span>
                    </span>
                  </span>
                  <span className="justify-self-end text-right text-[11px] font-medium tabular-nums text-sidebar-foreground/42">
                    {item.durationLabel}
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      )}
      </div>
    </>
  );
}

function ListenQueueIconButton(props: {
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

function ListenQueueArtwork(props: {
  httpBaseURL: string;
  item: ListenOnlineItem;
  selected: boolean;
}) {
  const posterCandidates = React.useMemo(
    () => buildListenPosterCandidates(props.httpBaseURL, props.item),
    [props.httpBaseURL, props.item.thumbnailUrl, props.item.videoId],
  );
  const posterCandidateKey = posterCandidates.join("\n");
  const [posterIndex, setPosterIndex] = React.useState(0);
  const activePoster =
    posterCandidates[
      Math.min(posterIndex, Math.max(posterCandidates.length - 1, 0))
    ] || LISTEN_DEFAULT_COVER_IMAGE_URL;

  React.useEffect(() => {
    setPosterIndex(0);
  }, [posterCandidateKey]);

  return (
    <span
      className={cn(
        "relative flex h-10 w-10 shrink-0 overflow-hidden rounded-xl bg-muted ring-1 ring-border/70",
        props.selected && "ring-primary/30",
      )}
    >
      <img
        key={activePoster}
        src={activePoster}
        alt=""
        className="h-full w-full object-cover"
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
    </span>
  );
}

function ListenLocalQueueArtwork(props: {
  item: ListenLocalItem;
  selected: boolean;
}) {
  const [coverFailed, setCoverFailed] = React.useState(false);
  const coverURL =
    !coverFailed && props.item.coverURL
      ? props.item.coverURL
      : LISTEN_DEFAULT_COVER_IMAGE_URL;

  React.useEffect(() => {
    setCoverFailed(false);
  }, [props.item.coverURL]);

  return (
    <span
      className={cn(
        "relative flex h-10 w-10 shrink-0 overflow-hidden rounded-xl bg-muted ring-1 ring-border/70",
        props.selected && "ring-primary/30",
      )}
    >
      <img
        src={coverURL}
        alt=""
        className="h-full w-full object-cover"
        loading="eager"
        onError={() => setCoverFailed(true)}
      />
    </span>
  );
}
