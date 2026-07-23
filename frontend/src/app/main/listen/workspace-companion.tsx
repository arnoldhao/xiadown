import {
  Ellipsis,
  ListMusic,
  Redo2,
  Repeat1,
  Shuffle,
  Trash2,
  Undo2,
} from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import { ListenCoverArtwork } from "@/shared/assets/listen-cover-artwork";
import {
  LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS,
  LISTEN_PLAYER_ICON_BUTTON_CLASS,
} from "@/shared/styles/listen";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";

import { resolveTrustedListenOnlineArtistLabel } from "@/app/main/listen/playback-helpers";
import { ListenPlayerFooter } from "@/app/main/listen/playback-ui";
import { buildListenPosterCandidates } from "@/app/main/listen/storage";
import type {
  ListenLocalItem,
  ListenOnlineItem,
  ListenPlayMode,
  ListenPlayerCompanionMode,
} from "@/app/main/listen/types";

export type { ListenPlayerCompanionMode };

type ListenWorkspaceCompanionText = ReturnType<typeof getXiaText>;

export function ListenWorkspaceLyricsCompanion(props: {
  artworkCandidates: string[];
  title: string;
  text: ListenWorkspaceCompanionText;
  lyricsControls?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section
      data-listen-companion-mode="lyrics"
      className="listen-workspace-companion-player relative flex h-full min-h-0 flex-col overflow-hidden"
    >
      <div className="listen-workspace-lyrics-content min-h-0 flex-1 overflow-hidden px-0 pb-1 pt-2">
        {props.children}
      </div>
      <ListenPlayerFooter
        mediaMode="lyrics"
        presentation="companion"
        reserveWindowControls={false}
        airPlaySupported={false}
        hasVideo={false}
        lyricsAvailable
        showMediaActions={false}
        text={props.text}
        onMediaModeChange={() => undefined}
        companionControls={props.lyricsControls}
      />
    </section>
  );
}

export function ListenWorkspaceOnlineQueueCompanion(props: {
  queueTitle: string;
  queueItems: ListenOnlineItem[];
  selectedQueueId: string;
  httpBaseURL: string;
  playMode: ListenPlayMode;
  text: ListenWorkspaceCompanionText;
  onPlayModeChange: (mode: ListenPlayMode) => void;
  onClearQueue?: () => void;
  onRemoveQueueItem?: (item: ListenOnlineItem) => void;
  onMoveQueueItem?: (item: ListenOnlineItem, direction: -1 | 1) => void;
  onUndoQueueEdit?: () => void;
  onRedoQueueEdit?: () => void;
  queueCanUndo?: boolean;
  queueCanRedo?: boolean;
  showFooter?: boolean;
  onSelectQueueTrack: (item: ListenOnlineItem) => void;
}) {
  const entries = props.queueItems.map<ListenWorkspaceQueueEntry>(
    (item, index) => ({
      id: item.id,
      title: item.title,
      artist: resolveListenWorkspaceQueueArtist(item),
      durationLabel: item.durationLabel,
      artworkCandidates: buildListenPosterCandidates(props.httpBaseURL, item),
      selected: item.id === props.selectedQueueId,
      onSelect: () => props.onSelectQueueTrack(item),
      onRemove: props.onRemoveQueueItem
        ? () => props.onRemoveQueueItem?.(item)
        : undefined,
      onMoveUp:
        props.onMoveQueueItem && index > 0
          ? () => props.onMoveQueueItem?.(item, -1)
          : undefined,
      onMoveDown:
        props.onMoveQueueItem && index < props.queueItems.length - 1
          ? () => props.onMoveQueueItem?.(item, 1)
          : undefined,
    }),
  );
  const canClear =
    Boolean(props.onClearQueue) &&
    entries.some((entry) => !entry.selected);

  return (
    <ListenWorkspaceQueueCompanion
      entries={entries}
      playMode={props.playMode}
      text={props.text}
      canClear={canClear}
      onClearQueue={props.onClearQueue}
      onPlayModeChange={props.onPlayModeChange}
      onUndoQueueEdit={props.onUndoQueueEdit}
      onRedoQueueEdit={props.onRedoQueueEdit}
      queueCanUndo={props.queueCanUndo}
      queueCanRedo={props.queueCanRedo}
      showFooter={props.showFooter}
    />
  );
}

