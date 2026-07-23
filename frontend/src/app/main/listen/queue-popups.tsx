import { Check,ChevronDown,ChevronUp,Pencil,Redo2,Trash2,Undo2,X } from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { ListenCoverArtwork } from "@/shared/assets/listen-cover-artwork";
import { LISTEN_PLAYER_ICON_BUTTON_CLASS } from "@/shared/styles/listen";
import { Button } from "@/shared/ui/button";
import { getXiaSurfaceAttributes } from "@/shared/ui/surface-contract";
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
        className="listen-queue-popup-scrim absolute inset-0 z-[var(--app-layer-floating-controls)]"
        onPointerDown={props.onClose}
      />
      <div
        className="listen-queue-popup app-glass-surface app-menu-content app-motion-surface absolute bottom-16 left-1/2 z-[var(--app-layer-popover)] flex max-h-[min(32rem,calc(100%-5.5rem))] w-[min(18rem,calc(100%-1.5rem))] min-w-0 -translate-x-1/2 flex-col p-1.5"
        data-elevation="floating"
        data-shape="panel"
        data-tint="neutral"
        style={anchorStyle}
        {...getXiaSurfaceAttributes("overlay")}
      >
      <div className="flex h-14 shrink-0 items-center justify-between gap-3 px-3">
        <div className="min-w-0 pl-0.5">
          <div className="listen-queue-popup__title truncate">
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
            <span className="listen-queue-popup__meta max-w-[5.75rem] truncate px-1">
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
        <div className="listen-queue-popup__empty flex min-h-0 flex-1 items-center justify-center px-6 pb-3">
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
                  className="listen-queue-popup-row group flex min-h-14 items-center gap-2 px-2 py-2"
                  data-selected={selected ? "true" : undefined}
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
                    className="listen-queue-popup-row__button grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_2.75rem] items-center gap-2"
                    onClick={() => props.onSelectQueueTrack(item)}
                  >
                    <span className="flex min-w-0 items-center gap-2">
                      <ListenQueueArtwork
                        httpBaseURL={props.httpBaseURL}
                        item={item}
                        selected={selected}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="listen-queue-popup-row__title flex min-w-0 items-center gap-1.5">
                          <span className="min-w-0 truncate">{item.title}</span>
                          {hasVideo ? <ListenMuseVideoIndicator /> : null}
                        </span>
                        <span className="listen-queue-popup-row__subtitle block truncate">
                          {artistLabel}
                        </span>
                      </span>
                    </span>
                    <span className="listen-queue-popup-row__duration justify-self-end">
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
        <div className="listen-queue-popup__footer flex shrink-0 flex-wrap items-center justify-start gap-1.5 px-3 py-2">
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
    <Button
      type="button"
      variant="ghost"
      size="compact"
      shape="capsule"
      tone={props.danger ? "destructive" : "neutral"}
      aria-label={props.label}
      title={props.label}
      disabled={props.disabled}
      className="listen-queue-header-action h-7 max-w-full gap-1.5 px-2.5"
      onClick={props.onClick}
    >
      <span className="shrink-0">{props.icon}</span>
      <span className="min-w-0 truncate">{props.label}</span>
    </Button>
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
        <Button
          type="button"
          variant="ghost"
          size="icon"
          shape="circle"
          tone="destructive"
          className="listen-queue-remove-button h-7 w-7"
          aria-label={props.removeLabel}
          title={props.removeLabel}
          onClick={(event) => {
            event.stopPropagation();
            props.onRemove?.();
          }}
        >
          <X className="h-3.5 w-3.5" />
        </Button>
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
      className="listen-queue-leading-slot flex h-7 w-7 shrink-0 items-center justify-center"
      data-selected={props.selected ? "true" : undefined}
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
      className="listen-queue-move-button flex h-3.5 w-5 items-center justify-center"
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
    <span className="listen-queue-playing-indicator flex h-4 items-end justify-center gap-0.5">
      <span className="h-2 w-0.5" />
      <span className="h-3.5 w-0.5" />
      <span className="h-2.5 w-0.5" />
    </span>
  );
}