export function ListenWorkspaceLocalQueueCompanion(props: {
  queueTitle: string;
  queueItems: ListenLocalItem[];
  selectedQueueId: string;
  playMode: ListenPlayMode;
  text: ListenWorkspaceCompanionText;
  onPlayModeChange: (mode: ListenPlayMode) => void;
  onClearQueue?: () => void;
  onRemoveQueueItem?: (item: ListenLocalItem) => void;
  onMoveQueueItem?: (item: ListenLocalItem, direction: -1 | 1) => void;
  onUndoQueueEdit?: () => void;
  onRedoQueueEdit?: () => void;
  queueCanUndo?: boolean;
  queueCanRedo?: boolean;
  showFooter?: boolean;
  onSelectQueueTrack: (item: ListenLocalItem) => void;
}) {
  const entries = props.queueItems.map<ListenWorkspaceQueueEntry>(
    (item, index) => ({
      id: item.id,
      title: item.title,
      artist: item.author,
      durationLabel: item.durationLabel,
      artworkCandidates: [
        item.coverURL || LISTEN_DEFAULT_COVER_IMAGE_URL,
        LISTEN_DEFAULT_COVER_IMAGE_URL,
      ],
      selected: item.id === props.selectedQueueId,
      onSelect: () => props.onSelectQueueTrack(item),
      onRemove: props.onRemoveQueueItem
        ? () => props.onRemoveQueueItem?.(item)
        : undefined,
      onMoveUp:
        props.onMoveQueueItem && index > 0
          ? () => props.onMoveQueueItem?.(item, -1)
          : undefined,
      onMoveDown:
        props.onMoveQueueItem && index < props.queueItems.length - 1
          ? () => props.onMoveQueueItem?.(item, 1)
          : undefined,
    }),
  );
  const canClear =
    Boolean(props.onClearQueue) &&
    entries.some((entry) => !entry.selected);

  return (
    <ListenWorkspaceQueueCompanion
      entries={entries}
      playMode={props.playMode}
      text={props.text}
      canClear={canClear}
      onClearQueue={props.onClearQueue}
      onPlayModeChange={props.onPlayModeChange}
      onUndoQueueEdit={props.onUndoQueueEdit}
      onRedoQueueEdit={props.onRedoQueueEdit}
      queueCanUndo={props.queueCanUndo}
      queueCanRedo={props.queueCanRedo}
      showFooter={props.showFooter}
    />
  );
}

type ListenWorkspaceQueueEntry = {
  id: string;
  title: string;
  artist: string;
  durationLabel: string;
  artworkCandidates: string[];
  selected: boolean;
  onSelect: () => void;
  onRemove?: () => void;
  onMoveUp?: () => void;
  onMoveDown?: () => void;
};