export function ListenLocalPlaybackQueuePopup(props: {
  anchor?: ListenQueuePopupAnchor | null;
  queueTitle: string;
  queueItems: ListenLocalItem[];
  selectedQueueId: string;
  text: ReturnType<typeof getXiaText>;
  onClearQueue?: () => void;
  onRemoveQueueItem?: (item: ListenLocalItem) => void;
  onMoveQueueItem?: (item: ListenLocalItem, direction: -1 | 1) => void;
  onUndoQueueEdit?: () => void;
  onRedoQueueEdit?: () => void;
  queueCanUndo?: boolean;
  queueCanRedo?: boolean;
  onSelectQueueTrack: (item: ListenLocalItem) => void;
  onClose: () => void;
}) {
  const anchorStyle = resolveListenQueuePopupStyle(props.anchor);
  const [editing, setEditing] = React.useState(false);
  const canEdit = props.queueItems.length > 0 && Boolean(props.onRemoveQueueItem);
  const canClear =
    props.queueItems.length > 0 &&
    Boolean(props.onClearQueue) &&
    props.queueItems.some((item) => item.id !== props.selectedQueueId);
  const queueCountLabel = props.text.listen.playlistTrackCount.replace(
    "{count}",
    String(props.queueItems.length),
  );
  const handleClearQueue = () => {
    if (!canClear) {
      return;
    }
    setEditing(false);
    props.onClearQueue?.();
  };
  const showEditFooter =
    editing &&
    Boolean(props.onUndoQueueEdit || props.onRedoQueueEdit || props.onClearQueue);
  return (
    <>
      <div
        aria-hidden="true"
        className="listen-queue-popup-scrim absolute inset-0 z-[var(--app-layer-floating-controls)]"
        onPointerDown={props.onClose}
      />
      <div
        className="listen-queue-popup app-glass-surface app-menu-content app-motion-surface absolute bottom-16 left-1/2 z-[var(--app-layer-popover)] flex max-h-[min(32rem,calc(100%-5.5rem))] w-[min(18rem,calc(100%-1.5rem))] min-w-0 -translate-x-1/2 flex-col p-1.5"
        data-elevation="floating"
        data-shape="panel"
        data-tint="neutral"
        style={anchorStyle}
        {...getXiaSurfaceAttributes("overlay")}
      >
      <div className="flex h-14 shrink-0 items-center justify-between gap-3 px-3">
        <div className="min-w-0 pl-0.5">
          <div className="listen-queue-popup__title truncate">
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
            <span className="listen-queue-popup__meta max-w-[5.75rem] truncate px-1">
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
          ) : (
            <ListenQueueIconButton
              label={props.text.actions.close}
              className="h-9 w-9"
              onClick={props.onClose}
            >
              <X className="h-4 w-4" />
            </ListenQueueIconButton>
          )}
        </div>
      </div>
      {props.queueItems.length === 0 ? (
        <div className="listen-queue-popup__empty flex min-h-0 flex-1 items-center justify-center px-6 pb-3">
          {props.text.listen.upNextEmpty}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto px-0 pb-0 pt-1">
          <div className="space-y-1.5">
            {props.queueItems.map((item, index) => {
              const selected = item.id === props.selectedQueueId;
              return (
                <div
                  key={item.id}
                  className="listen-queue-popup-row group flex min-h-14 items-center gap-2 px-2 py-2"
                  data-selected={selected ? "true" : undefined}
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
                    className="listen-queue-popup-row__button grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_2.75rem] items-center gap-2"
                    onClick={() => props.onSelectQueueTrack(item)}
                  >
                    <span className="flex min-w-0 items-center gap-2">
                      <ListenLocalQueueArtwork item={item} selected={selected} />
                      <span className="min-w-0 flex-1">
                        <span className="listen-queue-popup-row__title block truncate">
                          {item.title}
                        </span>
                        <span className="listen-queue-popup-row__subtitle block truncate">
                          {item.author}
                        </span>
                      </span>
                    </span>
                    <span className="listen-queue-popup-row__duration justify-self-end">
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
        <div className="listen-queue-popup__footer flex shrink-0 flex-wrap items-center justify-start gap-1.5 px-3 py-2">
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
      <Button
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

function ListenQueueArtwork(props: {
  httpBaseURL: string;
  item: ListenOnlineItem;
  selected: boolean;
}) {
  const posterCandidates = React.useMemo(
    () => buildListenPosterCandidates(props.httpBaseURL, props.item),
    [props.httpBaseURL, props.item.thumbnailUrl, props.item.videoId],
  );

  return (
    <span
      className="listen-queue-popup-artwork relative flex h-10 w-10 shrink-0 overflow-hidden"
      data-selected={props.selected ? "true" : "false"}
    >
      <ListenCoverArtwork
        alt=""
        candidates={posterCandidates}
        className="h-full w-full"
      />
    </span>
  );
}

function ListenLocalQueueArtwork(props: {
  item: ListenLocalItem;
  selected: boolean;
}) {
  return (
    <span
      className="listen-queue-popup-artwork relative flex h-10 w-10 shrink-0 overflow-hidden"
      data-selected={props.selected ? "true" : "false"}
    >
      <ListenCoverArtwork
        alt=""
        candidates={[props.item.coverURL]}
        className="h-full w-full"
      />
    </span>
  );
}