function ListenWorkspaceQueueCompanion(props: {
  entries: ListenWorkspaceQueueEntry[];
  playMode: ListenPlayMode;
  text: ListenWorkspaceCompanionText;
  canClear?: boolean;
  onClearQueue?: () => void;
  onPlayModeChange: (mode: ListenPlayMode) => void;
  onUndoQueueEdit?: () => void;
  onRedoQueueEdit?: () => void;
  queueCanUndo?: boolean;
  queueCanRedo?: boolean;
  showFooter?: boolean;
}) {
  return (
    <section
      data-listen-companion-mode="queue"
      className="listen-workspace-queue-surface flex h-full min-h-0 flex-col overflow-hidden"
    >
      <div
        className="listen-workspace-queue-content min-h-0 flex-1 overflow-y-auto overscroll-contain px-0 pb-2 pt-3 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        data-companion-scroll-owner="queue"
      >
        {props.entries.length === 0 ? (
          <div className="listen-workspace-queue-empty flex h-full min-h-0 flex-col items-center justify-center px-6 pb-8">
            <span className="listen-workspace-queue-empty__icon flex h-14 w-14 items-center justify-center">
              <ListMusic className="h-6 w-6" />
            </span>
            <span className="listen-workspace-queue-empty__label mt-3">
              {props.text.listen.upNextEmpty}
            </span>
          </div>
        ) : (
          <div className="space-y-0.5 pb-2 pr-0.5">
            {props.entries.map((entry) => (
              <ListenWorkspaceQueueRow
                key={entry.id}
                entry={entry}
                text={props.text}
              />
            ))}
          </div>
        )}
      </div>

      {props.showFooter === false ? null : (
        <footer className="listen-workspace-queue-footer wails-no-drag flex min-h-12 shrink-0 items-center justify-between gap-2 px-0 py-2">
          <ListenWorkspaceQueueModeSwitch
            playMode={props.playMode}
            text={props.text}
            onChange={props.onPlayModeChange}
          />
          <div className="listen-workspace-queue-footer__actions ml-auto flex shrink-0 items-center justify-end gap-0.5">
            {props.onUndoQueueEdit ? (
              <ListenWorkspaceQueueIconButton
                label={props.text.listen.undoQueue}
                disabled={!props.queueCanUndo}
                onClick={props.onUndoQueueEdit}
              >
                <Undo2 className="h-3.5 w-3.5" />
              </ListenWorkspaceQueueIconButton>
            ) : null}
            {props.onRedoQueueEdit ? (
              <ListenWorkspaceQueueIconButton
                label={props.text.listen.redoQueue}
                disabled={!props.queueCanRedo}
                onClick={props.onRedoQueueEdit}
              >
                <Redo2 className="h-3.5 w-3.5" />
              </ListenWorkspaceQueueIconButton>
            ) : null}
            {props.onClearQueue ? (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                shape="capsule"
                className="listen-workspace-queue-clear ml-0.5 shrink-0 px-2 py-1"
                disabled={!props.canClear}
                onClick={props.onClearQueue}
              >
                {props.text.actions.clear}
              </Button>
            ) : null}
          </div>
        </footer>
      )}
    </section>
  );
}

export function ListenWorkspaceQueueModeSwitch(props: {
  playMode: ListenPlayMode;
  text: ListenWorkspaceCompanionText;
  onChange: (mode: ListenPlayMode) => void;
}) {
  const buttonRefs = React.useRef<Array<HTMLButtonElement | null>>([]);
  const items: Array<{
    mode: ListenPlayMode;
    label: string;
    icon: React.ReactNode;
  }> = [
    {
      mode: "order",
      label: props.text.listen.playModeOrder,
      icon: <ListMusic className="h-4 w-4" />,
    },
    {
      mode: "shuffle",
      label: props.text.listen.playModeShuffle,
      icon: <Shuffle className="h-4 w-4" />,
    },
    {
      mode: "repeat",
      label: props.text.listen.playModeRepeat,
      icon: <Repeat1 className="h-4 w-4" />,
    },
  ];

  return (
    <div
      role="radiogroup"
      aria-label={props.text.listen.playbackMode}
      className="listen-companion-mode-controls wails-no-drag"
    >
      {items.map((item, index) => {
        const active = props.playMode === item.mode;
        return (
          <Button
            ref={(node) => {
              buttonRefs.current[index] = node;
            }}
            key={item.mode}
            type="button"
            variant="ghost"
            size="icon"
            shape="circle"
            role="radio"
            aria-checked={active}
            data-active={active ? "true" : "false"}
            className={cn(
              LISTEN_PLAYER_ICON_BUTTON_CLASS,
              LISTEN_PLAYER_FOOTER_ICON_BUTTON_CLASS,
              "listen-companion-mode-controls__button",
            )}
            aria-label={item.label}
            title={item.label}
            tabIndex={active ? 0 : -1}
            onClick={() => props.onChange(item.mode)}
            onKeyDown={(event) => {
              let nextIndex = index;
              if (event.key === "Home") {
                nextIndex = 0;
              } else if (event.key === "End") {
                nextIndex = items.length - 1;
              } else if (
                event.key === "ArrowLeft" ||
                event.key === "ArrowUp"
              ) {
                nextIndex = (index - 1 + items.length) % items.length;
              } else if (
                event.key === "ArrowRight" ||
                event.key === "ArrowDown"
              ) {
                nextIndex = (index + 1) % items.length;
              } else {
                return;
              }
              event.preventDefault();
              props.onChange(items[nextIndex].mode);
              buttonRefs.current[nextIndex]?.focus();
            }}
          >
            {item.icon}
          </Button>
        );
      })}
    </div>
  );
}

function ListenWorkspaceQueueRow(props: {
  entry: ListenWorkspaceQueueEntry;
  text: ListenWorkspaceCompanionText;
}) {
  const entry = props.entry;
  const hasMenu = Boolean(
    entry.onRemove || entry.onMoveUp || entry.onMoveDown,
  );

  return (
    <div
      data-selected={entry.selected ? "true" : "false"}
      className="listen-workspace-queue-row group/queue-row grid min-h-[3.65rem] grid-cols-[2.65rem_minmax(0,1fr)_auto] items-center gap-2 px-1.5 py-1.5"
    >
      <Button
        type="button"
        variant="ghost"
        size="icon"
        shape="square"
        className="listen-workspace-queue-row__artwork relative h-[2.65rem] w-[2.65rem] overflow-hidden"
        aria-label={`${props.text.listen.play}: ${entry.title}`}
        onClick={entry.onSelect}
      >
        <ListenWorkspaceCompanionArtwork
          candidates={entry.artworkCandidates}
          title={entry.title}
          className="h-full w-full object-cover"
        />
        {entry.selected ? (
          <span
            aria-hidden="true"
            className="listen-workspace-queue-playing-overlay absolute inset-0 flex items-center justify-center gap-0.5"
          >
            <span className="listen-workspace-queue-playing-overlay__bar h-2 w-0.5" />
            <span className="listen-workspace-queue-playing-overlay__bar h-3.5 w-0.5" />
            <span className="listen-workspace-queue-playing-overlay__bar h-2.5 w-0.5" />
          </span>
        ) : null}
      </Button>

      <button
        type="button"
        className="listen-workspace-queue-row__identity min-w-0"
        onClick={entry.onSelect}
      >
        <span
          className="listen-workspace-queue-row__title block truncate"
        >
          {entry.title}
        </span>
        <span className="listen-workspace-queue-row__subtitle mt-0.5 block truncate">
          {entry.artist || entry.durationLabel}
        </span>
      </button>

      <div className="flex min-w-7 items-center justify-end">
        {hasMenu ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                shape="circle"
                className="listen-workspace-queue-row__menu h-7 w-7"
                aria-label={`${props.text.listen.more}: ${entry.title}`}
                title={props.text.listen.more}
              >
                <Ellipsis className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" side="bottom" className="z-[var(--app-layer-popover)] w-44">
              <DropdownMenuItem disabled={!entry.onMoveUp} onSelect={entry.onMoveUp}>
                {props.text.listen.moveQueueItemUp}
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!entry.onMoveDown}
                onSelect={entry.onMoveDown}
              >
                {props.text.listen.moveQueueItemDown}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                disabled={!entry.onRemove}
                className="listen-dropdown-item--destructive"
                onSelect={entry.onRemove}
              >
                <Trash2 className="mr-2 h-3.5 w-3.5" />
                {props.text.listen.removeFromQueue}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : entry.durationLabel ? (
          <span className="listen-workspace-queue-row__duration pr-1">
            {entry.durationLabel}
          </span>
        ) : null}
      </div>
    </div>
  );
}

function ListenWorkspaceQueueIconButton(props: {
  label: string;
  disabled?: boolean;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      shape="circle"
      className="listen-workspace-queue-icon-button flex h-8 w-8 items-center justify-center"
      aria-label={props.label}
      title={props.label}
      disabled={props.disabled}
      onClick={props.onClick}
    >
      {props.children}
    </Button>
  );
}

function ListenWorkspaceCompanionArtwork(props: {
  candidates: string[];
  title: string;
  className?: string;
}) {
  return (
    <ListenCoverArtwork
      alt={props.title}
      candidates={[...props.candidates, LISTEN_DEFAULT_COVER_IMAGE_URL]}
      className={props.className}
    />
  );
}

function resolveListenWorkspaceQueueArtist(item: ListenOnlineItem) {
  const artist = resolveTrustedListenOnlineArtistLabel(item).replace(
    /^Album,\s*/i,
    "",
  );
  const normalized = artist.trim().toLowerCase();
  if (
    !normalized ||
    normalized === "album" ||
    normalized === "youtube" ||
    normalized === "youtube music" ||
    normalized === "unknown artist"
  ) {
    return "";
  }
  return artist.trim();
}
